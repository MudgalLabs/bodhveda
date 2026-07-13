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
