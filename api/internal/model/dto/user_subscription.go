package dto

import (
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
)

type UserSubscription struct {
	UserID             int           `json:"user_id"`
	PlanID             entity.PlanID `json:"plan_id"`
	CurrentPeriodStart time.Time     `json:"current_period_start"`
	CurrentPeriodEnd   time.Time     `json:"current_period_end"`
	CreatedAt          time.Time     `json:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at"`
}

func FromUserSubscription(sub *entity.UserSubscription) *UserSubscription {
	if sub == nil {
		return nil
	}

	return &UserSubscription{
		UserID:             sub.UserID,
		PlanID:             sub.PlanID,
		CurrentPeriodStart: sub.CurrentPeriodStart,
		CurrentPeriodEnd:   sub.CurrentPeriodEnd,
		CreatedAt:          sub.CreatedAt,
		UpdatedAt:          sub.UpdatedAt,
	}
}
