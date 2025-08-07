package pg

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
)

type BroadcastRepo struct {
	db   dbx.DBExecutor
	pool *pgxpool.Pool
}

func NewBroadcastRepo(db *pgxpool.Pool) repository.BroadcastRepository {
	return &BroadcastRepo{
		db:   db,
		pool: db,
	}
}

func (r *BroadcastRepo) Create(ctx context.Context, broadcast *entity.Broadcast) (*entity.Broadcast, error) {
	sql := `
		INSERT INTO broadcast (
			project_id, payload, channel, topic, event, completed_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, project_id, payload, channel, topic, event, completed_at, created_at, updated_at
	`
	row := r.db.QueryRow(ctx, sql, broadcast.ProjectID, broadcast.Payload, broadcast.Channel, broadcast.Topic,
		broadcast.Event, broadcast.CompletedAt, broadcast.CreatedAt, broadcast.UpdatedAt,
	)

	var newBroadcast entity.Broadcast

	err := row.Scan(&newBroadcast.ID, &newBroadcast.ProjectID, &newBroadcast.Payload, &newBroadcast.Channel,
		&newBroadcast.Topic, &newBroadcast.Event, &newBroadcast.CompletedAt, &newBroadcast.CreatedAt,
		&newBroadcast.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan broadcast: %w", err)
	}

	return &newBroadcast, nil
}
