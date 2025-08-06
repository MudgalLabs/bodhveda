package pg

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
	"github.com/mudgallabs/tantra/query"
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
	payload := repository.SearchRecipientPayload{
		Filters: repository.RecipientSearchFilter{
			ProjectID: &projectID,
		},
	}

	recipients, _, err := r.findRecipients(ctx, payload)
	return recipients, err
}

func (r *RecipientRepo) GetByProjectIDAndExternalID(ctx context.Context, projectID int, externalID string) (*entity.Recipient, error) {
	payload := repository.SearchRecipientPayload{
		Filters: repository.RecipientSearchFilter{
			ProjectID:  &projectID,
			ExternalID: &externalID,
		},
		Pagination: query.Pagination{Limit: 1},
	}

	recipients, _, err := r.findRecipients(ctx, payload)
	if err != nil {
		return nil, err
	}

	if len(recipients) == 0 {
		return nil, tantraRepo.ErrNotFound
	}

	return recipients[0], err
}

func (r *RecipientRepo) findRecipients(ctx context.Context, payload repository.SearchRecipientPayload) ([]*entity.Recipient, int, error) {
	baseSQL := `
		SELECT id, external_id, name, project_id, created_at, updated_at
		FROM recipient
		`

	builder := dbx.NewSQLBuilder(baseSQL)

	if payload.Filters.ProjectID != nil {
		builder.AddCompareFilter("project_id", "=", *payload.Filters.ProjectID)
	}

	if payload.Filters.ExternalID != nil {
		builder.AddCompareFilter("external_id", "=", *payload.Filters.ExternalID)
	}

	// Apply default sorting if not provided.
	if payload.Sort.Field == "" {
		payload.Sort.Field = "created_at"
	}

	if payload.Sort.Order == "" {
		payload.Sort.Order = query.SortOrderDESC
	}

	// Apply default pagination if not provided.
	if payload.Pagination.Limit <= 0 {
		payload.Pagination.Limit = 20
	}
	if payload.Pagination.Page <= 0 {
		payload.Pagination.Page = 1
	}

	builder.AddPagination(payload.Pagination.Limit, payload.Pagination.Offset())

	sql, args := builder.Build()

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query: %w", err)
	}

	defer rows.Close()

	recipients := []*entity.Recipient{}
	for rows.Next() {
		var newRecipient entity.Recipient

		err := rows.Scan(&newRecipient.ID, &newRecipient.ExternalID, &newRecipient.Name, &newRecipient.ProjectID, &newRecipient.CreatedAt, &newRecipient.UpdatedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("scan: %w", err)
		}

		recipients = append(recipients, &newRecipient)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows error: %w", err)
	}

	countSQL, countArgs := builder.Count()
	var total int
	err = r.db.QueryRow(ctx, countSQL, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	return recipients, total, nil
}
