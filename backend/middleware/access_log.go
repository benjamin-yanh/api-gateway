package middleware

import (
	"bytes"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const maxAccessLogJSONBodyBytes int64 = 256 << 10
const maxAccessLogResponseBodyBytes = 1 << 20

type accessLogBodyReader struct {
	io.Reader
	io.Closer
}

// accessLogResponseWriter forwards every response chunk immediately while
// retaining a bounded copy of JSON and SSE output for the access-log detail.
// It does not buffer delivery, so streaming behavior and flushing are unchanged.
type accessLogResponseWriter struct {
	gin.ResponseWriter
	body      bytes.Buffer
	truncated bool
}

func (w *accessLogResponseWriter) Write(data []byte) (int, error) {
	w.capture(data)
	return w.ResponseWriter.Write(data)
}

func (w *accessLogResponseWriter) WriteString(data string) (int, error) {
	w.capture([]byte(data))
	return w.ResponseWriter.WriteString(data)
}

func (w *accessLogResponseWriter) capture(data []byte) {
	if !isAccessLogResponseMediaType(w.Header().Get("Content-Type")) || len(data) == 0 {
		return
	}
	remaining := maxAccessLogResponseBodyBytes - w.body.Len()
	if remaining <= 0 {
		w.truncated = true
		return
	}
	if len(data) > remaining {
		_, _ = w.body.Write(data[:remaining])
		w.truncated = true
		return
	}
	_, _ = w.body.Write(data)
}

// AccessLog records data-plane request metadata after the request completes.
// Control-plane, dashboard, authentication, and static web traffic are not
// persisted, even though this middleware is installed on every server role.
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		requestURL := accessLogURL(c.Request.URL)
		headers := accessLogHeaders(c.Request.Header)
		body, bodySize, bodyOmitted := captureAccessLogJSONBody(c)
		responseWriter := &accessLogResponseWriter{ResponseWriter: c.Writer}
		c.Writer = responseWriter

		c.Next()

		routeTag, _ := c.Get(RouteTagKey)
		if routeTag != "relay" {
			return
		}

		requestIDValue, _ := c.Get(common.RequestIdKey)
		requestID, _ := requestIDValue.(string)
		responseSize := int64(c.Writer.Size())
		if responseSize < 0 {
			responseSize = 0
		}
		responseBody, responseBodyType := accessLogResponseBody(
			responseWriter.body.Bytes(),
			responseWriter.Header().Get("Content-Type"),
		)
		accessLog := &model.AccessLog{
			CreatedAt:             startedAt.Unix(),
			RequestId:             requestID,
			UserId:                c.GetInt("id"),
			Username:              c.GetString("username"),
			Method:                c.Request.Method,
			Url:                   requestURL,
			Route:                 c.FullPath(),
			Status:                c.Writer.Status(),
			LatencyMs:             time.Since(startedAt).Milliseconds(),
			ResponseSize:          responseSize,
			Ip:                    c.ClientIP(),
			NodeName:              common.NodeName,
			Headers:               headers,
			Body:                  model.AccessLogPayload(body),
			BodySize:              bodySize,
			BodyOmitted:           bodyOmitted,
			ResponseBody:          model.AccessLogPayload(responseBody),
			ResponseBodyType:      responseBodyType,
			ResponseBodyTruncated: responseWriter.truncated,
		}
		if err := model.CreateAccessLog(accessLog); err != nil {
			common.SysError("failed to record access log: " + err.Error())
		}
	}
}

func isAccessLogResponseMediaType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return mediaType == "text/event-stream" ||
		mediaType == "application/x-ndjson" ||
		mediaType == "application/json" ||
		strings.HasSuffix(mediaType, "+json")
}

func accessLogResponseBody(body []byte, contentType string) (string, string) {
	if len(body) == 0 {
		return "", ""
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || !isAccessLogResponseMediaType(contentType) {
		return "", ""
	}
	bodyText := strings.ToValidUTF8(string(body), "\uFFFD")
	return bodyText, mediaType
}

func captureAccessLogJSONBody(c *gin.Context) (string, int64, bool) {
	if c.Request.Body == nil || !isJSONMediaType(c.GetHeader("Content-Type")) {
		return "", 0, false
	}
	bodySize := c.Request.ContentLength
	if bodySize > maxAccessLogJSONBodyBytes {
		return "", bodySize, true
	}
	originalBody := c.Request.Body
	body, err := io.ReadAll(io.LimitReader(originalBody, maxAccessLogJSONBodyBytes+1))
	if err != nil {
		c.Request.Body = &accessLogBodyReader{
			Reader: io.MultiReader(bytes.NewReader(body), originalBody),
			Closer: originalBody,
		}
		return "", int64(len(body)), true
	}
	if int64(len(body)) > maxAccessLogJSONBodyBytes {
		c.Request.Body = &accessLogBodyReader{
			Reader: io.MultiReader(bytes.NewReader(body), originalBody),
			Closer: originalBody,
		}
		return "", int64(len(body)), true
	}
	_ = originalBody.Close()
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	bodySize = int64(len(body))
	if len(body) == 0 {
		return "", 0, false
	}

	var value any
	if err := common.Unmarshal(body, &value); err != nil {
		return "", bodySize, false
	}
	return strings.ToValidUTF8(string(body), "\uFFFD"), bodySize, false
}

func isJSONMediaType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func accessLogHeaders(headers http.Header) string {
	encoded, err := common.Marshal(headers)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func accessLogURL(requestURL *url.URL) string {
	if requestURL == nil {
		return ""
	}
	return requestURL.RequestURI()
}
