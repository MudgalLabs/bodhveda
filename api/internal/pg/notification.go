package pg

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
)

type NotificationRepo struct {
	db   dbx.DBExecutor
	pool *pgxpool.Pool
}

func NewNotificationRepo(db *pgxpool.Pool) repository.NotificationRepository {
	return &NotificationRepo{
		db:   db,
		pool: db,
	}
}

func (r *NotificationRepo) Create(ctx context.Context, notification *entity.Notification) (*entity.Notification, error) {
	sql := `
		INSERT INTO notification (
			project_id, recipient_external_id, payload, broadcast_id, channel,
			topic, event, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, project_id, recipient_external_id, payload, broadcast_id, channel, topic, event, created_at, updated_at
	`

	row := r.db.QueryRow(ctx, sql, notification.ProjectID, notification.RecipientExtID, notification.Payload,
		notification.BroadcastID, notification.Channel, notification.Topic, notification.Event,
		notification.CreatedAt, notification.UpdatedAt)

	var newNotification entity.Notification
	err := row.Scan(&newNotification.ID, &newNotification.ProjectID, &newNotification.RecipientExtID,
		&newNotification.Payload, &newNotification.BroadcastID, &newNotification.Channel, &newNotification.Topic,
		&newNotification.Event, &newNotification.CreatedAt, &newNotification.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert notification: %w", err)
	}

	return &newNotification, nil
}

func (r *NotificationRepo) BatchCreateTx(ctx context.Context, tx pgx.Tx, notifications []*entity.Notification) error {
	rows := make([][]any, len(notifications))
	for i, n := range notifications {
		rows[i] = []any{
			n.ProjectID,
			n.RecipientExtID,
			n.Payload,
			n.BroadcastID,
			n.Channel,
			n.Topic,
			n.Event,
			n.CreatedAt,
			n.UpdatedAt,
		}
	}

	_, err := tx.CopyFrom(ctx, pgx.Identifier{"notification"}, []string{
		"project_id", "recipient_external_id", "payload", "broadcast_id",
		"channel", "topic", "event", "created_at", "updated_at",
	}, pgx.CopyFromRows(rows))

	return err
}

func (r *NotificationRepo) Overview(ctx context.Context, projectID int) (*dto.NotificationsOverviewResult, error) {
	sql := `
		SELECT
		    COUNT(*) FILTER (WHERE n.broadcast_id IS NULL) AS total_direct_sent,
		    COUNT(DISTINCT b.id) AS total_broadcast_sent,
		    COUNT(*) AS total_notifications
		FROM notification n
		LEFT JOIN broadcast b ON n.broadcast_id = b.id
		WHERE n.project_id = $1;
	`

	result := &dto.NotificationsOverviewResult{}

	err := r.db.QueryRow(ctx, sql, projectID).Scan(
		&result.TotalDirectSent,
		&result.TotalBroadcastSent,
		&result.TotalNotifications,
	)
	if err != nil {
		return nil, fmt.Errorf("overview query: %w", err)
	}

	return result, nil
}
