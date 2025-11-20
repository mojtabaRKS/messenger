package sms

import (
	"arvan/message-gateway/internal/api/request"
	"context"
)

type SmsHandler struct {
	smsService smsService
}

type smsService interface {
	Send(ctx context.Context, priority, customerId int, req request.SendSmsRequest) error
}

func New(smsService smsService) *SmsHandler {
	return &SmsHandler{
		smsService: smsService,
	}
}
