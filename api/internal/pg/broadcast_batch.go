package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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
	return createBroadcastBatch(ctx, r.db, broadcastBatch)
}

func (r *BroadcastBatchRepo) CreateTx(ctx context.Context, tx pgx.Tx, broadcastBatch *entity.BroadcastBatch) (*entity.BroadcastBatch, error) {
	return createBroadcastBatch(ctx, tx, broadcastBatch)
}

func createBroadcastBatch(ctx context.Context, db dbx.DBExecutor, broadcastBatch *entity.BroadcastBatch) (*entity.BroadcastBatch, error) {
	sql := `
		INSERT INTO broadcast_batch (
			broadcast_id, recipients, recipient_external_ids, status, attempt, duration, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, broadcast_id, recipients, recipient_external_ids, status, attempt, duration, created_at, updated_at
	`
	row := db.QueryRow(ctx, sql, broadcastBatch.BroadcastID, broadcastBatch.Recipients, broadcastBatch.RecipientExtIDs,
		broadcastBatch.Status, broadcastBatch.Attempt, broadcastBatch.Duration, broadcastBatch.CreatedAt, broadcastBatch.UpdatedAt,
	)

	var newBatch entity.BroadcastBatch

	err := row.Scan(&newBatch.ID, &newBatch.BroadcastID, &newBatch.Recipients, &newBatch.RecipientExtIDs,
		&newBatch.Status, &newBatch.Attempt, &newBatch.Duration, &newBatch.CreatedAt, &newBatch.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &newBatch, nil
}

// ResumableForBroadcastTx returns the broadcast's batches that are still
// `enqueued`, so a retry can re-enqueue exactly the outstanding ones.
//
// ⚠️ Batches with a NULL recipient_external_ids predate the column and are
// skipped here rather than returned empty — see the migration. Delivering to an
// empty set would silently mark such a batch successful having reached nobody.
func (r *BroadcastBatchRepo) ResumableForBroadcastTx(ctx context.Context, tx pgx.Tx, broadcastID int) ([]*entity.BroadcastBatch, error) {
	sql := `
		SELECT id, broadcast_id, recipients, recipient_external_ids, status, attempt, duration, created_at, updated_at
		FROM broadcast_batch
		WHERE broadcast_id = $1 AND status = $2 AND recipient_external_ids IS NOT NULL
		ORDER BY id
	`

	rows, err := tx.Query(ctx, sql, broadcastID, enum.BroadcastBatchStatusEnqueued)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var batches []*entity.BroadcastBatch

	for rows.Next() {
		var b entity.BroadcastBatch
		if err := rows.Scan(&b.ID, &b.BroadcastID, &b.Recipients, &b.RecipientExtIDs,
			&b.Status, &b.Attempt, &b.Duration, &b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			return nil, err
		}
		batches = append(batches, &b)
	}

	return batches, rows.Err()
}

// CountForBroadcastTx counts every batch of a broadcast, regardless of status.
// Non-zero means preparation already ran, which is what tells a retry not to
// bill or fan out a second time.
func (r *BroadcastBatchRepo) CountForBroadcastTx(ctx context.Context, tx pgx.Tx, broadcastID int) (int, error) {
	var count int
	err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM broadcast_batch WHERE broadcast_id = $1`, broadcastID).Scan(&count)
	return count, err
}

func (r *BroadcastBatchRepo) Update(ctx context.Context, batchID int, payload *entity.BroadcastBatchUpdatePayload) error {
	return updateBroadcastBatch(ctx, r.db, batchID, payload)
}

func (r *BroadcastBatchRepo) UpdateTx(ctx context.Context, tx pgx.Tx, batchID int, payload *entity.BroadcastBatchUpdatePayload) error {
	return updateBroadcastBatch(ctx, tx, batchID, payload)
}

func updateBroadcastBatch(ctx context.Context, db dbx.DBExecutor, batchID int, payload *entity.BroadcastBatchUpdatePayload) error {
	sql := `
		UPDATE broadcast_batch
		SET updated_at = $2, status = $3, attempt = $4, duration = $5
		WHERE id = $1
	`
	_, err := db.Exec(ctx, sql, batchID, time.Now().UTC(), payload.Status, payload.Attempt, payload.Duration)
	return err
}

// StatusForUpdateTx takes a row lock on the batch and returns its status. See the
// interface doc for why the lock — not the read — is the reason this exists.
func (r *BroadcastBatchRepo) StatusForUpdateTx(ctx context.Context, tx pgx.Tx, batchID int) (enum.BroadcastBatchStatus, error) {
	sql := `SELECT status FROM broadcast_batch WHERE id = $1 FOR UPDATE`

	var status enum.BroadcastBatchStatus

	if err := tx.QueryRow(ctx, sql, batchID).Scan(&status); err != nil {
		return "", err
	}

	return status, nil
}

// PendingCount counts a broadcast's batches that have not succeeded.
//
// ⚠️ NOT-SUCCEEDED, not `enqueued`. It used to count only `enqueued`, which meant
// a `failed` batch was invisible here: `remaining` reached 0 and the broadcast was
// marked `completed` while one of its batches had delivered nothing. With in-app
// only that was a cosmetic lie; with email it reads as "this broadcast completed"
// when a thousand people were never mailed.
//
// A failed batch IS still outstanding — delivery returns its error now, so Asynq
// retries it and the batch-row guard makes the retry safe. If retries are
// exhausted the broadcast stays non-completed, which is the honest state: it did
// not complete.
func (r *BroadcastBatchRepo) PendingCount(ctx context.Context, broadcastID int) (int, error) {
	sql := `
		SELECT COUNT(id) FROM broadcast_batch
		WHERE broadcast_id = $1 AND status <> $2
	`
	var count int
	err := r.db.QueryRow(ctx, sql, broadcastID, enum.BroadcastBatchStatusSuccess).Scan(&count)
	return count, err
}

func (r *BroadcastBatchRepo) DeleteForProject(ctx context.Context, projectID int) (int, error) {
	sql := `
		DELETE FROM broadcast_batch bb
		USING broadcast b
		WHERE bb.broadcast_id = b.id AND b.project_id = $1

	`

	tag, err := r.db.Exec(ctx, sql, projectID)
	if err != nil {
		return 0, fmt.Errorf("delete broadcast batches for project: %w", err)
	}

	return int(tag.RowsAffected()), nil
}
