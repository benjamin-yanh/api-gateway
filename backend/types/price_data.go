package types

import (
	"fmt"
	"math"

	"github.com/shopspring/decimal"
)

type GroupRatioInfo struct {
	// GroupRatio 是本次请求实际生效的分组倍率。
	GroupRatio float64
	// GroupSpecialRatio 是用户分组到渠道分组的专属倍率覆盖值。
	GroupSpecialRatio float64
	// HasSpecialRatio 表示是否以 GroupSpecialRatio 覆盖普通分组倍率。
	HasSpecialRatio bool
}

type PriceData struct {
	// FreeModel 表示本次请求的有效价格为免费，应跳过预扣。
	FreeModel bool
	// ModelPrice 是固定价格计费值，结合 QuotaPerUnit 转换为额度。
	ModelPrice float64
	// ModelRatio 是按量计费时的每 Token 模型倍率。
	ModelRatio float64
	// CompletionRatio 是输出文本 Token 相对输入文本 Token 的倍率。
	CompletionRatio float64
	// CacheRatio 是缓存命中 Token 相对普通输入 Token 的倍率。
	CacheRatio float64
	// CacheCreationRatio 是默认缓存时长的缓存写入 Token 倍率。
	CacheCreationRatio float64
	// CacheCreation5mRatio 是 Claude 5 分钟缓存写入 Token 倍率。
	CacheCreation5mRatio float64
	// CacheCreation1hRatio 是 Claude 1 小时缓存写入 Token 倍率。
	CacheCreation1hRatio float64
	// ImageRatio 是多模态输入中图片 Token 的倍率。
	ImageRatio float64
	// AudioRatio 是音频输入 Token 的倍率。
	AudioRatio float64
	// AudioCompletionRatio 是音频输出 Token 在 AudioRatio 基础上的额外倍率。
	AudioCompletionRatio float64
	// otherRatios 保存已校验的附加乘数，例如任务媒体选项；只能通过 AddOtherRatio 写入。
	otherRatios map[string]float64
	// UsePrice 表示使用固定价格计费，而不是按 Token 倍率计费。
	UsePrice bool
	// Quota 是按次/任务计费应用分组和附加倍率后的额度。
	Quota int
	// QuotaToPreConsume 是按量请求在调用上游前需要预扣的额度。
	QuotaToPreConsume int
	// GroupRatioInfo 记录本次计费使用的分组倍率及用户分组专属覆盖信息。
	GroupRatioInfo GroupRatioInfo
}

func (p *PriceData) AddOtherRatio(key string, ratio float64) {
	if !isValidOtherRatio(ratio) {
		return
	}
	if p.otherRatios == nil {
		p.otherRatios = make(map[string]float64)
	}
	p.otherRatios[key] = ratio
}

func (p *PriceData) ReplaceOtherRatios(ratios map[string]float64) bool {
	p.otherRatios = nil
	for key, ratio := range ratios {
		p.AddOtherRatio(key, ratio)
	}
	return len(p.otherRatios) > 0
}

func (p *PriceData) HasOtherRatio(key string) bool {
	ratio, ok := p.otherRatios[key]
	return ok && isValidOtherRatio(ratio)
}

func (p *PriceData) OtherRatios() map[string]float64 {
	if len(p.otherRatios) == 0 {
		return nil
	}
	ratios := make(map[string]float64, len(p.otherRatios))
	for key, ratio := range p.otherRatios {
		if isValidOtherRatio(ratio) {
			ratios[key] = ratio
		}
	}
	if len(ratios) == 0 {
		return nil
	}
	return ratios
}

func (p *PriceData) OtherRatioMultiplier() float64 {
	multiplier := 1.0
	for _, ratio := range p.otherRatios {
		if isValidOtherRatio(ratio) && ratio != 1.0 {
			multiplier *= ratio
		}
	}
	return multiplier
}

func (p *PriceData) ApplyOtherRatiosToFloat(value float64) float64 {
	return value * p.OtherRatioMultiplier()
}

func (p *PriceData) ApplyOtherRatiosToDecimal(value decimal.Decimal) decimal.Decimal {
	for _, ratio := range p.otherRatios {
		if isValidOtherRatio(ratio) && ratio != 1.0 {
			value = value.Mul(decimal.NewFromFloat(ratio))
		}
	}
	return value
}

func (p *PriceData) RemoveOtherRatiosFromFloat(value float64) float64 {
	for _, ratio := range p.otherRatios {
		if isValidOtherRatio(ratio) && ratio != 1.0 {
			value /= ratio
		}
	}
	return value
}

func isValidOtherRatio(ratio float64) bool {
	return ratio > 0 && !math.IsInf(ratio, 1)
}

func (p *PriceData) ToSetting() string {
	return fmt.Sprintf("ModelPrice: %f, ModelRatio: %f, CompletionRatio: %f, CacheRatio: %f, GroupRatio: %f, UsePrice: %t, CacheCreationRatio: %f, CacheCreation5mRatio: %f, CacheCreation1hRatio: %f, QuotaToPreConsume: %d, ImageRatio: %f, AudioRatio: %f, AudioCompletionRatio: %f", p.ModelPrice, p.ModelRatio, p.CompletionRatio, p.CacheRatio, p.GroupRatioInfo.GroupRatio, p.UsePrice, p.CacheCreationRatio, p.CacheCreation5mRatio, p.CacheCreation1hRatio, p.QuotaToPreConsume, p.ImageRatio, p.AudioRatio, p.AudioCompletionRatio)
}
