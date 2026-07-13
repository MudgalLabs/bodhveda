-- +goose Up
-- +goose StatementBegin
-- Phase 5 (delivery status via provider webhooks): store the per-project webhook
-- signing secret. Resend signs webhooks via Svix; the signing secret is DISTINCT
-- from the Resend API key (a per-endpoint `whsec_...` secret the customer copies
-- from the Resend dashboard). It is encrypted at rest exactly like the provider
-- secret: `webhook_secret` BYTEA (AES-GCM ciphertext) + `webhook_nonce` BYTEA.
-- Nullable — a project may configure sending (Phase 4) before wiring webhooks.
ALTER TABLE project_email_settings
    ADD COLUMN IF NOT EXISTS webhook_secret BYTEA,
    ADD COLUMN IF NOT EXISTS webhook_nonce  BYTEA;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- ALTER TABLE project_email_settings
--     DROP COLUMN IF EXISTS webhook_secret,
--     DROP COLUMN IF EXISTS webhook_nonce;
-- +goose StatementEnd
