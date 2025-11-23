package sms

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ViewSmsTimeLine godoc
// @Summary      View SMS timeline
// @Description  Get the timeline/status history of a specific SMS message
// @Tags         SMS
// @Accept       json
// @Produce      json
// @Param        id path string true "SMS Message ID"
// @Success      200 {object} map[string]interface{} "SMS timeline data"
// @Failure      404 {object} map[string]string "SMS not found"
// @Failure      500 {object} map[string]string "Internal server error"
// @Router       /v1/sms/{id} [get]
// @Security     ApiKeyAuth
func (h *SmsHandler) ViewSmsTimeLine(c *gin.Context) {
	smsId := c.Param("id")

	if smsId == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "sms not found",
		})
		return
	}

	timelineData, err := h.smsService.ViewSmsTimeLine(c, smsId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data":    timelineData,
	})
}
