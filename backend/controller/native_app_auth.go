package controller

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const nativeAppAuthorizationTTL = 2 * time.Minute

var (
	errNativeAppInvalidRequest = errors.New("invalid native app token request")
	errNativeAppInvalidCode    = errors.New("native app authorization code is invalid or expired")
	errNativeAppPKCE           = errors.New("native app PKCE verification failed")

	nativeAppClientIDPattern      = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	nativeAppCodeChallengePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43,128}$`)
	nativeAppCodeVerifierPattern  = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)
	nativeAppStatePattern         = regexp.MustCompile(`^[A-Za-z0-9._~-]{16,512}$`)
)

type nativeAppAuthorizeRequest struct {
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
	State               string `json:"state"`
}

type nativeAppTokenRequest struct {
	ClientID     string `json:"client_id,omitempty"`
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
	CodeVerifier string `json:"code_verifier"`
	State        string `json:"state,omitempty"`
}

type nativeAppRefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type nativeAppFlowPayload struct {
	ClientID      string `json:"client_id"`
	RedirectURI   string `json:"redirect_uri"`
	CodeChallenge string `json:"code_challenge"`
	State         string `json:"state"`
}

func validateNativeAppRedirectURI(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "http" || parsed.Opaque != "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return "", false
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1024 || port > 65535 {
		return "", false
	}
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.String(), true
}

func nativeAppPKCEChallenge(verifier string) string {
	digest := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func writeNativeAppBundle(c *gin.Context, bundle *service.AuthBundle, user *model.User) {
	setAuthNoStore(c)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"access_token":       bundle.AccessToken,
			"token_type":         bundle.TokenType,
			"access_expires_at":  bundle.AccessExpiresAt,
			"refresh_token":      bundle.RefreshToken,
			"refresh_expires_at": bundle.Session.ExpiresAt,
			"session":            bundle.Session,
			"user":               buildSelfUserData(user),
		},
	})
}

// AuthorizeNativeApp creates a single-use authorization code for a loopback
// native application. The browser session remains separate from the app session.
func AuthorizeNativeApp(c *gin.Context) {
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "A browser login session is required"})
		return
	}
	var request nativeAppAuthorizeRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid authorization request"})
		return
	}
	request.ClientID = strings.TrimSpace(request.ClientID)
	request.CodeChallenge = strings.TrimSpace(request.CodeChallenge)
	request.CodeChallengeMethod = strings.TrimSpace(request.CodeChallengeMethod)
	redirectURI, validRedirect := validateNativeAppRedirectURI(request.RedirectURI)
	if !nativeAppClientIDPattern.MatchString(request.ClientID) || !validRedirect ||
		request.CodeChallengeMethod != "S256" || !nativeAppCodeChallengePattern.MatchString(request.CodeChallenge) ||
		!nativeAppStatePattern.MatchString(request.State) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid authorization request"})
		return
	}
	payload, err := common.Marshal(nativeAppFlowPayload{
		ClientID:      request.ClientID,
		RedirectURI:   redirectURI,
		CodeChallenge: request.CodeChallenge,
		State:         request.State,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	code, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose:   model.AuthFlowPurposeNativeApp,
		Intent:    model.AuthFlowIntentLogin,
		UserId:    identity.UserID,
		SessionId: identity.SessionID,
		Payload:   string(payload),
		ExpiresAt: time.Now().Add(nativeAppAuthorizationTTL),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	callback, _ := url.Parse(redirectURI)
	query := callback.Query()
	query.Set("code", code)
	query.Set("state", request.State)
	callback.RawQuery = query.Encode()
	setAuthNoStore(c)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"redirect_url": callback.String()}})
}

// ExchangeNativeAppCode verifies PKCE and exchanges the one-time code for a
// dedicated app login session. Refresh tokens are returned only in this body.
func ExchangeNativeAppCode(c *gin.Context) {
	var request nativeAppTokenRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid token request"})
		return
	}
	bundle, user, err := exchangeNativeAppAuthorizationCode(request, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		switch {
		case errors.Is(err, errNativeAppInvalidRequest):
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid token request"})
		case errors.Is(err, errNativeAppInvalidCode):
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Authorization code is invalid or expired"})
		case errors.Is(err, errNativeAppPKCE):
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "PKCE verification failed"})
		case errors.Is(err, service.ErrLoginSessionInvalid), errors.Is(err, service.ErrLoginSessionRevoked),
			errors.Is(err, model.ErrUserSessionLimit), errors.Is(err, model.ErrUserSessionIssuanceLimit):
			writeAuthSessionError(c, err)
		default:
			common.ApiError(c, err)
		}
		return
	}
	writeNativeAppBundle(c, bundle, user)
}

func exchangeNativeAppAuthorizationCode(request nativeAppTokenRequest, ip, userAgent string) (*service.AuthBundle, *model.User, error) {
	request.Code = strings.TrimSpace(request.Code)
	request.ClientID = strings.TrimSpace(request.ClientID)
	request.CodeVerifier = strings.TrimSpace(request.CodeVerifier)
	redirectURI, validRedirect := validateNativeAppRedirectURI(request.RedirectURI)
	if request.Code == "" || !validRedirect || !nativeAppCodeVerifierPattern.MatchString(request.CodeVerifier) {
		return nil, nil, errNativeAppInvalidRequest
	}
	flow, err := model.GetAuthFlow(request.Code, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeNativeApp})
	if err != nil {
		return nil, nil, errNativeAppInvalidCode
	}
	var payload nativeAppFlowPayload
	if err := common.UnmarshalJsonStr(flow.Payload, &payload); err != nil || payload.RedirectURI != redirectURI {
		return nil, nil, errNativeAppInvalidRequest
	}
	if request.ClientID != "" && subtle.ConstantTimeCompare([]byte(request.ClientID), []byte(payload.ClientID)) != 1 {
		return nil, nil, errNativeAppInvalidRequest
	}
	// state 和 PKCE verifier 都使用常量时间比较，避免把匹配进度通过耗时泄露。
	if request.State != "" && subtle.ConstantTimeCompare([]byte(request.State), []byte(payload.State)) != 1 {
		return nil, nil, errNativeAppInvalidRequest
	}
	actualChallenge := nativeAppPKCEChallenge(request.CodeVerifier)
	if subtle.ConstantTimeCompare([]byte(actualChallenge), []byte(payload.CodeChallenge)) != 1 {
		return nil, nil, errNativeAppPKCE
	}
	if _, err := service.ValidateSessionReference(flow.UserId, flow.SessionId); err != nil {
		return nil, nil, errNativeAppInvalidCode
	}
	// 先原子消费授权码再签发 Session，确保并发换取时只有一个请求成功。
	if _, err := model.ConsumeAuthFlow(request.Code, model.AuthFlowMatch{
		Purpose:   model.AuthFlowPurposeNativeApp,
		Intent:    model.AuthFlowIntentLogin,
		UserId:    flow.UserId,
		SessionId: flow.SessionId,
	}); err != nil {
		return nil, nil, errNativeAppInvalidCode
	}
	bundle, err := service.CreateLoginSession(flow.UserId, "native_app:"+payload.ClientID, ip, userAgent)
	if err != nil {
		return nil, nil, err
	}
	user, err := model.GetUserById(flow.UserId, false)
	if err != nil {
		_, _ = model.RevokeUserSession(flow.UserId, bundle.Session.SID, "user_load_failed")
		return nil, nil, err
	}
	return bundle, user, nil
}

func RefreshNativeAppAuth(c *gin.Context) {
	var request nativeAppRefreshRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || strings.TrimSpace(request.RefreshToken) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid refresh token"})
		return
	}
	bundle, user, err := service.RefreshLoginSession(strings.TrimSpace(request.RefreshToken), "", c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeAuthSessionError(c, err)
		return
	}
	writeNativeAppBundle(c, bundle, user)
}

func RevokeNativeAppAuth(c *gin.Context) {
	var request nativeAppRefreshRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || strings.TrimSpace(request.RefreshToken) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid refresh token"})
		return
	}
	if err := service.RevokeByRefreshToken(strings.TrimSpace(request.RefreshToken), "", "native_app_logout"); err != nil {
		writeAuthSessionError(c, err)
		return
	}
	setAuthNoStore(c)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}
