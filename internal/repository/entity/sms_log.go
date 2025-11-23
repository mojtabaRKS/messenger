package entity

import (
	"arvan/message-gateway/internal/domain"
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
	Id         string    `gorm:"id"`
	CustomerID int       `gorm:"customer_id"`
	Phone      string    `gorm:"phone"`
	Message    string    `gorm:"message"`
	Status     string    `gorm:"status"`
	Priority   int       `gorm:"priority"`
	CreatedAt  time.Time `gorm:"created_at"`
	Timestamp  time.Time `gorm:"timestamp"`
}

func (SMSStatusLog) TableName() string {
	return "sms_status_log"
}

func (s SMSStatusLog) ToDomain() domain.SMSStatus {
	return domain.SMSStatus{
		ID:         s.Id,
		CustomerID: s.CustomerID,
		Priority:   s.Priority,
		Phone:      s.Phone,
		Message:    s.Message,
		Status:     s.Status,
		CreatedAt:  s.CreatedAt,
	}
}
