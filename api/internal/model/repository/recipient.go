package repository

import (
	"context"

	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/tantra/query"
)

type RecipientRepository interface {
	RecipientReader
	RecipientWriter
}

type RecipientReader interface {
	List(ctx context.Context, projectID int, pagination query.Pagination) ([]*entity.RecipientListItem, int, error)
	Get(ctx context.Context, projectID int, externalID string) (*entity.Recipient, error)
	Exists(ctx context.Context, projectID int, externalID string) (bool, error)
	TotalCount(ctx context.Context, projectID int) (int, error)
}

type RecipientWriter interface {
	Create(ctx context.Context, recipient *entity.Recipient) (*entity.Recipient, error)
	BatchCreate(ctx context.Context, recipients []*entity.Recipient) (created []string, updated []string, err error)
	Update(ctx context.Context, projectID int, externalID string, payload *dto.UpdateRecipientPayload) (*entity.Recipient, error)
	SoftDelete(ctx context.Context, projectID int, externalID string) error
	Delete(ctx context.Context, projectID int, externalID string) error
	DeleteForProject(ctx context.Context, projectID int) (int, error)
}

type RecipientSearchFilter struct {
	ProjectID  *int    `json:"project_id"`
	ExternalID *string `json:"external_id"`
}

type SearchRecipientPayload = query.SearchPayload[RecipientSearchFilter]
