package router

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
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
}

// SetDataPlaneRouter registers only model relay and media proxy APIs. In
// particular, it must not expose any /api management endpoints.
func SetDataPlaneRouter(router *gin.Engine) {
	SetRelayRouter(router)
	SetVideoRouter(router)
}

func setFrontendRouter(router *gin.Engine, assets WebAssets) {
	frontendBaseUrl := os.Getenv("FRONTEND_BASE_URL")
	if common.IsMasterNode && frontendBaseUrl != "" {
		frontendBaseUrl = ""
		common.SysLog("FRONTEND_BASE_URL is ignored on master node")
	}
	if frontendBaseUrl == "" {
		SetWebRouter(router, assets)
	} else {
		frontendBaseUrl = strings.TrimSuffix(frontendBaseUrl, "/")
		router.NoRoute(func(c *gin.Context) {
			c.Set(middleware.RouteTagKey, "web")
			c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("%s%s", frontendBaseUrl, c.Request.RequestURI))
		})
	}
}
