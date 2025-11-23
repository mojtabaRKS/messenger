package command

import (
	"arvan/message-gateway/internal/config"
	"arvan/message-gateway/internal/constant"
	"arvan/message-gateway/internal/domain"
	"arvan/message-gateway/internal/infra"
	"arvan/message-gateway/internal/repository"
	"context"
	"encoding/json"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type StatusConsumerCommand struct {
	Logger *log.Logger
}

func (cmd StatusConsumerCommand) Command(ctx context.Context, cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "consume-status",
		Short: "consume SMS status messages from Kafka and push to ClickHouse",
		Run: func(_ *cobra.Command, _ []string) {
			cmd.main(cfg, ctx)
		},
	}
}

func (cmd StatusConsumerCommand) main(cfg *config.Config, ctx context.Context) {
	clickhouseDb, err := infra.NewClickHouseClient(cfg.Database.ClickHouse)
	if err != nil {
		cmd.Logger.WithContext(ctx).Fatalf("failed to initialize ClickHouse client: %v", err)
	}

	kafkaConsumerSmsStatus := infra.NewKafkaConsumer(cfg.Kafka, constant.TopicStatus)
	defer func() {
		if err := kafkaConsumerSmsStatus.Close(); err != nil {
			cmd.Logger.WithContext(ctx).Errorf("failed to close Kafka consumer: %v", err)
		}
	}()

	smsRepo := repository.NewSmsRepository(nil, clickhouseDb.GetDb())

	numConsumers := cfg.WorkerCount
	if numConsumers == 0 {
		numConsumers = 4
	}

	cmd.Logger.WithContext(ctx).Infof("starting %d consumer goroutines for sms.status topic", numConsumers)

	msgChan := make(chan domain.SMSStatus, 1000)

	for i := 0; i < numConsumers; i++ {
		consumerID := i
		go func() {
			for {
				select {
				case <-ctx.Done():
					cmd.Logger.WithContext(ctx).Infof("consumer %d: context cancelled, shutting down", consumerID)
					return
				default:
					m, err := kafkaConsumerSmsStatus.ReadMessage(ctx)
					if err != nil {
						select {
						case <-ctx.Done():
							return
						default:
						}
						cmd.Logger.WithContext(ctx).Errorf("consumer %d: read error: %v", consumerID, err)
						time.Sleep(500 * time.Millisecond)
						continue
					}

					var status domain.SMSStatus
					if err := json.Unmarshal(m.Value, &status); err != nil {
						cmd.Logger.WithContext(ctx).Errorf("consumer %d: failed to unmarshal message: %v, raw: %s", consumerID, err, string(m.Value))
						continue
					}

					select {
					case msgChan <- status:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}

	numWriters := 4
	for i := 0; i < numWriters; i++ {
		writerID := i
		go func() {
			batch := make([]domain.SMSStatus, 0, 100)
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			flushBatch := func() {
				if len(batch) == 0 {
					return
				}

				insertCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()

				for _, status := range batch {
					err := smsRepo.InsertSMSStatus(
						insertCtx,
						status.ID,
						status.CustomerID,
						status.Phone,
						status.Message,
						status.Status,
						status.Priority,
						status.CreatedAt,
						time.Now(),
					)
					if err != nil {
						cmd.Logger.WithContext(ctx).Errorf("writer %d: failed to insert status: %v", writerID, err)
					}

					cmd.Logger.WithContext(ctx).Info("writer %d: insert status: %v", writerID, status.ID)
				}

				cmd.Logger.WithContext(ctx).Debugf("writer %d: flushed %d messages to ClickHouse", writerID, len(batch))
				batch = batch[:0]
			}

			for {
				select {
				case <-ctx.Done():
					flushBatch()
					cmd.Logger.WithContext(ctx).Infof("writer %d: context cancelled, shutting down", writerID)
					return
				case status := <-msgChan:
					batch = append(batch, status)
					if len(batch) >= 100 {
						flushBatch()
					}
				case <-ticker.C:
					flushBatch()
				}
			}
		}()
	}

	cmd.Logger.WithContext(ctx).Info("status consumer started successfully")

	<-ctx.Done()
	cmd.Logger.WithContext(ctx).Info("status consumer: shutting down gracefully...")
	time.Sleep(2 * time.Second)
}
