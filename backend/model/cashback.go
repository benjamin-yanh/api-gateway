package model

import (
	"errors"
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var (
	ErrInvalidCashbackWithdrawal = errors.New("invalid cashback withdrawal amount")
	ErrCashbackBalanceChanged    = errors.New("cashback balance changed or current balance limit exceeded")
)

// CashbackWithdrawal is an immutable receipt in the main database so the
// receipt and both balances commit or roll back together. Amounts use quota units.
type CashbackWithdrawal struct {
	Id          int64 `json:"id" gorm:"primaryKey"`
	UserId      int   `json:"user_id" gorm:"index;not null"`
	Quota       int   `json:"quota" gorm:"type:int;not null"`
	CreatedTime int64 `json:"created_time" gorm:"bigint;not null"`
}

// WithdrawCashbackToBalance transfers the entire confirmed cashback balance.
// The conditional update rejects stale confirmations and concurrent retries on
// every supported database, without relying on SQLite row locks.
func WithdrawCashbackToBalance(userID, quota int) (*CashbackWithdrawal, error) {
	if userID <= 0 || quota <= 0 || quota > math.MaxInt32 {
		return nil, ErrInvalidCashbackWithdrawal
	}
	receipt := &CashbackWithdrawal{UserId: userID, Quota: quota, CreatedTime: common.GetTimestamp()}
	err := DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&User{}).
			Where("id = ? AND status = ? AND cashback_quota = ? AND quota <= ?", userID, common.UserStatusEnabled, quota, math.MaxInt32-quota).
			Updates(map[string]interface{}{
				"cashback_quota": 0,
				"quota":          gorm.Expr("quota + ?", quota),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrCashbackBalanceChanged
		}
		return tx.Create(receipt).Error
	})
	if err != nil {
		return nil, err
	}
	// Never report a committed transfer as failed: a cache outage must not cause
	// the client to retry an already completed financial operation.
	if err := cacheIncrUserQuota(userID, int64(quota)); err != nil {
		common.SysError(fmt.Sprintf("cashback withdrawal %d: failed to update user %d quota cache: %v", receipt.Id, userID, err))
	}
	return receipt, nil
}
