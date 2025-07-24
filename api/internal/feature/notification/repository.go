package notification

import (
	"bodhveda/internal/common"
	"bodhveda/internal/dbx"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Reader interface {
	List(ctx context.Context, projectID uuid.UUID, recipient string, limit, offset int) ([]*Notification, int, error)
	UnreadCount(ctx context.Context, projectID uuid.UUID, recipient string) (int, error)
}

type Writer interface {
	Create(ctx context.Context, notification *Notification) error
	Materialize(ctx context.Context, notifications []*Notification) error
	MarkAsRead(ctx context.Context, projectID uuid.UUID, recipient string, ids []uuid.UUID) error
	MarkAllAsRead(ctx context.Context, projectID uuid.UUID, recipient string) (int, error)
	Delete(ctx context.Context, projectID uuid.UUID, recipient string, ids []uuid.UUID) error
	DeleteAll(ctx context.Context, projectID uuid.UUID, recipient string) (int, error)
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

func (r *notificationRepository) List(ctx context.Context, projectID uuid.UUID, recipient string, limit, offset int) ([]*Notification, int, error) {
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

func (r *notificationRepository) UnreadCount(ctx context.Context, projectID uuid.UUID, recipient string) (int, error) {
	b := dbx.NewSQLBuilder("SELECT COUNT(*) FROM notification")
	b.AddCompareFilter("project_id", "=", projectID)
	b.AddCompareFilter("recipient", "=", recipient)
	b.AppendWhere("read_at IS NULL")

	query, args := b.Build()
	var count int
	err := r.db.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return count, nil
}

func (r *notificationRepository) Materialize(ctx context.Context, notifications []*Notification) error {
	if len(notifications) == 0 {
		return nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Build INSERT INTO notification
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

	notifQuery := `INSERT INTO notification (id, project_id, recipient, broadcast_id, payload, read_at, created_at, expires_at) VALUES ` +
		strings.Join(valueStrings, ",")

	if _, err := tx.Exec(ctx, notifQuery, valueArgs...); err != nil {
		return fmt.Errorf("insert notifications: %w", err)
	}

	// Build INSERT INTO broadcast_materialization
	bmValueStrings := make([]string, 0, len(notifications))
	bmArgs := make([]any, 0, len(notifications)*2)

	now := time.Now().UTC()

	for _, n := range notifications {
		if n.BroadcastID == nil {
			continue // skip direct notifications
		}
		pos := len(bmArgs)
		bmValueStrings = append(bmValueStrings, fmt.Sprintf("($%d,$%d,$%d)", pos+1, pos+2, pos+3))
		bmArgs = append(bmArgs, *n.BroadcastID, n.Recipient, now)
	}

	if len(bmArgs) > 0 {
		bmQuery := `INSERT INTO broadcast_materialization (broadcast_id, recipient, created_at) VALUES ` +
			strings.Join(bmValueStrings, ",")

		if _, err := tx.Exec(ctx, bmQuery, bmArgs...); err != nil {
			return fmt.Errorf("insert broadcast_materialization: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (r *notificationRepository) MarkAsRead(ctx context.Context, projectID uuid.UUID, recipient string, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}

	now := time.Now().UTC()
	b := dbx.NewSQLBuilder("UPDATE notification")
	b.SetColumn("read_at", now)
	b.AddCompareFilter("project_id", "=", projectID)
	b.AddCompareFilter("recipient", "=", recipient)
	b.AppendWhere("read_at IS NULL")

	// Convert []uuid.UUID to []any for AddArrayFilter
	idVals := make([]any, len(ids))
	for i, id := range ids {
		idVals[i] = id
	}
	b.AddArrayFilter("id", idVals)

	query, args := b.Build()
	_, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("mark notifications as read: %w", err)
	}
	return nil
}

func (r *notificationRepository) MarkAllAsRead(ctx context.Context, projectID uuid.UUID, recipient string) (int, error) {
	now := time.Now().UTC()
	b := dbx.NewSQLBuilder("UPDATE notification")
	b.SetColumn("read_at", now)
	b.AddCompareFilter("project_id", "=", projectID)
	b.AddCompareFilter("recipient", "=", recipient)
	b.AppendWhere("read_at IS NULL")

	query, args := b.Build()
	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("mark all notifications as read: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

func (r *notificationRepository) Delete(ctx context.Context, projectID uuid.UUID, recipient string, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	b := dbx.NewSQLBuilder("DELETE FROM notification")
	b.AddCompareFilter("project_id", "=", projectID)
	b.AddCompareFilter("recipient", "=", recipient)
	idVals := make([]any, len(ids))
	for i, id := range ids {
		idVals[i] = id
	}
	b.AddArrayFilter("id", idVals)
	query, args := b.Build()
	_, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete notifications: %w", err)
	}
	return nil
}

func (r *notificationRepository) DeleteAll(ctx context.Context, projectID uuid.UUID, recipient string) (int, error) {
	b := dbx.NewSQLBuilder("DELETE FROM notification")
	b.AddCompareFilter("project_id", "=", projectID)
	b.AddCompareFilter("recipient", "=", recipient)
	query, args := b.Build()
	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("delete all notifications: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
