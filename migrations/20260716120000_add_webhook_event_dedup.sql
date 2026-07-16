-- +goose Up
-- +goose StatementBegin
-- Idempotency ledger for inbound provider webhooks (#8). Providers (Resend via
-- Svix) retry webhooks, and Svix stamps every message with a stable per-event id
-- (the `svix-id` header) that is identical across all retries of that event. We
-- record it here and dedup on it so a replay is acked without re-processing —
-- which otherwise re-appends to notification_delivery.provider_response and re-runs
-- complaint suppression. `provider_event_id` is globally unique per provider (svix
-- ids are UUIDs), so the unique key is (provider, provider_event_id). project_id is
-- kept for scoping/audit and cascades on project delete.
CREATE TABLE IF NOT EXISTS webhook_event (
    id                BIGSERIAL PRIMARY KEY,
    project_id        INT NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    provider          TEXT NOT NULL,
    provider_event_id TEXT NOT NULL,
    received_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (provider, provider_event_id)
);
-- +goose StatementEnd

-- +goose StatementBegin
-- Supports the retention cleanup job (deletes rows older than the window).
CREATE INDEX IF NOT EXISTS ix_webhook_event_received_at ON webhook_event(received_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- DROP TABLE IF EXISTS webhook_event;
-- +goose StatementEnd
