package dto

import (
	"encoding/json"
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/tantra/query"
)

type Broadcast struct {
	ID          int                  `json:"id"`
	Payload     json.RawMessage      `json:"payload"`
	Target      Target               `json:"target"`
	Status      enum.BroadcastStatus `json:"status"`
	CompletedAt *time.Time           `json:"completed_at,omitempty"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
}

func FromBroadcast(broadcast *entity.Broadcast) *Broadcast {
	if broadcast == nil {
		return nil
	}

	return &Broadcast{
		ID:      broadcast.ID,
		Payload: broadcast.Payload,
		Target: Target{
			Channel: broadcast.Channel,
			Topic:   broadcast.Topic,
			Event:   broadcast.Event,
		},
		Status:      broadcast.Status,
		CompletedAt: broadcast.CompletedAt,
		CreatedAt:   broadcast.CreatedAt,
		UpdatedAt:   broadcast.UpdatedAt,
	}
}

// DeliveryTree is the console's answer to "what happened to this send?", for a
// broadcast and a direct send alike.
//
// One shape for both kinds on purpose: a direct send is the same tree with a
// fan-out of one. Special-casing them would make the two impossible to compare,
// and the interesting operator question ("did this reach anyone?") is the same
// question at both scales.
type DeliveryTree struct {
	Kind   enum.NotificationKind `json:"kind"`
	Target Target                `json:"target"`
	// Audience is present for broadcasts only, and only once fan-out has run.
	Audience *DeliveryTreeAudience `json:"audience,omitempty"`
	// Mediums is the per-medium breakdown. Today in_app for broadcasts, and
	// in_app + email for direct sends — but shaped as a list from day one so
	// broadcast email (and SMS, push) need no contract change.
	Mediums []DeliveryTreeMedium `json:"mediums"`
}

// DeliveryTreeAudience is the frozen recipient breakdown for a broadcast.
type DeliveryTreeAudience struct {
	Total    int `json:"total"`
	Eligible int `json:"eligible"`
	// ExcludedDisabled — the RECIPIENT opted out. Healthy; nothing to fix.
	ExcludedDisabled int `json:"excluded_disabled"`
	// ExcludedNotCataloged — the PROJECT never offered this target (no catalog
	// row, or a disabled one). This is the usual reason a broadcast silently
	// reaches nobody, and it is a config mistake rather than a recipient choice.
	ExcludedNotCataloged int `json:"excluded_not_cataloged"`
	// ⚠️ Expandable is false for broadcasts, and the console MUST respect it.
	// Excluded recipients are filtered out before any row is written, so there is
	// nothing to drill into — only a count. On a DIRECT send the same situation
	// produces a real `muted` row you can open. Rendering both as the same kind of
	// node would promise a drill-down that cannot exist.
	Expandable bool `json:"expandable"`
}

// DeliveryTreeMedium is one medium's branch: the per-status counts plus the
// coarse per-outcome rollup.
type DeliveryTreeMedium struct {
	Medium string `json:"medium"`
	Total  int    `json:"total"`
	// Statuses is the raw per-status count, e.g. {"delivered": 380, "muted": 18}.
	Statuses map[string]int `json:"statuses"`
	// Outcomes is the same data folded into enum.Outcome buckets. Carried
	// alongside rather than instead of Statuses because the console needs both:
	// the coarse bucket to colour the branch, the raw status to explain it.
	//
	// ⚠️ `suppressed` is deliberately NOT `failed` — see enum.Outcome. A muted
	// recipient is the system working, and counting it as a failure makes a
	// healthy project look broken.
	Outcomes map[string]int `json:"outcomes"`
	// Pending is the count still in flight. Non-zero long after the send is the
	// signature of a stalled worker, which is the whole reason this view exists.
	Pending int `json:"pending"`
}

// InAppMediumFromRollup builds the in_app branch from a per-status rollup of
// `notification` rows.
//
// Note it counts EVERY status, including `not_requested`. That row exists
// because the send was email-only, and omitting it would leave the branch total
// silently short of the notification count with nothing explaining the gap —
// the same trap Phase 10 hit in analytics.
func InAppMediumFromRollup(rollup map[enum.NotificationStatus]int) DeliveryTreeMedium {
	m := DeliveryTreeMedium{
		Medium:   string(enum.MediumInApp),
		Statuses: make(map[string]int, len(rollup)),
		Outcomes: make(map[string]int),
	}

	for status, count := range rollup {
		m.Statuses[string(status)] = count
		m.Outcomes[string(status.Outcome())] += count
		m.Total += count

		if !status.Terminal() {
			m.Pending += count
		}
	}

	return m
}

// EmailMediumFromDelivery builds the email branch from the delivery rows of a
// single direct send. Returns false when the send never attempted email, so the
// caller can omit the branch rather than render an empty one — "no email branch"
// and "an email branch with zero in it" are different facts.
func EmailMediumFromDelivery(status *enum.DeliveryStatus) (DeliveryTreeMedium, bool) {
	if status == nil {
		return DeliveryTreeMedium{}, false
	}

	m := DeliveryTreeMedium{
		Medium:   string(enum.MediumEmail),
		Total:    1,
		Statuses: map[string]int{string(*status): 1},
		Outcomes: map[string]int{string(status.Outcome()): 1},
	}
	if !status.Terminal() {
		m.Pending = 1
	}

	return m, true
}

type BroadcastListItem struct {
	Broadcast
	DeliveredCount int `json:"delivered_count"`
	ReadCount      int `json:"read_count"`
	OpenedCount    int `json:"opened_count"`
}

type ListBroadcastsFilters struct {
	ProjectID int

	query.Pagination
}

type ListBroadcastssResult struct {
	Broadcasts []*BroadcastListItem `json:"broadcasts"`
	Pagination query.PaginationMeta `json:"pagination"`
}
