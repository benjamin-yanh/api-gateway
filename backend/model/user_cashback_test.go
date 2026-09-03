package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserListsIncludeCashbackBalanceAndHistoricalCredits(t *testing.T) {
	db, user, _ := createSettledCashback(t, 1000, 100)
	other := User{Username: "other", AffCode: "other", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&other).Error)
	// A settled but not yet credited reward must not enter historical earnings.
	users, total, err := GetAllUsers(&common.PageInfo{Page: 1, PageSize: 1}, NewUserSortOptions("id", "asc"))
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, users, 1)
	require.NotNil(t, users[0].CashbackHistoryQuota)
	assert.Zero(t, *users[0].CashbackHistoryQuota)
	assert.Zero(t, users[0].CashbackQuota)

	_, err = CreditCashbackUsage("usage")
	require.NoError(t, err)
	_, err = CreditCashbackUsage("usage")
	require.NoError(t, err)
	users, _, err = SearchUsers(user.Username, "", nil, nil, 0, 20)
	require.NoError(t, err)
	require.Len(t, users, 1)
	require.NotNil(t, users[0].CashbackHistoryQuota)
	assert.Equal(t, int64(100), *users[0].CashbackHistoryQuota)
	assert.Equal(t, 100, users[0].CashbackQuota)

	_, err = WithdrawCashbackToBalance(user.Id, 100)
	require.NoError(t, err)
	_, err = RefundCashbackUsage("usage", "refund", 1000, 1)
	require.NoError(t, err)
	users, _, err = GetAllUsers(&common.PageInfo{Page: 1, PageSize: 20}, NewUserSortOptions("id", "asc"))
	require.NoError(t, err)
	require.Len(t, users, 2)
	assert.Zero(t, users[0].CashbackQuota)
	assert.Equal(t, int64(100), *users[0].CashbackHistoryQuota)
	assert.Zero(t, users[1].CashbackQuota)
	assert.Zero(t, *users[1].CashbackHistoryQuota)
	encoded, err := common.Marshal(users[0])
	require.NoError(t, err)
	var payload map[string]interface{}
	require.NoError(t, common.Unmarshal(encoded, &payload))
	assert.Equal(t, float64(0), payload["cashback_quota"])
	assert.Equal(t, float64(100), payload["cashback_history_quota"])
}

func TestUserCashbackHistoryDoesNotSaturateLifetimeTotalsAtInt32(t *testing.T) {
	db, user, _ := useCashbackBillingDB(t, 0)
	require.NoError(t, db.Create(&[]CashbackEntry{
		{ID: "first:credit", UserID: user.Id, Kind: "credit", CashbackDelta: 2000000000},
		{ID: "second:credit", UserID: user.Id, Kind: "credit", CashbackDelta: 2000000000},
		{ID: "refund", UserID: user.Id, Kind: "refund", CashbackDelta: -100},
	}).Error)
	users, _, err := SearchUsers(user.Username, "", nil, nil, 0, 20)
	require.NoError(t, err)
	require.Len(t, users, 1)
	require.NotNil(t, users[0].CashbackHistoryQuota)
	assert.Equal(t, int64(4000000000), *users[0].CashbackHistoryQuota)
}
