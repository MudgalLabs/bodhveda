-- Mandatory catalog entries — the escape hatch that makes strict targets shippable.
--
-- Strict targets make the catalog a GATEWAY: a target with no catalog row is a
-- 400 at send time, not a silent `muted`. That is the point — an uncataloged
-- in-app notification is un-mutable, and a typo'd target is today
-- indistinguishable from a real send.
--
-- But if the catalog is both the gate AND the preference surface, then
-- everything sendable becomes opt-out-able — including password resets, security
-- alerts and billing failures. Products cannot allow that, and the usual
-- workaround (send it outside the platform) defeats the platform.
--
-- `mandatory` resolves the contradiction: the target IS cataloged, therefore
-- sendable, but the recipient-level toggle is refused and the resolution cascade
-- ignores any recipient row for it. Cataloged, not negotiable.
--
-- ⚠️ This is why the concrete blocker measured on prod (2026-08-01) is not a
-- reason to keep strict targets behind a flag:
--
--     proj  target                   sent   cataloged
--        6  marketing/none/welcome    796       NO
--
-- Arthveda's welcome message is the single largest target in production and is
-- deliberately uncataloged — its seeder says a toggle for a notification that
-- only ever fires once is noise. That judgement is correct, and `mandatory` is
-- how it stays correct once the gate is on: Arthveda catalogs the welcome as
-- mandatory, the gate passes, and no toggle is ever rendered.
--
-- ⚠️ mandatory only means anything on a PROJECT-level row (recipient NULL). A
-- recipient row is the thing being overridden; a "mandatory" recipient row is a
-- contradiction. The partial index enforces that rather than trusting callers.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE preference
    ADD COLUMN IF NOT EXISTS mandatory BOOLEAN NOT NULL DEFAULT false;

-- A recipient-level row must never claim to be mandatory.
ALTER TABLE preference
    DROP CONSTRAINT IF EXISTS ck_preference_mandatory_is_project_level;

ALTER TABLE preference
    ADD CONSTRAINT ck_preference_mandatory_is_project_level
    CHECK (NOT mandatory OR recipient_external_id IS NULL);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE preference
    DROP CONSTRAINT IF EXISTS ck_preference_mandatory_is_project_level;

ALTER TABLE preference
    DROP COLUMN IF EXISTS mandatory;
-- +goose StatementEnd
