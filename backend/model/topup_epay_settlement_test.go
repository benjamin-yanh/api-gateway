package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertEpaySettlementOrder(t *testing.T, tradeNo string, userID int, money float64) {
	t.Helper()
	require.NoError(t, (&TopUp{
		UserId:          userID,
		Amount:          10,
		Money:           money,
		TradeNo:         tradeNo,
		PaymentMethod:   "alipay",
		PaymentProvider: PaymentProviderEpay,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}).Insert())
}

func TestSettleEpayTopUpRejectsUnderpayment(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 801, 0)
	insertEpaySettlementOrder(t, "epay-underpaid", 801, 9.99)

	_, _, err := SettleEpayTopUp("epay-underpaid", "provider-underpaid", "alipay", "0.01")
	require.ErrorIs(t, err, ErrTopUpAmountMismatch)
	assert.Equal(t, common.TopUpStatusPending, getTopUpStatusForPaymentGuardTest(t, "epay-underpaid"))
	assert.Zero(t, getUserQuotaForPaymentGuardTest(t, 801))
}

func TestSettleEpayTopUpIsIdempotent(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 802, 0)
	insertEpaySettlementOrder(t, "epay-idempotent", 802, 9.99)

	_, firstQuota, err := SettleEpayTopUp("epay-idempotent", "provider-once", "alipay", "9.99")
	require.NoError(t, err)
	require.Positive(t, firstQuota)
	_, replayQuota, err := SettleEpayTopUp("epay-idempotent", "provider-once", "alipay", "9.99")
	require.NoError(t, err)
	assert.Zero(t, replayQuota)
	assert.Equal(t, firstQuota, getUserQuotaForPaymentGuardTest(t, 802))
}

func TestSettleEpayTopUpRollsBackStatusWhenUserCreditFails(t *testing.T) {
	truncateTables(t)
	insertEpaySettlementOrder(t, "epay-missing-user", 99999, 9.99)

	_, _, err := SettleEpayTopUp("epay-missing-user", "provider-missing-user", "alipay", "9.99")
	require.Error(t, err)
	assert.Equal(t, common.TopUpStatusPending, getTopUpStatusForPaymentGuardTest(t, "epay-missing-user"))
}
