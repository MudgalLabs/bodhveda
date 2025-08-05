package service

import (
	"context"
	"fmt"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	tantraRepo "github.com/mudgallabs/tantra/repository"
	"github.com/mudgallabs/tantra/service"
)

type PreferenceService struct {
	repo repository.PreferenceRepository
}

func NewProjectPreferenceService(repo repository.PreferenceRepository) *PreferenceService {
	return &PreferenceService{repo: repo}
}

func (s *PreferenceService) CreateProjectPreference(ctx context.Context, payload dto.CreateProjectPreferencePayload) (*dto.ProjectPreference, service.Error, error) {
	if err := payload.Validate(); err != nil {
		return nil, service.ErrInvalidInput, err
	}

	pref := entity.NewProjectPreference(
		&payload.ProjectID,
		nil,
		payload.Channel,
		payload.Topic,
		payload.Event,
		payload.Label,
		payload.Enabled,
	)

	newPref, err := s.repo.Create(ctx, pref)
	if err != nil {
		if err == tantraRepo.ErrConflict {
			return nil, service.ErrConflict, fmt.Errorf("Preference already exists")
		}
		return nil, service.ErrInternalServerError, fmt.Errorf("repo create preference: %w", err)
	}

	return dto.FromProjectPreference(newPref), service.ErrNone, nil
}

func (s *PreferenceService) ListProjectPreferences(ctx context.Context, projectID int) ([]*dto.ProjectPreference, service.Error, error) {
	prefs, err := s.repo.ListProjectPreferences(ctx, projectID)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("repo list preferences: %w", err)
	}

	dtos := []*dto.ProjectPreference{}
	for _, e := range prefs {
		dtos = append(dtos, dto.FromProjectPreference(e))
	}

	return dtos, service.ErrNone, nil
}
