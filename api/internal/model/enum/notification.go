package enum

const NotificationMaxPayloadSize = 16 * 1024 // 16 KB

type NotificationKind string

const (
	NotificationKindDirect    NotificationKind = "direct"
	NotificationKindBroadcast NotificationKind = "broadcast"
)

func ParseNotificationKind(s string) NotificationKind {
	switch s {
	case string(NotificationKindDirect):
		return NotificationKindDirect
	case string(NotificationKindBroadcast):
		return NotificationKindBroadcast
	default:
		return NotificationKindDirect
	}
}
