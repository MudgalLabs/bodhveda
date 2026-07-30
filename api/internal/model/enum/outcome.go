package enum

// Outcome is the coarse, medium-independent answer to "how did this end up?".
//
// It exists because the raw status enums are too many and too specific to reason
// about safely. NotificationStatus has 6 values and DeliveryStatus has 12, and
// anyone summarising them — the console tree, an alert, a chart — has to decide
// which ones are bad. They will get it wrong in one specific way, and it matters:
//
// ⚠️ `muted` is NOT a failure. It means the recipient opted out, which is the
// system working exactly as designed. Folding it into "failed" makes every
// failure number permanently wrong, makes a healthy project look broken, and
// (when it drives an alert) fires on every opted-out recipient until someone
// mutes the alert. Hence OutcomeSuppressed as its own bucket: the honest answer
// to "did this reach a human?" is "no, on purpose".
//
// The mapping lives here, once, so the tree, the analytics and any future
// surface classify identically instead of each re-deciding.
type Outcome string

const (
	// OutcomePending — not finished; no conclusion is available yet.
	OutcomePending Outcome = "pending"
	// OutcomeSucceeded — Bodhveda did its job.
	OutcomeSucceeded Outcome = "succeeded"
	// OutcomeSuppressed — intentionally not delivered. Working as designed.
	OutcomeSuppressed Outcome = "suppressed"
	// OutcomeFailed — it did not reach the human and should have.
	OutcomeFailed Outcome = "failed"
	// OutcomeNotRequested — the SENDER never asked for this medium. In-app only,
	// from an email-only send. Distinct from every other value because it is not
	// an outcome at all: it is the request restated. Counting it as succeeded
	// would inflate delivery; counting it as failed would invent a problem.
	OutcomeNotRequested Outcome = "not_requested"
)

// Outcome classifies a per-(notification, medium) delivery status.
func (s DeliveryStatus) Outcome() Outcome {
	switch s {
	case DeliveryPending, DeliverySending:
		return OutcomePending
	case DeliverySent, DeliveryDelivered:
		return OutcomeSucceeded
	case DeliverySkippedMuted, DeliverySkippedNoContact, DeliverySuppressed:
		return OutcomeSuppressed
	case DeliveryFailed, DeliveryBounced, DeliveryComplained, DeliveryQuotaExceeded, DeliveryRejected:
		return OutcomeFailed
	default:
		// An unrecognised status is not assumed healthy — a value that reached the
		// database without reaching this switch is a bug, and reporting it as
		// succeeded would hide it.
		return OutcomeFailed
	}
}

// Terminal reports whether a delivery is expected to change again.
//
// ⚠️ `sent` is the subtle one, and callers must not treat it as final
// unconditionally. It means "the provider accepted the message". If the project
// has inbound provider webhooks configured, it will still advance to
// delivered/bounced/complained; if it does not, `sent` is as far as it will ever
// get. So this reports terminality in the ABSENCE of webhooks — the honest
// summary for a caller asking "is Bodhveda still working on this?", which is a
// different question from "is this the final word from the provider?".
func (s DeliveryStatus) Terminal() bool {
	switch s {
	case DeliveryPending, DeliverySending:
		return false
	default:
		return true
	}
}

// Outcome classifies the in-app inbox status carried on the `notification` row.
//
// In-app has no notification_delivery row (Phase 4 deliberately did not migrate
// the inbox onto delivery rows), so this is how the tree renders the in_app
// branch with the same vocabulary as every other medium.
func (s NotificationStatus) Outcome() Outcome {
	switch s {
	case NotificationStatusEnqueued:
		return OutcomePending
	case NotificationStatusDelivered:
		return OutcomeSucceeded
	case NotificationStatusMuted:
		return OutcomeSuppressed
	case NotificationStatusFailed, NotificationStatusQuotaExceeded:
		return OutcomeFailed
	case NotificationStatusNotRequested:
		return OutcomeNotRequested
	default:
		return OutcomeFailed
	}
}

// Terminal reports whether an in-app notification is still being worked on.
//
// ⚠️ `enqueued` is the ONLY non-terminal value, which is what makes it the
// signature of a stalled send: the notification:delivery task normally resolves
// it sub-second, so a row still `enqueued` minutes later means the send path
// stopped after the initial INSERT. internal/monitor's stuck_sends check is
// built on exactly this.
func (s NotificationStatus) Terminal() bool {
	return s != NotificationStatusEnqueued
}
