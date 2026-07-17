package enum

// DeliveryStatus is the status of a single per-(notification, medium) delivery
// record in `notification_delivery`. It is distinct from NotificationStatus,
// which is the in-app inbox outcome scalar on the `notification` row.
//
// In v1 (email, DIRECT-only) the statuses set are:
//
//   - DeliverySkippedMuted / DeliverySkippedNoContact are terminal outcomes
//     resolved synchronously on the send path (email disabled/uncataloged, or no
//     primary contact) — the email is never enqueued.
//   - DeliveryPending is set when the email:delivery task is enqueued.
//   - DeliverySent / DeliveryFailed are the outcomes the worker writes after
//     calling the provider adapter (DeliverySent = provider accepted).
//   - DeliveryDelivered / DeliveryBounced / DeliveryComplained are set by inbound
//     provider webhooks (Phase 5, pg.ApplyWebhookStatus). Bounced/complained are
//     sticky terminals. "opened"/"clicked" are NOT statuses — they are soft
//     signals stamped only on the opened_at/clicked_at columns.
//
// The remaining values (sending, suppressed, quota_exceeded, rejected) exist to
// match the table CHECK but are not set yet (suppressed is reserved for the
// Phase 6 unsubscribe/complaint-suppression work).
type DeliveryStatus string

const (
	DeliveryPending          DeliveryStatus = "pending"
	DeliverySending          DeliveryStatus = "sending"
	DeliverySent             DeliveryStatus = "sent"
	DeliveryDelivered        DeliveryStatus = "delivered"
	DeliveryBounced          DeliveryStatus = "bounced"
	DeliveryComplained       DeliveryStatus = "complained"
	DeliveryFailed           DeliveryStatus = "failed"
	DeliverySkippedMuted     DeliveryStatus = "muted"
	DeliverySkippedNoContact DeliveryStatus = "no_contact"
	DeliverySuppressed       DeliveryStatus = "suppressed"
	DeliveryQuotaExceeded    DeliveryStatus = "quota_exceeded"
	DeliveryRejected         DeliveryStatus = "rejected"
)

// Valid reports whether s is a status the `notification_delivery.status` CHECK
// accepts. Used to reject a filter naming a status that cannot exist, rather
// than letting it silently match zero rows.
func (s DeliveryStatus) Valid() bool {
	switch s {
	case DeliveryPending, DeliverySending, DeliverySent, DeliveryDelivered,
		DeliveryBounced, DeliveryComplained, DeliveryFailed, DeliverySkippedMuted,
		DeliverySkippedNoContact, DeliverySuppressed, DeliveryQuotaExceeded,
		DeliveryRejected:
		return true
	default:
		return false
	}
}

// EmailDeliveryFilter selects notifications by their EMAIL-medium delivery
// outcome. It deliberately folds the "medium" and "delivery status" dimensions
// into one value, because in v1 they are not independent:
//
//   - `email` is the only medium that has a notification_delivery row at all.
//     `in_app` keeps its outcome on the `notification` row (Phase 4 chose not to
//     migrate the inbox onto delivery rows), so a `medium=in_app` filter could
//     only ever mean "every notification" — a control that lies. The in-app
//     outcome is filtered by ListNotificationsFilters.Status instead.
//   - Which leaves exactly one real question about the email medium: did this
//     send attempt email, and how did it end up? That is this one value.
//
// EmailFilterNone / EmailFilterAny are the medium dimension (was email
// attempted?); any other value must be a DeliveryStatus and asks how it ended.
// EmailFilterNone is what keeps in-app-only notifications reachable — they are
// still the common case, and an EXISTS-shaped filter would otherwise make them
// unfindable rather than merely unlisted.
type EmailDeliveryFilter string

const (
	// EmailFilterNone matches notifications with NO email delivery row — i.e.
	// in-app-only sends (no `email` block, so email was never eligible).
	EmailFilterNone EmailDeliveryFilter = "none"
	// EmailFilterAny matches notifications that attempted email, whatever the
	// outcome.
	EmailFilterAny EmailDeliveryFilter = "any"
)

// Valid reports whether f is a usable email filter: one of the two medium
// sentinels, or a real DeliveryStatus.
func (f EmailDeliveryFilter) Valid() bool {
	switch f {
	case EmailFilterNone, EmailFilterAny:
		return true
	default:
		return DeliveryStatus(f).Valid()
	}
}
