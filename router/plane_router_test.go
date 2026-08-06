package router

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataPlaneRouterDoesNotExposeControlPlaneAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	SetDataPlaneRouter(engine)

	routes := engine.Routes()
	require.NotEmpty(t, routes)
	assertRouteRegistered(t, routes, "POST", "/v1/chat/completions")
	for _, route := range routes {
		assert.Falsef(t, strings.HasPrefix(route.Path, "/api"), "data plane exposed control route %s %s", route.Method, route.Path)
		assert.Falsef(t, strings.HasPrefix(route.Path, "/dashboard"), "data plane exposed dashboard route %s %s", route.Method, route.Path)
	}
}

func TestControlPlaneRouterDoesNotExposeRelayAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	SetControlPlaneRouter(engine)

	routes := engine.Routes()
	require.NotEmpty(t, routes)
	assertRouteRegistered(t, routes, "GET", "/api/status")
	for _, route := range routes {
		assert.NotEqual(t, "/v1/chat/completions", route.Path)
		assert.Falsef(t, strings.HasPrefix(route.Path, "/mj/"), "control plane exposed relay route %s %s", route.Method, route.Path)
		assert.Falsef(t, strings.HasPrefix(route.Path, "/suno/"), "control plane exposed relay route %s %s", route.Method, route.Path)
	}
}

func assertRouteRegistered(t *testing.T, routes gin.RoutesInfo, method string, path string) {
	t.Helper()
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return
		}
	}
	require.Failf(t, "route not registered", "%s %s", method, path)
}
