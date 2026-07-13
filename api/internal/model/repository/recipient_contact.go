package repository

import (
	"context"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
)

type RecipientContactRepository interface {
	RecipientContactReader
	RecipientContactWriter
}

type RecipientContactReader interface {
	List(ctx context.Context, projectID int, recipientExtID string) ([]*entity.RecipientContact, error)
	Get(ctx context.Context, projectID int, recipientExtID string, contactID int64) (*entity.RecipientContact, error)
	// GetPrimary returns the recipient's primary contact for a medium (the row
	// WHERE is_primary, guarded by ux_recipient_contact_one_primary), or
	// tantra repository.ErrNotFound when there is none. Used by the email
	// fan-out to resolve the address to send to.
	GetPrimary(ctx context.Context, projectID int, recipientExtID string, medium enum.Medium) (*entity.RecipientContact, error)
}

type RecipientContactWriter interface {
	Create(ctx context.Context, contact *entity.RecipientContact) (*entity.RecipientContact, error)
	Update(ctx context.Context, projectID int, recipientExtID string, contactID int64, payload *dto.UpdateRecipientContactPayload) (*entity.RecipientContact, error)
	Delete(ctx context.Context, projectID int, recipientExtID string, contactID int64) error
}
