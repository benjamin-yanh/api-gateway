package model

import (
	"math"
	"path/filepath"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func useCashbackTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "cashback.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &CashbackWithdrawal{}))
	previousDB, previousType, previousRedis := DB, common.MainDatabaseType(), common.RedisEnabled
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled = previousRedis
		_ = sqlDB.Close()
	})
	return db
}

func TestCashbackWithdrawalCreditsOnce(t *testing.T) {
	db := useCashbackTestDB(t)
	user := User{Username: "cashback-user", Quota: 100, CashbackQuota: 50, AffQuota: 20, UsedQuota: 30}
	require.NoError(t, db.Create(&user).Error)
	receipt, err := WithdrawCashbackToBalance(user.Id, 50)
	require.NoError(t, err)
	assert.Equal(t, user.Id, receipt.UserId)
	assert.Equal(t, 50, receipt.Quota)
	_, err = WithdrawCashbackToBalance(user.Id, 50)
	assert.ErrorIs(t, err, ErrCashbackBalanceChanged)
	var updated User
	require.NoError(t, db.First(&updated, user.Id).Error)
	assert.Equal(t, 150, updated.Quota)
	assert.Zero(t, updated.CashbackQuota)
	assert.Equal(t, 20, updated.AffQuota)
	assert.Equal(t, 30, updated.UsedQuota)
	var receipts []CashbackWithdrawal
	require.NoError(t, db.Find(&receipts).Error)
	require.Len(t, receipts, 1)
	assert.Equal(t, *receipt, receipts[0])
}

func TestUserProfileUpdateCannotRestoreWithdrawnCashback(t *testing.T) {
	db := useCashbackTestDB(t)
	user := User{Username: "cashback-user", Quota: 100, CashbackQuota: 50}
	require.NoError(t, db.Create(&user).Error)
	_, err := WithdrawCashbackToBalance(user.Id, 50)
	require.NoError(t, err)
	// Simulate a profile read before the withdrawal and saved afterwards.
	user.DisplayName = "Updated name"
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return user.UpdateWithTx(tx, false)
	}))
	var updated User
	require.NoError(t, db.First(&updated, user.Id).Error)
	assert.Equal(t, "Updated name", updated.DisplayName)
	assert.Equal(t, 150, updated.Quota)
	assert.Zero(t, updated.CashbackQuota)
}

func TestReferralRewardPreservesWithdrawnCashbackAndBalance(t *testing.T) {
	db := useCashbackTestDB(t)
	previousReward := common.QuotaForInviter
	common.QuotaForInviter = 10
	t.Cleanup(func() { common.QuotaForInviter = previousReward })
	user := User{Username: "cashback-user", Quota: 100, CashbackQuota: 50, AffQuota: 20, AffHistoryQuota: 30}
	require.NoError(t, db.Create(&user).Error)
	_, err := WithdrawCashbackToBalance(user.Id, 50)
	require.NoError(t, err)
	require.NoError(t, inviteUser(user.Id))
	var updated User
	require.NoError(t, db.First(&updated, user.Id).Error)
	assert.Equal(t, 150, updated.Quota)
	assert.Zero(t, updated.CashbackQuota)
	assert.Equal(t, 1, updated.AffCount)
	assert.Equal(t, 30, updated.AffQuota)
	assert.Equal(t, 40, updated.AffHistoryQuota)
}

func TestCashbackWithdrawalRejectsInvalidOrStaleAmounts(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		balance, cashback, requested int
		want                         error
	}{
		{"zero", 100, 50, 0, ErrInvalidCashbackWithdrawal},
		{"negative", 100, 50, -50, ErrInvalidCashbackWithdrawal},
		{"overflow input", 100, 50, math.MaxInt32 + 1, ErrInvalidCashbackWithdrawal},
		{"empty", 100, 0, 50, ErrCashbackBalanceChanged},
		{"stale balance", 100, 60, 50, ErrCashbackBalanceChanged},
		{"insufficient", 100, 50, 60, ErrCashbackBalanceChanged},
		{"overflow balance", math.MaxInt32 - 49, 50, 50, ErrCashbackBalanceChanged},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := useCashbackTestDB(t)
			user := User{Username: "cashback-user", Quota: tc.balance, CashbackQuota: tc.cashback}
			require.NoError(t, db.Create(&user).Error)
			_, err := WithdrawCashbackToBalance(user.Id, tc.requested)
			assert.ErrorIs(t, err, tc.want)
			var updated User
			require.NoError(t, db.First(&updated, user.Id).Error)
			assert.Equal(t, tc.balance, updated.Quota)
			assert.Equal(t, tc.cashback, updated.CashbackQuota)
			var count int64
			require.NoError(t, db.Model(&CashbackWithdrawal{}).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}

func TestCashbackWithdrawalRollsBackWhenReceiptCannotBeWritten(t *testing.T) {
	db := useCashbackTestDB(t)
	user := User{Username: "cashback-user", Quota: 100, CashbackQuota: 50}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Migrator().DropTable(&CashbackWithdrawal{}))
	_, err := WithdrawCashbackToBalance(user.Id, 50)
	require.Error(t, err)
	var updated User
	require.NoError(t, db.First(&updated, user.Id).Error)
	assert.Equal(t, 100, updated.Quota)
	assert.Equal(t, 50, updated.CashbackQuota)
}

func TestCashbackWithdrawalConcurrentConfirmationsCreditOnce(t *testing.T) {
	db := useCashbackTestDB(t)
	user := User{Username: "cashback-user", Quota: math.MaxInt32 - 50, CashbackQuota: 50}
	require.NoError(t, db.Create(&user).Error)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := WithdrawCashbackToBalance(user.Id, 50)
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	assert.Equal(t, 1, successes)
	var updated User
	require.NoError(t, db.First(&updated, user.Id).Error)
	assert.Equal(t, math.MaxInt32, updated.Quota)
	assert.Zero(t, updated.CashbackQuota)
	var count int64
	require.NoError(t, db.Model(&CashbackWithdrawal{}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}
