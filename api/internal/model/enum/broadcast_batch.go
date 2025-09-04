package enum

type BroadcastBatchStatus string

const (
	BroadcastBatchStatusEnqueued BroadcastBatchStatus = "enqueued"
	BroadcastBatchStatusSuccess  BroadcastBatchStatus = "success"
	BroadcastBatchStatusFailed   BroadcastBatchStatus = "failed"
)
