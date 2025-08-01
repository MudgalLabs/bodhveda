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

CREATE TABLE sessions (
        token   TEXT PRIMARY KEY,
	data    BYTEA NOT NULL,
	expiry  TIMESTAMPTZ NOT NULL
);

CREATE INDEX sessions_expiry_idx ON sessions (expiry);

CREATE TABLE project (
        id           SERIAL PRIMARY KEY,
        name         VARCHAR(255)   NOT NULL,
        user_id      INT NOT NULL REFERENCES user_identity(id),
        created_at   TIMESTAMPTZ    NOT NULL,
        updated_at   TIMESTAMPTZ NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS project;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS user_profile;
DROP TABLE IF EXISTS user_identity;
-- +goose StatementEnd
