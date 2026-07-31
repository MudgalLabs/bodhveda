package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/tantra/query"
)

type BroadcastRepository interface {
	BroadcastReader
	BroadcastWriter
}

type BroadcastReader interface {
	GetByID(ctx context.Context, id int) (*entity.Broadcast, error)
	List(ctx context.Context, projectID int, pagination query.Pagination) ([]*dto.BroadcastListItem, int, error)

	// StatusForUpdateTx reads a broadcast's status and holds a row lock on it for
	// the remainder of the transaction. The lock, not the read, is the point —
	// see the implementation.
	StatusForUpdateTx(ctx context.Context, tx pgx.Tx, broadcastID int) (enum.BroadcastStatus, error)
}

type BroadcastWriter interface {
	Create(ctx context.Context, notification *entity.Broadcast) (*entity.Broadcast, error)
	Update(ctx context.Context, notification *entity.Broadcast) error

	// UpdateTx is Update, enrolled in a caller's transaction.
	UpdateTx(ctx context.Context, tx pgx.Tx, notification *entity.Broadcast) error
	// SetAudience records the frozen audience breakdown, written once by
	// prepare_batches. Separate from Update on purpose — see the implementation.
	SetAudience(ctx context.Context, broadcastID int, audience *entity.BroadcastAudience) error

	// SetAudienceTx is SetAudience in the caller's transaction. ⚠️ Required, not
	// optional, for a caller holding the broadcast row via FOR UPDATE — see the
	// implementation.
	SetAudienceTx(ctx context.Context, tx pgx.Tx, broadcastID int, audience *entity.BroadcastAudience) error

	// SetEmailOutcomeTx records prepare_batches' decision about the email half:
	// the frozen eligible count and the block reason, if any. Deliberately does
	// not touch the email CONTENT columns — see the implementation.
	SetEmailOutcomeTx(ctx context.Context, tx pgx.Tx, broadcastID int, eligible int, blockedReason string) error
	DeleteForProject(ctx context.Context, projectID int) (int, error)
}
