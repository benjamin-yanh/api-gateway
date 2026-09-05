package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCompleteInitialSetupCanOnlyBeClaimedOnce(t *testing.T) {
	originalDB := DB
	originalOptionMap := common.OptionMap
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Setup{}, &User{}, &Option{}))
	DB = db
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		DB = originalDB
		common.OptionMap = originalOptionMap
	})

	firstRoot := &User{Username: "first@example.com", Email: "first@example.com", Password: "hash", Role: common.RoleRootUser, Status: common.UserStatusEnabled}
	secondRoot := &User{Username: "second@example.com", Email: "second@example.com", Password: "hash", Role: common.RoleRootUser, Status: common.UserStatusEnabled}
	setup := Setup{Version: "test", InitializedAt: time.Now().Unix()}

	require.NoError(t, CompleteInitialSetup(firstRoot, setup, map[string]string{"SelfUseModeEnabled": "false"}))
	assert.ErrorIs(t, CompleteInitialSetup(secondRoot, setup, map[string]string{"SelfUseModeEnabled": "true"}), ErrSetupAlreadyCompleted)

	var rootCount int64
	require.NoError(t, DB.Model(&User{}).Where("role = ?", common.RoleRootUser).Count(&rootCount).Error)
	assert.Equal(t, int64(1), rootCount)
	var root User
	require.NoError(t, DB.Where("role = ?", common.RoleRootUser).First(&root).Error)
	assert.Equal(t, "first@example.com", root.Email)
}
