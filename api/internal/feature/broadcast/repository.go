package broadcast

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Reader interface {
}

type Writer interface {
	Create(ctx context.Context, broadcast *Broadcast) error
}

type ReadWriter interface {
	Reader
	Writer
}

type broadcastRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *broadcastRepository {
	return &broadcastRepository{db}
}

func (r *broadcastRepository) Create(ctx context.Context, broadcast *Broadcast) error {
	query := `INSERT INTO broadcast (id, project_id, payload, created_at, expires_at) 
			  VALUES ($1, $2, $3, $4, $5)`
	_, err := r.db.Exec(ctx, query,
		broadcast.ID,
		broadcast.ProjectID,
		broadcast.Payload,
		broadcast.CreatedAt,
		broadcast.ExpiresAt,
	)

	if err != nil {
		return fmt.Errorf("insert broadcast: %w", err)
	}

	return nil
}
