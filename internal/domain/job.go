package domain

import "time"

type Job struct {
	ID         string
	CustomerID int
	Priority   int
	Phone      string
	Message    string
	CreatedAt  time.Time
}
