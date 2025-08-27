package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
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

func (r *RecipientRepo) BatchCreate(ctx context.Context, recipients []*entity.Recipient) (created []string, updated []string, err error) {
	if len(recipients) == 0 {
		return nil, nil, nil
	}

	sql := `
		INSERT INTO recipient (external_id, name, project_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (project_id, external_id) DO UPDATE
		SET name = EXCLUDED.name, updated_at = EXCLUDED.updated_at
		RETURNING (xmax = 0) AS inserted, external_id
	`

	batch := &pgx.Batch{}
	for _, recipient := range recipients {
		batch.Queue(sql, recipient.ExternalID, recipient.Name, recipient.ProjectID, recipient.CreatedAt, recipient.UpdatedAt)
	}

	batchResult := r.pool.SendBatch(ctx, batch)
	defer batchResult.Close()

	created = []string{}
	updated = []string{}
	for i := range recipients {
		var inserted bool
		var externalID string
		err := batchResult.QueryRow().Scan(&inserted, &externalID)
		if err != nil {
			return created, updated, fmt.Errorf("batch upsert recipient %d: %w", i, err)
		}
		if inserted {
			created = append(created, externalID)
		} else {
			updated = append(updated, externalID)
		}
	}

	return created, updated, nil
}

func (r *RecipientRepo) List(ctx context.Context, projectID int, pagination query.Pagination) ([]*entity.RecipientListItem, int, error) {
	payload := repository.SearchRecipientPayload{
		Filters: repository.RecipientSearchFilter{
			ProjectID: &projectID,
		},
		Pagination: pagination,
	}
	recipients, total, err := r.findRecipients(ctx, payload, true)
	return recipients, total, err
}

func (r *RecipientRepo) Get(ctx context.Context, projectID int, externalID string) (*entity.Recipient, error) {
	payload := repository.SearchRecipientPayload{
		Filters: repository.RecipientSearchFilter{
			ProjectID:  &projectID,
			ExternalID: &externalID,
		},
		Pagination: query.Pagination{Limit: 1},
	}

	recipients, _, err := r.findRecipients(ctx, payload, false)
	if err != nil {
		return nil, err
	}

	if len(recipients) == 0 {
		return nil, tantraRepo.ErrNotFound
	}

	return &recipients[0].Recipient, err
}

func (r *RecipientRepo) Update(ctx context.Context, projectID int, externalID string, payload *dto.UpdateRecipientPayload) (*entity.Recipient, error) {
	sql := `
		UPDATE recipient
		SET name = $1, updated_at = $2
		WHERE project_id = $3 AND external_id = $4
		RETURNING id, external_id, name, project_id, created_at, updated_at
	`
	row := r.db.QueryRow(ctx, sql, payload.Name, time.Now().UTC(), projectID, externalID)
	var updated entity.Recipient
	err := row.Scan(&updated.ID, &updated.ExternalID, &updated.Name, &updated.ProjectID, &updated.CreatedAt, &updated.UpdatedAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, tantraRepo.ErrNotFound
		}
		return nil, err
	}

	return &updated, nil
}

func (r *RecipientRepo) SoftDelete(ctx context.Context, projectID int, externalID string) error {
	sql := `
		UPDATE recipient
		SET deleted_at = $1
		WHERE project_id = $2 AND external_id = $3
	`
	res, err := r.db.Exec(ctx, sql, time.Now().UTC(), projectID, externalID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return tantraRepo.ErrNotFound
	}
	return nil
}

func (r *RecipientRepo) Delete(ctx context.Context, projectID int, externalID string) error {
	sql := `
		DELETE FROM recipient
		WHERE project_id = $1 AND external_id = $2
	`
	_, err := r.db.Exec(ctx, sql, projectID, externalID)
	return err
}

func (r *RecipientRepo) findRecipients(ctx context.Context, payload repository.SearchRecipientPayload, includeNotificationsCount bool) ([]*entity.RecipientListItem, int, error) {
	const baseFields = `
	r.id, r.external_id, r.name, r.project_id, r.created_at, r.updated_at
`

	var baseSQL string
	if includeNotificationsCount {
		baseSQL = fmt.Sprintf(`
		SELECT %s, 
		COALESCE(SUM(CASE WHEN n.id IS NOT NULL AND n.broadcast_id IS NULL THEN 1 ELSE 0 END), 0) AS direct_count,
		COALESCE(SUM(CASE WHEN n.id IS NOT NULL AND n.broadcast_id IS NOT NULL THEN 1 ELSE 0 END), 0) AS broadcast_count
		FROM recipient r
		LEFT JOIN notification n ON n.recipient_external_id = r.external_id
	`, baseFields)
	} else {
		baseSQL = fmt.Sprintf(`
		SELECT %s
		FROM recipient r
	`, baseFields)
	}

	builder := dbx.NewSQLBuilder(baseSQL)

	// Never include soft-deleted recipients in the results.
	builder.AppendWhere("r.deleted_at IS NULL")

	if payload.Filters.ProjectID != nil {
		builder.AddCompareFilter("r.project_id", "=", *payload.Filters.ProjectID)
	}

	if payload.Filters.ExternalID != nil {
		builder.AddCompareFilter("r.external_id", "=", *payload.Filters.ExternalID)
	}

	// Apply default sorting if not provided.
	if payload.Sort.Field == "" {
		payload.Sort.Field = "r.id"
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

	if includeNotificationsCount {
		builder.AddGroupBy("r.id, r.external_id, r.name, r.project_id, r.created_at, r.updated_at")
	}

	builder.AddPagination(payload.Pagination.Limit, payload.Pagination.Offset())
	builder.AddSorting(payload.Sort.Field, payload.Sort.Order)

	sql, args := builder.Build()

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query: %w", err)
	}

	defer rows.Close()

	recipients := []*entity.RecipientListItem{}
	for rows.Next() {
		var newRecipient entity.RecipientListItem
		var err error

		if includeNotificationsCount {
			err = rows.Scan(&newRecipient.ID, &newRecipient.ExternalID, &newRecipient.Name, &newRecipient.ProjectID, &newRecipient.CreatedAt, &newRecipient.UpdatedAt, &newRecipient.DirectNotificationsCount, &newRecipient.BroadcastNotificationsCount)
		} else {
			err = rows.Scan(&newRecipient.ID, &newRecipient.ExternalID, &newRecipient.Name, &newRecipient.ProjectID, &newRecipient.CreatedAt, &newRecipient.UpdatedAt)
		}

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

func (r *RecipientRepo) Exists(ctx context.Context, projectID int, externalID string) (bool, error) {
	sql := `
		SELECT EXISTS (
			SELECT 1 FROM recipient
			WHERE project_id = $1 AND external_id = $2
		)
	`

	var exists bool

	err := r.db.QueryRow(ctx, sql, projectID, externalID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("query and scan: %w", err)
	}

	return exists, nil
}

func (r *RecipientRepo) TotalCount(ctx context.Context, projectID int) (int, error) {
	sql := `
		SELECT COUNT(*)
		FROM recipient
		WHERE project_id = $1
	`

	var count int
	err := r.db.QueryRow(ctx, sql, projectID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("query and scan: %w", err)
	}

	return count, nil
}

func (r *RecipientRepo) DeleteForProject(ctx context.Context, projectID int) (int, error) {
	sql := `
		DELETE FROM recipient
		WHERE project_id = $1
	`

	tag, err := r.db.Exec(ctx, sql, projectID)
	if err != nil {
		return 0, fmt.Errorf("delete: %w", err)
	}

	return int(tag.RowsAffected()), nil
}
