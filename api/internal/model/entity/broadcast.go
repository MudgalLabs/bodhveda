package entity

import (
	"encoding/json"
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/enum"
)

type Broadcast struct {
	ID          int
	ProjectID   int
	Payload     json.RawMessage
	Channel     string
	Topic       string
	Event       string
	Status      enum.BroadcastStatus
	CompletedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	// Audience is the recipient breakdown FROZEN when prepare_batches resolved
	// this broadcast's audience. Nil for broadcasts sent before the counts
	// existed, and for ones whose fan-out has not run yet — the console must
	// render that as "not recorded", never as zero.
	Audience *BroadcastAudience

	// Email is the email content this broadcast should also go out as. Nil when
	// the send requested in-app only, which is still the common case.
	//
	// ⚠️ Stored on the broadcast row, not only in the Asynq payload, for the same
	// reason broadcast_batch stores its recipient ids: a retry must be able to
	// reconstruct what is being sent from the database alone.
	Email *BroadcastEmail
}

// BroadcastEmail is the email half of a broadcast: the content, the frozen count
// of who was eligible for it, and why it did not go out if it did not.
type BroadcastEmail struct {
	Subject string
	HTML    string
	Text    string

	// EligibleRecipients is frozen at prepare time, like the in-app audience.
	// Nil means fan-out has not resolved it yet; 0 means it resolved and nobody
	// was eligible — different facts, so it must not be flattened to an int.
	EligibleRecipients *int

	// BlockedReason is set when email was requested but deliberately did not go
	// out — currently only `recipient_cap_exceeded`. Empty when email ran.
	BlockedReason string
}

// EmailBlockedRecipientCapExceeded is recorded when a broadcast's email-eligible
// audience exceeds the project's max_broadcast_recipients_for_email.
//
// ⚠️ The cap BLOCKS rather than truncates. Sending to the first N of an
// over-cap audience would mail an arbitrary subset chosen by query order, which
// looks like success and is harder to notice than nothing being sent.
const EmailBlockedRecipientCapExceeded = "recipient_cap_exceeded"

// BroadcastAudience is the frozen recipient breakdown for one broadcast.
//
// ⚠️ It is stored rather than computed on read because these numbers are only
// true at fan-out time. Deriving `total - eligible` later against a live
// recipient count is wrong the moment anyone signs up or leaves, and goes
// NEGATIVE once recipients are deleted.
//
// ⚠️ The two exclusion buckets are different problems wearing the same word.
// ExcludedDisabled is the recipient opting out — healthy, nothing to fix.
// ExcludedNotCataloged is the PROJECT never offering the target, which is the
// usual reason a broadcast silently reaches nobody. They deliberately reuse the
// vocabulary the direct-send path already writes to
// notification_delivery.failure_reason ('preference_disabled' / 'not_cataloged').
type BroadcastAudience struct {
	Total                int
	Eligible             int
	ExcludedDisabled     int
	ExcludedNotCataloged int
}

func NewBroadcast(projectID int, payload json.RawMessage, channel string, topic string, event string) *Broadcast {
	now := time.Now().UTC()
	return &Broadcast{
		ProjectID:   projectID,
		Payload:     payload,
		Channel:     channel,
		Topic:       topic,
		Event:       event,
		Status:      enum.BroadcastStatusEnqueued,
		CompletedAt: nil,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// WithEmail attaches email content to a broadcast being created.
func (b *Broadcast) WithEmail(subject, html, text string) *Broadcast {
	b.Email = &BroadcastEmail{Subject: subject, HTML: html, Text: text}
	return b
}
