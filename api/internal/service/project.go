package service

import (
	"context"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/service"
)

type ProjectService struct {
	repo repository.ProjectRepository
}

func NewProjectService(repo repository.ProjectRepository) *ProjectService {
	return &ProjectService{
		repo,
	}
}

func (s *ProjectService) Create(ctx context.Context, userID int, payload dto.CreateProjectPaylaod) (*dto.Project, service.Error, error) {
	return nil, service.ErrNone, nil
}
