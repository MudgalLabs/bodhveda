# Multi-Medium Notification Delivery

## Progress

Tick each phase when its PR lands on `main`. Keep notes below the checklist for in-flight gotchas.

- [ ] **Out-of-band**: AWS SES production-access ticket opened
- [ ] **Out-of-band**: Free-plan `emails_sent` entitlement decided
- [ ] Phase 1 — `preference.medium` + index rebuild
- [ ] Phase 2 — Contacts table + API
- [ ] Phase 3 — Project email config + SES verification
- [ ] Phase 4a — `notification_delivery` table + backfill; in_app via delivery rows
- [ ] Phase 4b — `mediums[]` in send + email delivery processor
- [ ] Phase 5 — SNS webhook + suppression + reputation
- [ ] Phase 6 — Unsubscribe (List-Unsubscribe header + endpoint)
- [ ] Phase 7 — Console: analytics, reputation, suppression, catalog grid
- [ ] Phase 8 — Billing: per-metric split + rename migration
- [ ] **Cleanup**: drop `notification.status/read_at/opened_at` after dual-write stabilizes
- [ ] **Cleanup**: remove 30-day `mediums=["in_app"]` SDK compat shim

### Notes / in-flight decisions

_(empty — append dated notes here as implementation surfaces issues)_

---

## Context

Bodhveda today delivers notifications only to an inbox (read via REST API). The product needs to fan out the same notification event to additional mediums — starting with **email** (via AWS SES), with `sms`, `web_push`, `mobile_push` scaffolded so they slot in later without schema churn.

This doc is the finalized design. It's the result of a multi-round Q&A that locked 19 decisions; the intent here is to give the implementation a single reference to execute from, phase by phase. Code is NOT written yet — only the design and a phased rollout.

The mental model: a notification is a logical event (`{channel, topic, event}` + payload + recipient); each send specifies which **mediums** to attempt. Preferences are per-`(target, medium)` (Instagram-style toggles). Project owners declare the catalog of allowed `(target, medium)` combinations up front; recipients opt in/out per combo. Inbox becomes one medium among many (`in_app`) rather than the implicit default.

---

## Locked Decisions (quick reference)

1. **Medium enum**: `in_app, email, sms, web_push, mobile_push`. TEXT + CHECK constraints (matches existing style), no PG enum types.
2. `in_app` is a regular medium; inbox row is created only if `in_app ∈ mediums`. **Breaking change** to send API.
3. Both direct and broadcast sends take `mediums[]`. Broadcast computes `delivered_mediums = mediums ∩ recipient_enabled_mediums` per recipient.
4. **Catalog-gated**: project must declare `(target, medium)` in its preference catalog before sends are accepted. Send to undeclared combo → `400`.
5. Preferences only *deny* (no recipient pref → fall back to project default; project default missing is impossible because of the catalog gate).
6. **Contacts** live in a new `recipient_contact` table keyed on `(project, recipient, medium, address)`. Dedicated CRUD endpoints. No `unsubscribed_at` column — preferences + suppression handle that.
7. **Email content** = project-level `email_config` (verified domain, default from/reply-to, footer, tracking flags) + per-send `email.{subject, html, from_name?, reply_to?}` override. Subject + HTML always required per-send.
8. **Email backend** = AWS SES. **One SES identity per project**, even when two projects share the same domain (strict reputation isolation). Each project gets its own SES configuration set + SNS topic + DKIM records.
9. **Pre-verify**: `send(mediums:["email"])` returns `409 domain_not_verified` until the project's SES identity shows `verified`.
10. **Reputation**: per-project `project_email_reputation` row. Daily cap auto-ramps 500 → 2k → 10k over ~30 days; auto-pauses on bounce_rate > 5% or complaint_rate > 0.1%.
11. **Billing**: per-medium metrics (`in_app_notifications`, `emails_sent`, later `sms_sent`, `push_sent`). Plan entitlements become per-metric. Existing `notifications` usage rows rename to `in_app_notifications`.
12. **Tracking**: per-project `track_opens`, `track_clicks` booleans, **off by default** even post-verify. No per-send override v1.
13. **No-contact**: if broadcast wants `email` for a recipient with no email contact → `notification_delivery` row with `status=no_contact`, visible in analytics.
14. **Delivery record**: new `notification_delivery` table, one row per `(notification, medium)`. `notification` keeps only the logical event; `status`/`read_at`/`opened_at` move to the delivery row.
15. **Unsubscribe**: RFC 8058 `List-Unsubscribe` + `List-Unsubscribe-Post: List-Unsubscribe=One-Click` headers on every email. Clicking writes `preference(target, medium=email, enabled=false)`. Target-scoped, not global.
16. **Existing preference rows**: `ALTER TABLE preference ADD COLUMN medium TEXT NOT NULL DEFAULT 'in_app'` + recreate partial unique indexes with `medium` appended.
17. **Recipient-scoped API keys** can `POST/GET/PATCH` their own contacts; `DELETE` requires full scope.
18. **CAN-SPAM**: `physical_mailing_address` is a **required** field on `project_email_config`. Bodhveda injects it into every email footer alongside the unsubscribe link.
19. **Partial-medium failure**: `mediums=[in_app, email]` with `emails_sent` quota exceeded → `200 OK` with per-delivery statuses (`in_app:pending, email:quota_exceeded`). Never atomic-reject the whole send for a per-medium issue.

---

## Schema (DDL)

All migrations are Goose-format in `/Users/ceoshikhar/dev/bodhveda/migrations/`. Index creation under write load uses `CREATE UNIQUE INDEX CONCURRENTLY` with `-- +goose NO TRANSACTION`.

### Altered: `preference`

```sql
ALTER TABLE preference
    ADD COLUMN medium TEXT NOT NULL DEFAULT 'in_app'
    CHECK (medium IN ('in_app','email','sms','web_push','mobile_push'));

DROP INDEX IF EXISTS recipient_pref_unique;
DROP INDEX IF EXISTS project_pref_unique;

CREATE UNIQUE INDEX recipient_pref_unique
    ON preference(project_id, recipient_external_id, channel, topic, event, medium)
    WHERE recipient_external_id IS NOT NULL;

CREATE UNIQUE INDEX project_pref_unique
    ON preference(project_id, channel, topic, event, medium)
    WHERE recipient_external_id IS NULL;
```

Safety: `ADD COLUMN NOT NULL DEFAULT <const>` is a metadata-only op on PG ≥11; no table rewrite. The migration and the code update that threads `medium` through `INSERT ... ON CONFLICT` in `api/internal/pg/preference.go` MUST ship together (old ON CONFLICT clause stops matching the recreated partial unique the moment the index is rebuilt).

### New: `recipient_contact`

```sql
CREATE TABLE recipient_contact (
    id                      BIGSERIAL PRIMARY KEY,
    project_id              INT NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    recipient_external_id   VARCHAR(255) NOT NULL,
    medium                  TEXT NOT NULL
                            CHECK (medium IN ('email','sms','web_push','mobile_push')),
    address                 TEXT NOT NULL,
    is_primary              BOOLEAN NOT NULL DEFAULT false,
    verified_at             TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (project_id, recipient_external_id, medium, address),
    FOREIGN KEY (project_id, recipient_external_id)
        REFERENCES recipient(project_id, external_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX ux_recipient_contact_one_primary
    ON recipient_contact(project_id, recipient_external_id, medium)
    WHERE is_primary = true;

CREATE INDEX ix_recipient_contact_primary_lookup
    ON recipient_contact(project_id, recipient_external_id, medium)
    WHERE is_primary = true;
```

`in_app` is intentionally NOT a valid medium here — the "contact" for in_app is the `recipient_external_id` itself.

### New: `project_email_config`

```sql
CREATE TABLE project_email_config (
    project_id                INT PRIMARY KEY REFERENCES project(id) ON DELETE CASCADE,
    domain                    TEXT NOT NULL,
    default_from_email        TEXT NOT NULL,
    default_from_name         TEXT NOT NULL DEFAULT '',
    default_reply_to          TEXT,
    footer_html               TEXT NOT NULL DEFAULT '',
    physical_mailing_address  TEXT NOT NULL,              -- CAN-SPAM; auto-injected in footer
    track_opens               BOOLEAN NOT NULL DEFAULT false,
    track_clicks              BOOLEAN NOT NULL DEFAULT false,
    status                    TEXT NOT NULL DEFAULT 'pending_verification'
                              CHECK (status IN ('pending_verification','verified','failed','disabled')),
    ses_identity_arn          TEXT,
    ses_configuration_set     TEXT,
    sns_topic_arn             TEXT,
    dkim_tokens               JSONB,            -- CNAME records shown in console
    mail_from_domain          TEXT,             -- e.g. 'mail.acme.com' for DMARC alignment
    verified_at               TIMESTAMPTZ,
    last_verification_check   TIMESTAMPTZ,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (project_id, domain)
);

CREATE INDEX ix_project_email_config_status
    ON project_email_config(status);
```

`domain` is NOT globally unique — two projects may claim the same domain, each with its own SES identity.

### New: `project_email_reputation`

```sql
CREATE TABLE project_email_reputation (
    project_id           INT PRIMARY KEY REFERENCES project(id) ON DELETE CASCADE,
    verified_at          TIMESTAMPTZ NOT NULL,
    daily_cap            INT NOT NULL DEFAULT 500,
    sent_today           INT NOT NULL DEFAULT 0,
    reset_at             TIMESTAMPTZ NOT NULL,
    bounce_rate_7d       NUMERIC(5,4) NOT NULL DEFAULT 0.0000,
    complaint_rate_7d    NUMERIC(5,4) NOT NULL DEFAULT 0.0000,
    status               TEXT NOT NULL DEFAULT 'active'
                         CHECK (status IN ('active','throttled','paused')),
    paused_reason        TEXT,
    last_ramp_at         TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### New: `email_suppression`

Per-project. We mirror SES config-set-level suppressions into Postgres as the source of truth (queryable from the console; bounce vs. complaint distinguishable).

```sql
CREATE TABLE email_suppression (
    id             BIGSERIAL PRIMARY KEY,
    project_id     INT NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    address        TEXT NOT NULL,           -- lowercased
    reason         TEXT NOT NULL
                   CHECK (reason IN ('hard_bounce','complaint','manual','unsubscribed_globally')),
    source         TEXT NOT NULL,           -- 'ses','user','api','console'
    source_detail  JSONB,                   -- raw SNS event if from SES
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (project_id, address)
);
```

Kept on recipient-delete (the address is the suppression subject, not the person; CAN-SPAM + deliverability).

### New: `notification_delivery`

```sql
CREATE TABLE notification_delivery (
    id                      BIGSERIAL PRIMARY KEY,
    notification_id         INT NOT NULL REFERENCES notification(id) ON DELETE CASCADE,
    project_id              INT NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    recipient_external_id   VARCHAR(255) NOT NULL,

    medium                  TEXT NOT NULL
                            CHECK (medium IN ('in_app','email','sms','web_push','mobile_push')),
    contact_id              BIGINT REFERENCES recipient_contact(id) ON DELETE SET NULL,
    address_snapshot        TEXT,                 -- captured at enqueue time; immune to later contact edits

    status                  TEXT NOT NULL
                            CHECK (status IN (
                                'pending','sending','sent','delivered','bounced','complained',
                                'failed','muted','no_contact','suppressed','quota_exceeded','rejected'
                            )),
    provider                TEXT,                 -- 'ses','twilio','fcm','apns'
    provider_message_id     TEXT,
    provider_response       JSONB,
    failure_reason          TEXT,
    attempt                 INT NOT NULL DEFAULT 0,

    sent_at                 TIMESTAMPTZ,
    delivered_at            TIMESTAMPTZ,
    bounced_at              TIMESTAMPTZ,
    complained_at           TIMESTAMPTZ,
    opened_at               TIMESTAMPTZ,          -- email pixel OR in_app "viewed"
    clicked_at              TIMESTAMPTZ,
    read_at                 TIMESTAMPTZ,          -- in_app only; NULL elsewhere

    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (notification_id, medium)
);

CREATE INDEX ix_nd_notification ON notification_delivery(notification_id);
CREATE INDEX ix_nd_project_recipient
    ON notification_delivery(project_id, recipient_external_id, created_at DESC);
CREATE UNIQUE INDEX ux_nd_provider_message
    ON notification_delivery(medium, provider_message_id)
    WHERE provider_message_id IS NOT NULL;
CREATE INDEX ix_nd_inbox_unread
    ON notification_delivery(project_id, recipient_external_id)
    WHERE medium = 'in_app' AND read_at IS NULL;
CREATE INDEX ix_nd_email_status_time
    ON notification_delivery(project_id, created_at DESC)
    WHERE medium = 'email';
```

**`notification` column drops** (second migration, one release later, after dual-write has stabilized): `status`, `read_at`, `opened_at` move to `notification_delivery`. Backfill runs in the first migration:

```sql
INSERT INTO notification_delivery (
    notification_id, project_id, recipient_external_id,
    medium, status, read_at, opened_at, sent_at, delivered_at,
    created_at, updated_at
)
SELECT n.id, n.project_id, n.recipient_external_id,
       'in_app',
       CASE n.status
         WHEN 'delivered' THEN 'delivered'
         WHEN 'muted' THEN 'muted'
         WHEN 'quota_exceeded' THEN 'quota_exceeded'
         WHEN 'failed' THEN 'failed'
         ELSE 'pending'
       END,
       n.read_at, n.opened_at,
       n.completed_at, n.completed_at,
       n.created_at, n.updated_at
FROM notification n
ON CONFLICT DO NOTHING;
```

### Altered: `broadcast`

```sql
ALTER TABLE broadcast
    ADD COLUMN mediums           TEXT[] NOT NULL DEFAULT ARRAY['in_app'],
    ADD COLUMN email_subject     TEXT,
    ADD COLUMN email_html        TEXT,
    ADD COLUMN email_from_name   TEXT,
    ADD COLUMN email_reply_to    TEXT,
    ADD COLUMN medium_statuses   JSONB;  -- per-medium outcome for partial-quota cases

ALTER TABLE broadcast
    ADD CONSTRAINT broadcast_mediums_valid
    CHECK (mediums <@ ARRAY['in_app','email','sms','web_push','mobile_push']);
```

### New: `broadcast_eligibility`

Materializes the per-recipient eligible-mediums set from the big SQL so we never pass it through Asynq task payloads.

```sql
CREATE TABLE broadcast_eligibility (
    id                      BIGSERIAL PRIMARY KEY,
    broadcast_id            INT NOT NULL REFERENCES broadcast(id) ON DELETE CASCADE,
    recipient_external_id   VARCHAR(255) NOT NULL,
    eligible_mediums        TEXT[] NOT NULL,
    no_contact_mediums      TEXT[] NOT NULL DEFAULT '{}',
    suppressed_mediums      TEXT[] NOT NULL DEFAULT '{}',
    processed_at            TIMESTAMPTZ
);

CREATE INDEX ix_be_broadcast_unprocessed
    ON broadcast_eligibility(broadcast_id)
    WHERE processed_at IS NULL;
```

---

## API

### Developer API (API-key auth, `cmd/api/routes.go`)

| Method | Path | Scope | Notes |
|---|---|---|---|
| POST | `/notifications/send` | full | Breaking: `mediums[]` now required. Optional `email` block required iff `email ∈ mediums`. |
| POST | `/recipients/{id}/contacts` | full or recipient-self | `{medium, address, is_primary}`. Idempotent on unique key. |
| GET | `/recipients/{id}/contacts` | full or recipient-self | |
| PATCH | `/recipients/{id}/contacts/{contact_id}` | full or recipient-self | Address change invalidates `verified_at`. |
| DELETE | `/recipients/{id}/contacts/{contact_id}` | full only | |
| GET | `/recipients/{id}/preferences/check?channel=&topic=&event=&medium=` | recipient-self | Existing endpoint, `medium` param added. |
| PATCH | `/recipients/{id}/preferences` | recipient-self | `{target, medium, state:{enabled}}`. |
| POST | `/webhooks/ses/{project_id}` | SNS signature | Unauthenticated; signature-verified. Always 200. |
| GET | `/unsubscribe?token=` | token | Confirmation page (HTML). |
| POST | `/unsubscribe` | token | RFC 8058 one-click; form-encoded body. |

`POST /notifications/send` request:

```json
{
  "recipient_id": "user_abc",
  "target": { "channel": "posts", "topic": "post_123", "event": "new_comment" },
  "mediums": ["in_app", "email"],
  "payload": { "...": "..." },
  "email": {
    "subject": "New comment on your post",
    "html": "<p>...</p>",
    "from_name": "Acme",
    "reply_to": "support@acme.com"
  }
}
```

Response (direct):
```json
{
  "notification": { "id": 123, "target": {...}, "payload": {...}, "created_at": "..." },
  "deliveries": [
    { "id": 501, "medium": "in_app", "status": "pending" },
    { "id": 502, "medium": "email",  "status": "pending" }
  ]
}
```

Errors:
- `400 catalog_missing` — one or more `(target, medium)` pairs aren't in the catalog. Body lists offenders.
- `400 email_block_required` — `email ∈ mediums` but `email` block absent/incomplete.
- `400 from_email_domain_mismatch` — `email.from_name`/`reply_to` overrides fine, but `from_email` (if ever exposed) must be on the verified domain.
- `409 domain_not_verified` — `email ∈ mediums` but `project_email_config.status != 'verified'`.
- `422 mediums_empty`.
- `429` quota — only when ALL mediums exhausted; partial-quota returns 200 with per-delivery `quota_exceeded` status.

### Console API (session auth, all under `/console/projects/{project_id}`)

| Method | Path | Purpose |
|---|---|---|
| POST | `/notifications/send` | Test-send from console. Same shape as developer API. |
| GET | `/notifications` | List with joined per-delivery summaries. |
| GET | `/notifications/{id}` | Single notification + all its deliveries. |
| GET | `/deliveries?medium=&status=&from=&to=` | Per-medium delivery listing for analytics. |
| GET | `/deliveries/overview?medium=&period=` | Aggregate stats: sent, delivered, bounced, opened, clicked. |
| POST | `/preferences/bulk` | Create an N×M catalog grid in one shot. |
| GET | `/preferences?kind=project` | Response grouped by target: `{target, label, mediums:{in_app:bool,email:bool,...}}`. |
| GET | `/email-config` | Current config + DKIM records + status. |
| POST | `/email-config` | `{domain, default_from_email, default_from_name, physical_mailing_address}`. Creates SES identity + config set + SNS topic. Returns DKIM CNAMEs. |
| PATCH | `/email-config` | Update `default_from_name, footer_html, track_opens, track_clicks, default_reply_to, physical_mailing_address`. Domain is immutable — DELETE + POST to change. |
| DELETE | `/email-config` | Tears down SES identity + config set + SNS subscription. |
| POST | `/email-config/verify` | Force re-check. Rate-limited to 1/min. |
| GET | `/email-config/reputation` | `{status, daily_cap, sent_today, bounce_rate_7d, complaint_rate_7d, next_ramp_at}`. |
| GET | `/email-config/suppressions?page=&address=` | Paginated list. |
| DELETE | `/email-config/suppressions/{id}` | Manual removal. |

---

## Delivery Pipeline

### Direct send

HTTP handler → service → DB insert (in tx) → per-delivery Asynq enqueue.

1. **Handler** (`api/internal/handler/notification.go`) decodes the payload.
2. **Service** (`api/internal/service/notification.go`):
   1. Shape validation: `mediums` non-empty; `email` block present iff `email ∈ mediums`; subject + html non-empty.
   2. **Catalog gate**: one SQL — `SELECT medium FROM preference WHERE project_id=$1 AND recipient_external_id IS NULL AND channel=$2 AND topic=$3 AND event=$4 AND medium = ANY($5)`. Returned set must equal requested set; else 400.
   3. **Verification gate**: if `email ∈ mediums`, check `project_email_config.status = 'verified'`; else 409.
   4. **Recipient**: `CreateIfNotExists` (existing behavior).
   5. **Per-medium preference cascade** (new repo method `ShouldDirectNotificationBeDeliveredPerMedium`): returns `map[medium]bool`. False → delivery row pre-set to `muted`.
   6. **Contact lookup** (non-in_app mediums only): fetch primary contact row. Missing → delivery row pre-set to `no_contact`; don't enqueue.
   7. **Suppression check** (email only): `email_suppression` lookup. Hit → delivery row pre-set to `suppressed`; don't enqueue.
   8. **Transactional write** (`dbx.WithTx`):
      - INSERT `notification`.
      - INSERT N `notification_delivery` rows, statuses already resolved for `muted/no_contact/suppressed`; others `pending` with `address_snapshot` captured.
   9. **Enqueue**: for each `pending` delivery row, enqueue the medium-specific Asynq task (`TaskTypeInAppDelivery`, `TaskTypeEmailDelivery`, ...).
   10. Return 200 with inline `deliveries: [...]`.

**Billing stays in processors**, not the service — only commit-time consumption avoids double-charging on Asynq retry.

### Broadcast send

1. Handler + service validate (catalog gate, verification gate).
2. INSERT `broadcast` with `mediums`, `email_subject`, `email_html`, `email_from_name`, `email_reply_to`.
3. Enqueue `broadcast:prepare_batches` with the broadcast ID.
4. **`PrepareBroadcastBatches` processor**:
   1. Run eligibility SQL (see below) as `INSERT INTO broadcast_eligibility ... SELECT ...`. Server-side; no data round-trip.
   2. Aggregate per-medium counts; call `BillingService.CheckAndConsumeUsage` once per medium. If a medium exceeds quota, mark `broadcast.medium_statuses[medium] = quota_exceeded` and skip that medium's downstream delivery; continue for others. (Partial outcome.)
   3. Chunk `broadcast_eligibility` rows into batches of ~500 when email is present, ~1000 for in_app-only. Enqueue one `broadcast:delivery` task per batch with `(broadcast_id, id_range)`.
5. **`BroadcastDelivery` processor** (per batch):
   1. For each `(recipient, eligible_mediums)`: INSERT one `notification` row + N `notification_delivery` rows.
   2. In-app delivery is inline (DB-write == delivered); status → `delivered`.
   3. For non-in_app: enqueue one child task per delivery (e.g. `email:delivery` with the `delivery_id`).
   4. Mark `broadcast_eligibility.processed_at = now()`.
6. **`EmailDelivery` processor** per child task — same as direct-send email path below.

### Broadcast eligibility SQL (materializes to `broadcast_eligibility`)

```sql
WITH requested AS (SELECT unnest($5::text[]) AS medium),
recipients_in_project AS (
    SELECT r.external_id FROM recipient r
    WHERE r.project_id = $1 AND r.deleted_at IS NULL
),
cross_cases AS (
    SELECT r.external_id, m.medium
    FROM recipients_in_project r CROSS JOIN requested m
),
recipient_pref AS (
    SELECT recipient_external_id, medium, enabled FROM preference
    WHERE project_id = $1 AND recipient_external_id IS NOT NULL
      AND channel = $2 AND topic = $3 AND event = $4 AND medium = ANY($5)
),
project_pref AS (
    SELECT medium, enabled FROM preference
    WHERE project_id = $1 AND recipient_external_id IS NULL
      AND channel = $2 AND topic = $3 AND event = $4 AND medium = ANY($5)
),
contact_exists AS (
    SELECT recipient_external_id, medium FROM recipient_contact
    WHERE project_id = $1 AND is_primary = true AND medium = ANY($5)
),
suppressed AS (
    SELECT rc.recipient_external_id, 'email'::text AS medium
    FROM recipient_contact rc JOIN email_suppression s
      ON s.project_id = rc.project_id AND s.address = lower(rc.address)
    WHERE rc.project_id = $1 AND rc.medium = 'email' AND rc.is_primary = true
),
resolved AS (
    SELECT
        cc.external_id AS recipient_external_id,
        cc.medium,
        COALESCE(rp.enabled, pp.enabled, false) AS prefers_enabled,
        (ce.recipient_external_id IS NOT NULL OR cc.medium = 'in_app') AS has_contact,
        (sp.recipient_external_id IS NOT NULL) AS is_suppressed
    FROM cross_cases cc
    LEFT JOIN recipient_pref rp
      ON rp.recipient_external_id = cc.external_id AND rp.medium = cc.medium
    LEFT JOIN project_pref pp ON pp.medium = cc.medium
    LEFT JOIN contact_exists ce
      ON ce.recipient_external_id = cc.external_id AND ce.medium = cc.medium
    LEFT JOIN suppressed sp
      ON sp.recipient_external_id = cc.external_id AND sp.medium = cc.medium
)
INSERT INTO broadcast_eligibility
    (broadcast_id, recipient_external_id, eligible_mediums, no_contact_mediums, suppressed_mediums)
SELECT
    $6, recipient_external_id,
    COALESCE(array_agg(medium) FILTER (WHERE prefers_enabled AND has_contact AND NOT is_suppressed), '{}'),
    COALESCE(array_agg(medium) FILTER (WHERE prefers_enabled AND NOT has_contact), '{}'),
    COALESCE(array_agg(medium) FILTER (WHERE is_suppressed), '{}')
FROM resolved
GROUP BY recipient_external_id
HAVING array_length(array_agg(medium) FILTER (WHERE prefers_enabled), 1) > 0;
```

---

## SES Integration

Lives in a new feature folder: `api/internal/feature/email_delivery/` (feature-folder pattern per CLAUDE.md).

Files:
- `core.go` — domain types: `EmailConfig`, `Reputation`, `Suppression`, `EmailDelivery`.
- `ses_client.go` — wraps `aws-sdk-go-v2 sesv2`.
- `service.go` — `VerifyDomain(projectID)`, `SendEmail(payload)`, `HandleSNSEvent(event)`, `RecomputeReputation(projectID)`.
- `repository.go` — DB ops for `project_email_config`, `project_email_reputation`, `email_suppression`.
- `webhook_handler.go` — SNS endpoint.
- `sns_signature.go` — SNS message signature verification (allowlist `*.amazonaws.com` for `SigningCertURL`).
- `unsubscribe.go` — token HMAC + endpoint handlers.
- `reputation.go` — cron-triggered ramp/pause transitions.
- `templates/unsubscribe.html` — static page for GET confirmation.

### Domain verification flow

1. `POST /console/.../email-config` with `{domain, default_from_email, physical_mailing_address, ...}`.
2. Service calls SES `CreateEmailIdentity(domain)` with DKIM enabled → SES returns 3 CNAME tokens.
3. Create a per-project SES configuration set: `bv-proj-{project_id}`.
4. Create per-project SNS topic: `bv-proj-{project_id}-events`. Subscribe it to the config set via `PutConfigurationSetEventDestination` (event types: Send, Delivery, Bounce, Complaint, Open, Click, Reject).
5. Subscribe Bodhveda's HTTPS endpoint `POST /webhooks/ses/{project_id}` to that SNS topic.
6. Configure MAIL FROM domain `mail.{domain}` via `PutEmailIdentityMailFromAttributes` for DMARC alignment.
7. Persist `project_email_config` with `status='pending_verification'`, DKIM tokens, ARNs.
8. Return DKIM tokens + MX/SPF records for customer to add to DNS.
9. Hourly Asynq cron (`email:poll_verification`) polls SES `GetEmailIdentity` for all `pending_verification` projects. On success → `status='verified'`, `verified_at`, create `project_email_reputation` row.

### Email send (the critical path)

Inside `EmailDelivery` Asynq processor:

```
BEGIN TRANSACTION

UPDATE notification_delivery
   SET status='sending', attempt = attempt+1
 WHERE id=$1 AND status='pending'
 RETURNING *;

-- Zero rows → already sent or in-flight elsewhere; ACK and exit.

COMMIT
```

Then (outside the tx):

1. Check `email_suppression` for `lower(address_snapshot)`. Hit → UPDATE delivery `status='suppressed'`; return.
2. Check reputation atomically:
   ```sql
   UPDATE project_email_reputation
      SET sent_today = CASE WHEN now() > reset_at THEN 1 ELSE sent_today + 1 END,
          reset_at   = CASE WHEN now() > reset_at THEN now() + interval '1 day' ELSE reset_at END,
          updated_at = now()
    WHERE project_id=$1 AND status='active' AND sent_today < daily_cap
    RETURNING sent_today;
   ```
   Zero rows → `status='throttled'` on the delivery, requeue with 1h delay.
3. `BillingService.CheckAndConsumeUsage(emails_sent, 1)`. On quota-exceeded → UPDATE delivery `status='quota_exceeded'`; return.
4. Compose HTML:
   - Append `footer_html` from `project_email_config`.
   - Always append the CAN-SPAM block: physical address + one-click unsubscribe link (token with `(project_id, recipient_external_id, target, medium=email)` signed by `API_HASH_KEY`).
   - Inject 1×1 tracking pixel pointing to `/t/open/{delivery_id}` if `track_opens`.
   - Rewrite `<a href>` through `/t/click/{delivery_id}?to=` if `track_clicks`.
   - Set `List-Unsubscribe: <mailto:unsubscribe@mail.{domain}>, <https://bodhveda.example/unsubscribe?token=...>`
   - Set `List-Unsubscribe-Post: List-Unsubscribe=One-Click`
5. Call SES `SendEmail` with `ConfigurationSetName=bv-proj-{id}` and `EmailTags=[{Name:"delivery_id", Value:<id>}]`.
6. UPDATE delivery `status='sent'`, `provider_message_id`, `sent_at`. Non-ACID guarantee vs SES accepted; covered by the `status='sending'` reaper (see risks).
7. Map SES errors: throttling/5xx → requeue; `MessageRejected`/`AccountSendingPausedException`/validation → `status='failed'`, no retry.

### SNS event handler

`POST /webhooks/ses/{project_id}`:

1. Verify SNS signature (the `SigningCertURL` certificate from an `*.amazonaws.com` host; verify `MessageId/Timestamp/TopicArn/Type/Message/Subject` signing string; reject on timestamp >1h old).
2. On `SubscriptionConfirmation` → GET the `SubscribeURL` once.
3. On `Notification`: parse SES event JSON. Look up delivery row by `EmailTags.delivery_id` or fall back to `Mail.MessageId` → `provider_message_id`.
4. Update delivery row by event type:
   - `Send` → confirm `status='sent'` and `sent_at`.
   - `Delivery` → `status='delivered'`, `delivered_at`.
   - `Bounce` with `bounceType='Permanent'` → `status='bounced'`, `bounced_at`; `INSERT ... ON CONFLICT DO NOTHING` into `email_suppression(reason='hard_bounce')`.
   - `Bounce` with `bounceType='Transient'` → log on delivery; no status change.
   - `Complaint` → `status='complained'`, `complained_at`; suppression `reason='complaint'`.
   - `Open` → `opened_at = COALESCE(opened_at, now())`.
   - `Click` → `clicked_at = COALESCE(clicked_at, now())`.
   - `Reject` → `status='failed'`, `failure_reason='ses_reject'`.
5. **Status is monotonic**: never overwrite `bounced/complained/failed` if a later event arrives.
6. Always return 200 (even for unknown events — log and swallow; SNS otherwise retries).

### Reputation ramp (hourly Asynq cron `email:reputation_recompute`)

Per `project_email_reputation`:
1. Recompute `bounce_rate_7d = bounced_count / sent_count` over the last 7 days from `notification_delivery WHERE medium='email'`.
2. Recompute `complaint_rate_7d`.
3. If `bounce_rate_7d > 0.05` → `status='paused'`, `paused_reason='high_bounce'`.
4. If `complaint_rate_7d > 0.001` → `status='paused'`, `paused_reason='high_complaint'`.
5. If `status='active'` and healthy and `now() - last_ramp_at > ramp_interval`:
   - Day 1–7 → cap 500, Day 8–14 → 2000, Day 15–30 → 10000, Day 31+ → plan-based cap.
6. Daily fallback: reset `sent_today` if `now() > reset_at`.

---

## Unsubscribe

- **Token**: HMAC-SHA256 over `project_id|recipient_external_id|channel|topic|event|medium|iat` using `BODHVEDA_API_HASH_KEY`. Base64url-encoded. TTL 1 year; `iat` prevents indefinite replay.
- **`POST /unsubscribe`** — RFC 8058 one-click. Form body carries the token. Idempotent: always writes/updates the `preference(target, medium=email, enabled=false)` row. Returns short HTML.
- **`GET /unsubscribe?token=`** — Confirmation page. Shows project name, target label, and a button that POSTs back to the same route.

Both routes live outside API-key auth and outside strict CORS.

---

## Billing

Plan entitlement shape changes from `{notifications: limit}` to `{in_app_notifications, emails_sent, sms_sent, push_sent}`.

Migration (one SQL):
```sql
UPDATE usage_log       SET metric = 'in_app_notifications' WHERE metric = 'notifications';
UPDATE usage_aggregate SET metric = 'in_app_notifications' WHERE metric = 'notifications';
```

`MetricNotifications` → `MetricInAppNotifications` in `api/internal/model/entity/plan.go`. Add `MetricEmailsSent`, `MetricSMSSent`, `MetricPushSent`.

`BillingService.CheckAndConsumeUsage(metric, amount)` signature unchanged; each medium processor passes its own metric.

Free-plan entitlement for emails: **TBD product decision** (0? 500?). First period after launch is grace-only — existing customers' in-period usage carries across the rename, but if the new `in_app_notifications` limit is lower than the old `notifications` limit, don't lock them out.

---

## Rollout Phases

Each phase = one deployable unit. Migrations carry the new DDL; code threads the new fields.

### Phase 1 — `preference.medium` + index rebuild

Ship schema and code together (can't split — the ON CONFLICT clause must move in lock-step with the index).

Files:
- `migrations/<new>_add_medium_to_preference.sql`
- `api/internal/model/entity/preference.go` — add `Medium string`
- `api/internal/model/enum/medium.go` (new)
- `api/internal/model/dto/preference.go` — add `Medium`
- `api/internal/pg/preference.go` — all WHERE/ON CONFLICT include `medium`; default `'in_app'`
- `api/internal/handler/preference.go` — accept `medium` in body
- `console/src/features/preference/` — add medium selector (minimal)

Risk: partial-unique index rebuild. Use `CREATE UNIQUE INDEX CONCURRENTLY` + `-- +goose NO TRANSACTION`.

### Phase 2 — Contacts

Files:
- `migrations/<new>_add_recipient_contact.sql`
- `api/internal/model/entity/recipient_contact.go`
- `api/internal/model/repository/recipient_contact.go`
- `api/internal/pg/recipient_contact.go`
- `api/internal/service/recipient_contact.go`
- `api/internal/handler/recipient_contact.go`
- `api/cmd/api/routes.go` — mount `/recipients/{id}/contacts` (dev) and `/console/.../recipients/{id}/contacts` (console)
- `api/internal/app/app.go` — wire repo/service
- `console/src/features/recipient/` — contacts tab in recipient detail
- `console/src/lib/api.ts` — `API_ROUTES` entries

Risk: FK depends on existing composite unique `recipient(project_id, external_id)` — confirmed present.

### Phase 3 — Project email config + SES verification

Files:
- `migrations/<new>_add_project_email_config.sql`
- `api/internal/feature/email_delivery/` (new): `core.go`, `service.go`, `repository.go`, `ses_client.go`, `sns_signature.go`
- `api/cmd/api/routes.go` — `/console/.../email-config/*`
- `api/internal/env/env.go` — add `AWS_REGION`, credentials or IRSA
- `console/src/features/email_config/` — domain setup wizard
- Asynq cron registration for `email:poll_verification`
- `compose.yaml` — LocalStack for dev SES/SNS (optional)

Risks:
- **SES sandbox escape ticket** — start this outside of code. Without production access Bodhveda can only send to verified recipients. Days of lead time.
- IAM policy: `ses:CreateEmailIdentity/GetEmailIdentity/DeleteEmailIdentity/SendEmail/PutConfigurationSetEventDestination`, `sns:CreateTopic/Subscribe/Publish`. Lock to Bodhveda-owned resources.
- DKIM DNS propagation — customer-facing messaging must warn "can take hours."

### Phase 4a — `notification_delivery` table + backfill; in_app through delivery rows

Files:
- `migrations/<new>_add_notification_delivery.sql` (with the in_app backfill SELECT)
- `api/internal/model/entity/notification_delivery.go`
- `api/internal/model/repository/notification_delivery.go`
- `api/internal/pg/notification_delivery.go`
- `api/internal/pg/notification.go` — `BatchCreateTx` writes delivery rows; `UnreadCountForRecipient` switches to the partial index `ix_nd_inbox_unread`; `UpdateForRecipient` updates delivery row not notification
- `api/internal/job/processor/processor.go` — `NotificationDelivery` processor becomes `InAppDelivery` processor and updates the delivery row; broadcast delivery writes delivery rows alongside notifications

**Dual-write**: keep `notification.status/read_at/opened_at` in sync with delivery rows for this release. A follow-up migration drops those columns one release later.

### Phase 4b — `mediums[]` in send + email delivery processor

Files:
- `api/internal/model/dto/notification.go` — `Mediums []string`, `Email *EmailSendOverride` on `SendNotificationPayload`
- `api/internal/service/notification.go` — catalog gate (generalized), per-medium preference cascade, contact+suppression checks, transactional delivery-row insert, per-medium enqueue
- `api/internal/pg/preference.go` — replace `ShouldDirectNotificationBeDelivered` with per-medium version; replace `ListEligibleRecipientExtIDsForBroadcast` with the D.2 query writing to `broadcast_eligibility`
- `api/internal/job/task/task.go` — `TaskTypeEmailDelivery`, `TaskTypeSMSDelivery` (scaffold), etc.
- `api/internal/feature/email_delivery/email_delivery_processor.go`
- `api/cmd/worker/main.go` — register new processors
- `sdk/go/types.go`, `sdk/go/client.go`, `sdk/js/core/src/types.ts` — major version bump
- `docs/` — update quickstart + concepts

Risks:
- Breaking SDK change. Ship a 30-day compat shim that defaults `mediums=["in_app"]` with a deprecation log line; then remove.
- Broadcast × catalog × mediums interplay: if catalog has `in_app` but not `email` for a target, a send of `["in_app","email"]` 400s. That's intended — customer must declare catalog first.

### Phase 5 — SNS webhook + suppression + reputation

Files:
- `migrations/<new>_add_email_suppression_and_reputation.sql`
- `api/internal/feature/email_delivery/webhook_handler.go` + `reputation.go`
- `api/cmd/api/routes.go` — unauthenticated `/webhooks/ses/{project_id}`, outside CORS + API-key middleware, rate-limited separately
- Asynq periodic task `email:reputation_recompute` (hourly)
- `console/src/features/email_config/` — suppression list view, reputation view

Risks:
- SNS signature verification — get this right; forged events could create suppressions and deny service.
- Out-of-order events: only transition status forward; `bounced/complained/failed` are terminal.
- Transient vs permanent bounces — only `Permanent` suppresses; consider a counter for `Undetermined` → suppress after N.

### Phase 6 — Unsubscribe

Files:
- `api/internal/feature/email_delivery/unsubscribe.go`
- `api/internal/feature/email_delivery/templates/unsubscribe.html`
- `api/cmd/api/routes.go` — `GET/POST /unsubscribe` outside auth + CORS
- Header injection in `EmailDeliveryProcessor`

### Phase 7 — Console analytics + UX

Files:
- `console/src/features/notification/analytics.tsx` — per-medium breakdown
- `console/src/features/notification/list/notifications_list.tsx` — join delivery rows
- `console/src/features/email_config/` — reputation + suppression screens
- `console/src/features/preference/` — matrix UI for N×M catalog, grouped by target
- `console/src/features/recipient/detail.tsx` — contacts + per-medium preference toggles

Risk: N×M catalog grid. Group by target; show mediums as checkboxes per row.

### Phase 8 — Billing per-metric

Files:
- `migrations/<new>_rename_notifications_metric.sql`
- `api/internal/model/entity/plan.go` — new metric constants, per-metric entitlements
- `api/internal/service/billing.go` — no signature change
- Processors — pass the right metric each
- `console/src/features/billing/` — per-metric rows

Risk: customer-visible pricing change. Grace-only first billing period.

---

## Known Risks & Operational Blockers

- **F.1 SES sandbox**: AWS account must be moved out of the sandbox before any real customer can send. Open the support ticket at the start of Phase 3, not at the end.
- **F.2 At-least-once email**: the `status='sending'` guard protects against immediate Asynq redelivery but a crash between SES accept and row commit can send twice. Ship a reaper (cron) that finds stale `sending` rows and decides (re-SEND with idempotency key? mark failed?). Accept at-least-once in docs.
- **F.3 Payload limit**: `email.html` cap at 200KB in DTO validation. SES accepts more but >500KB hurts deliverability.
- **F.4 HTML validation**: server-side check that `email.from_name`/`reply_to` are sane; the `from_email` is ALWAYS `project_email_config.default_from_email` (we don't accept an override there) so the domain is guaranteed to match verified identity. Console-side HTML preview uses a sanitizer to prevent stored XSS in our own UI.
- **F.5 DMARC alignment**: MAIL FROM subdomain (`mail.{domain}`) is configured during Phase 3; customer must add MX + SPF TXT. Wizard must explain why or else we ship a deliverability cliff.
- **F.9 Contact race**: `address_snapshot` on the delivery row is populated at enqueue time. The processor sends to the snapshot regardless of subsequent contact edits. Documented.
- **F.10 Recipient-scoped key privilege**: POST/PATCH/GET on own contacts, DELETE forbidden. Attack surface limited because recipient keys can't trigger sends.
- **F.11 SNS endpoint hardening**: signature verification with cert-URL hostname allowlist; reject stale signing timestamps; always 200 (so SNS doesn't retry legitimately handled events).
- **F.15 Bounce subtypes**: only `Permanent` suppresses. Track `Undetermined` count on a side channel (or derive via query) and suppress after 3 to same address.
- **F.17 SES region**: defer per-project region config to v2; all projects in one region for v1.
- **F.20 Tests**: the repo has no tests today (`go test ./...` returns none). This scope is too big to ship untested. Minimum bar: SNS signature unit tests, eligibility SQL snapshot tests, LocalStack-backed SES integration smoke test in CI.

---

## Verification

Per phase; all assume `make dev` stack (Postgres + Redis + dev api + worker + console).

**Phase 1** — `preference.medium` migration:
- `goose -dir migrations postgres "$BODHVEDA_DB_URL" up`
- `psql` → `\d preference` shows `medium` column, partial unique indexes include it
- Insert two prefs with same target different medium — succeeds
- Insert duplicate (same target + same medium) — fails on unique

**Phase 2** — Contacts:
- `curl -H "Authorization: Bearer $KEY" -X POST /recipients/u1/contacts -d '{"medium":"email","address":"a@x.com","is_primary":true}'` → 201
- Attempt second primary for same (recipient, medium) → 409 on the partial unique
- Recipient-scoped key can POST/PATCH own; DELETE returns 403

**Phase 3** — SES verification:
- POST email-config with domain — response contains 3 DKIM CNAMEs
- Without DNS records added, after an hour of cron polling, status still `pending_verification`
- With LocalStack SES, simulate verification → `status='verified'`, `project_email_reputation` row created with `daily_cap=500`
- Send `mediums:["email"]` before verification — 409 `domain_not_verified`

**Phase 4a** — `notification_delivery`:
- Existing notifications have a backfilled `in_app` delivery row with same `read_at/opened_at`
- Mark-as-read updates the delivery row; old `notification.read_at` still in sync (dual-write)
- `UnreadCountForRecipient` hits the partial index

**Phase 4b** — `mediums[]` send:
- Send `mediums:["in_app","email"]` with matching catalog → 200 with 2 delivery rows, both `pending`
- Send with `email ∉ catalog` → 400 `catalog_missing`
- Send with email quota exhausted but in_app OK → 200, `email:quota_exceeded`, `in_app:pending`
- Recipient has no email contact → `notification_delivery` row `status='no_contact'`

**Phase 5** — SNS events:
- Simulated `Bounce(Permanent)` SNS message → delivery `status='bounced'`, row in `email_suppression`
- Simulated `Complaint` → `status='complained'`, suppression row
- Resend to suppressed address → delivery `status='suppressed'`, no SES call
- Reputation cron: inject 6% bounce rate → `status='paused'` with `paused_reason='high_bounce'`; sends 429 until unpaused
- Out-of-order: `Bounce` then `Delivery` → final status stays `bounced`

**Phase 6** — Unsubscribe:
- Open an email (Phase 4b output) → click List-Unsubscribe → recipient preference `(target, email)` flipped to `enabled=false`
- Resend same target+email → delivery `status='muted'`
- Other targets still send

**Phase 7** — Console:
- Catalog grid renders N×M correctly; toggling a cell persists via `POST /preferences` or `DELETE`
- Analytics overview shows sent/delivered/bounced/opened/clicked
- Suppression list paginates and delete removes

**Phase 8** — Billing:
- `usage_log.metric` renamed; console billing page shows per-metric rows
- Over-quota on email does not block in_app

---

## Appendix: Rejected Alternatives

For each major fork, the path we didn't take and the one-line reason it lost out. Helpful when "why not X?" comes up later.

| Decision | Rejected option | Why it lost |
|---|---|---|
| Medium model | `in_app` always implicit; `mediums[]` lists only extras | Asymmetric model — preferences can't disable in-app cleanly, diverges once sms/push land |
| Broadcast mediums | Broadcast takes only target; mediums inferred from prefs | Removes sender's ability to say "this one blast is email-only" |
| Contact storage | Columns on `recipient` (email now, phone/push later) | Can't model multiple contacts per medium, verification state, or primary/fallback |
| Contact storage | Passed per-send, never stored | Forces every send to carry contact data; breaks broadcast entirely |
| Email metadata | Everything per-send (no project defaults) | Noisier clients, no shared from/reply-to/footer |
| Email metadata | Project-only, no per-send override | No escape hatch for context-specific subject/from |
| Contacts API | Inline on recipient create/update only | Full-replace semantics; awkward for one-off add/remove |
| Catalog gating | Implicitly available (preferences only subtract) | No catalog = no "notification settings" UI for recipients; no safety net for undeclared mediums |
| Unsubscribe scope | Kill all email globally for this recipient | Less respectful of user intent; harder to re-engage |
| Unsubscribe scope | Hosted preference-center page | Best UX but too much to build v1; defer |
| SES tenancy | Shared config set + message tags | Tracking toggles are config-set-level → can't vary per project |
| SES tenancy | Shared config set + per-project SNS destinations | Half-measure; no benefit over per-project config sets |
| SES identity | Share identity across same-user projects on same domain | Shared SES-side reputation defeats the isolation goal |
| `unsubscribed_at` on contact | Keep the column | Three sources of "do not email" (pref + suppression + contact) drift-prone |
| Delivery record | Single `status` on `notification` + medium column | Conflates event with attempt; inbox query must filter medium; harder analytics |
| Pref migration | Auto-fan to all known mediums per catalog row | Too opinionated; customer should declare intentionally |
| Pref migration | Migrate + console banner, fully manual | Most work for customer for no gain over default-to-in_app |
| Pre-verify sends | Bodhveda.com subdomain for test sends | Couples bodhveda.com reputation to customer traffic — explicitly contrary to isolation goal |
| Pre-verify sends | Test sends to verified test recipients only | Complexity without much value; verification itself is the forcing function |
| Warmup | Fixed plan caps, no auto-ramp | Customer owns all reputation risk; Bodhveda takes collateral damage on their SES account |
| Warmup | Ramp + plan cap combined | Safest but overkill for v1 state tracking |
| Billing | Single `notifications` counter, 1 unit per medium | Can't price email differently from in_app; emails cost real money via SES |
| Billing | Single counter + separate email ceiling | Two-axis pricing without fully separating metrics — confusing |
| Tracking default | On by default post-verify | Apple MPP distorts opens; privacy regulation surface; SES click-URL rewrite looks phishy |
| Tracking default | Per-project + per-send override | More surface area; per-send override deferred to v2 |
| Tracking default | No tracking v1 | Customers expect delivery stats |
| No-contact handling | Silent skip (no delivery row) | Hides the problem; no "why didn't they get email?" visibility |
| No-contact handling | Aggregate counter only on broadcast | Loses per-recipient granularity for analytics |
| Recipient-key scope | Full-scope only for all contact ops | Too restrictive; forces customer to proxy through their backend |
| Recipient-key scope | All ops including DELETE allowed | DELETE has highest blast radius on stolen recipient key |
| CAN-SPAM address | Optional; customer responsible | Customer legal liability; customers may not know the requirement |
| CAN-SPAM address | Defer to v2 | Easy to add later but leaves v1 customers exposed |
