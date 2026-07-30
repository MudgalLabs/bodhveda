package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
)

type BroadcastBatchRepository interface {
	BroadcastBatchReader
	BroadcastBatchWriter
}

type BroadcastBatchReader interface {
	PendingCount(ctx context.Context, broadcastID int) (int, error)

	// StatusForUpdateTx reads a batch's status and holds a row lock on it for the
	// remainder of the transaction.
	//
	// The lock is the point, not the read. BroadcastDeliveryProcessor uses this to
	// decide whether a batch's fan-out has already been written, and a plain read
	// would let two workers holding the same batch both decide "not yet" and both
	// insert. Blocking the second until the first commits is what makes the
	// decision correct rather than merely likely.
	StatusForUpdateTx(ctx context.Context, tx pgx.Tx, batchID int) (enum.BroadcastBatchStatus, error)
}

type BroadcastBatchWriter interface {
	Create(ctx context.Context, broadcastBatch *entity.BroadcastBatch) (*entity.BroadcastBatch, error)
	Update(ctx context.Context, batchID int, paylaod *entity.BroadcastBatchUpdatePayload) error

	// UpdateTx is Update, enrolled in a caller's transaction so a batch's status
	// can be committed atomically with the work it describes.
	UpdateTx(ctx context.Context, tx pgx.Tx, batchID int, payload *entity.BroadcastBatchUpdatePayload) error

	DeleteForProject(ctx context.Context, projectID int) (int, error)
}
