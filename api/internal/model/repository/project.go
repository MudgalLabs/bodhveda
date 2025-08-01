package repository

import (
	"context"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
)

type ProjectRepository interface {
	ProjectReader
	ProjectWriter
}

type ProjectReader interface {
	// ListProjects(ctx context.Context, userID int) ([]*entity.Project, error)
}

type ProjectWriter interface {
	Create(ctx context.Context, userID int, payload dto.CreateProjectPaylaod) (*entity.Project, error)
}
