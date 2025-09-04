package entity

import "time"

type UsageLog struct {
	ID        int
	ProjectID int
	Metric    Metric
	Amount    int64
	UsedAt    time.Time
	CreatedAt time.Time
}
