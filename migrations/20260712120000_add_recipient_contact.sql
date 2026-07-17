-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS recipient_contact (
        id                      BIGSERIAL PRIMARY KEY,
        project_id              INT NOT NULL REFERENCES project(id) ON DELETE CASCADE,
        recipient_external_id   VARCHAR(255) NOT NULL,
        medium                  TEXT NOT NULL
                                CHECK (medium IN ('email', 'sms', 'web_push', 'mobile_push')),
        address                 TEXT NOT NULL,
        is_primary              BOOLEAN NOT NULL DEFAULT false,
        verified_at             TIMESTAMPTZ,
        created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
        updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),

        -- The same address can't be registered twice for a recipient+medium.
        UNIQUE (project_id, recipient_external_id, medium, address),

        -- Contacts hang off an existing recipient; deleting the recipient removes them.
        FOREIGN KEY (project_id, recipient_external_id)
            REFERENCES recipient(project_id, external_id) ON DELETE CASCADE
);

-- At most one primary contact per (recipient, medium). A second primary for the
-- same pair violates this partial unique index and surfaces as a 409.
-- This index also serves the "fetch the primary contact" lookup, so no separate
-- non-unique index is created — a plain index on the identical columns+predicate
-- would be pure write cost. (The old design doc's DDL listed both.)
CREATE UNIQUE INDEX IF NOT EXISTS ux_recipient_contact_one_primary
    ON recipient_contact(project_id, recipient_external_id, medium)
    WHERE is_primary = true;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- DROP TABLE IF EXISTS recipient_contact;
-- +goose StatementEnd
