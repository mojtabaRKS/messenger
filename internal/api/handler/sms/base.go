package sms

import (
	"arvan/message-gateway/internal/api/request"
	"arvan/message-gateway/internal/domain"
	"context"
)

type SmsHandler struct {
	smsService smsService
}

type smsService interface {
	Send(ctx context.Context, priority, customerId int, req request.SendSmsRequest) error
	GetAllSmsLog(ctx context.Context, customerId, limit, offset int) ([]domain.SMSStatus, int64, error)
	ViewSmsTimeLine(ctx context.Context, messageId string) ([]domain.SMSStatus, error)
}

func New(smsService smsService) *SmsHandler {
	return &SmsHandler{
		smsService: smsService,
	}
}
