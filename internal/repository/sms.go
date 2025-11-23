package repository

import (
	"arvan/message-gateway/internal/constant"
	"arvan/message-gateway/internal/domain"
	"arvan/message-gateway/internal/repository/entity"
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"time"
)

type smsRepository struct {
	postgres   *gorm.DB
	clickhouse *gorm.DB
}

func NewSmsRepository(
	postgres *gorm.DB,
	clickhouse *gorm.DB,
) *smsRepository {
	return &smsRepository{
		postgres:   postgres,
		clickhouse: clickhouse,
	}
}

func (sr *smsRepository) DeductBalanceAndSaveSms(ctx context.Context, customerId int, message, receiver string) (uuid.UUID, error) {
	msgId := uuid.New()

	ctx, cancel := context.WithTimeout(ctx, constant.DBTxTimeout)
	defer cancel()

	err := sr.postgres.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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

		err = gorm.G[entity.SmsLog](tx).
			Create(ctx, &entity.SmsLog{
				MessageId:  msgId,
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

func (sr *smsRepository) InsertSMSStatus(ctx context.Context, jobID string, customerID int, phone, message, status string, priority int, createdAt time.Time, timestamp time.Time) error {
	err := gorm.G[entity.SMSStatusLog](sr.clickhouse).Create(ctx, &entity.SMSStatusLog{
		CustomerID: customerID,
		Id:         jobID,
		Phone:      phone,
		Message:    message,
		Status:     status,
		Priority:   priority,
		CreatedAt:  createdAt,
		Timestamp:  timestamp,
	})

	if err != nil {
		return errors.Wrap(err, "failed to insert SMS status")
	}

	return nil
}

func (sr *smsRepository) GetAllSmsLog(ctx context.Context, customerId, limit, offset int) ([]domain.SMSStatus, int64, error) {
	total, err := gorm.G[entity.SMSStatusLog](sr.clickhouse).
		Where("customer_id = ?", customerId).
		Count(ctx, "id")

	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get all sms logs")
	}

	logs, err := gorm.G[entity.SMSStatusLog](sr.clickhouse).
		Where("customer_id = ?", customerId).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get all sms logs")
	}

	var smsList []domain.SMSStatus
	for _, log := range logs {
		smsList = append(smsList, log.ToDomain())
	}

	return smsList, total, nil
}
func (sr *smsRepository) ViewSmsTimeLine(ctx context.Context, messageId string) ([]domain.SMSStatus, error) {
	logs, err := gorm.G[entity.SMSStatusLog](sr.clickhouse).
		Where("id = ?", messageId).
		Order("created_at DESC").
		Find(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all sms logs")
	}

	var smsList []domain.SMSStatus
	for _, log := range logs {
		smsList = append(smsList, log.ToDomain())
	}

	return smsList, nil
}
