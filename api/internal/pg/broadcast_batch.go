package pg

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
)

type BroadcastBatchRepo struct {
	db   dbx.DBExecutor
	pool *pgxpool.Pool
}

func NewBroadcastBatchRepo(db *pgxpool.Pool) repository.BroadcastBatchRepository {
	return &BroadcastBatchRepo{
		db:   db,
		pool: db,
	}
}

func (r *BroadcastBatchRepo) Create(ctx context.Context, broadcastBatch *entity.BroadcastBatch) (*entity.BroadcastBatch, error) {
	sql := `
		INSERT INTO broadcast_batch (
			broadcast_id, recipients, status, attempt, duration, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, broadcast_id, recipients, status, attempt, duration, created_at, updated_at
	`
	row := r.db.QueryRow(ctx, sql, broadcastBatch.BroadcastID, broadcastBatch.Recipients, broadcastBatch.Status,
		broadcastBatch.Attempt, broadcastBatch.Duration, broadcastBatch.CreatedAt, broadcastBatch.UpdatedAt,
	)

	var newBatch entity.BroadcastBatch

	err := row.Scan(&newBatch.ID, &newBatch.BroadcastID, &newBatch.Recipients, &newBatch.Status, &newBatch.Attempt,
		&newBatch.Duration, &newBatch.CreatedAt, &newBatch.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &newBatch, nil
}

func (r *BroadcastBatchRepo) Update(ctx context.Context, batchID int, payload *entity.BroadcastBatchUpdatePayload) error {
	sql := `
		UPDATE broadcast_batch
		SET updated_at = $2, status = $3, attempt = $4, duration = $5
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, sql, batchID, time.Now().UTC(), payload.Status, payload.Attempt, payload.Duration)
	return err
}

func (r *BroadcastBatchRepo) PendingCount(ctx context.Context, broadcastID int) (int, error) {
	sql := `
		SELECT COUNT(id) FROM broadcast_batch
		WHERE broadcast_id = $1 AND status = $2
	`
	var count int
	err := r.db.QueryRow(ctx, sql, broadcastID, enum.BroadcastBatchStatusPending).Scan(&count)
	return count, err
}
