package service

import (
	"math"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCashbackCalculationUsesFinalChargeAndInternalPrecision(t *testing.T) {
	snapshot := usageCashbackSnapshot{InputPerMillion: "1", OutputPerMillion: "2", MaxRatio: "0.1", QuotaPerCNY: "100000"}
	tests := []struct {
		name          string
		input, output int64
		charge, want  int
	}{
		{"fraction of a million", 200000, 50000, 1000000, 30000},
		{"cap on final charge", 200000, 50000, 200000, 20000},
		{"less than one cent still credited", 100, 0, 10000, 10},
		{"less than one quota rounds down", 1, 0, 10000, 0},
		{"one quota charge cannot self replenish", 1000000, 0, 1, 0},
		{"free usage", 200000, 50000, 0, 0},
		{"large usage stays within cap", math.MaxInt32, math.MaxInt32, math.MaxInt32, 214748364},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, capped, err := calculateUsageCashback(snapshot, test.input, test.output, test.charge)
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
			if test.name == "cap on final charge" {
				assert.True(t, capped)
			}
		})
	}
}

func TestCashbackCalculationRejectsInvalidInputs(t *testing.T) {
	valid := usageCashbackSnapshot{InputPerMillion: "1", OutputPerMillion: "2", MaxRatio: "0.1", QuotaPerCNY: "100000"}
	tests := []struct {
		name     string
		snapshot usageCashbackSnapshot
		input    int64
		charge   int
	}{
		{"negative tokens", valid, -1, 100},
		{"oversized tokens", valid, int64(math.MaxInt32) + 1, 100},
		{"negative charge", valid, 1, -1},
		{"negative reward", usageCashbackSnapshot{InputPerMillion: "-1", OutputPerMillion: "2", MaxRatio: "0.1", QuotaPerCNY: "100000"}, 1, 100},
		{"nan reward", usageCashbackSnapshot{InputPerMillion: "NaN", OutputPerMillion: "2", MaxRatio: "0.1", QuotaPerCNY: "100000"}, 1, 100},
		{"full refund cap", usageCashbackSnapshot{InputPerMillion: "1", OutputPerMillion: "2", MaxRatio: "1", QuotaPerCNY: "100000"}, 1, 100},
		{"invalid conversion", usageCashbackSnapshot{InputPerMillion: "1", OutputPerMillion: "2", MaxRatio: "0.1", QuotaPerCNY: "0"}, 1, 100},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := calculateUsageCashback(test.snapshot, test.input, 0, test.charge)
			require.Error(t, err)
		})
	}
}

func TestCashbackTokenUsagePreservesProviderSemantics(t *testing.T) {
	tests := []struct {
		name          string
		usage         *dto.Usage
		input, output int64
		reason        string
	}{
		{"openai excludes read and write caches", &dto.Usage{BillingUsage: dto.NewOpenAIChatBillingUsage(&dto.Usage{PromptTokens: 1000000, CompletionTokens: 50000, PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 700000, CacheWriteTokens: 100000}})}, 200000, 50000, ""},
		{"responses reads input details", &dto.Usage{BillingUsage: dto.NewOpenAIResponsesBillingUsage(&dto.Usage{InputTokens: 1000000, OutputTokens: 50000, InputTokensDetails: &dto.InputTokenDetails{CachedTokens: 800000}})}, 200000, 50000, ""},
		{"claude input already excludes caches", &dto.Usage{BillingUsage: dto.NewClaudeMessagesBillingUsage(&dto.ClaudeUsage{InputTokens: 200000, OutputTokens: 50000, CacheReadInputTokens: 700000, CacheCreationInputTokens: 100000})}, 200000, 50000, ""},
		{"gemini counts thought output once", &dto.Usage{BillingUsage: dto.NewGeminiChatBillingUsage(&dto.GeminiUsageMetadata{PromptTokenCount: 900000, ToolUsePromptTokenCount: 100000, CachedContentTokenCount: 800000, CandidatesTokenCount: 30000, ThoughtsTokenCount: 20000})}, 200000, 50000, ""},
		{"missing usage does not earn", nil, 0, 0, "unknown_usage"},
		{"unverified format cannot infer semantics", &dto.Usage{PromptTokens: 1000000}, 0, 0, "unknown_usage"},
		{"estimated gemini cannot earn", &dto.Usage{BillingUsage: dto.NewEstimatedGeminiChatBillingUsage(&dto.Usage{PromptTokens: 1000000})}, 0, 0, "estimated_usage"},
		{"native overlapping cache prefixes preserve output", &dto.Usage{BillingUsage: dto.NewOpenAIChatBillingUsage(&dto.Usage{PromptTokens: 100, CompletionTokens: 20, PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 80, CacheWriteTokens: 50}})}, 0, 20, ""},
		{"cache exceeding total input requires review", &dto.Usage{BillingUsage: dto.NewOpenAIChatBillingUsage(&dto.Usage{PromptTokens: 100, PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 120}})}, 0, 0, "invalid_usage"},
		{"negative cache is not zero", &dto.Usage{BillingUsage: dto.NewOpenAIChatBillingUsage(&dto.Usage{PromptTokens: 100, PromptTokensDetails: dto.InputTokenDetails{CacheWriteTokens: -1}})}, 0, 0, "invalid_usage"},
		{"mixed image usage excluded", &dto.Usage{BillingUsage: dto.NewOpenAIChatBillingUsage(&dto.Usage{PromptTokens: 100, PromptTokensDetails: dto.InputTokenDetails{ImageTokens: 50}})}, 0, 0, "unsupported_usage"},
		{"contradictory input totals require review", &dto.Usage{BillingUsage: dto.NewOpenAIChatBillingUsage(&dto.Usage{PromptTokens: 100, InputTokens: 90})}, 0, 0, "invalid_usage"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input, output, _, reason := cashbackTokenUsage(test.usage)
			assert.Equal(t, test.input, input)
			assert.Equal(t, test.output, output)
			assert.Equal(t, test.reason, reason)
		})
	}
}

func useCashbackServiceDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "cashback-service.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.CashbackUsage{}, &model.CashbackEntry{}, &model.CashbackRefund{}, &model.UsageCashbackSetting{}))
	previousDB, previousRedis, previousBatch := model.DB, common.RedisEnabled, common.BatchUpdateEnabled
	previousType := common.MainDatabaseType()
	model.DB, common.RedisEnabled, common.BatchUpdateEnabled = db, false, false
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		model.DB, common.RedisEnabled, common.BatchUpdateEnabled = previousDB, previousRedis, previousBatch
		common.SetMainDatabaseType(previousType)
		_ = sqlDB.Close()
	})
	return db
}

func TestCashbackSessionSettlesWalletTokenAndRewardOnlyOnce(t *testing.T) {
	db := useCashbackServiceDB(t)
	user := model.User{Username: "cashback-integration", Quota: 1000000, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&user).Error)
	token := model.Token{UserId: user.Id, Key: "cashback-integration-token", RemainQuota: 1000000, Status: common.TokenStatusEnabled}
	require.NoError(t, db.Create(&token).Error)
	ctx, _ := gin.CreateTestContext(nil)
	ctx.Set(cashbackSnapshotContextKey, &usageCashbackSnapshot{Enabled: true, ModelName: "cashback-model", InputPerMillion: "1", OutputPerMillion: "2", MaxRatio: "0.1", QuotaPerCNY: "100000"})
	info := &relaycommon.RelayInfo{UserId: user.Id, TokenId: token.Id, TokenKey: token.Key, RequestId: "cashback-integration-request", OriginModelName: "cashback-model", UserSetting: dto.UserSetting{BillingPreference: "wallet_only"}}
	require.Nil(t, PreConsumeBilling(ctx, 500000, info))
	require.NoError(t, info.Billing.Reserve(600000))
	require.NoError(t, info.Billing.Reserve(600000))
	usage := &dto.Usage{BillingUsage: dto.NewOpenAIChatBillingUsage(&dto.Usage{PromptTokens: 200000, CompletionTokens: 50000})}
	prepareUsageCashback(ctx, info, usage, 200000, false)
	require.NoError(t, SettleBilling(ctx, info, 200000))
	require.NoError(t, SettleBilling(ctx, info, 200000))
	info.Billing.Refund(ctx)
	require.NoError(t, db.First(&user, user.Id).Error)
	require.NoError(t, db.First(&token, token.Id).Error)
	assert.Equal(t, 800000, user.Quota)
	assert.Equal(t, 20000, user.CashbackQuota)
	assert.Equal(t, 800000, token.RemainQuota)
	assert.Equal(t, 200000, token.UsedQuota)
	assert.False(t, info.Billing.NeedsRefund())
}

func TestCashbackUnknownUsageMaySettleWithoutReward(t *testing.T) {
	db := useCashbackServiceDB(t)
	user := model.User{Username: "cashback-unknown", Quota: 1000, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&user).Error)
	ctx, _ := gin.CreateTestContext(nil)
	ctx.Set(cashbackSnapshotContextKey, &usageCashbackSnapshot{Enabled: true, ModelName: "cashback-model", InputPerMillion: "1", OutputPerMillion: "2", MaxRatio: "0.1", QuotaPerCNY: "100000"})
	info := &relaycommon.RelayInfo{UserId: user.Id, IsPlayground: true, RequestId: "unknown-request", OriginModelName: "cashback-model", UserSetting: dto.UserSetting{BillingPreference: "wallet_only"}}
	require.Nil(t, PreConsumeBilling(ctx, 100, info))
	require.NoError(t, info.Billing.Settle(100))
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 900, user.Quota)
	assert.Zero(t, user.CashbackQuota)
	rows, count, err := model.ListCashbackUsages(user.Id, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	assert.Equal(t, "unknown_usage", rows[0].Reason)
}

func TestCashbackSnapshotDoesNotChangeWhenOfferEnabledMidRequest(t *testing.T) {
	db := useCashbackServiceDB(t)
	ctx, _ := gin.CreateTestContext(nil)
	info := &relaycommon.RelayInfo{OriginModelName: "cashback-model", RelayFormat: types.RelayFormatOpenAI}
	require.NoError(t, CaptureUsageCashback(ctx, info, nil))
	document, err := common.Marshal(model.UsageCashbackSettings{Version: 1, Enabled: true, MaxRatio: "0.1", Models: map[string]model.UsageCashbackModelRule{"cashback-model": {Enabled: true, InputPerMillion: "1", OutputPerMillion: "2"}}})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.UsageCashbackSetting{ID: 1, Version: 1, Document: string(document)}).Error)
	require.NoError(t, CaptureUsageCashback(ctx, info, nil))
	value, exists := ctx.Get(cashbackSnapshotContextKey)
	require.True(t, exists)
	assert.False(t, value.(*usageCashbackSnapshot).Enabled)
	newCtx, _ := gin.CreateTestContext(nil)
	require.NoError(t, CaptureUsageCashback(newCtx, info, nil))
	value, _ = newCtx.Get(cashbackSnapshotContextKey)
	assert.True(t, value.(*usageCashbackSnapshot).Enabled)
	common.BatchUpdateEnabled = true
	batchCtx, _ := gin.CreateTestContext(nil)
	require.Error(t, CaptureUsageCashback(batchCtx, info, nil))
}

func TestCashbackTextConsumptionAccruesWithConsumeLoggingDisabled(t *testing.T) {
	db := useCashbackServiceDB(t)
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	previousLogging := common.LogConsumeEnabled
	common.LogConsumeEnabled = false
	t.Cleanup(func() { common.LogConsumeEnabled = previousLogging })
	user := model.User{Username: "cashback-text", Quota: 1000000, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&user).Error)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	ctx.Set(cashbackSnapshotContextKey, &usageCashbackSnapshot{Enabled: true, ModelName: "cashback-model", InputPerMillion: "1", OutputPerMillion: "2", MaxRatio: "0.1", QuotaPerCNY: "100000"})
	info := &relaycommon.RelayInfo{
		UserId: user.Id, IsPlayground: true, RequestId: "cashback-text-request",
		OriginModelName: "cashback-model", StartTime: time.Now(), RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{},
		UserSetting: dto.UserSetting{BillingPreference: "wallet_only"},
		PriceData:   hosttypes.PriceData{ModelRatio: 1, CompletionRatio: 2, GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: 1}},
	}
	require.Nil(t, PreConsumeBilling(ctx, 500000, info))
	PostTextConsumeQuota(ctx, info, &dto.Usage{BillingUsage: dto.NewOpenAIChatBillingUsage(&dto.Usage{PromptTokens: 200000, CompletionTokens: 50000})}, nil)
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 700000, user.Quota)
	assert.Equal(t, 30000, user.CashbackQuota)
	rows, total, err := model.ListCashbackUsages(user.Id, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	assert.Equal(t, model.CashbackStateSettled, rows[0].State)
	assert.Equal(t, 300000, rows[0].ActualQuota)
	assert.EqualValues(t, 200000, rows[0].InputTokens)
	assert.EqualValues(t, 50000, rows[0].OutputTokens)
}

func TestCashbackSnapshotExcludesRequestedMediaWithoutUsageBreakdown(t *testing.T) {
	db := useCashbackServiceDB(t)
	document, err := common.Marshal(model.UsageCashbackSettings{Version: 1, Enabled: true, MaxRatio: "0.1", Models: map[string]model.UsageCashbackModelRule{"cashback-model": {Enabled: true, InputPerMillion: "1", OutputPerMillion: "2"}}})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.UsageCashbackSetting{ID: 1, Version: 1, Document: string(document)}).Error)
	for _, request := range []dto.Request{
		&dto.GeneralOpenAIRequest{Modalities: []byte(`["text","audio"]`)},
		&dto.GeminiChatRequest{GenerationConfig: dto.GeminiChatGenerationConfig{ResponseModalities: []string{"IMAGE", "TEXT"}}},
		&dto.OpenAIResponsesRequest{Tools: []byte(`[{"type":"image_generation"}]`)},
	} {
		ctx, _ := gin.CreateTestContext(nil)
		info := &relaycommon.RelayInfo{OriginModelName: "cashback-model", RelayFormat: types.RelayFormatOpenAI}
		require.NoError(t, CaptureUsageCashback(ctx, info, request))
		value, exists := ctx.Get(cashbackSnapshotContextKey)
		require.True(t, exists)
		assert.Equal(t, "unsupported_usage", value.(*usageCashbackSnapshot).Reason)
	}
}

func TestCashbackCalculationWithoutCap(t *testing.T) {
	snapshot := usageCashbackSnapshot{InputPerMillion: "1", OutputPerMillion: "2", QuotaPerCNY: "100000"}
	quota, capped, err := calculateUsageCashback(snapshot, 200000, 50000, 100)
	require.NoError(t, err)
	assert.Equal(t, 30000, quota)
	assert.False(t, capped)
	quota, _, err = calculateUsageCashback(snapshot, 200000, 50000, 0)
	require.NoError(t, err)
	assert.Zero(t, quota)
	snapshot.InputPerMillion = "1000000"
	_, _, err = calculateUsageCashback(snapshot, math.MaxInt32, 0, 100)
	require.Error(t, err)
}
