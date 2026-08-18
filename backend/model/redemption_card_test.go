package model

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func useRedemptionCardTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &Redemption{}, &RedemptionCard{}))

	previousDB := DB
	previousQuotaPerUnit := common.QuotaPerUnit
	previousExchangeRate := operation_setting.USDExchangeRate
	DB = db
	common.QuotaPerUnit = 7_300
	operation_setting.USDExchangeRate = 7.3
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	initCol()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(5)
	t.Cleanup(func() {
		DB = previousDB
		common.QuotaPerUnit = previousQuotaPerUnit
		operation_setting.USDExchangeRate = previousExchangeRate
		_ = sqlDB.Close()
	})
	return db
}

func TestRedeemCardCreditsOnce(t *testing.T) {
	db := useRedemptionCardTestDB(t)

	user := User{Username: "card-user", Password: "password", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&user).Error)
	card := RedemptionCard{
		Key:         "23456789ABCDEFGHJKLMNPQR",
		Group:       RedemptionCardGroup10RMB,
		Quota:       10_000,
		Status:      common.RedemptionCodeStatusEnabled,
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, db.Create(&card).Error)

	quota, err := Redeem(card.Key, user.Id)
	require.NoError(t, err)
	assert.Equal(t, card.Quota, quota)

	var updated User
	require.NoError(t, db.First(&updated, user.Id).Error)
	assert.Equal(t, card.Quota, updated.Quota)

	_, err = Redeem(card.Key, user.Id)
	assert.ErrorIs(t, err, ErrRedeemFailed)
	require.NoError(t, db.First(&updated, user.Id).Error)
	assert.Equal(t, card.Quota, updated.Quota)

	history, err := GetRecentRedemptionCardHistory(user.Id, 10)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, RedemptionCardGroup10RMB, history[0].Group)
	assert.Equal(t, 10, history[0].AmountRMB)
	assert.Equal(t, card.Quota, history[0].Quota)
}

func TestRedeemCardRejectsUnknownGroupWithoutCreditingUser(t *testing.T) {
	db := useRedemptionCardTestDB(t)

	user := User{Username: "invalid-card-user", Password: "password", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&user).Error)
	card := RedemptionCard{
		Key:         "3456789ABCDEFGHJKLMNPQRS",
		Group:       "UNKNOWN_CARD",
		Quota:       10_000,
		Status:      common.RedemptionCodeStatusEnabled,
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, db.Create(&card).Error)

	_, err := Redeem(card.Key, user.Id)
	assert.ErrorIs(t, err, ErrRedeemFailed)

	var updatedUser User
	require.NoError(t, db.First(&updatedUser, user.Id).Error)
	assert.Zero(t, updatedUser.Quota)
	var updatedCard RedemptionCard
	require.NoError(t, db.First(&updatedCard, card.Id).Error)
	assert.Equal(t, common.RedemptionCodeStatusEnabled, updatedCard.Status)
}

func TestRedeemCardConcurrentRequestsCreditOnlyOnce(t *testing.T) {
	db := useRedemptionCardTestDB(t)
	user := User{Username: "concurrent-card-user", Password: "password", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&user).Error)
	card := RedemptionCard{
		Key:         "456789ABCDEFGHJKLMNPQRST",
		Group:       RedemptionCardGroup3RMB,
		Quota:       3_000,
		Status:      common.RedemptionCodeStatusEnabled,
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, db.Create(&card).Error)

	const requests = 5
	results := make(chan error, requests)
	var waitGroup sync.WaitGroup
	waitGroup.Add(requests)
	for range requests {
		go func() {
			defer waitGroup.Done()
			_, err := Redeem(card.Key, user.Id)
			results <- err
		}()
	}
	waitGroup.Wait()
	close(results)

	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	assert.Equal(t, 1, successes)

	var updatedUser User
	require.NoError(t, db.First(&updatedUser, user.Id).Error)
	assert.Equal(t, card.Quota, updatedUser.Quota)
	var updatedCard RedemptionCard
	require.NoError(t, db.First(&updatedCard, card.Id).Error)
	assert.Equal(t, common.RedemptionCodeStatusUsed, updatedCard.Status)
}

func TestRecentRedemptionCardHistoryReturnsLatestTen(t *testing.T) {
	db := useRedemptionCardTestDB(t)
	const userID = 42
	for index := 1; index <= 12; index++ {
		card := RedemptionCard{
			Key:          fmt.Sprintf("%024d", index),
			Group:        RedemptionCardGroup3RMB,
			Quota:        3_000,
			Status:       common.RedemptionCodeStatusUsed,
			CreatedTime:  int64(index),
			RedeemedTime: int64(index),
			UsedUserId:   userID,
		}
		require.NoError(t, db.Create(&card).Error)
	}

	history, err := GetRecentRedemptionCardHistory(userID, 10)
	require.NoError(t, err)
	require.Len(t, history, 10)
	assert.EqualValues(t, 12, history[0].RedeemedTime)
	assert.EqualValues(t, 3, history[9].RedeemedTime)
}
