package model

import (
	"math"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func useCashbackBillingDB(t *testing.T, quota int) (*gorm.DB, User, Token) {
	t.Helper()
	db := useCashbackTestDB(t)
	require.NoError(t, db.AutoMigrate(&Token{}, &CashbackUsage{}, &CashbackEntry{}, &CashbackRefund{}))
	previousBatch := common.BatchUpdateEnabled
	common.BatchUpdateEnabled = false
	t.Cleanup(func() { common.BatchUpdateEnabled = previousBatch })
	user := User{Username: "billing-user", Quota: quota, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&user).Error)
	token := Token{UserId: user.Id, Key: "billing-key", RemainQuota: quota}
	require.NoError(t, db.Create(&token).Error)
	return db, user, token
}

func TestCashbackBillingDurableLifecycleAndIdentity(t *testing.T) {
	db, user, token := useCashbackBillingDB(t, 1000)
	input := CashbackUsage{ID: "request-a", RequestID: "display-a", UserID: user.Id, TokenID: token.Id, ModelName: "text-model", Snapshot: `{"rate":"1"}`, ReservedQuota: 300}
	_, err := BeginCashbackUsage(input)
	require.NoError(t, err)
	_, err = BeginCashbackUsage(input)
	require.NoError(t, err)
	_, err = ReserveCashbackUsage(input.ID, 400)
	require.NoError(t, err)
	_, err = ReserveCashbackUsage(input.ID, 400)
	require.NoError(t, err)
	_, err = ReserveCashbackUsage(input.ID, 350)
	require.NoError(t, err)
	plan := CashbackSettlementPlan{ActualQuota: 250, OriginalQuota: 25, InputTokens: 100, UsageSource: "upstream"}
	_, err = PlanCashbackSettlement(input.ID, plan)
	require.NoError(t, err)
	// Recovery uses only persisted state, without the original Go session.
	count, err := RetryCashbackUsages(10)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	_, err = PlanCashbackSettlement(input.ID, plan)
	require.NoError(t, err)
	_, err = SettleCashbackUsage(input.ID)
	require.NoError(t, err)
	_, err = CreditCashbackUsage(input.ID)
	require.NoError(t, err)
	_, err = BeginCashbackUsage(input)
	require.NoError(t, err)
	require.NoError(t, db.First(&user, user.Id).Error)
	require.NoError(t, db.First(&token, token.Id).Error)
	assert.Equal(t, 750, user.Quota)
	assert.Equal(t, 25, user.CashbackQuota)
	assert.Equal(t, 750, token.RemainQuota)
	assert.Equal(t, 250, token.UsedQuota)
	input.Snapshot = `{"rate":"2"}`
	_, err = BeginCashbackUsage(input)
	assert.ErrorIs(t, err, ErrCashbackUsageConflict)
	plan.ActualQuota++
	_, err = PlanCashbackSettlement(input.ID, plan)
	assert.ErrorIs(t, err, ErrCashbackUsageConflict)
	_, err = CancelCashbackUsage(input.ID)
	assert.ErrorIs(t, err, ErrCashbackUsageConflict)
}

func TestCashbackZeroDeltaAndNoReservationStillCreateObligations(t *testing.T) {
	for _, reserved := range []int{0, 100} {
		t.Run(strconv.Itoa(reserved), func(t *testing.T) {
			db, user, _ := useCashbackBillingDB(t, 200)
			_, err := BeginCashbackUsage(CashbackUsage{ID: "zero", UserID: user.Id, ModelName: "text", Snapshot: "{}", ReservedQuota: reserved})
			require.NoError(t, err)
			_, err = PlanCashbackSettlement("zero", CashbackSettlementPlan{ActualQuota: 100, OriginalQuota: 10})
			require.NoError(t, err)
			_, err = RetryCashbackUsages(10)
			require.NoError(t, err)
			require.NoError(t, db.First(&user, user.Id).Error)
			assert.Equal(t, 100, user.Quota)
			assert.Equal(t, 10, user.CashbackQuota)
		})
	}
}

func TestCashbackSettlementRollbackAndRecovery(t *testing.T) {
	db, user, token := useCashbackBillingDB(t, 1000)
	_, err := BeginCashbackUsage(CashbackUsage{ID: "recover", UserID: user.Id, TokenID: token.Id, ModelName: "text", Snapshot: "{}", ReservedQuota: 200})
	require.NoError(t, err)
	_, err = PlanCashbackSettlement("recover", CashbackSettlementPlan{ActualQuota: 300, OriginalQuota: 30})
	require.NoError(t, err)
	require.NoError(t, db.Migrator().DropTable(&CashbackEntry{}))
	_, err = SettleCashbackUsage("recover")
	require.Error(t, err)
	require.NoError(t, db.First(&user, user.Id).Error)
	require.NoError(t, db.First(&token, token.Id).Error)
	assert.Equal(t, 800, user.Quota)
	assert.Equal(t, 800, token.RemainQuota)
	row, err := GetCashbackUsage("recover", user.Id)
	require.NoError(t, err)
	assert.Equal(t, CashbackStatePlanned, row.State)
	require.NoError(t, db.AutoMigrate(&CashbackEntry{}))
	_, err = RetryCashbackUsages(10)
	require.NoError(t, err)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 700, user.Quota)
	assert.Equal(t, 30, user.CashbackQuota)
}

func TestCashbackPreconsumeRollbackAndIdempotentCancel(t *testing.T) {
	db, user, token := useCashbackBillingDB(t, 1000)
	require.NoError(t, db.Model(&token).Update("remain_quota", 50).Error)
	input := CashbackUsage{ID: "cancel", UserID: user.Id, TokenID: token.Id, ModelName: "text", Snapshot: "{}", ReservedQuota: 100}
	_, err := BeginCashbackUsage(input)
	assert.ErrorIs(t, err, ErrInsufficientQuota)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 1000, user.Quota)
	var count int64
	require.NoError(t, db.Model(&CashbackUsage{}).Count(&count).Error)
	assert.Zero(t, count)
	require.NoError(t, db.Model(&token).Update("remain_quota", 1000).Error)
	_, err = BeginCashbackUsage(input)
	require.NoError(t, err)
	_, err = CancelCashbackUsage(input.ID)
	require.NoError(t, err)
	_, err = CancelCashbackUsage(input.ID)
	require.NoError(t, err)
	_, err = PlanCashbackSettlement(input.ID, CashbackSettlementPlan{ActualQuota: 100, OriginalQuota: 10})
	assert.ErrorIs(t, err, ErrCashbackUsageConflict)
	require.NoError(t, db.First(&user, user.Id).Error)
	require.NoError(t, db.First(&token, token.Id).Error)
	assert.Equal(t, 1000, user.Quota)
	assert.Equal(t, 1000, token.RemainQuota)
	assert.Zero(t, token.UsedQuota)
}

func TestCashbackBatchGateDoesNotCancelExistingObligations(t *testing.T) {
	db, user, _ := useCashbackBillingDB(t, 1000)
	input := CashbackUsage{ID: "old", UserID: user.Id, ModelName: "text", Snapshot: "{}", ReservedQuota: 100}
	_, err := BeginCashbackUsage(input)
	require.NoError(t, err)
	_, err = PlanCashbackSettlement("old", CashbackSettlementPlan{ActualQuota: 100, OriginalQuota: 10})
	require.NoError(t, err)
	common.BatchUpdateEnabled = true
	input.ID = "new"
	_, err = BeginCashbackUsage(input)
	assert.ErrorIs(t, err, ErrCashbackBatchEnabled)
	_, err = RetryCashbackUsages(10)
	require.NoError(t, err)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 900, user.Quota)
	assert.Equal(t, 10, user.CashbackQuota)
}

func TestCashbackUnknownReservationIsNotAutomaticallyRefunded(t *testing.T) {
	db, user, _ := useCashbackBillingDB(t, 1000)
	_, err := BeginCashbackUsage(CashbackUsage{ID: "unknown", UserID: user.Id, ModelName: "text", Snapshot: "{}", ReservedQuota: 100})
	require.NoError(t, err)
	_, err = RetryCashbackUsages(10)
	require.NoError(t, err)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 900, user.Quota)
	assert.Zero(t, user.CashbackQuota)
	row, err := GetCashbackUsage("unknown", user.Id)
	require.NoError(t, err)
	assert.Equal(t, CashbackStateReserved, row.State)
	_, err = GetCashbackUsage("unknown", user.Id+1)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestCashbackPlanRejectsUnsafeAmounts(t *testing.T) {
	_, user, _ := useCashbackBillingDB(t, 1000)
	_, err := BeginCashbackUsage(CashbackUsage{ID: "bounds", UserID: user.Id, ModelName: "text", Snapshot: "{}"})
	require.NoError(t, err)
	for _, plan := range []CashbackSettlementPlan{
		{ActualQuota: -1}, {ActualQuota: math.MaxInt32 + 1},
		{ActualQuota: 10, OriginalQuota: -1}, {ActualQuota: 10, OriginalQuota: 10},
		{ActualQuota: 10, OriginalQuota: 11}, {InputTokens: -1},
	} {
		_, err := PlanCashbackSettlement("bounds", plan)
		assert.ErrorIs(t, err, ErrInvalidCashbackUsage)
	}
}

func TestCashbackFailedSettlementDoesNotStarveOtherObligations(t *testing.T) {
	db, user, _ := useCashbackBillingDB(t, 100)
	for _, id := range []string{"a-insufficient", "b-ready"} {
		_, err := BeginCashbackUsage(CashbackUsage{ID: id, UserID: user.Id, ModelName: "text", Snapshot: "{}"})
		require.NoError(t, err)
		charge := 200
		if id == "b-ready" {
			charge = 10
		}
		_, err = PlanCashbackSettlement(id, CashbackSettlementPlan{ActualQuota: charge, OriginalQuota: 1})
		require.NoError(t, err)
	}
	_, err := RetryCashbackUsages(1)
	assert.ErrorIs(t, err, ErrInsufficientQuota)
	_, err = RetryCashbackUsages(1)
	require.NoError(t, err)
	row, err := GetCashbackUsage("b-ready", user.Id)
	require.NoError(t, err)
	assert.Equal(t, CashbackStateSettled, row.State)
	assert.Equal(t, 1, row.CreditedQuota)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 90, user.Quota)
	assert.Equal(t, 1, user.CashbackQuota)
}

func TestCashbackAdmissionRejectsEmptyWalletWithZeroReservation(t *testing.T) {
	db, user, _ := useCashbackBillingDB(t, 0)
	_, err := BeginCashbackUsage(CashbackUsage{ID: "empty", UserID: user.Id, ModelName: "text", Snapshot: "{}", ReservedQuota: 0})
	assert.ErrorIs(t, err, ErrInsufficientQuota)
	var count int64
	require.NoError(t, db.Model(&CashbackUsage{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestCashbackAdmissionStatusDoesNotPreventExistingRefund(t *testing.T) {
	db, user, _ := useCashbackBillingDB(t, 100)
	input := CashbackUsage{ID: "accepted", UserID: user.Id, ModelName: "text", Snapshot: "{}", ReservedQuota: 100}
	_, err := BeginCashbackUsage(input)
	require.NoError(t, err)
	require.NoError(t, db.Model(&User{}).Where("id = ?", user.Id).Update("status", common.UserStatusDisabled).Error)
	_, err = BeginCashbackUsage(input)
	require.NoError(t, err)
	_, err = CancelCashbackUsage(input.ID)
	require.NoError(t, err)
	input.ID = "disabled"
	input.ReservedQuota = 0
	_, err = BeginCashbackUsage(input)
	assert.ErrorIs(t, err, ErrCashbackUserUnavailable)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 100, user.Quota)
}
