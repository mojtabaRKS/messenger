package domain

import "time"

type Plan struct {
	ID          int64
	Name        string
	Description string
	ApiKey      string
	Price       int
	Priority    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
