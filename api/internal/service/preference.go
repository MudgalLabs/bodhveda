package service

import (
	"context"
	"fmt"
	"time"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	tantraRepo "github.com/mudgallabs/tantra/repository"
	"github.com/mudgallabs/tantra/service"
)

type PreferenceService struct {
	repo          repository.PreferenceRepository
	recipientRepo repository.RecipientRepository
}

func NewProjectPreferenceService(repo repository.PreferenceRepository, recipientRepo repository.RecipientRepository) *PreferenceService {
	return &PreferenceService{
		repo:          repo,
		recipientRepo: recipientRepo,
	}
}

func (s *PreferenceService) CreateProjectPreference(ctx context.Context, payload dto.CreateProjectPreferencePayload) (*dto.ProjectPreference, service.Error, error) {
	if err := payload.Validate(); err != nil {
		return nil, service.ErrInvalidInput, err
	}

	pref := entity.NewPreference(
		&payload.ProjectID,
		nil,
		payload.Channel,
		payload.Topic,
		payload.Event,
		&payload.Label,
		payload.Enabled,
	)

	newPref, err := s.repo.Create(ctx, pref)
	if err != nil {
		if err == tantraRepo.ErrConflict {
			return nil, service.ErrConflict, fmt.Errorf("Preference already exists")
		}
		return nil, service.ErrInternalServerError, fmt.Errorf("repo create preference: %w", err)
	}

	return dto.FromPreferenceForProject(newPref), service.ErrNone, nil
}

func (s *PreferenceService) ListProjectPreferences(ctx context.Context, projectID int) ([]*dto.ProjectPreference, service.Error, error) {
	prefs, err := s.repo.ListPreferences(ctx, projectID, enum.PreferenceKindProject)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("repo list preferences: %w", err)
	}

	dtos := []*dto.ProjectPreference{}
	for _, e := range prefs {
		dtos = append(dtos, dto.FromPreferenceForProject(e))
	}

	return dtos, service.ErrNone, nil
}

func (s *PreferenceService) ListRecipientPreferences(ctx context.Context, projectID int) ([]*dto.RecipientPreference, service.Error, error) {
	prefs, err := s.repo.ListPreferences(ctx, projectID, enum.PreferenceKindRecipient)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("repo list preferences: %w", err)
	}

	dtos := []*dto.RecipientPreference{}
	for _, e := range prefs {
		dtos = append(dtos, dto.FromPreferenceForRecipient(e))
	}

	return dtos, service.ErrNone, nil
}

func (s *PreferenceService) UpsertRecipientPreference(ctx context.Context, payload dto.UpsertRecipientPreferencePayload) (*dto.RecipientPreference, service.Error, error) {
	if err := payload.Validate(); err != nil {
		return nil, service.ErrInvalidInput, err
	}

	pref := entity.NewPreference(
		&payload.ProjectID,
		&payload.RecipientExtID,
		payload.Channel,
		payload.Topic,
		payload.Event,
		nil,
		payload.Enabled,
	)

	pref.UpdatedAt = time.Now().UTC()

	newPref, err := s.repo.Create(ctx, pref)
	if err != nil {
		if err == tantraRepo.ErrConflict {
			return nil, service.ErrConflict, fmt.Errorf("Preference already exists")
		}
		return nil, service.ErrInternalServerError, fmt.Errorf("repo create preference: %w", err)
	}

	return dto.FromPreferenceForRecipient(newPref), service.ErrNone, nil
}
