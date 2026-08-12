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

func TestAccessLogRecordsSanitizedHeadersAndJSONBody(t *testing.T) {
	setupAccessLogMiddlewareTest(t)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestId(), AccessLog())
	engine.POST("/v1/test", RouteTag("relay"), func(c *gin.Context) {
		var payload map[string]any
		require.NoError(t, common.DecodeJson(c.Request.Body, &payload))
		c.Set("id", 42)
		c.Set("username", "owner@example.com")
		c.JSON(http.StatusCreated, gin.H{"success": true})
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
	}`, accessLog.Body)
	assert.NotContains(t, accessLog.Body, "body-secret")
	assert.NotContains(t, accessLog.Body, "token-secret")
	assert.False(t, accessLog.BodyOmitted)
	assert.Positive(t, accessLog.BodySize)
	assert.NotEmpty(t, accessLog.RequestId)
}

func TestAccessLogIgnoresNonJSONBody(t *testing.T) {
	setupAccessLogMiddlewareTest(t)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestId(), AccessLog())
	engine.POST("/api/upload", RouteTag("api"), func(c *gin.Context) {
		body, err := common.GetRequestBody(c)
		require.NoError(t, err)
		_, err = body.Seek(0, 0)
		require.NoError(t, err)
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/api/upload", strings.NewReader("plain text"))
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

func TestAccessLogSkipsWebAndItsOwnQueryEndpoint(t *testing.T) {
	setupAccessLogMiddlewareTest(t)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestId(), AccessLog())
	engine.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })
	engine.GET("/api/access-log/", RouteTag("api"), func(c *gin.Context) { c.Status(http.StatusOK) })

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/access-log/", nil))

	var count int64
	require.NoError(t, model.DB.Model(&model.AccessLog{}).Count(&count).Error)
	assert.Zero(t, count)
}
