package model

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// CashbackEntry is an immutable receipt. WalletDelta and CashbackDelta are
// separate so a recovery withheld from a refund never debits cashback twice.
type CashbackEntry struct {
	ID             string `json:"id" gorm:"type:varchar(192);primaryKey"`
	UsageID        string `json:"usage_id" gorm:"type:varchar(64);index;not null"`
	UserID         int    `json:"user_id" gorm:"index;not null"`
	ActorID        int    `json:"actor_id" gorm:"not null"`
	Kind           string `json:"kind" gorm:"type:varchar(32);not null"`
	WalletDelta    int    `json:"wallet_delta" gorm:"type:int;not null"`
	CashbackDelta  int    `json:"cashback_delta" gorm:"type:int;not null"`
	CancelledQuota int    `json:"cancelled_quota" gorm:"type:int;not null"`
	RecoveredQuota int    `json:"recovered_quota" gorm:"type:int;not null"`
	Reason         string `json:"reason" gorm:"type:text"`
	CreatedTime    int64  `json:"created_time" gorm:"bigint;index;not null"`
}

type CashbackRefund struct {
	ID              string `json:"id" gorm:"type:varchar(64);primaryKey"`
	UsageID         string `json:"usage_id" gorm:"type:varchar(64);index;not null"`
	UserID          int    `json:"user_id" gorm:"index;not null"`
	ActorID         int    `json:"actor_id" gorm:"not null"`
	Quota           int    `json:"quota" gorm:"type:int;not null"`
	CancelledQuota  int    `json:"cancelled_quota" gorm:"type:int;not null"`
	RecoveredQuota  int    `json:"recovered_quota" gorm:"type:int;not null"`
	CashbackDebited int    `json:"cashback_debited" gorm:"type:int;not null"`
	RefundWithheld  int    `json:"refund_withheld" gorm:"type:int;not null"`
	WalletCredited  int    `json:"wallet_credited" gorm:"type:int;not null"`
	CreatedTime     int64  `json:"created_time" gorm:"bigint;not null"`
}

func CreditCashbackUsage(id string) (*CashbackUsage, error) {
	var row CashbackUsage
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where("id = ?", id).First(&row).Error; err != nil {
			return err
		}
		if row.State != CashbackStateSettled {
			return ErrCashbackUsageConflict
		}
		if row.Paused {
			return ErrCashbackUsagePaused
		}
		if err := validateCashbackAccounting(row); err != nil {
			return err
		}
		amount := row.OriginalQuota - row.CancelledQuota - row.CreditedQuota
		if amount == 0 {
			return nil
		}
		if err := changeCashbackWallet(tx, &row, 0, amount, 0); err != nil {
			return err
		}
		row.CreditedQuota += amount
		row.CachePending = true
		if err := saveCashbackUsage(tx, &row); err != nil {
			return err
		}
		return tx.Create(&CashbackEntry{ID: id + ":credit", UsageID: id, UserID: row.UserID, Kind: "credit", CashbackDelta: amount, CreatedTime: common.GetTimestamp()}).Error
	})
	if err != nil {
		return nil, err
	}
	repairCashbackCache(&row)
	return &row, nil
}

func RefundCashbackUsage(id, eventID string, quota int, actorID ...int) (*CashbackRefund, error) {
	if eventID == "" || len(eventID) > 64 || quota <= 0 || quota > math.MaxInt32 {
		return nil, ErrInvalidCashbackUsage
	}
	actor := 0
	if len(actorID) > 0 {
		actor = actorID[0]
	}
	if actor < 0 || len(actorID) > 1 {
		return nil, ErrInvalidCashbackUsage
	}
	var row CashbackUsage
	var receipt CashbackRefund
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where("id = ?", id).First(&row).Error; err != nil {
			return err
		}
		err := tx.Where("id = ?", eventID).First(&receipt).Error
		if err == nil {
			if receipt.UsageID != id || receipt.Quota != quota {
				return ErrCashbackUsageConflict
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if row.State != CashbackStateSettled {
			return ErrCashbackUsageConflict
		}
		if err := validateCashbackAccounting(row); err != nil {
			return err
		}
		if quota > row.ActualQuota-row.RefundedQuota {
			return ErrInvalidCashbackUsage
		}
		cumulative := row.RefundedQuota + quota
		// QuoRem with precision zero performs exact integer division; Decimal.Div
		// would first round at its configured precision before Floor.
		quotient, _ := decimal.NewFromInt(int64(row.OriginalQuota)).Mul(decimal.NewFromInt(int64(cumulative))).QuoRem(decimal.NewFromInt(int64(row.ActualQuota)), 0)
		target, clamp := common.QuotaFromDecimalChecked(quotient)
		if clamp != nil {
			return clamp
		}
		toRevoke := target - row.CancelledQuota - row.RecoveredQuota
		if toRevoke < 0 {
			return ErrCashbackUsageConflict
		}
		cancelled := min(toRevoke, row.OriginalQuota-row.CreditedQuota-row.CancelledQuota)
		recovered := toRevoke - cancelled
		var user User
		if err := lockForUpdate(tx).Where("id = ?", row.UserID).First(&user).Error; err != nil {
			return err
		}
		if user.CashbackQuota < 0 {
			return ErrCashbackBalanceChanged
		}
		cashbackDebit := min(recovered, user.CashbackQuota)
		withheld := recovered - cashbackDebit
		if withheld > quota {
			return ErrCashbackUsageConflict
		}
		walletCredit := quota - withheld
		if err := changeCashbackWallet(tx, &row, walletCredit, -cashbackDebit, -quota); err != nil {
			return err
		}
		row.RefundedQuota = cumulative
		row.CancelledQuota += cancelled
		row.RecoveredQuota += recovered
		row.CachePending = true
		if err := saveCashbackUsage(tx, &row); err != nil {
			return err
		}
		receipt = CashbackRefund{ID: eventID, UsageID: id, UserID: row.UserID, ActorID: actor, Quota: quota, CancelledQuota: cancelled, RecoveredQuota: recovered, CashbackDebited: cashbackDebit, RefundWithheld: withheld, WalletCredited: walletCredit, CreatedTime: common.GetTimestamp()}
		if err := tx.Create(&receipt).Error; err != nil {
			return err
		}
		return tx.Create(&CashbackEntry{ID: id + ":refund:" + eventID, UsageID: id, UserID: row.UserID, ActorID: actor, Kind: "refund", WalletDelta: walletCredit, CashbackDelta: -cashbackDebit, CancelledQuota: cancelled, RecoveredQuota: recovered, CreatedTime: receipt.CreatedTime}).Error
	})
	if err != nil {
		return nil, err
	}
	repairCashbackCache(&row)
	return &receipt, nil
}

func validateCashbackAccounting(row CashbackUsage) error {
	if row.ActualQuota < 0 || row.ActualQuota > math.MaxInt32 || row.OriginalQuota < 0 || (row.OriginalQuota > 0 && row.OriginalQuota >= row.ActualQuota) || row.CreditedQuota < 0 || row.CancelledQuota < 0 || int64(row.CreditedQuota)+int64(row.CancelledQuota) > int64(row.OriginalQuota) || row.RecoveredQuota < 0 || row.RecoveredQuota > row.CreditedQuota || row.RefundedQuota < 0 || row.RefundedQuota > row.ActualQuota {
		return ErrCashbackUsageConflict
	}
	return nil
}

func PauseCashbackUsage(id, reason string, actorID ...int) error {
	return reviewCashbackUsage(id, reason, true, actorID...)
}

// ResumeCashbackUsage is an explicit, audited review decision, never an
// automatic worker action. It cannot alter a finalized amount or snapshot.
func ResumeCashbackUsage(id, reason string, actorID ...int) error {
	return reviewCashbackUsage(id, reason, false, actorID...)
}

func reviewCashbackUsage(id, reason string, paused bool, actorID ...int) error {
	if reason == "" {
		return ErrInvalidCashbackUsage
	}
	actor := 0
	if len(actorID) > 0 {
		actor = actorID[0]
	}
	if actor < 0 || len(actorID) > 1 {
		return ErrInvalidCashbackUsage
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var row CashbackUsage
		if err := lockForUpdate(tx).Where("id = ?", id).First(&row).Error; err != nil {
			return err
		}
		if row.Paused == paused && row.ReviewReason == reason {
			return nil
		}
		row.Paused, row.ReviewReason = paused, reason
		if err := saveCashbackUsage(tx, &row); err != nil {
			return err
		}
		kind := "resume"
		if paused {
			kind = "pause"
		}
		return tx.Create(&CashbackEntry{ID: row.ID + ":review:" + common.NewRequestId(), UsageID: row.ID, UserID: row.UserID, ActorID: actor, Kind: kind, Reason: reason, CreatedTime: common.GetTimestamp()}).Error
	})
}

// RetryCashbackUsages performs bounded, idempotent recovery. A reserved request
// may have reached the upstream, so absence of a final plan is never inferred
// to mean zero usage. No global/model setting can erase existing obligations.
func RetryCashbackUsages(limit int) (int, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var rows []CashbackUsage
	err := DB.Where("cache_pending = ? OR state = ? OR (state = ? AND paused = ? AND credited_quota + cancelled_quota < original_quota)", true, CashbackStatePlanned, CashbackStateSettled, false).Order("last_attempt_time ASC, created_time ASC, id ASC").Limit(limit).Find(&rows).Error
	if err != nil {
		return 0, err
	}
	completed := 0
	var failures []error
	for _, row := range rows {
		// Move even failed attempts to the back of the queue. An account with
		// insufficient quota must not starve other users' pending credits.
		row.LastAttemptTime = time.Now().UnixMilli()
		if err := DB.Model(&CashbackUsage{}).Where("id = ?", row.ID).Update("last_attempt_time", row.LastAttemptTime).Error; err != nil {
			failures = append(failures, fmt.Errorf("cashback %s retry marker: %w", row.ID, err))
			continue
		}
		repairCashbackCache(&row)
		if row.State == CashbackStatePlanned {
			settled, err := SettleCashbackUsage(row.ID)
			if err != nil {
				failures = append(failures, fmt.Errorf("cashback %s settlement: %w", row.ID, err))
				continue
			}
			row = *settled
		}
		if row.State == CashbackStateSettled && !row.Paused {
			if _, err := CreditCashbackUsage(row.ID); err != nil {
				failures = append(failures, fmt.Errorf("cashback %s credit: %w", row.ID, err))
				continue
			}
		}
		completed++
	}
	return completed, errors.Join(failures...)
}

// A zero userID is reserved for internal/admin callers. User-facing handlers
// must pass the authenticated user ID, never an ID supplied by the client.
func GetCashbackUsage(id string, userID int) (*CashbackUsage, error) {
	var row CashbackUsage
	query := DB.Where("id = ?", id)
	if userID != 0 {
		query = query.Where("user_id = ?", userID)
	}
	if err := query.First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func ListCashbackUsages(userID, offset, limit int) ([]CashbackUsage, int64, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	query := DB.Model(&CashbackUsage{})
	if userID != 0 {
		query = query.Where("user_id = ?", userID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []CashbackUsage
	if err := query.Order("created_time DESC, id DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
