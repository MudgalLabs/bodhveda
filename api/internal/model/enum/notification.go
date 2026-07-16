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
)
