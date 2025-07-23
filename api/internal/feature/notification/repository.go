package notification

import (
	"bodhveda/internal/common"
	"bodhveda/internal/dbx"
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Reader interface {
	Inbox(ctx context.Context, projectID uuid.UUID, recipient string, limit, offset int) ([]*Notification, int, error)
}

type Writer interface {
	Create(ctx context.Context, notification *Notification) error
	batchCreate(ctx context.Context, notifications []*Notification) error
}

type ReadWriter interface {
	Reader
	Writer
}

type notificationRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *notificationRepository {
	return &notificationRepository{db}
}

func (r *notificationRepository) Create(ctx context.Context, notification *Notification) error {
	query := `INSERT INTO notification (id, project_id, recipient, broadcast_id, payload, read_at, created_at, expires_at) 
			  VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := r.db.Exec(ctx, query,
		notification.ID,
		notification.ProjectID,
		notification.Recipient,
		notification.BroadcastID,
		notification.Payload,
		notification.ReadAt,
		notification.CreatedAt,
		notification.ExpiresAt,
	)

	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}

	return nil
}

func (r *notificationRepository) Inbox(ctx context.Context, projectID uuid.UUID, recipient string, limit, offset int) ([]*Notification, int, error) {
	baseSQL := `SELECT id, project_id, recipient, broadcast_id, payload, read_at, created_at, expires_at
				FROM notification`

	b := dbx.NewSQLBuilder(baseSQL)

	if projectID != uuid.Nil {
		b.AddCompareFilter("project_id", "=", projectID)
	}

	if recipient != "" {
		b.AddCompareFilter("recipient", "=", recipient)
	}

	b.AddSorting("created_at", common.SortOrderDESC)

	b.AddPagination(limit, offset)

	query, args := b.Build()

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query notifications: %w", err)
	}
	defer rows.Close()

	notifications := []*Notification{}
	for rows.Next() {
		n := &Notification{}
		err := rows.Scan(
			&n.ID,
			&n.ProjectID,
			&n.Recipient,
			&n.BroadcastID,
			&n.Payload,
			&n.ReadAt,
			&n.CreatedAt,
			&n.ExpiresAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan notification: %w", err)
		}
		notifications = append(notifications, n)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows error: %w", err)
	}

	countQuery, countArgs := b.Count()
	var total int

	err = r.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count notifications: %w", err)
	}

	return notifications, total, nil
}

func (r *notificationRepository) batchCreate(ctx context.Context, notifications []*Notification) error {
	if len(notifications) == 0 {
		return nil
	}

	// Build placeholders and values
	valueStrings := make([]string, 0, len(notifications))
	valueArgs := make([]any, 0, len(notifications)*8)

	for i, n := range notifications {
		pos := i * 8
		valueStrings = append(valueStrings, fmt.Sprintf(
			"($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			pos+1, pos+2, pos+3, pos+4, pos+5, pos+6, pos+7, pos+8,
		))

		valueArgs = append(valueArgs,
			n.ID,
			n.ProjectID,
			n.Recipient,
			n.BroadcastID,
			n.Payload,
			n.ReadAt,
			n.CreatedAt,
			n.ExpiresAt,
		)
	}

	query := `INSERT INTO notification (id, project_id, recipient, broadcast_id, payload, read_at, created_at, expires_at) VALUES ` +
		strings.Join(valueStrings, ",")

	_, err := r.db.Exec(ctx, query, valueArgs...)
	if err != nil {
		return fmt.Errorf("batch insert notification: %w", err)
	}

	return nil
}
