package repository

import (
	"arvan/message-gateway/intrernal/domain"
	"arvan/message-gateway/intrernal/repository/entity"
	"context"
	"gorm.io/gorm"
)

type PlanRepository struct {
	db *gorm.DB
}

func NewPlanRepository(db *gorm.DB) *PlanRepository {
	return &PlanRepository{
		db: db,
	}
}

func (pr *PlanRepository) GetAllPlans(ctx context.Context) ([]domain.Plan, error) {
	dbPlans, err := gorm.G[entity.Plan](pr.db).Find(ctx)
	if err != nil {
		return nil, err
	}

	plans := make([]domain.Plan, 0)
	for _, plan := range dbPlans {
		dPlan := plan.ToDomain()
		plans = append(plans, dPlan)
	}

	return plans, nil
}
