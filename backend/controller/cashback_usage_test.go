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

func useUsageCashbackControllerDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "cashback-controller.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.CashbackUsage{}, &model.CashbackEntry{}, &model.CashbackRefund{}))
	previousDB := model.DB
	model.DB = db
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { model.DB = previousDB; _ = sqlDB.Close() })
	return db
}

func TestUsageCashbackRecordsAreScopedAndExplainRefunds(t *testing.T) {
	db := useUsageCashbackControllerDB(t)
	rows := []model.CashbackUsage{
		{ID: "owned", RequestID: "request", UserID: 1, ModelName: "text", ActualQuota: 100, OriginalQuota: 10, CreditedQuota: 10, RecoveredQuota: 5, RefundedQuota: 50, State: model.CashbackStateSettled, Reason: "capped", Snapshot: `{"version":3,"input_per_million":"1","output_per_million":"2","max_ratio":"0.1","quota_per_cny":"secret-internal-rate"}`, ReviewReason: "private operator note"},
		{ID: "other", RequestID: "request", UserID: 2, ModelName: "other-model", ActualQuota: 200, OriginalQuota: 20, State: model.CashbackStateSettled},
	}
	require.NoError(t, db.Create(&rows).Error)
	require.NoError(t, db.Create(&model.CashbackRefund{ID: "refund", UsageID: "owned", UserID: 1, ActorID: 9, Quota: 50, RecoveredQuota: 5, CashbackDebited: 2, RefundWithheld: 3, WalletCredited: 47}).Error)
	require.NoError(t, db.Create(&model.CashbackEntry{ID: "entry", UsageID: "owned", UserID: 1, ActorID: 9, Kind: "refund", WalletDelta: 47, CashbackDelta: -2, Reason: "private operator note"}).Error)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 1)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/cashback/records?request_id=request&user_id=2", nil)
	GetMyUsageCashbackRecords(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	var page struct {
		Data struct {
			Total int                   `json:"total"`
			Items []UsageCashbackRecord `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &page))
	assert.Equal(t, 1, page.Data.Total)
	require.Len(t, page.Data.Items, 1)
	assert.Equal(t, "owned", page.Data.Items[0].ID)
	assert.Equal(t, "credited", page.Data.Items[0].Status)
	assert.True(t, page.Data.Items[0].Capped)
	assert.NotContains(t, recorder.Body.String(), "private operator note")
	assert.NotContains(t, recorder.Body.String(), "secret-internal-rate")
	assert.NotContains(t, recorder.Body.String(), "snapshot")
	require.NotNil(t, page.Data.Items[0].Rule)
	assert.Equal(t, "0.1", page.Data.Items[0].Rule.MaxRatio)

	for _, id := range []string{"owned", "other"} {
		recorder = httptest.NewRecorder()
		ctx, _ = gin.CreateTestContext(recorder)
		ctx.Set("id", 1)
		ctx.Params = gin.Params{{Key: "id", Value: id}}
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/cashback/records/"+id, nil)
		GetMyUsageCashbackRecord(ctx)
		if id == "other" {
			assert.Equal(t, http.StatusNotFound, recorder.Code)
			continue
		}
		require.Equal(t, http.StatusOK, recorder.Code)
		var detail struct {
			Data struct {
				Refunds []struct {
					Quota    int `json:"quota"`
					Withheld int `json:"refund_withheld"`
					Credited int `json:"wallet_credited"`
				} `json:"refunds"`
			} `json:"data"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &detail))
		require.Len(t, detail.Data.Refunds, 1)
		assert.Equal(t, 50, detail.Data.Refunds[0].Quota)
		assert.Equal(t, 3, detail.Data.Refunds[0].Withheld)
		assert.Equal(t, 47, detail.Data.Refunds[0].Credited)
		assert.NotContains(t, recorder.Body.String(), "private operator note")
		assert.NotContains(t, recorder.Body.String(), "actor_id")
	}
}

func TestUsageCashbackRecordStatusDoesNotPromiseUnconfirmedMoney(t *testing.T) {
	db := useUsageCashbackControllerDB(t)
	rows := []model.CashbackUsage{
		{ID: "executing", UserID: 1, State: model.CashbackStateReserved},
		{ID: "pending", UserID: 1, State: model.CashbackStateSettled, OriginalQuota: 10},
		{ID: "review", UserID: 1, State: model.CashbackStateSettled, OriginalQuota: 10, Paused: true},
		{ID: "estimated", UserID: 1, State: model.CashbackStateSettled, Reason: "estimated_usage"},
		{ID: "reversed", UserID: 1, State: model.CashbackStateSettled, OriginalQuota: 10, CancelledQuota: 10},
	}
	require.NoError(t, db.Create(&rows).Error)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 1)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/cashback/records", nil)
	GetMyUsageCashbackRecords(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	var result struct {
		Data struct {
			Items []UsageCashbackRecord `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &result))
	want := map[string]string{"executing": "processing", "pending": "pending", "review": "pending_review", "estimated": "not_eligible", "reversed": "reversed"}
	require.Len(t, result.Data.Items, len(want))
	for _, item := range result.Data.Items {
		assert.Equal(t, want[item.ID], item.Status, item.ID)
	}
}

func TestUsageCashbackRefundRejectsInvalidAmountsBeforeWriting(t *testing.T) {
	for _, body := range []string{`{`, `{"event_id":"x","quota":0}`, `{"event_id":"x","quota":-1}`, `{"event_id":"x","quota":2147483648}`, `{"event_id":"x","quota":1.5}`, `{"event_id":"","quota":1}`} {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/cashback/records/x/refund", strings.NewReader(body))
		RefundUsageCashbackRecord(ctx)
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	}
}

func TestUsageCashbackAdminDetailHandlesDatabaseFailure(t *testing.T) {
	db := useUsageCashbackControllerDB(t)
	require.NoError(t, db.Migrator().DropTable(&model.CashbackUsage{}))
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 1)
	ctx.Params = gin.Params{{Key: "id", Value: "owned"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/cashback/records/owned", nil)
	require.NotPanics(t, func() { GetAdminUsageCashbackRecord(ctx) })
	var result struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &result))
	assert.False(t, result.Success)
}
