-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS user_identity (
        id              SERIAL PRIMARY KEY,
        email           VARCHAR(255) NOT NULL UNIQUE,
        password_hash   TEXT NOT NULL DEFAULT '',
        verified        BOOLEAN NOT NULL,
        oauth_provider  VARCHAR(32) NOT NULL,
        last_login_at   TIMESTAMPTZ,
        created_at      TIMESTAMPTZ NOT NULL,
        updated_at      TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS user_profile (
        user_id         INT PRIMARY KEY UNIQUE REFERENCES user_identity(id),
        email           VARCHAR(255) NOT NULL UNIQUE,
        name            VARCHAR(255) NOT NULL,
        avatar_url      TEXT,
        created_at      TIMESTAMPTZ NOT NULL,
        updated_at      TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
        token   TEXT PRIMARY KEY,
	data    BYTEA NOT NULL,
	expiry  TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON sessions (expiry);

CREATE TABLE IF NOT EXISTS project (
        id           SERIAL PRIMARY KEY,
        name         VARCHAR(255) NOT NULL,
        user_id      INT NOT NULL REFERENCES user_identity(id),
        created_at   TIMESTAMPTZ NOT NULL,
        updated_at   TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS api_key (
        id              SERIAL PRIMARY KEY,
        name            VARCHAR(255) NOT NULL,
        token           BYTEA NOT NULL,
        nonce           BYTEA NOT NULL,
        token_hash      VARCHAR(255) NOT NULL,
        scope           VARCHAR(63) NOT NULL CHECK (scope IN ('full', 'recipient')),
        project_id      INT NOT NULL REFERENCES project(id) ON DELETE CASCADE,
        user_id         INT NOT NULL REFERENCES user_identity(id) ON DELETE CASCADE,
        created_at      TIMESTAMPTZ NOT NULL,
        updated_at      TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS recipient (
        id              SERIAL PRIMARY KEY,
        external_id     VARCHAR(255) NOT NULL,
        name            VARCHAR(255) NOT NULL DEFAULT '',
        project_id      INT NOT NULL REFERENCES project(id) ON DELETE CASCADE,
        created_at      TIMESTAMPTZ NOT NULL,
        updated_at      TIMESTAMPTZ NOT NULL,

        UNIQUE (project_id, external_id)
);

CREATE TABLE IF NOT EXISTS preference (
        id              SERIAL PRIMARY KEY,
        project_id      INT REFERENCES project(id) ON DELETE CASCADE,
        recipient_id    INT REFERENCES recipient(id) ON DELETE CASCADE,
        channel         TEXT NOT NULL,
        topic           TEXT NOT NULL,
        event           TEXT NOT NULL,
        label           VARCHAR(255),
        enabled         BOOLEAN NOT NULL,
        created_at      TIMESTAMPTZ NOT NULL,
        updated_at      TIMESTAMPTZ NOT NULL,

        -- Enforce mutual exclusivity: must be either project OR recipient preference.
        CHECK (
                (recipient_id IS NULL AND project_id IS NOT NULL)
                OR (recipient_id IS NOT NULL AND project_id IS NOT NULL)
        ),

        -- Enforce that label is only allowed for project preferences
        CHECK (
                (recipient_id IS NULL AND label IS NOT NULL)
                OR (recipient_id IS NOT NULL AND label IS NULL)
        )
);

-- Unique for project preferences.
CREATE UNIQUE INDEX unique_project_preference
ON preference (project_id, channel, topic, event)
WHERE recipient_id IS NULL;

-- Unique for recipient preferences.
CREATE UNIQUE INDEX unique_recipient_preference
ON preference (project_id, recipient_id, channel, topic, event)
WHERE recipient_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- DROP TABLE IF EXISTS api_key;
-- DROP TABLE IF EXISTS recipient;
-- DROP TABLE IF EXISTS preference;
-- DROP TABLE IF EXISTS project;
-- DROP TABLE IF EXISTS sessions;
-- DROP TABLE IF EXISTS user_profile;
-- DROP TABLE IF EXISTS user_identity;
-- +goose StatementEnd
