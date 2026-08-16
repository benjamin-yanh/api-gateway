package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRootEmailTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	previousSetup := constant.Setup
	previousSelfUseMode := operation_setting.SelfUseModeEnabled
	previousDemoSite := operation_setting.DemoSiteEnabled
	previousOptionMap := common.OptionMap
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Option{}, &model.Setup{}))
	model.DB = db
	constant.Setup = false
	common.OptionMap = make(map[string]string)
	t.Cleanup(func() {
		model.DB = previousDB
		constant.Setup = previousSetup
		operation_setting.SelfUseModeEnabled = previousSelfUseMode
		operation_setting.DemoSiteEnabled = previousDemoSite
		common.OptionMap = previousOptionMap
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestPostSetupStoresRootEmailAsAccountIdentity(t *testing.T) {
	db := setupRootEmailTestDB(t)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/setup", strings.NewReader(`{
		"email":"  ROOT@Example.COM ",
		"password":"password123",
		"confirmPassword":"password123"
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	PostSetup(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)
	var root model.User
	require.NoError(t, db.Where("role = ?", common.RoleRootUser).First(&root).Error)
	assert.Equal(t, "root@example.com", root.Username)
	assert.Equal(t, "root@example.com", root.Email)
}

func TestPostSetupRejectsNonEmailRootIdentity(t *testing.T) {
	setupRootEmailTestDB(t)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/setup", strings.NewReader(`{
		"email":"root",
		"password":"password123",
		"confirmPassword":"password123"
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	PostSetup(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":false`)
	assert.False(t, model.RootUserExists())
}
