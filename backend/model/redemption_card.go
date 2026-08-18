package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"gorm.io/gorm"
)

const (
	RedemptionCardGroup3RMB   = "3_RMB_CARD"
	RedemptionCardGroup10RMB  = "10_RMB_CARD"
	RedemptionCardGroup50RMB  = "50_RMB_CARD"
	RedemptionCardGroup100RMB = "100_RMB_CARD"
	RedemptionCardGroup200RMB = "200_RMB_CARD"
)

var redemptionCardAmounts = map[string]int{
	RedemptionCardGroup3RMB:   3,
	RedemptionCardGroup10RMB:  10,
	RedemptionCardGroup50RMB:  50,
	RedemptionCardGroup100RMB: 100,
	RedemptionCardGroup200RMB: 200,
}

type RedemptionCard struct {
	Id           uint64 `json:"id"`
	Key          string `json:"-" gorm:"type:char(24);uniqueIndex;not null"`
	Group        string `json:"group" gorm:"column:group;type:varchar(32);index;not null"`
	Quota        int    `json:"quota" gorm:"not null"`
	Status       int    `json:"status" gorm:"not null"`
	CreatedTime  int64  `json:"created_time" gorm:"bigint;not null"`
	RedeemedTime int64  `json:"redeemed_time" gorm:"bigint;not null"`
	UsedUserId   int    `json:"used_user_id" gorm:"index;not null"`
}

type RedemptionCardHistory struct {
	Id           uint64 `json:"id"`
	Group        string `json:"group"`
	AmountRMB    int    `json:"amount_rmb"`
	Quota        int    `json:"quota"`
	RedeemedTime int64  `json:"redeemed_time"`
}

func GetRecentRedemptionCardHistory(userID int, limit int) ([]RedemptionCardHistory, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	if limit <= 0 || limit > 10 {
		limit = 10
	}

	var cards []RedemptionCard
	err := DB.Select("id", commonGroupCol, "quota", "redeemed_time").
		Where("used_user_id = ? AND status = ?", userID, common.RedemptionCodeStatusUsed).
		Order("redeemed_time DESC, id DESC").
		Limit(limit).
		Find(&cards).Error
	if err != nil {
		return nil, err
	}

	history := make([]RedemptionCardHistory, 0, len(cards))
	for _, card := range cards {
		amountRMB, valid := redemptionCardAmounts[card.Group]
		if !valid {
			continue
		}
		history = append(history, RedemptionCardHistory{
			Id:           card.Id,
			Group:        card.Group,
			AmountRMB:    amountRMB,
			Quota:        card.Quota,
			RedeemedTime: card.RedeemedTime,
		})
	}
	return history, nil
}

func redeemCard(tx *gorm.DB, key string, userID int) (int, error) {
	card := &RedemptionCard{}
	if err := lockForUpdate(tx).Where(commonKeyCol+" = ?", key).First(card).Error; err != nil {
		return 0, err
	}
	if card.Status != common.RedemptionCodeStatusEnabled {
		return 0, ErrRedeemFailed
	}
	amountRMB, valid := redemptionCardAmounts[card.Group]
	if !valid {
		return 0, errors.New("invalid redemption card group")
	}
	if common.QuotaPerUnit <= 0 || operation_setting.USDExchangeRate <= 0 {
		return 0, errors.New("invalid quota conversion settings")
	}
	quota, err := common.QuotaRoundStrict(
		float64(amountRMB) * common.QuotaPerUnit / operation_setting.USDExchangeRate,
	)
	if err != nil || quota <= 0 {
		return 0, errors.New("invalid redemption card quota")
	}

	result := tx.Model(&RedemptionCard{}).
		Where("id = ? AND status = ?", card.Id, common.RedemptionCodeStatusEnabled).
		Updates(map[string]interface{}{
			"quota":         quota,
			"redeemed_time": common.GetTimestamp(),
			"status":        common.RedemptionCodeStatusUsed,
			"used_user_id":  userID,
		})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected != 1 {
		return 0, ErrRedeemFailed
	}
	if err := tx.Model(&User{}).Where("id = ?", userID).
		Update("quota", gorm.Expr("quota + ?", quota)).Error; err != nil {
		return 0, err
	}
	return quota, nil
}
