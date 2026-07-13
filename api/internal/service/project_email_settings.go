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

	// Start from the existing row (so blank secrets keep the current encrypted
	// values) or a fresh one, then apply the payload. The provider secret and the
	// webhook secret are rotated independently: each is (re)encrypted only when the
	// caller supplies a new plaintext, otherwise the existing ciphertext is kept.
	var settings *entity.ProjectEmailSettings
	if existing != nil {
		settings = existing
	} else {
		now := time.Now().UTC()
		settings = &entity.ProjectEmailSettings{ProjectID: payload.ProjectID, CreatedAt: now}
	}
	settings.Provider = enum.EmailProvider(payload.Provider)
	settings.FromName = payload.FromName
	settings.FromAddress = payload.FromAddress
	settings.UpdatedAt = time.Now().UTC()

	if payload.Secret != "" {
		if err := settings.SetSecret(payload.Secret); err != nil {
			return nil, service.ErrInternalServerError, fmt.Errorf("encrypt provider secret: %w", err)
		}
	}
	if payload.WebhookSecret != "" {
		if err := settings.SetWebhookSecret(payload.WebhookSecret); err != nil {
			return nil, service.ErrInternalServerError, fmt.Errorf("encrypt webhook secret: %w", err)
		}
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

	// The webhook secret is optional; only decrypt + mask it when configured.
	var webhookMasked string
	if settings.HasWebhookSecret() {
		webhookPlain, err := settings.DecryptWebhookSecret()
		if err != nil {
			return nil, fmt.Errorf("decrypt webhook secret: %w", err)
		}
		webhookMasked = dto.MaskSecret(webhookPlain)
	}

	return &dto.ProjectEmailSettings{
		Provider:            string(settings.Provider),
		FromName:            settings.FromName,
		FromAddress:         settings.FromAddress,
		SecretMasked:        dto.MaskSecret(plain),
		WebhookSecretMasked: webhookMasked,
		WebhookSecretSet:    settings.HasWebhookSecret(),
		CreatedAt:           settings.CreatedAt,
		UpdatedAt:           settings.UpdatedAt,
	}, nil
}
