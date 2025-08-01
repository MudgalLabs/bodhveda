package pg

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
)

type ProjectRepo struct {
	db dbx.DBExecutor
}

func NewProjectRepo(db *pgxpool.Pool) repository.ProjectRepository {
	return &ProjectRepo{db}
}

func (r *ProjectRepo) Create(ctx context.Context, userID int, payload dto.CreateProjectPaylaod) (*entity.Project, error) {
	return nil, nil
}
