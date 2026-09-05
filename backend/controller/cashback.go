package controller

import (
	"errors"
	"math"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func WithdrawCashback(c *gin.Context) {
	userID := c.GetInt("id")
	if userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "code": "unauthorized"})
		return
	}
	var request struct {
		Method string `json:"method"`
		Quota  int    `json:"quota"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || request.Quota <= 0 || request.Quota > math.MaxInt32 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "invalid_cashback_amount"})
		return
	}
	if request.Method != "balance" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "withdrawal_method_unavailable"})
		return
	}
	receipt, err := model.WithdrawCashbackToBalance(userID, request.Quota)
	if errors.Is(err, model.ErrCashbackBalanceChanged) {
		c.JSON(http.StatusConflict, gin.H{"success": false, "code": "cashback_balance_changed"})
		return
	}
	if err != nil {
		common.SysError("cashback withdrawal failed: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "cashback_withdrawal_failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": receipt})
}
