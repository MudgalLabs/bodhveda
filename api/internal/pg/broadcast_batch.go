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
			broadcast_id, status, attempt, duration, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, broadcast_id, status, attempt, duration, created_at, updated_at
	`
	row := r.db.QueryRow(ctx, sql, broadcastBatch.BroadcastID, broadcastBatch.Status, broadcastBatch.Attempt,
		broadcastBatch.Duration, broadcastBatch.CreatedAt, broadcastBatch.UpdatedAt,
	)

	var newBatch entity.BroadcastBatch
	err := row.Scan(&newBatch.ID, &newBatch.BroadcastID, &newBatch.Status, &newBatch.Attempt, &newBatch.Duration,
		&newBatch.CreatedAt, &newBatch.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &newBatch, nil
}

func (r *BroadcastBatchRepo) Update(ctx context.Context, batchID int, status enum.BroadcastBatchStatus, duration int) error {
	sql := `
		UPDATE broadcast_batch
		SET status = $1, duration = $2, updated_at = $3
		WHERE id = $4
	`
	_, err := r.db.Exec(ctx, sql, status, duration, time.Now().UTC(), batchID)
	return err
}
