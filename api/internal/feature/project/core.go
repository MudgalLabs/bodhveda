package project

import (
	"time"

	"github.com/google/uuid"
)

type Project struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	UserID    uuid.UUID  `json:"user_id" db:"user_id"` // The user who owns the project.
	Name      string     `json:"name" db:"name"`
	Plan      planName   `json:"plan" db:"plan"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt *time.Time `json:"updated_at" db:"updated_at"`
}

type planName string

const (
	PlanFree planName = "free"
)

// Plan represents the plan details for a project.
// TODO: Should this be a record in a table?
// When we create a new project, we assign a plan to it?
type Plan struct {
	Name             planName `json:"name"`
	MonthlyLimit     int
	DailyLimit       *int // nil if unlimited
	RetentionDays    int
	LogRetentionDays int
	PriceCents       int
	AllowBroadcast   bool
}

func GetPlan(plan planName) *Plan {
	switch plan {
	case PlanFree:
		dailyLimit := 100
		return &Plan{
			Name:             PlanFree,
			MonthlyLimit:     1000,
			DailyLimit:       &dailyLimit,
			RetentionDays:    30,
			LogRetentionDays: 7,
			PriceCents:       0,
			AllowBroadcast:   false,
		}
	default:
		return nil // or handle other plans
	}
}

func (p *Plan) NotificationExpiresAt() time.Time {
	// Notifications expire after the retention period.
	return time.Now().UTC().AddDate(0, 0, p.RetentionDays)
}
