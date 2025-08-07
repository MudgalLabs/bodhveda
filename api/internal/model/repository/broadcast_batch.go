package repository

import (
	"context"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
)

type BroadcastBatchRepository interface {
	BroadcastBatchReader
	BroadcastBatchWriter
}

type BroadcastBatchReader interface {
}

type BroadcastBatchWriter interface {
	Create(ctx context.Context, broadcastBatch *entity.BroadcastBatch) (*entity.BroadcastBatch, error)
	Update(ctx context.Context, batchID int, status enum.BroadcastBatchStatus, duration int) error
}
