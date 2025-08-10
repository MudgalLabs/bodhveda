package service

import (
	"context"
	"fmt"

	"github.com/mudgallabs/bodhveda/internal/env"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/cipher"
	tantraRepo "github.com/mudgallabs/tantra/repository"
	"github.com/mudgallabs/tantra/service"
)

type APIKeyService struct {
	repo        repository.APIKeyRepository
	projectRepo repository.ProjectReader
}

func NewAPIKeyService(repo repository.APIKeyRepository, projectRepo repository.ProjectReader) *APIKeyService {
	return &APIKeyService{
		repo:        repo,
		projectRepo: projectRepo,
	}
}

func (s *APIKeyService) Create(ctx context.Context, payload dto.CreateAPIKeyPayload) (*string, service.Error, error) {
	err := payload.Validate()
	if err != nil {
		return nil, service.ErrInvalidInput, err
	}

	apikey, err := entity.NewAPIKey(payload.UserID, payload.ProjectID, payload.Name, payload.Scope)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("create apikey: %w", err)
	}

	apikey, err = s.repo.Create(ctx, apikey)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("apikey repo create: %w", err)
	}

	plainToken, err := cipher.Decrypt(apikey.Token, apikey.Nonce, []byte(env.CipherKey))
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("decrypt apikey token: %w", err)
	}

	return &plainToken, service.ErrNone, nil
}

func (s *APIKeyService) List(ctx context.Context, userID, projectID int) ([]*dto.APIKey, service.Error, error) {
	apiKeys, err := s.repo.List(ctx, userID, projectID)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("list api keys: %w", err)
	}

	apiKeysDTOs := []*dto.APIKey{}
	for _, apiKey := range apiKeys {
		dto := dto.FromAPIKey(apiKey)
		apiKeysDTOs = append(apiKeysDTOs, dto)
	}

	return apiKeysDTOs, service.ErrNone, nil
}

func (s *APIKeyService) Delete(ctx context.Context, userID, projectID, apiKeyID int) (service.Error, error) {
	err := s.repo.Delete(ctx, userID, projectID, apiKeyID)
	if err != nil {
		if err == tantraRepo.ErrNotFound {
			return service.ErrNotFound, err
		}

		return service.ErrInternalServerError, err
	}

	return service.ErrNone, nil
}
