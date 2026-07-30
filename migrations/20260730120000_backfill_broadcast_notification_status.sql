-- +goose Up
-- +goose StatementBegin
-- Backfill for the broadcast-notification status bug (fixed in
-- BroadcastDeliveryProcessor on 2026-07-30).
--
-- `entity.NewNotification` defaults to `enqueued`, which is correct for a DIRECT
-- send — notification:delivery still has to resolve preferences and billing.
-- A broadcast has no such second step: prepare_batches already resolved
-- eligibility and consumed usage, so the row is in the inbox the moment it is
-- written. But BroadcastDeliveryProcessor never updated the status, so EVERY
-- broadcast notification ever created sat permanently in `enqueued` — the only
-- non-terminal NotificationStatus.
--
-- It stayed invisible because `recipientFeedVisible` treats `enqueued` as
-- visible, so recipient inboxes always looked correct. It only became load-bearing
-- when two things started reading the status:
--
--   * the console delivery tree reported every broadcast as 100% pending, and
--   * internal/monitor's stuck_sends check counted all of them as stalled sends
--     and would have alerted every 30 minutes, forever.
--
-- These rows ARE delivered — they have been in recipients' inboxes since they
-- were written — so this records the truth rather than changing an outcome.
--
-- ⚠️ Scoped to broadcast rows ONLY (broadcast_id IS NOT NULL). A DIRECT
-- notification sitting in `enqueued` is a genuinely stalled send and must keep
-- showing as one — that is precisely the signal stuck_sends exists to catch, and
-- backfilling those would erase real evidence.
--
-- completed_at is set from updated_at (when the row was last written) rather than
-- now(), so the history is not rewritten to look like everything completed at
-- migration time.
UPDATE notification
SET status = 'delivered',
    completed_at = COALESCE(completed_at, updated_at)
WHERE broadcast_id IS NOT NULL
  AND status = 'enqueued';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Deliberately NOT reversible: the pre-migration state was wrong, and there is
-- no way to tell which rows were `enqueued` because of this bug from any that
-- were legitimately mid-flight.
-- +goose StatementEnd
