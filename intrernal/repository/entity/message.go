package entity

import "time"

type Message struct {
	Id                int64 `gorm:"primary_key"`
	UserId            int64
	ToNumber          string
	Body              string
	Status            string
	ProviderMessageId int
	CreatedAt         time.Time
}
