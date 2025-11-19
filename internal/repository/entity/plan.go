package entity

import (
	"arvan/message-gateway/intrernal/domain"
	"time"
)

type Plan struct {
	ID          int64     `gorm:"primary_key" json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       int       `json:"price"`
	ApiKey      string    `json:"api_key"`
	Priority    int       `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (p Plan) ToDomain() domain.Plan {
	return domain.Plan{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
		ApiKey:      p.ApiKey,
		Priority:    p.Priority,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}
