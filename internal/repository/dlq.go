package repository

import (
	"arvan/message-gateway/internal/domain"
	"arvan/message-gateway/internal/repository/entity"
	"context"
	"gorm.io/gorm"
	"time"
)

type dlqRepository struct {
	db *gorm.DB
}

func NewDlqRepository(db *gorm.DB) *dlqRepository {
	return &dlqRepository{
		db: db,
	}
}

func (dr *dlqRepository) InsertDLQ(ctx context.Context, km domain.KafkaMessage) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	return gorm.G[entity.KafkaDlq](dr.db).Create(ctx, &entity.KafkaDlq{
		Topic:         km.Topic,
		Key:           km.Key,
		Payload:       km.Payload,
		AttemptCount:  km.Attempts,
		LastAttemptAt: time.Now(),
	})
}
