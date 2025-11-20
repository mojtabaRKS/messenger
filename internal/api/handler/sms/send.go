package sms

import (
	"arvan/message-gateway/internal/api/request"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (h *SmsHandler) Send(c *gin.Context) {
	var req request.SendSmsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userId := c.MustGet("userId").(int)
	priority := c.MustGet("priority").(int)
	err := h.smsService.Send(c, userId, priority, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "queued"})
}
