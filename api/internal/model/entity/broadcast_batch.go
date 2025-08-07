package entity

import (
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/enum"
)

type BroadcastBatch struct {
	ID          int
	BroadcastID int
	Status      enum.BroadcastBatchStatus
	Attempt     int
	Duration    int // Duration in milliseconds
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewBroadcastBatch(broadcastID int) *BroadcastBatch {
	now := time.Now().UTC()
	return &BroadcastBatch{
		BroadcastID: broadcastID,
		Status:      enum.BroadcastBatchStatusPending,
		Attempt:     0,
		Duration:    0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
