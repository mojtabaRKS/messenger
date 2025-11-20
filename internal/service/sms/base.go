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
	smsRepository smsRepository
	dlqRepository dlqRepository
	redisClient   *redis.Client
	logger        *logrus.Logger
	kafkaClient   *kafka.Conn
	kafkaWriter   *kafka.Writer
	data          map[string]int
	kafkaWorkChan chan domain.KafkaMessage
	dataChecksum  string
}

type smsRepository interface {
	DeductBalanceAndSaveSms(ctx context.Context, customerId int, message, receiver string) (uuid.UUID, error)
}

type dlqRepository interface {
	InsertDLQ(ctx context.Context, km domain.KafkaMessage) error
}

func NewSmsService(
	smsRepository smsRepository,
	dlqRepo dlqRepository,
	redisClient *redis.Client,
	logger *logrus.Logger,
	kafkaClient *kafka.Conn,
	kafkaWriter *kafka.Writer,
) *smsService {
	return &smsService{
		smsRepository: smsRepository,
		dlqRepository: dlqRepo,
		redisClient:   redisClient,
		logger:        logger,
		kafkaClient:   kafkaClient,
		kafkaWriter:   kafkaWriter,
		kafkaWorkChan: make(chan domain.KafkaMessage, constant.KafkaWorkerBufSize),
	}
}
