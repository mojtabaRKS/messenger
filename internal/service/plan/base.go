package plan

import (
	"arvan/message-gateway/internal/domain"
	"context"

	"github.com/redis/go-redis/v9"
)

type planService struct {
	planRepository planRepository
	redisClient    *redis.Client
}

type planRepository interface {
	GetAllPlans(ctx context.Context) ([]domain.Plan, error)
}

func NewPlanService(planRepository planRepository, redisClient *redis.Client) *planService {
	return &planService{
		planRepository: planRepository,
		redisClient:    redisClient,
	}
}
