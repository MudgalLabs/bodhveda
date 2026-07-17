-- +goose NO TRANSACTION

-- The `notification` table had NO index except its primary key — not even on
-- `project_id`. Every console notifications list was therefore a sequential scan
-- of the whole table, filtered down to one project afterwards.
--
-- This was invisible for two reasons: the dev DB holds a handful of rows, and
-- when one project owns the highest ids the planner walks `notification_pkey`
-- backward and stops at LIMIT, which looks fine. It is not fine for any OTHER
-- project: their rows sit behind that one in id order, so the backward walk has
-- to cross the whole table to reach them, and the planner gives up and seq scans.
--
-- Measured on 400k rows across two projects (Phase 9.4), listing a project whose
-- 4 rows sat behind 200k others:
--   before: Parallel Seq Scan, 4707 buffers, 14.3 ms
--   after:  Index Scan,           4 buffers,  0.058 ms
--
-- (project_id, id DESC) is the universal prefix of the list query: every variant
-- filters `project_id = ?` and orders by `id DESC` with a LIMIT, so this serves
-- the scan, the ordering and the early stop in one. The added filters Phase 9.4
-- introduces (status, target, date range, email delivery) ride along as filters
-- on top of it — measured sub-millisecond except for deliberately broad windows.
--
-- CONCURRENTLY because `notification` is on the send hot path: a plain CREATE
-- INDEX takes ACCESS EXCLUSIVE and would block every send for the duration of
-- the build. Follows the Phase 2 precedent (see
-- 20260712130000_add_medium_to_preference.sql).
--
-- Deliberately NOT added, having measured them (see agent-docs/overview.md,
-- "Phase 9.4 — deviations"): an index on (channel, topic, event) or created_at.
-- Each one taxes every INSERT on the send path to speed a filter combination
-- that already answers in ~12-45 ms.

-- +goose Up
-- +goose StatementBegin
CREATE INDEX CONCURRENTLY IF NOT EXISTS ix_notification_project_id
    ON notification(project_id, id DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX CONCURRENTLY IF EXISTS ix_notification_project_id;
-- +goose StatementEnd
