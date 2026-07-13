package enum

// DeliveryStatus is the status of a single per-(notification, medium) delivery
// record in `notification_delivery`. It is distinct from NotificationStatus,
// which is the in-app inbox outcome scalar on the `notification` row.
//
// In v1 (email, DIRECT-only) only a subset is ever set:
//
//   - DeliverySkippedMuted / DeliverySkippedNoContact are terminal outcomes
//     resolved synchronously on the send path (email disabled/uncataloged, or no
//     primary contact) — the email is never enqueued.
//   - DeliveryPending is set when the email:delivery task is enqueued.
//   - DeliverySent / DeliveryFailed are the terminal outcomes the worker writes
//     after calling the provider adapter.
//
// The remaining values (sending, delivered, bounced, complained, suppressed,
// quota_exceeded, rejected) exist to match the table CHECK and are reserved for
// Phase 5 (provider webhooks) — v1 does not set them.
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
