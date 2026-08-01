-- Strict targets becomes a per-project setting, default OFF.
--
-- Strict targets shipped unconditional: any send naming a (target, medium) with
-- no catalog row was a 400. That is the right END STATE, but it is a maturity
-- setting, not a correctness rule, and shipping it as the only mode gets the
-- learning order backwards.
--
-- With the gate always on, a new user's very first targeted send fails with
-- "create a project preference for it before sending" — at the exact moment they
-- have built nothing and do not yet know what the catalog is. That is a wall,
-- not a guardrail. The intended order is: send untargeted -> add a target ->
-- discover preferences -> seed them -> harden by turning this on once the
-- catalog is stable.
--
-- ⚠️ DEFAULT false, AND THAT INCLUDES NEW PROJECTS. An earlier design
-- (agent-docs/strict-targets-design.md §3.2) proposed defaulting new projects to
-- true and only grandfathering existing ones. That still walls off the first
-- targeted send for every new user, which is the whole problem. The column
-- default does the backfill for existing rows for free.
--
-- The catalog keeps every other job it has while this is off: it is still the
-- default for a (target, medium), still the surface a settings screen renders,
-- still what `mandatory` hangs off. Off only means an uncataloged target is not
-- REJECTED — for email it still resolves to not-delivered, because the medium
-- default for anything other than in_app is "do not send".

-- +goose Up
-- +goose StatementBegin
ALTER TABLE project
    ADD COLUMN IF NOT EXISTS strict_targets BOOLEAN NOT NULL DEFAULT false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE project
    DROP COLUMN IF EXISTS strict_targets;
-- +goose StatementEnd
