package repository

import (
	"context"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/tantra/query"
)

type BroadcastRepository interface {
	BroadcastReader
	BroadcastWriter
}

type BroadcastReader interface {
	GetByID(ctx context.Context, id int) (*entity.Broadcast, error)
	List(ctx context.Context, projectID int, pagination query.Pagination) ([]*dto.BroadcastListItem, int, error)
}

type BroadcastWriter interface {
	Create(ctx context.Context, notification *entity.Broadcast) (*entity.Broadcast, error)
	Update(ctx context.Context, notification *entity.Broadcast) error
	DeleteForProject(ctx context.Context, projectID int) (int, error)
}
