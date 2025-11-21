package sms

import (
	"arvan/message-gateway/internal/constant"
	"arvan/message-gateway/internal/domain"
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/segmentio/kafka-go"

	"arvan/message-gateway/internal/api/request"
)

func (ss *smsService) Send(ctx context.Context, priority, customerId int, req request.SendSmsRequest) error {
	// Fast Redis-based balance deduction (no DB transaction on critical path)
	msgId, err := ss.balanceService.DeductBalanceAndQueueSms(ctx, customerId, req.Message, req.PhoneNumber)
	if err != nil {
		return err
	}

	payload := domain.Sms{
		MessageId:  msgId.String(),
		CustomerId: customerId,
		To:         req.PhoneNumber,
		Priority:   priority,
		Message:    req.Message,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339Nano),
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal payload")
	}
	kmsg := domain.KafkaMessage{
		Key:      strconv.Itoa(customerId),
		Payload:  b,
		Topic:    constant.TopicAccepted,
		Attempts: 0,
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

// ProduceMessages is a worker that processes messages from the channel synchronously.
// Multiple workers run in parallel (configured by KafkaWriteWorkerPool constant).
func (ss *smsService) ProduceMessages(workerID int) {
	for km := range ss.kafkaWorkChan {
		// Process synchronously in worker (no goroutine spawn)
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
			ss.logger.Warnf("kafka worker %d: write attempt %d failed: %v", workerID, attempt+1, err)
			time.Sleep(constant.KafkaRetryBackoff * time.Duration(attempt+1))
		}
		if !success {
			// push to DLQ in DB
			km.Attempts += constant.KafkaWriteRetries
			if err := ss.dlqRepository.InsertDLQ(context.Background(), km); err != nil {
				ss.logger.Errorf("kafka worker %d: failed to insert dlq: %v", workerID, err)
			}
		}
	}
}
