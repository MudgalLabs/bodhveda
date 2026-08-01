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
	// Get looks a project up by id alone — no user scoping, because the send path
	// reaches it through an API key that is already project-scoped.
	Get(ctx context.Context, projectID int) (*entity.Project, error)
	List(ctx context.Context, userID int) ([]*entity.Project, error)
	UserOwns(ctx context.Context, userID, projectID int) (bool, error)
}

type ProjectWriter interface {
	Create(ctx context.Context, project *entity.Project) (*entity.Project, error)
	// Update is partial: a nil strictTargets keeps the stored value.
	Update(ctx context.Context, userID, projectID int, name string, strictTargets *bool) (*entity.Project, error)
	SoftDelete(ctx context.Context, userID, projectID int) error
	Delete(ctx context.Context, projectID int) error
}
