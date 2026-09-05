package model

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrUsageCashbackSettingsConflict = errors.New("cashback_settings_conflict")

var cashbackAmountPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]{1,8})?$`)
var cashbackRatioPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]{1,6})?$`)

type UsageCashbackModelRule struct {
	Enabled          bool   `json:"enabled"`
	InputPerMillion  string `json:"input_per_million"`
	OutputPerMillion string `json:"output_per_million"`
}

type UsageCashbackSettings struct {
	Version   int64                             `json:"version"`
	Enabled   bool                              `json:"enabled"`
	MaxRatio  string                            `json:"max_ratio"`
	Models    map[string]UsageCashbackModelRule `json:"models"`
	UpdatedBy int                               `json:"updated_by"`
	UpdatedAt int64                             `json:"updated_at"`
}

// UsageCashbackSetting stores the entire active rule document in one main-DB row.
// Relay snapshots deliberately read this row, rather than eventually synced options.
type UsageCashbackSetting struct {
	ID       int    `gorm:"primaryKey"`
	Version  int64  `gorm:"not null"`
	Document string `gorm:"type:text;not null"`
}

// UsageCashbackSettingRevision is immutable audit history committed with the rule.
type UsageCashbackSettingRevision struct {
	Version   int64  `gorm:"primaryKey;autoIncrement:false"`
	UpdatedBy int    `gorm:"not null"`
	UpdatedAt int64  `gorm:"not null"`
	Document  string `gorm:"type:text;not null"`
}

type UsageCashbackModelSupport struct {
	Supported bool   `json:"supported"`
	Reason    string `json:"reason"`
}

func GetUsageCashbackModelSupport() map[string]UsageCashbackModelSupport {
	result := make(map[string]UsageCashbackModelSupport)
	for _, pricing := range GetPricing() {
		support := UsageCashbackModelSupport{Reason: "cashback_text_tokens_only"}
		if pricing.QuotaType == 0 {
			for _, endpoint := range pricing.SupportedEndpointTypes {
				switch endpoint {
				case constant.EndpointTypeOpenAI, constant.EndpointTypeOpenAIResponse, constant.EndpointTypeAnthropic, constant.EndpointTypeGemini:
					support = UsageCashbackModelSupport{Supported: true}
				}
			}
		}
		result[pricing.ModelName] = support
	}
	return result
}

func GetUsageCashbackSettings() (*UsageCashbackSettings, error) {
	var row UsageCashbackSetting
	err := DB.First(&row, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &UsageCashbackSettings{Models: map[string]UsageCashbackModelRule{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var settings UsageCashbackSettings
	if err := common.UnmarshalJsonStr(row.Document, &settings); err != nil {
		return nil, err
	}
	if settings.Version != row.Version {
		return nil, errors.New("cashback_settings_version_mismatch")
	}
	if settings.Models == nil {
		settings.Models = map[string]UsageCashbackModelRule{}
	}
	return &settings, nil
}

func ValidateUsageCashbackSettings(settings *UsageCashbackSettings) error {
	if settings.Version < 0 || settings.Version == math.MaxInt64 || settings.Models == nil || len(settings.Models) > 10000 {
		return errors.New("invalid_cashback_settings")
	}
	if settings.Enabled && common.BatchUpdateEnabled {
		return errors.New("cashback_requires_durable_billing")
	}
	if settings.MaxRatio != "" {
		if len(settings.MaxRatio) > 8 || !cashbackRatioPattern.MatchString(settings.MaxRatio) {
			return errors.New("invalid_cashback_max_ratio")
		}
		ratio, err := decimal.NewFromString(settings.MaxRatio)
		if err != nil || !ratio.IsPositive() || !ratio.LessThan(decimal.NewFromInt(1)) {
			return errors.New("invalid_cashback_max_ratio")
		}
	}
	for name, rule := range settings.Models {
		if strings.TrimSpace(name) != name || name == "" || len(name) > 512 {
			return errors.New("invalid_cashback_model")
		}
		positive := false
		for _, value := range []string{rule.InputPerMillion, rule.OutputPerMillion} {
			if len(value) > 16 || !cashbackAmountPattern.MatchString(value) {
				return fmt.Errorf("invalid_cashback_amount: %s", name)
			}
			amount, err := decimal.NewFromString(value)
			if err != nil || amount.GreaterThan(decimal.NewFromInt(1000000)) {
				return fmt.Errorf("invalid_cashback_amount: %s", name)
			}
			positive = positive || amount.IsPositive()
		}
		if rule.Enabled && !positive {
			return fmt.Errorf("cashback_positive_rate_required: %s", name)
		}
	}
	return nil
}

func SaveUsageCashbackSettings(settings UsageCashbackSettings, userID int) (*UsageCashbackSettings, error) {
	if userID <= 0 {
		return nil, errors.New("invalid_cashback_operator")
	}
	if err := ValidateUsageCashbackSettings(&settings); err != nil {
		return nil, err
	}
	expected := settings.Version
	settings.Version++
	settings.UpdatedBy = userID
	settings.UpdatedAt = common.GetTimestamp()
	document, err := common.Marshal(settings)
	if err != nil {
		return nil, err
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		// Concurrent first saves serialize on the primary key; CAS still decides
		// which caller owns version zero, on SQLite, MySQL and PostgreSQL.
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&UsageCashbackSetting{ID: 1, Document: `{"version":0,"models":{}}`}).Error; err != nil {
			return err
		}
		updated := tx.Model(&UsageCashbackSetting{}).Where("id = ? AND version = ?", 1, expected).
			Updates(map[string]interface{}{"version": settings.Version, "document": string(document)})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrUsageCashbackSettingsConflict
		}
		return tx.Create(&UsageCashbackSettingRevision{Version: settings.Version, UpdatedBy: userID, UpdatedAt: settings.UpdatedAt, Document: string(document)}).Error
	})
	if err != nil {
		return nil, err
	}
	return &settings, nil
}
