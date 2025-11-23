package domain

import "time"

type Sms struct {
	MessageId  string    `json:"message_id"`
	CustomerId int       `json:"customer_id"`
	Priority   int       `json:"priority"`
	To         string    `json:"to"`
	Message    string    `json:"message"`
	CreatedAt  time.Time `json:"created_at"`
}
