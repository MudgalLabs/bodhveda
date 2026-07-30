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
}

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
