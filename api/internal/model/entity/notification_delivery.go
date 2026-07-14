package entity

import (
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/enum"
)

// NotificationDelivery is a per-(notification, medium) delivery record. In v1 it
// is written for EMAIL only — the in-app inbox outcome stays on the notification
// row (see agent-docs/overview.md). The struct exposes the subset of the
// `notification_delivery` columns v1 touches; the remaining columns
// (delivered_at, bounced_at, ...) exist in the table for Phase 5 webhooks.
type NotificationDelivery struct {
	ID                int64
	NotificationID    int
	ProjectID         int
	RecipientExtID    string
	Medium            enum.Medium
	ContactID         *int64
	AddressSnapshot   *string
	Status            enum.DeliveryStatus
	Provider          *string
	ProviderMessageID *string
	FailureReason     *string
	Attempt           int
	SentAt            *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// NewNotificationDelivery builds a delivery record with a resolved status. For a
// sendable email it is DeliveryPending (address_snapshot + contact captured);
// for a skipped outcome it is one of muted/no_contact/failed with a
// failure_reason.
func NewNotificationDelivery(
	notificationID, projectID int, recipientExtID string, medium enum.Medium,
	status enum.DeliveryStatus,
) *NotificationDelivery {
	now := time.Now().UTC()
	return &NotificationDelivery{
		NotificationID: notificationID,
		ProjectID:      projectID,
		RecipientExtID: recipientExtID,
		Medium:         medium,
		Status:         status,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
