package entity

import "time"

const (
	SubscriptionRenewalGracePeriod = 3 * 24 * time.Hour // 3 days grace period for renewal
)

type UserSubscription struct {
	UserID             int
	PlanID             PlanID
	CurrentPeriodStart time.Time
	CurrentPeriodEnd   time.Time
	UpdatedAt          time.Time
	CreatedAt          time.Time
}

func NewUserSubscription(userID int, planID PlanID) *UserSubscription {
	now := time.Now().UTC()
	periodEnd := now.AddDate(0, 1, 0)

	return &UserSubscription{
		UserID:             userID,
		PlanID:             planID,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   periodEnd,
		UpdatedAt:          now,
		CreatedAt:          now,
	}
}

func RenewSubscription(userID int, planID PlanID, createdAt time.Time) *UserSubscription {
	sub := NewUserSubscription(userID, planID)
	sub.CreatedAt = createdAt
	return sub
}
