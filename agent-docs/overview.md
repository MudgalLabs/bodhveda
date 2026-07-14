# Bodhveda — Engineering Overview

> Internal architecture reference written by/for AI coding agents. Lives in `agent-docs/`
> (separate from `docs/`, which is the published Mintlify site). Kept here so future work
> has a single place that explains what the repo is, how it's structured, what exists
> today, and the design decisions we've committed to. Update it when the architecture or a
> decision changes.

## What Bodhveda is

An open-source (AGPLv3) notification platform. A customer (developer) creates a
**project**, gets an **API key**, and sends **notifications** to their **recipients**.
Recipients read notifications through an inbox-style API, and per-recipient
**preferences** gate what actually gets delivered.

**Crucial fact about the current state:** Bodhveda today is an **in-app inbox only**.
There is *no outbound transport* — no email, no push, nothing leaves the system.
"Delivering" a notification means inserting a row into the `notification` table that
the recipient later pulls via `GET /recipients/{id}/notifications`. Keep this in mind
when reading "delivery" anywhere in the code: it means *persist to the inbox*, not
*send over a wire*.

## Repo layout (monorepo)

- `api/` — Go backend. Two binaries: `cmd/api` (chi HTTP server, `:1338`) and
  `cmd/worker` (Asynq worker). All logic under `internal/`.
- `console/` — React 19 + Vite + TanStack Router/Query. Dev on `:6970`. Deploys to
  Cloudflare separately from the API.
- `sdk/go/`, `sdk/js/` — SDKs (`sdk/js/core` publishes as `bodhveda`, plus a `react` pkg).
- `migrations/` — Goose SQL migrations. **No runner is wired in** — apply manually with
  `goose -dir migrations postgres "$BODHVEDA_DB_URL" up`.
- `docs/` — Mintlify site (`docs.json` + MDX under `docs/docs` and `docs/api-reference`).
- `compose.yaml` (base, incl. dev-only console + asynqmon) and `compose.deploy.yaml`
  (prod overlay overriding `image:` on api/worker/migrate).

## Backend layering (`api/internal/`)

Strict `handler → service → repository`, wired in `internal/app/app.go` (`APP` singleton
holds DB pool, Asynq client, services, repos).

- `handler/` — chi handlers; decode request, call service, respond via tantra `httpx`.
- `service/` — business logic. Constructors take repos + cross-service deps + Asynq client.
- `pg/` — pgx repository implementations of interfaces in `model/repository/`.
- `model/` — `entity/` (DB rows/domain), `dto/` (request/response), `enum/` (string enums
  + typed errors in `enum/error.go`), `repository/` (interfaces only).
- `middleware/` — auth (`AuthMiddleware` console session, `APIKeyBasedAuthMiddleware`
  developer API), scope (`VerifyAPIKeyHasFullScope`), ownership
  (`VerifyUserOwnsThisProject`), `CreateRecipientIfNotExists`, logging, timezone.
- `feature/user_identity/`, `feature/user_profile/` — newer "feature-folder" pattern
  (core+service+repo in one package). **Everything else uses the layered split above.
  Follow the existing pattern of whatever domain you're extending; don't refactor mid-task.**
- `job/` — Asynq plumbing: `task/task.go` (task-type constants), `processor/processor.go`
  (all handlers). API enqueues, worker consumes.
- `env/`, `app/` — config + `APP` singleton.

External shared lib **`github.com/mudgallabs/tantra`** provides logger, dbx pgx helpers,
httpx responses, oauth, session manager. Don't reimplement these.

## Two routing surfaces (`cmd/api/routes.go`)

1. **Developer API** — `Authorization: Bearer <api key>`, permissive CORS (`*`), no
   credentials, 100 req/min/IP (`httprate`). API keys have a `scope`:
   - `full` — can send + do everything; gates `/notifications/send` and recipient CRUD
     via `VerifyAPIKeyHasFullScope`.
   - `recipient` — inbox/preferences only, can't send. `/recipients/{recipient_external_id}/…`
     auto-creates the recipient via `CreateRecipientIfNotExists`.
2. **Console API** — `/console/...`, cookie session (scs/pgxstore), strict CORS to
   `BODHVEDA_WEB_URL` with credentials. Project routes nested under `{project_id}`,
   gated by `VerifyUserOwnsThisProject`.

Shared handlers/services where sensible (e.g. `Notification.List` vs `.ListForRecipient`).

## The core domain model

### Notification = Target + payload

A notification carries a **`Target` = {channel, topic, event}** plus a free-form JSON
`payload` (16 KB cap, `enum.NotificationMaxPayloadSize`).

> ⚠️ **`Target.Channel` is a categorization label, not a transport medium.** Examples:
> `channel="posts", topic="post_123", event="new_comment"` or
> `channel="announcements", topic="none", event="new_feature"`. This is why "channel"
> is unavailable as a name for email/push transports.
>
> `topic` reserved words: `any` (preferences only — matches all topics in a channel)
> and `none` (rule has no topic). A send `Target` may use `none` but never `any`.

Two send modes (`SendNotificationPayload`, dispatched in `service.NotificationService.Send`):

- **Direct** — `recipient_id` set. Creates one `notification` row (status `enqueued`),
  enqueues `notification:delivery`.
- **Broadcast** — no recipient, requires a matching **project preference** to exist
  (else 400). Creates a `broadcast` row, enqueues `broadcast:prepare_batches`.

### Preferences (`preference` table) — two levels

- **Project-level** (`recipient_external_id NULL`, `label NOT NULL`) — the catalog of
  subscribable targets. Broadcast requires one of these to exist for its target.
- **Recipient-level** (`recipient_external_id NOT NULL`, `label NULL`) — per-recipient
  opt-in/opt-out.
- Uniqueness enforced by two partial unique indexes; a CHECK enforces the label/recipient
  XOR (`migrations/20250801205117_init.sql`).

**Delivery gating** (`pg/preference.go`):
- `ShouldDirectNotificationBeDelivered` — resolves in priority order: recipient-exact →
  recipient-fallback(`topic='any'`) → project-exact → project-fallback → **default
  `true`** (direct notifications deliver unless explicitly muted).
- `ListEligibleRecipientExtIDsForBroadcast` — the broadcast counterpart (opt-in based).
- `DoesProjectPreferenceExist` — the broadcast precondition check.

### Recipients

Addressed externally by a customer-chosen `external_id` string (stored lowercase), never
the internal serial `id`. Recipient-scoped routes use `{recipient_external_id}`.
**Today a recipient has no contact attributes at all** (no email, no device tokens) —
just `external_id` + `name`. Adding email/push means adding recipient contact info.

## Delivery pipeline (Asynq, `cmd/worker`)

API enqueues, worker consumes. Task types (`job/task/task.go`):

- `notification:delivery` — for one direct send. `NotificationDeliveryProcessor`:
  checks `ShouldDirectNotificationBeDelivered` → if muted, status `muted`; else
  `billingService.CheckAndConsumeUsage` (status `quota_exceeded` if over) → else status
  `delivered`. Sets `completed_at`, updates the row. **This is where a real transport
  step would slot in.**
- `broadcast:prepare_batches` — `PrepareBroadcastBatchesProcessor`: lists eligible
  recipients, consumes usage for the whole set, splits into `broadcast_batch` chunks
  (size = clamp between 100–1000, ~len/10), enqueues one `broadcast:delivery` per batch.
- `broadcast:delivery` — `BroadcastDeliveryProcessor`: `BatchCreateTx` inserts a
  `notification` row per recipient in a tx, updates batch status, and when the last
  batch finishes marks the broadcast `completed`.
- `recipient:delete_data`, `project:delete_data` — async cascading cleanup.

`make up` runs **asynqmon** on `:7755` (dev-only, absent from prod on purpose).
`make dev` starts the worker in its own hot-reloading tmux pane
(`api/air.worker.toml` → `./cmd/worker`) alongside the api and console, so jobs are
exercised locally without any extra step. (To run just the worker standalone:
`go run ./cmd/worker` from `api/`.)

Notification statuses (`enum`): `enqueued`, `muted`, `delivered`, `quota_exceeded`,
`failed`. Broadcast: `enqueued`, `completed`, `quota_exceeded`, `failed`.
**Note:** status is a single scalar on the `notification` row — it assumes exactly one
delivery outcome. A multi-medium world needs per-medium delivery records instead.

## Billing / usage

`service.BillingService` + `pg/usage_*.go`, `user_subscription.go`. Meters the
`notifications` metric per project, consulted on both send paths
(`CheckAndConsumeUsage`) to enforce plan limits. `ErrQuotaExceeded` maps to the
`quota_exceeded` statuses above.

## Console (`console/src/`)

- TanStack Router, file-based routes under `routes/` (`routeTree.gen.ts` auto-generated —
  don't hand-edit). Auth context injected in `App.tsx`.
- TanStack Query; `QueryCache.onError` → `apiErrorHandler` (`lib/api.ts`): toasts
  non-401, silently redirects to sign-in on 401/403.
- Single axios instance, `withCredentials: true`. All endpoint URLs centralized in
  `API_ROUTES` (`lib/api.ts`) — add there, don't hardcode.
- Features under `src/features/{api_key,auth,billing,home,notification,preference,project,recipient}/`
  mirror backend domains. UI lib: `netra`. Tailwind v4.
- Vite reads root `.env` (`envDir: "../"`), exposes only `BODHVEDA_`-prefixed vars.

## Conventions worth remembering

- API key plaintext token is returned **only** on create; stored encrypted (`token`
  BYTEA + `nonce`), looked up by HMAC `token_hash`. Never log/return the plaintext
  elsewhere.
- `UserIdentity` carries the password hash — must never be serialized to clients.
- Recipient `external_id` is the external handle; internal serial `id` stays internal.

## Mediums — committed design

Bodhveda is gaining real outbound transports beyond the in-app inbox. Decisions locked
in so far (this section is the source of truth; update as we build):

### Terminology

- **Medium** = a delivery transport. Values so far: `inapp` (today's inbox — the implicit
  default), `email` (next), later `webpush`. **Not** "channel" — that word is owned by
  `Target.Channel` (a categorization label). Don't overload it.

### Semantics: how a medium fires (RESOLVED)

A send fires a medium only when **all three** hold — sender intent, catalog, and
preference:

1. **Sender intent = presence of that medium's content block.** The send call carries a
   per-medium content block; including the `email` block signals "email is eligible for
   this send." **No `email` block ⇒ no email** (there is nothing to send). `payload` is the
   in-app block. This is the "content-block implies intent" model — non-breaking, no
   explicit `mediums[]` array.
2. **Catalog allows it.** The target must have that medium enabled in the project catalog.
   If a target only offers in-app (no way to enable email), email never sends even if an
   `email` block is present — it's simply skipped (recorded as a delivery outcome, not a
   hard 400, so shared send code across mixed targets doesn't break).
3. **Preference allows it.** When an `email` block IS present, the per-medium preference
   check runs **before** the email is sent; a disabled email preference ⇒ no email.

So one `notifications.send` can fan out to inbox + email, but the sender still controls it
per-send by which content blocks it includes, and the recipient controls it via
preferences. No fallback "derive email from payload" — if there's no `email` block, there's
no email.

### HARD RULE: email is DIRECT-only, never on broadcast (v1)

**Broadcasts must never send email** (yet). Bulk-blasting email is how you destroy sender
reputation and get accounts suspended — exactly the risk BYO-first exists to avoid. Email
fans out **only on direct sends** (`recipient_id` set). Broadcasts stay in-app only. The
broadcast pipeline (`prepare_batches` → `delivery`) is untouched by the email medium. (The
old design doc's broadcast-email machinery — `broadcast.email_*`, `broadcast_eligibility`
with email — is explicitly OUT of v1.) Resurface's digest is a per-user **direct** send, so
this rule does not block the validation target.

### Send API: per-medium content

Because one send may fan out to inbox + email, and each medium needs different content
(inbox = free-form `{title, body}`; email = typed `{subject, html, text}`), the send call
carries **per-medium content**, resolved at delivery time by whichever mediums fire:

- Keep the existing `payload` as the **in-app/default** content (free-form JSON) —
  backward compatible.
- Add an optional typed **sibling `email` block: `{ subject, html, text }`** (at least one
  of `html`/`text`; `text` recommended for deliverability, auto-derivable from `html`).
  **A sibling object, NOT `email_*` scalars inside `payload`:** `payload` is customer
  free-form JSON for in-app rendering (arbitrary keys like `post_url`); injecting reserved
  keys collides and couples concerns, and email needs ≥3 typed, validatable fields
  (subject/html/text, later reply_to/cc). Name it `email` (concise), accepting the minor
  asymmetry with `payload`. Later mediums join as siblings (`web_push`). A unified
  `content: { inapp, email }` map would be cleaner but renaming `payload` is breaking —
  defer to a hypothetical v2.
- **Subject and body come from the Send API, NOT from target/medium config.** Real email
  subjects/bodies are per-send dynamic (Resurface's digest subject is built from that
  day's counts — a static per-target subject can't express it). Bodhveda is a
  **pass-through** for v1: the caller renders its own template (e.g. `@react-email`'s
  `render()` → html + text) and passes the result. No templating engine, no variables.
- **No payload fallback** — if there's no `email` block, no email is sent, full stop
  (decided). The block's presence is the sender's "email eligible" signal (see Semantics).
  Then catalog + preference gate it. `text` recommended for deliverability, auto-derivable
  from `html` when omitted.
- **Deferred (not v1):** per-target email *templates* editable in the console (the
  Knock/Courier model — templates in the platform, payload carries variables). Legitimate
  future feature; deferred like managed SES to keep v1 BYO-minimal.

Later mediums (webpush) add their own typed block (`{ title, body, icon, ... }`) the same
way.

### Provider strategy: BYO adapter, Resend first

- **Adapter-based**, and we ship **BYO-provider** first (customer brings their own email
  provider). Rationale (see decision log below): reselling commodity email means owning
  deliverability/reputation — the hardest email problem — at the worst time. The
  comparables (Knock, Novu, Courier) are all BYO and monetize orchestration, not email
  bytes. Managed sending on our own AWS SES comes **later, as a paid upsell tier**, using
  the *same* adapter interface — so BYO-first throws nothing away.
- **Credentials load from the project's settings** (not platform-global). Each project
  configures its provider creds + a "from" identity (name + verified address).
- **First adapter: Resend.** Chosen partly for dogfooding — Resend's free tier (3k/mo,
  100/day) lets us wire multiple owned products' domains through Bodhveda cheaply.
- Design the adapter interface with **event normalization** built in: each provider emits
  delivered/opened/clicked/bounced/complained with a different webhook schema (SES via
  SNS; Resend/Postmark/Mailgun each their own). Adapters normalize provider events →
  Bodhveda delivery-record status transitions, so analytics stay uniform across providers
  and across a future managed-SES adapter. A webhook ingestion endpoint is required.
- Email "opened" is a **soft** signal (Apple Mail Privacy Protection pre-fetches pixels →
  inflated opens; blocked images → deflated). Label it as directional in the console;
  in-app "read" stays the trustworthy signal.

### Schema shape

> Much of this is already worked out in detail in **`design/multi-medium-delivery.md`**
> (an earlier, very thorough design). See "Reconciliation with the old design doc" below
> for what we harvest from it vs. where today's BYO decisions supersede it. Concrete DDL
> for the tables here lives in that doc — reuse it, adjusting for BYO/Resend and
> direct-only email.

1. **`recipient_contact` table** (DECIDED — not a bare `email` column). Keyed on
   `(project, recipient, medium, address)` with primary/fallback + verification state.
   Chosen because web-push is the *next* medium after email, so build a schema that already
   supports `email`, `web_push`, `mobile_push`, `sms` (medium CHECK enum) and multiple
   contacts per medium — no re-migration when the next medium lands. Only `email` is wired
   in v1. (`in_app` is intentionally NOT a contact medium — its "address" is the
   `recipient_external_id`.)
2. **Preference gains a `medium` dimension + catalog gating.** Add `medium` to `preference`
   (+ rebuild the partial unique indexes with `medium` appended — see old doc for the
   `CREATE UNIQUE INDEX CONCURRENTLY` / `NO TRANSACTION` approach). The gating queries in
   `pg/preference.go` resolve *per medium*. Project-level preferences form the **catalog**:
   a `(target, medium)` must be declared before that medium can fire.
3. **`notification_delivery` map table**, one row per `(notification, medium)`, with its
   own status + attempts + provider message id + timestamps. **v1 scope:** use it for
   **email (and future non-in_app mediums)** only; leave `in_app` status on the existing
   `notification` row (its `status`/`read_at`/`opened_at`) untouched. Rationale: migrating
   the core inbox path onto delivery rows (the old doc's dual-write + column-drop) is a big,
   risky change we don't need for email — defer that consolidation. Adopt the old doc's
   detailed `notification_delivery` DDL, minus the in_app backfill for now.
4. **Project email settings** — Resend API key (encrypted at rest like API-key tokens:
   BYTEA + nonce, never logged/returned) + from-identity (name + address) + a `provider`
   discriminator. (The old doc's `project_email_config`/`reputation`/`suppression` tables
   are SES-specific → they belong to the later managed-email tier, not v1.)

### Worker / pipeline

- New task types + processors per transport (e.g. `email:delivery`), reusing the existing
  Asynq retry machinery, slotting in alongside `notification:delivery`. The in-app
  "delivery" (inbox row insert) stays as-is; email delivery is an additional fan-out branch
  **on the direct-send path only** (never broadcast — see HARD RULE), gated by catalog +
  medium preference + presence of the `email` block.
- Inbound provider (Resend) webhooks → update the `notification_delivery` row.

### Surface area to follow

Console (project provider config, per-medium preference toggles, per-medium delivery
status), SDKs (recipient contacts CRUD; delivery status), docs.

## Reconciliation with the old design doc (`design/multi-medium-delivery.md`)

An earlier, very thorough design doc exists. It locked 19 decisions and full DDL, but was
written **pre-BYO** — its central bet is that Bodhveda owns AWS SES end-to-end. Today's
decisions supersede that for v1; the SES material becomes the **managed-email tier
blueprint** for later. Do not execute the old doc's SES phases in v1.

**Superseded by today (BYO-first):**
- Provider = own AWS SES (per-project SES identity, reputation ramp/warmup, SNS webhooks,
  suppression lists, sandbox-escape) → **BYO Resend first**; SES apparatus = later managed
  tier.
- `in_app` becomes a first-class medium + **breaking** `mediums[]` required on send →
  today: **non-breaking, content-block-implies-intent**; `in_app` stays as-is on the
  `notification` row.
- Broadcast can send email → today: **email is DIRECT-only, never broadcast** (HARD RULE).
- Billing meters `emails_sent` for cost-recovery (SES costs us) → under BYO the customer
  pays Resend, so an email metric is for **plan tiers**, not cost-recovery. Metering is
  optional in v1; decide during the delivery phase.

**Harvest from the old doc NOW (better/more complete than this overview):**
- The `recipient_contact` table DDL (decision #6) — our contact model.
- The `notification_delivery` table DDL (decision #14) — adopt, email-only in v1 (skip the
  in_app backfill/dual-write/column-drop).
- Catalog-gating of `(target, medium)` (decision #4).
- `preference.medium` + partial-unique-index rebuild via `CREATE UNIQUE INDEX CONCURRENTLY`
  + `-- +goose NO TRANSACTION` (decision #16 / Schema section).
- Partial-medium failure ⇒ `200` with per-delivery statuses, never atomic-reject the whole
  send (decision #19).
- Unsubscribe = RFC 8058 `List-Unsubscribe` + one-click, HMAC token over `API_HASH_KEY`,
  target-scoped preference flip (decisions #15, Unsubscribe section).
- Email content = project defaults + per-send override (decision #7) — merge with today's
  `text` field, which the old doc omits.
- Recipient-scoped API keys may CRUD their own contacts, DELETE is full-scope only (#17).
- The **Rejected Alternatives** table — institutional memory; keep it as-is.

**Deferred to the managed-email tier (from the old doc, not v1):**
- `project_email_config`, `project_email_reputation`, `email_suppression`, SES identity
  provisioning, SNS signature verification, reputation ramp/pause, CAN-SPAM footer
  injection, open/click tracking infra. (Some, like CAN-SPAM address + open tracking, may
  resurface earlier if we host managed sending sooner — revisit per phase.)

## Validation target: Resurface (`../resurface`)

Resurface is the real-world app that will prove the email medium works end to end. Its
current digest flow (`cron/src/digest.ts`) does two things **side by side**:

- Sends the daily digest **email via Resend directly**: renders a branded `@react-email`
  component (`cron/src/emails/FollowUpDigest.tsx`) to `html` + `text`, sets `subject`,
  and adds `List-Unsubscribe` / `List-Unsubscribe-Post: One-Click` headers with a signed
  unsubscribe token. From identity: `RESEND_FROM_EMAIL` = `Resurface <hey@resurface.to>`.
  Double-gated on `emailDigestEnabled` (user pref) AND `isPro` (plan).
- **Mirrors it in-app via Bodhveda**: `sendDigestNotification()` in
  `web/lib/bodhveda.ts` calls `notifications.send({ recipient_id, target: targets.digestSent,
  payload: { title, body } })`. Target catalog is `web/lib/bodhveda-targets.ts`
  (`digest/none/sent`). Recipients are created with `{ id, name }` — **no email today**.

**The "final test":** delete Resurface's Resend integration entirely; a single Bodhveda
`notifications.send({ target: digestSent })` fans out to inbox **and** email. For that to
actually replace Resend, the Bodhveda email medium must handle what Resurface relies on:

1. **Recipient email contact** — Resurface registers the user's email as a
   `recipient_contact` (medium=email, primary) via the contacts API (Phase 1). Integration
   pattern (like Arthveda↔Grahak): Resurface syncs it **server-side** on every `/me`
   (never from the browser, so the email never rides a client call). NOTE: Resurface's
   digest is a per-user **direct** send, so the "email is DIRECT-only, never broadcast" rule
   does not block the cutover.
1b. **Preference migration + entitlement split** — Resurface's `emailDigestEnabled` /
   `inAppDigestEnabled` (its DB today) become Bodhveda per-medium preferences on the
   `digest/none/sent` target; Resurface stops storing them. BUT the `isPro` gate is an
   **entitlement** (Resurface business logic) and stays in Resurface — Bodhveda has no plan
   concept. Rule becomes: if `isPro`, send to Bodhveda; Bodhveda decides email vs inapp
   from preferences.
2. **Rich email body, not just `{title, body}`** — RESOLVED: caller supplies a typed
   `email: { subject, html, text }` block on the send call (Resurface renders its
   `@react-email` template → html/text and passes it). Bodhveda does no templating in v1.
   Falls back to `payload.title`/`payload.body` if the block is omitted. See "Send API:
   per-medium content" above.
3. **Unsubscribe is a preference flip — via a SEPARATE public endpoint.** Two distinct
   surfaces write the same email-medium preference:
   - **Authenticated** — the app's In-App/Email toggles → the existing preference API
     (logged-in recipient).
   - **Unauthenticated** — email clients require a `List-Unsubscribe` header + one-click
     `List-Unsubscribe-Post` (RFC 8058; effectively mandated by Gmail/Yahoo bulk-sender
     rules since Feb 2024). This is hit from the *mail client*, with no session/API key,
     so it needs a **public, token-gated endpoint** Bodhveda hosts: a signed/opaque token
     identifies `(project, recipient, target)`; hitting it flips the email medium pref
     off. Must accept **POST** (one-click, auto-POSTed by the mailbox provider) and ideally
     **GET** (renders a small confirmation page).
   - Since Bodhveda sends the email, Bodhveda owns the whole thing (token + header
     injection + endpoint). Resurface deletes its hand-rolled `signUnsubscribeToken` code.
4. **From identity per project** — mirrors `RESEND_FROM_EMAIL`; lives in project settings.
5. **Idempotency** — Resurface dedups sends via a `DigestLog` unique `(userId, localDate)`.
   Consider a caller-supplied idempotency key on send so retries don't double-email.

## Decision log

- **BYO-provider adapter over platform-owned email resale (for v1).** Owning email means
  owning sender reputation/deliverability — SES aggregates bounce/complaint across all
  customers and suspends the whole account past ~0.1% complaints / ~5% bounces, so one bad
  customer = platform-wide outage; isolating needs dedicated IPs + warmup (only economical
  at volume). Category peers (Knock/Novu/Courier) are BYO and monetize orchestration, not
  email. Margins come from the existing notifications/MAR meter, not email markup. Managed
  SES is a later upsell on the same adapter interface. Medium name is **"medium"**;
  email is opt-in per target; provider creds live in project settings; **Resend is the
  first adapter** (dogfooding via its free tier).
- **Email is DIRECT-only in v1 — never on broadcast.** Bulk email blasts are the fastest
  way to wreck sender reputation / get suspended, the exact risk BYO-first exists to avoid.
  Broadcasts stay in-app only; revisit broadcast email only once managed sending +
  reputation controls exist. See HARD RULE above.
- **Content-block-implies-intent send model (non-breaking).** The sender signals which
  mediums to attempt by which content blocks it includes (`payload` ⇒ in-app, `email` ⇒
  email); no `mediums[]` array, no breaking change. **No payload→email fallback** — no
  `email` block means no email. Catalog + per-medium preference still gate. Chosen over the
  old doc's explicit-`mediums[]`-breaking model because it keeps the send API compatible
  while still giving the sender per-send control.
- **`recipient_contact` table over a bare `email` column.** Web-push is the next medium
  after email, so build a contacts schema that already supports email/web_push/mobile_push/
  sms (+ multiple contacts, primary/fallback, verification) and skip a re-migration.
- **`notification_delivery` for email (non-in_app) only in v1.** Adopt the old doc's
  delivery-record table for email, but leave `in_app` status on the `notification` row —
  don't do the old doc's risky inbox migration/dual-write/column-drop until there's a reason.
- **The old design doc (`design/multi-medium-delivery.md`) is retained as the managed-email
  tier blueprint.** Its SES/reputation/suppression apparatus is deferred, not discarded.

## Roadmap — phased delivery (one phase per session)

Each phase is scoped to a single working session and should leave `main` in a shippable,
independently testable state. Phases are ordered by dependency. When a phase completes,
update its status here (`TODO` → `DONE`) and note anything that changed the plan.

Every phase's hand-off prompt starts by telling the new session to read this file first —
this doc is the shared brain across sessions. **Follow the existing layered
handler→service→pg pattern; don't refactor domains mid-phase (see top of doc).**

### Status

- Phase 0 — Design & decisions — **DONE** (this doc + `design/multi-medium-delivery.md`).
- Phase 1 — Recipient contacts (`recipient_contact` table) — **DONE** (see "Phase 1 — deviations" below)
- Phase 2 — Medium model + per-medium preferences + catalog gating — **DONE** (see "Phase 2 — deviations" below)
- Phase 3 — Project email provider settings (Resend creds + from-identity) — **DONE** (see "Phase 3 — deviations" below)
- Phase 4 — Email delivery core (adapter + `email:delivery` worker + `notification_delivery` + send `email` block; DIRECT-only) — **DONE** (see "Phase 4 — deviations (as built)" below)
- Phase 5 — Delivery status via Resend webhooks — **DONE** (see "Phase 5 — deviations (as built)" below)
- Phase 6 — Unsubscribe (List-Unsubscribe header + public endpoint) — **DONE** (see "Phase 6 — deviations (as built)" below)
- Phase 7 — Release prep: Mintlify docs + SDK bump/README + publish runbook — **DONE** (see "Phase 7 — deviations (as built)" below)
- Phase 7.5 — Deploy email medium to VPS + Cloudflare, verify live — **TODO**
- Phase 8 — Resurface cutover against the LIVE instance (the final end-to-end test) — **TODO**

---

### Phase 1 — Recipient contacts (`recipient_contact` table)

- **Goal:** recipients can carry per-medium contact addresses (email in v1), via a
  future-proof contacts table + CRUD, exposed in the SDKs. No delivery yet.
- **In scope:** the `recipient_contact` table (DDL in `design/multi-medium-delivery.md` §
  "New: `recipient_contact`") — keyed `(project, recipient, medium, address)`, `is_primary`
  + `verified_at`, medium CHECK enum `email|sms|web_push|mobile_push` (only `email`
  exercised now); entity + repository + service + handlers; dev-API routes
  `/recipients/{id}/contacts` (POST/GET/PATCH full-or-recipient-self, DELETE full-scope
  only — old doc #17) and console routes; wire in `app.go`; `API_ROUTES` + console
  contacts UI; JS/Go SDK `recipients.contacts.*`. Server-side `/me`-sync pattern
  (Arthveda↔Grahak) is how customers keep it fresh.
- **Out of scope:** mediums on preferences, provider config, sending. The bare
  `email`-column approach is explicitly rejected (need multi-contact + verification +
  web_push next).
- **Depends on:** nothing (FK relies on the existing `recipient(project_id, external_id)`
  composite unique — confirm it's present).
- **Done when:** a full-scope key can POST/GET/PATCH a recipient's email contact; a second
  primary for the same `(recipient, medium)` 409s on the partial unique; a recipient-scoped
  key can POST/PATCH/GET its own but DELETE 403s; SDK round-trips it.

#### Phase 1 — deviations (as built)

Migration: `migrations/20260712120000_add_recipient_contact.sql` (Goose; apply manually with
`goose -dir migrations postgres "$BODHVEDA_DB_URL" up` — no runner is wired in). Backend
follows the layered `handler → service → pg` split (NOT a feature folder), wired in `app.go`.

- **Dropped the redundant `ix_recipient_contact_primary_lookup` index.** The old doc's DDL
  listed both a unique partial index and a plain index on the *identical* columns+predicate
  (`(project, recipient, medium) WHERE is_primary`). The unique index
  `ux_recipient_contact_one_primary` already serves the primary-contact lookup, so the plain
  duplicate (pure write-cost overhead) was omitted.
- **POST is 409-on-conflict, not idempotent.** The old doc's API table said "idempotent on
  unique key", but the Phase 1 "Done when" requires a second primary to **409**. So a
  duplicate `(medium, address)` OR a second primary for a `(recipient, medium)` returns
  `409 conflict` (both surface via the two unique constraints). No upsert.
- **Contacts API accepts all four contact mediums** (`email|sms|web_push|mobile_push`),
  matching the CHECK constraint, even though only `email` is exercised — this is the
  future-proofing the contacts-table decision exists for. `in_app` is rejected (not a
  contact medium). Medium validity lives in `enum.Medium.ValidContactMedium()`;
  `enum/medium.go` is the contact-addressable subset (excludes `in_app`). **Phase 2** will
  introduce the broader shared medium concept (including `in_app`) on preferences.
- **Address normalization:** email addresses are trimmed + lowercased (case-insensitive,
  and aligns with future `lower(address)` suppression lookups); other mediums' addresses are
  only trimmed (push tokens are case-sensitive). PATCHing the address to a different value
  nulls `verified_at` (old doc API-table rule); an unchanged address keeps it.
- **Scope gating** (dev API, under `/recipients/{recipient_external_id}/contacts`, which
  auto-creates the recipient via `CreateRecipientIfNotExists`): POST/GET/PATCH allowed for
  full **or** recipient scope (no `VerifyAPIKeyHasFullScope` gate, exactly like preferences);
  DELETE gated by `VerifyAPIKeyHasFullScope` → recipient-scoped DELETE returns 403.
- **Console UI** is a "Contacts" modal launched from the recipient row's actions dropdown
  (`recipient_contacts_modal.tsx`) — the console has no recipient *detail page* today, so a
  modal (list + add + make-primary + delete) matches the existing modal-driven recipient UX
  rather than inventing a new detail route.
- **SDKs:** Go (`sdk/go` — `client.Recipients.Contacts.{List,Create,Update,Delete}`) and the
  core JS SDK (`sdk/js/core` — `recipients.contacts.{list,create,update,delete}`) both got
  contacts methods. The **React SDK was intentionally NOT given contacts hooks** — contacts
  are synced *server-side* on `/me` (never from the browser, so the email never rides a
  client call, per "Validation target: Resurface" §1); the React package still re-exports the
  new contact types from core for typing convenience.
  - **Not yet version-bumped or published.** The contacts methods are additive and compile,
    but the SDK packages (`sdk/go`, `sdk/js/core`) were left at their current versions and
    NOT published to pkg.go.dev / npm. Publishing is deliberately bundled with the
    email-medium launch (alongside the Mintlify docs, Phase 7), so the whole feature ships as
    one versioned release rather than dribbling out per phase.
- **Untouched (as scoped):** preferences/mediums, provider config, sending, and the broadcast
  pipeline. No bare `email` column was added.

```
Read agent-docs/overview.md in full first (esp. "Mediums — committed design" and
"Reconciliation with the old design doc"), plus the recipient_contact DDL in
design/multi-medium-delivery.md. Implement Phase 1 (Recipient contacts) as scoped: build the
`recipient_contact` table (medium CHECK enum email|sms|web_push|mobile_push, only email used
now; (project,recipient,medium,address) unique; one-primary-per-medium partial index;
is_primary + verified_at; FK to recipient(project_id, external_id)) end-to-end — entity, pg
repo, service, handlers, dev-API routes /recipients/{id}/contacts (POST/GET/PATCH allowed for
full OR recipient-self scope, DELETE full-scope only), console routes + UI, app.go wiring,
API_ROUTES, and JS/Go SDK contacts methods. Do NOT touch preferences/mediums, provider config,
or sending. Do NOT add a bare email column — the contacts table is deliberate (web_push is
next). Follow the layered handler→service→pg pattern; Goose SQL migrations applied manually.
Update Phase 1 status to DONE and note deviations.
```

### Phase 2 — Medium model + per-medium preferences + catalog gating

- **Goal:** the preference/gating layer understands mediums and enforces a **catalog**, so
  the system can decide *per medium* whether a target may deliver. Still no email is sent.
- **In scope:** a shared `medium` enum (`in_app`, `email`, + `web_push`/`mobile_push`/`sms`
  scaffolded to match the contacts enum); `medium` column on `preference` + rebuild the
  partial unique indexes with `medium` appended (`CREATE UNIQUE INDEX CONCURRENTLY` +
  `-- +goose NO TRANSACTION`; backfill existing rows to `in_app`; ship SQL + the
  `ON CONFLICT` code change together — old doc §"Altered: preference"); make the gating
  queries in `pg/preference.go` (`ShouldDirectNotificationBeDelivered`,
  `DoesProjectPreferenceExist`) resolve **per medium**; **catalog gate** — project-level
  `(target, medium)` preferences are the catalog; a medium can only fire if declared;
  preference API + console expose per-medium (In-App / Email) toggles.
- **Out of scope:** provider config, adapters, sending, delivery records. Broadcast email
  gating stays out entirely (email is direct-only).
- **Depends on:** Phase 1 (shares the `medium` enum values).
- **Done when:** a recipient can have `email` enabled/disabled for a target independently
  of `in_app`; per-medium gating returns the right decision; a `(target, medium)` not in the
  catalog is treated as unavailable; console shows two toggles; legacy in-app behavior is
  unchanged (legacy prefs backfilled to `in_app`).

#### Phase 2 — deviations (as built)

Migration: `migrations/20260712130000_add_medium_to_preference.sql` (Goose, `NO TRANSACTION`;
apply manually with goose — no runner is wired in). Backend follows the existing layered
`handler → service → pg` split; no domain was refactored.

- **Shared enum lives in `enum/medium.go` (extended in place, not a new file).** Phase 1's
  contacts enum gained `MediumInApp` plus `Valid()` (all five — matches the
  `preference.medium` CHECK), `Active()` (in_app + email — the only transports that fire in
  v1), and `DefaultMedium = in_app`. `ValidContactMedium()` (email/sms/web_push/mobile_push,
  no in_app) is unchanged — contacts and preferences are overlapping-but-distinct subsets.
- **Gating queries take a `medium enum.Medium` parameter** rather than gaining new
  method names. `ShouldDirectNotificationBeDelivered`, `DoesProjectPreferenceExist`, and
  `ListEligibleRecipientExtIDsForBroadcast` all filter the preference cascade by medium.
  The direct-delivery default is **medium-dependent**: `in_app` defaults to DELIVER (legacy
  "deliver unless muted", no catalog required); every other medium defaults to NOT deliver —
  it fires only when cataloged (a project-level row exists) or the recipient explicitly
  enabled it. That default *is* the catalog gate for non-in_app transports. `in_app` behavior
  is byte-for-byte preserved (backfill + in_app default true + all existing call sites pass
  `enum.MediumInApp`).
- **Broadcast stays in-app only (HARD RULE).** The broadcast precondition
  (`DoesProjectPreferenceExist`) and fan-out (`ListEligibleRecipientExtIDsForBroadcast`) call
  sites pass `enum.MediumInApp`. No broadcast/email machinery was added; the pipeline is
  untouched.
- **`ON CONFLICT` moved in lock-step with the index rebuild.** `pg/preference.go`'s
  recipient upsert now targets `(project_id, recipient_external_id, channel, topic, event,
  medium) WHERE recipient_external_id IS NOT NULL`. A duplicate `(target, medium)` project
  preference 409s on the rebuilt `project_pref_unique` (verified live: same target with
  in_app + email coexists; a second email row is rejected).
- **API is backward compatible — omitted `medium` ⇒ `in_app`.** Every preference
  payload (`CreateProjectPreference`, `UpsertRecipientPreference`,
  `PatchRecipientPreferenceTarget`, `CheckRecipientTarget`) normalizes a missing/blank medium
  to `in_app` and validates it is `Active()` (in_app|email); the check endpoint reads it from
  the `medium` query param. Response DTOs (project + recipient + the recipient-facing
  target/state shapes) all carry `medium`. This keeps the current (un-bumped) SDKs working:
  they send no medium and transparently operate on in-app, exactly as before.
- **Catalog creation is restricted to *active* mediums (in_app, email).** The
  `preference.medium` CHECK accepts all five (scaffolding for web_push/sms/mobile_push), but
  the DTO validation rejects cataloging a medium that can't fire yet.
- **Console: multi-select medium in the create-preference modal + a Medium column.** The
  create modal declares which mediums a target offers (In-App / Email, `type="multiple"`
  ToggleGroup) and creates one project preference per selected medium (one `POST` each — the
  backend stores a row per `(target, medium)`). The project and recipient preference tables
  gained a "Medium" column. A full recipient-facing per-target toggle **grid** was NOT built —
  the developer console has no recipient *detail* preference screen today (recipient prefs are
  a read-only list); the recipient-side In-App/Email toggles are exercised through the
  preference API (SDK-consumed), which is what Resurface will use.
- **SDKs untouched this phase.** Consistent with Phase 1 (SDK publishing is deliberately
  bundled with the email-medium launch), the Go/JS SDK preference types were left as-is; the
  server's omitted-medium→in_app default keeps them functioning.
- **Untouched (as scoped):** provider config, adapters, sending, delivery records, and the
  broadcast pipeline. No email leaves the system.

```
Read agent-docs/overview.md in full first, plus the "Altered: preference" DDL in
design/multi-medium-delivery.md. Implement Phase 2 (Medium model + per-medium preferences +
catalog gating) as scoped: a shared `medium` enum matching the contacts enum (in_app, email,
web_push, mobile_push, sms — only in_app+email active); add `medium` to the `preference`
table and rebuild its partial unique indexes with `medium` appended using CREATE UNIQUE INDEX
CONCURRENTLY + `-- +goose NO TRANSACTION`, backfilling existing rows to in_app, shipping the
ON CONFLICT code change in lock-step; make the gating queries in pg/preference.go resolve per
medium; enforce catalog gating (project-level (target, medium) preferences define what may
fire); surface per-medium In-App/Email toggles in the preference API + console. Do NOT build
adapters/provider settings/sending (Phase 4), and keep email out of any broadcast path.
Preserve in-app behavior exactly. Update Phase 2 status to DONE when finished.
```

### Phase 3 — Project email provider settings

- **Goal:** a project can configure its Resend credentials + from-identity, stored
  securely.
- **In scope:** storage for per-project provider config (Resend API key encrypted at rest
  like API-key tokens — BYTEA + nonce, never logged/returned; a `from` name + address);
  console UI + console API to set/rotate/mask it; a `provider` discriminator field so more
  adapters can be added later.
- **Out of scope:** using the creds to send (Phase 4); webhooks (Phase 5).
- **Depends on:** nothing hard (can run parallel to Phase 2, but sequence it after).
- **Done when:** a project saves Resend creds + from-identity via the console; secret is
  encrypted at rest and returned only masked.

```
Read agent-docs/overview.md in full first. Implement Phase 3 (Project email provider settings)
as scoped: per-project storage for a Resend API key (encrypted at rest exactly like api_key
tokens — BYTEA + nonce, never logged or returned in plaintext) plus a from-identity (name +
address) and a `provider` discriminator for future adapters. Add console UI + console API to
set/rotate/mask it. Do NOT wire sending or webhooks yet. Update Phase 3 status to DONE when
finished.
```

#### Phase 3 — deviations (as built)

Migration: `migrations/20260712140000_add_project_email_settings.sql` (Goose; apply manually
with goose — no runner is wired in). Backend follows the existing layered `handler → service →
pg` split; no domain was refactored.

- **One row per project (`project_id` is the PK), written via upsert.** The
  `project_email_settings` table holds `provider` (TEXT, CHECK `IN ('resend')`, default
  `'resend'`), `secret` BYTEA + `nonce` BYTEA (AES-GCM ciphertext of the Resend API key,
  encrypted exactly like an `api_key` token via tantra `cipher.Encrypt`/`Decrypt` over
  `env.CipherKey`), `from_name`, `from_address`, timestamps. FK
  `project_id → project(id) ON DELETE CASCADE` (so deleting a project drops its settings).
- **Secret is never returned in plaintext.** The response DTO carries only `secret_masked`
  — the last 4 chars behind `••••••••` (`dto.MaskSecret`). The plaintext is decrypted
  **only** server-side to derive that mask (`service.toMaskedDTO`) and, later, to send
  (`entity.DecryptSecret`, Phase 4). It's never logged.
- **Provider discriminator is a real enum** (`enum/email_provider.go`,
  `EmailProviderResend` + `DefaultEmailProvider` + `Valid()`), matching the table CHECK. Only
  `resend` is accepted in v1; the type exists so more adapters slot in without a re-migration.
- **Console-only surface (no Developer API).** Provider config is an owner/console concern,
  not something a recipient- or full-scope API key touches. Routes live under
  `/console/projects/{project_id}/email-settings`: `GET` (returns the masked settings, or
  `data: null` when unconfigured) and `PUT` (upsert). Gated by the existing
  `VerifyUserOwnsThisProject`.
- **Single `PUT` upsert does set + rotate + identity-edit.** The secret is **required on
  first configuration** and **optional afterwards**: an update that omits (or blanks) `secret`
  keeps the existing encrypted key and only changes provider/from-identity (the service loads
  the existing row, carries `secret`/`nonce` forward, and preserves `created_at`); supplying a
  new secret rotates it (fresh encrypt + new nonce). `DTO.SetHasExisting` drives the
  "required only when first configuring" validation.
- **`from_address` is normalized** (trimmed + lowercased, must contain `@`); `from_name`
  trimmed + required.
- **Console UI:** an "Email" sidebar item (route `/projects/$id/settings`, `IconSend`) opens
  a single settings form (`features/email_settings/`) — provider select (Resend only), a
  write-only API-key `PasswordInput` (placeholder tells you it's kept if left blank once
  configured; shows the masked hint in an Alert), and from name/address. Mirrors the
  api-key/create modal's field patterns. No dedicated modal — a plain page form fits a
  once-per-project config better than the recipient-contacts modal style.
- **Tests:** `service/project_email_settings_test.go` (in-memory fake repo — encrypt-at-rest,
  last-4 masking, no-plaintext-leak, rotate, keep-secret-on-identity-only-update,
  first-config-requires-secret, get-when-unconfigured) and `pg/project_email_settings_test.go`
  (real-Postgres round-trip of the insert/`ON CONFLICT` upsert/`Get`/`ErrNotFound` SQL, gated
  on `TEST_DB_URL`, self-cleaning). Both pass.
- **SDKs untouched** (consistent with Phases 1–2 — provider config is console-only anyway;
  the SDK bump is bundled with the Phase 7 launch).
- **Untouched (as scoped):** no sending (Phase 4), no webhooks (Phase 5). The stored creds are
  not yet read by any send path.

### Phase 4 — Email delivery core (DIRECT-only)

- **Goal:** the payoff — a **direct** `notifications.send` that includes an `email` block,
  for a target where email is cataloged + preferred, actually emails the recipient via
  Resend, recorded in `notification_delivery`.
- **In scope:** a medium **adapter interface** + **Resend adapter**; the send API gains the
  typed sibling **`email: { subject, html, text }`** block — **presence ⇒ email eligible;
  absence ⇒ no email (no payload fallback)**; the `notification_delivery` table (old doc
  DDL) used for **email only** in v1 (in_app stays on the `notification` row); the
  send-path fan-out that, after the inbox write, checks **catalog + per-medium preference +
  primary email contact** and enqueues a new **`email:delivery`** Asynq task; the processor
  sends via the Resend adapter and writes the delivery row; **partial-medium ⇒ `200` with
  per-delivery statuses** (never atomic-reject the whole send — old doc #19); email-medium
  billing/metering decision (note: under BYO it's for plan tiers, not cost-recovery —
  optional in v1).
- **Out of scope:** **broadcast email (forbidden — HARD RULE; touch nothing in the broadcast
  pipeline)**; inbound webhooks / open tracking (Phase 5); unsubscribe (Phase 6);
  consolidating in_app onto delivery rows.
- **Depends on:** Phases 1, 2, 3.
- **Done when:** a direct send with an `email` block to an email-cataloged, email-preferred
  target with a primary email contact delivers a real email via Resend and writes a
  `notification_delivery` row; no `email` block ⇒ no email; disabled/uncataloged email ⇒ no
  email with a visible delivery outcome; a recipient with no email contact ⇒ `no_contact`
  delivery row; the in-app path and broadcasts are byte-for-byte unchanged.

```
Read agent-docs/overview.md in full first (esp. Semantics, the HARD RULE that email is
DIRECT-only, the Phase 1/2/3 "deviations (as built)" sections, and the notification_delivery
DDL in design/multi-medium-delivery.md). Implement Phase 4 (Email delivery core, DIRECT-only)
as scoped.

Build on what Phases 1–3 already shipped (reuse — do NOT re-derive):
- Contacts: the `recipient_contact` table + repo. Add/use a "get primary email contact for
  (project, recipient)" lookup (the primary is the row WHERE is_primary, medium=email, guarded
  by ux_recipient_contact_one_primary).
- Mediums + gating: `enum/medium.go` (`enum.MediumEmail`, `Active()`) and the per-medium
  gating queries in `pg/preference.go` — `ShouldDirectNotificationBeDelivered(medium)` already
  resolves catalog + per-medium preference for a given medium. Call it with `enum.MediumEmail`
  to decide whether email may fire (non-in_app defaults to NOT deliver unless cataloged/enabled
  — that default IS the catalog gate).
- Provider config: `project_email_settings` (`enum.EmailProviderResend`, encrypted secret via
  the reserved `entity.DecryptSecret`, `from_name`/`from_address`). Load it to construct the
  adapter; if a project has no email settings, email can't fire.

Then implement:
1. A medium adapter interface + a Resend adapter (creds/from-identity from
   project_email_settings; normalize send result). Provider selected via the `provider`
   discriminator.
2. A typed sibling `email: {subject, html, text}` block on the send API (both the Developer
   `POST /notifications/send` and the console send path share the Notification service) —
   presence ⇒ email eligible, absence ⇒ no email (NO payload fallback).
3. The `notification_delivery` table (old-doc DDL) used for EMAIL ONLY in v1 — leave in_app
   status on the `notification` row; do NOT migrate the inbox onto delivery rows.
4. Direct-send fan-out: after the existing inbox write, when an `email` block is present, gate
   on `ShouldDirectNotificationBeDelivered(email)` (catalog + per-medium pref) AND a primary
   email contact AND configured project_email_settings, then enqueue a new `email:delivery`
   Asynq task (add the task type in internal/job/task, processor in cmd/worker) whose processor
   sends via the Resend adapter and writes the delivery row. Record `no_contact` /
   skipped-uncataloged / disabled as visible delivery outcomes rather than hard-failing.
5. Partial-medium failures return 200 with per-delivery statuses (never atomic-reject; old
   doc #19).

Do NOT send email on broadcasts (forbidden HARD RULE) — leave the broadcast pipeline and its
`enum.MediumInApp` call sites untouched. Do NOT build webhooks (Phase 5) or unsubscribe
(Phase 6). Note the email-metering decision (under BYO it's for plan tiers, not cost-recovery —
optional in v1). Keep the in-app path byte-for-byte unchanged. SDKs stay un-bumped (bundled at
Phase 7) but add the `email` block to the send types if trivial. Follow the layered
handler→service→pg pattern; Goose SQL migration applied manually. Update Phase 4 status to DONE,
add a "Phase 4 — deviations (as built)" section, and record the final notification_delivery
schema + delivery-status enum.
```

#### Phase 4 — deviations (as built)

Migration: `migrations/20260713120000_add_notification_delivery.sql` (Goose; apply manually with
goose — no runner is wired in). Backend follows the existing layered `handler → service → pg`
split; no domain was refactored. `go build ./...`, `go vet ./...`, and the new tests all pass;
the migration is applied and the table + indexes verified live.

- **`notification_delivery` is EMAIL-ONLY in v1, exactly as scoped.** The full old-doc DDL was
  adopted (all status values + every timestamp column: delivered/bounced/complained/opened/
  clicked/read_at) so Phase 5 webhooks need no re-migration — but the in_app backfill /
  dual-write / column-drop was deliberately NOT done. In-app status stays on the `notification`
  row. In v1 only email rows are ever written, and only the statuses
  `pending`/`sent`/`failed`/`muted`/`no_contact` are ever set (the rest are reserved for Phase 5).
  The **final schema + delivery-status enum are recorded at the end of this section.**
- **Delivery rows are created SYNCHRONOUSLY on the send path; the worker only UPDATES them.**
  The prompt said the processor "writes the delivery row"; in practice `fanOutEmail`
  (service layer) resolves every outcome and inserts the row up-front — terminal skips
  (`muted`/`no_contact`/`failed`) never enqueue, and the sendable case inserts a `pending` row
  (with `address_snapshot` + `contact_id`) and enqueues `email:delivery` carrying the row id.
  The processor then `UpdateResult`s that row → `sent`/`failed`. This is the old doc's "insert N
  rows, statuses already resolved, others pending" model, and it's what makes the **synchronous
  200 response carry per-medium statuses** (old doc #19) possible.
- **Gate order in `fanOutEmail`: preference/catalog → provider settings → primary contact.**
  All three are recorded as **visible delivery outcomes** rather than hard-failing the send:
  - `ShouldDirectNotificationBeDelivered(email)` returns false ⇒ status `muted`. To keep the two
    causes distinguishable, `failure_reason` is set to `not_cataloged` (no project-level
    `(target, email)` row — checked via `DoesProjectPreferenceExist`) vs `preference_disabled`
    (an explicit disable). Both share the `muted` status since the old-doc enum has no separate
    "uncataloged" value.
  - no `project_email_settings` ⇒ status `failed`, `failure_reason=provider_not_configured`.
  - no primary email contact (`recipient_contact` WHERE is_primary, medium=email) ⇒ status
    `no_contact` (added `RecipientContactRepository.GetPrimary`).
  - all pass ⇒ status `pending`, enqueue.
- **A failed email fan-out NEVER rejects the send.** `fanOutEmail` returns an error only for
  logging; the direct send still returns 200 with the in-app notification. Even a DB error
  writing the delivery row is logged, not propagated.
- **Send API `email` block** (`dto.EmailContent` on `SendNotificationPayload`): typed sibling
  `{subject, html, text}`. Presence ⇒ email eligible; **no payload fallback**. Validation requires
  `subject` + at least one of `html`/`text`, and **rejects an `email` block on a broadcast**
  (400 — enforces the HARD RULE at the edge rather than silently dropping). `text` is
  auto-derived from `html` when omitted via a deliberately-naive tag stripper
  (`EmailContent.ResolvedText()` / `htmlToText`) — real callers (e.g. `@react-email`'s
  `render()`) pass their own text. The block decodes on **both** send surfaces (Developer
  `POST /notifications/send` and console) since they share the Notification service — no handler
  changes were needed.
- **Adapter interface + Resend adapter live in `internal/email/`.** `Adapter` normalizes
  `Message` → `SendResult{provider, provider_message_id}`; `NewAdapter(provider, apiKey)` selects
  by the `enum.EmailProvider` discriminator. The Resend adapter calls the REST API directly
  (`POST https://api.resend.com/emails`) — **no Resend Go SDK dependency added**; from-identity is
  formatted `"Name <address>"`. Its request URL is an injectable field so tests hit an
  `httptest` server (no external calls, no creds).
- **The worker loads settings FRESH and decrypts per-send (`entity.DecryptSecret`).** The
  provider secret never rides through Redis (the `email:delivery` payload carries only the
  delivery id + project id + normalized content + recipient address), so key rotation is
  respected and no plaintext secret is persisted in the queue. Retries use Asynq's existing
  machinery (`MaxRetry(5)`); each attempt updates the row's `attempt` and, on hard failure, its
  `failure_reason`.
- **Email is NOT metered in v1.** Under BYO the customer pays Resend directly, so an email metric
  would be for plan tiers, not cost-recovery — deferred (the in-app `notifications` meter is
  untouched). Revisit if/when a managed-sending tier lands.
- **Broadcast pipeline untouched.** No broadcast code changed; its `DoesProjectPreferenceExist` /
  `ListEligibleRecipientExtIDsForBroadcast` call sites still pass `enum.MediumInApp`. The
  `email:delivery` task is only ever enqueued from the direct-send path.
- **SDKs (un-bumped, per Phases 1–3):** added the optional `email {subject, html, text}` block +
  a `deliveries[]` field on the send response to `sdk/go` (`types.go`) and `sdk/js/core`
  (`types.ts`). No version bump / publish (bundled with Phase 7). React SDK re-exports core types
  as before.
- **Tests added** (repo's established pattern): `internal/email/resend_test.go` (httptest —
  success, from-identity formatting, provider error), `internal/model/dto/notification_email_test.go`
  (email-on-broadcast rejected, subject/content required, `ResolvedText` derivation), and
  `internal/service/notification_email_test.go` (fake-repo coverage of the four skip outcomes:
  uncataloged/disabled `muted`, `provider_not_configured` `failed`, `no_contact`). The live
  `pending → sent` path exercises real Resend and is left for the Phase 8 Resurface cutover.
- **Untouched (as scoped):** provider webhooks / delivery-status ingestion (Phase 5), unsubscribe
  (Phase 6), and any consolidation of in_app onto delivery rows.

**Final `notification_delivery` schema** (email-only in v1; full column set present for Phase 5):

```sql
CREATE TABLE notification_delivery (
    id                      BIGSERIAL PRIMARY KEY,
    notification_id         INT NOT NULL REFERENCES notification(id) ON DELETE CASCADE,
    project_id              INT NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    recipient_external_id   VARCHAR(255) NOT NULL,
    medium                  TEXT NOT NULL
                            CHECK (medium IN ('in_app','email','sms','web_push','mobile_push')),
    contact_id              BIGINT REFERENCES recipient_contact(id) ON DELETE SET NULL,
    address_snapshot        TEXT,                 -- captured at enqueue; immune to later contact edits
    status                  TEXT NOT NULL
                            CHECK (status IN (
                                'pending','sending','sent','delivered','bounced','complained',
                                'failed','muted','no_contact','suppressed','quota_exceeded','rejected'
                            )),
    provider                TEXT,                 -- 'resend' in v1
    provider_message_id     TEXT,                 -- correlates Phase 5 webhooks
    provider_response       JSONB,
    failure_reason          TEXT,                 -- e.g. not_cataloged / preference_disabled / provider_not_configured
    attempt                 INT NOT NULL DEFAULT 0,
    sent_at                 TIMESTAMPTZ,
    delivered_at            TIMESTAMPTZ,          -- Phase 5
    bounced_at              TIMESTAMPTZ,          -- Phase 5
    complained_at           TIMESTAMPTZ,          -- Phase 5
    opened_at               TIMESTAMPTZ,          -- Phase 5 (soft signal)
    clicked_at              TIMESTAMPTZ,          -- Phase 5
    read_at                 TIMESTAMPTZ,          -- in_app only; unused in v1
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (notification_id, medium)
);
-- ix_nd_notification, ix_nd_project_recipient, ux_nd_provider_message (partial, WHERE
-- provider_message_id IS NOT NULL), ix_nd_email_status_time (partial, WHERE medium='email').
```

**Delivery-status enum** (`enum.DeliveryStatus`, matches the CHECK). v1 sets only: `pending`
(enqueued), `sent` (provider accepted — the v1 success terminal without webhooks), `failed`
(provider error / not configured / decrypt error), `muted` (preference/catalog disallows —
`failure_reason` distinguishes `not_cataloged` vs `preference_disabled`), `no_contact` (no primary
email contact). Reserved for Phase 5: `sending`, `delivered`, `bounced`, `complained`,
`suppressed`, `quota_exceeded`, `rejected`.

### Phase 5 — Delivery status via provider webhooks

- **Goal:** delivered/opened/bounced/complained status flows back into the delivery record
  and the console — parity with in-app read/opened where the medium allows.
- **In scope:** a public webhook ingestion endpoint; Resend event → normalized status
  transitions on the delivery record (the adapter interface owns normalization so future
  providers/managed-SES slot in); console surfaces per-medium delivery status/analytics.
- **Out of scope:** unsubscribe (Phase 6).
- **Depends on:** Phase 4.
- **Done when:** a Resend webhook updates the matching delivery record's status; console
  shows email delivered/opened. (Note in docs: email "opened" is a soft signal.)

```
Read agent-docs/overview.md in full first (esp. the Phase 4 "deviations (as built)" section,
the final notification_delivery schema, and the DeliveryStatus enum with its reserved-for-Phase-5
values). Implement Phase 5 (Delivery status via provider webhooks) as scoped.

Build on Phase 4 (reuse — do NOT re-derive):
- `notification_delivery` already has every column Phase 5 needs (delivered_at/bounced_at/
  complained_at/opened_at/clicked_at, provider_message_id, provider_response). No table
  re-migration — at most a settings column for the webhook signing secret (below).
- The row is correlated to a provider event by `provider_message_id` (set by the Resend adapter
  on send; unique partial index `ux_nd_provider_message`). Add a delivery-repo
  `UpdateStatusByProviderMessageID` (or similar) — the webhook path looks up by that, NOT by
  delivery id.
- `enum.DeliveryStatus` already declares the reserved terminals (`delivered`, `bounced`,
  `complained`, plus `sending`); just start setting them. `opened`/`clicked` are timestamp-only
  soft signals (per old doc), not status transitions — set the `*_at` column, keep `status`.
- Normalization lives in the adapter interface in `internal/email/` — add a
  `NormalizeWebhookEvent(headers, body) → NormalizedEvent{provider_message_id, kind, at}` (or
  similar) to the `Adapter`, implemented by the Resend adapter, so managed-SES/other providers
  slot in later. The public endpoint stays provider-agnostic and dispatches by project settings.

Then implement:
1. Webhook signing secret storage: Resend signs webhooks via Svix (`svix-id`,
   `svix-timestamp`, `svix-signature` headers, HMAC over a per-endpoint signing secret that is
   DISTINCT from the API key). Add an encrypted `webhook_secret` (+ nonce) to
   `project_email_settings` (same cipher pattern as the API key; console PUT lets the owner set
   it), and verify every inbound event against it — reject unverified with 401.
2. A public webhook ingestion endpoint (mounted OUTSIDE the API-key auth group and outside the
   httprate/CORS dev-API group — it's called by Resend, not customers; auth IS the signature).
   Resolve which project by a project-scoped URL path or by the Svix endpoint, verify, normalize,
   then transition the matching delivery row.
3. Status transitions must be ORDER-TOLERANT and non-regressing: webhooks can arrive out of
   order and duplicated. A late `delivered` must NOT overwrite a `bounced`/`complained`; apply
   a terminal-priority guard (bounced/complained/failed are sticky) and make the update
   idempotent. Stamp the matching `*_at` and append raw event to `provider_response`.
4. Console: surface per-medium delivery status/analytics (the delivery rows) on the relevant
   notification/recipient views — at minimum delivered/bounced/complained/opened counts or
   badges. Note in copy that email "opened" is a soft signal.

Keep normalization inside the adapter interface so other providers slot in later. Do NOT
implement unsubscribe (Phase 6) — though you MAY note that a `complained` (spam) event should
eventually suppress future email (leave the actual suppression to a later phase). Broadcast
pipeline stays untouched. SDKs stay un-bumped (Phase 7). Follow the layered handler→service→pg
pattern; Goose SQL migration applied manually. Update Phase 5 status to DONE and add a
"Phase 5 — deviations (as built)" section (record the webhook URL shape, the signing-secret
storage, and the exact Resend-event → status mapping + the non-regression rules).
```

#### Phase 5 — deviations (as built)

Migration: `migrations/20260713140000_add_webhook_secret_to_email_settings.sql` (Goose; apply
manually with goose — no runner is wired in). No `notification_delivery` re-migration was needed:
Phase 4 already shipped the full column set (`delivered_at`/`bounced_at`/`complained_at`/
`opened_at`/`clicked_at`, `provider_response`, `provider_message_id` + the partial unique index
`ux_nd_provider_message`). Backend follows the layered `handler → service → pg` split; no domain
was refactored. `go build`/`go vet`/tests pass; the migration is applied and the transition SQL +
full webhook path were verified live against Postgres.

- **Webhook URL shape: `POST /webhooks/email/{project_id}`** — project resolved from the URL
  path (not the Svix endpoint id). The customer configures exactly this URL as their Resend
  webhook endpoint. It is mounted at the **root router in `cmd/api/routes.go`, OUTSIDE** both the
  developer API-key group (no `APIKeyBasedAuthMiddleware`, no permissive-CORS block) and the
  console session group — **auth IS the signature**. To keep it clear of the dev-API per-IP rate
  limiter, `httprate.LimitByIP(100, time.Minute)` was **moved off the root router onto the two
  groups that had it** (dev-API + console); the webhook and the `/`,`/ping` health checks are no
  longer rate-limited (a provider can burst many events from a small IP pool).
- **Signing-secret storage: encrypted `webhook_secret` + `webhook_nonce` on
  `project_email_settings`** (nullable BYTEA, AES-GCM via the same tantra `cipher` + `env.CipherKey`
  pattern as the provider API key). It is **distinct from the Resend API key** — Resend signs
  webhooks via Svix with a per-endpoint `whsec_...` secret the customer copies from the Resend
  dashboard. `entity.SetWebhookSecret`/`DecryptWebhookSecret`/`HasWebhookSecret`. The console `PUT`
  sets/rotates it independently of the API key: the `Upsert` service was reworked so **each secret
  is (re)encrypted only when its plaintext is supplied, otherwise the existing ciphertext is
  carried forward** (blank ⇒ keep). The webhook secret is **always optional** (a project may send
  before wiring webhooks); the API key stays required-on-first-config. Never returned in plaintext —
  the DTO exposes `webhook_secret_masked` (last 4) + `webhook_secret_set`.
- **Normalization lives in the adapter interface** (`internal/email/`), as scoped. `Adapter` gained
  **`VerifyWebhookSignature(secret, headers, body) error`** and
  **`NormalizeWebhookEvent(headers, body) → NormalizedEvent{ProviderMessageID, Kind, At, Raw}`**;
  both implemented by the Resend adapter. The public endpoint/service stay provider-agnostic and
  select the adapter via the `provider` discriminator (`NewAdapter(provider, "")` — the webhook path
  needs no send API key). Svix verification is implemented **manually (no Svix SDK)**, matching the
  no-Resend-SDK decision: HMAC-SHA256 over `"{svix-id}.{svix-timestamp}.{body}"` with the
  base64-decoded `whsec_` key, constant-time compared against each `v1,<sig>` in the space-delimited
  `svix-signature` header, plus a **±5-min `svix-timestamp` tolerance** (replay guard). Missing
  headers / bad timestamp / wrong secret / tampered body all return `ErrWebhookSignatureInvalid` ⇒
  the endpoint responds **401**.
- **Exact Resend-event → status mapping** (`resendEventKind` + `webhookStatusFor`):
  | Resend `type` | normalized kind | status set | `*_at` stamped |
  |---|---|---|---|
  | `email.sent` | sent | `sent` | — |
  | `email.delivered` | delivered | `delivered` | `delivered_at` |
  | `email.bounced` | bounced | `bounced` (terminal) | `bounced_at` |
  | `email.complained` | complained | `complained` (terminal) | `complained_at` |
  | `email.opened` | opened | *(unchanged — soft signal)* | `opened_at` |
  | `email.clicked` | clicked | *(unchanged — soft signal)* | `clicked_at` |
  | anything else (e.g. `email.delivery_delayed`) | unknown | *(ignored, 200 ack)* | — |
  `opened`/`clicked` are **timestamp-only** (per the old doc): they stamp the column but never touch
  `status`. The console labels "opened" as a directional soft signal (Apple MPP caveat) via a
  tooltip.
- **Non-regression rules (order-tolerant + idempotent).** All transitions go through one guarded SQL
  UPDATE in `pg.ApplyWebhookStatus`, matched **by `provider_message_id`** (not delivery id), so
  duplicated/out-of-order events converge:
  - Status advances only when the incoming status **outranks** the current one, via a rank ladder
    `pending(0) < sending(1) < sent(2) < delivered(3) < {bounced,complained,failed}(4) < else(5)`.
    A **late `delivered` never overwrites `bounced`/`complained`/`failed`** (rank 3 ≯ 4), and among
    the terminals the **first one wins** (equal rank ⇒ strict `>` keeps the incumbent) — so
    bounced/complained/failed are **sticky**. `complained` *can* follow `delivered` (4 > 3), matching
    real "marked spam after receipt".
  - Each `*_at` column is **first-write-wins** (`COALESCE(col, $at)`) — idempotent under duplicates.
  - The raw event is **appended** to the `provider_response` JSONB array
    (`COALESCE(provider_response,'[]') || $event`).
  - An event whose `provider_message_id` matches **no** row (not ours / deleted) is **acknowledged
    with 200** and logged (retrying can't make it match), as is an unmapped/unknown event.
  - *(Minor:* the matching `*_at` is stamped even if the terminal guard keeps `status` unchanged —
    e.g. a contradictory late `delivered` after `bounced` would set `delivered_at` while `status`
    stays `bounced`. Harmless: analytics count `delivered` from `status`, not `delivered_at`, and
    Resend does not emit both for one message. Left as-is per "stamp the matching `*_at`".)*
- **Console.** (1) Email settings form gained a **"Delivery status webhook"** section showing the
  copy-ready webhook URL (`${API_BASE_URL}/webhooks/email/{project_id}`, `API_BASE_URL` newly
  exported from `lib/api.ts`) + a write-only `whsec_...` `PasswordInput` (masked hint + keep-blank-to-
  keep behavior mirroring the API key). (2) A **read-only "Email delivery" KPI row**
  (`email_delivery_overview.tsx`) above the **direct** notifications table (email is DIRECT-only)
  showing sent/delivered/opened/bounced/complained/failed/no_contact/muted counts; it **self-hides
  until the project has attempted ≥1 email**, and the "Opened" tile carries the soft-signal tooltip.
  Backed by a new console endpoint **`GET /console/projects/{project_id}/email-deliveries/overview`**
  → `NotificationService.EmailDeliveryOverview` → `deliveryRepo.EmailDeliveryOverviewForProject`
  (per-status `count(*) FILTER (…)`, `medium='email'`; opened/clicked counted from the `*_at`
  columns, since they're not statuses).
- **Suppression is NOT implemented (deferred to Phase 6, as scoped).** A `complained` event only
  records the complaint on the delivery row; the service carries a NOTE that a spam complaint should
  eventually **suppress future email** to that contact, but the actual suppression is left to a later
  phase.
- **Broadcast pipeline untouched; SDKs un-bumped** (per Phases 1–4 — the SDK bump is bundled with the
  Phase 7 launch; delivery-status webhooks are inbound-only and add no SDK surface).
- **Tests:** `internal/email/resend_webhook_test.go` (Svix verify: valid / tampered-body / wrong-
  secret / stale-timestamp / missing-headers, and the full event→kind mapping) and
  `internal/service/email_webhook_test.go` (real-Postgres end-to-end: bad signature ⇒ 401; valid
  `email.delivered` ⇒ row `sent`→`delivered` + `delivered_at` stamped + one `provider_response`
  entry; a late `email.sent` does **not** regress `delivered`; gated on `TEST_DB_URL`, self-cleaning).
  The order-tolerance/rank/COALESCE/jsonb-append SQL was additionally validated live on Postgres
  (sticky-bounce, forward path, idempotent duplicate, complaint-after-delivery).

### Phase 6 — Unsubscribe (List-Unsubscribe + public endpoint)

- **Goal:** outbound emails carry compliant unsubscribe headers and Bodhveda hosts the
  one-click unsubscribe that flips the email preference.
- **In scope:** signed/opaque token for `(project, recipient, target)`; inject
  `List-Unsubscribe` + `List-Unsubscribe-Post: One-Click` into outbound email; a public
  token-gated endpoint accepting POST (one-click) and GET (confirmation page) that disables
  the email medium pref for that target/recipient.
- **Out of scope:** none beyond the above.
- **Depends on:** Phases 2 (pref write path) + 4 (outbound email).
- **Done when:** clicking unsubscribe in a delivered email disables that target's email
  medium for that recipient, and subsequent sends skip email; headers pass Gmail/Yahoo
  one-click requirements.

```
Read agent-docs/overview.md in full first (esp. the Phase 2, 4, and 5 "deviations (as built)"
sections). Implement Phase 6 (Unsubscribe) as scoped.

Build on prior phases (reuse — do NOT re-derive):
- The preference this flips is the SAME per-medium recipient preference the authenticated
  console/API toggle already writes. Flip it via the existing write path
  `PreferenceService.UpdateRecipientPreferenceTarget(projectID, recipientExtID,
  PatchRecipientPreferenceTargetPayload{...medium: email, enabled: false})` — do NOT add a
  parallel disable path. After the flip, `fanOutEmail`'s existing gate
  (`preferenceRepo.ShouldDirectNotificationBeDelivered(..., enum.MediumEmail)` in
  `service/notification.go`) already returns false ⇒ subsequent sends record a `muted`
  delivery with `failure_reason=preference_disabled`. Verify that end-to-end.
- The token identifies `(project_id, recipient_external_id, target{channel,topic,event})` — all
  of which are already in hand inside `fanOutEmail` (`target := dto.TargetFromNotification(...)`).
  Sign it with **HMAC-SHA256 over `env.HashKey`** (`BODHVEDA_API_HASH_KEY`, already used to HMAC
  API-key tokens) — an opaque, self-contained, URL-safe token (base64url payload + sig), NOT the
  cipher key. No new DB table needed (the token carries its own claims; the endpoint re-derives
  and verifies). Reject tampered/expired tokens with 400/401.
- The public endpoint mounts EXACTLY like Phase 5's webhook: at the ROOT router in
  `cmd/api/routes.go`, OUTSIDE the developer API-key group (no `APIKeyBasedAuthMiddleware`, no
  permissive-CORS block) and OUTSIDE the console session group, and OUTSIDE the per-group
  `httprate` limiter — the token IS the auth. Suggested shape `GET|POST /unsubscribe/email?t=<token>`
  (project is inside the token, so no `{project_id}` path segment needed).
- Header injection happens on the outbound `email.Message`. Phase 4's `Message` struct
  (`internal/email/adapter.go`) has no headers field yet — add one (e.g. `Headers map[string]string`
  or explicit `ListUnsubscribe`/`ListUnsubscribePost`) and have the Resend adapter's `Send` pass it
  through to the Resend `headers` map. Build the token + URL in `fanOutEmail` (it has project/
  recipient/target) and carry it through `EmailDeliveryTaskPayload` → the worker → `Message`.

Then implement:
1. `List-Unsubscribe: <https://{API_BASE_URL}/unsubscribe/email?t=...>` +
   `List-Unsubscribe-Post: List-Unsubscribe=One-Click` on every outbound email (match these exact
   header values — they're what Resurface's digest.ts already sends and what Gmail/Yahoo one-click
   requires).
2. The public token-gated endpoint: **POST** = one-click (flip the pref off, return 200, no body
   needed) and **GET** = a minimal server-rendered confirmation page (flip + "you've been
   unsubscribed from <target> emails"). Both idempotent.
3. If a `complained` (spam) webhook event (Phase 5) can be wired to the same pref flip cheaply,
   do it here (Phase 5 explicitly deferred suppression to Phase 6 and left a NOTE in the webhook
   service). If it's more than a small hook, leave the NOTE and keep Phase 6 to explicit unsubscribe.

Broadcast pipeline stays untouched (email is DIRECT-only). SDKs stay un-bumped (Phase 7). Follow
the layered handler→service→pg pattern; Goose SQL migration applied manually IF any schema is
needed (likely none). Update Phase 6 status to DONE and add a "Phase 6 — deviations (as built)"
section recording the token format + signing key, the endpoint URL shape, and whether `complained`
was wired to suppression.
```

#### Phase 6 — deviations (as built)

No migration and no schema change — Phase 6 is stateless, exactly as scoped. The
token carries its own claims; the endpoint re-derives + verifies. Backend follows the
layered `handler → service → pg` split (the one new `pg` method is a read for the
complaint hook). `go build`/`go vet`/tests pass; the full unsubscribe loop and the
`complained`→suppression path were verified **live** against the running API + Postgres.

- **Token format + signing key.** Opaque, self-contained, URL-safe:
  `base64url(claimsJSON) + "." + base64url(HMAC-SHA256(claimsJSON, HashKey))`, signed with
  **`env.HashKey`** (`BODHVEDA_API_HASH_KEY` — the same key that HMACs API-key tokens, **not**
  the cipher key). Claims use short keys `{p:project_id, r:recipient_external_id, c/t/e:
  channel/topic/event, exp:unix_seconds}`. Medium is **not** in the token — this is the email
  unsubscribe surface, so email is implied. Lives in `internal/email/unsubscribe.go`
  (`BuildUnsubscribeToken`/`ParseUnsubscribeToken`/`UnsubscribeURL`, `UnsubscribeClaims`).
  **TTL = 180 days** (`unsubscribeTokenTTL`) — generous, since a recipient may unsubscribe
  from an old email. Tampered/malformed/wrong-key ⇒ `ErrUnsubscribeTokenInvalid` (→ **400**);
  well-signed-but-past-`exp` ⇒ `ErrUnsubscribeTokenExpired` (→ **401**). Signature compare is
  constant-time (`hmac.Equal`).
- **Endpoint URL shape: `GET|POST /unsubscribe/email?t=<token>`** (project is inside the token,
  so no `{project_id}` path segment). Mounted at the **root router in `cmd/api/routes.go`**,
  next to Phase 5's webhook and **OUTSIDE** the developer API-key group (no
  `APIKeyBasedAuthMiddleware`, no permissive-CORS block), the console session group, and the
  per-group `httprate` limiter — **the signed token IS the auth**. Both methods are idempotent
  (they upsert the same disabled recipient preference):
  - **POST** = RFC 8058 one-click (auto-POSTed by Gmail/Yahoo). Flips the pref, returns `200`
    (JSON success, no meaningful body). Errors return JSON (400/401) like the other dev-API
    surfaces.
  - **GET** = flips the pref **and** renders a minimal self-contained HTML confirmation page
    ("You've been unsubscribed from `<channel / topic / event>` emails"). Error cases render an
    HTML error page instead of JSON (a human is in a browser). User-supplied target text is
    `html.EscapeString`-escaped.
- **The flip reuses the existing authenticated write path — no parallel disable.**
  `service.UnsubscribeService.UnsubscribeEmail` parses the token, then calls
  `PreferenceService.UpdateRecipientPreferenceTarget(projectID, recipientExtID,
  {target, medium: email, enabled: false})` — the same upsert the console/API email toggle
  uses. Verified live: the redeemed token writes a recipient-level `preference` row
  `(digest/none/sent, medium=email, enabled=f)`, after which
  `ShouldDirectNotificationBeDelivered(..., email)` returns **false**, so subsequent direct
  sends record a `muted` delivery with `failure_reason=preference_disabled` (Phase 4 gate,
  unchanged). `UnsubscribeService` reads `env.HashKey` at construction.
- **Header injection on the outbound `email.Message`.** Phase 4's `Message`
  (`internal/email/adapter.go`) gained a generic **`Headers map[string]string`** (chosen over
  two explicit fields — future-proof, and the Resend adapter already has a natural
  passthrough). The Resend adapter's `resendSendRequest` gained
  `Headers map[string]string json:"headers,omitempty"` and passes it to the Resend `headers`
  map. On **every** outbound email the worker sets the two exact RFC 8058 headers
  Gmail/Yahoo one-click requires (matching what Resurface's `digest.ts` already sends):
  - `List-Unsubscribe: <https://{BODHVEDA_API_URL}/unsubscribe/email?t=...>`
  - `List-Unsubscribe-Post: List-Unsubscribe=One-Click`
- **Token/URL built on the send path, carried through the queue.** `fanOutEmail`
  (`service/notification.go`) has project/recipient/target in hand, so it builds the token +
  URL there (`buildUnsubscribeURL`, best-effort: a build error or missing `BODHVEDA_API_URL`
  ⇒ email still sends, just without the header) and carries the string through the new
  `EmailDeliveryTaskPayload.UnsubscribeURL` → the `email:delivery` worker → `Message.Headers`.
  The token is **not** rebuilt in the worker (no HashKey needed there).
- **New env var `BODHVEDA_API_URL`** wired into `internal/env/env.go` (`env.APIURL`) — it
  already existed in `.env`/`.env.example` (used by the console + Google OAuth URLs) but was
  never loaded by the Go side. Added to the `api` **and** `worker` service `environment:` blocks
  in `compose.yaml` (only `api` builds the token today; worker included for consistency).
- **`complained` (spam) WAS wired to suppression here** (Phase 5 left the NOTE; it turned out to
  be a small hook). On a `complained` webhook event, `EmailWebhookService` now looks up the
  delivery's recipient + target (new reader `NotificationDeliveryRepository.
  GetTargetByProviderMessageID`, a `notification_delivery ⋈ notification` join keyed by
  `provider_message_id`) and flips the **email** preference for that `(recipient, target)` off
  via the same `PreferenceService.UpdateRecipientPreferenceTarget` path — i.e. a spam complaint
  auto-unsubscribes, identical effect to an explicit unsubscribe. **Best-effort:** any error
  logs and never fails the webhook ack (the complaint is already recorded on the delivery row).
  This required injecting `*PreferenceService` into `NewEmailWebhookService` (app.go wiring
  updated). It is **target-scoped** (matching explicit unsubscribe); **address-level**
  suppression across *all* targets remains the old doc's `email_suppression` table, deferred to
  the managed-email tier. Verified live (the complaint flips `enabled=f` for the delivery's
  target). The old Phase 5 NOTE in `email_webhook.go` is replaced by the implementation.
- **Broadcast pipeline untouched** (email is DIRECT-only). **SDKs un-bumped** (Phase 7) — Phase 6
  adds no SDK surface: the unsubscribe header rides outbound email, and the public endpoint is
  hit by mail clients, not SDK callers.
- **Tests:** `internal/email/unsubscribe_test.go` (token round-trip, tampered signature, wrong
  key, malformed, expired, URL builder) and `internal/service/unsubscribe_test.go` (real-Postgres:
  cataloged email delivers → after unsubscribe it's muted, idempotent repeat, malformed rejected;
  gated on `TEST_DB_URL`, self-cleaning). The existing `internal/service/email_webhook_test.go`
  gained a `complained`→suppression assertion. The full HTTP surface (POST 200 one-click, GET
  HTML page, bad-token 400) was additionally driven **live** against the running API.

### Phase 7 — Release prep: Mintlify docs + SDK bump/README + publish runbook

- **Goal:** everything a downstream consumer (incl. the Phase 8 Resurface cutover) needs is
  written, versioned, and **ready to publish** — the published docs document the email medium,
  and the SDKs expose it with a bumped version + updated README. Publishing itself is a human
  step (see the runbook this phase produces).
- **In scope:**
  - `docs/` (Mintlify) — mediums concept, send API `email` block, recipient contacts,
    per-medium preferences, unsubscribe. (The ONLY phase that touches `docs/`; agent notes stay
    in `agent-docs/`.)
  - **SDKs** (`sdk/go`, `sdk/js/core` = npm `bodhveda`, `sdk/js/react` = `@bodhveda/react`):
    audit that email is fully exposed (types + client methods for the send `email` block,
    `deliveries[]` on the direct-send response, and recipient-contacts CRUD — much of this
    landed incrementally in Phases 1–6; this phase confirms parity & fills gaps), refresh each
    README, add/adjust a CHANGELOG note, and **bump versions** (JS is at `0.0.6`; Go versions
    via a `sdk/go/vX.Y.Z` git tag on the module subpath).
  - A **publish RUNBOOK** (the exact human steps: `npm publish` for both JS packages, the Go
    module git tag, and Mintlify deploy) committed to `agent-docs/`.
- **Out of scope:** actually running `npm publish` / pushing tags / deploying (the human does
  that from the runbook — publishes are irreversible & credential-gated); VPS/Cloudflare app
  deploy (Phase 7.5).
- **Depends on:** Phases 1–6 (documents + wraps shipped behavior).
- **Done when:** docs build (`mint`) and cover the whole email flow; SDKs build/typecheck with
  email exposed and versions bumped; the publish runbook is written; nothing is published yet.

```
Read agent-docs/overview.md in full first, esp. the Phase 1–6 "deviations (as built)" sections
— they are the source of truth. This is a RELEASE-PREP phase: you PREPARE and verify locally,
but do NOT publish or deploy anything (no `npm publish`, no `git push --tags`, no Mintlify
deploy) — those are irreversible/credential-gated human steps you instead write into a runbook.
Do NOT change backend behavior; SDK changes are limited to exposing already-shipped email
features. If code and docs disagree, the code wins — note it, don't "fix" it here.

PART A — Mintlify docs (docs/ only; NOT agent-docs/).
Know the structure: prose in `docs/docs/` (`introduction`, `quickstart`, `sdks`,
`concepts/{recipients,targets,preferences,notifications}.mdx`); nav in `docs/docs.json`; the API
Reference is **OpenAPI-driven** — each endpoint MDX is a thin stub
(`openapi: "POST /notifications/send"`) rendered from `docs/api-reference/openapi.json`, so
document request/response changes by editing **`openapi.json` FIRST**, not by hand-writing tables.
Add new pages to BOTH the file tree and docs.json. Document (all shipped in Phases 1–6):
1. New **Mediums** concept page (`docs/docs/concepts/mediums.mdx`, add to nav): in_app vs email;
   content-block-implies-intent (`payload` ⇒ in-app, `email` block ⇒ email eligible; no
   `mediums[]`, no payload→email fallback); **email is DIRECT-only, never broadcast**; each medium
   independently preference-gated.
2. **Send API `email` block** — add `email: {subject, html, text}` to the send request in
   `openapi.json` (no templating; caller supplies rendered html/text; `text` optional, derived
   from html); DIRECT-only (email on broadcast ⇒ 400). Update the send-notification MDX prose.
3. **Recipient contacts** — Phase 1's `recipient_contact` + contacts API is currently
   UNDOCUMENTED. Add the contacts endpoints (create/list/update/delete under
   `/recipients/{recipient_external_id}/contacts`, Phase 1 scope rules) to `openapi.json` + new
   endpoint MDX stubs, and a contacts section on `concepts/recipients.mdx` (email needs a primary
   email contact).
4. **Per-medium preferences** — update `concepts/preferences.mdx` + the preferences endpoints in
   `openapi.json` for the `medium` dimension (catalog + recipient opt-in/out are per (target, medium)).
5. **Unsubscribe** — short section: outbound email carries `List-Unsubscribe` one-click headers;
   Bodhveda hosts the unsubscribe which flips the recipient's email preference for that target off.
   Automatic — no dev-API endpoint to document. Delivery-status webhooks + console delivery overview
   are console/provider-facing (mention conceptually, not as a dev-API surface).

PART B — SDKs (expose already-shipped email; do NOT invent new API surface).
Audit all three packages for parity with the live API, then bump + document:
- `sdk/go` (module `github.com/MudgalLabs/bodhveda/sdk/go`): confirm the send call accepts the
  `email` block, the direct-send response exposes `deliveries[]`, and recipient-contacts CRUD
  exists (routes already in `sdk/go/routes/routes.go`; verify client methods + types in
  `types.go`). Update `sdk/go/README.md`. Go is versioned by a git tag `sdk/go/vX.Y.Z` — do NOT
  create the tag (runbook), but decide the version and put it in the README/CHANGELOG.
- `sdk/js/core` (npm `bodhveda`, currently `0.0.6`): it already has `contacts.*`, `send()` email,
  and `deliveries` in `src/types.ts` — verify end-to-end, update `README.md`, bump `package.json`
  version, `npm run build` to refresh `dist/`.
- `sdk/js/react` (`@bodhveda/react`, `0.0.6`): bump in lockstep if it re-exports changed core
  types; update README if it surfaces email.
- Pick ONE coherent version bump across JS (e.g. 0.0.6 → 0.1.0 since this is the email feature)
  and a matching Go tag version; record it. Add a short CHANGELOG entry per package.

PART C — publish runbook.
Write `agent-docs/release-email-medium.md` — the exact human publish steps IN ORDER:
`cd sdk/js/core && npm publish`; `cd sdk/js/react && npm publish`; the Go module tag
(`git tag sdk/go/vX.Y.Z && git push origin sdk/go/vX.Y.Z`); and how the Mintlify site deploys
(confirm whether docs.bodhveda.com auto-deploys from git or needs `mint deploy` — check
docs/docs.json / any Mintlify config and state which). Note npm needs `npm login` and that npm
un-publish is restricted — versions are effectively permanent. This runbook feeds Phase 7.5.

Verify locally: `mint` builds the docs & nav resolves; `go build ./...` in sdk/go; `npm run build`
in both JS packages; openapi.json stays valid JSON. Commit everything. Update Phase 7 status to
DONE and add a "Phase 7 — deviations (as built)" section listing the docs pages/nav/openapi ops
added, the SDK gaps found+filled, the chosen version numbers, and the runbook path. Do NOT publish.
```

#### Phase 7 — deviations (as built)

Release-prep only: **nothing was published or deployed** (that's Phase 7.5, from the runbook).
No backend behavior changed. `go build ./...` (sdk/go), `npm run build` (both JS packages), and
`mint broken-links` (docs) all pass; `openapi.json` + `docs.json` stay valid JSON.

**Chosen versions.** JS core `bodhveda` **0.0.6 → 0.1.0**; JS react `@bodhveda/react`
**0.0.6 → 0.1.0** (its `bodhveda` dep bumped `^0.0.5 → ^0.1.0` so it resolves the new core). Go
SDK is tagged, not un-versioned as the prompt assumed — the latest tag is **`sdk/go/v0.1.9`**,
so the next is **`sdk/go/v0.2.0`** (the tag is created by the human in the runbook, not here). A
coherent minor bump everywhere for the additive email feature.

**PART A — Mintlify docs (`docs/`).**
- **New concept page** `docs/docs/concepts/mediums.mdx` (added to `docs.json` Concepts nav):
  in_app vs email, content-block-implies-intent (`payload` ⇒ in-app, `email` ⇒ email eligible;
  no `mediums[]`, no payload→email fallback), email DIRECT-only, per-medium gating (catalog +
  preference + primary contact), and the automatic `List-Unsubscribe` behavior.
- **`openapi.json` (edited first, MDX renders from it):**
  - `SendNotificationPayload` gained an `email` prop → new **`EmailContent`** schema
    (`subject` required, `html`/`text`, direct-only note); `SendNotificationResponse` gained
    **`deliveries[]`** → new **`NotificationDelivery`** schema; added a fan-out response example
    + an email-block curl code sample.
  - **Contacts endpoints (were UNDOCUMENTED):** `GET`/`POST` `/recipients/{recipient_id}/contacts`
    and `PATCH`/`DELETE` `/recipients/{recipient_id}/contacts/{contact_id}`, tagged `Contacts`,
    with `RecipientContact` / `CreateRecipientContactPayload` / `UpdateRecipientContactPayload`
    schemas. Scope rules encoded via `security` (POST/GET/PATCH either-scope, DELETE full-scope).
  - **Per-medium preferences:** `medium` added to the set-preference request body, the
    check-preference query params, and the list/set/check response target examples
    (default `in_app`). Also corrected the stale set-preference example (`state.subscribed` →
    `state.enabled`) while there.
- **MDX stubs:** `docs/api-reference/endpoint/recipients/contacts/{list,create,update,delete}-contact.mdx`
  (thin `openapi:` stubs), added to `docs.json` under a new **Contacts** group.
- **Prose updates:** `concepts/recipients.mdx` (Contacts section), `concepts/preferences.mdx`
  (per-medium section), `concepts/notifications.mdx` (email fan-out note), and the
  send-notification endpoint MDX ("Delivering email" section).
- **Left as-is (noted, not fixed):** `mint broken-links` reports two PRE-EXISTING broken links in
  `docs/quickstart.mdx` (`/docs/concepts/target`, `/docs/concepts/introduction`) — unrelated to
  email, out of this phase's scope. All new email pages/links resolve.

**PART B — SDKs.** The three enumerated items (send `email` block, `deliveries[]`,
recipient-contacts CRUD) were **already present** in both JS core (`src/types.ts` +
`recipients.contacts.*`) and Go (`types.go` + `client.Recipients.Contacts.*`) from Phases 1/4 —
parity confirmed. **Gap found + filled: per-medium preferences were not exposed.** The preference
`set`/`check` calls silently dropped `medium` (JS `set` posted only `{target, state}`; Go/JS
`check` sent only the target), so email preferences (needed for the Phase 8 Resurface cutover)
were unreachable via SDK. Added an optional `medium` to the preference set/check requests + the
preference response target types in both SDKs (JS `PreferenceMedium = "in_app" | "email"`; Go
`MediumInApp` constant), defaulting to `in_app` — additive and backward-compatible. READMEs got
Email-send, Recipient-Contacts, and per-medium-preference sections; each package got a
`CHANGELOG.md`. React re-exports the changed core types (`export * from "bodhveda"`) so it's
bumped in lockstep with a short README note (no new hooks — email/contacts are server-side).
- **Noted, NOT fixed (out of scope):** the Go SDK's `RecipientsNotifications.UnreadCount` route
  uses `/notifications/unread_count` while the live API is `/notifications/unread-count` — a
  pre-existing, non-email bug. Left per "code wins; don't fix unrelated things here."

**PART C — runbook.** `agent-docs/release-email-medium.md` — ordered human steps: publish
`bodhveda` (core) first, then `@bodhveda/react`, then `git tag sdk/go/v0.2.0 && git push`, then
the docs. **Docs deploy = Mintlify GitHub App auto-deploy from `main`** (confirmed: there is NO
docs job in `.github/workflows/deploy.yml` and no `mint` config beyond `docs.json`); the runbook
gives the dashboard-confirm path + `mint deploy` fallback. Notes `npm login` and that npm
un-publish is effectively permanent.

**Verification notes.** React's `node_modules` was absent and its `bodhveda` dep now points at
the unpublished `0.1.0`; verified its build by installing the **local** core
(`npm install --no-save ../core …`) — package.json/lockfiles untouched by that. The runbook's
core-before-react ordering is what makes a clean `npm install` work at publish time.

<!-- retained detailed docs-only prompt fragment below for reference; superseded by the release-prep prompt above -->
<details><summary>Earlier docs-only Phase 7 prompt (superseded)</summary>

```
Read agent-docs/overview.md in full first, esp. the Phase 1–6 "deviations (as built)" sections
— they are the source of truth for what to document. This phase touches ONLY docs/ (the public
Mintlify site). Do NOT touch agent-docs/, and do NOT change any Go/console/SDK code — you are
documenting SHIPPED behavior, not adding features. If reality and the docs disagree, the code
wins; note the discrepancy rather than "fixing" it here.

Know the docs structure before editing:
- Prose lives in `docs/docs/` (`introduction`, `quickstart`, `sdks`, and `concepts/{recipients,
  targets,preferences,notifications}.mdx`). Nav is `docs/docs.json` (the "Documentation" tab's
  Concepts group + the "API Reference" tab's groups). Add new pages to BOTH the file tree and
  docs.json.
- The API Reference is **OpenAPI-driven**: each endpoint MDX is a thin stub with frontmatter
  `openapi: "POST /notifications/send"` that renders from `docs/api-reference/openapi.json`. So
  documenting new/changed request or response fields means **editing `openapi.json` FIRST**, then
  the MDX renders it. Match this pattern — do not hand-write request tables in MDX.

Document the email medium (all shipped in Phases 1–6):
1. **New "Mediums" concept page** (`docs/docs/concepts/mediums.mdx`, add to nav): in_app vs email;
   content-block-implies-intent (a `payload` block ⇒ in-app, an `email` block ⇒ email eligible;
   no `mediums[]` array, no payload→email fallback); **email is DIRECT-only, never broadcast**;
   each medium is independently gated by preferences.
2. **Send API `email` block** — add `email: {subject, html, text}` to the send-notification request
   in `openapi.json` (Bodhveda does NO templating; caller supplies rendered html/text; `text` is
   optional and derived from html if omitted). Note the DIRECT-only rule (email on a broadcast is
   a 400). Update `docs/api-reference/endpoint/notifications/send-notification.mdx` prose.
3. **Recipient email contacts** — Phase 1 shipped a `recipient_contact` table + a contacts API that
   is **currently UNDOCUMENTED**. Add the recipient contacts endpoints (create/list/update/delete
   under `/recipients/{recipient_external_id}/contacts`, scope rules per Phase 1) to `openapi.json`
   + new endpoint MDX stubs under `docs/api-reference/endpoint/recipients/contacts/`, and a
   contacts section on `concepts/recipients.mdx` (a recipient needs a primary email contact to get
   email).
4. **Per-medium preferences** — update `concepts/preferences.mdx` + the preferences endpoints
   (`openapi.json`) for the `medium` dimension (catalog entries are per (target, medium); recipient
   opt-in/out is per (target, medium)).
5. **Unsubscribe** — a short section (on the mediums or preferences page): every outbound email
   carries `List-Unsubscribe` one-click headers and Bodhveda hosts the unsubscribe; it flips the
   recipient's email preference for that target off (same effect as toggling the preference). This
   is automatic — there's no dev-API endpoint to document (the public `/unsubscribe/email` route is
   hit by mail clients, not SDK callers). Delivery-status webhooks + the console delivery overview
   (Phase 5) are console/provider-facing; mention delivery statuses conceptually but they are not a
   developer-API surface.

Match the existing MDX voice/style. Verify the docs build (mint dev / mint build) and the nav
resolves. Keep openapi.json valid. Update Phase 7 status to DONE and add a short "Phase 7 —
deviations (as built)" section listing exactly which pages/nav entries + openapi.json operations
were added or changed (and note the contacts API was newly documented, not just email).
```

</details>

### Phase 7.5 — Deploy email medium to VPS + Cloudflare, verify live

- **Goal:** the email medium is running on the LIVE Bodhveda (api.bodhveda.com + worker) and the
  live Console, verified end-to-end, so the Phase 8 Resurface cutover can point at production
  instead of a local dev instance.
- **In scope (human-executed, this doc guides):**
  - Apply the email-medium migrations to the **production** DB (goose, manual — no runner is
    wired): the Phase 1–6 migrations, notably `recipient_contact`, `project_email_settings`
    (+ `webhook_secret`), `notification_delivery`, and preference `medium`.
  - Set the new **`BODHVEDA_API_URL`** env var on the prod api **and** worker (Phase 6 added it to
    `compose.yaml`; the VPS `.env` must define it — it builds the unsubscribe link).
  - Ship the API/worker image: merging to `main` triggers `.github/workflows/deploy.yml`
    (build+push `bodhveda_api` image → SSH deploy). Confirm the **worker** picks up the new image
    too (compose `deploy` overlay), since `email:delivery` runs there.
  - Deploy the **Console** to Cloudflare (separate from deploy.yml — see
    [[project-console-cloudflare-deploy]]; fresh `npm ci` means a broken lockfile only surfaces here).
  - Publish the SDKs + docs from the **Phase 7 runbook** (`agent-docs/release-email-medium.md`) if
    not already done.
- **Out of scope:** any code changes (this is deploy + verify only; a bug found here loops back to
  the owning phase).
- **Depends on:** Phase 7 (docs/SDK ready + runbook).
- **Done when:** against the LIVE instance, a real project can configure Resend email settings,
  a direct send with an `email` block delivers a real email, the Resend delivery webhook flips the
  delivery row to `delivered`, and the one-click unsubscribe link flips the pref (subsequent sends
  go `muted`). Record the results.

```
Read agent-docs/overview.md in full first (esp. Phase 3–6 deviations + the Phase 7 runbook
`agent-docs/release-email-medium.md`). This is a DEPLOY + VERIFY phase — no code changes. Your job
is to guide/execute the production rollout and then prove the email medium works live. Anything
irreversible or credential-gated (prod DB migration, prod env edits, merge-to-main that triggers
the CI deploy, Cloudflare deploy, npm/tag publish) is confirmed with the human before running; you
prepare exact commands and a checklist.

1. DB migration (prod): the app has NO migration runner — migrations are applied manually with
   goose. List every email-medium migration under migrations/ that must be applied to the
   production DB (recipient_contact, project_email_settings incl. the webhook_secret column,
   notification_delivery, preference medium) and give the exact `goose -dir migrations postgres
   "$PROD_DB_URL" up` invocation. Have the human run it (or run against a prod DB URL they supply).
2. Env: `BODHVEDA_API_URL` is newly read by the Go side (Phase 6) for the unsubscribe link and is
   in compose.yaml for api+worker — confirm it's set in the VPS `.env` (value = the public API
   URL, e.g. https://api.bodhveda.com). Flag any other new env the email medium needs.
3. Ship api+worker: merging to `main` fires `.github/workflows/deploy.yml` (builds+pushes the
   `bodhveda_api` image, SSH-deploys via `docker compose -f compose.yaml -f compose.deploy.yaml`).
   Confirm the **worker** service is redeployed on the new image too (it runs `email:delivery`),
   and that migrate/asynqmon behave as expected in prod (asynqmon is dev-only — must stay absent).
4. Console → Cloudflare: deploy the console separately (not deploy.yml). Watch for lockfile drift
   surfacing only under Cloudflare's fresh `npm ci`.
5. Publish SDKs + docs per the Phase 7 runbook if not already done.
6. VERIFY LIVE (the real point): on the live instance, create/pick a project, set Resend email
   settings (real key + verified from-domain), configure the Resend webhook to
   `https://<api>/webhooks/email/<project_id>` with the signing secret, register a recipient email
   contact, catalog + opt-in the target for the email medium, then send a DIRECT notification with
   an `email` block. Confirm: the email arrives; `notification_delivery` goes pending→sent→delivered
   (webhook); the email's List-Unsubscribe one-click flips the pref and a resend records `muted`.
   Capture the outcomes.

Update Phase 7.5 status to DONE and add a "Phase 7.5 — deviations (as built)" section recording
what was migrated/deployed, the live verification results (with the project id used), and anything
that had to be fixed (looping the fix back to its owning phase). Then Phase 8 (Resurface cutover)
can target the live instance + published SDK.
```

### Phase 8 — Resurface cutover (the final end-to-end test)

- **Goal:** Resurface drops Resend entirely and uses Bodhveda for both in-app and email;
  proves the whole medium works.
- **In scope (in `../resurface`):** remove the Resend dependency + `cron/src/digest.ts`
  direct send + its react-email render path (or keep the template but pass its html/text to
  Bodhveda's `email` block); remove `signUnsubscribeToken` + local `List-Unsubscribe`;
  register the user's email as a Bodhveda `recipient_contact` server-side on `/me`; migrate
  `emailDigestEnabled` /
  `inAppDigestEnabled` to Bodhveda preferences (keep the `isPro` entitlement gate in
  Resurface); one `notifications.send({ target: digestSent, payload, email })` fans out to
  inbox + email.
- **Depends on:** Phases 1–6, **and Phase 7 + 7.5** (Resurface pulls the published SDK, follows
  the live docs, and talks to the DEPLOYED Bodhveda — not a local dev instance).
- **Done when:** a digest run sends both the in-app bell notification and the email through
  Bodhveda only, unsubscribe works from the email, and no `RESEND_*` remains in Resurface.

```
Read agent-docs/overview.md in full first (esp. the "Validation target: Resurface" section).
Implement Phase 8 (Resurface cutover) in ../resurface: remove the Resend integration
(cron/src/digest.ts direct send, signUnsubscribeToken, RESEND_* env), register the user's
email as a Bodhveda recipient_contact server-side on /me, migrate the email/in-app digest
prefs to Bodhveda per-medium
preferences while keeping the isPro entitlement gate in Resurface, and replace the dual send
with a single Bodhveda notifications.send carrying both payload (in-app) and the email block.
Render the existing @react-email template to html/text and pass it. Verify a digest run emits
both the in-app notification and the email via Bodhveda only, and that email unsubscribe works.
Update Phase 8 status to DONE when finished.
```
