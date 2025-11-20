package entity

import (
	"github.com/google/uuid"
	"time"
)

type SmsLog struct {
	Id         uuid.UUID
	CustomerId int
	ToNumber   string
	Body       string
	CreatedAt  time.Time
}

func (SmsLog) TableName() string {
	return "sms_logs"
}
