package entity

import "time"

type UsageAggregate struct {
	ProjectID   int
	Metric      Metric
	PeriodStart time.Time
	PeriodEnd   time.Time
	Used        int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewUsageAggregate(projectID int, metric Metric, periodStart, periodEnd time.Time) *UsageAggregate {
	now := time.Now().UTC()
	return &UsageAggregate{
		ProjectID:   projectID,
		Metric:      metric,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		Used:        0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
