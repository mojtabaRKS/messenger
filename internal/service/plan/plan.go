package plan

import (
	"arvan/message-gateway/internal/constant"
	"context"
	"encoding/json"
)

func (ps *planService) GetAllPlansAndSetInRedis(ctx context.Context) (map[string]int, error) {
	plans, err := ps.planRepository.GetAllPlans(ctx)
	if err != nil {
		return nil, err
	}

	data := make(map[string]int)
	for _, plan := range plans {
		data[plan.ApiKey] = plan.Priority
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	// set keys forever
	err = ps.redisClient.Set(ctx, constant.RedisPlanKey, jsonData, 0).Err()
	if err != nil {
		return nil, err
	}

	return data, nil
}
