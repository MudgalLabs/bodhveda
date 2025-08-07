package repository

import (
	"context"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
)

type BroadcastBatchRepository interface {
	BroadcastBatchReader
	BroadcastBatchWriter
}

type BroadcastBatchReader interface {
	PendingCount(ctx context.Context, broadcastID int) (int, error)
}

type BroadcastBatchWriter interface {
	Create(ctx context.Context, broadcastBatch *entity.BroadcastBatch) (*entity.BroadcastBatch, error)
	Update(ctx context.Context, batchID int, paylaod *entity.BroadcastBatchUpdatePayload) error
}
