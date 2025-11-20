package infra

import (
	"arvan/message-gateway/internal/config"
	"arvan/message-gateway/internal/constant"
	"fmt"
	"github.com/segmentio/kafka-go"
	"time"
)

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
