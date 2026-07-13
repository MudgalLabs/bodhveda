package repository

import (
	"context"

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
}

type NotificationDeliveryWriter interface {
	// Create inserts a delivery row (status already resolved by the caller).
	Create(ctx context.Context, delivery *entity.NotificationDelivery) (*entity.NotificationDelivery, error)
	// UpdateResult records the terminal outcome of a provider send attempt
	// (status + provider message id + failure reason + attempt + sent_at).
	UpdateResult(ctx context.Context, id int64, result NotificationDeliveryResult) error
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
