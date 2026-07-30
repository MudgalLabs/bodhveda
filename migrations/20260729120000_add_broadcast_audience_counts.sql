-- +goose Up
-- +goose StatementBegin
-- Audience counts for the broadcast delivery tree, FROZEN at fan-out time.
--
-- Why stored and not computed on read: the console tree wants to show
-- "1,000 recipients → 600 excluded → 400 eligible", and the only place those
-- numbers are true is the moment prepare_batches resolved the audience.
-- Recomputing `total - eligible` later against a live `COUNT(*) FROM recipient`
-- is wrong as soon as anyone signs up or leaves, and can go NEGATIVE (recipients
-- deleted after the send). A plausible-looking wrong number is worse than none.
--
-- ⚠️ The two exclusion columns are NOT the same thing, and collapsing them into
-- one "excluded" number would hide the most common broadcast mistake. Eligibility
-- (see PreferenceRepo.ListEligibleRecipientExtIDsForBroadcast) is:
--
--     recipient pref enabled = true
--     OR (no recipient pref AND project catalog pref enabled = true)
--
-- so a recipient is excluded for one of two completely different reasons:
--
--   excluded_disabled       the RECIPIENT opted out (their own pref is false).
--                           Normal, healthy, nothing to fix.
--   excluded_not_cataloged  the PROJECT never offered this target — no catalog
--                           row, or a disabled one. This is a config mistake, and
--                           it is why a broadcast silently reaches nobody.
--
-- These are deliberately the same two names the DIRECT send path already writes
-- to notification_delivery.failure_reason ('preference_disabled' /
-- 'not_cataloged'), so the console can use one vocabulary for both send kinds.
--
-- All four are NULLABLE on purpose: broadcasts sent before this migration have no
-- recorded audience, and the tree must render "not recorded" for them rather than
-- an invented 0.
ALTER TABLE broadcast
    ADD COLUMN IF NOT EXISTS total_recipients        INT,
    ADD COLUMN IF NOT EXISTS eligible_recipients     INT,
    ADD COLUMN IF NOT EXISTS excluded_disabled       INT,
    ADD COLUMN IF NOT EXISTS excluded_not_cataloged  INT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- ALTER TABLE broadcast
--     DROP COLUMN IF EXISTS total_recipients,
--     DROP COLUMN IF EXISTS eligible_recipients,
--     DROP COLUMN IF EXISTS excluded_disabled,
--     DROP COLUMN IF EXISTS excluded_not_cataloged;
-- +goose StatementEnd
