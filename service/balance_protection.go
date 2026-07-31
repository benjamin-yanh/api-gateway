package service

import (
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	relaykittypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

type balanceProtectionPrice struct {
	modelName           string
	outputQuotaPerToken float64
	fixedQuota          float64
}

// ApplyBalanceProtection limits the effective output budget when the user's
// remaining finite quota enters the configured "last N output tokens" zone.
// It runs after prompt estimation and before pricing, so pre-consume and the
// outbound request observe the same max-token value.
func ApplyBalanceProtection(info *relaycommon.RelayInfo, request dto.Request, promptTokens int, meta *relaykittypes.TokenCountMeta) error {
	cfg := operation_setting.GetQuotaSetting()
	if !cfg.BalanceProtectionEnabled || info == nil || request == nil || meta == nil {
		return nil
	}
	if promptTokens < 0 {
		return fmt.Errorf("estimated prompt tokens cannot be negative")
	}
	switch info.RelayFormat {
	case relaykittypes.RelayFormatOpenAIRealtime, relaykittypes.RelayFormatOpenAIResponsesCompaction:
		return fmt.Errorf("balance protection does not yet support relay format %s", info.RelayFormat)
	}
	if cfg.BalanceProtectionMinimumOutput == 0 ||
		cfg.BalanceProtectionMaximumOutput < cfg.BalanceProtectionMinimumOutput ||
		cfg.BalanceProtectionSafetyRatio < 1 ||
		math.IsNaN(cfg.BalanceProtectionSafetyRatio) ||
		math.IsInf(cfg.BalanceProtectionSafetyRatio, 0) {
		return fmt.Errorf("invalid balance protection configuration")
	}
	thresholdTokens, err := cfg.BalanceProtectionThresholdTokens()
	if err != nil {
		return fmt.Errorf("invalid balance protection configuration: %w", err)
	}

	clientMax, supported := requestOutputLimit(request)
	if !supported {
		return nil
	}
	if common.BatchUpdateEnabled {
		return fmt.Errorf("balance protection requires BatchUpdateEnabled=false")
	}
	policyMax := cfg.BalanceProtectionMaximumOutput
	if claudeRequest, ok := request.(*dto.ClaudeRequest); ok && clientMax == 0 {
		defaultMax := model_setting.GetClaudeSettings().GetDefaultMaxTokens(claudeRequest.Model)
		if defaultMax > 0 && uint(defaultMax) < policyMax {
			policyMax = uint(defaultMax)
		}
	}

	availableQuota, err := finiteAvailableQuota(info)
	if err != nil {
		return err
	}
	thresholdPrice, err := mostExpensiveOutputPrice(info, thresholdTokens)
	if err != nil {
		return err
	}
	thresholdQuotaFloat := float64(thresholdTokens) *
		thresholdPrice.outputQuotaPerToken * cfg.BalanceProtectionSafetyRatio
	if thresholdPrice.fixedQuota > thresholdQuotaFloat {
		thresholdQuotaFloat = thresholdPrice.fixedQuota * cfg.BalanceProtectionSafetyRatio
	}
	thresholdQuota, err := common.QuotaFromFloatStrict(thresholdQuotaFloat)
	if err != nil {
		return fmt.Errorf("calculate balance protection threshold: %w", err)
	}
	if thresholdQuota <= 0 {
		return nil
	}

	currentPrice, err := outputPriceForModel(info.OriginModelName, info.UserGroup, info.UsingGroup)
	if err != nil {
		return err
	}
	prospectiveMax := policyMax
	if clientMax > 0 && clientMax < prospectiveMax {
		prospectiveMax = clientMax
	}

	estimatedInputQuota := 0
	requestWorstQuota := 0
	if currentPrice.fixedQuota > 0 {
		requestWorstQuota, err = common.QuotaFromFloatStrict(currentPrice.fixedQuota * cfg.BalanceProtectionSafetyRatio)
		if err != nil {
			return err
		}
	} else if currentPrice.outputQuotaPerToken > 0 {
		modelRatio, ok, matchedName := ratio_setting.GetModelRatio(info.OriginModelName)
		tiered := billing_setting.GetBillingMode(info.OriginModelName) == billing_setting.BillingModeTieredExpr
		if !ok && !tiered {
			return fmt.Errorf("模型 %s 的输入价格未配置", matchedName)
		}
		inputQuotaPerToken := modelRatio * GetUserGroupRatio(info.UserGroup, info.UsingGroup)
		if tiered {
			inputQuotaPerToken = currentPrice.outputQuotaPerToken
		}
		estimatedInputQuota, err = common.QuotaFromFloatStrict(float64(promptTokens) * inputQuotaPerToken)
		if err != nil {
			return err
		}
		outputWorst, err := common.QuotaFromFloatStrict(
			float64(prospectiveMax) * currentPrice.outputQuotaPerToken * cfg.BalanceProtectionSafetyRatio,
		)
		if err != nil {
			return err
		}
		requestWorstQuota = estimatedInputQuota + outputWorst
	}

	// Look ahead by one worst-case request. Without this guard, a request that
	// starts one quota above the threshold could cross the entire protection
	// zone before the next request gets a chance to activate it.
	if availableQuota > thresholdQuota+requestWorstQuota {
		return nil
	}

	budgetMax := policyMax

	if currentPrice.fixedQuota > 0 {
		if availableQuota < requestWorstQuota {
			return fmt.Errorf("余额不足以覆盖模型 %s 的固定价格预留", info.OriginModelName)
		}
	} else if currentPrice.outputQuotaPerToken > 0 {
		remaining := availableQuota - estimatedInputQuota
		if remaining <= 0 {
			return fmt.Errorf("余额不足以覆盖输入 Token")
		}
		affordable := math.Floor(float64(remaining) / (currentPrice.outputQuotaPerToken * cfg.BalanceProtectionSafetyRatio))
		if affordable < float64(budgetMax) {
			budgetMax = uint(affordable)
		}
	}

	if budgetMax < cfg.BalanceProtectionMinimumOutput {
		return fmt.Errorf("余额只允许最多 %d 个输出 Token，低于站点最小值 %d", budgetMax, cfg.BalanceProtectionMinimumOutput)
	}
	effectiveMax := budgetMax
	if clientMax > 0 && clientMax < effectiveMax {
		effectiveMax = clientMax
	}

	applyRequestOutputLimit(request, effectiveMax)
	meta.MaxTokens = int(effectiveMax)
	info.BalanceProtectionActive = true
	info.EffectiveMaxOutputTokens = effectiveMax
	info.BalanceProtectionThresholdQuota = thresholdQuota
	info.BalanceProtectionAvailableQuota = availableQuota
	info.BalanceProtectionMostExpensiveModel = thresholdPrice.modelName
	info.ForcePreConsume = true
	return nil
}

func finiteAvailableQuota(info *relaycommon.RelayInfo) (int, error) {
	userQuota, err := model.GetUserQuota(info.UserId, true)
	if err != nil {
		return 0, fmt.Errorf("query user quota: %w", err)
	}
	subscriptions, err := model.GetAllActiveUserSubscriptions(info.UserId)
	if err != nil {
		return 0, fmt.Errorf("query subscription quota: %w", err)
	}
	subscriptionQuota := 0
	for _, summary := range subscriptions {
		if summary.Subscription == nil {
			continue
		}
		if summary.Subscription.AmountTotal == 0 {
			subscriptionQuota = int(^uint32(0) >> 1)
			break
		}
		remaining := summary.Subscription.AmountTotal - summary.Subscription.AmountUsed
		if remaining > int64(subscriptionQuota) {
			if remaining > int64(^uint32(0)>>1) {
				subscriptionQuota = int(^uint32(0) >> 1)
			} else {
				subscriptionQuota = int(remaining)
			}
		}
	}

	available := userQuota
	switch common.NormalizeBillingPreference(info.UserSetting.BillingPreference) {
	case "subscription_only":
		available = subscriptionQuota
	case "wallet_only":
		available = userQuota
	case "wallet_first":
		if subscriptionQuota > available {
			available = subscriptionQuota
		}
	case "subscription_first":
		if len(subscriptions) == 0 {
			available = userQuota
			break
		}
		available = subscriptionQuota
		allowOverflow, err := model.UserActiveSubscriptionsAllowWalletOverflow(info.UserId)
		if err != nil {
			return 0, fmt.Errorf("query subscription wallet overflow policy: %w", err)
		}
		if allowOverflow && userQuota > available {
			available = userQuota
		}
	default:
		if subscriptionQuota > available {
			available = subscriptionQuota
		}
	}
	if !info.TokenUnlimited {
		token, err := model.GetTokenByKey(info.TokenKey, true)
		if err != nil {
			return 0, fmt.Errorf("query token quota: %w", err)
		}
		if token.RemainQuota < available {
			available = token.RemainQuota
		}
	}
	return available, nil
}

func mostExpensiveOutputPrice(info *relaycommon.RelayInfo, thresholdTokens int64) (balanceProtectionPrice, error) {
	groupMap := GetUserUsableGroups(info.UserGroup)
	groups := make([]string, 0, len(groupMap))
	for group := range groupMap {
		groups = append(groups, group)
	}
	pairs, err := model.GetEnabledGroupModels(groups)
	if err != nil {
		return balanceProtectionPrice{}, fmt.Errorf("query enabled models: %w", err)
	}
	var highest balanceProtectionPrice
	for _, pair := range pairs {
		price, err := outputPriceForModel(pair.Model, info.UserGroup, pair.Group)
		if err != nil {
			return balanceProtectionPrice{}, err
		}
		candidate := price.outputQuotaPerToken * float64(thresholdTokens)
		if price.fixedQuota > candidate {
			candidate = price.fixedQuota
		}
		current := highest.outputQuotaPerToken * float64(thresholdTokens)
		if highest.fixedQuota > current {
			current = highest.fixedQuota
		}
		if candidate > current {
			highest = price
		}
	}
	return highest, nil
}

func outputPriceForModel(modelName, userGroup, channelGroup string) (balanceProtectionPrice, error) {
	groupRatio := GetUserGroupRatio(userGroup, channelGroup)
	if billing_setting.GetBillingMode(modelName) == billing_setting.BillingModeTieredExpr {
		ceilings := operation_setting.GetQuotaSetting().BalanceProtectionTieredPriceCeilings
		if ceilings == nil {
			return balanceProtectionPrice{}, fmt.Errorf("balance protection tiered price ceilings is not configured")
		}
		ceiling, ok := ceilings.Get(modelName)
		if !ok {
			return balanceProtectionPrice{}, fmt.Errorf("tiered_expr 模型 %s 缺少 balance protection price ceiling", modelName)
		}
		return balanceProtectionPrice{
			modelName:           modelName,
			outputQuotaPerToken: ceiling * common.QuotaPerUnit * groupRatio / 1_000_000,
		}, nil
	}
	if modelPrice, usePrice := ratio_setting.GetModelPrice(modelName, false); usePrice {
		return balanceProtectionPrice{
			modelName:  modelName,
			fixedQuota: modelPrice * common.QuotaPerUnit * groupRatio,
		}, nil
	}
	modelRatio, ok, matchedName := ratio_setting.GetModelRatio(modelName)
	if !ok {
		return balanceProtectionPrice{}, fmt.Errorf("模型 %s 的倍率未配置", matchedName)
	}
	return balanceProtectionPrice{
		modelName:           modelName,
		outputQuotaPerToken: modelRatio * ratio_setting.GetCompletionRatio(modelName) * groupRatio,
	}, nil
}

func requestOutputLimit(request dto.Request) (uint, bool) {
	switch req := request.(type) {
	case *dto.GeneralOpenAIRequest:
		return req.GetMaxTokens(), true
	case *dto.OpenAIResponsesRequest:
		if req.MaxOutputTokens != nil {
			return *req.MaxOutputTokens, true
		}
		return 0, true
	case *dto.ClaudeRequest:
		if req.MaxTokens != nil {
			return *req.MaxTokens, true
		}
		if req.MaxTokensToSample != nil {
			return *req.MaxTokensToSample, true
		}
		return 0, true
	case *dto.GeminiChatRequest:
		var maximum uint
		for i := range req.Requests {
			if req.Requests[i].GenerationConfig.MaxOutputTokens != nil &&
				*req.Requests[i].GenerationConfig.MaxOutputTokens > maximum {
				maximum = *req.Requests[i].GenerationConfig.MaxOutputTokens
			}
		}
		if req.GenerationConfig.MaxOutputTokens != nil {
			if *req.GenerationConfig.MaxOutputTokens > maximum {
				maximum = *req.GenerationConfig.MaxOutputTokens
			}
		}
		return maximum, true
	default:
		return 0, false
	}
}

func applyRequestOutputLimit(request dto.Request, limit uint) {
	switch req := request.(type) {
	case *dto.GeneralOpenAIRequest:
		if req.MaxTokens != nil {
			req.MaxTokens = common.GetPointer(limit)
		}
		if req.MaxCompletionTokens != nil {
			req.MaxCompletionTokens = common.GetPointer(limit)
		}
		if req.MaxTokens == nil && req.MaxCompletionTokens == nil {
			req.MaxTokens = common.GetPointer(limit)
		}
	case *dto.OpenAIResponsesRequest:
		req.MaxOutputTokens = common.GetPointer(limit)
	case *dto.ClaudeRequest:
		req.MaxTokens = common.GetPointer(limit)
		req.MaxTokensToSample = nil
	case *dto.GeminiChatRequest:
		if len(req.Requests) == 0 || len(req.Contents) > 0 || req.GenerationConfig.MaxOutputTokens != nil {
			req.GenerationConfig.MaxOutputTokens = common.GetPointer(limit)
		}
		for i := range req.Requests {
			req.Requests[i].GenerationConfig.MaxOutputTokens = common.GetPointer(limit)
		}
	}
}
