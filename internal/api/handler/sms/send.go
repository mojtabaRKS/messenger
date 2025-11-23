package sms

import (
	"arvan/message-gateway/internal/api/request"
	"arvan/message-gateway/internal/constant"
	"net/http"

	"github.com/pkg/errors"

	"github.com/gin-gonic/gin"
)

// Send godoc
// @Summary      Send SMS
// @Description  Send an SMS message to a phone number
// @Tags         SMS
// @Accept       json
// @Produce      json
// @Param        request body request.SendSmsRequest true "SMS request body"
// @Success      200 {object} map[string]string "SMS queued successfully"
// @Failure      400 {object} map[string]string "Invalid request body"
// @Failure      402 {object} map[string]string "Insufficient balance"
// @Failure      500 {object} map[string]string "Internal server error"
// @Router       /v1/sms/send [post]
// @Security     ApiKeyAuth
func (h *SmsHandler) Send(c *gin.Context) {
	var req request.SendSmsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userId := c.MustGet(constant.UserIdKey).(int)
	priority := c.MustGet(constant.PriorityKey).(int)
	err := h.smsService.Send(c, priority, userId, req)
	if err != nil {
		if errors.Is(err, constant.InsufficientBalanceErr) {
			c.JSON(http.StatusPaymentRequired, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "queued"})
}
