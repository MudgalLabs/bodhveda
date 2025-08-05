package pg

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
	tantraRepo "github.com/mudgallabs/tantra/repository"
)

type RecipientRepo struct {
	db   dbx.DBExecutor
	pool *pgxpool.Pool
}

func NewRecipientRepo(db *pgxpool.Pool) repository.RecipientRepository {
	return &RecipientRepo{
		db:   db,
		pool: db,
	}
}

func (r *RecipientRepo) Create(ctx context.Context, recipient *entity.Recipient) (*entity.Recipient, error) {
	sql := `
		INSERT INTO recipient (external_id, name, project_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, external_id, name, project_id, created_at, updated_at
	`
	row := r.db.QueryRow(ctx, sql, recipient.ExternalID, recipient.Name, recipient.ProjectID, recipient.CreatedAt, recipient.UpdatedAt)

	var newRecipient entity.Recipient

	err := row.Scan(&newRecipient.ID, &newRecipient.ExternalID, &newRecipient.Name, &newRecipient.ProjectID, &newRecipient.CreatedAt, &newRecipient.UpdatedAt)
	if err != nil {
		if dbx.IsUniqueViolation(err) {
			return nil, tantraRepo.ErrConflict
		}
		return nil, err
	}

	return &newRecipient, nil
}

func (r *RecipientRepo) List(ctx context.Context, projectID int) ([]*entity.Recipient, error) {
	// TODO: Add pagination and filtering.
	sql := `
		SELECT id, external_id, name, project_id, created_at, updated_at
		FROM recipient
		WHERE project_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, sql, projectID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	recipients := []*entity.Recipient{}
	for rows.Next() {
		var newRecipient entity.Recipient
		err := rows.Scan(&newRecipient.ID, &newRecipient.ExternalID, &newRecipient.Name, &newRecipient.ProjectID, &newRecipient.CreatedAt, &newRecipient.UpdatedAt)
		if err != nil {
			return nil, err
		}
		recipients = append(recipients, &newRecipient)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return recipients, nil
}
