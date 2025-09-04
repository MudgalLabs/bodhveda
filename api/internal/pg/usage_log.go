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

type UsageLogRepo struct {
	db   dbx.DBExecutor
	pool *pgxpool.Pool
}

func NewUsageLogRepo(db *pgxpool.Pool) repository.UsageLogRepository {
	return &UsageLogRepo{
		db:   db,
		pool: db,
	}
}

func (r *UsageLogRepo) Add(ctx context.Context, tx pgx.Tx, projectID int, metric entity.Metric, amount int64, periodStart, periodEnd time.Time) error {
	now := time.Now().UTC()

	_, err := tx.Exec(ctx, `
        INSERT INTO usage_log (project_id, metric, amount, used_at)
        VALUES ($1, $2, $3, $4)
    `, projectID, metric, amount, now)
	if err != nil {
		return fmt.Errorf("inserting usage log: %w", err)
	}

	_, err = tx.Exec(ctx, `
        INSERT INTO usage_aggregate (project_id, metric, period_start, period_end, used)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (project_id, metric, period_start)
        DO UPDATE SET used = usage_aggregate.used + EXCLUDED.used;
    `, projectID, metric, periodStart, periodEnd, amount)
	if err != nil {
		return fmt.Errorf("upserting usage aggregate: %w", err)
	}

	return nil
}
