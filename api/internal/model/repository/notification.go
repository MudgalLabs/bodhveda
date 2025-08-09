package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
)

type NotificationRepository interface {
	NotificationReader
	NotificationWriter
}

type NotificationReader interface {
	Overview(ctx context.Context, projectID int) (*dto.NotificationsOverviewResult, error)
	ListForRecipient(ctx context.Context, projectID int, recipientExtID string, before string, limit int) ([]*entity.Notification, error)
	UnreadCountForRecipient(ctx context.Context, projectID int, recipientExtID string) (int, error)
}

type NotificationWriter interface {
	Create(ctx context.Context, notification *entity.Notification) (*entity.Notification, error)
	BatchCreateTx(ctx context.Context, tx pgx.Tx, notifications []*entity.Notification) error
	// Pass nil for notificationIDs to mark all as read for the recipient.
	MarkAsReadForRecipient(ctx context.Context, projectID int, recipientExtID string, notificationIDs []int) (int, error)
	MarkAsUnreadForRecipient(ctx context.Context, projectID int, recipientExtID string, notificationIDs []int) (int, error)
	// Pass nil for notificationIDs to mark all as read for the recipient.
	MarkAsOpenedForRecipient(ctx context.Context, projectID int, recipientExtID string, notificationIDs []int) (int, error)
	// Pass nil for notificationIDs to delete all notifications for the recipient.
	DeleteForRecipient(ctx context.Context, projectID int, recipientExtID string, notificationIDs []int) (int, error)
}
