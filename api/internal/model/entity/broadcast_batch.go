package entity

import (
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/enum"
)

type BroadcastBatch struct {
	ID          int
	BroadcastID int
	Recipients  int
	Status      enum.BroadcastBatchStatus
	Attempt     int
	Duration    int // Duration in milliseconds
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewBroadcastBatch(broadcastID, recipients int) *BroadcastBatch {
	now := time.Now().UTC()
	return &BroadcastBatch{
		BroadcastID: broadcastID,
		Status:      enum.BroadcastBatchStatusPending,
		Recipients:  recipients,
		Attempt:     0,
		Duration:    0,
		CreatedAt:   now,
		UpdatedAt:   now,
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
