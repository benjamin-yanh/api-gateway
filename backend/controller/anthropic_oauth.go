package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const anthropicOAuthScopes = "org:create_api_key user:profile user:inference user:sessions:claude_code user:mcp_servers user:file_upload"

// anthropicOAuthTokenRequest 同时覆盖授权码与刷新令牌两种 grant。
// Claude Desktop 使用 JSON，请求表单的支持用于兼容其他标准 OAuth 客户端。
type anthropicOAuthTokenRequest struct {
	GrantType    string `json:"grant_type"`
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
	ClientID     string `json:"client_id"`
	CodeVerifier string `json:"code_verifier"`
	State        string `json:"state"`
	RefreshToken string `json:"refresh_token"`
}

// AnthropicOAuthAuthorize 是 Claude 客户端看到的 OAuth 授权入口。
// 实际授权确认由现有 /native-app 页面完成，这样登录、二次验证和前端会话
// 都只维护一套实现；这里仅负责校验外部参数并安全地跳转到确认页。
func AnthropicOAuthAuthorize(c *gin.Context) {
	request := nativeAppAuthorizeRequest{
		ClientID:            strings.TrimSpace(c.Query("client_id")),
		RedirectURI:         strings.TrimSpace(c.Query("redirect_uri")),
		CodeChallenge:       strings.TrimSpace(c.Query("code_challenge")),
		CodeChallengeMethod: strings.TrimSpace(c.Query("code_challenge_method")),
		State:               strings.TrimSpace(c.Query("state")),
	}
	_, validRedirect := validateNativeAppRedirectURI(request.RedirectURI)
	if c.Query("response_type") != "code" || !nativeAppClientIDPattern.MatchString(request.ClientID) ||
		!validRedirect || request.CodeChallengeMethod != "S256" ||
		!nativeAppCodeChallengePattern.MatchString(request.CodeChallenge) || !nativeAppStatePattern.MatchString(request.State) {
		writeAnthropicOAuthError(c, http.StatusBadRequest, "invalid_request", "Invalid authorization request")
		return
	}
	query := url.Values{}
	query.Set("client_id", request.ClientID)
	query.Set("redirect_uri", request.RedirectURI)
	query.Set("code_challenge", request.CodeChallenge)
	query.Set("code_challenge_method", request.CodeChallengeMethod)
	query.Set("state", request.State)
	setAuthNoStore(c)
	c.Redirect(http.StatusFound, "/native-app?"+query.Encode())
}

// AnthropicOAuthToken 将一次性授权码换成面板登录 Session，也支持刷新该
// Session。这里返回的是 OAuth 登录凭据，不是调用模型使用的 sk- API Key。
func AnthropicOAuthToken(c *gin.Context) {
	var request anthropicOAuthTokenRequest
	contentType := strings.ToLower(c.GetHeader("Content-Type"))
	if strings.HasPrefix(contentType, "application/json") {
		if err := common.DecodeJson(c.Request.Body, &request); err != nil {
			writeAnthropicOAuthError(c, http.StatusBadRequest, "invalid_request", "Invalid token request")
			return
		}
	} else {
		request = anthropicOAuthTokenRequest{
			GrantType:    c.PostForm("grant_type"),
			Code:         c.PostForm("code"),
			RedirectURI:  c.PostForm("redirect_uri"),
			ClientID:     c.PostForm("client_id"),
			CodeVerifier: c.PostForm("code_verifier"),
			State:        c.PostForm("state"),
			RefreshToken: c.PostForm("refresh_token"),
		}
	}

	switch strings.TrimSpace(request.GrantType) {
	case "authorization_code":
		// client_id 和 state 在通用本地应用接口中是可选字段，但 Claude
		// 协议必须提供并与授权阶段完全一致，避免授权码被其他客户端换取。
		if !nativeAppClientIDPattern.MatchString(strings.TrimSpace(request.ClientID)) ||
			!nativeAppStatePattern.MatchString(strings.TrimSpace(request.State)) {
			writeAnthropicOAuthError(c, http.StatusBadRequest, "invalid_request", "client_id and state are required")
			return
		}
		bundle, _, err := exchangeNativeAppAuthorizationCode(nativeAppTokenRequest{
			ClientID:     request.ClientID,
			Code:         request.Code,
			RedirectURI:  request.RedirectURI,
			CodeVerifier: request.CodeVerifier,
			State:        request.State,
		}, c.ClientIP(), c.Request.UserAgent())
		if err != nil {
			writeAnthropicTokenExchangeError(c, err)
			return
		}
		writeAnthropicTokenBundle(c, bundle)
	case "refresh_token":
		refreshToken := strings.TrimSpace(request.RefreshToken)
		if refreshToken == "" {
			writeAnthropicOAuthError(c, http.StatusBadRequest, "invalid_request", "refresh_token is required")
			return
		}
		bundle, _, err := service.RefreshLoginSession(refreshToken, "", c.ClientIP(), c.Request.UserAgent())
		if err != nil {
			writeAnthropicOAuthError(c, http.StatusBadRequest, "invalid_grant", "Refresh token is invalid or expired")
			return
		}
		writeAnthropicTokenBundle(c, bundle)
	default:
		writeAnthropicOAuthError(c, http.StatusBadRequest, "unsupported_grant_type", "Only authorization_code and refresh_token are supported")
	}
}

// AnthropicOAuthCodeCallback 保留 Anthropic 官方路径的兼容响应。
// 桌面应用的正常流程不会使用这里作为回调，而是在本机 loopback 地址接收 code。
func AnthropicOAuthCodeCallback(c *gin.Context) {
	setAuthNoStore(c)
	if oauthError := strings.TrimSpace(c.Query("error")); oauthError != "" {
		writeAnthropicOAuthError(c, http.StatusBadRequest, oauthError, strings.TrimSpace(c.Query("error_description")))
		return
	}
	code := strings.TrimSpace(c.Query("code"))
	state := strings.TrimSpace(c.Query("state"))
	if code == "" || state == "" {
		writeAnthropicOAuthError(c, http.StatusBadRequest, "invalid_request", "code and state are required")
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": code, "state": state})
}

// AnthropicCreateAPIKey 为已完成 OAuth 登录的 Claude 客户端创建独立模型 Key。
// 必须是服务端可验证的登录 Session；面板 PAT 不能借此签发新 Key。
func AnthropicCreateAPIKey(c *gin.Context) {
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		writeAnthropicOAuthError(c, http.StatusUnauthorized, "invalid_token", "A live OAuth session is required")
		return
	}
	count, err := model.CountUserTokens(identity.UserID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	maxTokens := operation_setting.GetMaxUserTokens()
	if int(count) >= maxTokens {
		writeAnthropicOAuthError(c, http.StatusConflict, "token_limit_reached", fmt.Sprintf("Maximum token count reached (%d)", maxTokens))
		return
	}
	key, err := common.GenerateKey()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	now := common.GetTimestamp()
	// UnlimitedQuota 表示 Key 本身不设独立额度，并不代表免费调用；模型请求
	// 仍会按所属用户余额正常预扣和结算。
	token := model.Token{
		UserId:         identity.UserID,
		Name:           "Claude Code",
		Key:            key,
		CreatedTime:    now,
		AccessedTime:   now,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}
	if err := token.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	setAuthNoStore(c)
	c.JSON(http.StatusOK, gin.H{"raw_key": "sk-" + key})
}

// AnthropicOAuthProfile 返回 Claude Desktop 会读取的组织和账户字段。
// UUID 由站点地址和用户 ID 确定性生成，重启后保持稳定，同时不暴露数据库主键。
func AnthropicOAuthProfile(c *gin.Context) {
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		writeAnthropicOAuthError(c, http.StatusUnauthorized, "invalid_token", "A live OAuth session is required")
		return
	}
	user, err := model.GetUserById(identity.UserID, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	name := strings.TrimSpace(user.DisplayName)
	if name == "" {
		name = user.Username
	}
	organizationID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(strings.TrimRight(system_setting.ServerAddress, "/"))).String()
	setAuthNoStore(c)
	c.JSON(http.StatusOK, gin.H{
		"organization": gin.H{"uuid": organizationID, "name": common.SystemName},
		"account": gin.H{
			"uuid":          uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("user:%d", user.Id))).String(),
			"email":         user.Email,
			"email_address": user.Email,
			"full_name":     name,
			"display_name":  name,
		},
	})
}

// AnthropicOAuthRoles 返回 Claude CLI 探测能力时需要的角色和授权范围。
// 这些兼容字段只描述客户端能力，后端权限仍由当前 Session 和用户角色决定。
func AnthropicOAuthRoles(c *gin.Context) {
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		writeAnthropicOAuthError(c, http.StatusUnauthorized, "invalid_token", "A live OAuth session is required")
		return
	}
	user, err := model.GetUserCache(identity.UserID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	organizationRole := "user"
	workspaceRole := "developer"
	if user.Role >= common.RoleRootUser {
		organizationRole = "owner"
		workspaceRole = "admin"
	} else if user.Role >= common.RoleAdminUser {
		organizationRole = "admin"
		workspaceRole = "admin"
	}
	setAuthNoStore(c)
	c.JSON(http.StatusOK, gin.H{
		"organization_role": organizationRole,
		"workspace_role":    workspaceRole,
		"roles":             []string{"user", "claude_code"},
		"scopes":            strings.Fields(anthropicOAuthScopes),
	})
}

// writeAnthropicTokenBundle 把内部 AuthBundle 转换为标准 OAuth Token 响应。
func writeAnthropicTokenBundle(c *gin.Context, bundle *service.AuthBundle) {
	setAuthNoStore(c)
	expiresIn := bundle.AccessExpiresAt - time.Now().Unix()
	if expiresIn < 0 {
		expiresIn = 0
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token":  bundle.AccessToken,
		"refresh_token": bundle.RefreshToken,
		"token_type":    bundle.TokenType,
		"expires_in":    expiresIn,
		"scope":         anthropicOAuthScopes,
	})
}

// writeAnthropicTokenExchangeError 将内部会话错误收敛为 OAuth 2.0 错误码，
// 避免向外部客户端暴露数据库或 Session 的实现细节。
func writeAnthropicTokenExchangeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errNativeAppInvalidRequest):
		writeAnthropicOAuthError(c, http.StatusBadRequest, "invalid_request", "Invalid token request")
	case errors.Is(err, errNativeAppInvalidCode), errors.Is(err, errNativeAppPKCE):
		writeAnthropicOAuthError(c, http.StatusBadRequest, "invalid_grant", "Authorization code is invalid, expired, or failed PKCE verification")
	case errors.Is(err, service.ErrLoginSessionInvalid), errors.Is(err, service.ErrLoginSessionRevoked):
		writeAnthropicOAuthError(c, http.StatusUnauthorized, "invalid_grant", "The browser login session is no longer valid")
	case errors.Is(err, model.ErrUserSessionLimit), errors.Is(err, model.ErrUserSessionIssuanceLimit):
		writeAnthropicOAuthError(c, http.StatusTooManyRequests, "temporarily_unavailable", "Session issuance limit reached")
	default:
		common.ApiError(c, err)
	}
}

// writeAnthropicOAuthError 保证所有兼容端点使用一致且不可缓存的 OAuth 错误格式。
func writeAnthropicOAuthError(c *gin.Context, status int, code, description string) {
	setAuthNoStore(c)
	c.JSON(status, gin.H{"error": code, "error_description": description})
}
