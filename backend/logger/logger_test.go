package logger

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultQuotaFormatUsesRMB(t *testing.T) {
	require.Equal(t, operation_setting.QuotaDisplayTypeCNY, operation_setting.GetQuotaDisplayType())
	require.EqualValues(t, 500000, common.QuotaPerUnit)
	assert.Equal(t, "￥7.300000", FormatQuota(500000))
	assert.Equal(t, "￥7.300000 额度", LogQuota(500000))
}
