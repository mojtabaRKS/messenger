package domain

import "time"

type SMSStatus struct {
	ID         string    `json:"ID"`
	CustomerID int       `json:"CustomerID"`
	Phone      string    `json:"Phone"`
	Message    string    `json:"Message"`
	Priority   int       `json:"Priority"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"CreatedAt"`
}
