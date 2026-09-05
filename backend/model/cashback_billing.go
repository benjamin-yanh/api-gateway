package model

import (
	"errors"
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	CashbackStateReserved  = "reserved"
	CashbackStatePlanned   = "planned"
	CashbackStateSettled   = "settled"
	CashbackStateCancelled = "cancelled"
)

var (
	ErrInvalidCashbackUsage    = errors.New("invalid cashback usage")
	ErrCashbackUsageConflict   = errors.New("cashback usage state changed or conflicting operation")
	ErrCashbackBatchEnabled    = errors.New("cashback billing requires synchronous quota updates")
	ErrCashbackUsagePaused     = errors.New("cashback usage requires review")
	ErrCashbackUserUnavailable = errors.New("cashback user is not enabled")
)

// CashbackUsage is both a durable wallet billing session and its cashback
// outbox. Its identity and snapshot are immutable; balances are integer quota.
// Reserved sessions with no final usage are never automatically refunded.
type CashbackUsage struct {
	ID                   string `json:"id" gorm:"type:varchar(64);primaryKey"`
	RequestID            string `json:"request_id" gorm:"type:varchar(128);index"`
	UserID               int    `json:"user_id" gorm:"index;not null"`
	TokenID              int    `json:"token_id" gorm:"not null"`
	ModelName            string `json:"model_name" gorm:"type:varchar(255);not null"`
	Snapshot             string `json:"snapshot" gorm:"type:text;not null"`
	InitialReservedQuota int    `json:"initial_reserved_quota" gorm:"type:int;not null"`
	ReservedQuota        int    `json:"reserved_quota" gorm:"type:int;not null"`
	ActualQuota          int    `json:"actual_quota" gorm:"type:int;not null"`
	OriginalQuota        int    `json:"original_quota" gorm:"type:int;not null"`
	CreditedQuota        int    `json:"credited_quota" gorm:"type:int;not null"`
	CancelledQuota       int    `json:"cancelled_quota" gorm:"type:int;not null"`
	RecoveredQuota       int    `json:"recovered_quota" gorm:"type:int;not null"`
	RefundedQuota        int    `json:"refunded_quota" gorm:"type:int;not null"`
	InputTokens          int64  `json:"input_tokens" gorm:"bigint;not null"`
	OutputTokens         int64  `json:"output_tokens" gorm:"bigint;not null"`
	UsageSource          string `json:"usage_source" gorm:"type:varchar(128)"`
	Reason               string `json:"reason" gorm:"type:text"`
	State                string `json:"state" gorm:"type:varchar(32);index;not null"`
	Paused               bool   `json:"paused" gorm:"not null"`
	ReviewReason         string `json:"review_reason" gorm:"type:text"`
	Version              int64  `json:"version" gorm:"bigint;not null"`
	CachePending         bool   `json:"cache_pending" gorm:"not null"`
	CreatedTime          int64  `json:"created_time" gorm:"bigint;index;not null"`
	UpdatedTime          int64  `json:"updated_time" gorm:"bigint;not null"`
	LastAttemptTime      int64  `json:"last_attempt_time" gorm:"bigint;index;not null"`
}

type CashbackSettlementPlan struct {
	ActualQuota   int
	OriginalQuota int
	InputTokens   int64
	OutputTokens  int64
	UsageSource   string
	Reason        string
}

func BeginCashbackUsage(input CashbackUsage) (*CashbackUsage, error) {
	if common.BatchUpdateEnabled {
		return nil, ErrCashbackBatchEnabled
	}
	if input.ID == "" || len(input.ID) > 64 || len(input.RequestID) > 128 || input.UserID <= 0 || input.TokenID < 0 || input.ModelName == "" || len(input.ModelName) > 255 || input.Snapshot == "" || input.ReservedQuota < 0 || input.ReservedQuota > math.MaxInt32 {
		return nil, ErrInvalidCashbackUsage
	}
	row := CashbackUsage{ID: input.ID, RequestID: input.RequestID, UserID: input.UserID, TokenID: input.TokenID, ModelName: input.ModelName, Snapshot: input.Snapshot, InitialReservedQuota: input.ReservedQuota, ReservedQuota: input.ReservedQuota, State: CashbackStateReserved, Version: 1, CachePending: true, CreatedTime: common.GetTimestamp(), UpdatedTime: common.GetTimestamp()}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var existing CashbackUsage
		err := lockForUpdate(tx).Where("id = ?", row.ID).First(&existing).Error
		if err == nil {
			if !sameCashbackIdentity(existing, input) {
				return ErrCashbackUsageConflict
			}
			row = existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// Even a zero reservation (trusted request) needs a currently funded,
		// enabled account. Perform admission in the same transaction as creation,
		// but never re-apply it to an existing idempotent obligation.
		var user User
		if err := lockForUpdate(tx).Where("id = ?", row.UserID).First(&user).Error; err != nil {
			return err
		}
		if user.Status != common.UserStatusEnabled {
			return ErrCashbackUserUnavailable
		}
		if user.Quota <= 0 {
			return ErrInsufficientQuota
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if err := changeCashbackWallet(tx, &row, -row.ReservedQuota, 0, row.ReservedQuota); err != nil {
			return err
		}
		return tx.Create(&CashbackEntry{ID: row.ID + ":begin", UsageID: row.ID, UserID: row.UserID, Kind: "reserve", WalletDelta: -row.ReservedQuota, CreatedTime: row.CreatedTime}).Error
	})
	if err != nil {
		// A concurrent creator can win the primary-key race. Only the exact
		// same immutable request is an idempotent replay.
		var existing CashbackUsage
		if DB.Where("id = ?", input.ID).First(&existing).Error == nil && sameCashbackIdentity(existing, input) {
			repairCashbackCache(&existing)
			return &existing, nil
		}
		return nil, err
	}
	repairCashbackCache(&row)
	return &row, nil
}

func sameCashbackIdentity(row, input CashbackUsage) bool {
	return row.UserID == input.UserID && row.TokenID == input.TokenID && row.RequestID == input.RequestID && row.ModelName == input.ModelName && row.Snapshot == input.Snapshot && row.InitialReservedQuota == input.ReservedQuota
}

func ReserveCashbackUsage(id string, target int) (*CashbackUsage, error) {
	if target < 0 || target > math.MaxInt32 {
		return nil, ErrInvalidCashbackUsage
	}
	var row CashbackUsage
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where("id = ?", id).First(&row).Error; err != nil {
			return err
		}
		if row.State != CashbackStateReserved {
			return ErrCashbackUsageConflict
		}
		if target <= row.ReservedQuota {
			return nil
		}
		delta := target - row.ReservedQuota
		if err := changeCashbackWallet(tx, &row, -delta, 0, delta); err != nil {
			return err
		}
		row.ReservedQuota = target
		row.CachePending = true
		if err := saveCashbackUsage(tx, &row); err != nil {
			return err
		}
		return tx.Create(&CashbackEntry{ID: fmt.Sprintf("%s:reserve:%d", id, target), UsageID: id, UserID: row.UserID, Kind: "reserve", WalletDelta: -delta, CreatedTime: common.GetTimestamp()}).Error
	})
	if err != nil {
		return nil, err
	}
	repairCashbackCache(&row)
	return &row, nil
}

// PlanCashbackSettlement freezes final usage before the debit transaction so
// a crashed worker can replay settlement without reading optional consume logs.
func PlanCashbackSettlement(id string, plan CashbackSettlementPlan) (*CashbackUsage, error) {
	if plan.ActualQuota < 0 || plan.ActualQuota > math.MaxInt32 || plan.OriginalQuota < 0 || plan.OriginalQuota > math.MaxInt32 || (plan.ActualQuota == 0 && plan.OriginalQuota > 0) || plan.InputTokens < 0 || plan.OutputTokens < 0 || plan.InputTokens > math.MaxInt32 || plan.OutputTokens > math.MaxInt32 || len(plan.UsageSource) > 128 {
		return nil, ErrInvalidCashbackUsage
	}
	var row CashbackUsage
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where("id = ?", id).First(&row).Error; err != nil {
			return err
		}
		if row.State == CashbackStatePlanned || row.State == CashbackStateSettled {
			if row.ActualQuota == plan.ActualQuota && row.OriginalQuota == plan.OriginalQuota && row.InputTokens == plan.InputTokens && row.OutputTokens == plan.OutputTokens && row.UsageSource == plan.UsageSource && row.Reason == plan.Reason {
				return nil
			}
			return ErrCashbackUsageConflict
		}
		if row.State != CashbackStateReserved {
			return ErrCashbackUsageConflict
		}
		row.ActualQuota, row.OriginalQuota = plan.ActualQuota, plan.OriginalQuota
		row.InputTokens, row.OutputTokens = plan.InputTokens, plan.OutputTokens
		row.UsageSource, row.Reason, row.State = plan.UsageSource, plan.Reason, CashbackStatePlanned
		switch plan.Reason {
		case "invalid_usage", "quota_saturation", "invalid_cashback_calculation":
			row.Paused, row.ReviewReason = true, plan.Reason
		}
		return saveCashbackUsage(tx, &row)
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func SettleCashbackUsage(id string) (*CashbackUsage, error) {
	var row CashbackUsage
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where("id = ?", id).First(&row).Error; err != nil {
			return err
		}
		if row.State == CashbackStateSettled {
			return nil
		}
		if row.State != CashbackStatePlanned {
			return ErrCashbackUsageConflict
		}
		delta := row.ActualQuota - row.ReservedQuota
		if err := changeCashbackWallet(tx, &row, -delta, 0, delta); err != nil {
			return err
		}
		row.State, row.CachePending = CashbackStateSettled, true
		if err := saveCashbackUsage(tx, &row); err != nil {
			return err
		}
		return tx.Create(&CashbackEntry{ID: id + ":settle", UsageID: id, UserID: row.UserID, Kind: "settle", WalletDelta: -delta, CreatedTime: common.GetTimestamp()}).Error
	})
	if err != nil {
		return nil, err
	}
	repairCashbackCache(&row)
	return &row, nil
}

func CancelCashbackUsage(id string) (*CashbackUsage, error) {
	var row CashbackUsage
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where("id = ?", id).First(&row).Error; err != nil {
			return err
		}
		if row.State == CashbackStateCancelled {
			return nil
		}
		// A persisted final-usage plan must be settled, not mistaken for an
		// unconsumed reservation after a transient settlement failure.
		if row.State != CashbackStateReserved {
			return ErrCashbackUsageConflict
		}
		if err := changeCashbackWallet(tx, &row, row.ReservedQuota, 0, -row.ReservedQuota); err != nil {
			return err
		}
		row.State, row.CachePending = CashbackStateCancelled, true
		if err := saveCashbackUsage(tx, &row); err != nil {
			return err
		}
		return tx.Create(&CashbackEntry{ID: id + ":cancel", UsageID: id, UserID: row.UserID, Kind: "cancel", WalletDelta: row.ReservedQuota, CreatedTime: common.GetTimestamp()}).Error
	})
	if err != nil {
		return nil, err
	}
	repairCashbackCache(&row)
	return &row, nil
}

func saveCashbackUsage(tx *gorm.DB, row *CashbackUsage) error {
	previous := row.Version
	row.Version++
	row.UpdatedTime = common.GetTimestamp()
	result := tx.Model(&CashbackUsage{}).Where("id = ? AND version = ?", row.ID, previous).Select("*").Omit("id", "created_time").Updates(row)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrCashbackUsageConflict
	}
	return nil
}

// changeCashbackWallet serializes wallet and token accounting in the main DB.
// A positive tokenCharge spends the token allowance; negative refunds it.
// No legacy helpers are used here because they can queue non-durable updates.
func changeCashbackWallet(tx *gorm.DB, usage *CashbackUsage, walletDelta, cashbackDelta, tokenCharge int) error {
	var user User
	if err := lockForUpdate(tx).Where("id = ?", usage.UserID).First(&user).Error; err != nil {
		return err
	}
	wallet := int64(user.Quota) + int64(walletDelta)
	cashback := int64(user.CashbackQuota) + int64(cashbackDelta)
	if wallet < 0 {
		return ErrInsufficientQuota
	}
	if wallet > math.MaxInt32 || cashback < 0 || cashback > math.MaxInt32 {
		return ErrCashbackBalanceChanged
	}
	if walletDelta != 0 || cashbackDelta != 0 {
		result := tx.Model(&User{}).Where("id = ? AND quota = ? AND cashback_quota = ?", user.Id, user.Quota, user.CashbackQuota).Updates(map[string]interface{}{"quota": wallet, "cashback_quota": cashback})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrCashbackUsageConflict
		}
	}
	if usage.TokenID == 0 {
		return nil
	}
	var token Token
	// Existing obligations remain refundable after an API key is soft-deleted.
	if err := lockForUpdate(tx.Unscoped()).Where("id = ? AND user_id = ?", usage.TokenID, usage.UserID).First(&token).Error; err != nil {
		return err
	}
	if tokenCharge == 0 {
		return nil
	}
	remaining := int64(token.RemainQuota) - int64(tokenCharge)
	used := int64(token.UsedQuota) + int64(tokenCharge)
	if !token.UnlimitedQuota && remaining < 0 {
		return ErrInsufficientQuota
	}
	if remaining < math.MinInt32 || remaining > math.MaxInt32 || used < 0 || used > math.MaxInt32 {
		return ErrCashbackBalanceChanged
	}
	result := tx.Unscoped().Model(&Token{}).Where("id = ? AND remain_quota = ? AND used_quota = ?", token.Id, token.RemainQuota, token.UsedQuota).Updates(map[string]interface{}{"remain_quota": remaining, "used_quota": used, "accessed_time": common.GetTimestamp()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrCashbackUsageConflict
	}
	return nil
}

// Cache failure never turns a committed operation into a reported failure.
// The durable flag permits retry without repeating any monetary delta.
func repairCashbackCache(row *CashbackUsage) {
	if !row.CachePending {
		return
	}
	if err := invalidateUserCache(row.UserID); err != nil {
		common.SysError(fmt.Sprintf("cashback %s user cache invalidation: %v", row.ID, err))
		return
	}
	if common.RedisEnabled && row.TokenID > 0 {
		var token Token
		if err := DB.Unscoped().Select("key").Where("id = ? AND user_id = ?", row.TokenID, row.UserID).First(&token).Error; err != nil {
			common.SysError(fmt.Sprintf("cashback %s token lookup for cache invalidation: %v", row.ID, err))
			return
		}
		if err := cacheDeleteToken(token.Key); err != nil {
			common.SysError(fmt.Sprintf("cashback %s token cache invalidation: %v", row.ID, err))
			return
		}
	}
	result := DB.Model(&CashbackUsage{}).Where("id = ? AND version = ?", row.ID, row.Version).Update("cache_pending", false)
	if result.Error != nil {
		common.SysError(fmt.Sprintf("cashback %s cache repair marker: %v", row.ID, result.Error))
		return
	}
	if result.RowsAffected == 1 {
		row.CachePending = false
	}
}
