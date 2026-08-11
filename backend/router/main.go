package router

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

func SetRouter(router *gin.Engine, assets WebAssets) {
	SetControlPlaneRouter(router)
	SetDataPlaneRouter(router)
	setFrontendRouter(router, assets)
}

// SetControlPlaneRouter registers only management, authentication, billing,
// and dashboard APIs. Relay endpoints must never be added here.
func SetControlPlaneRouter(router *gin.Engine) {
	SetApiRouter(router)
	SetDashboardRouter(router)
	SetAnthropicControlRouter(router)
}

// SetDataPlaneRouter registers only model relay and media proxy APIs. In
// particular, it must not expose any /api management endpoints.
func SetDataPlaneRouter(router *gin.Engine) {
	SetRelayRouter(router)
	SetVideoRouter(router)
	SetAnthropicDataRouter(router)
}

func setFrontendRouter(router *gin.Engine, assets WebAssets) {
	frontendBaseUrl := strings.TrimSpace(os.Getenv("FRONTEND_BASE_URL"))
	if frontendBaseUrl != "" {
		frontendBaseUrl = strings.TrimSuffix(frontendBaseUrl, "/")
		router.NoRoute(func(c *gin.Context) {
			c.Set(middleware.RouteTagKey, "web")
			c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("%s%s", frontendBaseUrl, c.Request.RequestURI))
		})
		return
	}
	if len(assets.IndexPage) > 0 {
		SetWebRouter(router, assets)
		return
	}
	router.NoRoute(func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "api")
		if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") || strings.HasPrefix(c.Request.RequestURI, "/assets") {
			controller.RelayNotFound(c)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "not found",
		})
	})
}
