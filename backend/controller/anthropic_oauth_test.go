package controller

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnthropicOAuthAuthorizeRedirectsToConsentPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/anthropic/oauth/authorize", AnthropicOAuthAuthorize)

	query := url.Values{
		"response_type":         {"code"},
		"client_id":             {"22422756-60c9-4084-8eb7-27705fd5cf9a"},
		"redirect_uri":          {"http://127.0.0.1:49152/callback"},
		"code_challenge":        {strings.Repeat("a", 43)},
		"code_challenge_method": {"S256"},
		"state":                 {strings.Repeat("s", 32)},
	}
	request := httptest.NewRequest(http.MethodGet, "/anthropic/oauth/authorize?"+query.Encode(), nil)
	response := httptest.NewRecorder()

	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusFound, response.Code)
	location, err := url.Parse(response.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "/native-app", location.Path)
	assert.Equal(t, query.Get("client_id"), location.Query().Get("client_id"))
	assert.Equal(t, query.Get("redirect_uri"), location.Query().Get("redirect_uri"))
	assert.Equal(t, query.Get("state"), location.Query().Get("state"))
}

func TestAnthropicOAuthAuthorizeRejectsRemoteCallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest(http.MethodGet, "/anthropic/oauth/authorize?response_type=code&client_id=desktop&redirect_uri=https%3A%2F%2Fexample.com%2Fcallback&code_challenge="+strings.Repeat("a", 43)+"&code_challenge_method=S256&state="+strings.Repeat("s", 32), nil)
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = request

	AnthropicOAuthAuthorize(context)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Contains(t, response.Body.String(), `"error":"invalid_request"`)
}

func TestAnthropicOAuthTokenRejectsUnsupportedGrant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest(http.MethodPost, "/anthropic/v1/oauth/token", strings.NewReader(`{"grant_type":"client_credentials"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = request

	AnthropicOAuthToken(context)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Contains(t, response.Body.String(), `"error":"unsupported_grant_type"`)
}
