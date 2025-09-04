package dto

import "github.com/mudgallabs/bodhveda/internal/model/entity"

type UsageEvent struct {
	UserID    int
	ProjectID int
	Metric    entity.Metric
	Amount    int64
}
