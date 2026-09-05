package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageCashbackSettingsVersionAndAuditCommitTogether(t *testing.T) {
	db := useCashbackTestDB(t)
	require.NoError(t, db.AutoMigrate(&UsageCashbackSetting{}, &UsageCashbackSettingRevision{}))
	previousBatch := common.BatchUpdateEnabled
	common.BatchUpdateEnabled = false
	t.Cleanup(func() { common.BatchUpdateEnabled = previousBatch })
	initial, err := GetUsageCashbackSettings()
	require.NoError(t, err)
	assert.False(t, initial.Enabled)
	assert.Zero(t, initial.Version)
	initial.Enabled, initial.MaxRatio = true, "0.1"
	initial.Models["text-model"] = UsageCashbackModelRule{Enabled: true, InputPerMillion: "1.25", OutputPerMillion: "0"}
	saved, err := SaveUsageCashbackSettings(*initial, 7)
	require.NoError(t, err)
	assert.EqualValues(t, 1, saved.Version)
	assert.Equal(t, 7, saved.UpdatedBy)
	_, err = SaveUsageCashbackSettings(*initial, 8)
	assert.ErrorIs(t, err, ErrUsageCashbackSettingsConflict)
	var revisions []UsageCashbackSettingRevision
	require.NoError(t, db.Find(&revisions).Error)
	require.Len(t, revisions, 1)
	assert.Equal(t, 7, revisions[0].UpdatedBy)
	// An audit failure must not publish the disabled rule or consume its version.
	require.NoError(t, db.Migrator().DropTable(&UsageCashbackSettingRevision{}))
	saved.Enabled = false
	_, err = SaveUsageCashbackSettings(*saved, 8)
	require.Error(t, err)
	current, err := GetUsageCashbackSettings()
	require.NoError(t, err)
	assert.True(t, current.Enabled)
	assert.EqualValues(t, 1, current.Version)
	assert.Equal(t, "1.25", current.Models["text-model"].InputPerMillion)
}

func TestUsageCashbackSettingsRejectInvalidFinancialRules(t *testing.T) {
	previousBatch := common.BatchUpdateEnabled
	common.BatchUpdateEnabled = false
	t.Cleanup(func() { common.BatchUpdateEnabled = previousBatch })
	for _, tc := range []struct {
		name  string
		ratio string
		input string
	}{
		{"zero cap", "0", "1"},
		{"full cap", "1", "1"},
		{"negative cap", "-0.1", "1"},
		{"cap precision", "0.0000001", "1"},
		{"negative amount", "0.1", "-1"},
		{"amount precision", "0.1", "0.000000001"},
		{"amount overflow", "0.1", "1000000.00000001"},
		{"exponent", "0.1", "1e3"},
		{"non-finite", "0.1", "NaN"},
		{"empty amount", "0.1", ""},
		{"no rewarding tokens", "0.1", "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := UsageCashbackSettings{Enabled: true, MaxRatio: tc.ratio, Models: map[string]UsageCashbackModelRule{
				"text-model": {Enabled: true, InputPerMillion: tc.input, OutputPerMillion: "0"},
			}}
			assert.Error(t, ValidateUsageCashbackSettings(&settings))
		})
	}
	valid := UsageCashbackSettings{Enabled: true, MaxRatio: "0.999999", Models: map[string]UsageCashbackModelRule{
		"text-model": {Enabled: true, InputPerMillion: "0.00000001", OutputPerMillion: "1000000.00000000"},
	}}
	require.NoError(t, ValidateUsageCashbackSettings(&valid))
	valid.MaxRatio = ""
	require.NoError(t, ValidateUsageCashbackSettings(&valid))
	common.BatchUpdateEnabled = true
	assert.EqualError(t, ValidateUsageCashbackSettings(&valid), "cashback_requires_durable_billing")
	valid.Enabled = false
	assert.NoError(t, ValidateUsageCashbackSettings(&valid), "operators must still be able to disable cashback")
}

func TestUsageCashbackSettingsSnapshotSurvivesClosingAndReopening(t *testing.T) {
	db := useCashbackTestDB(t)
	require.NoError(t, db.AutoMigrate(&UsageCashbackSetting{}, &UsageCashbackSettingRevision{}))
	previousBatch := common.BatchUpdateEnabled
	common.BatchUpdateEnabled = false
	t.Cleanup(func() { common.BatchUpdateEnabled = previousBatch })
	settings := UsageCashbackSettings{Enabled: true, MaxRatio: "0.1", Models: map[string]UsageCashbackModelRule{
		"text-model": {Enabled: true, InputPerMillion: "1", OutputPerMillion: "2"},
	}}
	saved, err := SaveUsageCashbackSettings(settings, 7)
	require.NoError(t, err)
	active, err := GetUsageCashbackSettings()
	require.NoError(t, err)
	saved.Enabled = false
	saved, err = SaveUsageCashbackSettings(*saved, 7)
	require.NoError(t, err)
	closed, err := GetUsageCashbackSettings()
	require.NoError(t, err)
	saved.Enabled = true
	saved.Models["text-model"] = UsageCashbackModelRule{Enabled: true, InputPerMillion: "3", OutputPerMillion: "4"}
	_, err = SaveUsageCashbackSettings(*saved, 7)
	require.NoError(t, err)
	assert.True(t, active.Enabled)
	assert.Equal(t, "1", active.Models["text-model"].InputPerMillion)
	assert.False(t, closed.Enabled)
	assert.EqualValues(t, 2, closed.Version)
	// A missing schema/DB outage must not be confused with default-disabled rules.
	require.NoError(t, db.Migrator().DropTable(&UsageCashbackSetting{}))
	_, err = GetUsageCashbackSettings()
	assert.Error(t, err)
}
