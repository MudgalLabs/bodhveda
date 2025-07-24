package broadcast

import (
	"bodhveda/internal/common"
	"bodhveda/internal/dbx"
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Reader interface {
	List(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]*Broadcast, int, error)
	Unmaterialized(ctx context.Context, projectID uuid.UUID, recipient string) ([]*Broadcast, int, error)
}

type Writer interface {
	Create(ctx context.Context, broadcast *Broadcast) error
	Delete(ctx context.Context, projectID uuid.UUID, ids []uuid.UUID) error
	DeleteAll(ctx context.Context, projectID uuid.UUID) (int, error)
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

func (r *broadcastRepository) Delete(ctx context.Context, projectID uuid.UUID, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	b := dbx.NewSQLBuilder("DELETE FROM broadcast")
	b.AddCompareFilter("project_id", "=", projectID)
	idVals := make([]any, len(ids))
	for i, id := range ids {
		idVals[i] = id
	}
	b.AddArrayFilter("id", idVals)
	query, args := b.Build()
	_, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete broadcasts: %w", err)
	}
	return nil
}

func (r *broadcastRepository) DeleteAll(ctx context.Context, projectID uuid.UUID) (int, error) {
	b := dbx.NewSQLBuilder("DELETE FROM broadcast")
	b.AddCompareFilter("project_id", "=", projectID)
	query, args := b.Build()
	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("delete all broadcasts: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

func (r *broadcastRepository) Unmaterialized(ctx context.Context, projectID uuid.UUID, recipient string) ([]*Broadcast, int, error) {
	query := `
		SELECT b.id, b.project_id, b.payload, b.created_at, b.expires_at
		FROM broadcast b
		LEFT JOIN broadcast_materialization bm
			ON bm.broadcast_id = b.id AND bm.recipient = $1
		WHERE b.project_id = $2
		  AND b.expires_at > NOW()
		  AND bm.broadcast_id IS NULL;
	`

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

func (r *broadcastRepository) List(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]*Broadcast, int, error) {
	baseSQL := `SELECT id, project_id, payload, created_at, expires_at
				FROM broadcast`

	b := dbx.NewSQLBuilder(baseSQL)

	if projectID != uuid.Nil {
		b.AddCompareFilter("project_id", "=", projectID)
	}

	b.AddSorting("created_at", common.SortOrderDESC)

	b.AddPagination(limit, offset)

	query, args := b.Build()

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query broadcasts: %w", err)
	}
	defer rows.Close()

	broadcasts := []*Broadcast{}
	for rows.Next() {
		b := &Broadcast{}
		err := rows.Scan(
			&b.ID,
			&b.ProjectID,
			&b.Payload,
			&b.CreatedAt,
			&b.ExpiresAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan broadcast: %w", err)
		}
		broadcasts = append(broadcasts, b)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows error: %w", err)
	}

	countQuery, countArgs := b.Count()
	var total int

	err = r.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count broadcasts: %w", err)
	}

	return broadcasts, total, nil
}
