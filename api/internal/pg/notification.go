package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
	"github.com/mudgallabs/tantra/query"
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
			topic, event, read_at, opened_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, project_id, recipient_external_id, payload, broadcast_id, channel, topic, event, read_at, opened_at, created_at, updated_at
	`

	row := r.db.QueryRow(ctx, sql, notification.ProjectID, notification.RecipientExtID, notification.Payload,
		notification.BroadcastID, notification.Channel, notification.Topic, notification.Event,
		notification.ReadAt, notification.OpenedAt, notification.CreatedAt, notification.UpdatedAt)

	var newNotification entity.Notification
	err := row.Scan(&newNotification.ID, &newNotification.ProjectID, &newNotification.RecipientExtID,
		&newNotification.Payload, &newNotification.BroadcastID, &newNotification.Channel, &newNotification.Topic,
		&newNotification.Event, &newNotification.ReadAt, &newNotification.OpenedAt, &newNotification.CreatedAt, &newNotification.UpdatedAt,
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
			n.ReadAt,
			n.OpenedAt,
			n.CreatedAt,
			n.UpdatedAt,
		}
	}

	_, err := tx.CopyFrom(ctx, pgx.Identifier{"notification"}, []string{
		"project_id", "recipient_external_id", "payload", "broadcast_id",
		"channel", "topic", "event", "read_at", "opened_at", "created_at", "updated_at",
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

/*
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_notification_id_project_recipient
ON notification (id DESC, project_id, recipient_external_id);
*/

func (r *NotificationRepo) ListForRecipient(ctx context.Context, projectID int, recipientExtID string, cursor *query.Cursor) ([]*entity.Notification, *query.Cursor, error) {
	returnedCursor := &query.Cursor{
		After:  nil,
		Before: nil,
	}

	b := dbx.NewSQLBuilder(`
		SELECT id, project_id, recipient_external_id, payload, broadcast_id, channel, topic, event, read_at, opened_at, created_at, updated_at
		FROM notification
	`)
	b.AddCompareFilter("project_id", dbx.OperatorEQ, projectID)
	b.AddCompareFilter("recipient_external_id", dbx.OperatorEQ, recipientExtID)

	if cursor.Before != nil && *cursor.Before != "" && cursor.After == nil {
		b.AddCompareFilter("id", dbx.OperatorLT, cursor.Before)
	}

	if cursor.After != nil && *cursor.After != "" && cursor.Before == nil {
		b.AddCompareFilter("id", dbx.OperatorGT, cursor.After)
	}

	b.AddSorting("id", "DESC")
	b.AddPagination(*cursor.Limit, 0)

	sql, args := b.Build()

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("query: %w", err)
	}

	defer rows.Close()

	notifications := []*entity.Notification{}
	for rows.Next() {
		var notification entity.Notification

		err := rows.Scan(&notification.ID, &notification.ProjectID, &notification.RecipientExtID, &notification.Payload, &notification.BroadcastID, &notification.Channel, &notification.Topic, &notification.Event, &notification.ReadAt, &notification.OpenedAt, &notification.CreatedAt, &notification.UpdatedAt)
		if err != nil {
			return nil, nil, fmt.Errorf("scan: %w", err)
		}

		notifications = append(notifications, &notification)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("rows error: %w", err)
	}

	if len(notifications) > 0 {
		lastNotification := notifications[len(notifications)-1]
		before := fmt.Sprintf("%d", lastNotification.ID)
		after := fmt.Sprintf("%d", notifications[0].ID)
		returnedCursor.Before = &before
		returnedCursor.After = &after
	}

	return notifications, returnedCursor, nil
}

func (r *NotificationRepo) UnreadCountForRecipient(ctx context.Context, projectID int, recipientExtID string) (int, error) {
	sql := `
		SELECT COUNT(*) FROM notification
		WHERE project_id = $1 AND recipient_external_id = $2 AND read_at IS NULL
	`
	var count int

	err := r.db.QueryRow(ctx, sql, projectID, recipientExtID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("query and scan: %w", err)
	}

	return count, nil
}

func (r *NotificationRepo) MarkAsReadForRecipient(ctx context.Context, projectID int, recipientExtID string, notificationIDs []int) (int, error) {
	now := time.Now().UTC()
	sb := dbx.NewSQLBuilder("UPDATE notification")
	sb.SetColumn("read_at", now)
	sb.SetColumn("updated_at", now)
	sb.AddCompareFilter("project_id", dbx.OperatorEQ, projectID)
	sb.AddCompareFilter("recipient_external_id", dbx.OperatorEQ, recipientExtID)

	if notificationIDs == nil {
		// Mark all as read for the recipient
	} else if len(notificationIDs) == 0 {
		return 0, nil
	} else {
		// Mark only specific notifications as read
		ids := make([]any, len(notificationIDs))
		for i, id := range notificationIDs {
			ids[i] = id
		}
		sb.AddArrayFilter("id", ids)
	}

	sb.AppendWhere("read_at IS NULL")

	sql, args := sb.Build()
	res, err := r.db.Exec(ctx, sql, args...)
	if err != nil {
		return 0, fmt.Errorf("update notifications as read: %w", err)
	}

	return int(res.RowsAffected()), nil
}

func (r *NotificationRepo) MarkAsOpenedForRecipient(ctx context.Context, projectID int, recipientExtID string, notificationIDs []int) (int, error) {
	now := time.Now().UTC()
	sb := dbx.NewSQLBuilder("UPDATE notification")
	sb.SetColumn("opened_at", now)
	sb.SetColumn("updated_at", now)
	sb.AddCompareFilter("project_id", dbx.OperatorEQ, projectID)
	sb.AddCompareFilter("recipient_external_id", dbx.OperatorEQ, recipientExtID)

	if notificationIDs == nil {
		// Mark all as opened for the recipient
	} else if len(notificationIDs) == 0 {
		return 0, nil
	} else {
		// Mark only specific notifications as opened
		ids := make([]any, len(notificationIDs))
		for i, id := range notificationIDs {
			ids[i] = id
		}
		sb.AddArrayFilter("id", ids)
	}

	sb.AppendWhere("opened_at IS NULL")

	sql, args := sb.Build()
	res, err := r.db.Exec(ctx, sql, args...)
	if err != nil {
		return 0, fmt.Errorf("update notifications as opened: %w", err)
	}

	return int(res.RowsAffected()), nil
}

func (r *NotificationRepo) MarkAsUnreadForRecipient(ctx context.Context, projectID int, recipientExtID string, notificationIDs []int) (int, error) {
	if len(notificationIDs) == 0 {
		return 0, nil
	}

	sql := `
		UPDATE notification
		SET read_at = NULL, updated_at = $1
		WHERE project_id = $2
		  AND recipient_external_id = $3
		  AND id = ANY($4)
		  AND read_at IS NOT NULL
	`

	now := time.Now().UTC()

	res, err := r.db.Exec(ctx, sql, now, projectID, recipientExtID, notificationIDs)
	if err != nil {
		return 0, fmt.Errorf("update notifications as unread: %w", err)
	}

	return int(res.RowsAffected()), nil
}

func (r *NotificationRepo) DeleteForRecipient(ctx context.Context, projectID int, recipientExtID string, notificationIDs []int) (int, error) {
	sb := dbx.NewSQLBuilder("DELETE FROM notification")
	sb.AddCompareFilter("project_id", dbx.OperatorEQ, projectID)
	sb.AddCompareFilter("recipient_external_id", dbx.OperatorEQ, recipientExtID)

	if notificationIDs == nil {
		// Delete all for recipient (no id filter)
	} else if len(notificationIDs) == 0 {
		return 0, nil
	} else {
		ids := make([]any, len(notificationIDs))
		for i, id := range notificationIDs {
			ids[i] = id
		}
		sb.AddArrayFilter("id", ids)
	}

	sql, args := sb.Build()
	res, err := r.db.Exec(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	return int(res.RowsAffected()), nil
}

func (r *NotificationRepo) DeleteForProject(ctx context.Context, projectID int) (int, error) {
	sql := `
		DELETE FROM notification
		WHERE project_id = $1
	`
	res, err := r.db.Exec(ctx, sql, projectID)
	if err != nil {
		return 0, fmt.Errorf("delete notifications for project: %w", err)
	}
	return int(res.RowsAffected()), nil
}
