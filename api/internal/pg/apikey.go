package pg

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
)

type APIKeyRepo struct {
	db   dbx.DBExecutor
	pool *pgxpool.Pool
}

func NewAPIKeyRepo(db *pgxpool.Pool) repository.APIKeyRepository {
	return &APIKeyRepo{
		db:   db,
		pool: db,
	}
}

func (r *APIKeyRepo) Create(ctx context.Context, key *entity.APIKey) (*entity.APIKey, error) {
	sql := `
		INSERT INTO api_key (name, token, nonce, scope, project_id, user_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, name, token, nonce, scope, project_id, user_id, created_at, updated_at
	`

	row := r.db.QueryRow(ctx, sql, key.Name, key.Token, key.Nonce, key.Scope, key.ProjectID, key.UserID, key.CreatedAt, key.UpdatedAt)

	var apiKey entity.APIKey

	err := row.Scan(&apiKey.ID, &apiKey.Name, &apiKey.Token, &apiKey.Nonce, &apiKey.Scope, &apiKey.ProjectID, &apiKey.UserID, &apiKey.CreatedAt, &apiKey.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &apiKey, nil
}

func (r *APIKeyRepo) List(ctx context.Context, userID, projectID int) ([]*entity.APIKey, error) {
	sql := `
		SELECT id, name, token, nonce, scope, project_id, user_id, created_at, updated_at
		FROM api_key
		WHERE user_id = $1 AND project_id = $2
	`

	rows, err := r.db.Query(ctx, sql, userID, projectID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	apiKeys := []*entity.APIKey{}
	for rows.Next() {
		var apiKey entity.APIKey

		err := rows.Scan(
			&apiKey.ID,
			&apiKey.Name,
			&apiKey.Token,
			&apiKey.Nonce,
			&apiKey.Scope,
			&apiKey.ProjectID,
			&apiKey.UserID,
			&apiKey.CreatedAt,
			&apiKey.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		apiKeys = append(apiKeys, &apiKey)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return apiKeys, nil
}
