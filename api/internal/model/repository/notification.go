package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/tantra/query"
)

type NotificationRepository interface {
	NotificationReader
	NotificationWriter
}

type NotificationReader interface {
	// Get returns one notification by id, scoped to the project, with its
	// email-medium delivery outcome attached (nil when there was no email send).
	// Returns tantra repository.ErrNotFound when no such row exists in the project.
	Get(ctx context.Context, projectID, id int) (*entity.Notification, error)
	Overview(ctx context.Context, projectID int) (*dto.NotificationsOverviewResult, error)
	ListForRecipient(ctx context.Context, projectID int, recipientExtID string, cursor *query.Cursor) ([]*entity.Notification, *query.Cursor, error)
	UnreadCountForRecipient(ctx context.Context, projectID int, recipientExtID string) (int, error)
	ListNotifications(ctx context.Context, filters *dto.ListNotificationsFilters) ([]*entity.Notification, int, error)
	// InAppAnalyticsSeries returns per-day in-app notification counts over a date
	// range, bucketed by day in the viewer's timezone `tz` (Phase 9.5).
	InAppAnalyticsSeries(ctx context.Context, projectID int, from, to *time.Time, tz string) ([]dto.AnalyticsInAppDay, error)
	// TargetVolumes returns the top `limit` targets by in-app notification volume
	// over the range (Phase 9.5).
	TargetVolumes(ctx context.Context, projectID int, from, to *time.Time, limit int) ([]dto.AnalyticsTargetStat, error)
}

type NotificationWriter interface {
	Create(ctx context.Context, notification *entity.Notification) (*entity.Notification, error)
	BatchCreateTx(ctx context.Context, tx pgx.Tx, notifications []*entity.Notification) error
	Update(ctx context.Context, notification *entity.Notification) error
	UpdateForRecipient(ctx context.Context, projectID int, recipientExtID string, payload dto.UpdateRecipientNotificationsPayload) (int, error)
	DeleteForRecipient(ctx context.Context, projectID int, recipientExtID string, notificationIDs []int) (int, error)
	DeleteForProject(ctx context.Context, projectID int) (int, error)
}
