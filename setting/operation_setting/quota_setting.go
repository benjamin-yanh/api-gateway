package operation_setting

import (
	"fmt"
	"math"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/types"
)

type QuotaSetting struct {
	EnableFreeModelPreConsume            bool                          `json:"enable_free_model_pre_consume"`            // 是否对免费模型启用预消耗
	BalanceProtectionEnabled             bool                          `json:"balance_protection_enabled"`               // 是否启用最后 N Token 余额保护
	BalanceProtectionThreshold10KTokens  int64                         `json:"balance_protection_threshold_10k_tokens"`  // 保护区大小，配置单位为 1 万 Token
	BalanceProtectionSafetyRatio         float64                       `json:"balance_protection_safety_ratio"`          // 输出预算的安全系数
	BalanceProtectionMinimumOutput       uint                          `json:"balance_protection_minimum_output_tokens"` // 低于该输出上限时拒绝请求
	BalanceProtectionMaximumOutput       uint                          `json:"balance_protection_maximum_output_tokens"` // 站点级输出 Token 硬上限
	BalanceProtectionTieredPriceCeilings *types.RWMap[string, float64] `json:"balance_protection_tiered_price_ceilings"` // tiered_expr 模型的保守输出价格（USD/1M）
}

const BalanceProtectionThresholdTokenUnit int64 = 10_000

// 默认配置
var quotaSetting = QuotaSetting{
	EnableFreeModelPreConsume:            true,
	BalanceProtectionEnabled:             false,
	BalanceProtectionThreshold10KTokens:  100,
	BalanceProtectionSafetyRatio:         1.10,
	BalanceProtectionMinimumOutput:       64,
	BalanceProtectionMaximumOutput:       1_000_000,
	BalanceProtectionTieredPriceCeilings: types.NewRWMap[string, float64](),
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("quota_setting", &quotaSetting)
}

func GetQuotaSetting() *QuotaSetting {
	return &quotaSetting
}

// BalanceProtectionThresholdTokens converts the administrator-facing
// ten-thousand-token unit into the exact token count used by billing.
func (s *QuotaSetting) BalanceProtectionThresholdTokens() (int64, error) {
	if s == nil ||
		s.BalanceProtectionThreshold10KTokens <= 0 ||
		s.BalanceProtectionThreshold10KTokens > math.MaxInt64/BalanceProtectionThresholdTokenUnit {
		return 0, fmt.Errorf("invalid balance protection threshold")
	}
	return s.BalanceProtectionThreshold10KTokens * BalanceProtectionThresholdTokenUnit, nil
}

// ValidateQuotaSettingOption validates hot-loaded balance protection values
// before they are persisted. config.UpdateConfigFromMap intentionally accepts
// partial maps, so validation is performed at the option boundary.
func ValidateQuotaSettingOption(key, value string) error {
	const prefix = "quota_setting."
	if len(key) <= len(prefix) || key[:len(prefix)] != prefix {
		return nil
	}
	switch key[len(prefix):] {
	case "balance_protection_enabled":
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("balance_protection_enabled must be true or false")
		}
	case "balance_protection_threshold_10k_tokens":
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil || v <= 0 || v > math.MaxInt64/BalanceProtectionThresholdTokenUnit {
			return fmt.Errorf("balance_protection_threshold_10k_tokens must be a positive integer that can be converted to tokens")
		}
	case "balance_protection_safety_ratio":
		v, err := strconv.ParseFloat(value, 64)
		if err != nil || v < 1 || math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("balance_protection_safety_ratio must be a finite number >= 1")
		}
	case "balance_protection_minimum_output_tokens", "balance_protection_maximum_output_tokens":
		v, err := strconv.ParseUint(value, 10, 64)
		if err != nil || v == 0 || v > 1_000_000 {
			return fmt.Errorf("%s must be an integer in [1, 1000000]", key[len(prefix):])
		}
	case "balance_protection_tiered_price_ceilings":
		ceilings := map[string]float64{}
		if err := common.Unmarshal([]byte(value), &ceilings); err != nil {
			return fmt.Errorf("invalid balance protection tiered price ceilings: %w", err)
		}
		if ceilings == nil {
			return fmt.Errorf("balance protection tiered price ceilings must be a JSON object")
		}
		for modelName, ceiling := range ceilings {
			if modelName == "" || ceiling <= 0 || math.IsNaN(ceiling) || math.IsInf(ceiling, 0) {
				return fmt.Errorf("invalid tiered price ceiling for model %q", modelName)
			}
		}
	}
	return nil
}
