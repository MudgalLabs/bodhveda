-- One notification per (broadcast, recipient), enforced by the database.
--
-- Broadcast delivery is idempotent under Asynq retries today because
-- BroadcastDeliveryProcessor takes a FOR UPDATE lock on the batch row and
-- returns early when it already reads `success`. That guard is correct and
-- tested — but it is a guarantee held in application code, and the thing it
-- protects is "did this person get mailed twice".
--
-- This index makes it a guarantee held by Postgres. If the batch guard is ever
-- bypassed (a new code path that fans out without taking the lock, a bug in the
-- status transition, a manual re-enqueue), the duplicate INSERT now fails loudly
-- and the task retries into the guard, instead of quietly writing a second
-- notification and a second email.
--
-- ⚠️ Verified safe before writing: production had ZERO duplicate
-- (broadcast_id, recipient_external_id) pairs at the time of this migration, so
-- there is nothing to clean up first and index creation cannot fail on existing
-- data. The table is small (~1k rows), so a plain CREATE INDEX is instant and
-- CONCURRENTLY is unnecessary.
--
-- PARTIAL on `broadcast_id IS NOT NULL`: direct sends are the overwhelming
-- majority of inserts and have a NULL broadcast_id, so they never touch this
-- index at all — no write cost on the hot path.
--
-- ⚠️ ix_notification_broadcast is deliberately KEPT. This index covers
-- `WHERE broadcast_id = $1` as a prefix, so it looks redundant, but the existing
-- one carries INCLUDE (status) which makes the per-broadcast status rollup an
-- index-only scan. Dropping it would trade a duplicate-prevention win for a
-- regression on the query the delivery tree runs.

-- +goose Up
-- +goose StatementBegin
CREATE UNIQUE INDEX IF NOT EXISTS ux_notification_broadcast_recipient
    ON notification (broadcast_id, recipient_external_id)
    WHERE broadcast_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS ux_notification_broadcast_recipient;
-- +goose StatementEnd
