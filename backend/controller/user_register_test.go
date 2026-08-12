package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	appI18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRegistrationTest(t *testing.T) *gorm.DB {
	t.Helper()
	require.NoError(t, appI18n.Init())
	previousDB := model.DB
	previousRegisterEnabled := common.RegisterEnabled
	previousPasswordRegisterEnabled := common.PasswordRegisterEnabled
	previousEmailVerificationEnabled := common.EmailVerificationEnabled
	previousRedisEnabled := common.RedisEnabled
	previousQuotaForNewUser := common.QuotaForNewUser
	previousGenerateDefaultToken := constant.GenerateDefaultToken

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	model.DB = db
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	common.RedisEnabled = false
	common.QuotaForNewUser = 0
	constant.GenerateDefaultToken = false

	t.Cleanup(func() {
		model.DB = previousDB
		common.RegisterEnabled = previousRegisterEnabled
		common.PasswordRegisterEnabled = previousPasswordRegisterEnabled
		common.EmailVerificationEnabled = previousEmailVerificationEnabled
		common.RedisEnabled = previousRedisEnabled
		common.QuotaForNewUser = previousQuotaForNewUser
		constant.GenerateDefaultToken = previousGenerateDefaultToken
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func performRegistrationRequest(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Request.Header.Set("Accept-Language", "en")
	Register(context)
	return recorder
}

func TestRegisterRejectsInvalidEmailWithoutExposingValidatorInternals(t *testing.T) {
	setupRegistrationTest(t)

	recorder := performRegistrationRequest(t, `{"username":"not-an-email","password":"password123"}`)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Please enter a valid email address")
	assert.NotContains(t, recorder.Body.String(), "Field validation")
	assert.NotContains(t, recorder.Body.String(), "Invalid input Key")
}

func TestRegisterRejectsUsernameLongerThan128WithLengthHint(t *testing.T) {
	setupRegistrationTest(t)
	longEmail := strings.Repeat("a", 117) + "@example.com"

	recorder := performRegistrationRequest(t, fmt.Sprintf(`{"username":%q,"password":"password123"}`, longEmail))

	assert.Contains(t, recorder.Body.String(), "cannot exceed 128 characters")
	assert.NotContains(t, recorder.Body.String(), "max tag")
}

func TestRegisterStoresNormalizedEmailAsUsernameAndEmail(t *testing.T) {
	db := setupRegistrationTest(t)

	recorder := performRegistrationRequest(t, `{"username":"  User@Example.COM ","password":"password123"}`)

	assert.Contains(t, recorder.Body.String(), `"success":true`)
	var user model.User
	require.NoError(t, db.Where("username = ?", "user@example.com").First(&user).Error)
	assert.Equal(t, "user@example.com", user.Username)
	assert.Equal(t, "user@example.com", user.Email)
}
