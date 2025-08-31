package pg

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudgallabs/bodhveda/internal/model/dto"
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
		INSERT INTO preference (project_id, recipient_external_id, channel, topic, event, label, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (project_id, recipient_external_id, channel, topic, event)
		WHERE recipient_external_id IS NOT NULL
		DO UPDATE SET
			enabled = EXCLUDED.enabled,
			updated_at = EXCLUDED.updated_at
		RETURNING id, project_id, recipient_external_id, channel, topic, event, label, enabled, created_at, updated_at
	`

	row := r.db.QueryRow(ctx, sql, pref.ProjectID, pref.RecipientExtID, pref.Channel, pref.Topic, pref.Event, pref.Label, pref.Enabled, pref.CreatedAt, pref.UpdatedAt)

	var newPref entity.Preference

	err := row.Scan(&newPref.ID, &newPref.ProjectID, &newPref.RecipientExtID, &newPref.Channel, &newPref.Topic, &newPref.Event, &newPref.Label, &newPref.Enabled, &newPref.CreatedAt, &newPref.UpdatedAt)
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
			ProjectID:          projectID,
		},
	})
	return prefs, err
}

func (r *PreferenceRepo) ListPreferencesForRecipient(ctx context.Context, projectID int, recipientExtID string) ([]*entity.Preference, error) {
	returned, _, err := r.findPreferences(ctx, repository.SearchPreferencePayload{
		Filters: repository.PreferenceSearchFilter{
			ProjectOrRecipient: enum.PreferenceKindRecipient,
			ProjectID:          projectID,
			RecipientExtID:     &recipientExtID,
		},
	})
	return returned, err
}

func (r *PreferenceRepo) findPreferences(ctx context.Context, payload repository.SearchPreferencePayload) ([]*entity.Preference, int, error) {
	baseSQL := `
		SELECT 
			p.id, p.project_id, p.recipient_external_id, p.channel, p.topic, p.event, p.label, p.enabled, p.created_at, p.updated_at
		FROM preference p
	`

	builder := dbx.NewSQLBuilder(baseSQL)

	if payload.Filters.ProjectID > 0 {
		builder.AddCompareFilter("p.project_id", "=", payload.Filters.ProjectID)
	}

	switch payload.Filters.ProjectOrRecipient {
	case enum.PreferenceKindRecipient:
		builder.AppendWhere("p.recipient_external_id IS NOT NULL")
	case enum.PreferenceKindProject:
		builder.AppendWhere("p.recipient_external_id IS NULL")
	}

	// Add recipient_ext_id filter if set
	if payload.Filters.RecipientExtID != nil {
		builder.AddCompareFilter("p.recipient_external_id", "=", *payload.Filters.RecipientExtID)
	}

	// Apply default sorting if not provided.
	if payload.Sort.Field == "" {
		payload.Sort.Field = "p.channel, p.label"
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

	prefs := []*entity.Preference{}
	for rows.Next() {
		var newPref entity.Preference
		err := rows.Scan(&newPref.ID, &newPref.ProjectID, &newPref.RecipientExtID, &newPref.Channel, &newPref.Topic, &newPref.Event, &newPref.Label, &newPref.Enabled, &newPref.CreatedAt, &newPref.UpdatedAt)

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

func (r *PreferenceRepo) ShouldDirectNotificationBeDelivered(ctx context.Context, projectID int, recipientExtID string, target dto.Target) (bool, error) {
	shouldDeliver := true

	shouldDeliverSQL := `
		-- INPUTS:
		-- $1 = project_id
		-- $2 = recipient_external_id
		-- $3 = channel
		-- $4 = topic (e.g. post_123)
		-- $5 = event

		WITH
		-- 1. Try recipient preference for exact match
		recipient_exact_pref AS (
		    SELECT enabled
		    FROM preference
		    WHERE project_id = $1
		      AND recipient_external_id= $2
		      AND channel = $3
		      AND topic = $4
		      AND event = $5
		    LIMIT 1
		),

		-- 2. Try recipient preference for fallback (topic = 'any'), only if topic != 'none'
		recipient_fallback_pref AS (
		    SELECT enabled
		    FROM preference
		    WHERE project_id = $1
		      AND recipient_external_id= $2
		      AND channel = $3
		      AND topic = 'any'
		      AND event = $5
		      AND $4 != 'none'
		    LIMIT 1
		),

		-- 3. Try project-level preference for exact match
		project_exact_pref AS (
		    SELECT enabled
		    FROM preference
		    WHERE project_id = $1
		      AND recipient_external_id IS NULL
		      AND channel = $3
		      AND topic = $4
		      AND event = $5
		    LIMIT 1
		),

		-- 4. Try project-level preference for fallback (topic = 'any'), only if topic != 'none'
		project_fallback_pref AS (
		    SELECT enabled
		    FROM preference
		    WHERE project_id = $1
		      AND recipient_external_id IS NULL
		      AND channel = $3
		      AND topic = 'any'
		      AND event = $5
		      AND $4 != 'none'
		    LIMIT 1
		)

		-- Final selection logic: pick the first available preference match
		SELECT
		    COALESCE(
		        (SELECT enabled FROM recipient_exact_pref),
		        (SELECT enabled FROM recipient_fallback_pref),
		        (SELECT enabled FROM project_exact_pref),
		        (SELECT enabled FROM project_fallback_pref),
	        true  -- default: DELIVER if nothing found (for direct notification)
	    ) AS should_deliver;
	`

	err := r.db.QueryRow(ctx, shouldDeliverSQL,
		projectID,
		recipientExtID,
		target.Channel,
		target.Topic,
		target.Event,
	).Scan(&shouldDeliver)
	if err != nil {
		return false, err
	}

	return shouldDeliver, err
}

func (r *PreferenceRepo) ListEligibleRecipientExtIDsForBroadcast(ctx context.Context, projectID int, target dto.Target) ([]string, error) {
	sql := `
		-- INPUTS:
		-- $1 = project_id
		-- $2 = channel
		-- $3 = topic
		-- $4 = event

		SELECT r.external_id
		FROM recipient r
		LEFT JOIN preference rp
			ON rp.project_id = r.project_id
			AND rp.recipient_external_id = r.external_id
			AND rp.channel = $2
			AND rp.topic = $3
			AND rp.event = $4
		LEFT JOIN preference pp
			ON pp.project_id = r.project_id
			AND pp.recipient_external_id IS NULL
			AND pp.channel = $2
			AND pp.topic = $3
			AND pp.event = $4
		WHERE r.project_id = $1
			AND (
				rp.enabled = true
				OR (rp.id IS NULL AND pp.enabled = true)
			);
	`

	rows, err := r.db.Query(ctx, sql, projectID, target.Channel, target.Topic, target.Event)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	defer rows.Close()

	var extIDs []string
	for rows.Next() {
		var extID string
		if err := rows.Scan(&extID); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		extIDs = append(extIDs, extID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return extIDs, nil
}

func (r *PreferenceRepo) DeleteForRecipient(ctx context.Context, projectID int, recipientExtID string) (int, error) {
	sql := `
		DELETE FROM preference
		WHERE project_id = $1 AND recipient_external_id = $2;
	`

	tag, err := r.db.Exec(ctx, sql, projectID, recipientExtID)
	if err != nil {
		return 0, fmt.Errorf("delete: %w", err)
	}

	return int(tag.RowsAffected()), nil
}

func (r *PreferenceRepo) DeleteForProject(ctx context.Context, projectID int) (int, error) {
	sql := `
		DELETE FROM preference
		WHERE project_id = $1;
	`

	tag, err := r.db.Exec(ctx, sql, projectID)
	if err != nil {
		return 0, fmt.Errorf("delete: %w", err)
	}

	return int(tag.RowsAffected()), nil
}

func (r *PreferenceRepo) Delete(ctx context.Context, projectID int, preferenceID int) error {
	sql := `
		DELETE FROM preference
		WHERE project_id = $1 AND id = $2;
	`

	tag, err := r.db.Exec(ctx, sql, projectID, preferenceID)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return tantraRepo.ErrNotFound
	}

	return nil
}

func (r *PreferenceRepo) DoesProjectPreferenceExist(ctx context.Context, projectID int, target dto.Target) (bool, error) {
	sql := `
		SELECT true
		FROM preference
		WHERE project_id = $1
		  AND recipient_external_id IS NULL
		  AND channel = $2
		  AND topic = $3
		  AND event = $4
		LIMIT 1;
	`

	var exists bool

	err := r.db.QueryRow(ctx, sql, projectID, target.Channel, target.Topic, target.Event).Scan(&exists)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}

		return false, fmt.Errorf("query: %w", err)
	}

	return exists, nil
}
