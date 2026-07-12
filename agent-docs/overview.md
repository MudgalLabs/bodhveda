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

`make up` runs **asynqmon** on `:7755` (dev-only, absent from prod on purpose). The
worker is **not** started by `make dev` — run `go run ./cmd/worker` from `api/` to
exercise jobs.

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
- Phase 4 — Email delivery core (adapter + `email:delivery` worker + `notification_delivery` + send `email` block; DIRECT-only) — **TODO**
- Phase 5 — Delivery status via Resend webhooks — **TODO**
- Phase 6 — Unsubscribe (List-Unsubscribe header + public endpoint) — **TODO**
- Phase 7 — Public docs (Mintlify) for the email medium — **TODO**
- Phase 8 — Resurface cutover (the final end-to-end test) — **TODO**

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
DIRECT-only, and the notification_delivery DDL in design/multi-medium-delivery.md). Implement
Phase 4 (Email delivery core, DIRECT-only) as scoped: a medium adapter interface + a Resend
adapter; a typed sibling `email: {subject, html, text}` block on the send API where presence
means "email eligible" and absence means no email (NO payload fallback); the
notification_delivery table used for email only (leave in_app status on the notification row);
direct-send fan-out that, after the inbox write, gates email on catalog + per-medium
preference + a primary email contact, then enqueues an `email:delivery` Asynq task whose
processor sends via Resend and records the delivery row; partial-medium failures return 200
with per-delivery statuses. Do NOT send email on broadcasts (forbidden) — leave the broadcast
pipeline untouched. Do NOT build webhooks (Phase 5) or unsubscribe (Phase 6). Note the
email-metering decision. Update Phase 4 status to DONE and record the delivery-record schema.
```

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
Read agent-docs/overview.md in full first. Implement Phase 5 (Delivery status via provider
webhooks) as scoped: a public webhook ingestion endpoint that verifies Resend events and maps
them, via the adapter's normalization layer, to status transitions on the per-(notification,
medium) delivery record; surface per-medium delivery status/analytics in the console. Keep
normalization inside the adapter interface so other providers slot in later. Do NOT implement
unsubscribe (Phase 6). Update Phase 5 status to DONE when finished.
```

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
Read agent-docs/overview.md in full first. Implement Phase 6 (Unsubscribe) as scoped: a
signed/opaque token identifying (project, recipient, target); inject List-Unsubscribe +
List-Unsubscribe-Post: One-Click headers into outbound email; host a public token-gated
endpoint (POST one-click + GET confirmation page) that flips the email-medium preference off —
the same preference the authenticated toggle controls. Update Phase 6 status to DONE when
finished.
```

### Phase 7 — Public docs (Mintlify)

- **Goal:** the published docs site documents the email medium.
- **In scope:** `docs/` (Mintlify) updates — mediums concept, the send API `email` block,
  recipient email, per-medium preferences, unsubscribe behavior; `docs.json` nav; API
  reference. (This is the ONLY phase that touches `docs/`; agent notes stay in `agent-docs/`.)
- **Depends on:** Phases 1–6 (documents shipped behavior).
- **Done when:** docs build and cover the whole email flow.

```
Read agent-docs/overview.md in full first. Implement Phase 7 (Public docs) as scoped: update
the Mintlify site under docs/ (NOT agent-docs/) to document the email medium — mediums concept,
send API `email` block, recipient email, per-medium preferences, and unsubscribe — including
docs.json nav and API reference. Match the existing MDX style under docs/docs and
docs/api-reference. Update Phase 7 status to DONE when finished.
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
- **Depends on:** Phases 1–6 (7 optional to precede).
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
