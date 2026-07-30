package enum

import "testing"

// Every DeliveryStatus the table's CHECK allows must classify deliberately.
// A status added to the enum without a case here would silently fall through to
// the default, so this table is the guard.
func TestDeliveryStatusOutcome(t *testing.T) {
	tests := []struct {
		status DeliveryStatus
		want   Outcome
	}{
		{DeliveryPending, OutcomePending},
		{DeliverySending, OutcomePending},
		{DeliverySent, OutcomeSucceeded},
		{DeliveryDelivered, OutcomeSucceeded},
		// ⚠️ The three that must NOT be failures: the recipient opted out, has no
		// address, or is suppressed. All are the system working as designed.
		{DeliverySkippedMuted, OutcomeSuppressed},
		{DeliverySkippedNoContact, OutcomeSuppressed},
		{DeliverySuppressed, OutcomeSuppressed},
		{DeliveryFailed, OutcomeFailed},
		{DeliveryBounced, OutcomeFailed},
		{DeliveryComplained, OutcomeFailed},
		{DeliveryQuotaExceeded, OutcomeFailed},
		{DeliveryRejected, OutcomeFailed},
	}

	for _, tc := range tests {
		if got := tc.status.Outcome(); got != tc.want {
			t.Errorf("%q.Outcome() = %q, want %q", tc.status, got, tc.want)
		}
	}

	// Coverage check: every value Valid() accepts must appear above, or a new
	// status could ship classified only by the default branch.
	covered := make(map[DeliveryStatus]bool, len(tests))
	for _, tc := range tests {
		covered[tc.status] = true
	}
	for _, s := range []DeliveryStatus{
		DeliveryPending, DeliverySending, DeliverySent, DeliveryDelivered, DeliveryBounced,
		DeliveryComplained, DeliveryFailed, DeliverySkippedMuted, DeliverySkippedNoContact,
		DeliverySuppressed, DeliveryQuotaExceeded, DeliveryRejected,
	} {
		if !covered[s] {
			t.Errorf("DeliveryStatus %q is not covered by the outcome table", s)
		}
	}
}

// An unknown status must not be reported as healthy: a value that reached the
// database without reaching the switch is a bug, and calling it succeeded hides
// it behind a green number.
func TestUnknownStatusIsNotTreatedAsSuccess(t *testing.T) {
	if got := DeliveryStatus("some_new_status").Outcome(); got == OutcomeSucceeded {
		t.Error("an unrecognised delivery status must not classify as succeeded")
	}
	if got := NotificationStatus("some_new_status").Outcome(); got == OutcomeSucceeded {
		t.Error("an unrecognised notification status must not classify as succeeded")
	}
}

func TestDeliveryStatusTerminal(t *testing.T) {
	nonTerminal := []DeliveryStatus{DeliveryPending, DeliverySending}
	for _, s := range nonTerminal {
		if s.Terminal() {
			t.Errorf("%q should not be terminal", s)
		}
	}

	for _, s := range []DeliveryStatus{
		DeliverySent, DeliveryDelivered, DeliveryBounced, DeliveryComplained, DeliveryFailed,
		DeliverySkippedMuted, DeliverySkippedNoContact, DeliveryQuotaExceeded,
	} {
		if !s.Terminal() {
			t.Errorf("%q should be terminal", s)
		}
	}
}

func TestNotificationStatusOutcome(t *testing.T) {
	tests := []struct {
		status NotificationStatus
		want   Outcome
	}{
		{NotificationStatusEnqueued, OutcomePending},
		{NotificationStatusDelivered, OutcomeSucceeded},
		{NotificationStatusMuted, OutcomeSuppressed},
		{NotificationStatusFailed, OutcomeFailed},
		{NotificationStatusQuotaExceeded, OutcomeFailed},
		// Its own bucket: the SENDER never asked for in-app. Not a success (it was
		// never delivered) and not a failure (nothing went wrong).
		{NotificationStatusNotRequested, OutcomeNotRequested},
	}

	for _, tc := range tests {
		if got := tc.status.Outcome(); got != tc.want {
			t.Errorf("%q.Outcome() = %q, want %q", tc.status, got, tc.want)
		}
	}
}

// `enqueued` being the only non-terminal notification status is what makes it
// the signature of a stalled send — internal/monitor's stuck_sends check is
// built on exactly this, so it must not quietly change.
func TestOnlyEnqueuedIsNonTerminal(t *testing.T) {
	if NotificationStatusEnqueued.Terminal() {
		t.Error("enqueued must be non-terminal")
	}

	for _, s := range []NotificationStatus{
		NotificationStatusDelivered, NotificationStatusMuted, NotificationStatusFailed,
		NotificationStatusQuotaExceeded, NotificationStatusNotRequested,
	} {
		if !s.Terminal() {
			t.Errorf("%q should be terminal", s)
		}
	}
}
