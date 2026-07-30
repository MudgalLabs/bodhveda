-- +goose NO TRANSACTION

-- The broadcast delivery tree's per-status rollup is
--
--     SELECT status, COUNT(*) FROM notification WHERE broadcast_id = $1 GROUP BY status
--
-- and `notification` had NO index on broadcast_id at all, so that was a full
-- sequential scan of every notification in the install to summarise ONE
-- broadcast. That is the opposite of what the tree needs: a broadcast's rows are
-- a tiny, highly selective slice of the table.
--
-- ⚠️ `notification` is on the send hot path (every direct send and every
-- broadcast batch inserts into it), so every index here taxes writes — the
-- standing rule in agent-docs/overview.md is to measure before adding one. This
-- one is justified twice over: the rollup is unservable without it, and the
-- index is PARTIAL on `broadcast_id IS NOT NULL`, so direct sends (the bulk of
-- inserts, which have a NULL broadcast_id) never touch it at all. The write cost
-- falls only on broadcast fan-out, which is the thing being made queryable.
--
-- INCLUDE (status) makes this covering for the rollup: the aggregate is answered
-- from the index without visiting the heap.
--
-- ⚠️ Coordinate before adding another: overview.md §Open/next plans a partial
-- UNIQUE index on (broadcast_id, recipient_external_id) to fix the
-- BroadcastDeliveryProcessor idempotency bug. That one would serve
-- `WHERE broadcast_id = $1` as a prefix too — when it lands, re-measure and drop
-- whichever of the two is redundant rather than carrying both on a hot table.
--
-- CONCURRENTLY (hence NO TRANSACTION) so building it never blocks sends.

-- +goose Up
-- +goose StatementBegin
CREATE INDEX CONCURRENTLY IF NOT EXISTS ix_notification_broadcast
    ON notification (broadcast_id) INCLUDE (status)
    WHERE broadcast_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- DROP INDEX CONCURRENTLY IF EXISTS ix_notification_broadcast;
-- +goose StatementEnd
