package model

import (
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// AccessLogPayload uses a larger text column on MySQL so bounded request and
// response bodies do not exceed that dialect's 64 KiB TEXT limit. PostgreSQL
// and SQLite both use their unbounded TEXT storage.
type AccessLogPayload string

func (AccessLogPayload) GormDataType() string {
	return "text"
}

func (AccessLogPayload) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	if db.Dialector.Name() == "mysql" {
		return "MEDIUMTEXT"
	}
	return "TEXT"
}

// AccessLog stores an admin-only audit view of API requests, including request
// headers, JSON request bodies, and supported response bodies.
type AccessLog struct {
	Id                    int64            `json:"id"`
	CreatedAt             int64            `json:"created_at" gorm:"bigint;index:idx_access_logs_created_at"`
	RequestId             string           `json:"request_id" gorm:"type:varchar(64);index:idx_access_logs_request_id"`
	UserId                int              `json:"user_id" gorm:"index:idx_access_logs_user_id"`
	Username              string           `json:"username" gorm:"type:varchar(128);index:idx_access_logs_username;default:''"`
	Method                string           `json:"method" gorm:"type:varchar(16);index:idx_access_logs_method"`
	Url                   string           `json:"url" gorm:"type:text"`
	Route                 string           `json:"route" gorm:"type:varchar(255);default:''"`
	Status                int              `json:"status" gorm:"index:idx_access_logs_status"`
	LatencyMs             int64            `json:"latency_ms" gorm:"bigint;default:0"`
	ResponseSize          int64            `json:"response_size" gorm:"bigint;default:0"`
	Ip                    string           `json:"ip" gorm:"type:varchar(64);index:idx_access_logs_ip;default:''"`
	NodeName              string           `json:"node_name" gorm:"type:varchar(128);default:''"`
	Headers               string           `json:"headers" gorm:"type:text"`
	Body                  AccessLogPayload `json:"body,omitempty"`
	BodySize              int64            `json:"body_size" gorm:"bigint;default:0"`
	BodyOmitted           bool             `json:"body_omitted"`
	ResponseBody          AccessLogPayload `json:"response_body,omitempty"`
	ResponseBodyType      string           `json:"response_body_type,omitempty" gorm:"type:varchar(128);default:''"`
	ResponseBodyTruncated bool             `json:"response_body_truncated"`
}

type AccessLogListItem struct {
	Id           int64  `json:"id"`
	CreatedAt    int64  `json:"created_at"`
	RequestId    string `json:"request_id"`
	UserId       int    `json:"user_id"`
	Username     string `json:"username"`
	Method       string `json:"method"`
	Url          string `json:"url"`
	Route        string `json:"route"`
	Status       int    `json:"status"`
	LatencyMs    int64  `json:"latency_ms"`
	ResponseSize int64  `json:"response_size"`
	Ip           string `json:"ip"`
	NodeName     string `json:"node_name"`
	BodySize     int64  `json:"body_size"`
	BodyOmitted  bool   `json:"body_omitted"`
}

type AccessLogFilter struct {
	Method    string
	Status    int
	Url       string
	RequestId string
	StartTime int64
	EndTime   int64
}

func CreateAccessLog(accessLog *AccessLog) error {
	return DB.Create(accessLog).Error
}

func GetAccessLogs(filter AccessLogFilter, startIdx int, limit int) ([]*AccessLogListItem, int64, error) {
	tx := DB.Model(&AccessLog{})
	if method := strings.ToUpper(strings.TrimSpace(filter.Method)); method != "" {
		tx = tx.Where("method = ?", method)
	}
	if filter.Status > 0 {
		tx = tx.Where("status = ?", filter.Status)
	}
	if filter.RequestId != "" {
		tx = tx.Where("request_id = ?", filter.RequestId)
	}
	if filter.Url != "" {
		pattern := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(filter.Url)
		tx = tx.Where("url LIKE ? ESCAPE '!'", "%"+pattern+"%")
	}
	if filter.StartTime > 0 {
		tx = tx.Where("created_at >= ?", filter.StartTime)
	}
	if filter.EndTime > 0 {
		tx = tx.Where("created_at <= ?", filter.EndTime)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]*AccessLogListItem, 0)
	err := tx.Select(
		"id", "created_at", "request_id", "user_id", "username", "method",
		"url", "route", "status", "latency_ms", "response_size", "ip",
		"node_name", "body_size", "body_omitted",
	).Order("created_at desc, id desc").Limit(limit).Offset(startIdx).Scan(&items).Error
	return items, total, err
}

func GetAccessLogById(id int64) (*AccessLog, error) {
	var accessLog AccessLog
	if err := DB.First(&accessLog, id).Error; err != nil {
		return nil, err
	}
	return &accessLog, nil
}

func IsAccessLogNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
