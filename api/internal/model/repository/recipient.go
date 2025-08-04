package repository

import (
	"context"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
)

type RecipientRepository interface {
	RecipientReader
	RecipientWriter
}

type RecipientReader interface {
	List(ctx context.Context, projectID int) ([]*entity.Recipient, error)
}

type RecipientWriter interface {
	Create(ctx context.Context, recipient *entity.Recipient) (*entity.Recipient, error)
}
