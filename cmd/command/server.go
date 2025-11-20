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
	db, err := infra.NewPostgresClient(ctx, cfg.Database.Postgres)
	if err != nil {
		cmd.Logger.WithContext(ctx).Fatal(errors.Wrap(err, "server : failed to connect to postgresql"))
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

	kafkaWriter := infra.NewKafkaWriter(cfg.Kafka)

	// create repositories
	planRepository := repository.NewPlanRepository(db)
	dlqRepository := repository.NewDlqRepository(db)
	smsRepository := repository.NewSmsRepository(db)

	// create services
	planServiceInstance := plan.NewPlanService(planRepository, redisClient)
	smsServiceInstance := smsService.NewSmsService(smsRepository, dlqRepository, redisClient, cmd.Logger, kafkaWriter)

	// set plans in redis for get on demand
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

	// create handlers
	smsHandler := sms.New(smsServiceInstance)

	// create middlewares
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

	// start background kafka workers
	for i := 0; i < constant.KafkaWorkerCount; i++ {
		go smsServiceInstance.ProduceMessages(i)
	}

	// run the server
	if err := server.Serve(ctx, fmt.Sprintf(":%d", cfg.HTTP.Port)); err != nil {
		cmd.Logger.Fatal(err)
	}
}
