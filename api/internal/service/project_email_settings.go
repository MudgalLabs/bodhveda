package service

import (
	"context"
	"fmt"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	tantraRepo "github.com/mudgallabs/tantra/repository"
	"github.com/mudgallabs/tantra/service"
)

type ProjectEmailSettingsService struct {
	repo repository.ProjectEmailSettingsRepository
}

func NewProjectEmailSettingsService(repo repository.ProjectEmailSettingsRepository) *ProjectEmailSettingsService {
	return &ProjectEmailSettingsService{
		repo: repo,
	}
}

// Get returns the project's email settings as a masked DTO, or (nil, ErrNone,
// nil) when the project has none configured yet — the console treats a null body
// as "not configured".
func (s *ProjectEmailSettingsService) Get(ctx context.Context, projectID int) (*dto.ProjectEmailSettings, service.Error, error) {
	if projectID <= 0 {
		return nil, service.ErrInvalidInput, fmt.Errorf("projectID required")
	}

	settings, err := s.repo.Get(ctx, projectID)
	if err != nil {
		if err == tantraRepo.ErrNotFound {
			return nil, service.ErrNone, nil
		}
		return nil, service.ErrInternalServerError, fmt.Errorf("project email settings repo get: %w", err)
	}

	result, err := s.toMaskedDTO(settings)
	if err != nil {
		return nil, service.ErrInternalServerError, err
	}

	return result, service.ErrNone, nil
}

// Upsert sets or rotates the project's email settings. The secret may be omitted
// on update to keep the existing key (identity/provider-only edit); it is
// required when configuring for the first time.
func (s *ProjectEmailSettingsService) Upsert(ctx context.Context, payload *dto.UpsertProjectEmailSettingsPayload) (*dto.ProjectEmailSettings, service.Error, error) {
	if payload.ProjectID <= 0 {
		return nil, service.ErrInvalidInput, fmt.Errorf("projectID required")
	}

	// Load any existing config first so the caller can rotate identity without
	// resending the secret, and so Validate knows whether the secret is required.
	existing, err := s.repo.Get(ctx, payload.ProjectID)
	if err != nil && err != tantraRepo.ErrNotFound {
		return nil, service.ErrInternalServerError, fmt.Errorf("project email settings repo get: %w", err)
	}
	payload.SetHasExisting(existing != nil)

	if err := payload.Validate(); err != nil {
		return nil, service.ErrInvalidInput, err
	}

	var settings *entity.ProjectEmailSettings
	if payload.Secret != "" {
		// New key provided (first-time config or rotation): encrypt it fresh.
		settings, err = entity.NewProjectEmailSettings(
			payload.ProjectID, enum.EmailProvider(payload.Provider), payload.Secret, payload.FromName, payload.FromAddress,
		)
		if err != nil {
			return nil, service.ErrInternalServerError, fmt.Errorf("build project email settings: %w", err)
		}
		if existing != nil {
			settings.CreatedAt = existing.CreatedAt
		}
	} else {
		// Identity/provider-only update: keep the existing encrypted secret.
		settings = existing
		settings.Provider = enum.EmailProvider(payload.Provider)
		settings.FromName = payload.FromName
		settings.FromAddress = payload.FromAddress
	}

	saved, err := s.repo.Upsert(ctx, settings)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("project email settings repo upsert: %w", err)
	}

	result, err := s.toMaskedDTO(saved)
	if err != nil {
		return nil, service.ErrInternalServerError, err
	}

	return result, service.ErrNone, nil
}

// toMaskedDTO decrypts the secret only to derive a display-safe masked hint; the
// plaintext never leaves this function.
func (s *ProjectEmailSettingsService) toMaskedDTO(settings *entity.ProjectEmailSettings) (*dto.ProjectEmailSettings, error) {
	plain, err := settings.DecryptSecret()
	if err != nil {
		return nil, fmt.Errorf("decrypt provider secret: %w", err)
	}

	return &dto.ProjectEmailSettings{
		Provider:     string(settings.Provider),
		FromName:     settings.FromName,
		FromAddress:  settings.FromAddress,
		SecretMasked: dto.MaskSecret(plain),
		CreatedAt:    settings.CreatedAt,
		UpdatedAt:    settings.UpdatedAt,
	}, nil
}
