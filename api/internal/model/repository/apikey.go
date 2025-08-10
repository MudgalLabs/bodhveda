package repository

import (
	"context"

	"github.com/mudgallabs/bodhveda/internal/model/entity"
)

type APIKeyRepository interface {
	APIKeyReader
	APIKeyWriter
}

type APIKeyReader interface {
	List(ctx context.Context, userID, projectID int) ([]*entity.APIKey, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (*entity.APIKey, error)
	DeleteForProject(ctx context.Context, projectID int) (int, error)
	Delete(ctx context.Context, userID, projectID, apiKeyID int) error
}

type APIKeyWriter interface {
	Create(ctx context.Context, key *entity.APIKey) (*entity.APIKey, error)
}
