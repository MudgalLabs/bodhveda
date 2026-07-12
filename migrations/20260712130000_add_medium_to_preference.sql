-- Adds a `medium` dimension to preferences so the gating layer can decide per
-- transport (in_app, email, ...) whether a target may deliver. Project-level
-- (recipient_external_id IS NULL) rows form the catalog: a (target, medium) must
-- be declared before that medium can fire.
--
-- Existing rows backfill to 'in_app' via the column default (metadata-only on
-- PG >= 11, no table rewrite), preserving legacy in-app behavior exactly.
--
-- The two partial unique indexes are rebuilt with `medium` appended. This runs
-- outside a transaction (the NO TRANSACTION directive below) so CREATE UNIQUE
-- INDEX CONCURRENTLY can be used and never takes a table-blocking lock. The
-- matching INSERT ... ON CONFLICT change in api/internal/pg/preference.go MUST
-- ship in lock-step: the old ON CONFLICT target stops matching the recreated
-- partial unique the moment the index is rebuilt.

-- +goose NO TRANSACTION

-- +goose Up
ALTER TABLE preference
    ADD COLUMN IF NOT EXISTS medium TEXT NOT NULL DEFAULT 'in_app'
    CHECK (medium IN ('in_app', 'email', 'sms', 'web_push', 'mobile_push'));

DROP INDEX IF EXISTS recipient_pref_unique;
DROP INDEX IF EXISTS project_pref_unique;

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS recipient_pref_unique
    ON preference(project_id, recipient_external_id, channel, topic, event, medium)
    WHERE recipient_external_id IS NOT NULL;

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS project_pref_unique
    ON preference(project_id, channel, topic, event, medium)
    WHERE recipient_external_id IS NULL;

-- +goose Down
DROP INDEX IF EXISTS recipient_pref_unique;
DROP INDEX IF EXISTS project_pref_unique;

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS recipient_pref_unique
    ON preference(project_id, recipient_external_id, channel, topic, event)
    WHERE recipient_external_id IS NOT NULL;

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS project_pref_unique
    ON preference(project_id, channel, topic, event)
    WHERE recipient_external_id IS NULL;

ALTER TABLE preference DROP COLUMN IF EXISTS medium;
