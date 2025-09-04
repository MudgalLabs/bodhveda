package dto

import "github.com/mudgallabs/bodhveda/internal/model/entity"

type UsageAggregate struct {
	UserID int           `json:"user_id"`
	Metric entity.Metric `json:"metric"`
	Used   int64         `json:"used"`
	Limit  int64         `json:"limit"`
}

func NewUsageAggregate(userID int, metric entity.Metric, used, limit int64) UsageAggregate {
	return UsageAggregate{
		UserID: userID,
		Metric: metric,
		Used:   used,
		Limit:  limit,
	}
}
