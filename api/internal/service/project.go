package service

import (
	"context"
	"fmt"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
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

func (s *ProjectService) Create(ctx context.Context, payload dto.CreateProjectPaylaod) (*dto.Project, service.Error, error) {
	// TODO: Limit free users to 1/2 project.

	err := payload.Validate()
	if err != nil {
		return nil, service.ErrInvalidInput, err
	}

	project := entity.NewProject(payload.UserID, payload.Name)
	project, err = s.repo.Create(ctx, project)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("project repo create: %w", err)
	}

	return dto.FromProject(project), service.ErrNone, nil
}

func (s *ProjectService) List(ctx context.Context, userID int) ([]*dto.Project, service.Error, error) {
	projects, err := s.repo.List(ctx, userID)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("project repo list: %w", err)
	}

	return dto.FromProjects(projects), service.ErrNone, nil
}
