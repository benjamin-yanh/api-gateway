package model

import (
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDecreaseUserQuotaIfEnoughSerializesConcurrentReservations(t *testing.T) {
	originalDB := DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&User{}))
	DB = db
	t.Cleanup(func() {
		DB = originalDB
		require.NoError(t, sqlDB.Close())
	})

	user := User{Quota: 100}
	require.NoError(t, DB.Create(&user).Error)

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- DecreaseUserQuotaIfEnough(user.Id, 80)
		}()
	}
	wg.Wait()
	close(results)

	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	assert.Equal(t, 1, successes)
	var remaining int
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Select("quota").Scan(&remaining).Error)
	assert.Equal(t, 20, remaining)
}

func TestDecreaseTokenQuotaIfEnoughKeepsFiniteQuotaNonNegative(t *testing.T) {
	originalDB := DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Token{}))
	DB = db
	t.Cleanup(func() { DB = originalDB })

	token := Token{Key: "test-key", RemainQuota: 50}
	require.NoError(t, DB.Create(&token).Error)
	require.NoError(t, DecreaseTokenQuotaIfEnough(token.Id, token.Key, 40))
	require.Error(t, DecreaseTokenQuotaIfEnough(token.Id, token.Key, 20))

	var stored Token
	require.NoError(t, DB.First(&stored, token.Id).Error)
	assert.Equal(t, 10, stored.RemainQuota)
	assert.Equal(t, 40, stored.UsedQuota)
}

func TestGetEnabledGroupModelsPreservesGroupSpecificPairs(t *testing.T) {
	originalDB := DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Ability{}))
	DB = db
	t.Cleanup(func() { DB = originalDB })

	priority := int64(0)
	require.NoError(t, DB.Create([]Ability{
		{Group: "default", Model: "model-a", ChannelId: 1, Enabled: true, Priority: &priority},
		{Group: "premium", Model: "model-a", ChannelId: 2, Enabled: true, Priority: &priority},
		{Group: "default", Model: "disabled-model", ChannelId: 3, Enabled: false, Priority: &priority},
	}).Error)

	pairs, err := GetEnabledGroupModels([]string{"default", "premium"})
	require.NoError(t, err)
	assert.ElementsMatch(t, []GroupModel{
		{Group: "default", Model: "model-a"},
		{Group: "premium", Model: "model-a"},
	}, pairs)
}
