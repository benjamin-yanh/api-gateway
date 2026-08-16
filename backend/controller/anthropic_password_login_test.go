package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAnthropicPasswordLoginTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousPasswordLoginEnabled := common.PasswordLoginEnabled

	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.TwoFA{}, &model.Log{}, &model.Ability{}))
	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	common.PasswordLoginEnabled = true
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.PasswordLoginEnabled = previousPasswordLoginEnabled
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func createAnthropicLoginUser(t *testing.T, db *gorm.DB, id int, email, password, accessToken string) model.User {
	t.Helper()
	hash, err := common.Password2Hash(password)
	require.NoError(t, err)
	user := model.User{
		Id:          id,
		Username:    email,
		Email:       email,
		Password:    hash,
		Status:      common.UserStatusEnabled,
		Role:        common.RoleCommonUser,
		Group:       "default",
		AffCode:     fmt.Sprintf("aff-%d", id),
		AuthVersion: 1,
	}
	user.SetAccessToken(accessToken)
	require.NoError(t, db.Create(&user).Error)
	return user
}

func TestAnthropicPasswordLoginReturnsOnlyCurrentUserCredentials(t *testing.T) {
	db := setupAnthropicPasswordLoginTestDB(t)
	user := createAnthropicLoginUser(t, db, 101, "owner@example.com", "password123", "owner-access-token")
	other := createAnthropicLoginUser(t, db, 202, "other@example.com", "password456", "other-access-token")
	require.NoError(t, db.Create(&[]model.Token{
		{UserId: user.Id, Name: "primary", Key: "owner-key", Status: common.TokenStatusEnabled},
		{UserId: user.Id, Name: "disabled", Key: "owner-disabled-key", Status: common.TokenStatusDisabled},
		{UserId: other.Id, Name: "other", Key: "other-key", Status: common.TokenStatusEnabled},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/anthropic/auth/login", strings.NewReader(`{
		"username":"owner@example.com","password":"password123"
	}`))

	AnthropicPasswordLogin(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		AccessToken string                 `json:"access_token"`
		APIKeys     []anthropicLoginAPIKey `json:"api_keys"`
		TokenType   string                 `json:"token_type"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "owner-access-token", response.AccessToken)
	assert.Equal(t, "Bearer", response.TokenType)
	require.Len(t, response.APIKeys, 1)
	assert.Equal(t, "sk-owner-key", response.APIKeys[0].APIKey)
	assert.Equal(t, common.TokenStatusEnabled, response.APIKeys[0].Status)
	assert.NotContains(t, recorder.Body.String(), "owner-disabled-key")
	assert.NotContains(t, recorder.Body.String(), "other-key")
	assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
}

func TestAnthropicPasswordLoginFiltersModelsByClientFamily(t *testing.T) {
	testCases := []struct {
		name           string
		requestPath    string
		client         string
		headers        map[string]string
		expectedModels []string
	}{
		{
			name:           "anthropic compatibility path",
			requestPath:    "/anthropic/auth/login",
			expectedModels: []string{"anthropic/claude-opus-4-1", "claude-sonnet-4-5"},
		},
		{
			name:           "Claude user agent",
			requestPath:    "/auth/login",
			headers:        map[string]string{"User-Agent": "claude-cli/2.1.0"},
			expectedModels: []string{"anthropic/claude-opus-4-1", "claude-sonnet-4-5"},
		},
		{
			name:           "Codex originator",
			requestPath:    "/auth/login",
			headers:        map[string]string{"Originator": "codex_cli_rs"},
			expectedModels: []string{"gpt-5.2-codex", "openai/o3"},
		},
		{
			name:           "ChatGPT user agent",
			requestPath:    "/auth/login",
			headers:        map[string]string{"User-Agent": "ChatGPT/1.2026.224"},
			expectedModels: []string{"gpt-5.2-codex", "openai/o3"},
		},
		{
			name:           "declared OpenAI client",
			requestPath:    "/auth/login",
			client:         "OpenAI Desktop",
			expectedModels: []string{"gpt-5.2-codex", "openai/o3"},
		},
		{
			name:        "unknown client",
			requestPath: "/auth/login",
			headers:     map[string]string{"User-Agent": "custom-client/1.0"},
			expectedModels: []string{
				"anthropic/claude-opus-4-1",
				"claude-sonnet-4-5",
				"deepseek-chat",
				"gemini-3-pro",
				"gpt-5.2-codex",
				"openai/o3",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			db := setupAnthropicPasswordLoginTestDB(t)
			createAnthropicLoginUser(t, db, 101, "owner@example.com", "password123", "owner-access-token")
			modelNames := []string{
				"claude-sonnet-4-5",
				"anthropic/claude-opus-4-1",
				"gpt-5.2-codex",
				"openai/o3",
				"gemini-3-pro",
				"deepseek-chat",
			}
			for index, modelName := range modelNames {
				require.NoError(t, db.Create(&model.Ability{
					Group:     "default",
					Model:     modelName,
					ChannelId: index + 1,
					Enabled:   true,
				}).Error)
			}

			body := fmt.Sprintf(`{"username":"owner@example.com","password":"password123","client":%q}`, testCase.client)
			request := httptest.NewRequest(http.MethodPost, testCase.requestPath, strings.NewReader(body))
			for key, value := range testCase.headers {
				request.Header.Set(key, value)
			}
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = request

			AnthropicPasswordLogin(ctx)

			require.Equal(t, http.StatusOK, recorder.Code)
			var response struct {
				Models []string `json:"models"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			assert.Equal(t, testCase.expectedModels, response.Models)
		})
	}
}

func TestAnthropicPasswordLoginRejectsInvalidPassword(t *testing.T) {
	db := setupAnthropicPasswordLoginTestDB(t)
	createAnthropicLoginUser(t, db, 101, "owner@example.com", "password123", "owner-access-token")

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/anthropic/auth/login", strings.NewReader(`{
		"username":"owner@example.com","password":"wrong-password"
	}`))

	AnthropicPasswordLogin(ctx)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "owner-access-token")
}

func TestAnthropicPasswordLoginDoesNotBypassTwoFactor(t *testing.T) {
	db := setupAnthropicPasswordLoginTestDB(t)
	user := createAnthropicLoginUser(t, db, 101, "owner@example.com", "password123", "owner-access-token")
	require.NoError(t, db.Create(&model.TwoFA{UserId: user.Id, Secret: "secret", IsEnabled: true}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/anthropic/auth/login", strings.NewReader(`{
		"username":"owner@example.com","password":"password123"
	}`))

	AnthropicPasswordLogin(ctx)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "two_factor_required")
	assert.NotContains(t, recorder.Body.String(), "owner-access-token")
}
