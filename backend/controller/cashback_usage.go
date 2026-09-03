package controller

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetUsageCashbackSettings(c *gin.Context) {
	settings, err := model.GetUsageCashbackSettings()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"version": settings.Version, "enabled": settings.Enabled, "max_ratio": settings.MaxRatio,
		"models": settings.Models, "updated_by": settings.UpdatedBy, "updated_at": settings.UpdatedAt,
		"supported_models": model.GetUsageCashbackModelSupport(),
	})
}

// UsageCashbackRecord is the user-facing view. Internal snapshots, token IDs,
// cache repair flags and operator notes never cross the user endpoint.
type UsageCashbackRecord struct {
	ID             string                   `json:"id"`
	RequestID      string                   `json:"request_id"`
	ModelName      string                   `json:"model_name"`
	ActualQuota    int                      `json:"actual_quota"`
	OriginalQuota  int                      `json:"original_quota"`
	CreditedQuota  int                      `json:"credited_quota"`
	CancelledQuota int                      `json:"cancelled_quota"`
	RecoveredQuota int                      `json:"recovered_quota"`
	RefundedQuota  int                      `json:"refunded_quota"`
	InputTokens    int64                    `json:"input_tokens"`
	OutputTokens   int64                    `json:"output_tokens"`
	Reason         string                   `json:"reason"`
	Capped         bool                     `json:"capped"`
	State          string                   `json:"state"`
	Status         string                   `json:"status"`
	CreatedTime    int64                    `json:"created_time"`
	UpdatedTime    int64                    `json:"updated_time"`
	Rule           *UsageCashbackRecordRule `json:"rule,omitempty"`
}

type UsageCashbackRecordRule struct {
	InputPerMillion  string `json:"input_per_million"`
	OutputPerMillion string `json:"output_per_million"`
	MaxRatio         string `json:"max_ratio"`
}

func usageCashbackRecord(row model.CashbackUsage) UsageCashbackRecord {
	status := "processing"
	switch {
	case row.Paused:
		status = "pending_review"
	case row.State == model.CashbackStateCancelled:
		status = "not_eligible"
	case row.State == model.CashbackStateSettled && row.OriginalQuota == 0:
		status = "not_eligible"
	case row.OriginalQuota > 0 && row.CancelledQuota+row.RecoveredQuota >= row.OriginalQuota:
		status = "reversed"
	case row.State == model.CashbackStateSettled && row.CreditedQuota+row.CancelledQuota >= row.OriginalQuota:
		status = "credited"
	case row.State == model.CashbackStateSettled:
		status = "pending"
	}
	var rule UsageCashbackRecordRule
	var publicRule *UsageCashbackRecordRule
	if err := common.UnmarshalJsonStr(row.Snapshot, &rule); err == nil && rule.MaxRatio != "" {
		publicRule = &rule
	}
	return UsageCashbackRecord{
		ID: row.ID, RequestID: row.RequestID, ModelName: row.ModelName,
		ActualQuota: row.ActualQuota, OriginalQuota: row.OriginalQuota, CreditedQuota: row.CreditedQuota,
		CancelledQuota: row.CancelledQuota, RecoveredQuota: row.RecoveredQuota, RefundedQuota: row.RefundedQuota,
		InputTokens: row.InputTokens, OutputTokens: row.OutputTokens, Reason: row.Reason, Capped: row.Reason == "capped",
		State: row.State, Status: status, CreatedTime: row.CreatedTime, UpdatedTime: row.UpdatedTime, Rule: publicRule,
	}
}

func GetMyUsageCashbackRecords(c *gin.Context) {
	if c.GetInt("id") <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "code": "unauthorized"})
		return
	}
	listUsageCashbackRecords(c, c.GetInt("id"), false)
}

func GetAdminUsageCashbackRecords(c *gin.Context) {
	userID := 0
	if value := c.Query("user_id"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "invalid_user_id"})
			return
		}
		userID = parsed
	}
	listUsageCashbackRecords(c, userID, true)
}

func listUsageCashbackRecords(c *gin.Context, userID int, admin bool) {
	page := common.GetPageQuery(c)
	if page.Page < 1 || page.PageSize < 1 || page.Page > math.MaxInt/page.PageSize || len(c.Query("request_id")) > 128 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "invalid_pagination"})
		return
	}
	items, total, err := model.ListUsageCashbackRecords(userID, c.Query("request_id"), page.GetStartIdx(), page.PageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	views := make([]interface{}, 0, len(items))
	for _, row := range items {
		view := usageCashbackRecord(row)
		if admin {
			views = append(views, struct {
				UsageCashbackRecord
				UserID       int    `json:"user_id"`
				Snapshot     string `json:"snapshot"`
				ReviewReason string `json:"review_reason"`
				CachePending bool   `json:"cache_pending"`
			}{view, row.UserID, row.Snapshot, row.ReviewReason, row.CachePending})
		} else {
			views = append(views, view)
		}
	}
	page.SetTotal(int(total))
	page.SetItems(views)
	common.ApiSuccess(c, page)
}

func GetMyUsageCashbackRecord(c *gin.Context) {
	if c.GetInt("id") <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "code": "unauthorized"})
		return
	}
	getUsageCashbackRecord(c, c.GetInt("id"), false)
}

func GetAdminUsageCashbackRecord(c *gin.Context) {
	getUsageCashbackRecord(c, 0, true)
}

func getUsageCashbackRecord(c *gin.Context, userID int, admin bool) {
	row, entries, refunds, err := model.GetUsageCashbackRecordHistory(c.Param("id"), userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "code": "cashback_record_not_found"})
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if admin {
		common.ApiSuccess(c, gin.H{
			"record": usageCashbackRecord(*row), "entries": entries, "refunds": refunds,
			"snapshot": row.Snapshot, "review_reason": row.ReviewReason, "user_id": row.UserID,
		})
		return
	}
	publicEntries := make([]gin.H, 0, len(entries))
	for _, entry := range entries {
		publicEntries = append(publicEntries, gin.H{
			"id": entry.ID, "kind": entry.Kind, "wallet_delta": entry.WalletDelta,
			"cashback_delta": entry.CashbackDelta, "cancelled_quota": entry.CancelledQuota,
			"recovered_quota": entry.RecoveredQuota, "created_time": entry.CreatedTime,
		})
	}
	publicRefunds := make([]gin.H, 0, len(refunds))
	for _, refund := range refunds {
		publicRefunds = append(publicRefunds, gin.H{
			"id": refund.ID, "quota": refund.Quota, "cancelled_quota": refund.CancelledQuota,
			"recovered_quota": refund.RecoveredQuota, "cashback_debited": refund.CashbackDebited,
			"refund_withheld": refund.RefundWithheld, "wallet_credited": refund.WalletCredited,
			"created_time": refund.CreatedTime,
		})
	}
	common.ApiSuccess(c, gin.H{"record": usageCashbackRecord(*row), "entries": publicEntries, "refunds": publicRefunds})
}

func RetryUsageCashbackRecord(c *gin.Context) {
	row, err := model.GetCashbackUsage(c.Param("id"), 0)
	if err == nil && row.Paused {
		err = model.ErrCashbackUsagePaused
	}
	if err == nil && row.State == model.CashbackStatePlanned {
		row, err = model.SettleCashbackUsage(row.ID)
	}
	if err == nil {
		row, err = model.CreditCashbackUsage(row.ID)
	}
	if err != nil {
		usageCashbackOperationError(c, err)
		return
	}
	recordManageAudit(c, "cashback.retry", map[string]interface{}{"usage_id": row.ID})
	common.ApiSuccess(c, usageCashbackRecord(*row))
}

func RefundUsageCashbackRecord(c *gin.Context) {
	var request struct {
		EventID string `json:"event_id"`
		Quota   int    `json:"quota"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || request.EventID == "" || len(request.EventID) > 64 || request.Quota <= 0 || request.Quota > math.MaxInt32 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "invalid_cashback_refund"})
		return
	}
	refund, err := model.RefundCashbackUsage(c.Param("id"), request.EventID, request.Quota, c.GetInt("id"))
	if err != nil {
		usageCashbackOperationError(c, err)
		return
	}
	recordManageAudit(c, "cashback.refund", map[string]interface{}{"usage_id": c.Param("id"), "event_id": request.EventID, "quota": request.Quota})
	common.ApiSuccess(c, refund)
}

func PauseUsageCashbackRecord(c *gin.Context) {
	var request struct {
		Reason string `json:"reason"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || strings.TrimSpace(request.Reason) == "" || len(request.Reason) > 2000 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "invalid_cashback_review_reason"})
		return
	}
	if err := model.PauseCashbackUsage(c.Param("id"), request.Reason, c.GetInt("id")); err != nil {
		usageCashbackOperationError(c, err)
		return
	}
	recordManageAudit(c, "cashback.pause", map[string]interface{}{"usage_id": c.Param("id"), "reason": request.Reason})
	common.ApiSuccess(c, nil)
}

func usageCashbackOperationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"success": false, "code": "cashback_record_not_found"})
	case errors.Is(err, model.ErrInvalidCashbackUsage):
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "invalid_cashback_operation"})
	case errors.Is(err, model.ErrCashbackUsageConflict), errors.Is(err, model.ErrCashbackUsagePaused):
		c.JSON(http.StatusConflict, gin.H{"success": false, "code": "cashback_record_requires_review"})
	default:
		common.ApiError(c, err)
	}
}

func GetUsageCashbackRules(c *gin.Context) {
	settings, err := model.GetUsageCashbackSettings()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"version": settings.Version, "enabled": settings.Enabled, "max_ratio": settings.MaxRatio,
		"models": settings.Models, "supported_models": model.GetUsageCashbackModelSupport(),
	})
}

func UpdateUsageCashbackSettings(c *gin.Context) {
	var request model.UsageCashbackSettings
	if err := common.DecodeJson(http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20), &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "invalid_cashback_settings"})
		return
	}
	if err := model.ValidateUsageCashbackSettings(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": strings.SplitN(err.Error(), ":", 2)[0]})
		return
	}
	previous, err := model.GetUsageCashbackSettings()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	supported := model.GetUsageCashbackModelSupport()
	for name, rule := range request.Models {
		// A removed/disabled channel must not prevent an operator from shutting
		// down existing rules. New commitments still require a supported model.
		if rule.Enabled && (request.Enabled || rule != previous.Models[name]) && !supported[name].Supported {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "cashback_model_unsupported", "model": name})
			return
		}
	}
	settings, err := model.SaveUsageCashbackSettings(request, c.GetInt("id"))
	if errors.Is(err, model.ErrUsageCashbackSettingsConflict) {
		c.JSON(http.StatusConflict, gin.H{"success": false, "code": "cashback_settings_conflict"})
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"version": settings.Version, "enabled": settings.Enabled, "max_ratio": settings.MaxRatio,
		"models": settings.Models, "updated_by": settings.UpdatedBy, "updated_at": settings.UpdatedAt,
		"supported_models": supported,
	})
}
