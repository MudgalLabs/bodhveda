package enum

type BroadcastBatchStatus string

const (
	BroadcastBatchStatusPending    BroadcastBatchStatus = "pending"
	BroadcastBatchStatusProcessing BroadcastBatchStatus = "processing"
	BroadcastBatchStatusCompleted  BroadcastBatchStatus = "completed"
	BroadcastBatchStatusFailed     BroadcastBatchStatus = "failed"
)
