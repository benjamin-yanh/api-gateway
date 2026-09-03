package model

import "gorm.io/gorm"

// attachUserCashbackHistory adds gross credited rewards to a management page.
// Withdrawals and refund recoveries do not erase historical credits. A single
// grouped query covers the current page, within the user-list transaction.
func attachUserCashbackHistory(tx *gorm.DB, users []*User) error {
	if len(users) == 0 {
		return nil
	}
	ids := make([]int, 0, len(users))
	for _, user := range users {
		ids = append(ids, user.Id)
	}
	var totals []struct {
		UserID int
		Quota  int64
	}
	if err := tx.Model(&CashbackEntry{}).
		Select("user_id, SUM(cashback_delta) AS quota").
		Where("user_id IN ? AND kind = ?", ids, "credit").
		Group("user_id").Scan(&totals).Error; err != nil {
		return err
	}
	byUser := make(map[int]int64, len(totals))
	for _, total := range totals {
		byUser[total.UserID] = total.Quota
	}
	for _, user := range users {
		quota := byUser[user.Id]
		user.CashbackHistoryQuota = &quota
	}
	return nil
}

// ListUsageCashbackRecords accepts userID zero only from authenticated admin
// callers. The user endpoint always supplies its authenticated identity.
func ListUsageCashbackRecords(userID int, requestID string, offset, limit int) ([]CashbackUsage, int64, error) {
	query := DB.Model(&CashbackUsage{})
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if requestID != "" {
		query = query.Where("request_id = ?", requestID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]CashbackUsage, 0)
	err := query.Order("created_time DESC").Order("id DESC").Offset(offset).Limit(limit).Find(&items).Error
	return items, total, err
}

func GetUsageCashbackRecordHistory(id string, userID int) (*CashbackUsage, []CashbackEntry, []CashbackRefund, error) {
	usage, err := GetCashbackUsage(id, userID)
	if err != nil {
		return nil, nil, nil, err
	}
	entries := make([]CashbackEntry, 0)
	if err := DB.Where("usage_id = ?", id).Order("created_time ASC").Order("id ASC").Find(&entries).Error; err != nil {
		return nil, nil, nil, err
	}
	refunds := make([]CashbackRefund, 0)
	if err := DB.Where("usage_id = ?", id).Order("created_time ASC").Order("id ASC").Find(&refunds).Error; err != nil {
		return nil, nil, nil, err
	}
	return usage, entries, refunds, nil
}
