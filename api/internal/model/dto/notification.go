package dto

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/tantra/apires"
	"github.com/mudgallabs/tantra/query"
	"github.com/mudgallabs/tantra/service"
)

type Notification struct {
	ID             int                     `json:"id"`
	RecipientExtID string                  `json:"recipient_id"`
	Payload        json.RawMessage         `json:"payload"`
	BroadcastID    *int                    `json:"broadcast_id"`
	Target         Target                  `json:"target"`
	State          NotificationState       `json:"state"`
	Status         enum.NotificationStatus `json:"status"`
	CompletedAt    *time.Time              `json:"completed_at,omitempty"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

type NotificationState struct {
	Opened bool `json:"opened"`
	Read   bool `json:"read"`
}

type NotificationStateFilter struct {
	Opened *bool `schema:"opened"`
	Read   *bool `schema:"read"`
}

func FromNotification(notification *entity.Notification) *Notification {
	dto := &Notification{
		ID:             notification.ID,
		RecipientExtID: notification.RecipientExtID,
		Payload:        notification.Payload,
		Target: Target{
			Channel: notification.Channel,
			Topic:   notification.Topic,
			Event:   notification.Event,
		},
		State: NotificationState{
			Read:   notification.ReadAt != nil,
			Opened: notification.OpenedAt != nil,
		},
		BroadcastID: notification.BroadcastID,
		Status:      notification.Status,
		CompletedAt: notification.CompletedAt,
		CreatedAt:   notification.CreatedAt,
		UpdatedAt:   notification.UpdatedAt,
	}

	return dto
}

type Target struct {
	Channel string `json:"channel"`
	// Cannot be "any" as that's reserved for preferences and it makes no sense to
	// send notifications to "any" topic. Although "none" is allowed.
	Topic string `json:"topic"`
	Event string `json:"event"`
}

func TargetFromBroadcast(broadcast *entity.Broadcast) Target {
	return Target{
		Channel: broadcast.Channel,
		Topic:   broadcast.Topic,
		Event:   broadcast.Event,
	}
}

func TargetFromNotification(notification *entity.Notification) Target {
	return Target{
		Channel: notification.Channel,
		Topic:   notification.Topic,
		Event:   notification.Event,
	}
}

func TargetFromPreference(pref *entity.Preference) Target {
	return Target{
		Channel: pref.Channel,
		Topic:   pref.Topic,
		Event:   pref.Event,
	}
}

// EmailContent is the typed sibling `email` block on a send call. Its presence
// is the sender's "email is eligible for this send" signal (content-block-implies-
// intent — see agent-docs/overview.md, "Semantics"). Absence ⇒ no email; there
// is NO fallback that derives email from `payload`.
//
// Bodhveda is a pass-through in v1: the caller renders its own template and
// passes the result. Subject is required; at least one of HTML/Text must be
// present. Text is recommended for deliverability and is auto-derived from HTML
// when omitted.
type EmailContent struct {
	Subject string `json:"subject"`
	HTML    string `json:"html"`
	Text    string `json:"text"`
}

// ResolvedText returns Text, or a naive plain-text rendering of HTML when Text
// was omitted (deliverability aid). It is intentionally minimal — real callers
// (e.g. @react-email's render()) supply their own text.
func (e *EmailContent) ResolvedText() string {
	if strings.TrimSpace(e.Text) != "" {
		return e.Text
	}
	return htmlToText(e.HTML)
}

// htmlToText strips tags for a rough text/plain alternative. Not a full renderer.
func htmlToText(html string) string {
	var b strings.Builder
	inTag := false
	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			b.WriteByte(' ')
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(strings.Join(strings.Fields(b.String()), " "))
}

type SendNotificationPayload struct {
	ProjectID int

	// RecipientExtID is the ID of the recipient for the notification.
	// Optional, if nil then it's a broadcast notification, if present then it's a direct notification.
	RecipientExtID *string `json:"recipient_id"`

	// Optional for direct notifications, but required for broadcast notifications.
	Target *Target `json:"target"`

	// Payload is the actual notification payload (the in-app/default content).
	// TODO: Add a 4KB limit to this field.
	Payload json.RawMessage `json:"payload"`

	// Email, when present, makes email eligible for this send (direct-only).
	// Absence ⇒ no email. See EmailContent.
	Email *EmailContent `json:"email"`
}

// HasEmail reports whether the send carries an email content block (the sender's
// intent-to-email signal).
func (p *SendNotificationPayload) HasEmail() bool {
	return p.Email != nil
}

func (p *SendNotificationPayload) Validate() error {
	var errs service.InputValidationErrors

	if p.ProjectID <= 0 {
		errs.Add(apires.NewApiError("Project is required", "Project ID must be a positive integer", "project_id", p.ProjectID))
	}

	if p.RecipientExtID != nil && *p.RecipientExtID == "" {
		errs.Add(apires.NewApiError("Recipient ID cannot be empty if provided", "Recipient ID cannot be empty if this field is provided. Omit the field if you want to send a broadcast notification.", "recipient_id", p.RecipientExtID))
	} else if p.RecipientExtID != nil {
		lowered := strings.ToLower(*p.RecipientExtID)
		p.RecipientExtID = &lowered
	}

	// If RecipientExtID is nil, then it's a broadcast notification.
	// We need to ensure that valid channel, topic, and event are provided, if this is a broadcast notification
	// OR even if it's a direct notification, but a value was provided for channel/topic/event.
	if p.RecipientExtID == nil || (p.Target != nil && (p.Target.Channel != "" || p.Target.Topic != "" || p.Target.Event != "")) {
		if p.Target.Channel == "" {
			errs.Add(apires.NewApiError("Channel is required", "Channel cannot be empty", "channel", p.Target.Channel))
		}

		switch p.Target.Topic {
		case "":
			errs.Add(apires.NewApiError("Topic is required", "Topic cannot be empty", "topic", p.Target.Topic))
		case "any":
			errs.Add(apires.NewApiError("Invalid topic", "Topic cannot be 'any'. It's reserved for creating project preferences.", "topic", p.Target.Topic))
		}

		if p.Target.Event == "" {
			errs.Add(apires.NewApiError("Event is required", "Event cannot be empty", "event", p.Target.Event))
		}
	}

	// Email block: email is DIRECT-only (HARD RULE — never on broadcast). Reject
	// an email block on a broadcast rather than silently dropping it.
	if p.Email != nil {
		if p.RecipientExtID == nil {
			errs.Add(apires.NewApiError("Email not supported on broadcasts", "The 'email' block is only supported on direct sends (a recipient_id must be set). Broadcasts are in-app only.", "email", nil))
		}

		if strings.TrimSpace(p.Email.Subject) == "" {
			errs.Add(apires.NewApiError("Email subject is required", "email.subject cannot be empty when an email block is provided", "email.subject", p.Email.Subject))
		}

		if strings.TrimSpace(p.Email.HTML) == "" && strings.TrimSpace(p.Email.Text) == "" {
			errs.Add(apires.NewApiError("Email content is required", "At least one of email.html or email.text must be provided", "email", nil))
		}
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func (p *SendNotificationPayload) IsDirect() bool {
	return p.RecipientExtID != nil && *p.RecipientExtID != ""
}

func (p *SendNotificationPayload) IsBroadcast() bool {
	return p.RecipientExtID == nil
}

type SendNotificationResult struct {
	// Notification is the notification that was sent.
	// Nil, if this is a broadcast notification.
	// Nil, if the notification was rejected by preferences.
	Notification *Notification `json:"notification"`
	// Broadcast is the broadcast that was sent.
	// Nil, if this is a direct notification.
	Broadcast *Broadcast `json:"broadcast"`
	// Deliveries carries the per-medium delivery outcomes resolved on a direct
	// send (email in v1). A partial-medium failure does NOT reject the whole send
	// (old doc #19) — the send returns 200 and the outcome is reported here. In-app
	// is intentionally absent (its outcome lives on the notification row).
	Deliveries []*NotificationDelivery `json:"deliveries,omitempty"`
}

// NotificationDelivery is the API representation of a per-(notification, medium)
// delivery record. Returned in the send response so callers see per-medium
// outcomes (pending/muted/no_contact/failed at send time; sent/failed later).
type NotificationDelivery struct {
	Medium        string    `json:"medium"`
	Status        string    `json:"status"`
	Address       *string   `json:"address,omitempty"`
	FailureReason *string   `json:"failure_reason,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func FromNotificationDelivery(d *entity.NotificationDelivery) *NotificationDelivery {
	if d == nil {
		return nil
	}
	return &NotificationDelivery{
		Medium:        string(d.Medium),
		Status:        string(d.Status),
		Address:       d.AddressSnapshot,
		FailureReason: d.FailureReason,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
}

// EmailDeliveryTaskPayload is the Asynq payload for the email:delivery task. It
// carries the delivery row id (the row is created synchronously on the send path
// with status=pending) plus the normalized email content + recipient address.
// The provider secret is NOT included — the worker loads and decrypts the
// project's email settings fresh, so key rotation is respected and no secret
// rides through Redis.
type EmailDeliveryTaskPayload struct {
	DeliveryID int64
	ProjectID  int
	To         string
	Subject    string
	HTML       string
	Text       string
}

type NotificationsOverviewResult struct {
	TotalNotifications int `json:"total_notifications"`
	TotalDirectSent    int `json:"total_direct_sent"`
	TotalBroadcastSent int `json:"total_broadcast_sent"`
}

// EmailDeliveryOverview aggregates the email `notification_delivery` rows for a
// project into per-status counts, powering the console's email analytics (Phase
// 5). `Opened` / `Clicked` are counted from the *_at timestamps (they are soft
// signals that do not change `status`) — note in the UI that email "opened" is
// directional only (Apple Mail Privacy Protection inflates it).
type EmailDeliveryOverview struct {
	Total      int `json:"total"`
	Pending    int `json:"pending"`
	Sent       int `json:"sent"`
	Delivered  int `json:"delivered"`
	Bounced    int `json:"bounced"`
	Complained int `json:"complained"`
	Failed     int `json:"failed"`
	NoContact  int `json:"no_contact"`
	Muted      int `json:"muted"`
	Opened     int `json:"opened"`
	Clicked    int `json:"clicked"`
}

func FromNotifications(notifications []*entity.Notification) []*Notification {
	if notifications == nil {
		return nil
	}

	dtos := make([]*Notification, len(notifications))

	for i, n := range notifications {
		notificationDto := FromNotification(n)
		dtos[i] = notificationDto
	}

	return dtos
}

type ListRecipientNotificationsRequest struct {
	RecipientExtID string
	Before         string
	Limit          int
}

type NotificationIDsPayload struct {
	IDs []int `json:"ids"`
}

type PrepareBroadcastBatchesPayload struct {
	UserID    int
	Broadcast *entity.Broadcast
}

type NotificationDeliveryTaskPayload struct {
	UserID       int
	Notification *entity.Notification
}

type BroadcastDeliveryTaskPayload struct {
	ProjectID       int
	BroadcastID     int
	BatchID         int
	RecipientExtIDs []string
	Payload         json.RawMessage
	Channel         string
	Topic           string
	Event           string
}

type UpdateRecipientNotificationsPayload struct {
	NotificationIDsPayload
	State NotificationStateFilter `json:"state"`
}

type ListNotificationsFilters struct {
	ProjectID int

	query.Pagination
	Kind enum.NotificationKind `schema:"kind"`
}

type ListNotificationsResult struct {
	Notifications []*Notification      `json:"notifications"`
	Pagination    query.PaginationMeta `json:"pagination"`
}
