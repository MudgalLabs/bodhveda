package dto

import (
	"encoding/json"
	"html"
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
	// Email is the email-medium delivery outcome for this notification, present
	// only when the send included an email block. The console renders it beside
	// the in-app Status so a diverging outcome (e.g. in-app muted, email
	// delivered) is visible per row.
	Email *NotificationEmailDelivery `json:"email,omitempty"`
}

// NotificationEmailDelivery is the email-medium delivery summary on a listed
// notification. It carries every bounded column of the delivery row: the list
// renders `status` + `failure_reason` inline, and the delivery detail dialog
// renders the rest without a second fetch.
//
// The raw webhook event history (provider_response) is NOT here — it is
// unbounded and is fetched per-notification from the deliveries endpoint. See
// agent-docs/overview.md, "Phase 9.1 — deviations (as built)".
type NotificationEmailDelivery struct {
	Status enum.DeliveryStatus `json:"status"`
	// FailureReason explains a non-delivering outcome. It is the ONLY thing that
	// separates the two causes of `muted`: `not_cataloged` (the project has no
	// (target, email) catalog row) vs `preference_disabled` (the recipient opted
	// out). See fanOutEmail in service/notification.go.
	FailureReason     *string    `json:"failure_reason,omitempty"`
	Attempt           int        `json:"attempt"`
	Provider          *string    `json:"provider,omitempty"`
	ProviderMessageID *string    `json:"provider_message_id,omitempty"`
	AddressSnapshot   *string    `json:"address_snapshot,omitempty"`
	SentAt            *time.Time `json:"sent_at,omitempty"`
	DeliveredAt       *time.Time `json:"delivered_at,omitempty"`
	BouncedAt         *time.Time `json:"bounced_at,omitempty"`
	ComplainedAt      *time.Time `json:"complained_at,omitempty"`
	// OpenedAt / ClickedAt are soft, directional signals only (Apple Mail Privacy
	// Protection inflates opens; blocked images deflate them). In-app `read` is
	// the trustworthy signal — the console must label these as directional.
	OpenedAt  *time.Time `json:"opened_at,omitempty"`
	ClickedAt *time.Time `json:"clicked_at,omitempty"`
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

	if e := notification.Email; e != nil {
		dto.Email = &NotificationEmailDelivery{
			Status:            e.Status,
			FailureReason:     e.FailureReason,
			Attempt:           e.Attempt,
			Provider:          e.Provider,
			ProviderMessageID: e.ProviderMessageID,
			AddressSnapshot:   e.AddressSnapshot,
			SentAt:            e.SentAt,
			DeliveredAt:       e.DeliveredAt,
			BouncedAt:         e.BouncedAt,
			ComplainedAt:      e.ComplainedAt,
			OpenedAt:          e.OpenedAt,
			ClickedAt:         e.ClickedAt,
		}
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

// nonRenderedTags hold content that is not visible body text — their inner text
// (CSS rules, scripts, head metadata) must be dropped, not just their tags, or it
// would leak into the text/plain alternative.
var nonRenderedTags = map[string]bool{"style": true, "script": true, "head": true}

// htmlToText produces a rough text/plain alternative from HTML. Not a full renderer
// — it strips tags, skips the inner content of style/script/head, decodes HTML
// entities, and collapses whitespace. Real callers (e.g. @react-email's render())
// supply their own text; this is only a fallback when `text` is omitted.
func htmlToText(input string) string {
	var b strings.Builder
	i, n := 0, len(input)
	for i < n {
		if input[i] != '<' {
			b.WriteByte(input[i])
			i++
			continue
		}
		// At a '<': find the tag's closing '>'.
		close := strings.IndexByte(input[i:], '>')
		if close == -1 {
			break // unterminated tag — drop the rest
		}
		name := tagName(input[i+1 : i+close])
		i += close + 1
		if nonRenderedTags[name] {
			// Skip everything up to and including the matching close tag.
			if end := indexCloseTag(input[i:], name); end == -1 {
				i = n
			} else {
				i += end
			}
			continue
		}
		b.WriteByte(' ') // tag boundary becomes a space
	}
	text := html.UnescapeString(b.String())
	return strings.TrimSpace(strings.Join(strings.Fields(text), " "))
}

// tagName extracts the lowercased element name from a tag's inner text (between
// '<' and '>'), ignoring a leading '/', attributes, and a trailing '/'.
func tagName(inner string) string {
	inner = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(inner), "/"))
	for i := 0; i < len(inner); i++ {
		switch inner[i] {
		case ' ', '\t', '\n', '\r', '/', '>':
			return strings.ToLower(inner[:i])
		}
	}
	return strings.ToLower(inner)
}

// indexCloseTag returns the byte offset just past the matching `</name ... >` in s
// (case-insensitive), or -1 if there is none.
func indexCloseTag(s, name string) int {
	idx := strings.Index(strings.ToLower(s), "</"+name)
	if idx == -1 {
		return -1
	}
	gt := strings.IndexByte(s[idx:], '>')
	if gt == -1 {
		return -1
	}
	return idx + gt + 1
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
		if p.Target == nil {
			// A broadcast with no target at all — guard the nil deref and report the
			// missing target as a validation error rather than panicking.
			errs.Add(apires.NewApiError("Target is required", "A target (channel, topic, and event) is required for a broadcast notification.", "target", nil))
		} else {
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
	// UnsubscribeURL is the public one-click unsubscribe link (Phase 6) injected as
	// the outbound email's List-Unsubscribe header. Built on the send path (which
	// has project/recipient/target) and carried through so the worker can set the
	// header without re-deriving the token.
	UnsubscribeURL string
}

type NotificationsOverviewResult struct {
	TotalNotifications int `json:"total_notifications"`
	TotalDirectSent    int `json:"total_direct_sent"`
	TotalBroadcastSent int `json:"total_broadcast_sent"`
}

// AnalyticsFilters bounds a project analytics request to a date range. Like
// ListNotificationsFilters, CreatedFrom/CreatedTo are absolute RFC3339 instants,
// not calendar days: the console turns the picked day range into the viewer's
// local start-of-day / end-of-day before sending, so "last 30 days" means the
// operator's days, not UTC's. The per-day BUCKETING is done in the viewer's
// timezone too (carried via the X-Timezone header — see the repo methods).
type AnalyticsFilters struct {
	ProjectID   int
	CreatedFrom *time.Time `schema:"created_from"`
	CreatedTo   *time.Time `schema:"created_to"`
}

// Validate rejects an inverted range with a 400 rather than letting it answer
// with an empty chart. It mirrors ListNotificationsFilters.Validate — a blank
// `?created_from=` is still a hard 400 from gorilla/schema (it decodes into a
// *time.Time), so the console omits empties rather than blanking them.
func (f *AnalyticsFilters) Validate() error {
	if f.CreatedFrom != nil && f.CreatedTo != nil && f.CreatedTo.Before(*f.CreatedFrom) {
		var errs service.InputValidationErrors
		errs.Add(apires.NewApiError("Invalid date range", "`created_to` is before `created_from`", "created_to", *f.CreatedTo))
		return errs
	}
	return nil
}

// ProjectAnalytics is the console Home page's analytics payload (Phase 9.5): a
// time-series + breakdowns for one project over a date range.
//
// In-app and email outcomes live in DIFFERENT tables and are aggregated
// SEPARATELY — never one GROUP BY over a join. In-app status is a scalar on the
// `notification` row; email lives in a `notification_delivery` row that only
// exists when the send carried an `email` block. A join would drop every
// in-app-only notification (still the common case), so InApp is aggregated over
// `notification` and Email over `notification_delivery WHERE medium='email'`.
//
// Email.Attempted == 0 is the self-hiding signal: a project that has never
// attempted an email renders no email charts (the console hides the Email panel
// and Delivery health entirely).
type ProjectAnalytics struct {
	InApp   AnalyticsInApp        `json:"in_app"`
	Email   AnalyticsEmail        `json:"email"`
	Targets []AnalyticsTargetStat `json:"targets"`
}

// AnalyticsInApp is the in-app (notification-row) side: the trustworthy inbox
// outcome. Totals are summed from the per-day Series server-side (the series is
// bounded by the number of days in range, so a second scan for totals is waste).
type AnalyticsInApp struct {
	Total    int                    `json:"total"`
	ByStatus AnalyticsInAppByStatus `json:"by_status"`
	Series   []AnalyticsInAppDay    `json:"series"`
}

// AnalyticsInAppByStatus counts the in-app notification statuses. Every value
// here is one a `notification` row can actually hold (enum.NotificationStatus) —
// there are no reserved-but-never-written members to mislead a chart.
type AnalyticsInAppByStatus struct {
	Enqueued      int `json:"enqueued"`
	Muted         int `json:"muted"`
	Delivered     int `json:"delivered"`
	QuotaExceeded int `json:"quota_exceeded"`
	Failed        int `json:"failed"`
}

// AnalyticsInAppDay is one calendar day's in-app counts, the day computed in the
// viewer's timezone (Day is `YYYY-MM-DD`). Days with no notifications are absent
// — the console gap-fills zeros across the range so the axis is continuous.
type AnalyticsInAppDay struct {
	Day           string `json:"day"`
	Total         int    `json:"total"`
	Enqueued      int    `json:"enqueued"`
	Muted         int    `json:"muted"`
	Delivered     int    `json:"delivered"`
	QuotaExceeded int    `json:"quota_exceeded"`
	Failed        int    `json:"failed"`
}

// AnalyticsEmail is the email (notification_delivery) side. Attempted is the
// count of email delivery rows (medium='email') in range — 0 means the project
// has never sent email in this window, and the console hides every email chart.
//
// Opened / Clicked are SOFT, directional signals counted from the *_at columns
// (Apple Mail Privacy Protection inflates opens; blocked images deflate them).
// They are NOT statuses and must never be charted as the trustworthy in-app
// `read` — see OPEN_SOFT_SIGNAL_COPY in the console.
type AnalyticsEmail struct {
	Attempted int                    `json:"attempted"`
	ByStatus  AnalyticsEmailByStatus `json:"by_status"`
	Opened    int                    `json:"opened"`
	Clicked   int                    `json:"clicked"`
	Series    []AnalyticsEmailDay    `json:"series"`
}

// AnalyticsEmailByStatus counts the email delivery statuses. Only the eight
// statuses v1 actually writes appear — the four reserved DeliveryStatus values
// (sending/suppressed/quota_exceeded/rejected) are omitted so a chart never
// shows an axis for data that cannot exist.
type AnalyticsEmailByStatus struct {
	Pending    int `json:"pending"`
	Sent       int `json:"sent"`
	Delivered  int `json:"delivered"`
	Bounced    int `json:"bounced"`
	Complained int `json:"complained"`
	Failed     int `json:"failed"`
	NoContact  int `json:"no_contact"`
	Muted      int `json:"muted"`
}

// AnalyticsEmailDay is one calendar day's email counts (Day in the viewer's
// timezone). Only the delivery-health statuses that drive the over-time chart
// are broken out per day; the full per-status split lives in ByStatus.
type AnalyticsEmailDay struct {
	Day        string `json:"day"`
	Attempted  int    `json:"attempted"`
	Delivered  int    `json:"delivered"`
	Bounced    int    `json:"bounced"`
	Complained int    `json:"complained"`
}

// AnalyticsTargetStat is one {channel, topic, event} target's activity over the
// range — "which targets actually fire". Notifications is the in-app count (over
// `notification`); the Email* counts come from the email delivery rows joined
// back to their notification's target (a join that is CORRECT here, because the
// question is specifically about email deliveries and their targets — unlike the
// in-app aggregate, where a join would drop rows).
type AnalyticsTargetStat struct {
	Channel         string `json:"channel"`
	Topic           string `json:"topic"`
	Event           string `json:"event"`
	Notifications   int    `json:"notifications"`
	EmailAttempted  int    `json:"email_attempted"`
	EmailDelivered  int    `json:"email_delivered"`
	EmailBounced    int    `json:"email_bounced"`
	EmailComplained int    `json:"email_complained"`
}

// NotificationDeliveryDetail is the FULL delivery record for one
// (notification, medium), including the provider webhook event history. It is
// served per-notification (Phase 9.1) rather than on every list row, because
// Events is unbounded — one raw provider event body per webhook.
type NotificationDeliveryDetail struct {
	ID                int64               `json:"id"`
	Medium            enum.Medium         `json:"medium"`
	Status            enum.DeliveryStatus `json:"status"`
	FailureReason     *string             `json:"failure_reason,omitempty"`
	Attempt           int                 `json:"attempt"`
	Provider          *string             `json:"provider,omitempty"`
	ProviderMessageID *string             `json:"provider_message_id,omitempty"`
	// AddressSnapshot is the address captured at enqueue time — immune to later
	// edits of the recipient's contact, so it reflects where this email actually went.
	AddressSnapshot *string    `json:"address_snapshot,omitempty"`
	SentAt          *time.Time `json:"sent_at,omitempty"`
	DeliveredAt     *time.Time `json:"delivered_at,omitempty"`
	BouncedAt       *time.Time `json:"bounced_at,omitempty"`
	ComplainedAt    *time.Time `json:"complained_at,omitempty"`
	// OpenedAt / ClickedAt are soft, directional signals only — see the note on
	// NotificationEmailDelivery.
	OpenedAt  *time.Time      `json:"opened_at,omitempty"`
	ClickedAt *time.Time      `json:"clicked_at,omitempty"`
	Events    []DeliveryEvent `json:"events"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// DeliveryEvent is one entry of the delivery row's provider_response JSONB array
// — a raw provider webhook body (appended once per webhook, Phase 5) reduced to
// what a timeline needs.
//
// Kind/At are normalized by the provider's own adapter (the same normalizer the
// inbound webhook path uses), so the console renders a timeline without knowing
// any provider's JSON shape — which is what keeps a future provider/managed-SES
// adapter a backend-only change. Kind is empty for an event the adapter does not
// recognize; Raw is always the verbatim event.
type DeliveryEvent struct {
	Kind string          `json:"kind"`
	At   *time.Time      `json:"at,omitempty"`
	Raw  json.RawMessage `json:"raw"`
}

// FromNotificationDeliveryDetail builds the detail DTO. events are normalized by
// the caller (the service, which owns adapter selection) and may be nil when no
// webhook has landed for this row.
func FromNotificationDeliveryDetail(d *entity.NotificationDelivery, events []DeliveryEvent) *NotificationDeliveryDetail {
	if events == nil {
		events = []DeliveryEvent{}
	}

	return &NotificationDeliveryDetail{
		ID:                d.ID,
		Medium:            d.Medium,
		Status:            d.Status,
		FailureReason:     d.FailureReason,
		Attempt:           d.Attempt,
		Provider:          d.Provider,
		ProviderMessageID: d.ProviderMessageID,
		AddressSnapshot:   d.AddressSnapshot,
		SentAt:            d.SentAt,
		DeliveredAt:       d.DeliveredAt,
		BouncedAt:         d.BouncedAt,
		ComplainedAt:      d.ComplainedAt,
		OpenedAt:          d.OpenedAt,
		ClickedAt:         d.ClickedAt,
		Events:            events,
		CreatedAt:         d.CreatedAt,
		UpdatedAt:         d.UpdatedAt,
	}
}

// ListNotificationDeliveriesResult is the deliveries-for-one-notification
// response. Deliveries is empty (not an error) when the send carried no email —
// in-app has no delivery row in v1.
type ListNotificationDeliveriesResult struct {
	Deliveries []*NotificationDeliveryDetail `json:"deliveries"`
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
	// Email carries the send's email block, if any. The whole email fan-out
	// (gate + contact lookup + delivery-row insert + provider enqueue) runs in
	// the worker now, not on the request path — so the send API returns after a
	// single notification INSERT. Nil when the send carried no email block.
	Email *EmailContent
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
	// RecipientExtID narrows the list to one recipient. Unlike the recipient's
	// own inbox feed (ListForRecipient), this keeps `muted`/`quota_exceeded`
	// rows and their email delivery outcome — an operator debugging "why didn't
	// they get it" is looking for exactly those.
	//
	// This is an EXACT match, and it is what the recipient detail page's feed
	// pins itself to. It is deliberately distinct from RecipientSearch below:
	// one addresses a known recipient, the other looks for one.
	RecipientExtID *string `schema:"recipient_id"`

	// Status filters the IN-APP outcome — the scalar on the `notification` row.
	// There is no `medium` filter beside it: in_app is the medium whose status
	// this is (Phase 4 kept the inbox outcome off notification_delivery), and
	// email's is Email below. See enum.EmailDeliveryFilter.
	Status *enum.NotificationStatus `schema:"status"`

	// Channel / Topic / Event filter the target, each independently and by exact
	// match. A target is an identity the list already renders verbatim, so exact
	// is both what an operator can copy off the screen and what an index could
	// serve; a substring match on a target component would mean neither.
	Channel *string `schema:"channel"`
	Topic   *string `schema:"topic"`
	Event   *string `schema:"event"`

	// Email filters on the email medium's delivery row. It compiles to an
	// EXISTS/NOT EXISTS subquery rather than a join — see ListNotifications in
	// pg/notification.go for why that matters (a join would drop in-app-only
	// rows, which are still the common case).
	Email *enum.EmailDeliveryFilter `schema:"email"`

	// CreatedFrom / CreatedTo bound `notification.created_at` (inclusive). They
	// are absolute instants (RFC3339), not dates: the console turns the picked
	// day range into the viewer's local start-of-day / end-of-day before
	// sending, so "last week" means the operator's week, not UTC's.
	CreatedFrom *time.Time `schema:"created_from"`
	CreatedTo   *time.Time `schema:"created_to"`

	// RecipientSearch is a case-insensitive SUBSTRING match on the recipient's
	// external id — the "find me the recipient" half, as opposed to
	// RecipientExtID's "I know which one".
	RecipientSearch *string `schema:"recipient_search"`
}

// Validate rejects a filter value that could never match, so a typo answers with
// a 400 instead of an empty list that reads as "you have no such notifications".
//
// It also normalizes: blank query params (`?status=`) are treated as absent,
// since a UI clearing a control has no reason to distinguish the two, and
// external ids are lowercased to match how they are stored.
func (f *ListNotificationsFilters) Validate() error {
	var errs service.InputValidationErrors

	f.RecipientExtID = normalizeOptionalStr(f.RecipientExtID, true)
	f.RecipientSearch = normalizeOptionalStr(f.RecipientSearch, true)
	f.Channel = normalizeOptionalStr(f.Channel, false)
	f.Topic = normalizeOptionalStr(f.Topic, false)
	f.Event = normalizeOptionalStr(f.Event, false)

	if f.Status != nil {
		if *f.Status == "" {
			f.Status = nil
		} else if !f.Status.Valid() {
			errs.Add(apires.NewApiError("Invalid status", "Not a notification status", "status", *f.Status))
		}
	}

	if f.Email != nil {
		if *f.Email == "" {
			f.Email = nil
		} else if !f.Email.Valid() {
			errs.Add(apires.NewApiError("Invalid email filter", "Expected `none`, `any`, or a delivery status", "email", *f.Email))
		}
	}

	if f.Kind != "" && f.Kind != enum.NotificationKindDirect &&
		f.Kind != enum.NotificationKindBroadcast && f.Kind != enum.NotificationKindAll {
		errs.Add(apires.NewApiError("Invalid kind", "Expected `direct`, `broadcast`, or `all`", "kind", f.Kind))
	}

	if f.CreatedFrom != nil && f.CreatedTo != nil && f.CreatedTo.Before(*f.CreatedFrom) {
		errs.Add(apires.NewApiError("Invalid date range", "`created_to` is before `created_from`", "created_to", *f.CreatedTo))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

// normalizeOptionalStr collapses a blank filter to "absent" and optionally
// lowercases it (external ids are stored lowercase, so a filter keyed on one
// must be too — see ListNotifications in service/notification.go).
func normalizeOptionalStr(s *string, lower bool) *string {
	if s == nil {
		return nil
	}

	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}

	if lower {
		v = strings.ToLower(v)
	}

	return &v
}

type ListNotificationsResult struct {
	Notifications []*Notification      `json:"notifications"`
	Pagination    query.PaginationMeta `json:"pagination"`
}
