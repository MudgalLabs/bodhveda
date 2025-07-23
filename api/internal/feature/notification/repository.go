package notification

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Reader interface {
}

type Writer interface {
	Create(ctx context.Context, notification *Notification) error
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
		return err
	}

	return nil
}
