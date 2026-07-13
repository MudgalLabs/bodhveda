package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
)

type NotificationDeliveryRepository interface {
	NotificationDeliveryReader
	NotificationDeliveryWriter
}

type NotificationDeliveryReader interface {
	// Get returns a delivery row by id.
	Get(ctx context.Context, id int64) (*entity.NotificationDelivery, error)
	// ListForNotification returns all delivery rows for a notification.
	ListForNotification(ctx context.Context, notificationID int) ([]*entity.NotificationDelivery, error)
	// EmailDeliveryOverviewForProject aggregates email delivery rows into
	// per-status counts for the console analytics view (Phase 5).
	EmailDeliveryOverviewForProject(ctx context.Context, projectID int) (*dto.EmailDeliveryOverview, error)
}

type NotificationDeliveryWriter interface {
	// Create inserts a delivery row (status already resolved by the caller).
	Create(ctx context.Context, delivery *entity.NotificationDelivery) (*entity.NotificationDelivery, error)
	// UpdateResult records the terminal outcome of a provider send attempt
	// (status + provider message id + failure reason + attempt + sent_at).
	UpdateResult(ctx context.Context, id int64, result NotificationDeliveryResult) error
	// ApplyWebhookStatus transitions the delivery row matched by
	// provider_message_id in response to an inbound provider webhook (Phase 5).
	// It is order-tolerant, idempotent, and non-regressing: a terminal status
	// (bounced/complained/failed) is sticky and a later `delivered` must not
	// overwrite it. Returns ErrNotFound when no row matches the message id.
	ApplyWebhookStatus(ctx context.Context, update DeliveryWebhookUpdate) error
}

// DeliveryWebhookUpdate carries a normalized provider event applied to the
// delivery row keyed by ProviderMessageID.
//
//   - Status is the target status for this event, or nil for soft signals
//     (opened/clicked) that stamp a timestamp without changing status.
//   - Kind selects which `*_at` column is stamped (delivered/bounced/complained/
//     opened/clicked). The stamp is first-write-wins (idempotent).
//   - RawEvent is appended to provider_response for audit.
type DeliveryWebhookUpdate struct {
	ProviderMessageID string
	Status            *enum.DeliveryStatus
	Kind              string
	At                time.Time
	RawEvent          json.RawMessage
}

// NotificationDeliveryResult carries the fields the worker updates after a send
// attempt.
type NotificationDeliveryResult struct {
	Status            enum.DeliveryStatus
	Provider          *string
	ProviderMessageID *string
	FailureReason     *string
	Attempt           int
}
