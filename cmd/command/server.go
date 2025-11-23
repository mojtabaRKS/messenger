package command

import (
	"arvan/message-gateway/internal/api/middleware"
	"arvan/message-gateway/internal/constant"
	"arvan/message-gateway/internal/repository"
	"arvan/message-gateway/internal/service/plan"
	smsService "arvan/message-gateway/internal/service/sms"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"arvan/message-gateway/internal/api"
	"arvan/message-gateway/internal/api/handler/sms"
	"arvan/message-gateway/internal/config"
	"arvan/message-gateway/internal/infra"
	balanceService "arvan/message-gateway/internal/service/balance"
)

type Server struct {
	Logger *logrus.Logger
}

func (cmd Server) Command(ctx context.Context, cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "run Gateway server",
		Run: func(_ *cobra.Command, _ []string) {
			cmd.main(cfg, ctx)
		},
	}
}

func (cmd Server) main(cfg *config.Config, ctx context.Context) {
	psql, err := infra.NewPostgresClient(ctx, cfg.Database.Postgres)
	if err != nil {
		cmd.Logger.WithContext(ctx).Fatal(errors.Wrap(err, "server : failed to connect to postgresql"))
		return
	}

	clickhouse, err := infra.NewClickHouseClient(cfg.Database.ClickHouse)
	if err != nil {
		cmd.Logger.WithContext(ctx).Fatal(errors.Wrap(err, "server : failed to connect to clickhouse"))
		return
	}

	redisClient, err := infra.NewRedisClient(ctx, cfg.Database.Redis, cmd.Logger)
	if err != nil {
		cmd.Logger.WithContext(ctx).Fatal(errors.Wrap(err, "server : failed to connect to redis"))
		return
	}

	defer func() {
		if err = redisClient.Close(); err != nil {
			cmd.Logger.WithContext(ctx).Fatal(errors.Wrap(err, "server : failed to close redis"))
		}
	}()

	kafkaWriterSmsAccepted := infra.NewKafkaWriter(cfg.Kafka, constant.TopicAccepted)
	kafkaSmsStatusWriter := infra.NewKafkaWriter(cfg.Kafka, constant.TopicStatus)

	planRepository := repository.NewPlanRepository(psql.GetDb())
	dlqRepository := repository.NewDlqRepository(psql.GetDb())
	smsRepository := repository.NewSmsRepository(psql.GetDb(), clickhouse.GetDb())

	planServiceInstance := plan.NewPlanService(planRepository, redisClient)

	balanceServiceInstance := balanceService.NewBalanceService(
		redisClient,
		psql.GetDb(),
		cmd.Logger,
		constant.BalanceQueueSize,
		constant.BalanceWriterWorkers,
	)
	if err := balanceServiceInstance.InitializeBalanceCache(ctx); err != nil {
		cmd.Logger.WithContext(ctx).Fatal(errors.Wrap(err, "server : failed to initialize balance cache"))
		return
	}

	smsServiceInstance := smsService.NewSmsService(
		balanceServiceInstance,
		dlqRepository,
		smsRepository,
		redisClient,
		cmd.Logger,
		kafkaWriterSmsAccepted,
		kafkaSmsStatusWriter,
	)

	plans, err := planServiceInstance.GetAllPlansAndSetInRedis(ctx)
	if err != nil {
		cmd.Logger.WithContext(ctx).Fatal(errors.Wrap(err, "server : failed to get all plans and set in redis"))
		return
	}

	marshalled, err := json.Marshal(plans)
	if err != nil {
		cmd.Logger.WithContext(ctx).Fatal(errors.Wrap(err, "server : failed to marshal plans"))
		return
	}

	h := sha256.New()
	h.Write(marshalled)
	hash := h.Sum(nil)

	smsHandler := sms.New(smsServiceInstance)

	priorityMiddleware := middleware.NewPriorityMiddleware(
		redisClient,
		planServiceInstance,
		plans,
		string(hash),
	)

	server := api.New(cfg.AppEnv)
	server.SetupAPIRoutes(
		smsHandler,
		priorityMiddleware,
	)

	for i := 0; i < constant.KafkaWriteWorkerPool; i++ {
		go smsServiceInstance.ProduceMessages(i)
	}
	cmd.Logger.WithContext(ctx).Infof("started %d kafka producer workers", constant.KafkaWriteWorkerPool)

	defer func() {
		cmd.Logger.Info("shutting down balance service...")
		balanceServiceInstance.Stop()
		cmd.Logger.Info("balance service stopped")
	}()

	if err := server.Serve(ctx, fmt.Sprintf(":%d", cfg.HTTP.Port)); err != nil {
		cmd.Logger.Fatal(err)
	}
}
