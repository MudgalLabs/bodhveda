package broadcast

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Reader interface {
	Unmaterialized(ctx context.Context, projectID uuid.UUID, recipient string) ([]*Broadcast, int, error)
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

func (r *broadcastRepository) Unmaterialized(ctx context.Context, projectID uuid.UUID, recipient string) ([]*Broadcast, int, error) {
	query := `
		SELECT b.id, b.project_id, b.payload, b.created_at, b.expires_at FROM broadcast b
		LEFT JOIN notification n ON b.id = n.broadcast_id
		AND n.recipient = $1
		AND n.project_id = $2
		WHERE b.project_id = $2
		AND b.expires_at > NOW()
		AND n.id IS NULL;`

	rows, err := r.db.Query(ctx, query, recipient, projectID)
	if err != nil {
		return nil, 0, fmt.Errorf("query unmaterialized broadcasts: %w", err)
	}
	defer rows.Close()

	var broadcasts []*Broadcast
	for rows.Next() {
		var b Broadcast
		if err := rows.Scan(&b.ID, &b.ProjectID, &b.Payload, &b.CreatedAt, &b.ExpiresAt); err != nil {
			return nil, 0, fmt.Errorf("scan broadcast: %w", err)
		}
		broadcasts = append(broadcasts, &b)
	}

	if rows.Err() != nil {
		return nil, 0, fmt.Errorf("rows error: %w", rows.Err())
	}

	return broadcasts, len(broadcasts), nil
}
