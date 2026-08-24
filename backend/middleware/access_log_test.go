package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAccessLogMiddlewareTest(t *testing.T) {
	t.Helper()
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.AccessLog{}))
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
	})
}

func TestAccessLogRedactsSecretsFromHeadersURLAndJSONBodies(t *testing.T) {
	setupAccessLogMiddlewareTest(t)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestId(), AccessLog())
	engine.POST("/v1/test", RouteTag("relay"), func(c *gin.Context) {
		var payload map[string]any
		require.NoError(t, common.DecodeJson(c.Request.Body, &payload))
		c.Set("id", 42)
		c.Set("username", "owner@example.com")
		c.JSON(http.StatusCreated, gin.H{
			"success":      true,
			"access_token": "response-secret",
		})
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/test?api_key=query-secret&mode=fast", strings.NewReader(`{
		"model":"gpt-test",
		"password":"body-secret",
		"nested":{"access_token":"token-secret","message":"keep-me"}
	}`))
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("Authorization", "Bearer header-secret")
	request.Header.Set("X-Goog-Api-Key", "google-secret")
	request.Header.Set("X-Trace", "keep-header")
	request.ContentLength = -1
	response := httptest.NewRecorder()

	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusCreated, response.Code)
	var accessLog model.AccessLog
	require.NoError(t, model.DB.First(&accessLog).Error)
	assert.Equal(t, int64(42), int64(accessLog.UserId))
	assert.Equal(t, "owner@example.com", accessLog.Username)
	assert.Equal(t, "POST", accessLog.Method)
	assert.Equal(t, "/v1/test", accessLog.Route)
	assert.Equal(t, http.StatusCreated, accessLog.Status)
	assert.NotContains(t, accessLog.Url, "query-secret")
	assert.Contains(t, accessLog.Url, "api_key=%5BREDACTED%5D")
	assert.Contains(t, accessLog.Url, "mode=fast")
	assert.Contains(t, accessLog.Headers, `"Authorization":["[REDACTED]"]`)
	assert.Contains(t, accessLog.Headers, `"X-Goog-Api-Key":["[REDACTED]"]`)
	assert.Contains(t, accessLog.Headers, `"X-Trace":["keep-header"]`)
	assert.NotContains(t, accessLog.Headers, "header-secret")
	assert.NotContains(t, accessLog.Headers, "google-secret")
	assert.JSONEq(t, `{
		"model":"gpt-test",
		"password":"[REDACTED]",
		"nested":{"access_token":"[REDACTED]","message":"keep-me"}
	}`, string(accessLog.Body))
	assert.False(t, accessLog.BodyOmitted)
	assert.Positive(t, accessLog.BodySize)
	assert.Equal(t, "application/json", accessLog.ResponseBodyType)
	assert.JSONEq(t, `{"success":true,"access_token":"[REDACTED]"}`, string(accessLog.ResponseBody))
	assert.NotContains(t, string(accessLog.Body), "body-secret")
	assert.NotContains(t, string(accessLog.Body), "token-secret")
	assert.NotContains(t, string(accessLog.ResponseBody), "response-secret")
	assert.False(t, accessLog.ResponseBodyTruncated)
	assert.NotEmpty(t, accessLog.RequestId)
}

func TestAccessLogCollectsStreamingResponseInSingleRecord(t *testing.T) {
	setupAccessLogMiddlewareTest(t)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestId(), AccessLog())
	engine.POST("/v1/messages", RouteTag("relay"), func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream; charset=utf-8")
		_, err := c.Writer.WriteString("data: {\"type\":\"message_start\",\"token\":\"stream-secret\"}\n\n")
		require.NoError(t, err)
		c.Writer.Flush()
		_, err = c.Writer.WriteString("data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"hello\"}}\n\n")
		require.NoError(t, err)
		_, err = c.Writer.WriteString("data: [DONE]\n\n")
		require.NoError(t, err)
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-test"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	var logs []model.AccessLog
	require.NoError(t, model.DB.Find(&logs).Error)
	require.Len(t, logs, 1)
	accessLog := logs[0]
	assert.Equal(t, "text/event-stream", accessLog.ResponseBodyType)
	assert.Contains(t, accessLog.ResponseBody, `data: {"token":"[REDACTED]","type":"message_start"}`)
	assert.NotContains(t, accessLog.ResponseBody, "stream-secret")
	assert.Contains(t, accessLog.ResponseBody, `data: {"delta":{"text":"hello"},"type":"content_block_delta"}`)
	assert.Contains(t, accessLog.ResponseBody, "data: [DONE]")
	assert.Less(t,
		strings.Index(string(accessLog.ResponseBody), "message_start"),
		strings.Index(string(accessLog.ResponseBody), "content_block_delta"),
	)
	assert.False(t, accessLog.ResponseBodyTruncated)
	assert.JSONEq(t, `{"model":"claude-test"}`, string(accessLog.Body))
}

func TestAccessLogTruncatesOversizedStreamingResponse(t *testing.T) {
	setupAccessLogMiddlewareTest(t)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestId(), AccessLog())
	engine.GET("/v1/stream", RouteTag("relay"), func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		_, err := c.Writer.WriteString("data: " + strings.Repeat("x", maxAccessLogResponseBodyBytes+1))
		require.NoError(t, err)
	})

	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/stream", nil))

	var accessLog model.AccessLog
	require.NoError(t, model.DB.First(&accessLog).Error)
	assert.Len(t, accessLog.ResponseBody, maxAccessLogResponseBodyBytes)
	assert.True(t, accessLog.ResponseBodyTruncated)
}

func TestAccessLogIgnoresNonJSONBodyOnDataPlane(t *testing.T) {
	setupAccessLogMiddlewareTest(t)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestId(), AccessLog())
	engine.POST("/v1/audio/transcriptions", RouteTag("relay"), func(c *gin.Context) {
		body, err := common.GetRequestBody(c)
		require.NoError(t, err)
		_, err = body.Seek(0, 0)
		require.NoError(t, err)
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", strings.NewReader("plain text"))
	request.Header.Set("Content-Type", "text/plain")
	response := httptest.NewRecorder()

	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
	var accessLog model.AccessLog
	require.NoError(t, model.DB.First(&accessLog).Error)
	assert.Empty(t, accessLog.Body)
	assert.Zero(t, accessLog.BodySize)
	assert.False(t, accessLog.BodyOmitted)
}

func TestAccessLogRecordsOnlyDataPlaneRoutes(t *testing.T) {
	setupAccessLogMiddlewareTest(t)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestId(), AccessLog())
	engine.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })
	engine.POST("/api/user/login", RouteTag("api"), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"success": true}) })
	engine.GET("/dashboard", RouteTag("old_api"), func(c *gin.Context) { c.Status(http.StatusOK) })
	engine.GET("/api/access-log/", RouteTag("api"), func(c *gin.Context) { c.Status(http.StatusOK) })
	engine.GET("/v1/models", RouteTag("relay"), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"data": []any{}}) })

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/user/login", strings.NewReader(`{"username":"owner@example.com","password":"secret"}`))
	loginRequest.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(httptest.NewRecorder(), loginRequest)
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/dashboard", nil))
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/access-log/", nil))
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/models", nil))

	var count int64
	require.NoError(t, model.DB.Model(&model.AccessLog{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
	var accessLog model.AccessLog
	require.NoError(t, model.DB.First(&accessLog).Error)
	assert.Equal(t, "/v1/models", accessLog.Url)
}
