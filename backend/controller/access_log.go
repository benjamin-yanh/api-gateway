package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func GetAccessLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	status, _ := strconv.Atoi(c.Query("status"))
	startTime, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTime, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	items, total, err := model.GetAccessLogs(model.AccessLogFilter{
		Method:    c.Query("method"),
		Status:    status,
		Url:       strings.TrimSpace(c.Query("url")),
		RequestId: strings.TrimSpace(c.Query("request_id")),
		StartTime: startTime,
		EndTime:   endTime,
	}, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetAccessLog(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid access log id"})
		return
	}
	accessLog, err := model.GetAccessLogById(id)
	if model.IsAccessLogNotFound(err) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "access log not found"})
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, accessLog)
}
