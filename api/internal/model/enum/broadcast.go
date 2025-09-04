package enum

type BroadcastStatus string

const (
	BroadcastStatusEnqueued      BroadcastStatus = "enqueued"
	BroadcastStatusCompleted     BroadcastStatus = "completed"
	BroadcastStatusQuotaExceeded BroadcastStatus = "quota_exceeded"
	BroadcastStatusFailed        BroadcastStatus = "failed"
)
