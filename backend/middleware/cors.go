package middleware

import (
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// RelayCORS permits browser clients from any origin, but never permits cookies
// or other ambient credentials on the public relay surface.
func RelayCORS() gin.HandlerFunc {
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowCredentials = false
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"*"}
	return cors.New(config)
}

// ControlPlaneCORS defaults to same-origin. Cross-origin dashboard clients must
// be explicitly allowlisted with CONTROL_PLANE_CORS_ORIGINS (comma-separated).
func ControlPlaneCORS() gin.HandlerFunc {
	config := cors.DefaultConfig()
	for _, origin := range strings.Split(os.Getenv("CONTROL_PLANE_CORS_ORIGINS"), ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			config.AllowOrigins = append(config.AllowOrigins, origin)
		}
	}
	if len(config.AllowOrigins) == 0 {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	config.AllowCredentials = len(config.AllowOrigins) > 0
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Authorization", "Content-Type", "X-Requested-With", "X-CSRF-Token"}
	return cors.New(config)
}

func Version() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-New-Api-Version", common.Version)
		c.Next()
	}
}

// SecurityHeaders applies browser hardening without constraining the frontend's
// script and connection sources. TLS termination proxies should forward
// X-Forwarded-Proto so HSTS is emitted on public HTTPS responses.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Content-Security-Policy", "frame-ancestors 'none'; base-uri 'self'; object-src 'none'")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}
