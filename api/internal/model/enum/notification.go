package enum

const NotificationMaxPayloadSize = 16 * 1024 // 16 KB

type NotificationKind string

const (
	NotificationKindDirect    NotificationKind = "direct"
	NotificationKindBroadcast NotificationKind = "broadcast"
	// NotificationKindAll matches both kinds. An omitted kind still means
	// `direct` (the project Notifications list depends on that default), so
	// wanting both must be asked for explicitly.
	NotificationKindAll NotificationKind = "all"
)

func ParseNotificationKind(s string) NotificationKind {
	switch s {
	case string(NotificationKindDirect):
		return NotificationKindDirect
	case string(NotificationKindBroadcast):
		return NotificationKindBroadcast
	case string(NotificationKindAll):
		return NotificationKindAll
	default:
		return NotificationKindDirect
	}
}

type NotificationStatus string

const (
	NotificationStatusEnqueued      NotificationStatus = "enqueued"
	NotificationStatusMuted         NotificationStatus = "muted"
	NotificationStatusDelivered     NotificationStatus = "delivered"
	NotificationStatusQuotaExceeded NotificationStatus = "quota_exceeded"
	NotificationStatusFailed        NotificationStatus = "failed"
	// NotificationStatusNotRequested means the SENDER did not ask for in-app
	// delivery — the send carried no `payload` block, only an `email` one. It is
	// set at INSERT and never changes: unlike every other status here it is not
	// an outcome the worker resolves, it is the request restated.
	//
	// It is NOT `muted` (which means the RECIPIENT opted out) — conflating sender
	// intent with recipient opt-out would tell an operator exactly the wrong
	// thing. The row exists only so the email delivery, analytics, and
	// GET /notifications/{id} have something to hang off; it is excluded from
	// every recipient-facing read path.
	NotificationStatusNotRequested NotificationStatus = "not_requested"
)

// Valid reports whether s is a status a notification row can actually hold.
// Used to reject a filter naming a status that cannot exist, rather than
// letting it silently match zero rows.
func (s NotificationStatus) Valid() bool {
	switch s {
	case NotificationStatusEnqueued, NotificationStatusMuted, NotificationStatusDelivered,
		NotificationStatusQuotaExceeded, NotificationStatusFailed, NotificationStatusNotRequested:
		return true
	default:
		return false
	}
}
