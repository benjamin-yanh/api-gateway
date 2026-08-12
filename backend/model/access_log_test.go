package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetAccessLogsFiltersAndOmitsDetailPayload(t *testing.T) {
	originalDB := DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AccessLog{}))
	DB = db
	t.Cleanup(func() { DB = originalDB })

	logs := []*AccessLog{
		{CreatedAt: 10, RequestId: "request-1", Method: "POST", Url: "/v1/chat/completions?mode=fast", Status: 200, Headers: `{"X-Test":["one"]}`, Body: `{"model":"one"}`},
		{CreatedAt: 20, RequestId: "request-2", Method: "GET", Url: "/v1/models", Status: 200, Headers: `{"X-Test":["two"]}`},
		{CreatedAt: 30, RequestId: "request-3", Method: "POST", Url: "/v1/chat/completions?mode=slow", Status: 500, Headers: `{"X-Test":["three"]}`, Body: `{"model":"three"}`},
	}
	for _, accessLog := range logs {
		require.NoError(t, DB.Create(accessLog).Error)
	}

	items, total, err := GetAccessLogs(AccessLogFilter{
		Method: "post",
		Status: 500,
		Url:    "chat/completions",
	}, 0, 20)

	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, "request-3", items[0].RequestId)
	detail, err := GetAccessLogById(items[0].Id)
	require.NoError(t, err)
	assert.JSONEq(t, `{"X-Test":["three"]}`, detail.Headers)
	assert.JSONEq(t, `{"model":"three"}`, detail.Body)
}
