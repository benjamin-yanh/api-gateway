package controller

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
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

func TestWithdrawCashbackRejectsUnavailableMethodsAndInvalidAmounts(t *testing.T) {
	for _, body := range []string{
		`{"method":"bank_card","quota":50}`, `{"method":"alipay","quota":50}`,
		`{"method":"wechat","quota":50}`, `{"method":"usdt","quota":50}`,
		`{"method":"unknown","quota":50}`, `{"method":"balance","quota":0}`,
		`{"method":"balance","quota":-1}`, `{"method":"balance","quota":2147483648}`,
		`{"method":"balance","quota":18446744073686646784}`,
		`{"method":"balance","quota":1.5}`, `{`,
	} {
		t.Run(body, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Set("id", 1)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/cashback/withdraw", strings.NewReader(body))
			WithdrawCashback(ctx)
			assert.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}

func TestWithdrawCashbackUsesAuthenticatedUserAndRejectsReplay(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "cashback.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.CashbackWithdrawal{}))
	previousDB, previousRedis := model.DB, common.RedisEnabled
	model.DB, common.RedisEnabled = db, false
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { model.DB, common.RedisEnabled = previousDB, previousRedis; _ = sqlDB.Close() })
	owner := model.User{Username: "owner", AffCode: "owner", Quota: 100, CashbackQuota: 50}
	other := model.User{Username: "other", AffCode: "other", Quota: 200, CashbackQuota: 50}
	require.NoError(t, db.Create(&owner).Error)
	require.NoError(t, db.Create(&other).Error)
	for _, status := range []int{http.StatusOK, http.StatusConflict} {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Set("id", owner.Id)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/cashback/withdraw", strings.NewReader(`{"method":"balance","quota":50,"user_id":2}`))
		WithdrawCashback(ctx)
		assert.Equal(t, status, recorder.Code)
	}
	require.NoError(t, db.First(&owner, owner.Id).Error)
	require.NoError(t, db.First(&other, other.Id).Error)
	assert.Equal(t, 150, owner.Quota)
	assert.Zero(t, owner.CashbackQuota)
	assert.Equal(t, 200, other.Quota)
	assert.Equal(t, 50, other.CashbackQuota)
	assert.Equal(t, 50, buildSelfUserData(&other)["cashback_quota"])
}
