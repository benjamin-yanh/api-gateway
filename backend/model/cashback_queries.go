package model

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
