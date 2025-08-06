package repository

import (
	"context"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/tantra/query"
)

type RecipientRepository interface {
	RecipientReader
	RecipientWriter
}

type RecipientReader interface {
	List(ctx context.Context, projectID int) ([]*entity.Recipient, error)
	GetByProjectIDAndExternalID(ctx context.Context, projectID int, externalID string) (*entity.Recipient, error)
}

type RecipientWriter interface {
	Create(ctx context.Context, recipient *entity.Recipient) (*entity.Recipient, error)
}

type RecipientSearchFilter struct {
	ProjectID  *int    `json:"project_id"`
	ExternalID *string `json:"external_id"`
}

type SearchRecipientPayload = query.SearchPayload[RecipientSearchFilter]
