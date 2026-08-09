package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStandaloneBackendReturnsJSONNotFoundWithoutFrontendURL(t *testing.T) {
	t.Setenv("FRONTEND_BASE_URL", "")
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	setFrontendRouter(engine, WebAssets{})

	request := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.JSONEq(t, `{"success":false,"message":"not found"}`, recorder.Body.String())
}

func TestStandaloneBackendRedirectsToConfiguredFrontend(t *testing.T) {
	t.Setenv("FRONTEND_BASE_URL", "https://console.example.com/")
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	setFrontendRouter(engine, WebAssets{})

	request := httptest.NewRequest(http.MethodGet, "/pricing?group=default", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusMovedPermanently, recorder.Code)
	assert.Equal(t, "https://console.example.com/pricing?group=default", recorder.Header().Get("Location"))
}
