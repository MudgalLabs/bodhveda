package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
)

type UsageAggregateRepository interface {
	UsageAggreRegateRepositoryReader
	UsageAggregateRepositoryWriter
}

type UsageAggreRegateRepositoryReader interface {
	Get(ctx context.Context, projectID []int, metric entity.Metric, periodStart, periodEnd time.Time) (int64, error)
	// ⚠️ Read inside the caller's transaction by CheckAndConsumeUsageTx — see there.
	GetTx(ctx context.Context, tx pgx.Tx, projectID []int, metric entity.Metric, periodStart, periodEnd time.Time) (int64, error)
}

type UsageAggregateRepositoryWriter interface{}
