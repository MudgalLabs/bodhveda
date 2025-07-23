-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS user_identity (
        id UUID PRIMARY KEY,
        email VARCHAR(255) NOT NULL UNIQUE,
        password_hash TEXT NOT NULL DEFAULT '',
        verified BOOLEAN NOT NULL,
        oauth_provider VARCHAR(32) NOT NULL,
        last_login_at TIMESTAMPTZ,
        created_at TIMESTAMPTZ NOT NULL,
        updated_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS user_profile (
        user_id UUID PRIMARY KEY UNIQUE REFERENCES user_identity(id),
        email VARCHAR(255) NOT NULL UNIQUE,
        name VARCHAR(255) NOT NULL,
        avatar_url TEXT,
        created_at TIMESTAMPTZ NOT NULL,
        updated_at TIMESTAMPTZ
);

CREATE TABLE sessions (
	token TEXT PRIMARY KEY,
	data BYTEA NOT NULL,
	expiry TIMESTAMPTZ NOT NULL
);

CREATE INDEX sessions_expiry_idx ON sessions (expiry);

CREATE TABLE broadcast (
    id              UUID PRIMARY KEY,
    project_id      UUID NOT NULL,
    payload         JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL
);

CREATE TABLE notification (
    id              UUID PRIMARY KEY,
    project_id      UUID NOT NULL,
    recipient       TEXT NOT NULL,
    broadcast_id    UUID REFERENCES broadcast(id) ON DELETE CASCADE,
    payload         JSONB NOT NULL,
    read_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL
);

-- Indexes for efficient querying.

-- All project & recipient scoped reads, writes, deletes.
CREATE INDEX notification_project_recipient_idx
  ON notification (project_id, recipient);

-- For analytics/metrics/admin lookups by project.
CREATE INDEX notification_project_created_at_idx
  ON notification (project_id, created_at DESC);

-- For broadcast lookups by project.
CREATE INDEX broadcast_project_id_idx
  ON broadcast (project_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE user_identity CASCADE;
DROP TABLE user_profile CASCADE;
DROP TABLE sessions CASCADE;
DROP TABLE IF EXISTS notification;
DROP TABLE IF EXISTS broadcast;
-- +goose StatementEnd
