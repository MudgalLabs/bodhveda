-- +goose Up
-- +goose StatementBegin
-- Per-project email provider settings. Holds the customer's BYO provider
-- credentials (Resend in v1) plus the "from" identity emails are sent as.
--
-- The provider secret (Resend API key) is encrypted at rest exactly like an
-- api_key token: `secret` BYTEA (AES-GCM ciphertext) + `nonce` BYTEA. The
-- plaintext is never stored, logged, or returned — the console only ever sees a
-- masked hint. `provider` is a discriminator so more adapters can be added later
-- without a re-migration (only 'resend' is accepted in v1).
--
-- One row per project (project_id is the PK), created/updated via upsert.
CREATE TABLE IF NOT EXISTS project_email_settings (
        project_id      INT PRIMARY KEY REFERENCES project(id) ON DELETE CASCADE,
        provider        TEXT NOT NULL DEFAULT 'resend'
                        CHECK (provider IN ('resend')),
        secret          BYTEA NOT NULL,
        nonce           BYTEA NOT NULL,
        from_name       TEXT NOT NULL,
        from_address    TEXT NOT NULL,
        created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
        updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- DROP TABLE IF EXISTS project_email_settings;
-- +goose StatementEnd
