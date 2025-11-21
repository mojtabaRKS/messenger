package infra

import (
	"arvan/message-gateway/internal/config"
	"arvan/message-gateway/internal/constant"
	"fmt"
	"github.com/segmentio/kafka-go"
	"time"
)

func NewKafkaWriter(cfg config.Kafka, topic string) *kafka.Writer {
	return &kafka.Writer{
		Addr:         kafka.TCP(fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)),
		Topic:        topic,
		Balancer:     &kafka.Hash{},
		RequiredAcks: constant.KafkaProducerAcks,
		Async:        false, // workers perform sync writes with timeout + retries
		BatchTimeout: 10 * time.Millisecond,
		BatchSize:    1024,
	}
}

func NewKafkaConsumer(cfg config.Kafka, topic string) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:               []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)},
		Topic:                 topic,
		GroupID:               constant.KafkaGroupID,
		MinBytes:              1e3,  // 1KB
		MaxBytes:              10e6, // 10MB
		MaxWait:               500 * time.Millisecond,
		WatchPartitionChanges: true,
	})
}
