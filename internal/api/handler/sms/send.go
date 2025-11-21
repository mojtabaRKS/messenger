package sms

import (
	"arvan/message-gateway/internal/api/request"
	"arvan/message-gateway/internal/constant"
	"github.com/pkg/errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

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
