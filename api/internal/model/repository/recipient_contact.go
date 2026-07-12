package repository

import (
	"context"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
)

type RecipientContactRepository interface {
	RecipientContactReader
	RecipientContactWriter
}

type RecipientContactReader interface {
	List(ctx context.Context, projectID int, recipientExtID string) ([]*entity.RecipientContact, error)
	Get(ctx context.Context, projectID int, recipientExtID string, contactID int64) (*entity.RecipientContact, error)
}

type RecipientContactWriter interface {
	Create(ctx context.Context, contact *entity.RecipientContact) (*entity.RecipientContact, error)
	Update(ctx context.Context, projectID int, recipientExtID string, contactID int64, payload *dto.UpdateRecipientContactPayload) (*entity.RecipientContact, error)
	Delete(ctx context.Context, projectID int, recipientExtID string, contactID int64) error
}
