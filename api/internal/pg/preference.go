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
	// The ON CONFLICT target must match the recipient partial unique index, which
	// now includes `medium` (see migration 20260712130000). This clause and that
	// index move in lock-step.
	sql := `
		INSERT INTO preference (project_id, recipient_external_id, channel, topic, event, medium, label, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (project_id, recipient_external_id, channel, topic, event, medium)
		WHERE recipient_external_id IS NOT NULL
		DO UPDATE SET
			enabled = EXCLUDED.enabled,
			updated_at = EXCLUDED.updated_at
		RETURNING id, project_id, recipient_external_id, channel, topic, event, medium, label, enabled, created_at, updated_at
	`

	row := r.db.QueryRow(ctx, sql, pref.ProjectID, pref.RecipientExtID, pref.Channel, pref.Topic, pref.Event, pref.Medium, pref.Label, pref.Enabled, pref.CreatedAt, pref.UpdatedAt)

	var newPref entity.Preference

	err := row.Scan(&newPref.ID, &newPref.ProjectID, &newPref.RecipientExtID, &newPref.Channel, &newPref.Topic, &newPref.Event, &newPref.Medium, &newPref.Label, &newPref.Enabled, &newPref.CreatedAt, &newPref.UpdatedAt)
	if err != nil {
		if dbx.IsUniqueViolation(err) {
			return nil, tantraRepo.ErrConflict
		}
		return nil, err
	}

	return &newPref, nil
}

// GetProjectPreferenceByID fetches one catalog entry by id, scoped to the
// project and to project-level rows (recipient_external_id IS NULL). The NULL
// filter is what confines the Developer API's catalog surface to catalog rows —
// a recipient-level row with the same id resolves to ErrNotFound here.
func (r *PreferenceRepo) GetProjectPreferenceByID(ctx context.Context, projectID int, preferenceID int) (*entity.Preference, error) {
	sql := `
		SELECT id, project_id, recipient_external_id, channel, topic, event, medium, label, enabled, created_at, updated_at
		FROM preference
		WHERE project_id = $1 AND id = $2 AND recipient_external_id IS NULL
	`

	row := r.db.QueryRow(ctx, sql, projectID, preferenceID)

	var pref entity.Preference
	err := row.Scan(&pref.ID, &pref.ProjectID, &pref.RecipientExtID, &pref.Channel, &pref.Topic, &pref.Event, &pref.Medium, &pref.Label, &pref.Enabled, &pref.CreatedAt, &pref.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, tantraRepo.ErrNotFound
		}
		return nil, err
	}

	return &pref, nil
}

// UpdateProjectPreference updates a catalog entry's mutable fields (label and
// the project-level default). Scoped to project-level rows (recipient NULL) for
// the same reason GetProjectPreferenceByID is; RETURNING gives back the fresh
// row so the caller need not re-read. ErrNotFound when nothing matched.
func (r *PreferenceRepo) UpdateProjectPreference(ctx context.Context, projectID int, preferenceID int, label string, enabled bool) (*entity.Preference, error) {
	sql := `
		UPDATE preference
		SET label = $3, enabled = $4, updated_at = now()
		WHERE project_id = $1 AND id = $2 AND recipient_external_id IS NULL
		RETURNING id, project_id, recipient_external_id, channel, topic, event, medium, label, enabled, created_at, updated_at
	`

	row := r.db.QueryRow(ctx, sql, projectID, preferenceID, label, enabled)

	var pref entity.Preference
	err := row.Scan(&pref.ID, &pref.ProjectID, &pref.RecipientExtID, &pref.Channel, &pref.Topic, &pref.Event, &pref.Medium, &pref.Label, &pref.Enabled, &pref.CreatedAt, &pref.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, tantraRepo.ErrNotFound
		}
		return nil, err
	}

	return &pref, nil
}

// DeleteProjectPreference removes a catalog entry by id, scoped to the project
// and to project-level rows — see GetProjectPreferenceByID for why the NULL
// filter matters. ErrNotFound when no project-level row with that id exists.
func (r *PreferenceRepo) DeleteProjectPreference(ctx context.Context, projectID int, preferenceID int) error {
	sql := `
		DELETE FROM preference
		WHERE project_id = $1 AND id = $2 AND recipient_external_id IS NULL;
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

func (r *PreferenceRepo) ListPreferences(ctx context.Context, projectID int, kind enum.PreferenceKind) ([]*entity.Preference, error) {
	prefs, _, err := r.findPreferences(ctx, repository.SearchPreferencePayload{
		Filters: repository.PreferenceSearchFilter{
			ProjectOrRecipient: kind,
			ProjectID:          projectID,
		},
	})
	return prefs, err
}

func (r *PreferenceRepo) findPreferences(ctx context.Context, payload repository.SearchPreferencePayload) ([]*entity.Preference, int, error) {
	baseSQL := `
		SELECT
			p.id, p.project_id, p.recipient_external_id, p.channel, p.topic, p.event, p.medium, p.label, p.enabled, p.created_at, p.updated_at
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
		err := rows.Scan(&newPref.ID, &newPref.ProjectID, &newPref.RecipientExtID, &newPref.Channel, &newPref.Topic, &newPref.Event, &newPref.Medium, &newPref.Label, &newPref.Enabled, &newPref.CreatedAt, &newPref.UpdatedAt)

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

// ShouldDirectNotificationBeDelivered resolves, for a single medium, whether a
// direct notification should be delivered. The preference cascade runs entirely
// within that medium (recipient-exact → recipient-fallback → project-exact →
// project-fallback).
//
// The default when nothing matches is medium-dependent:
//   - in_app defaults to DELIVER (true), preserving legacy behavior — direct
//     in-app notifications deliver unless explicitly muted, no catalog required.
//   - every other medium defaults to NOT deliver (false): it fires only when it
//     is cataloged (a project-level row exists) or the recipient explicitly
//     enabled it. This is the catalog gate for non-in_app transports.
func (r *PreferenceRepo) ShouldDirectNotificationBeDelivered(ctx context.Context, projectID int, recipientExtID string, target dto.Target, medium enum.Medium) (bool, error) {
	// Default delivery decision when no preference row is found.
	defaultDeliver := medium == enum.MediumInApp

	shouldDeliver := defaultDeliver

	shouldDeliverSQL := `
		-- INPUTS:
		-- $1 = project_id
		-- $2 = recipient_external_id
		-- $3 = channel
		-- $4 = topic (e.g. post_123)
		-- $5 = event
		-- $6 = medium
		-- $7 = default delivery decision when no preference matches

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
		      AND medium = $6
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
		      AND medium = $6
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
		      AND medium = $6
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
		      AND medium = $6
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
	        $7  -- default: medium-dependent (in_app delivers, others don't)
	    ) AS should_deliver;
	`

	err := r.db.QueryRow(ctx, shouldDeliverSQL,
		projectID,
		recipientExtID,
		target.Channel,
		target.Topic,
		target.Event,
		string(medium),
		defaultDeliver,
	).Scan(&shouldDeliver)
	if err != nil {
		return false, err
	}

	return shouldDeliver, err
}

// ResolveRecipientPreferences answers, for one recipient, what EVERY known
// (target, medium) actually resolves to — the same decision
// ShouldDirectNotificationBeDelivered would return, for every cell, in one
// round trip.
//
// It exists because a catalog-shaped read lies. The catalog is a DEFAULT, not a
// gate: an explicit recipient row wins the cascade before the catalog is
// consulted, so an uncataloged (target, email) with a recipient row set to true
// DELIVERS — while a read that only walks project preferences cannot see that
// row at all. Hence the target universe below is the catalog UNION the
// recipient's own rows.
//
// This is the second SQL resolver of the same cascade (the first being
// ShouldDirectNotificationBeDelivered, which answers one cell for the send
// path and must stay a single cheap query on the hot path). They are kept in
// step by a test that asserts they agree cell-for-cell —
// TestResolveRecipientPreferencesAgreesWithGating. Change one, change both.
//
// The cascade, per cell, is identical to the gating one: recipient-exact →
// recipient-fallback (topic='any', only when the cell's topic isn't 'none') →
// project-exact → project-fallback → medium-dependent default (in_app delivers,
// everything else does not). The partial unique indexes on preference guarantee
// at most one row per (project, recipient, channel, topic, event, medium), so
// these LEFT JOINs resolve one row each and cannot fan out — the structural
// reason the gating query's LIMIT 1 has no counterpart here.
func (r *PreferenceRepo) ResolveRecipientPreferences(ctx context.Context, projectID int, recipientExtID string, mediums []enum.Medium) ([]*entity.ResolvedPreference, error) {
	return r.resolvePreferences(ctx, projectID, recipientExtID, mediums, nil)
}

// ResolveRecipientPreferenceForTargets resolves exactly the targets it is given,
// with the same cascade — including targets nothing is stored about anywhere.
//
// That is the difference from ResolveRecipientPreferences, whose universe is
// "everything known" (the catalog UNION the recipient's rows). A caller asking
// about ONE arbitrary target cannot use that universe: an uncataloged target the
// recipient has no row for is absent from it, yet it still resolves — a project
// 'topic=any' rule can cover it, and in_app delivers by default regardless. So
// the answer must be computed, not looked up.
func (r *PreferenceRepo) ResolveRecipientPreferenceForTargets(ctx context.Context, projectID int, recipientExtID string, mediums []enum.Medium, targets []dto.Target) ([]*entity.ResolvedPreference, error) {
	if len(targets) == 0 {
		return []*entity.ResolvedPreference{}, nil
	}
	return r.resolvePreferences(ctx, projectID, recipientExtID, mediums, targets)
}

// resolvePreferences is the one cascade, run over a universe of cells.
//
// When targets is nil the universe is everything known about this recipient (the
// catalog UNION their own rows); otherwise it is exactly the targets given. Only
// that universe differs — the cascade below is shared, which is what keeps the
// list read and the single-target check from drifting apart.
func (r *PreferenceRepo) resolvePreferences(ctx context.Context, projectID int, recipientExtID string, mediums []enum.Medium, targets []dto.Target) ([]*entity.ResolvedPreference, error) {
	mediumStrs := make([]string, len(mediums))
	for i, m := range mediums {
		mediumStrs[i] = string(m)
	}

	args := []any{projectID, recipientExtID, mediumStrs}

	// Every target anything is known about: the project catalog PLUS any target
	// this recipient has a row for. That union is the point — a recipient row on
	// an uncataloged target still delivers, so omitting it would hide a live
	// preference.
	targetSQL := `
		    SELECT DISTINCT channel, topic, event
		    FROM preference
		    WHERE project_id = $1
		      AND (recipient_external_id IS NULL OR recipient_external_id = $2)`

	if targets != nil {
		channels := make([]string, len(targets))
		topics := make([]string, len(targets))
		events := make([]string, len(targets))
		for i, t := range targets {
			channels[i], topics[i], events[i] = t.Channel, t.Topic, t.Event
		}
		args = append(args, channels, topics, events)

		// The caller named the universe. Nothing needs to be stored for these to
		// resolve.
		targetSQL = `
		    SELECT DISTINCT channel, topic, event
		    FROM unnest($4::text[], $5::text[], $6::text[]) AS t(channel, topic, event)`
	}

	sql := `
		-- INPUTS:
		-- $1 = project_id
		-- $2 = recipient_external_id
		-- $3 = mediums to resolve (text[])
		-- $4,$5,$6 = explicit target universe (text[]), only when given

		WITH
		medium AS (
		    SELECT unnest($3::text[]) AS medium
		),

		target AS (` + targetSQL + `
		),

		cell AS (
		    SELECT t.channel, t.topic, t.event, m.medium
		    FROM target t CROSS JOIN medium m
		)

		SELECT
		    c.channel,
		    c.topic,
		    c.event,
		    c.medium,
		    pe.label,
		    -- Cataloged = a project-level row for this EXACT (target, medium).
		    -- Context for the UI; it deliberately does not gate the enabled value.
		    (pe.id IS NOT NULL) AS cataloged,
		    -- The cascade. Mirrors ShouldDirectNotificationBeDelivered's COALESCE
		    -- exactly, including the medium-dependent default.
		    COALESCE(
		        re.enabled,
		        rf.enabled,
		        pe.enabled,
		        pf.enabled,
		        c.medium = 'in_app'
		    ) AS enabled,
		    CASE
		        WHEN re.id IS NOT NULL THEN 'recipient_exact'
		        WHEN rf.id IS NOT NULL THEN 'recipient_any'
		        WHEN pe.id IS NOT NULL THEN 'project_exact'
		        WHEN pf.id IS NOT NULL THEN 'project_any'
		        ELSE 'default'
		    END AS source
		FROM cell c
		-- 1. recipient, exact topic
		LEFT JOIN preference re
		    ON re.project_id = $1
		   AND re.recipient_external_id = $2
		   AND re.channel = c.channel
		   AND re.topic = c.topic
		   AND re.event = c.event
		   AND re.medium = c.medium
		-- 2. recipient, topic='any' fallback (never for a 'none'-topic cell)
		LEFT JOIN preference rf
		    ON rf.project_id = $1
		   AND rf.recipient_external_id = $2
		   AND rf.channel = c.channel
		   AND rf.topic = 'any'
		   AND rf.event = c.event
		   AND rf.medium = c.medium
		   AND c.topic != 'none'
		-- 3. project, exact topic
		LEFT JOIN preference pe
		    ON pe.project_id = $1
		   AND pe.recipient_external_id IS NULL
		   AND pe.channel = c.channel
		   AND pe.topic = c.topic
		   AND pe.event = c.event
		   AND pe.medium = c.medium
		-- 4. project, topic='any' fallback (never for a 'none'-topic cell)
		LEFT JOIN preference pf
		    ON pf.project_id = $1
		   AND pf.recipient_external_id IS NULL
		   AND pf.channel = c.channel
		   AND pf.topic = 'any'
		   AND pf.event = c.event
		   AND pf.medium = c.medium
		   AND c.topic != 'none'
		ORDER BY c.channel, c.topic, c.event, c.medium;
	`

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	resolved := []*entity.ResolvedPreference{}
	for rows.Next() {
		var p entity.ResolvedPreference
		if err := rows.Scan(&p.Channel, &p.Topic, &p.Event, &p.Medium, &p.Label, &p.Cataloged, &p.Enabled, &p.Source); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		resolved = append(resolved, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return resolved, nil
}

// ListEligibleRecipientExtIDsForBroadcast returns recipients opted in to a
// (target, medium) for broadcast fan-out. Broadcasts are in-app only in v1 (email
// is direct-only — see the HARD RULE in agent-docs/overview.md), so callers pass
// enum.MediumInApp; the medium filter keeps the query correct now that
// preferences are per-medium.
func (r *PreferenceRepo) ListEligibleRecipientExtIDsForBroadcast(ctx context.Context, projectID int, target dto.Target, medium enum.Medium) ([]string, error) {
	sql := `
		-- INPUTS:
		-- $1 = project_id
		-- $2 = channel
		-- $3 = topic
		-- $4 = event
		-- $5 = medium

		SELECT r.external_id
		FROM recipient r
		LEFT JOIN preference rp
			ON rp.project_id = r.project_id
			AND rp.recipient_external_id = r.external_id
			AND rp.channel = $2
			AND rp.topic = $3
			AND rp.event = $4
			AND rp.medium = $5
		LEFT JOIN preference pp
			ON pp.project_id = r.project_id
			AND pp.recipient_external_id IS NULL
			AND pp.channel = $2
			AND pp.topic = $3
			AND pp.event = $4
			AND pp.medium = $5
		WHERE r.project_id = $1
			AND (
				rp.enabled = true
				OR (rp.id IS NULL AND pp.enabled = true)
			);
	`

	rows, err := r.db.Query(ctx, sql, projectID, target.Channel, target.Topic, target.Event, string(medium))
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

// DoesProjectPreferenceExist reports whether a (target, medium) is in the project
// catalog — i.e. a project-level preference row exists for it. It gates the
// broadcast precondition (callers pass enum.MediumInApp) and is the catalog
// primitive for non-in_app mediums.
func (r *PreferenceRepo) DoesProjectPreferenceExist(ctx context.Context, projectID int, target dto.Target, medium enum.Medium) (bool, error) {
	sql := `
		SELECT true
		FROM preference
		WHERE project_id = $1
		  AND recipient_external_id IS NULL
		  AND channel = $2
		  AND topic = $3
		  AND event = $4
		  AND medium = $5
		LIMIT 1;
	`

	var exists bool

	err := r.db.QueryRow(ctx, sql, projectID, target.Channel, target.Topic, target.Event, string(medium)).Scan(&exists)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}

		return false, fmt.Errorf("query: %w", err)
	}

	return exists, nil
}
