package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func GetRecentRedemptionCardHistory(c *gin.Context) {
	history, err := model.GetRecentRedemptionCardHistory(c.GetInt("id"), 10)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, history)
}
