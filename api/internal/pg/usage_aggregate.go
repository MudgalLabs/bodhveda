package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
)

type UsageAggregateRepo struct {
	db   dbx.DBExecutor
	pool *pgxpool.Pool
}

func NewUsageAggregateRepo(db *pgxpool.Pool) repository.UsageAggregateRepository {
	return &UsageAggregateRepo{
		db:   db,
		pool: db,
	}
}

func (r *UsageAggregateRepo) Get(ctx context.Context, projectIDs []int, metric entity.Metric, periodStart, periodEnd time.Time) (int64, error) {
	var used int64

	err := r.db.QueryRow(ctx, `
        SELECT COALESCE(SUM(used), 0) AS used
        FROM usage_aggregate
        WHERE project_id = ANY($1) AND metric = $2 AND period_start = $3
        LIMIT 1;
    `, projectIDs, metric, periodStart).Scan(&used)

	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("querying usage aggregate: %w", err)
	}

	return used, nil
}
