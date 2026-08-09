package operation_setting

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateQuotaSettingOption(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr bool
	}{
		{name: "threshold in ten thousand tokens", key: "quota_setting.balance_protection_threshold_10k_tokens", value: "100"},
		{name: "minimum threshold", key: "quota_setting.balance_protection_threshold_10k_tokens", value: "1"},
		{name: "zero threshold", key: "quota_setting.balance_protection_threshold_10k_tokens", value: "0", wantErr: true},
		{name: "negative threshold", key: "quota_setting.balance_protection_threshold_10k_tokens", value: "-1", wantErr: true},
		{name: "fractional threshold", key: "quota_setting.balance_protection_threshold_10k_tokens", value: "1.5", wantErr: true},
		{name: "non-numeric threshold", key: "quota_setting.balance_protection_threshold_10k_tokens", value: "one hundred", wantErr: true},
		{name: "largest convertible threshold", key: "quota_setting.balance_protection_threshold_10k_tokens", value: "922337203685477"},
		{name: "overflowing threshold", key: "quota_setting.balance_protection_threshold_10k_tokens", value: "922337203685478", wantErr: true},
		{name: "safety ratio", key: "quota_setting.balance_protection_safety_ratio", value: "1.1"},
		{name: "unsafe ratio", key: "quota_setting.balance_protection_safety_ratio", value: "0.9", wantErr: true},
		{name: "max output", key: "quota_setting.balance_protection_maximum_output_tokens", value: "1000000"},
		{name: "oversized max output", key: "quota_setting.balance_protection_maximum_output_tokens", value: "1000001", wantErr: true},
		{name: "tiered ceilings", key: "quota_setting.balance_protection_tiered_price_ceilings", value: `{"model-a":30.5}`},
		{name: "invalid tiered ceiling", key: "quota_setting.balance_protection_tiered_price_ceilings", value: `{"model-a":0}`, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateQuotaSettingOption(test.key, test.value)
			if test.wantErr {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBalanceProtectionThresholdDefaultUsesTenThousandTokenUnits(t *testing.T) {
	cfg := GetQuotaSetting()

	assert.Equal(t, int64(100), cfg.BalanceProtectionThreshold10KTokens)
	tokens, err := cfg.BalanceProtectionThresholdTokens()
	require.NoError(t, err)
	assert.Equal(t, int64(1_000_000), tokens)
}

func TestBalanceProtectionThresholdTokens(t *testing.T) {
	tests := []struct {
		name    string
		units   int64
		want    int64
		wantErr bool
	}{
		{name: "one unit is ten thousand tokens", units: 1, want: 10_000},
		{name: "default one hundred units is one million tokens", units: 100, want: 1_000_000},
		{
			name:  "largest convertible value does not overflow",
			units: math.MaxInt64 / BalanceProtectionThresholdTokenUnit,
			want:  (math.MaxInt64 / BalanceProtectionThresholdTokenUnit) * BalanceProtectionThresholdTokenUnit,
		},
		{name: "zero is invalid", units: 0, wantErr: true},
		{name: "negative is invalid", units: -1, wantErr: true},
		{name: "multiplication overflow is rejected", units: math.MaxInt64/BalanceProtectionThresholdTokenUnit + 1, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &QuotaSetting{BalanceProtectionThreshold10KTokens: test.units}
			got, err := cfg.BalanceProtectionThresholdTokens()
			if test.wantErr {
				require.Error(t, err)
				assert.Zero(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestBalanceProtectionThresholdTokensRejectsNilSetting(t *testing.T) {
	var cfg *QuotaSetting

	tokens, err := cfg.BalanceProtectionThresholdTokens()

	require.Error(t, err)
	assert.Zero(t, tokens)
}

func TestBalanceProtectionThresholdHotLoadUsesTenThousandTokenValue(t *testing.T) {
	cfg := &QuotaSetting{BalanceProtectionThreshold10KTokens: 100}

	require.NoError(t, config.UpdateConfigFromMap(cfg, map[string]string{
		"balance_protection_threshold_10k_tokens": "250",
	}))
	assert.Equal(t, int64(250), cfg.BalanceProtectionThreshold10KTokens)

	tokens, err := cfg.BalanceProtectionThresholdTokens()
	require.NoError(t, err)
	assert.Equal(t, int64(2_500_000), tokens)
}
