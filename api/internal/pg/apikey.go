package pg

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
	tantraRepo "github.com/mudgallabs/tantra/repository"
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
		INSERT INTO api_key (name, token, nonce, token_hash, scope, project_id, user_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, name, token, nonce, token_hash, scope, project_id, user_id, created_at, updated_at
	`

	row := r.db.QueryRow(ctx, sql, key.Name, key.Token, key.Nonce, key.TokenHash, key.Scope, key.ProjectID, key.UserID, key.CreatedAt, key.UpdatedAt)

	var apiKey entity.APIKey

	err := row.Scan(&apiKey.ID, &apiKey.Name, &apiKey.Token, &apiKey.Nonce, &apiKey.TokenHash, &apiKey.Scope, &apiKey.ProjectID, &apiKey.UserID, &apiKey.CreatedAt, &apiKey.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &apiKey, nil
}

func (r *APIKeyRepo) List(ctx context.Context, userID, projectID int) ([]*entity.APIKey, error) {
	sql := `
		SELECT id, name, token, nonce, token_hash, scope, project_id, user_id, created_at, updated_at
		FROM api_key
		WHERE user_id = $1 AND project_id = $2
		ORDER BY id DESC
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
			&apiKey.TokenHash,
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

func (r *APIKeyRepo) GetByTokenHash(ctx context.Context, tokenHash string) (*entity.APIKey, error) {
	sql := `
		SELECT id, name, token, nonce, token_hash, scope, project_id, user_id, created_at, updated_at
		FROM api_key
		WHERE token_hash = $1
	`
	row := r.db.QueryRow(ctx, sql, tokenHash)

	var apiKey entity.APIKey
	err := row.Scan(
		&apiKey.ID,
		&apiKey.Name,
		&apiKey.Token,
		&apiKey.Nonce,
		&apiKey.TokenHash,
		&apiKey.Scope,
		&apiKey.ProjectID,
		&apiKey.UserID,
		&apiKey.CreatedAt,
		&apiKey.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &apiKey, nil
}

func (r *APIKeyRepo) DeleteForProject(ctx context.Context, projectID int) (int, error) {
	sql := `
		DELETE FROM api_key
		WHERE project_id = $1
	`

	tag, err := r.db.Exec(ctx, sql, projectID)
	if err != nil {
		return 0, err
	}

	return int(tag.RowsAffected()), nil
}

func (r *APIKeyRepo) Delete(ctx context.Context, userID, projectID, apiKeyID int) error {
	sql := `
		DELETE FROM api_key
		WHERE id = $1 AND user_id = $2 AND project_id = $3
	`

	tag, err := r.db.Exec(ctx, sql, apiKeyID, userID, projectID)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return tantraRepo.ErrNotFound
	}

	return nil
}
