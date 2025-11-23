package sms

import (
	"arvan/message-gateway/internal/constant"
	"arvan/message-gateway/pkg/paginator"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetAllSmsLog godoc
// @Summary      Get all SMS logs
// @Description  Retrieve all SMS logs for the authenticated user with pagination
// @Tags         SMS
// @Accept       json
// @Produce      json
// @Param        page query int false "Page number" default(1)
// @Param        page_size query int false "Number of items per page" default(10)
// @Success      200 {object} map[string]interface{} "List of SMS logs with pagination metadata"
// @Failure      400 {object} map[string]interface{} "Invalid user ID"
// @Failure      500 {object} map[string]interface{} "Internal server error"
// @Router       /v1/sms/log [get]
// @Security     ApiKeyAuth
func (h *SmsHandler) GetAllSmsLog(c *gin.Context) {
	userId := c.Value(constant.UserIdKey)

	iUserId, ok := userId.(int)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "cant find userId",
		})
		return
	}

	pagination := paginator.New(c)

	all, count, err := h.smsService.GetAllSmsLog(c, iUserId, pagination.Size, pagination.From)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data":    all,
		"meta": gin.H{
			"page_size": pagination.Size,
			"page":      pagination.Page,
			"total":     count,
		},
	})
}
