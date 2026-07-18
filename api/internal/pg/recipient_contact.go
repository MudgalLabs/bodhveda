package pg

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
	tantraRepo "github.com/mudgallabs/tantra/repository"
)

type RecipientContactRepo struct {
	db   dbx.DBExecutor
	pool *pgxpool.Pool
}

func NewRecipientContactRepo(db *pgxpool.Pool) repository.RecipientContactRepository {
	return &RecipientContactRepo{
		db:   db,
		pool: db,
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

// SetPrimaryContact — see the interface doc for the four cases. The read of the
// current primary is FOR UPDATE so a concurrent setter serializes behind it; the
// no-primary case relies on ux_recipient_contact_one_primary to reject a racing
// second insert (surfaced as ErrConflict).
func (r *RecipientContactRepo) SetPrimaryContact(ctx context.Context, contact *entity.RecipientContact) (*entity.RecipientContact, error) {
	var result *entity.RecipientContact

	err := dbx.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		selSQL := fmt.Sprintf(`
			SELECT %s
			FROM recipient_contact
			WHERE project_id = $1 AND recipient_external_id = $2 AND medium = $3 AND is_primary
			FOR UPDATE
		`, recipientContactFields)

		primary, err := scanRecipientContact(tx.QueryRow(ctx, selSQL, contact.ProjectID, contact.RecipientExtID, string(contact.Medium)))
		hasPrimary := err == nil
		if err != nil && err.Error() != "no rows in result set" {
			return err
		}

		// Primary already has this address → idempotent no-op, verification kept.
		if hasPrimary && primary.Address == contact.Address {
			result = primary
			return nil
		}

		if hasPrimary {
			// Move the primary onto the new address. The changed address nulls
			// verified_at; the CASE keeps it a no-op if somehow unchanged.
			updSQL := fmt.Sprintf(`
				UPDATE recipient_contact
				SET address = $1,
				    verified_at = CASE WHEN address IS DISTINCT FROM $1 THEN NULL ELSE verified_at END,
				    updated_at = now()
				WHERE id = $2
				RETURNING %s
			`, recipientContactFields)

			updated, err := scanRecipientContact(tx.QueryRow(ctx, updSQL, contact.Address, primary.ID))
			if err != nil {
				if dbx.IsUniqueViolation(err) {
					return tantraRepo.ErrConflict
				}
				return err
			}
			result = updated
			return nil
		}

		// No primary yet. If a (non-primary) contact already holds this address,
		// promote it — the address is unchanged, so verification is preserved.
		promoteSQL := fmt.Sprintf(`
			UPDATE recipient_contact
			SET is_primary = true, updated_at = now()
			WHERE project_id = $1 AND recipient_external_id = $2 AND medium = $3 AND address = $4
			RETURNING %s
		`, recipientContactFields)

		promoted, err := scanRecipientContact(tx.QueryRow(ctx, promoteSQL, contact.ProjectID, contact.RecipientExtID, string(contact.Medium), contact.Address))
		if err == nil {
			result = promoted
			return nil
		}
		if err.Error() != "no rows in result set" {
			if dbx.IsUniqueViolation(err) {
				return tantraRepo.ErrConflict
			}
			return err
		}

		// Nothing exists for this address → insert a fresh primary.
		insSQL := fmt.Sprintf(`
			INSERT INTO recipient_contact (project_id, recipient_external_id, medium, address, is_primary, verified_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, true, NULL, now(), now())
			RETURNING %s
		`, recipientContactFields)

		created, err := scanRecipientContact(tx.QueryRow(ctx, insSQL, contact.ProjectID, contact.RecipientExtID, string(contact.Medium), contact.Address))
		if err != nil {
			if dbx.IsUniqueViolation(err) {
				return tantraRepo.ErrConflict
			}
			return err
		}
		result = created
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
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

func (r *RecipientContactRepo) GetPrimary(ctx context.Context, projectID int, recipientExtID string, medium enum.Medium) (*entity.RecipientContact, error) {
	sql := fmt.Sprintf(`
		SELECT %s
		FROM recipient_contact
		WHERE project_id = $1 AND recipient_external_id = $2 AND medium = $3 AND is_primary
		LIMIT 1
	`, recipientContactFields)

	row := r.db.QueryRow(ctx, sql, projectID, recipientExtID, string(medium))
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
