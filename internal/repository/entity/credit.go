package entity

import "time"

type Credit struct {
	Id            int `gorm:"primary_key"`
	UserId        int
	CreditBalance int
	UpdatedAt     time.Time
}
