-- Broadcast email: content on the broadcast, and a per-project recipient cap.
--
-- Until now `email` on a broadcast was a hard 400 and broadcasts were in-app
-- only. The content has to live on the `broadcast` row rather than only in the
-- Asynq payload for the same reason broadcast_batch gained recipient ids
-- (20260731120000): the database must be the durable record of what is being
-- sent, or a retry cannot reconstruct it.
--
-- ⚠️ `payload` stays NOT NULL — a broadcast still always fans out in-app, and
-- email is additive. Email-only BROADCASTS are deliberately not supported in this
-- first cut, unlike email-only direct sends (Phase 10). A broadcast is the high
-- blast-radius path, and "this also went to their inbox" is the property that
-- makes a mistaken email recoverable/visible.
--
-- max_broadcast_recipients_for_email is a SAFETY RAIL, not a billing limit. It
-- exists so a mis-targeted broadcast cannot become an accidental marketing blast
-- to the whole recipient list. It caps the EMAIL medium only: in-app fan-out is
-- unaffected and still reaches everyone eligible.
--
-- ⚠️ Exceeding it BLOCKS email entirely (recorded as email_blocked_reason
-- 'recipient_cap_exceeded'), it does not truncate to the first N. Truncating
-- would mail an arbitrary subset chosen by query order — worse than not mailing,
-- because it looks like it worked.
--
-- Default 100: low enough that reaching it is a deliberate act, and every current
-- consumer (Arthveda, Grahak, Resurface) is well under it.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE broadcast
    ADD COLUMN IF NOT EXISTS email_subject TEXT,
    ADD COLUMN IF NOT EXISTS email_html    TEXT,
    ADD COLUMN IF NOT EXISTS email_text    TEXT,
    -- Frozen at prepare time, like the in-app audience columns. NULL means email
    -- was not requested; 0 means it was requested and nobody was eligible.
    ADD COLUMN IF NOT EXISTS email_eligible_recipients INT,
    -- Why email did not fan out, when it did not. NULL when email ran (or was
    -- never requested).
    ADD COLUMN IF NOT EXISTS email_blocked_reason TEXT;

ALTER TABLE project_email_settings
    ADD COLUMN IF NOT EXISTS max_broadcast_recipients_for_email INT NOT NULL DEFAULT 100;

ALTER TABLE project_email_settings
    DROP CONSTRAINT IF EXISTS ck_max_broadcast_recipients_for_email_pos;

ALTER TABLE project_email_settings
    ADD CONSTRAINT ck_max_broadcast_recipients_for_email_pos
    CHECK (max_broadcast_recipients_for_email >= 0);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE project_email_settings
    DROP CONSTRAINT IF EXISTS ck_max_broadcast_recipients_for_email_pos,
    DROP COLUMN IF EXISTS max_broadcast_recipients_for_email;

ALTER TABLE broadcast
    DROP COLUMN IF EXISTS email_blocked_reason,
    DROP COLUMN IF EXISTS email_eligible_recipients,
    DROP COLUMN IF EXISTS email_text,
    DROP COLUMN IF EXISTS email_html,
    DROP COLUMN IF EXISTS email_subject;
-- +goose StatementEnd
