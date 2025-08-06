package pg

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/entity"
	"github.com/mudgallabs/bodhveda/internal/model/enum"
	"github.com/mudgallabs/bodhveda/internal/model/repository"
	"github.com/mudgallabs/tantra/dbx"
	"github.com/mudgallabs/tantra/query"
	tantraRepo "github.com/mudgallabs/tantra/repository"
)

type PreferenceRepo struct {
	db   dbx.DBExecutor
	pool *pgxpool.Pool
}

func NewPreferenceRepo(db *pgxpool.Pool) repository.PreferenceRepository {
	return &PreferenceRepo{
		db:   db,
		pool: db,
	}
}

func (r *PreferenceRepo) Create(ctx context.Context, pref *entity.Preference) (*entity.Preference, error) {
	sql := `
		INSERT INTO preference (project_id, recipient_id, channel, topic, event, label, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (recipient_id, channel, topic, event) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			updated_at = EXCLUDED.updated_at
		RETURNING id, project_id, recipient_id, channel, topic, event, label, enabled, created_at, updated_at
	`

	row := r.db.QueryRow(ctx, sql, pref.ProjectID, pref.RecipientID, pref.Channel, pref.Topic, pref.Event, pref.Label, pref.Enabled, pref.CreatedAt, pref.UpdatedAt)

	var newPref entity.Preference

	err := row.Scan(&newPref.ID, &newPref.ProjectID, &newPref.RecipientID, &newPref.Channel, &newPref.Topic, &newPref.Event, &newPref.Label, &newPref.Enabled, &newPref.CreatedAt, &newPref.UpdatedAt)
	if err != nil {
		if dbx.IsUniqueViolation(err) {
			return nil, tantraRepo.ErrConflict
		}
		return nil, err
	}

	return &newPref, nil
}

func (r *PreferenceRepo) ListPreferences(ctx context.Context, projectID int, kind enum.PreferenceKind) ([]*entity.Preference, error) {
	prefs, _, err := r.findPreferences(ctx, repository.SearchPreferencePayload{
		Filters: repository.PreferenceSearchFilter{
			ProjectOrRecipient: kind,
			ProjectID:          &projectID,
		},
	})
	return prefs, err
}

func (r *PreferenceRepo) findPreferences(ctx context.Context, payload repository.SearchPreferencePayload) ([]*entity.Preference, int, error) {
	baseSQL := `
		SELECT p.id, p.project_id, p.recipient_id, '' AS recipient_external_id, p.channel, p.topic, p.event, p.label, p.enabled, p.created_at, p.updated_at
		FROM preference p
	`

	if payload.Filters.ProjectOrRecipient == enum.PreferenceKindRecipient {
		baseSQL = `
		SELECT p.id, p.project_id, p.recipient_id, r.external_id AS recipient_external_id, p.channel, p.topic, p.event, p.label, p.enabled, p.created_at, p.updated_at
		FROM preference p
		JOIN recipient r ON r.id = p.recipient_id
	`
	}

	builder := dbx.NewSQLBuilder(baseSQL)

	if payload.Filters.ProjectID != nil {
		// When we are filtering for project preferences, we filter by project_id on the preference itself.
		if payload.Filters.ProjectOrRecipient == enum.PreferenceKindProject {
			builder.AddCompareFilter("p.project_id", "=", *payload.Filters.ProjectID)
		}

		// When we are filtering for recipient preferences, we need to filter by project_id
		// which we get from the recipient table as on preference table, for recipient preferences,
		// project_id is NULL.
		if payload.Filters.ProjectOrRecipient == enum.PreferenceKindRecipient {
			builder.AddCompareFilter("r.project_id", "=", *payload.Filters.ProjectID)
		}
	}

	// Apply default sorting if not provided.
	if payload.Sort.Field == "" {
		payload.Sort.Field = "p.channel, p.label"
	}

	if payload.Sort.Order == "" {
		payload.Sort.Order = query.SortOrderASC
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

	prefs := []*entity.Preference{}
	for rows.Next() {
		var newPref entity.Preference
		err := rows.Scan(&newPref.ID, &newPref.ProjectID, &newPref.RecipientID, &newPref.RecipientExtID, &newPref.Channel, &newPref.Topic, &newPref.Event, &newPref.Label, &newPref.Enabled, &newPref.CreatedAt, &newPref.UpdatedAt)

		if err != nil {
			return nil, 0, err
		}

		prefs = append(prefs, &newPref)
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

	return prefs, total, nil
}
