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
        project_id      INT NOT NULL REFERENCES project(id) ON DELETE CASCADE,
        external_id     VARCHAR(255) NOT NULL,
        name            VARCHAR(255) NOT NULL DEFAULT '',
        created_at      TIMESTAMPTZ NOT NULL,
        updated_at      TIMESTAMPTZ NOT NULL,

        UNIQUE (project_id, external_id)
);

CREATE TABLE IF NOT EXISTS preference (
        id                      SERIAL PRIMARY KEY,
        project_id              INT NOT NULL REFERENCES project(id) ON DELETE CASCADE,
        recipient_external_id   VARCHAR(255),
        channel                 TEXT NOT NULL,
        topic                   TEXT NOT NULL,
        event                   TEXT NOT NULL,
        label                   VARCHAR(255),
        enabled                 BOOLEAN NOT NULL,
        created_at              TIMESTAMPTZ NOT NULL,
        updated_at              TIMESTAMPTZ NOT NULL,

        -- Enforce that label is only allowed for project preferences
        CONSTRAINT preference_label_for_project_only CHECK (
            (recipient_external_id IS NULL AND label IS NOT NULL)
            OR (recipient_external_id IS NOT NULL AND label IS NULL)
        )
);

-- Enforce uniqueness for recipient-level preferences
CREATE UNIQUE INDEX IF NOT EXISTS recipient_pref_unique
ON preference(project_id, recipient_external_id, channel, topic, event)
WHERE recipient_external_id IS NOT NULL;

-- Enforce uniqueness for project-level preferences
CREATE UNIQUE INDEX IF NOT EXISTS project_pref_unique
ON preference(project_id, channel, topic, event)
WHERE recipient_external_id IS NULL;

CREATE TABLE IF NOT EXISTS broadcast (
        id              SERIAL PRIMARY KEY,
        project_id      INT NOT NULL REFERENCES project(id) ON DELETE CASCADE,
        payload         JSONB NOT NULL,
        channel         TEXT NOT NULL,
        topic           TEXT NOT NULL,
        event           TEXT NOT NULL,
        completed_at    TIMESTAMPTZ,
        created_at      TIMESTAMPTZ NOT NULL,
        updated_at      TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS notification (
        id                      SERIAL PRIMARY KEY,
        project_id              INT NOT NULL REFERENCES project(id) ON DELETE CASCADE,
        recipient_external_id   VARCHAR(255) NOT NULL,
        payload                 JSONB NOT NULL,
        broadcast_id            INT REFERENCES broadcast(id) ON DELETE CASCADE,
        channel                 TEXT NOT NULL,
        topic                   TEXT NOT NULL,
        event                   TEXT NOT NULL,
        created_at              TIMESTAMPTZ NOT NULL,
        updated_at              TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS broadcast_batch (
        id              SERIAL PRIMARY KEY,
        broadcast_id    INT NOT NULL REFERENCES broadcast(id) ON DELETE CASCADE,
        recipients      INT NOT NULL DEFAULT 0,
        status          TEXT NOT NULL CHECK (status IN ('pending', 'completed', 'failed')),
        attempt         INT NOT NULL DEFAULT 0,
        duration        INT NOT NULL DEFAULT 0,
        created_at      TIMESTAMPTZ NOT NULL,
        updated_at      TIMESTAMPTZ NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- DROP TABLE IF EXISTS api_key;
-- DROP TABLE IF EXISTS recipient;
-- DROP TABLE IF EXISTS preference;
-- DROP TABLE IF EXISTS broadcast_batch;
-- DROP TABLE IF EXISTS broadcast;
-- DROP TABLE IF EXISTS notification;
-- DROP TABLE IF EXISTS project;
-- DROP TABLE IF EXISTS sessions;
-- DROP TABLE IF EXISTS user_profile;
-- DROP TABLE IF EXISTS user_identity;
-- +goose StatementEnd
