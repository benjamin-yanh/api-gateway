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
	assertRouteRegistered(t, routes, "GET", "/anthropic/v1/models")
	assertRouteRegistered(t, routes, "GET", "/anthropic/v1/models/:model")
	assertRouteRegistered(t, routes, "POST", "/anthropic/v1/messages")
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
	assertRouteRegistered(t, routes, "GET", "/anthropic/oauth/authorize")
	assertRouteRegistered(t, routes, "POST", "/anthropic/v1/oauth/token")
	assertRouteRegistered(t, routes, "POST", "/anthropic/api/oauth/claude_cli/create_api_key")
	assertRouteRegistered(t, routes, "GET", "/anthropic/api/oauth/claude_cli/roles")
	assertRouteRegistered(t, routes, "GET", "/anthropic/api/oauth/profile")
	for _, route := range routes {
		assert.NotEqual(t, "/v1/chat/completions", route.Path)
		assert.NotEqual(t, "/anthropic/v1/messages", route.Path)
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
