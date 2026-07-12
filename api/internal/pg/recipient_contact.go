package pg

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
	tantraRepo "github.com/mudgallabs/tantra/repository"
)

type RecipientContactRepo struct {
	db dbx.DBExecutor
}

func NewRecipientContactRepo(db *pgxpool.Pool) repository.RecipientContactRepository {
	return &RecipientContactRepo{
		db: db,
	}
}

const recipientContactFields = `
	id, project_id, recipient_external_id, medium, address, is_primary, verified_at, created_at, updated_at
`

func scanRecipientContact(row interface {
	Scan(dest ...any) error
}) (*entity.RecipientContact, error) {
	var c entity.RecipientContact
	var medium string
	err := row.Scan(&c.ID, &c.ProjectID, &c.RecipientExtID, &medium, &c.Address, &c.IsPrimary, &c.VerifiedAt, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	c.Medium = enum.Medium(medium)
	return &c, nil
}

func (r *RecipientContactRepo) Create(ctx context.Context, contact *entity.RecipientContact) (*entity.RecipientContact, error) {
	sql := fmt.Sprintf(`
		INSERT INTO recipient_contact (project_id, recipient_external_id, medium, address, is_primary, verified_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING %s
	`, recipientContactFields)

	row := r.db.QueryRow(ctx, sql,
		contact.ProjectID, contact.RecipientExtID, string(contact.Medium), contact.Address,
		contact.IsPrimary, contact.VerifiedAt, contact.CreatedAt, contact.UpdatedAt,
	)

	created, err := scanRecipientContact(row)
	if err != nil {
		if dbx.IsUniqueViolation(err) {
			return nil, tantraRepo.ErrConflict
		}
		return nil, err
	}

	return created, nil
}

func (r *RecipientContactRepo) List(ctx context.Context, projectID int, recipientExtID string) ([]*entity.RecipientContact, error) {
	sql := fmt.Sprintf(`
		SELECT %s
		FROM recipient_contact
		WHERE project_id = $1 AND recipient_external_id = $2
		ORDER BY medium ASC, is_primary DESC, id ASC
	`, recipientContactFields)

	rows, err := r.db.Query(ctx, sql, projectID, recipientExtID)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	contacts := []*entity.RecipientContact{}
	for rows.Next() {
		contact, err := scanRecipientContact(rows)
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		contacts = append(contacts, contact)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return contacts, nil
}

func (r *RecipientContactRepo) Get(ctx context.Context, projectID int, recipientExtID string, contactID int64) (*entity.RecipientContact, error) {
	sql := fmt.Sprintf(`
		SELECT %s
		FROM recipient_contact
		WHERE project_id = $1 AND recipient_external_id = $2 AND id = $3
	`, recipientContactFields)

	row := r.db.QueryRow(ctx, sql, projectID, recipientExtID, contactID)
	contact, err := scanRecipientContact(row)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, tantraRepo.ErrNotFound
		}
		return nil, err
	}

	return contact, nil
}

func (r *RecipientContactRepo) Update(ctx context.Context, projectID int, recipientExtID string, contactID int64, payload *dto.UpdateRecipientContactPayload) (*entity.RecipientContact, error) {
	setClauses := []string{}
	args := []any{}
	argN := 1

	if payload.Address != nil {
		// Changing the address invalidates verification. If the address is unchanged
		// the existing verified_at is preserved.
		setClauses = append(setClauses, fmt.Sprintf("address = $%d", argN))
		args = append(args, *payload.Address)
		argN++
		setClauses = append(setClauses, fmt.Sprintf("verified_at = CASE WHEN address IS DISTINCT FROM $%d THEN NULL ELSE verified_at END", argN))
		args = append(args, *payload.Address)
		argN++
	}

	if payload.IsPrimary != nil {
		setClauses = append(setClauses, fmt.Sprintf("is_primary = $%d", argN))
		args = append(args, *payload.IsPrimary)
		argN++
	}

	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argN))
	args = append(args, time.Now().UTC())
	argN++

	sql := fmt.Sprintf(`
		UPDATE recipient_contact
		SET %s
		WHERE project_id = $%d AND recipient_external_id = $%d AND id = $%d
		RETURNING %s
	`, strings.Join(setClauses, ", "), argN, argN+1, argN+2, recipientContactFields)
	args = append(args, projectID, recipientExtID, contactID)

	row := r.db.QueryRow(ctx, sql, args...)
	updated, err := scanRecipientContact(row)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, tantraRepo.ErrNotFound
		}
		if dbx.IsUniqueViolation(err) {
			return nil, tantraRepo.ErrConflict
		}
		return nil, err
	}

	return updated, nil
}

func (r *RecipientContactRepo) Delete(ctx context.Context, projectID int, recipientExtID string, contactID int64) error {
	sql := `
		DELETE FROM recipient_contact
		WHERE project_id = $1 AND recipient_external_id = $2 AND id = $3
	`
	res, err := r.db.Exec(ctx, sql, projectID, recipientExtID, contactID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return tantraRepo.ErrNotFound
	}
	return nil
}
