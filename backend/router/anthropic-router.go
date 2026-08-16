package router

import (
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

// SetAnthropicControlRouter 注册登录、资料与 Key 签发接口。
// 这些接口需要访问用户和 Session 数据，因此只能部署在控制面实例。
func SetAnthropicControlRouter(router *gin.Engine) {
	login := router.Group("/auth")
	login.Use(middleware.RouteTag("api"))
	login.Use(gzip.Gzip(gzip.DefaultCompression))
	login.Use(middleware.BodyStorageCleanup())
	login.Use(middleware.GlobalAPIRateLimit())
	login.POST("/login", middleware.PasswordLoginRateLimit(), middleware.DisableCache(), middleware.AnonymousRequestBodyLimit(), controller.AnthropicPasswordLogin)

	group := router.Group("/anthropic")
	group.Use(middleware.RouteTag("api"))
	group.Use(gzip.Gzip(gzip.DefaultCompression))
	group.Use(middleware.BodyStorageCleanup())
	group.Use(middleware.GlobalAPIRateLimit())
	{
		group.POST("/auth/login", middleware.PasswordLoginRateLimit(), middleware.DisableCache(), middleware.AnonymousRequestBodyLimit(), controller.AnthropicPasswordLogin)
		group.GET("/oauth/authorize", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.AnthropicOAuthAuthorize)
		group.GET("/oauth/code/callback", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.AnthropicOAuthCodeCallback)
		group.POST("/v1/oauth/token", middleware.CriticalRateLimit(), middleware.DisableCache(), middleware.AnonymousRequestBodyLimit(), controller.AnthropicOAuthToken)
		group.POST("/api/oauth/claude_cli/create_api_key", middleware.UserAuth(), middleware.CriticalRateLimit(), middleware.DisableCache(), controller.AnthropicCreateAPIKey)
		group.GET("/api/oauth/claude_cli/roles", middleware.UserAuth(), middleware.DisableCache(), controller.AnthropicOAuthRoles)
		group.GET("/api/oauth/profile", middleware.UserAuth(), middleware.DisableCache(), controller.AnthropicOAuthProfile)
	}
}

// SetAnthropicDataRouter 注册模型查询和 Messages 转发接口。
// 对外保留 /anthropic 前缀，对内则恢复为项目已有的标准 Anthropic 路径，
// 从而让鉴权、渠道分发、计费和上游适配逻辑全部复用原实现。
func SetAnthropicDataRouter(router *gin.Engine) {
	stripPrefix := func(c *gin.Context) {
		// Gin 已经完成路由匹配，此时修改 URL 只影响后续中间件和 Relay，
		// 不会导致请求重新匹配或绕过当前路由的鉴权链。
		c.Request.URL.Path = strings.TrimPrefix(c.Request.URL.Path, "/anthropic")
		if c.Request.URL.RawPath != "" {
			c.Request.URL.RawPath = strings.TrimPrefix(c.Request.URL.RawPath, "/anthropic")
		}
		c.Next()
	}

	models := router.Group("/anthropic/v1/models")
	models.Use(middleware.RouteTag("relay"), stripPrefix)
	models.GET("", func(c *gin.Context) {
		controller.ListModels(c, constant.ChannelTypeAnthropic)
	})
	models.GET("/:model", func(c *gin.Context) {
		controller.RetrieveModel(c, constant.ChannelTypeAnthropic)
	})

	messages := router.Group("/anthropic/v1/messages")
	messages.Use(middleware.RouteTag("relay"), stripPrefix, middleware.SystemPerformanceCheck(), middleware.TokenAuth(), middleware.ModelRequestRateLimit(), middleware.Distribute())
	messages.POST("", func(c *gin.Context) {
		controller.Relay(c, types.RelayFormatClaude)
	})
}
