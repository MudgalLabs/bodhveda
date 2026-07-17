-- +goose NO TRANSACTION

-- Phase 9.5 (console analytics) aggregates `notification` by day/status/target
-- over a DATE RANGE. That query shape is the exact opposite of the list Phase
-- 9.4 indexed, and the index 9.4 added cannot serve it:
--
--   - The list is `WHERE project_id=? ORDER BY id DESC LIMIT n` — the id-DESC
--     ordering + LIMIT ride ix_notification_project_id (project_id, id DESC),
--     which is why 9.4 explicitly DECLINED a created_at index (its added filters
--     were narrow, LIMIT-bounded riders on that prefix).
--   - Analytics is a FULL aggregate (`GROUP BY day` / `GROUP BY target`) with NO
--     LIMIT and its only selective predicate on `created_at`. A (project_id, id)
--     index can't range-seek created_at, so without a created_at index every
--     analytics query is a Parallel Seq Scan of the WHOLE (multi-tenant)
--     notification table — and the date-range control provides ZERO work
--     reduction, since the scan reads every row regardless of the window.
--
-- Measured on 350k notifications across two projects (Phase 9.5), project 74
-- (200k rows), last-30-days window:
--                                  before (seq scan)   after this index
--   in-app day series (Q1)         5095 buf / 38.8 ms  817 buf / 19.6 ms
--   target volumes    (Q2)         5040 buf / 20.4 ms  817 buf /  7.7 ms
-- The buffer count drops ~6x and, unlike time, the seq-scan cost scales with the
-- ENTIRE table across all tenants while the index scan scales only with the
-- queried project's window — so the gap widens without bound as the platform
-- grows. This is the same unbounded-scan class of bug 9.4 fixed for the list,
-- one query shape over.
--
-- (project_id, created_at) is the universal prefix of every analytics aggregate:
-- all filter `project_id = ?` and range `created_at`. It is a SECOND
-- project-prefixed index on notification (the list keeps (project_id, id DESC));
-- they serve two distinct, first-class read patterns and neither can range-seek
-- the other's second column.
--
-- CONCURRENTLY because `notification` is on the send hot path: a plain CREATE
-- INDEX takes ACCESS EXCLUSIVE and would block every send for the build (the
-- Phase 2 / Phase 9.4 precedent).
--
-- Deliberately NOT added, having measured them (see agent-docs/overview.md,
-- "Phase 9.5 — deviations"):
--   - An index on (channel, topic, event): the target GROUP BY runs over the
--     already-windowed row set (HashAggregate, ~8 ms), so it buys nothing and
--     taxes every INSERT.
--   - Anything for the email per-target join (Q4): its plan self-corrects — a
--     hash join while notification is small, and the planner flips to a bounded
--     nested-loop PK lookup (~one lookup per in-window email delivery, not per
--     table row) once notification is large. The email side is already served by
--     ix_nd_email_status_time. No new index, no query rewrite.

-- +goose Up
-- +goose StatementBegin
CREATE INDEX CONCURRENTLY IF NOT EXISTS ix_notification_project_created
    ON notification(project_id, created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX CONCURRENTLY IF EXISTS ix_notification_project_created;
-- +goose StatementEnd
