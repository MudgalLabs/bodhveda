package entity

import (
	"encoding/json"
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/enum"
)

type Notification struct {
	ID             int
	ProjectID      int
	RecipientExtID string
	Payload        json.RawMessage
	BroadcastID    *int
	Channel        string
	Topic          string
	Event          string
	ReadAt         *time.Time
	OpenedAt       *time.Time
	Status         enum.NotificationStatus
	CompletedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time

	// Email delivery summary for this notification's email medium. Populated
	// ONLY by ListNotifications (batch-joined from notification_delivery);
	// nil when the send included no email. The in-app outcome stays on Status
	// above — email has its own lifecycle (pending → sent → delivered → …).
	Email *NotificationEmailDelivery
}

// NotificationEmailDelivery is the email-medium delivery summary attached to a
// listed notification. It carries every BOUNDED column of the delivery row, so
// the list can explain an outcome (failure_reason) and the detail dialog can
// render the lifecycle without a second fetch.
//
// It deliberately omits provider_response — the raw webhook event history is
// unbounded (one provider event body appended per webhook, Phase 5) and would
// ride every list row on every refetch. It is served per-notification by
// NotificationDeliveryReader.ListForNotification instead. See
// agent-docs/overview.md, "Phase 9.1 — deviations (as built)".
type NotificationEmailDelivery struct {
	Status            enum.DeliveryStatus
	FailureReason     *string
	Attempt           int
	Provider          *string
	ProviderMessageID *string
	AddressSnapshot   *string
	SentAt            *time.Time
	DeliveredAt       *time.Time
	BouncedAt         *time.Time
	ComplainedAt      *time.Time
	// OpenedAt / ClickedAt are soft, directional signals — provider open
	// tracking is unreliable (Apple Mail Privacy Protection pre-fetches images).
	// In-app ReadAt above is the trustworthy signal.
	OpenedAt  *time.Time
	ClickedAt *time.Time
}

func NewNotification(projectID int, recipientExtID string, payload json.RawMessage, broadcastID *int, channel, topic, event string) *Notification {
	now := time.Now().UTC()

	return &Notification{
		ProjectID:      projectID,
		RecipientExtID: recipientExtID,
		Payload:        payload,
		BroadcastID:    broadcastID,
		Channel:        channel,
		Topic:          topic,
		Event:          event,
		ReadAt:         nil,
		OpenedAt:       nil,
		Status:         enum.NotificationStatusEnqueued,
		CompletedAt:    nil,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
