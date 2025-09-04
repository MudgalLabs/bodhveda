package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
)

type UsageLogRepository interface {
	UsageLogRepositoryReader
	UsageLogRepositoryWriter
}

type UsageLogRepositoryReader interface{}

type UsageLogRepositoryWriter interface {
	Add(ctx context.Context, tx pgx.Tx, projectID int, metric entity.Metric, amount int64, periodStart, periodEnd time.Time) error
}
