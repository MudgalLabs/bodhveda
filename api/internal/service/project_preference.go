package service

import (
	"context"
	"fmt"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/service"
)

type ProjectPreferenceService struct {
	repo repository.ProjectPreferenceRepository
}

func NewProjectPreferenceService(repo repository.ProjectPreferenceRepository) *ProjectPreferenceService {
	return &ProjectPreferenceService{repo: repo}
}

func (s *ProjectPreferenceService) Create(ctx context.Context, payload dto.CreateProjectPreferencePayload) (*dto.ProjectPreference, service.Error, error) {
	if err := payload.Validate(); err != nil {
		return nil, service.ErrInvalidInput, err
	}

	e := &entity.ProjectPreference{
		ProjectID:      payload.ProjectID,
		Channel:        payload.Channel,
		Topic:          payload.Topic,
		Event:          payload.Event,
		Label:          payload.Label,
		DefaultEnabled: payload.DefaultEnabled,
	}

	newPref, err := s.repo.Create(ctx, e)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("repo create project preference: %w", err)
	}

	return dto.FromProjectPreference(newPref), service.ErrNone, nil
}

func (s *ProjectPreferenceService) List(ctx context.Context, projectID int) ([]*dto.ProjectPreference, service.Error, error) {
	prefs, err := s.repo.List(ctx, projectID)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("repo list project preferences: %w", err)
	}

	dtos := []*dto.ProjectPreference{}
	for _, e := range prefs {
		dtos = append(dtos, dto.FromProjectPreference(e))
	}

	return dtos, service.ErrNone, nil
}
