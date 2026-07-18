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
	// SetPrimaryContact idempotently ensures contact.Address is the recipient's
	// primary contact for contact.Medium, in one transaction:
	//   - no primary yet, address unknown → insert a new primary (verified_at NULL)
	//   - no primary yet, address already a contact → promote that row to primary
	//   - primary already has this address → return it unchanged (verification kept)
	//   - primary has a different address → update it in place, nulling verified_at
	// The one-primary-per-(recipient,medium) invariant is held by
	// ux_recipient_contact_one_primary; moving a primary onto an address a
	// different contact already holds collides with the (recipient,medium,address)
	// unique and surfaces as ErrConflict.
	SetPrimaryContact(ctx context.Context, contact *entity.RecipientContact) (*entity.RecipientContact, error)
	Update(ctx context.Context, projectID int, recipientExtID string, contactID int64, payload *dto.UpdateRecipientContactPayload) (*entity.RecipientContact, error)
	Delete(ctx context.Context, projectID int, recipientExtID string, contactID int64) error
}
