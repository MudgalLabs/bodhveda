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
	GetByID(ctx context.Context, id int) (*entity.Broadcast, error)
}

type BroadcastWriter interface {
	Create(ctx context.Context, notification *entity.Broadcast) (*entity.Broadcast, error)
	Update(ctx context.Context, notification *entity.Broadcast) error
	DeleteForProject(ctx context.Context, projectID int) (int, error)
}
