package repository

import (
	"arvan/message-gateway/internal/constant"
	"arvan/message-gateway/internal/repository/entity"
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type smsRepository struct {
	db *gorm.DB
}

func NewSmsRepository(db *gorm.DB) *smsRepository {
	return &smsRepository{
		db: db,
	}
}

func (sr *smsRepository) DeductBalanceAndSaveSms(ctx context.Context, customerId int, message, receiver string) (uuid.UUID, error) {
	msgId := uuid.New()

	// very short-lived transaction
	ctx, cancel := context.WithTimeout(ctx, constant.DBTxTimeout)
	defer cancel()

	err := sr.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rowsAffected, err := gorm.G[entity.Balance](tx).
			Where("customer_id = ? AND balance_bigint >= ?", customerId, 10).
			// we assume every sms is cost 10 rials
			Update(ctx, "balance_bigint", gorm.Expr("balance_bigint - ?", 10))
		if rowsAffected == 0 {
			return constant.InsufficientBalanceErr
		}

		if err != nil {
			return errors.Wrap(err, "failed to deduct balance")
		}

		// append only sms log table if cost is affected
		err = gorm.G[entity.SmsLog](tx).
			Create(ctx, &entity.SmsLog{
				Id:         msgId,
				CustomerId: customerId,
				ToNumber:   receiver,
				Body:       message,
			})

		if err != nil {
			return errors.Wrap(err, "failed to deduct sms")
		}

		return nil
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, constant.InsufficientBalanceErr
		}

		return uuid.Nil, err
	}

	return msgId, nil
}
