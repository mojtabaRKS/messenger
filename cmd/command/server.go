package command

import (
	"arvan/message-gateway/intrernal/api/middleware"
	"arvan/message-gateway/intrernal/repository"
	"arvan/message-gateway/intrernal/service/plan"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"arvan/message-gateway/intrernal/api"
	"arvan/message-gateway/intrernal/api/handler/sms"
	"arvan/message-gateway/intrernal/config"
	"arvan/message-gateway/intrernal/infra"
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

	// create repositories
	planRepository := repository.NewPlanRepository(db)

	// create services
	planService := plan.NewPlanService(planRepository, redisClient)

	// set plans in redis for get on demand
	plans, err := planService.GetAllPlansAndSetInRedis(ctx)
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
	smsHandler := sms.New()

	// create middlewares
	priorityMiddleware := middleware.NewPriorityMiddleware(
		redisClient,
		planService,
		plans,
		string(hash),
	)

	server := api.New(cfg.AppEnv)
	server.SetupAPIRoutes(
		smsHandler,
		priorityMiddleware,
	)

	// run the server
	if err := server.Serve(ctx, fmt.Sprintf(":%d", cfg.HTTP.Port)); err != nil {
		cmd.Logger.Fatal(err)
	}
}
