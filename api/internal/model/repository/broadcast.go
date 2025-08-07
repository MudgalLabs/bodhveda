package repository

import (
	"context"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
)

type BroadcastRepository interface {
	BroadcastReader
	BroadcastWriter
}

type BroadcastReader interface {
}

type BroadcastWriter interface {
	Create(ctx context.Context, notification *entity.Broadcast) (*entity.Broadcast, error)
}
