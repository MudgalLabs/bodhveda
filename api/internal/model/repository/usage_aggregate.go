package repository

import (
	"context"
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
)

type UsageAggregateRepository interface {
	UsageAggreRegateRepositoryReader
	UsageAggregateRepositoryWriter
}

type UsageAggreRegateRepositoryReader interface {
	Get(ctx context.Context, projectID []int, metric entity.Metric, periodStart, periodEnd time.Time) (int64, error)
}

type UsageAggregateRepositoryWriter interface{}
