package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(SecurityHeaders())
	engine.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Forwarded-Proto", "https")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	assert.Equal(t, "nosniff", response.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", response.Header().Get("X-Frame-Options"))
	assert.Contains(t, response.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'")
	assert.Equal(t, "strict-origin-when-cross-origin", response.Header().Get("Referrer-Policy"))
	assert.Equal(t, "camera=(), microphone=(), geolocation=()", response.Header().Get("Permissions-Policy"))
	assert.Equal(t, "max-age=31536000; includeSubDomains", response.Header().Get("Strict-Transport-Security"))
}

func TestSecurityHeadersOmitsHSTSOnPlainHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(SecurityHeaders())
	engine.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Empty(t, response.Header().Get("Strict-Transport-Security"))
}
