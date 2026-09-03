package model

import (
	"math"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func createSettledCashback(t *testing.T, charge, reward int) (*gorm.DB, User, Token) {
	t.Helper()
	db, user, token := useCashbackBillingDB(t, 2000)
	_, err := BeginCashbackUsage(CashbackUsage{ID: "usage", UserID: user.Id, TokenID: token.Id, ModelName: "text", Snapshot: "{}", ReservedQuota: charge})
	require.NoError(t, err)
	_, err = PlanCashbackSettlement("usage", CashbackSettlementPlan{ActualQuota: charge, OriginalQuota: reward})
	require.NoError(t, err)
	_, err = SettleCashbackUsage("usage")
	require.NoError(t, err)
	return db, user, token
}

func TestCashbackRefundRecoversTransferredOrRemainingReward(t *testing.T) {
	for _, tc := range []struct {
		name                                                   string
		remaining, wantCashbackDebit, wantWithheld, wantCredit int
	}{
		{"available", 100, 100, 0, 1000},
		{"partly available", 40, 40, 60, 940},
		{"transferred", 0, 0, 100, 900},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, user, token := createSettledCashback(t, 1000, 100)
			_, err := CreditCashbackUsage("usage")
			require.NoError(t, err)
			if tc.remaining < 100 {
				_, err = WithdrawCashbackToBalance(user.Id, 100)
				require.NoError(t, err)
				// Model a separate subsequent reward when only part is available.
				require.NoError(t, db.Model(&User{}).Where("id = ?", user.Id).Update("cashback_quota", tc.remaining).Error)
			}
			receipt, err := RefundCashbackUsage("usage", "refund", 1000, 42)
			require.NoError(t, err)
			assert.Equal(t, 42, receipt.ActorID)
			assert.Equal(t, tc.wantCashbackDebit, receipt.CashbackDebited)
			assert.Equal(t, tc.wantWithheld, receipt.RefundWithheld)
			assert.Equal(t, tc.wantCredit, receipt.WalletCredited)
			repeated, err := RefundCashbackUsage("usage", "refund", 1000, 42)
			require.NoError(t, err)
			assert.Equal(t, *receipt, *repeated)
			require.NoError(t, db.First(&token, token.Id).Error)
			assert.Equal(t, 2000, token.RemainQuota)
			assert.Zero(t, token.UsedQuota)
			row, err := GetCashbackUsage("usage", user.Id)
			require.NoError(t, err)
			assert.Equal(t, 100, row.OriginalQuota)
			assert.Equal(t, 100, row.RecoveredQuota)
			assert.Equal(t, 1000, row.RefundedQuota)
			_, err = RefundCashbackUsage("usage", "refund", 900)
			assert.ErrorIs(t, err, ErrCashbackUsageConflict)
			_, err = RefundCashbackUsage("usage", "too-much", 1)
			assert.ErrorIs(t, err, ErrInvalidCashbackUsage)
		})
	}
}

func TestCashbackPartialRefundBeforeAndAfterCreditUsesOriginalEntitlement(t *testing.T) {
	db, user, token := createSettledCashback(t, 1000, 30)
	first, err := RefundCashbackUsage("usage", "first", 200)
	require.NoError(t, err)
	assert.Equal(t, 6, first.CancelledQuota)
	assert.Zero(t, first.RecoveredQuota)
	assert.Equal(t, 200, first.WalletCredited)
	row, err := CreditCashbackUsage("usage")
	require.NoError(t, err)
	assert.Equal(t, 24, row.CreditedQuota)
	_, err = WithdrawCashbackToBalance(user.Id, 24)
	require.NoError(t, err)
	last, err := RefundCashbackUsage("usage", "last", 800)
	require.NoError(t, err)
	assert.Zero(t, last.CancelledQuota)
	assert.Equal(t, 24, last.RecoveredQuota)
	assert.Equal(t, 24, last.RefundWithheld)
	_, err = CreditCashbackUsage("usage")
	require.NoError(t, err)
	require.NoError(t, db.First(&user, user.Id).Error)
	require.NoError(t, db.First(&token, token.Id).Error)
	assert.Equal(t, 2000, user.Quota)
	assert.Zero(t, user.CashbackQuota)
	assert.Equal(t, 2000, token.RemainQuota)
	row, err = GetCashbackUsage("usage", user.Id)
	require.NoError(t, err)
	assert.Equal(t, 30, row.OriginalQuota)
	assert.Equal(t, 6, row.CancelledQuota)
	assert.Equal(t, 24, row.RecoveredQuota)
}

func TestCashbackPartialRefundRoundingIsCumulative(t *testing.T) {
	_, _, _ = createSettledCashback(t, 7, 3)
	for i, tc := range []struct{ quota, revoke int }{{1, 0}, {2, 1}, {1, 0}, {3, 2}} {
		id := []string{"a", "b", "c", "d"}[i]
		receipt, err := RefundCashbackUsage("usage", id, tc.quota)
		require.NoError(t, err)
		assert.Equal(t, tc.revoke, receipt.CancelledQuota)
		assert.Zero(t, receipt.RecoveredQuota)
	}
	row, err := CreditCashbackUsage("usage")
	require.NoError(t, err)
	assert.Equal(t, 3, row.CancelledQuota)
	assert.Zero(t, row.CreditedQuota)
}

func TestCashbackRefundReceiptFailureRollsBackBalancesAndCumulativeRefund(t *testing.T) {
	db, user, token := createSettledCashback(t, 1000, 100)
	_, err := CreditCashbackUsage("usage")
	require.NoError(t, err)
	// Reject the receipt after the transactional balance changes, not before
	// the refund's initial receipt lookup.
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("reject-cashback-refund", func(tx *gorm.DB) {
		if tx.Statement.Table == "cashback_refunds" {
			tx.AddError(ErrCashbackUsageConflict)
		}
	}))
	_, err = RefundCashbackUsage("usage", "rollback", 500)
	require.Error(t, err)
	require.NoError(t, db.First(&user, user.Id).Error)
	require.NoError(t, db.First(&token, token.Id).Error)
	assert.Equal(t, 1000, user.Quota)
	assert.Equal(t, 100, user.CashbackQuota)
	assert.Equal(t, 1000, token.RemainQuota)
	row, err := GetCashbackUsage("usage", user.Id)
	require.NoError(t, err)
	assert.Zero(t, row.RefundedQuota)
	assert.Zero(t, row.RecoveredQuota)
	require.NoError(t, db.Callback().Create().Remove("reject-cashback-refund"))
	_, err = RefundCashbackUsage("usage", "rollback", 500)
	require.NoError(t, err)
}

func TestCashbackCreditCapacityAndPauseRemainRecoverable(t *testing.T) {
	db, user, _ := createSettledCashback(t, 1000, 100)
	require.NoError(t, db.Model(&User{}).Where("id = ?", user.Id).Update("cashback_quota", math.MaxInt32).Error)
	_, err := CreditCashbackUsage("usage")
	assert.ErrorIs(t, err, ErrCashbackBalanceChanged)
	row, err := GetCashbackUsage("usage", user.Id)
	require.NoError(t, err)
	assert.Zero(t, row.CreditedQuota)
	require.NoError(t, PauseCashbackUsage("usage", "check usage", 42))
	require.NoError(t, db.Model(&User{}).Where("id = ?", user.Id).Update("cashback_quota", 0).Error)
	_, err = RetryCashbackUsages(10)
	require.NoError(t, err)
	_, err = CreditCashbackUsage("usage")
	assert.ErrorIs(t, err, ErrCashbackUsagePaused)
	require.NoError(t, ResumeCashbackUsage("usage", "usage confirmed", 42))
	_, err = RetryCashbackUsages(10)
	require.NoError(t, err)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 100, user.CashbackQuota)
	var entries []CashbackEntry
	require.NoError(t, db.Where("kind IN ?", []string{"pause", "resume"}).Find(&entries).Error)
	require.Len(t, entries, 2)
	assert.Equal(t, 42, entries[0].ActorID)
	assert.Equal(t, 42, entries[1].ActorID)
}

func TestCashbackConcurrentPartialRefundsDoNotLoseCumulativeAmount(t *testing.T) {
	db, user, _ := createSettledCashback(t, 1000, 100)
	// SQLite serializes writers; use its single-connection deployment mode so
	// the assertion checks accounting rather than driver busy-timeout policy.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, event := range []string{"concurrent-a", "concurrent-b"} {
		wg.Add(1)
		go func(event string) {
			defer wg.Done()
			<-start
			_, err := RefundCashbackUsage("usage", event, 500)
			results <- err
		}(event)
	}
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	row, err := GetCashbackUsage("usage", user.Id)
	require.NoError(t, err)
	assert.Equal(t, 1000, row.RefundedQuota)
	assert.Equal(t, 100, row.CancelledQuota)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 2000, user.Quota)
}

func TestCashbackReviewPlanStillSettlesMoney(t *testing.T) {
	db, user, _ := useCashbackBillingDB(t, 2000)
	_, err := BeginCashbackUsage(CashbackUsage{ID: "review", UserID: user.Id, ModelName: "text", Snapshot: "{}", ReservedQuota: 100})
	require.NoError(t, err)
	_, err = PlanCashbackSettlement("review", CashbackSettlementPlan{ActualQuota: 200, Reason: "invalid_usage"})
	require.NoError(t, err)
	_, err = RetryCashbackUsages(10)
	require.NoError(t, err)
	row, err := GetCashbackUsage("review", user.Id)
	require.NoError(t, err)
	assert.Equal(t, CashbackStateSettled, row.State)
	assert.True(t, row.Paused)
	assert.Equal(t, "invalid_usage", row.ReviewReason)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 1800, user.Quota)
	assert.Zero(t, user.CashbackQuota)
}

func TestCashbackCacheFailureDoesNotRepeatCredit(t *testing.T) {
	db, user, _ := createSettledCashback(t, 1000, 100)
	server := useUserCacheMiniRedis(t)
	server.SetError("ERR simulated cache outage")
	row, err := CreditCashbackUsage("usage")
	require.NoError(t, err)
	assert.True(t, row.CachePending)
	row, err = CreditCashbackUsage("usage")
	require.NoError(t, err)
	assert.Equal(t, 100, row.CreditedQuota)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 100, user.CashbackQuota)
	server.SetError("")
	_, err = RetryCashbackUsages(10)
	require.NoError(t, err)
	row, err = GetCashbackUsage("usage", user.Id)
	require.NoError(t, err)
	assert.False(t, row.CachePending)
	var count int64
	require.NoError(t, db.Model(&CashbackEntry{}).Where("usage_id = ? AND kind = ?", "usage", "credit").Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestCashbackRefundConcurrentWithTransferPreservesNetBalance(t *testing.T) {
	db, user, _ := createSettledCashback(t, 1000, 100)
	_, err := CreditCashbackUsage("usage")
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	start := make(chan struct{})
	var transferErr, refundErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, transferErr = WithdrawCashbackToBalance(user.Id, 100)
	}()
	go func() {
		defer wg.Done()
		<-start
		_, refundErr = RefundCashbackUsage("usage", "concurrent-refund", 1000)
	}()
	close(start)
	wg.Wait()
	require.NoError(t, refundErr)
	if transferErr != nil {
		assert.ErrorIs(t, transferErr, ErrCashbackBalanceChanged)
	}
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 2000, user.Quota)
	assert.Zero(t, user.CashbackQuota)
}
