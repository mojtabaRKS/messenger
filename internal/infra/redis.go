package infra

import (
	"arvan/message-gateway/internal/config"
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient(ctx context.Context, cfg config.Redis, logger *log.Logger) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.Database,
	})

	_, err := rdb.Ping(ctx).Result()
	logger.Info(fmt.Sprintf("redis is running on %s : %s on db %d", cfg.Host, cfg.Port, cfg.Database))
	if err != nil {
		return nil, err
	}

	return rdb, err
}
