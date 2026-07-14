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

// TargetWithLabel represents a target with an optional label. Medium is the
// medium this preference applies to (in_app or email).
type TargetWithLabel struct {
	Target
	Medium Medium  `json:"medium,omitempty"`
	Label  *string `json:"label,omitempty"`
}

// PreferenceState represents the state of a preference.
type PreferenceState struct {
	Enabled bool `json:"enabled"`
	Inherit bool `json:"inherit"`
}

// Preference represents a preference.
type Preference struct {
	Target TargetWithLabel `json:"target"`
	State  PreferenceState `json:"state"`
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
type CheckPreferenceResponse struct {
	Target TargetWithLabel `json:"target"`
	State  PreferenceState `json:"state"`
}
