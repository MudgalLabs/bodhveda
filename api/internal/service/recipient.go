package service

import (
	"context"
	"fmt"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/service"
)

type RecipientService struct {
	repo repository.RecipientRepository
}

func NewRecipientService(repo repository.RecipientRepository) *RecipientService {
	return &RecipientService{
		repo,
	}
}

func (s *RecipientService) Create(ctx context.Context, payload dto.CreateRecipientPayload) (*dto.Recipient, service.Error, error) {
	err := payload.Validate()
	if err != nil {
		return nil, service.ErrInvalidInput, err
	}

	name := ""
	if payload.Name != nil {
		name = *payload.Name
	}

	recipient := entity.NewRecipient(payload.ProjectID, payload.ExternalID, name)
	recipient, err = s.repo.Create(ctx, recipient)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("recipient repo create: %w", err)
	}

	return dto.FromRecipient(recipient), service.ErrNone, nil
}

func (s *RecipientService) List(ctx context.Context, projectID int) ([]*dto.Recipient, service.Error, error) {
	recipients, err := s.repo.List(ctx, projectID)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("recipient repo list: %w", err)
	}

	return dto.FromRecipients(recipients), service.ErrNone, nil
}
