package dto

import (
	"testing"

	"github.com/mudgallabs/bodhveda/internal/model/enum"
)

func TestInAppMediumFromRollup(t *testing.T) {
	rollup := map[enum.NotificationStatus]int{
		enum.NotificationStatusDelivered:     380,
		enum.NotificationStatusMuted:         18,
		enum.NotificationStatusFailed:        2,
		enum.NotificationStatusQuotaExceeded: 5,
		enum.NotificationStatusEnqueued:      3,
	}

	m := InAppMediumFromRollup(rollup)

	if m.Medium != "in_app" {
		t.Errorf("medium = %q, want in_app", m.Medium)
	}
	if m.Total != 408 {
		t.Errorf("total = %d, want 408", m.Total)
	}

	// ⚠️ The distinction the whole Outcome type exists for: `muted` is the
	// recipient opting out, which is the system working. Folding it into failed
	// would report 25 failures here instead of 7 and make a healthy broadcast
	// look broken.
	if got := m.Outcomes[string(enum.OutcomeSuppressed)]; got != 18 {
		t.Errorf("suppressed = %d, want 18 (muted must not be a failure)", got)
	}
	if got := m.Outcomes[string(enum.OutcomeFailed)]; got != 7 {
		t.Errorf("failed = %d, want 7 (failed + quota_exceeded only)", got)
	}
	if got := m.Outcomes[string(enum.OutcomeSucceeded)]; got != 380 {
		t.Errorf("succeeded = %d, want 380", got)
	}
	if got := m.Outcomes[string(enum.OutcomePending)]; got != 3 {
		t.Errorf("pending = %d, want 3", got)
	}

	// Pending is surfaced on its own because a non-zero value long after the send
	// is the signature of a stalled worker.
	if m.Pending != 3 {
		t.Errorf("Pending = %d, want 3", m.Pending)
	}

	// Raw statuses are kept alongside: the coarse bucket colours the branch, the
	// raw status explains it.
	if m.Statuses["muted"] != 18 {
		t.Errorf("raw statuses should be preserved, got %v", m.Statuses)
	}
}

// `not_requested` gets its own bucket, and must still be counted in the total.
// Dropping it would leave the branch total short of the notification count with
// nothing explaining the gap — the trap Phase 10 hit in analytics.
func TestInAppRollupCountsNotRequested(t *testing.T) {
	m := InAppMediumFromRollup(map[enum.NotificationStatus]int{
		enum.NotificationStatusDelivered:    10,
		enum.NotificationStatusNotRequested: 4,
	})

	if m.Total != 14 {
		t.Errorf("total = %d, want 14 — not_requested must be counted", m.Total)
	}
	if got := m.Outcomes[string(enum.OutcomeNotRequested)]; got != 4 {
		t.Errorf("not_requested = %d, want 4", got)
	}
	if got := m.Outcomes[string(enum.OutcomeSucceeded)]; got != 10 {
		t.Errorf("not_requested must not inflate succeeded, got %d", got)
	}
	if got := m.Outcomes[string(enum.OutcomeFailed)]; got != 0 {
		t.Errorf("not_requested must not invent a failure, got %d", got)
	}
	if m.Pending != 0 {
		t.Errorf("not_requested is terminal, got Pending = %d", m.Pending)
	}
}

func TestInAppRollupOnEmptyBroadcast(t *testing.T) {
	m := InAppMediumFromRollup(map[enum.NotificationStatus]int{})

	if m.Total != 0 || m.Pending != 0 {
		t.Errorf("empty rollup should be all zero, got %+v", m)
	}
	// Non-nil maps so the JSON is `{}` rather than `null` — the console iterates
	// these without a guard.
	if m.Statuses == nil || m.Outcomes == nil {
		t.Error("maps must be non-nil so they marshal as {} not null")
	}
}

func TestEmailMediumFromDelivery(t *testing.T) {
	t.Run("absent when email was never attempted", func(t *testing.T) {
		// "no email branch" and "an email branch with zero in it" are different
		// facts; the caller omits the branch entirely.
		if _, ok := EmailMediumFromDelivery(nil); ok {
			t.Error("a send with no email delivery row must not produce a branch")
		}
	})

	t.Run("terminal failure", func(t *testing.T) {
		status := enum.DeliveryBounced
		m, ok := EmailMediumFromDelivery(&status)
		if !ok {
			t.Fatal("expected a branch")
		}
		if m.Total != 1 || m.Pending != 0 {
			t.Errorf("bounced is terminal, got total=%d pending=%d", m.Total, m.Pending)
		}
		if got := m.Outcomes[string(enum.OutcomeFailed)]; got != 1 {
			t.Errorf("bounced should be a failure, got %d", got)
		}
	})

	t.Run("in flight", func(t *testing.T) {
		status := enum.DeliveryPending
		m, _ := EmailMediumFromDelivery(&status)
		if m.Pending != 1 {
			t.Errorf("pending delivery should count as in flight, got %d", m.Pending)
		}
	})

	t.Run("muted is suppressed, not failed", func(t *testing.T) {
		status := enum.DeliverySkippedMuted
		m, _ := EmailMediumFromDelivery(&status)
		if got := m.Outcomes[string(enum.OutcomeSuppressed)]; got != 1 {
			t.Errorf("muted email should be suppressed, got %d", got)
		}
		if got := m.Outcomes[string(enum.OutcomeFailed)]; got != 0 {
			t.Errorf("muted email must not count as failed, got %d", got)
		}
	})
}
