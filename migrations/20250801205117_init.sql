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
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- DROP TABLE IF EXISTS project;
-- DROP TABLE IF EXISTS api_key;
-- DROP TABLE IF EXISTS recipient;
-- DROP TABLE IF EXISTS sessions;
-- DROP TABLE IF EXISTS user_profile;
-- DROP TABLE IF EXISTS user_identity;
-- +goose StatementEnd
