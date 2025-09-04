package entity

type PlanID string

const (
	PlanFree PlanID = "free"
	PlanPro  PlanID = "pro"
)

type Metric string

const (
	MetricNotifications Metric = "notifications"
)

// entitlement holds per-metric limit + period (in days).
// nil Limit means "unlimited".
type entitlement struct {
	Metric     Metric
	Limit      *int64 // nil means unlimited
	PeriodDays int
}

type plan struct {
	ID           PlanID
	Description  string
	Entitlements map[Metric]entitlement
}

var notificationsLimitFree int64 = 10_000
var notificationsLimitPro int64 = 100_000

// allPlans holds all available plans.
var allPlans = map[PlanID]plan{
	"free": {
		ID:          PlanFree,
		Description: "Free tier with limited notifications",
		Entitlements: map[Metric]entitlement{
			MetricNotifications: {Metric: MetricNotifications, Limit: &notificationsLimitFree, PeriodDays: 30},
		},
	},
	"pro": {
		ID:          PlanPro,
		Description: "Pro tier with higher limits",
		Entitlements: map[Metric]entitlement{
			MetricNotifications: {Metric: MetricNotifications, Limit: &notificationsLimitPro, PeriodDays: 30},
		},
	},
}

func GetPlan(planID PlanID) (*plan, bool) {
	plan, exists := allPlans[planID]
	if !exists {
		return nil, false
	}
	return &plan, true
}
