package entity

import (
	"encoding/json"
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/enum"
)

// NotificationDelivery is a per-(notification, medium) delivery record. In v1 it
// is written for EMAIL only — the in-app inbox outcome stays on the notification
// row (see agent-docs/overview.md).
//
// This is the FULL row, including the Phase 5 webhook columns and the unbounded
// ProviderResponse history. The notifications LIST uses the narrower
// NotificationEmailDelivery projection instead; this struct backs the
// per-notification delivery detail (Phase 9.1).
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
	DeliveredAt       *time.Time
	BouncedAt         *time.Time
	ComplainedAt      *time.Time
	// OpenedAt / ClickedAt are soft, directional signals (Apple MPP inflates
	// opens) — never treat them as equivalent to in-app `read`.
	OpenedAt  *time.Time
	ClickedAt *time.Time
	// ProviderResponse is a JSONB ARRAY of raw provider webhook bodies, appended
	// one per event by ApplyWebhookStatus (Phase 5). Unbounded — never project it
	// into a list response. Nil when no webhook has ever landed for this row.
	ProviderResponse json.RawMessage
	CreatedAt        time.Time
	UpdatedAt        time.Time
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
