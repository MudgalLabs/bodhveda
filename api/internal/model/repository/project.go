package repository

import (
	"context"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
)

type ProjectRepository interface {
	ProjectReader
	ProjectWriter
}

type ProjectReader interface {
	List(ctx context.Context, userID int) ([]*entity.Project, error)
	UserOwns(ctx context.Context, userID, projectID int) (bool, error)
}

type ProjectWriter interface {
	Create(ctx context.Context, project *entity.Project) (*entity.Project, error)
}
