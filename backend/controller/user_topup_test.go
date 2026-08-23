package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestTopUpDoesNotRequirePaymentCompliance(t *testing.T) {
	paymentSetting := operation_setting.GetPaymentSetting()
	previousComplianceConfirmed := paymentSetting.ComplianceConfirmed
	previousComplianceTermsVersion := paymentSetting.ComplianceTermsVersion

	paymentSetting.ComplianceConfirmed = false
	paymentSetting.ComplianceTermsVersion = ""
	t.Cleanup(func() {
		paymentSetting.ComplianceConfirmed = previousComplianceConfirmed
		paymentSetting.ComplianceTermsVersion = previousComplianceTermsVersion
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 1)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/user/topup",
		strings.NewReader(`{`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	TopUp(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "unexpected EOF")
	assert.NotContains(t, recorder.Body.String(), "Payment, redemption")
}
