package bodhveda

import (
	"encoding/json"
	"time"
)

// Target represents a target for notifications.
type Target struct {
	Channel string `json:"channel"`
	Topic   string `json:"topic"`
	Event   string `json:"event"`
}

// NotificationState represents the state of a notification.
type NotificationState struct {
	Opened bool `json:"opened"`
	Read   bool `json:"read"`
}

type NotificationStateOptional struct {
	Opened *bool `json:"opened,omitempty"`
	Read   *bool `json:"read,omitempty"`
}

// Notification represents a notification.
type Notification struct {
	ID             int               `json:"id"`
	RecipientExtID string            `json:"recipient_id"`
	Payload        json.RawMessage   `json:"payload"`
	BroadcastID    *int              `json:"broadcast_id"`
	Target         Target            `json:"target"`
	State          NotificationState `json:"state"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// Broadcast represents a broadcast.
type Broadcast struct {
	ID          int             `json:"id"`
	Payload     json.RawMessage `json:"payload"`
	Target      Target          `json:"target"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// Recipient represents a recipient.
type Recipient struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Medium is a delivery transport. MediumInApp and MediumEmail are the mediums a
// preference can apply to; the contact mediums (email/sms/web_push/mobile_push)
// are the transports a recipient contact can be registered for. Only in-app and
// email are active today; the rest are reserved for future transports.
type Medium string

const (
	MediumInApp      Medium = "in_app"
	MediumEmail      Medium = "email"
	MediumSMS        Medium = "sms"
	MediumWebPush    Medium = "web_push"
	MediumMobilePush Medium = "mobile_push"
)

// RecipientContact represents a per-medium contact address for a recipient.
type RecipientContact struct {
	ID         int64      `json:"id"`
	Medium     Medium     `json:"medium"`
	Address    string     `json:"address"`
	IsPrimary  bool       `json:"is_primary"`
	VerifiedAt *time.Time `json:"verified_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// CreateRecipientContactRequest is the request to add a contact to a recipient.
type CreateRecipientContactRequest struct {
	Medium    Medium `json:"medium"`
	Address   string `json:"address"`
	IsPrimary bool   `json:"is_primary"`
}

// CreateRecipientContactResponse is the response after creating a contact.
type CreateRecipientContactResponse struct {
	RecipientContact
}

// ListRecipientContactsResponse is the response after listing a recipient's contacts.
type ListRecipientContactsResponse struct {
	Contacts []RecipientContact `json:"contacts"`
}

// UpdateRecipientContactRequest updates a contact. Both fields are optional; a
// changed address invalidates the contact's verification.
type UpdateRecipientContactRequest struct {
	Address   *string `json:"address,omitempty"`
	IsPrimary *bool   `json:"is_primary,omitempty"`
}

// UpdateRecipientContactResponse is the response after updating a contact.
type UpdateRecipientContactResponse struct {
	RecipientContact
}

// SetPrimaryContactRequest is the body of the idempotent "ensure this is the
// primary contact for this medium" upsert (PUT). Unlike CreateRecipientContactRequest
// there is no IsPrimary field — setting the primary IS the operation: it creates
// the primary if absent, updates the existing primary's address if it differs
// (which resets verification), or no-ops if it already matches.
type SetPrimaryContactRequest struct {
	Medium  Medium `json:"medium"`
	Address string `json:"address"`
}

// SetPrimaryContactResponse is the response after setting a primary contact.
type SetPrimaryContactResponse struct {
	RecipientContact
}

// ProjectPreference is one entry in the project's preference CATALOG. The
// catalog declares which (target, medium) pairs a project may send, and supplies
// the default a recipient inherits until they override it with a toggle of their
// own. It is distinct from Preference, which is one recipient's RESOLVED state:
// manage the catalog with Client.Preferences, and a recipient's own toggles with
// Client.Recipients.Preferences.
type ProjectPreference struct {
	ID        int64  `json:"id"`
	ProjectID int64  `json:"project_id"`
	Target    Target `json:"target"`
	// Medium is the medium this catalog entry gates (in_app or email).
	Medium Medium `json:"medium"`
	// DefaultEnabled is the project-level default for this (target, medium):
	// whether a recipient who has expressed no preference of their own is
	// delivered to.
	DefaultEnabled bool      `json:"default_enabled"`
	Label          string    `json:"label"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// CreateProjectPreferenceRequest creates ONE catalog entry. Strict: creating an
// entry whose (channel, topic, event, medium) already exists rejects with a 409 —
// use ProjectPreferences.Update to change an existing entry, or UpsertMany to
// declaratively merge a whole catalog. Medium defaults to in_app when empty.
type CreateProjectPreferenceRequest struct {
	Channel        string `json:"channel"`
	Topic          string `json:"topic"`
	Event          string `json:"event"`
	Medium         Medium `json:"medium,omitempty"`
	Label          string `json:"label"`
	DefaultEnabled bool   `json:"default_enabled"`
}

// UpdateProjectPreferenceRequest updates a catalog entry. The natural key
// (channel/topic/event/medium) is immutable, so only the label and default change.
type UpdateProjectPreferenceRequest struct {
	Label          string `json:"label"`
	DefaultEnabled bool   `json:"default_enabled"`
}

// UpsertProjectPreferenceItem is one item of a declarative bulk upsert — the same
// shape as CreateProjectPreferenceRequest.
type UpsertProjectPreferenceItem = CreateProjectPreferenceRequest

// UpsertProjectPreferencesOptions configures ProjectPreferences.UpsertMany.
type UpsertProjectPreferencesOptions struct {
	// Prune, when true, DELETES catalog entries NOT present in the array, making
	// the array the project's entire desired catalog. Default false (merge):
	// absent entries are left untouched. Pruning un-catalogs a (target, medium),
	// which turns a non-in_app medium off for recipients relying on the catalog
	// default — hence it is opt-in.
	Prune bool
}

// TargetWithLabel represents a target with an optional label. Medium is the
// medium this preference applies to (in_app or email).
type TargetWithLabel struct {
	Target
	Medium Medium  `json:"medium,omitempty"`
	Label  *string `json:"label,omitempty"`
}

// PreferenceState represents the state of a preference that was just written.
// It describes the stored row, so it carries no catalog context — reads answer a
// different question and reply with ResolvedPreferenceState.
type PreferenceState struct {
	Enabled bool `json:"enabled"`
	// Inherited is true when the recipient has no rule of their own for this
	// exact (target, medium).
	Inherited bool `json:"inherited"`
}

// ResolvedPreferenceState is what a preference read returns: whether a send
// would ACTUALLY deliver, plus the context to explain it.
type ResolvedPreferenceState struct {
	// Enabled is the resolved decision — what a send to this (target, medium)
	// would do. It is not a stored flag: it is resolved through the recipient's
	// exact rule, their topic="any" rule, the project's exact rule, the project's
	// topic="any" rule, and finally the medium's default (in_app delivers, every
	// other medium does not).
	Enabled bool `json:"enabled"`
	// Inherited is true when the recipient has no rule of their own for this
	// exact (target, medium); the value came from elsewhere in the cascade.
	Inherited bool `json:"inherited"`
	// Cataloged reports whether a project-level rule exists for this exact
	// (target, medium). It is context for rendering, NOT a gate: an explicit
	// recipient rule on an uncataloged pair still delivers. Enabled is the answer.
	Cataloged bool `json:"cataloged"`
}

// Preference represents a resolved preference.
type Preference struct {
	Target TargetWithLabel         `json:"target"`
	State  ResolvedPreferenceState `json:"state"`
}

// CreateRecipientRequest represents the request to create a recipient.
type CreateRecipientRequest struct {
	ID   string  `json:"id"`
	Name *string `json:"name,omitempty"`
}

// CreateRecipientResponse represents the response after creating a recipient.
type CreateRecipientResponse struct {
	Recipient
}

// CreateRecipientsBatchRequest represents the request to create multiple recipients in a batch.
type CreateRecipientsBatchRequest struct {
	Recipients []CreateRecipientRequest `json:"recipients"`
}

type BatchCreateRecipientResultItem struct {
	ID string `json:"id"`
}

type BatchCreatereRecicpientResultItemWithError struct {
	BatchCreateRecipientResultItem
	BatchIndex int `json:"batch_index"`
	Errors     []struct {
		Message      string  `json:"message"`
		Description  string  `json:"description"`
		PropertyPath *string `json:"property_path,omitempty"`
		InvalidValue any     `json:"invalid_value,omitempty"`
	} `json:"errors"`
}

// CreateRecipientsBatchResponse represents the response after creating multiple recipients in a batch.
type CreateRecipientsBatchResponse struct {
	Created []BatchCreateRecipientResultItem             `json:"created"`
	Updated []BatchCreateRecipientResultItem             `json:"updated"`
	Failed  []BatchCreatereRecicpientResultItemWithError `json:"failed"`
}

// GetRecipientResponse represents the response after retrieving a recipient.
type GetRecipientResponse struct {
	Recipient
}

// UpdateRecipientRequest represents the request to update a recipient.
type UpdateRecipientRequest struct {
	Name *string `json:"name"`
}

// UpdateRecipientResponse represents the response after updating a recipient.
type UpdateRecipientResponse struct {
	Recipient
}

// EmailContent is the typed email block on a send. Its presence makes email
// eligible for this send (direct-only); absence means no email. Bodhveda is a
// pass-through — the caller renders its own template and passes the result.
// Subject is required and at least one of HTML/Text must be set; Text is
// recommended for deliverability and is auto-derived from HTML when omitted.
type EmailContent struct {
	Subject string `json:"subject"`
	HTML    string `json:"html,omitempty"`
	Text    string `json:"text,omitempty"`
}

// SendNotificationRequest represents the request to send a notification.
type SendNotificationRequest struct {
	Payload     json.RawMessage `json:"payload"`
	RecipientID *string         `json:"recipient_id"`
	Target      *Target         `json:"target"`
	// Email, when present, attempts an email delivery (direct sends only). It is
	// gated by catalog + per-medium preference + a primary email contact.
	Email *EmailContent `json:"email,omitempty"`
}

// NotificationDelivery is a per-medium delivery outcome returned on a direct
// send (email in v1).
type NotificationDelivery struct {
	Medium        string  `json:"medium"`
	Status        string  `json:"status"`
	Address       *string `json:"address,omitempty"`
	FailureReason *string `json:"failure_reason,omitempty"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

// SendNotificationResponse represents the response after sending a notification.
type SendNotificationResponse struct {
	Notification *Notification `json:"notification"`
	Broadcast    *Broadcast    `json:"broadcast"`
	// Deliveries carries per-medium delivery outcomes for a direct send (email).
	// A partial-medium failure never rejects the send — the outcome is here.
	Deliveries []*NotificationDelivery `json:"deliveries,omitempty"`
}

// ListNotificationsRequest represents the request parameters for listing notifications.
type ListNotificationsRequest struct {
	Limit  *int    `json:"limit,omitempty"`
	Before *string `json:"before,omitempty"`
	After  *string `json:"after,omitempty"`
}

// ListNotificationsResponse represents the response after listing notifications.
type ListNotificationsResponse struct {
	Notifications []Notification `json:"notifications"`
	Cursor        struct {
		Before *string `json:"before,omitempty"`
		After  *string `json:"after,omitempty"`
	} `json:"cursor"`
}

// UnreadCountResponse represents the response containing the count of unread notifications.
type UnreadCountResponse struct {
	UnreadCount int `json:"unread_count"`
}

// UpdateNotificationsStateRequest represents the request to update the state of multiple notifications.
type UpdateNotificationsStateRequest struct {
	IDs   []int                     `json:"ids"`
	State NotificationStateOptional `json:"state"`
}

// UpdateNotificationsStateResponse represents the response after updating the state of multiple notifications.
type UpdateNotificationsStateResponse struct {
	UpdatedCount int `json:"updated_count"`
}

// DeleteNotificationsRequest represents the request to delete multiple notifications.
type DeleteNotificationsRequest struct {
	IDs []int `json:"ids"`
}

// DeleteNotificationsResponse represents the response after deleting multiple notifications.
type DeleteNotificationsResponse struct {
	DeletedCount int `json:"deleted_count"`
}

// ListPreferencesResponse represents the response after listing preferences.
type ListPreferencesResponse struct {
	Preferences []Preference `json:"preferences"`
}

// SetPreferenceRequest represents the request to set a preference. Medium
// defaults to in_app when empty.
type SetPreferenceRequest struct {
	Target Target `json:"target"`
	Medium Medium `json:"medium,omitempty"`
	State  struct {
		Enabled bool `json:"enabled"`
	} `json:"state"`
}

// SetPreferenceResponse represents the response after setting a preference.
type SetPreferenceResponse struct {
	Target TargetWithLabel `json:"target"`
	State  PreferenceState `json:"state"`
}

// CheckPreferenceRequest represents the request to check a preference. Medium
// defaults to in_app when empty.
type CheckPreferenceRequest struct {
	Target Target `json:"target"`
	Medium Medium `json:"medium,omitempty"`
}

// CheckPreferenceResponse represents the response after checking a preference.
// The target need not be cataloged, or stored at all — any (channel, topic,
// event) resolves.
type CheckPreferenceResponse struct {
	Target TargetWithLabel         `json:"target"`
	State  ResolvedPreferenceState `json:"state"`
}
