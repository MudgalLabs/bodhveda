-- Make a broadcast batch self-describing.
--
-- `broadcast_batch` recorded only `recipients` — a COUNT. The mapping from a
-- batch to the recipients it must deliver to existed in exactly one place: the
-- Asynq task payload. That made preparation unresumable. If
-- PrepareBroadcastBatchesProcessor committed its batch rows and then failed
-- partway through enqueuing their tasks, the un-enqueued batches could never be
-- recovered, because nothing in the database knew who they were for.
--
-- Storing the ids on the batch makes the DATABASE the durable record of what
-- must be delivered and the queue merely the trigger — so a retry can re-enqueue
-- exactly the outstanding batches instead of re-deriving them (a fresh
-- eligibility query would slice differently the moment a recipient is added or
-- removed, silently delivering to the wrong set).
--
-- NULLable on purpose: batches created before this migration have no id list and
-- are NOT resumable. That is a fact about them, and the processor treats NULL as
-- "cannot resume, log and leave alone" rather than pretending an empty audience.
-- No backfill exists because the information was never recorded — inventing it
-- is not possible.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE broadcast_batch
    ADD COLUMN IF NOT EXISTS recipient_external_ids TEXT[];
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE broadcast_batch
    DROP COLUMN IF EXISTS recipient_external_ids;
-- +goose StatementEnd
