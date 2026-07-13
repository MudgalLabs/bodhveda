-- +goose Up
-- +goose StatementBegin
-- Per-(notification, medium) delivery record.
--
-- v1 scope: this table is written for EMAIL (and future non-in_app mediums)
-- ONLY. The in-app inbox is NOT migrated onto delivery rows — its status /
-- read_at / opened_at stay on the `notification` row (see agent-docs/overview.md,
-- "notification_delivery for email (non-in_app) only in v1"). So the old design
-- doc's in_app backfill / dual-write / column-drop is deliberately NOT done here.
--
-- One row per (notification_id, medium). `address_snapshot` captures the contact
-- address at enqueue time so later contact edits don't rewrite history.
-- `provider_message_id` is the provider's id (Resend) used to correlate inbound
-- webhooks in Phase 5. The full status/timestamp column set is created now
-- (delivered/bounced/complained/opened/clicked) so Phase 5 webhooks need no
-- re-migration; v1 only ever sets pending/sent/failed/muted/no_contact.
CREATE TABLE IF NOT EXISTS notification_delivery (
        id                      BIGSERIAL PRIMARY KEY,
        notification_id         INT NOT NULL REFERENCES notification(id) ON DELETE CASCADE,
        project_id              INT NOT NULL REFERENCES project(id) ON DELETE CASCADE,
        recipient_external_id   VARCHAR(255) NOT NULL,

        medium                  TEXT NOT NULL
                                CHECK (medium IN ('in_app','email','sms','web_push','mobile_push')),
        contact_id              BIGINT REFERENCES recipient_contact(id) ON DELETE SET NULL,
        address_snapshot        TEXT,

        status                  TEXT NOT NULL
                                CHECK (status IN (
                                    'pending','sending','sent','delivered','bounced','complained',
                                    'failed','muted','no_contact','suppressed','quota_exceeded','rejected'
                                )),
        provider                TEXT,
        provider_message_id     TEXT,
        provider_response       JSONB,
        failure_reason          TEXT,
        attempt                 INT NOT NULL DEFAULT 0,

        sent_at                 TIMESTAMPTZ,
        delivered_at            TIMESTAMPTZ,
        bounced_at              TIMESTAMPTZ,
        complained_at           TIMESTAMPTZ,
        opened_at               TIMESTAMPTZ,
        clicked_at              TIMESTAMPTZ,
        read_at                 TIMESTAMPTZ,

        created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
        updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),

        UNIQUE (notification_id, medium)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS ix_nd_notification ON notification_delivery(notification_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS ix_nd_project_recipient
    ON notification_delivery(project_id, recipient_external_id, created_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
-- Correlates inbound provider webhooks (Phase 5) back to the delivery row.
CREATE UNIQUE INDEX IF NOT EXISTS ux_nd_provider_message
    ON notification_delivery(medium, provider_message_id)
    WHERE provider_message_id IS NOT NULL;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS ix_nd_email_status_time
    ON notification_delivery(project_id, created_at DESC)
    WHERE medium = 'email';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- DROP TABLE IF EXISTS notification_delivery;
-- +goose StatementEnd
