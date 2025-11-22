package entity

import (
	"fmt"
	"github.com/google/uuid"
	"time"
)

type SmsLog struct {
	MessageId  uuid.UUID `gorm:"message_id"`
	CustomerId int       `gorm:"customer_id"`
	ToNumber   string    `gorm:"to_number"`
	Body       string    `gorm:"body"`
	CreatedAt  time.Time `gorm:"created_at"`
}

func (SmsLog) TableName() string {
	return fmt.Sprintf("sms_logs_%s", time.Now().Format("2006_01_02"))
}

type SMSStatusLog struct {
	JobID      string    `gorm:"job_id"`
	CustomerID string    `gorm:"customer_id"`
	Phone      string    `gorm:"phone"`
	Message    string    `gorm:"message"`
	Status     string    `gorm:"status"`
	Priority   int32     `gorm:"priority"`
	CreatedAt  time.Time `gorm:"created_at"`
	Timestamp  time.Time `gorm:"timestamp"`
}

func (SMSStatusLog) TableName() string {
	return "sms_status_log"
}
