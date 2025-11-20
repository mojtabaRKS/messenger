package infra

import (
	"arvan/message-gateway/internal/config"
	"arvan/message-gateway/internal/constant"
	"context"
	"fmt"
	"github.com/segmentio/kafka-go"
	"time"
)

func NewKafkaClient(ctx context.Context, cfg config.Kafka) (*kafka.Conn, error) {
	topic := constant.KafkaTopic
	partition := 0

	conn, err := kafka.DialLeader(ctx, "tcp", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), topic, partition)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func NewKafkaWriter(cfg config.Kafka) *kafka.Writer {
	return &kafka.Writer{
		Addr:         kafka.TCP(fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)),
		Topic:        constant.KafkaTopic,
		Balancer:     &kafka.Hash{},
		RequiredAcks: constant.KafkaProducerAcks,
		Async:        false, // workers perform sync writes with timeout + retries
		BatchTimeout: 10 * time.Millisecond,
		BatchSize:    1024,
	}
}
