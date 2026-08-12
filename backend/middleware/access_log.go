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
const accessLogRedactedValue = "[REDACTED]"

type accessLogBodyReader struct {
	io.Reader
	io.Closer
}

// AccessLog records API request metadata after the request completes. Static
// web traffic and the access-log query endpoints themselves are excluded.
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		requestURL := sanitizedAccessLogURL(c.Request.URL)
		headers := sanitizedAccessLogHeaders(c.Request.Header)
		body, bodySize, bodyOmitted := captureAccessLogJSONBody(c)

		c.Next()

		routeTag, _ := c.Get(RouteTagKey)
		if routeTag != "api" && routeTag != "relay" && routeTag != "old_api" {
			return
		}
		if strings.HasPrefix(c.Request.URL.Path, "/api/access-log") {
			return
		}

		requestIDValue, _ := c.Get(common.RequestIdKey)
		requestID, _ := requestIDValue.(string)
		responseSize := int64(c.Writer.Size())
		if responseSize < 0 {
			responseSize = 0
		}
		accessLog := &model.AccessLog{
			CreatedAt:    startedAt.Unix(),
			RequestId:    requestID,
			UserId:       c.GetInt("id"),
			Username:     c.GetString("username"),
			Method:       c.Request.Method,
			Url:          requestURL,
			Route:        c.FullPath(),
			Status:       c.Writer.Status(),
			LatencyMs:    time.Since(startedAt).Milliseconds(),
			ResponseSize: responseSize,
			Ip:           c.ClientIP(),
			NodeName:     common.NodeName,
			Headers:      headers,
			Body:         body,
			BodySize:     bodySize,
			BodyOmitted:  bodyOmitted,
		}
		if err := model.CreateAccessLog(accessLog); err != nil {
			common.SysError("failed to record access log: " + err.Error())
		}
	}
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
	sanitized, err := common.Marshal(redactAccessLogJSON(value))
	if err != nil {
		return "", bodySize, true
	}
	return string(sanitized), bodySize, false
}

func isJSONMediaType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func redactAccessLogJSON(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		redacted := make(map[string]any, len(typed))
		for key, item := range typed {
			if isSensitiveAccessLogKey(key) {
				redacted[key] = accessLogRedactedValue
				continue
			}
			redacted[key] = redactAccessLogJSON(item)
		}
		return redacted
	case []any:
		redacted := make([]any, len(typed))
		for index, item := range typed {
			redacted[index] = redactAccessLogJSON(item)
		}
		return redacted
	default:
		return value
	}
}

func sanitizedAccessLogHeaders(headers http.Header) string {
	values := make(map[string][]string, len(headers))
	for key, items := range headers {
		if isSensitiveAccessLogKey(key) {
			values[key] = []string{accessLogRedactedValue}
			continue
		}
		values[key] = append([]string(nil), items...)
	}
	encoded, err := common.Marshal(values)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func sanitizedAccessLogURL(requestURL *url.URL) string {
	if requestURL == nil {
		return ""
	}
	copyURL := *requestURL
	query := copyURL.Query()
	for key := range query {
		if isSensitiveAccessLogKey(key) {
			query.Set(key, accessLogRedactedValue)
		}
	}
	copyURL.RawQuery = query.Encode()
	return copyURL.RequestURI()
}

func isSensitiveAccessLogKey(key string) bool {
	normalized := strings.NewReplacer("-", "_", ".", "_").Replace(strings.ToLower(strings.TrimSpace(key)))
	switch normalized {
	case "authorization", "proxy_authorization", "cookie", "set_cookie",
		"password", "passwd", "secret", "client_secret", "private_key",
		"api_key", "apikey", "x_api_key", "access_token", "refresh_token",
		"id_token", "session_token", "session_id", "token", "credential",
		"signature", "key":
		return true
	}
	return strings.Contains(normalized, "authorization") ||
		strings.HasSuffix(normalized, "_password") ||
		strings.HasSuffix(normalized, "_secret") ||
		strings.HasSuffix(normalized, "_api_key") ||
		strings.HasSuffix(normalized, "_token") ||
		strings.HasSuffix(normalized, "_signature")
}
