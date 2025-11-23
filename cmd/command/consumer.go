package command

import (
	"arvan/message-gateway/internal/config"
	"arvan/message-gateway/internal/constant"
	"arvan/message-gateway/internal/domain"
	"arvan/message-gateway/internal/infra"
	"arvan/message-gateway/internal/provider"
	"arvan/message-gateway/internal/queue"
	"arvan/message-gateway/internal/worker"
	"context"
	"encoding/json"
	"github.com/segmentio/kafka-go"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type ConsumerCommand struct {
	Logger *log.Logger
}

func (cmd ConsumerCommand) Command(ctx context.Context, cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "consume",
		Short: "run consumer command",
		Run: func(_ *cobra.Command, _ []string) {
			cmd.main(cfg, ctx)
		},
	}
}

func (cmd ConsumerCommand) main(cfg *config.Config, ctx context.Context) {
	queueManager := queue.NewQueueManager()
	smsProvider := provider.NewStubProvider()
	kafkaConsumerSmsAccepted := infra.NewKafkaConsumer(cfg.Kafka, constant.TopicAccepted)
	kafkaSmsStatusWriter := infra.NewKafkaWriter(cfg.Kafka, constant.TopicStatus)
	pool := worker.NewWorkerPool(queueManager, smsProvider, cfg.WorkerCount, kafkaSmsStatusWriter)

	pool.Start(ctx)

	numConsumers := cfg.WorkerCount
	if numConsumers == 0 {
		numConsumers = 10
	}

	for i := 0; i < numConsumers; i++ {
		consumerID := i
		go func() {
			for {
				m, err := kafkaConsumerSmsAccepted.ReadMessage(ctx)
				if err != nil {
					select {
					case <-ctx.Done():
						return
					default:
					}
					cmd.Logger.WithContext(ctx).Errorf("kafka consumer %d: read error: %v", consumerID, err)
					time.Sleep(500 * time.Millisecond)
					continue
				}

				var sms domain.Sms
				if err := json.Unmarshal(m.Value, &sms); err != nil {
					cmd.Logger.WithContext(ctx).Errorf("kafka consumer %d: invalid payload: %v", consumerID, err)
					continue
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
					"processing",
				}

				marshalled, err := json.Marshal(msg)
				if err != nil {
					cmd.Logger.WithContext(ctx).Warnf("kafka publish to status topic consumer_id [%d]:  error: %v", consumerID, err)
				}

				if err = kafkaSmsStatusWriter.WriteMessages(ctx, kafka.Message{
					Key:   []byte(job.ID),
					Value: marshalled,
					Time:  time.Now(),
				}); err != nil {
					cmd.Logger.WithContext(ctx).Warnf("kafka publish to status topic consumer_id [%d]: error: %v", consumerID, err)
				}

				if err := queueManager.Enqueue(sms.CustomerId, job); err != nil {
					cmd.Logger.WithContext(ctx).Errorf("kafka consumer %d: enqueue error: %v", consumerID, err)
				}
			}
		}()
	}

	cmd.Logger.WithContext(ctx).Infof("started %d kafka consumer goroutines", numConsumers)

	select {
	case <-ctx.Done():
		cmd.Logger.WithContext(ctx).Info("kafka consumer: context done, shutting down...")
		if err := kafkaConsumerSmsAccepted.Close(); err != nil {
			cmd.Logger.WithContext(ctx).Errorf("kafka consumer: close error: %s", err.Error())
		}
		pool.Stop(ctx)
	}
}
