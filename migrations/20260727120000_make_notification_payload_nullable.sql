-- An email-only direct send: a send that delivers on the email medium WITHOUT
-- creating an in-app inbox row.
--
-- The motivating case is a debounced email beside an instant in-app feed. A
-- product wants one in-app row per event AS IT HAPPENS, but ONE email covering
-- the last five minutes ("3 new messages on <thread>"). That is two sends with
-- different timing — an instant one for in-app, a later one for email. The later
-- one was inexpressible: `payload` was required, so every email send also wrote
-- an in-app row, and the debounced email dropped a duplicate into the feed five
-- minutes after the instant one.
--
-- The fix follows the rule agent-docs/overview.md already states — "sender intent
-- = presence of that medium's content block". `payload` was the sole exception:
-- required regardless of intent. Making it nullable makes that rule TRUE rather
-- than aspirational, and it needs no new field or flag. Absent `payload` ⇒ no
-- in-app delivery.
--
-- Note the NOT NULL was the ONLY thing enforcing payload's presence —
-- SendNotificationPayload.Validate() never checked it, so omitting `payload`
-- 500'd on a constraint violation rather than 400'ing. The service now validates
-- it properly: at least one content block (`payload` or `email`) must be present,
-- so an accidental omission is still rejected. What is no longer rejected is a
-- DELIBERATE email-only send.
--
-- The notification row still EXISTS for such a send — it is suppressed, not
-- missing — because notification_delivery.notification_id is NOT NULL REFERENCES
-- notification(id), the analytics target breakdown joins deliveries back through
-- it, and GET /notifications/{id} is how a caller reads an email outcome back
-- now that sends are fully async. It carries status `not_requested` and is
-- excluded from every recipient-facing read path.
--
-- No CHECK constraint guards notification.status (see
-- 20250902095838_add_subscription_and_usage.sql — it is TEXT NOT NULL DEFAULT
-- 'enqueued' and nothing more), so the new `not_requested` status needs no
-- migration of its own.
--
-- `broadcast.payload` is deliberately left NOT NULL by this migration. Broadcasts
-- are in-app only today (an `email` block on one is a 400), so a broadcast with
-- no payload has no medium at all. That changes when broadcast email lands; the
-- column is relaxed then, by that work, not speculatively here.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE notification
    ALTER COLUMN payload DROP NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Email-only sends leave payload NULL, and NOT NULL cannot be restored while any
-- exist. Backfill them to an empty object first: it loses the "in-app was never
-- requested" signal in the payload column, but `status = 'not_requested'` still
-- carries it, so the distinction survives the rollback.
UPDATE notification SET payload = '{}'::jsonb WHERE payload IS NULL;

ALTER TABLE notification
    ALTER COLUMN payload SET NOT NULL;
-- +goose StatementEnd
