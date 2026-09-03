package service

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

const cashbackSnapshotContextKey = "usage_cashback_snapshot"

// A disabled snapshot is retained too: enabling an offer during a request must
// not retroactively change that request's eligibility.
type usageCashbackSnapshot struct {
	Enabled          bool   `json:"enabled"`
	Version          int64  `json:"version"`
	ModelName        string `json:"model_name"`
	InputPerMillion  string `json:"input_per_million"`
	OutputPerMillion string `json:"output_per_million"`
	MaxRatio         string `json:"max_ratio"`
	QuotaPerCNY      string `json:"quota_per_cny"`
	Reason           string `json:"reason,omitempty"`
}

// CaptureUsageCashback fixes the offer before the first upstream attempt,
// including requests that initially use a free group or a subscription.
func CaptureUsageCashback(c *gin.Context, info *relaycommon.RelayInfo, request dto.Request) error {
	if _, exists := c.Get(cashbackSnapshotContextKey); exists {
		return nil
	}
	snapshot := &usageCashbackSnapshot{ModelName: info.OriginModelName}
	switch info.RelayFormat {
	case types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses, types.RelayFormatClaude, types.RelayFormatGemini:
	default:
		c.Set(cashbackSnapshotContextKey, snapshot)
		return nil
	}
	if info.PriceData.UsePrice || info.IsChannelTest {
		c.Set(cashbackSnapshotContextKey, snapshot)
		return nil
	}
	settings, err := model.GetUsageCashbackSettings()
	if err != nil {
		return fmt.Errorf("load usage cashback rules: %w", err)
	}
	snapshot.Version = settings.Version
	rule := settings.Models[info.OriginModelName]
	if !settings.Enabled || !rule.Enabled {
		c.Set(cashbackSnapshotContextKey, snapshot)
		return nil
	}
	if common.BatchUpdateEnabled {
		return errors.New("usage cashback requires durable billing: disable batch updates before accepting requests")
	}
	for _, value := range []float64{common.QuotaPerUnit, operation_setting.USDExchangeRate} {
		if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return errors.New("invalid usage cashback currency conversion")
		}
	}
	snapshot.Enabled = true
	snapshot.InputPerMillion = rule.InputPerMillion
	snapshot.OutputPerMillion = rule.OutputPerMillion
	snapshot.MaxRatio = settings.MaxRatio
	snapshot.QuotaPerCNY = decimal.NewFromFloat(common.QuotaPerUnit).
		DivRound(decimal.NewFromFloat(operation_setting.USDExchangeRate), 32).String()
	if request != nil {
		meta := request.GetTokenCountMeta()
		if meta != nil && len(meta.Files) > 0 {
			snapshot.Reason = "unsupported_usage"
		}
		// Output modalities are not represented by input file metadata. Exclude
		// them even when an upstream omits the corresponding token breakdown.
		switch typed := request.(type) {
		case *dto.GeneralOpenAIRequest:
			var modalities []string
			if len(typed.Modalities) > 0 {
				if err := common.Unmarshal(typed.Modalities, &modalities); err != nil {
					snapshot.Reason = "unsupported_usage"
				}
			}
			for _, modality := range modalities {
				if !strings.EqualFold(modality, "text") {
					snapshot.Reason = "unsupported_usage"
				}
			}
			if len(typed.Audio) > 0 && string(typed.Audio) != "null" {
				snapshot.Reason = "unsupported_usage"
			}
		case *dto.GeminiChatRequest:
			for _, modality := range typed.GenerationConfig.ResponseModalities {
				if !strings.EqualFold(modality, "text") {
					snapshot.Reason = "unsupported_usage"
				}
			}
			if len(typed.GenerationConfig.ImageConfig) > 0 || len(typed.GenerationConfig.SpeechConfig) > 0 {
				snapshot.Reason = "unsupported_usage"
			}
		case *dto.OpenAIResponsesRequest:
			var tools []struct {
				Type string `json:"type"`
			}
			if len(typed.Tools) > 0 {
				if err := common.Unmarshal(typed.Tools, &tools); err != nil {
					snapshot.Reason = "unsupported_usage"
				}
			}
			for _, tool := range tools {
				if tool.Type == "image_generation" {
					snapshot.Reason = "unsupported_usage"
				}
			}
		}
	}
	c.Set(cashbackSnapshotContextKey, snapshot)
	return nil
}

type cashbackBillingSession struct {
	mu       sync.Mutex
	info     *relaycommon.RelayInfo
	snapshot usageCashbackSnapshot
	usage    *model.CashbackUsage
	plan     *model.CashbackSettlementPlan
}

func newCashbackBillingSession(c *gin.Context, info *relaycommon.RelayInfo, quota int) (relaycommon.BillingSettler, *types.NewAPIError) {
	if c == nil {
		return nil, nil
	}
	value, exists := c.Get(cashbackSnapshotContextKey)
	if !exists {
		return nil, nil
	}
	snapshot, ok := value.(*usageCashbackSnapshot)
	if !ok || !snapshot.Enabled {
		return nil, nil
	}
	if common.BatchUpdateEnabled {
		return nil, types.NewError(errors.New("cashback_requires_durable_billing"), types.ErrorCodeUpdateDataError, types.ErrOptionWithSkipRetry())
	}
	data, err := common.Marshal(snapshot)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeUpdateDataError, types.ErrOptionWithSkipRetry())
	}
	tokenID := info.TokenId
	if info.IsPlayground {
		tokenID = 0
	}
	usage, err := model.BeginCashbackUsage(model.CashbackUsage{
		ID: common.GetUUID(), RequestID: info.RequestId, UserID: info.UserId,
		TokenID: tokenID, ModelName: snapshot.ModelName, Snapshot: string(data), ReservedQuota: quota,
	})
	if err != nil {
		if errors.Is(err, model.ErrInsufficientQuota) {
			return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInsufficientUserQuota, http.StatusForbidden, types.ErrOptionWithSkipRetry())
		}
		return nil, types.NewError(err, types.ErrorCodeUpdateDataError, types.ErrOptionWithSkipRetry())
	}
	info.BillingSource = BillingSourceWallet
	info.FinalPreConsumedQuota = usage.ReservedQuota
	info.SubscriptionId = 0
	info.SubscriptionPreConsumed = 0
	return &cashbackBillingSession{info: info, snapshot: *snapshot, usage: usage}, nil
}

func (s *cashbackBillingSession) GetPreConsumedQuota() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.usage.ReservedQuota
}

func (s *cashbackBillingSession) Reserve(target int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	usage, err := model.ReserveCashbackUsage(s.usage.ID, target)
	if err != nil {
		return err
	}
	s.usage = usage
	s.info.FinalPreConsumedQuota = usage.ReservedQuota
	return nil
}

func (s *cashbackBillingSession) NeedsRefund() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.plan == nil && s.usage.State == model.CashbackStateReserved
}

func (s *cashbackBillingSession) Refund(c *gin.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// An accepted settlement intent must never fall back to refunding its
	// reservation, even if the funding adjustment has not succeeded yet.
	if s.plan != nil {
		return
	}
	usage, err := model.CancelCashbackUsage(s.usage.ID)
	if err != nil {
		logger.LogError(c, "cancel cashback reservation: "+err.Error())
		return
	}
	s.usage = usage
}

func (s *cashbackBillingSession) Settle(actualQuota int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.plan == nil {
		// Unknown provider outcomes and non-text settlement paths cannot earn
		// rewards merely because a reservation was retained as a charge.
		s.plan = &model.CashbackSettlementPlan{ActualQuota: actualQuota, Reason: "unknown_usage"}
	}
	if s.plan.ActualQuota != actualQuota {
		return errors.New("cashback settlement amount changed after final usage")
	}
	usage, err := model.PlanCashbackSettlement(s.usage.ID, *s.plan)
	if err != nil {
		return err
	}
	s.usage = usage
	usage, err = model.SettleCashbackUsage(usage.ID)
	if err != nil {
		return err
	}
	s.usage = usage
	// Failure here must not undo a committed consumption. The durable obligation
	// remains pending and the control-plane worker retries without a new request.
	if credited, err := model.CreditCashbackUsage(usage.ID); err != nil {
		common.SysError("credit usage cashback " + usage.ID + ": " + err.Error())
	} else {
		s.usage = credited
	}
	return nil
}

func prepareUsageCashback(c *gin.Context, info *relaycommon.RelayInfo, usage *dto.Usage, actualQuota int, tieredFallback bool) {
	session, ok := info.Billing.(*cashbackBillingSession)
	if !ok {
		return
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.plan != nil {
		return
	}
	plan := model.CashbackSettlementPlan{ActualQuota: actualQuota}
	switch {
	case session.snapshot.Reason != "":
		plan.Reason = session.snapshot.Reason
	case info.QuotaClamp != nil:
		plan.Reason = "quota_saturation"
	case info.BalanceProtectionUsageUnknown:
		plan.Reason = "unknown_usage"
	case tieredFallback || common.GetContextKeyBool(c, constant.ContextKeyLocalCountTokens):
		plan.Reason = "estimated_usage"
	case actualQuota == 0:
		plan.Reason = "zero_charge"
	default:
		plan.InputTokens, plan.OutputTokens, plan.UsageSource, plan.Reason = cashbackTokenUsage(usage)
		if plan.Reason == "" {
			amount, capped, err := calculateUsageCashback(session.snapshot, plan.InputTokens, plan.OutputTokens, actualQuota)
			if err != nil {
				plan.Reason = "invalid_cashback_calculation"
				logger.LogError(c, "calculate usage cashback: "+err.Error())
			} else {
				plan.OriginalQuota = amount
				if amount == 0 {
					plan.Reason = "below_minimum"
				} else if capped {
					plan.Reason = "capped"
				}
			}
		}
	}
	session.plan = &plan
}

// cashbackTokenUsage deliberately does not use expression P/C: their contents
// depend on which subcategories the billing expression separately prices.
func cashbackTokenUsage(usage *dto.Usage) (int64, int64, string, string) {
	if usage == nil || usage.BillingUsage == nil {
		return 0, 0, "", "unknown_usage"
	}
	billing := usage.BillingUsage
	if billing.Estimated {
		return 0, 0, billing.Source, "estimated_usage"
	}
	var input, output, cached, written int64
	switch billing.Source {
	case dto.BillingUsageSourceOAIChat, dto.BillingUsageSourceOAIResponses:
		raw := billing.OpenAIUsage
		if billing.Semantic != dto.BillingUsageSemanticOpenAI || raw == nil {
			return 0, 0, billing.Source, "invalid_usage"
		}
		if raw.PromptTokens < 0 || raw.InputTokens < 0 || raw.CompletionTokens < 0 || raw.OutputTokens < 0 || raw.TotalTokens < 0 {
			return 0, 0, billing.Source, "invalid_usage"
		}
		input, output = int64(raw.PromptTokens), int64(raw.CompletionTokens)
		if input == 0 {
			input = int64(raw.InputTokens)
		} else if raw.InputTokens > 0 && int64(raw.InputTokens) != input {
			return 0, 0, billing.Source, "invalid_usage"
		}
		if output == 0 {
			output = int64(raw.OutputTokens)
		} else if raw.OutputTokens > 0 && int64(raw.OutputTokens) != output {
			return 0, 0, billing.Source, "invalid_usage"
		}
		details := raw.PromptTokensDetails
		if raw.InputTokensDetails != nil {
			if details != (dto.InputTokenDetails{}) && details != *raw.InputTokensDetails {
				return 0, 0, billing.Source, "invalid_usage"
			}
			details = *raw.InputTokensDetails
		}
		if details.CachedTokens < 0 || details.CachedCreationTokens < 0 || details.CacheWriteTokens < 0 || raw.PromptCacheHitTokens < 0 || raw.CompletionTokenDetails.ReasoningTokens < 0 {
			return 0, 0, billing.Source, "invalid_usage"
		}
		if details.ImageTokens != 0 || details.AudioTokens != 0 || raw.CompletionTokenDetails.ImageTokens != 0 || raw.CompletionTokenDetails.AudioTokens != 0 {
			return 0, 0, billing.Source, "unsupported_usage"
		}
		cached = int64(details.CachedTokens)
		if raw.PromptCacheHitTokens > 0 {
			if cached > 0 && cached != int64(raw.PromptCacheHitTokens) {
				return 0, 0, billing.Source, "invalid_usage"
			}
			cached = int64(raw.PromptCacheHitTokens)
		}
		written = int64(details.CacheCreationTokensTotal())
		if cached > input || written > input {
			return 0, 0, billing.Source, "invalid_usage"
		}
		// Native OpenAI cache-write prefixes can overlap the read prefix.
		// Match billing's ordinary-input floor without discarding valid output.
		input = max(0, input-cached-written)
	case dto.BillingUsageSourceClaudeMessages:
		raw := billing.ClaudeUsage
		if billing.Semantic != dto.BillingUsageSemanticAnthropic || raw == nil {
			return 0, 0, billing.Source, "invalid_usage"
		}
		if raw.InputTokens < 0 || raw.OutputTokens < 0 || raw.CacheReadInputTokens < 0 || raw.CacheCreationInputTokens < 0 || raw.ClaudeCacheCreation5mTokens < 0 || raw.ClaudeCacheCreation1hTokens < 0 {
			return 0, 0, billing.Source, "invalid_usage"
		}
		if raw.CacheCreation != nil && (raw.CacheCreation.Ephemeral5mInputTokens < 0 || raw.CacheCreation.Ephemeral1hInputTokens < 0) {
			return 0, 0, billing.Source, "invalid_usage"
		}
		// Native Anthropic input_tokens already excludes both cache categories.
		input, output = int64(raw.InputTokens), int64(raw.OutputTokens)
	case dto.BillingUsageSourceGeminiChat:
		raw := billing.GeminiUsageMetadata
		if billing.Semantic != dto.BillingUsageSemanticGemini || raw == nil {
			return 0, 0, billing.Source, "invalid_usage"
		}
		for _, count := range []int{raw.PromptTokenCount, raw.ToolUsePromptTokenCount, raw.CandidatesTokenCount, raw.ThoughtsTokenCount, raw.CachedContentTokenCount, raw.TotalTokenCount} {
			if count < 0 || count > common.MaxQuota {
				return 0, 0, billing.Source, "invalid_usage"
			}
		}
		for _, group := range [][]dto.GeminiPromptTokensDetails{raw.PromptTokensDetails, raw.ToolUsePromptTokensDetails, raw.CandidatesTokensDetails} {
			for _, detail := range group {
				if detail.TokenCount < 0 {
					return 0, 0, billing.Source, "invalid_usage"
				}
				if detail.TokenCount > 0 && detail.Modality != "TEXT" {
					return 0, 0, billing.Source, "unsupported_usage"
				}
			}
		}
		input = int64(raw.PromptTokenCount) + int64(raw.ToolUsePromptTokenCount)
		output = int64(raw.CandidatesTokenCount) + int64(raw.ThoughtsTokenCount)
		cached = int64(raw.CachedContentTokenCount)
		if cached > input || (output == 0 && int64(raw.TotalTokenCount) > input) {
			return 0, 0, billing.Source, "invalid_usage"
		}
		input -= cached
	default:
		return 0, 0, billing.Source, "unknown_usage"
	}
	if input < 0 || output < 0 || input > common.MaxQuota || output > common.MaxQuota {
		return 0, 0, billing.Source, "invalid_usage"
	}
	return input, output, billing.Source, ""
}

func calculateUsageCashback(snapshot usageCashbackSnapshot, input, output int64, actualQuota int) (int, bool, error) {
	if input < 0 || output < 0 || input > common.MaxQuota || output > common.MaxQuota || actualQuota < 0 || actualQuota > common.MaxQuota {
		return 0, false, errors.New("invalid cashback quantities")
	}
	ri, err := decimal.NewFromString(snapshot.InputPerMillion)
	if err != nil || ri.IsNegative() {
		return 0, false, errors.New("invalid input cashback rate")
	}
	ro, err := decimal.NewFromString(snapshot.OutputPerMillion)
	if err != nil || ro.IsNegative() {
		return 0, false, errors.New("invalid output cashback rate")
	}
	k, err := decimal.NewFromString(snapshot.QuotaPerCNY)
	if err != nil || !k.IsPositive() {
		return 0, false, errors.New("invalid cashback currency conversion")
	}
	x, err := decimal.NewFromString(snapshot.MaxRatio)
	if err != nil || !x.IsPositive() || !x.LessThan(decimal.NewFromInt(1)) {
		return 0, false, errors.New("invalid cashback cap")
	}
	base := decimal.NewFromInt(input).Mul(ri).Add(decimal.NewFromInt(output).Mul(ro)).
		Mul(k).Shift(-6)
	cap := decimal.NewFromInt(int64(actualQuota)).Mul(x)
	quota, clamp := common.QuotaFromDecimalChecked(decimal.Min(base, cap).Floor())
	if clamp != nil {
		return 0, false, clamp
	}
	return quota, base.GreaterThan(cap), nil
}
