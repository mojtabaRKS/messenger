package sms

import (
	"arvan/message-gateway/internal/constant"
	"arvan/message-gateway/internal/domain"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/segmentio/kafka-go"
	"strconv"
	"time"

	"arvan/message-gateway/internal/api/request"
)

func (ss *smsService) Send(ctx context.Context, priority, customerId int, req request.SendSmsRequest) error {
	msgId, err := ss.smsRepository.DeductBalanceAndSaveSms(ctx, customerId, req.Message, req.PhoneNumber)
	if err != nil {
		return err
	}

	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s:%s", req.PhoneNumber, req.Message)))
	hashed := h.Sum(nil)

	// prepare Kafka payload
	payload := map[string]interface{}{
		"message_id":  msgId.String(),
		"customer_id": customerId,
		"to":          req.PhoneNumber,
		"created_at":  time.Now().UTC().Format(time.RFC3339Nano),
		"body_hash":   string(hashed),
	}
	b, _ := json.Marshal(payload)
	kmsg := domain.KafkaMessage{
		Key:      strconv.Itoa(customerId),
		Payload:  b,
		Topic:    constant.KafkaTopic,
		Attempts: 0,
		Priority: priority,
	}

	// Non-blocking enqueue to background workers. If the channel is full, we synchronously write to kafka_dlq
	select {
	case ss.kafkaWorkChan <- kmsg:
		// enqueued successfully
	default:
		// worker queue full; fail-safe: persist to kafka_dlq synchronously (fast insert)
		if err := ss.dlqRepository.InsertDLQ(ctx, kmsg); err != nil {
			// If DLQ insert fails, log heavily — but do NOT undo billing (billing must remain correct).
			ss.logger.Error(errors.Wrap(err, "CRITICAL: dlq insert failed: %v"))
		}
	}

	return nil
}

func (ss *smsService) ProduceMessages(workerID int) {
	for km := range ss.kafkaWorkChan {
		// run every message in separate goroutine
		go func() {
			// attempt sync write with retries
			success := false
			for attempt := 0; attempt < constant.KafkaWriteRetries; attempt++ {
				ctx, cancel := context.WithTimeout(context.Background(), constant.KafkaWriteTimeout)
				err := ss.kafkaWriter.WriteMessages(ctx, kafka.Message{
					Key:   []byte(km.Key),
					Value: km.Payload,
					Time:  time.Now(),
				})
				cancel()
				if err == nil {
					success = true
					break
				}
				ss.logger.Printf("kafka write attempt %d failed: %v", attempt+1, err)
				time.Sleep(constant.KafkaRetryBackoff * time.Duration(attempt+1))
			}
			if !success {
				// push to DLQ in DB (non-blocking background insert)
				km.Attempts += constant.KafkaWriteRetries
				if err := ss.dlqRepository.InsertDLQ(context.Background(), km); err != nil {
					// Very rare: DLQ insert failed. Log and continue.
					ss.logger.Printf("kafka worker %d: failed to insert dlq: %v", workerID, err)
				}
			}
		}()
	}
}
