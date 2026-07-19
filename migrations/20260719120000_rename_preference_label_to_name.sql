-- Renames the project-preference `label` column to `name` and adds an optional
-- `description`, so a catalog entry can carry both a human name
-- (e.g. "Marketing emails") and a longer blurb explaining what it covers
-- (e.g. "Receive notifications about new products, features, and more.").
--
-- `name` keeps label's old rule: it is required for project-level (catalog) rows
-- and forbidden on recipient-level rows. The existing CHECK constraint follows
-- the column across the RENAME automatically; we rename the constraint too so its
-- name stops lying. `description` is OPTIONAL (nullable) but, like name, only
-- belongs on project-level rows — a recipient row must never carry one.

-- +goose Up
ALTER TABLE preference RENAME COLUMN label TO name;

ALTER TABLE preference
    RENAME CONSTRAINT preference_label_for_project_only TO preference_name_for_project_only;

ALTER TABLE preference ADD COLUMN IF NOT EXISTS description VARCHAR(1024);

-- description is meaningful only for project-level (catalog) rows; recipient
-- rows must not carry one. It stays optional for project rows, so the check is
-- one-directional (unlike name's presence rule).
ALTER TABLE preference ADD CONSTRAINT preference_description_for_project_only CHECK (
    recipient_external_id IS NULL OR description IS NULL
);

-- +goose Down
ALTER TABLE preference DROP CONSTRAINT IF EXISTS preference_description_for_project_only;

ALTER TABLE preference DROP COLUMN IF EXISTS description;

ALTER TABLE preference
    RENAME CONSTRAINT preference_name_for_project_only TO preference_label_for_project_only;

ALTER TABLE preference RENAME COLUMN name TO label;
