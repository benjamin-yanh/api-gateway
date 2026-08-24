package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRelayCORSDoesNotAllowCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RelayCORS())
	engine.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Origin", "https://untrusted.example")
	response := httptest.NewRecorder()

	engine.ServeHTTP(response, request)

	assert.Equal(t, "*", response.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, response.Header().Get("Access-Control-Allow-Credentials"))
}

func TestControlPlaneCORSRequiresExplicitOrigin(t *testing.T) {
	t.Setenv("CONTROL_PLANE_CORS_ORIGINS", "https://admin.example")
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(ControlPlaneCORS())
	engine.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	allowed := httptest.NewRequest(http.MethodGet, "/", nil)
	allowed.Header.Set("Origin", "https://admin.example")
	allowedResponse := httptest.NewRecorder()
	engine.ServeHTTP(allowedResponse, allowed)
	assert.Equal(t, "https://admin.example", allowedResponse.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", allowedResponse.Header().Get("Access-Control-Allow-Credentials"))

	blocked := httptest.NewRequest(http.MethodGet, "/", nil)
	blocked.Header.Set("Origin", "https://evil.example")
	blockedResponse := httptest.NewRecorder()
	engine.ServeHTTP(blockedResponse, blocked)
	assert.Empty(t, blockedResponse.Header().Get("Access-Control-Allow-Origin"))
}

func TestControlPlaneCORSDefaultsToSameOriginWithoutPanic(t *testing.T) {
	t.Setenv("CONTROL_PLANE_CORS_ORIGINS", "")
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(ControlPlaneCORS())
	engine.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Origin", "https://evil.example")
	response := httptest.NewRecorder()

	engine.ServeHTTP(response, request)

	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Empty(t, response.Header().Get("Access-Control-Allow-Origin"))
}
