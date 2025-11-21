package sms

import (
	"arvan/message-gateway/internal/constant"
	"arvan/message-gateway/internal/domain"
	"context"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

type smsService struct {
	balanceService balanceService
	dlqRepository  dlqRepository
	redisClient    *redis.Client
	logger         *logrus.Logger
	kafkaWriter    *kafka.Writer
	kafkaWorkChan  chan domain.KafkaMessage
}

type balanceService interface {
	DeductBalanceAndQueueSms(ctx context.Context, customerId int, message, receiver string) (uuid.UUID, error)
}

type dlqRepository interface {
	InsertDLQ(ctx context.Context, km domain.KafkaMessage) error
}

func NewSmsService(
	balanceService balanceService,
	dlqRepo dlqRepository,
	redisClient *redis.Client,
	logger *logrus.Logger,
	kafkaWriter *kafka.Writer,
) *smsService {
	return &smsService{
		balanceService: balanceService,
		dlqRepository:  dlqRepo,
		redisClient:    redisClient,
		logger:         logger,
		kafkaWriter:    kafkaWriter,
		kafkaWorkChan:  make(chan domain.KafkaMessage, constant.KafkaWorkerBufSize),
	}
}
