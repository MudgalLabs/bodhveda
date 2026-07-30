package entity

import (
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/enum"
)

type BroadcastBatch struct {
	ID          int
	BroadcastID int
	Recipients  int
	// RecipientExtIDs is the batch's own record of who it must deliver to.
	//
	// It is what makes preparation resumable: without it the batch→recipient
	// mapping lived only in the Asynq payload, so a batch whose task was never
	// enqueued could not be recovered. Nil for batches created before the column
	// existed — those cannot be resumed, and the processor says so rather than
	// treating them as empty.
	RecipientExtIDs []string
	Status          enum.BroadcastBatchStatus
	Attempt         int
	Duration        int // Duration in milliseconds
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func NewBroadcastBatch(broadcastID int, recipientExtIDs []string) *BroadcastBatch {
	now := time.Now().UTC()
	return &BroadcastBatch{
		BroadcastID:     broadcastID,
		Status:          enum.BroadcastBatchStatusEnqueued,
		Recipients:      len(recipientExtIDs),
		RecipientExtIDs: recipientExtIDs,
		Attempt:         0,
		Duration:        0,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

type BroadcastBatchUpdatePayload struct {
	Status   enum.BroadcastBatchStatus
	Attempt  int
	Duration int
}

func NewBroadcastBatchUpdatePayload(status enum.BroadcastBatchStatus, attempt int, duration int) *BroadcastBatchUpdatePayload {
	return &BroadcastBatchUpdatePayload{
		Status:   status,
		Attempt:  attempt,
		Duration: duration,
	}
}
