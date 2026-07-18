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

type RecipientContactService struct {
	repo          repository.RecipientContactRepository
	recipientRepo repository.RecipientRepository
}

func NewRecipientContactService(repo repository.RecipientContactRepository, recipientRepo repository.RecipientRepository) *RecipientContactService {
	return &RecipientContactService{
		repo:          repo,
		recipientRepo: recipientRepo,
	}
}

func (s *RecipientContactService) Create(ctx context.Context, payload dto.CreateRecipientContactPayload) (*dto.RecipientContact, service.Error, error) {
	if err := payload.Validate(); err != nil {
		return nil, service.ErrInvalidInput, err
	}

	// A contact hangs off an existing recipient (FK). Check up-front so callers
	// get a clean 404 rather than a foreign-key error.
	exists, err := s.recipientRepo.Exists(ctx, payload.ProjectID, payload.RecipientExtID)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("recipient exists check: %w", err)
	}
	if !exists {
		return nil, service.ErrNotFound, fmt.Errorf("Recipient not found")
	}

	contact := entity.NewRecipientContact(payload.ProjectID, payload.RecipientExtID, enum.Medium(payload.Medium), payload.Address, payload.IsPrimary)
	contact, err = s.repo.Create(ctx, contact)
	if err != nil {
		if err == tantraRepo.ErrConflict {
			// Either the same (medium, address) already exists for the recipient,
			// or a primary contact already exists for this medium.
			return nil, service.ErrConflict, fmt.Errorf("Contact already exists or a primary contact for this medium is already set")
		}
		return nil, service.ErrInternalServerError, fmt.Errorf("recipient contact repo create: %w", err)
	}

	return dto.FromRecipientContact(contact), service.ErrNone, nil
}

// SetPrimary idempotently ensures an address is the recipient's primary contact
// for a medium — the single-call "keep the primary email current" sync. It is a
// create-or-update: 200 whether it inserted, promoted, updated, or no-op'd (see
// the repo method for the four cases).
func (s *RecipientContactService) SetPrimary(ctx context.Context, payload dto.SetPrimaryContactPayload) (*dto.RecipientContact, service.Error, error) {
	if err := payload.Validate(); err != nil {
		return nil, service.ErrInvalidInput, err
	}

	// A contact hangs off an existing recipient (FK). Check up-front so callers
	// get a clean 404 rather than a foreign-key error.
	exists, err := s.recipientRepo.Exists(ctx, payload.ProjectID, payload.RecipientExtID)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("recipient exists check: %w", err)
	}
	if !exists {
		return nil, service.ErrNotFound, fmt.Errorf("Recipient not found")
	}

	contact := entity.NewRecipientContact(payload.ProjectID, payload.RecipientExtID, enum.Medium(payload.Medium), payload.Address, true)
	contact, err = s.repo.SetPrimaryContact(ctx, contact)
	if err != nil {
		if err == tantraRepo.ErrConflict {
			// The target address is already a different contact for this recipient
			// and medium — can't be made primary without displacing it explicitly.
			return nil, service.ErrConflict, fmt.Errorf("Another contact already uses this address for this medium")
		}
		return nil, service.ErrInternalServerError, fmt.Errorf("recipient contact repo set primary: %w", err)
	}

	return dto.FromRecipientContact(contact), service.ErrNone, nil
}

func (s *RecipientContactService) List(ctx context.Context, projectID int, recipientExtID string) (*dto.ListRecipientContactsResult, service.Error, error) {
	if projectID <= 0 || recipientExtID == "" {
		return nil, service.ErrInvalidInput, fmt.Errorf("projectID and recipient id required")
	}

	contacts, err := s.repo.List(ctx, projectID, recipientExtID)
	if err != nil {
		return nil, service.ErrInternalServerError, fmt.Errorf("recipient contact repo list: %w", err)
	}

	return &dto.ListRecipientContactsResult{
		Contacts: dto.FromRecipientContacts(contacts),
	}, service.ErrNone, nil
}

func (s *RecipientContactService) Update(ctx context.Context, payload *dto.UpdateRecipientContactPayload) (*dto.RecipientContact, service.Error, error) {
	if payload.ProjectID <= 0 || payload.RecipientExtID == "" || payload.ContactID <= 0 {
		return nil, service.ErrInvalidInput, fmt.Errorf("projectID, recipient id and contact id required")
	}

	// Load the existing contact so validation can normalize the address against
	// the correct medium, and so a missing contact returns 404 before the write.
	existing, err := s.repo.Get(ctx, payload.ProjectID, payload.RecipientExtID, payload.ContactID)
	if err != nil {
		if err == tantraRepo.ErrNotFound {
			return nil, service.ErrNotFound, fmt.Errorf("Contact not found")
		}
		return nil, service.ErrInternalServerError, fmt.Errorf("recipient contact repo get: %w", err)
	}
	payload.Medium = existing.Medium

	if err := payload.Validate(); err != nil {
		return nil, service.ErrInvalidInput, err
	}

	updated, err := s.repo.Update(ctx, payload.ProjectID, payload.RecipientExtID, payload.ContactID, payload)
	if err != nil {
		if err == tantraRepo.ErrNotFound {
			return nil, service.ErrNotFound, fmt.Errorf("Contact not found")
		}
		if err == tantraRepo.ErrConflict {
			return nil, service.ErrConflict, fmt.Errorf("Another contact conflicts with this change (duplicate address or existing primary for this medium)")
		}
		return nil, service.ErrInternalServerError, fmt.Errorf("recipient contact repo update: %w", err)
	}

	return dto.FromRecipientContact(updated), service.ErrNone, nil
}

func (s *RecipientContactService) Delete(ctx context.Context, projectID int, recipientExtID string, contactID int64) (service.Error, error) {
	if projectID <= 0 || recipientExtID == "" || contactID <= 0 {
		return service.ErrInvalidInput, fmt.Errorf("projectID, recipient id and contact id required")
	}

	err := s.repo.Delete(ctx, projectID, recipientExtID, contactID)
	if err != nil {
		if err == tantraRepo.ErrNotFound {
			return service.ErrNotFound, fmt.Errorf("Contact not found")
		}
		return service.ErrInternalServerError, fmt.Errorf("recipient contact repo delete: %w", err)
	}

	return service.ErrNone, nil
}
