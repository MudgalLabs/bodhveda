package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/tantra/query"
)

type NotificationRepository interface {
	NotificationReader
	NotificationWriter
}

type NotificationReader interface {
	Overview(ctx context.Context, projectID int) (*dto.NotificationsOverviewResult, error)
	ListForRecipient(ctx context.Context, projectID int, recipientExtID string, cursor *query.Cursor) ([]*entity.Notification, *query.Cursor, error)
	UnreadCountForRecipient(ctx context.Context, projectID int, recipientExtID string) (int, error)
	ListNotifications(ctx context.Context, projectID int, kind enum.NotificationKind, pagination query.Pagination) ([]*entity.Notification, int, error)
}

type NotificationWriter interface {
	Create(ctx context.Context, notification *entity.Notification) (*entity.Notification, error)
	BatchCreateTx(ctx context.Context, tx pgx.Tx, notifications []*entity.Notification) error
	UpdateForRecipient(ctx context.Context, projectID int, recipientExtID string, payload dto.UpdateRecipientNotificationsPayload) (int, error)
	DeleteForRecipient(ctx context.Context, projectID int, recipientExtID string, notificationIDs []int) (int, error)
	DeleteForProject(ctx context.Context, projectID int) (int, error)
}
