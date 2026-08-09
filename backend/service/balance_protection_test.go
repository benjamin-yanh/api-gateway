package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	relaykittypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestApplyRequestOutputLimitPreservesProtocolSemantics(t *testing.T) {
	t.Run("openai clamps both supplied aliases", func(t *testing.T) {
		maxTokens := uint(500)
		maxCompletion := uint(600)
		req := &dto.GeneralOpenAIRequest{
			MaxTokens:           &maxTokens,
			MaxCompletionTokens: &maxCompletion,
		}
		applyRequestOutputLimit(req, 128)
		assert.Equal(t, uint(128), *req.MaxTokens)
		assert.Equal(t, uint(128), *req.MaxCompletionTokens)
	})

	t.Run("responses injects max output", func(t *testing.T) {
		req := &dto.OpenAIResponsesRequest{}
		applyRequestOutputLimit(req, 128)
		assert.Equal(t, uint(128), *req.MaxOutputTokens)
	})

	t.Run("claude canonicalizes legacy alias", func(t *testing.T) {
		legacy := uint(500)
		req := &dto.ClaudeRequest{MaxTokensToSample: &legacy}
		applyRequestOutputLimit(req, 128)
		assert.Equal(t, uint(128), *req.MaxTokens)
		assert.Nil(t, req.MaxTokensToSample)
	})

	t.Run("gemini writes generation config", func(t *testing.T) {
		req := &dto.GeminiChatRequest{}
		applyRequestOutputLimit(req, 128)
		assert.Equal(t, uint(128), *req.GenerationConfig.MaxOutputTokens)
	})
}

func TestAppendBalanceProtectionAdminInfo(t *testing.T) {
	adminInfo := map[string]interface{}{}
	appendBalanceProtectionAdminInfo(&relaycommon.RelayInfo{
		BalanceProtectionActive:             true,
		BalanceProtectionAvailableQuota:     800,
		BalanceProtectionThresholdQuota:     1000,
		EffectiveMaxOutputTokens:            128,
		BalanceProtectionMostExpensiveModel: "expensive-model",
	}, adminInfo)

	protection, ok := adminInfo["balance_protection"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, 128, int(protection["effective_max_tokens"].(uint)))
	assert.Equal(t, "expensive-model", protection["most_expensive_model"])
}

func TestApplyBalanceProtectionCapsOutputFromAvailableWalletQuota(t *testing.T) {
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Ability{}, &model.UserSubscription{}))
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })

	priority := int64(0)
	require.NoError(t, model.DB.Create(&model.Ability{
		Group: "default", Model: "balance-test-model", ChannelId: 1, Enabled: true, Priority: &priority,
	}).Error)
	user := model.User{Quota: 10_000}
	require.NoError(t, model.DB.Create(&user).Error)

	originalModelRatios := ratio_setting.ModelRatio2JSONString()
	originalCompletionRatios := ratio_setting.CompletionRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"balance-test-model":1}`))
	require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(`{"balance-test-model":2}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(originalModelRatios))
		require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(originalCompletionRatios))
	})

	cfg := operation_setting.GetQuotaSetting()
	originalEnabled := cfg.BalanceProtectionEnabled
	originalThreshold := cfg.BalanceProtectionThreshold10KTokens
	originalSafety := cfg.BalanceProtectionSafetyRatio
	originalMinimum := cfg.BalanceProtectionMinimumOutput
	originalMaximum := cfg.BalanceProtectionMaximumOutput
	cfg.BalanceProtectionEnabled = true
	cfg.BalanceProtectionThreshold10KTokens = 100
	cfg.BalanceProtectionSafetyRatio = 1.10
	cfg.BalanceProtectionMinimumOutput = 64
	cfg.BalanceProtectionMaximumOutput = 1_000_000
	t.Cleanup(func() {
		cfg.BalanceProtectionEnabled = originalEnabled
		cfg.BalanceProtectionThreshold10KTokens = originalThreshold
		cfg.BalanceProtectionSafetyRatio = originalSafety
		cfg.BalanceProtectionMinimumOutput = originalMinimum
		cfg.BalanceProtectionMaximumOutput = originalMaximum
	})

	clientMax := uint(8_000)
	request := &dto.GeneralOpenAIRequest{MaxTokens: &clientMax}
	meta := &relaykittypes.TokenCountMeta{MaxTokens: int(clientMax)}
	info := &relaycommon.RelayInfo{
		UserId:          user.Id,
		UserGroup:       "default",
		UsingGroup:      "default",
		TokenUnlimited:  true,
		OriginModelName: "balance-test-model",
		UserSetting:     dto.UserSetting{BillingPreference: "wallet_only"},
	}

	require.NoError(t, ApplyBalanceProtection(info, request, 1_000, meta))
	assert.True(t, info.BalanceProtectionActive)
	assert.Equal(t, uint(4_090), info.EffectiveMaxOutputTokens)
	assert.Equal(t, int(info.EffectiveMaxOutputTokens), meta.MaxTokens)
	assert.Equal(t, info.EffectiveMaxOutputTokens, *request.MaxTokens)
	assert.Equal(t, 2_200_000, info.BalanceProtectionThresholdQuota)
	assert.True(t, info.ForcePreConsume)
}
