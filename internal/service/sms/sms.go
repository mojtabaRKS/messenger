package sms

import (
	"arvan/message-gateway/internal/constant"
	"arvan/message-gateway/internal/domain"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/segmentio/kafka-go"

	"arvan/message-gateway/internal/api/request"
)

func (ss *smsService) Send(ctx context.Context, priority, customerId int, req request.SendSmsRequest) error {
	msgId, err := ss.balanceService.DeductBalanceAndQueueSms(ctx, customerId, req.Message, req.PhoneNumber)
	if err != nil {
		return err
	}

	sms := domain.Sms{
		MessageId:  msgId.String(),
		CustomerId: customerId,
		To:         req.PhoneNumber,
		Priority:   priority,
		Message:    req.Message,
		CreatedAt:  time.Now(),
	}
	b, err := json.Marshal(sms)
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

func (ss *smsService) ProduceMessages(workerID int) {
	for km := range ss.kafkaWorkChan {
		success := false
		for attempt := 0; attempt < constant.KafkaWriteRetries; attempt++ {
			ctx, cancel := context.WithTimeout(context.Background(), constant.KafkaWriteTimeout)

			// log error but move on
			err := ss.pushToStatusTopic(ctx, km)
			if err != nil {
				ss.logger.Warnf("kafka worker %d: write attempt %d failed: %v", workerID, attempt+1, err)
			}

			err = ss.kafkaWriterSmsAccepted.WriteMessages(ctx, kafka.Message{
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

// no need to prevent sms send because logging is not critical
func (ss *smsService) pushToStatusTopic(ctx context.Context, kmsg domain.KafkaMessage) error {
	var sms domain.Sms
	err := json.Unmarshal(kmsg.Payload, &sms)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal payload")
	}

	job := domain.Job{
		ID:         uuid.NewString(),
		CustomerID: sms.CustomerId,
		Phone:      sms.To,
		Message:    sms.Message,
		Priority:   sms.Priority,
		CreatedAt:  time.Now(),
	}

	msg := struct {
		domain.Job `json:",inline"`
		Status     string `json:"status"`
	}{
		job,
		"init",
	}

	marshalled, err := json.Marshal(msg)
	if err != nil {
		return errors.Wrap(err, "failed to marshal payload")
	}

	err = ss.kafkaWriterSmsStatus.WriteMessages(ctx, kafka.Message{
		Key:   []byte(job.ID),
		Value: marshalled,
		Time:  time.Now(),
	})
	if err != nil {
		return errors.Wrap(err, "failed to write messages")
	}

	return nil
}

func (ss *smsService) GetAllSmsLog(ctx context.Context, customerId, limit, offset int) ([]domain.SMSStatus, int64, error) {
	return ss.smsRepository.GetAllSmsLog(ctx, customerId, limit, offset)
}
func (ss *smsService) ViewSmsTimeLine(ctx context.Context, messageId string) ([]domain.SMSStatus, error) {
	return ss.smsRepository.ViewSmsTimeLine(ctx, messageId)
}
