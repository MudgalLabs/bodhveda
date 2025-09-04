-- +goose Up
-- +goose StatementBegin
-- Core schema for multi-metric usage metering

-- Plans (free, pro, etc.)
-- CREATE TABLE IF NOT EXISTS plan (
--     id            SERIAL PRIMARY KEY,
--     name          TEXT NOT NULL UNIQUE,            -- e.g., 'free', 'pro'
--     description   TEXT,
--     created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
-- );

-- Per-plan entitlements (metric + limit per period)
-- Example metric values: 'notifications', 'storage_bytes', 'webhooks'
-- CREATE TABLE IF NOT EXISTS plan_entitlement (
--     id          SERIAL PRIMARY KEY,
--     plan_id     INT NOT NULL REFERENCES plan(id),
--     metric      TEXT NOT NULL,
--     "limit"     BIGINT,                              -- NULL = unlimited
--     period      INTERVAL NOT NULL,                   -- e.g. '30 days'
--     created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
--     CONSTRAINT  ck_plan_entitlement_limit_nonneg CHECK ("limit" IS NULL OR "limit" >= 0)
-- );

-- Prevent duplicate metric definitions per plan
-- CREATE UNIQUE INDEX IF NOT EXISTS ux_plan_entitlement_plan_metric
--   ON plan_entitlement(plan_id, metric);

-- Active subscription for a user (anchors the usage window)
CREATE TABLE IF NOT EXISTS user_subscription (
    user_id                 INT PRIMARY KEY UNIQUE REFERENCES user_profile(user_id),
    plan_id                 TEXT NOT NULL, -- e.g., 'free', 'pro'
    current_period_start    TIMESTAMPTZ NOT NULL,
    current_period_end      TIMESTAMPTZ NOT NULL,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Append-only usage log (audit trail)
CREATE TABLE IF NOT EXISTS usage_log (
    id              BIGSERIAL PRIMARY KEY,
    project_id      INT NOT NULL REFERENCES project(id),
    metric          TEXT NOT NULL,
    amount          BIGINT NOT NULL,
    used_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT ck_usage_amount_pos CHECK (amount > 0)
);

-- Hot-path index for time-bounded queries
CREATE INDEX IF NOT EXISTS ix_usage_project_metric_time
  ON usage_log(project_id, metric, used_at);

-- Fast O(1) enforcement cache; can be recomputed from logs
CREATE TABLE IF NOT EXISTS usage_aggregate (
    project_id      INT REFERENCES project(id),
    metric          TEXT NOT NULL,
    period_start    TIMESTAMPTZ NOT NULL,
    period_end      TIMESTAMPTZ NOT NULL,
    used            BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (project_id, metric, period_start),
    CONSTRAINT ck_agg_used_nonneg CHECK (used >= 0)
);

ALTER TABLE broadcast_batch
    DROP CONSTRAINT IF EXISTS broadcast_batch_status_check;

ALTER TABLE broadcast
    ADD COLUMN status TEXT;

-- Backfill broadcast.status
UPDATE broadcast
SET status = CASE
    WHEN completed_at IS NOT NULL THEN 'completed'
    ELSE 'enqueued'
END
WHERE status IS NULL;

-- Lock down broadcast.status as NOT NULL with default
ALTER TABLE broadcast
    ALTER COLUMN status SET NOT NULL,
    ALTER COLUMN status SET DEFAULT 'enqueued';


ALTER TABLE notification
    ADD COLUMN status TEXT,
    ADD COLUMN completed_at TIMESTAMPTZ;

-- Backfill notification.completed_at from created_at
UPDATE notification
SET completed_at = created_at
WHERE completed_at IS NULL;

-- Backfill notification.status
UPDATE notification
SET status = 'delivered'
WHERE status IS NULL;

-- Lock down notification.status and completed_at
ALTER TABLE notification
    ALTER COLUMN status SET NOT NULL,
    ALTER COLUMN status SET DEFAULT 'enqueued';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE broadcast
    DROP COLUMN IF EXISTS status;

ALTER TABLE notification
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS completed_at;

ALTER TABLE broadcast_batch
    ADD CONSTRAINT broadcast_batch_status_check
    CHECK (status IN ('pending', 'completed', 'failed'));

DROP TABLE IF EXISTS usage_aggregate;
DROP INDEX IF EXISTS ix_usage_user_metric_time;
DROP TABLE IF EXISTS usage_log;
DROP TABLE IF EXISTS user_subscription;
-- DROP INDEX IF EXISTS ux_plan_entitlement_plan_metric;
-- DROP TABLE IF EXISTS plan_entitlement;
-- DROP TABLE IF EXISTS plan;
-- +goose StatementEnd
