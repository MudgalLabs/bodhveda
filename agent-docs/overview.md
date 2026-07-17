# Bodhveda — Engineering Overview

> Internal architecture reference written by/for AI coding agents. Lives in `agent-docs/`
> (separate from `docs/`, which is the published Mintlify site). Kept here so future work
> has a single place that explains what the repo is, how it's structured, what exists
> today, and the design decisions we've committed to. Update it when the architecture or a
> decision changes.
>
> **This describes what is BUILT.** Phases 0–8 (the email medium) are shipped, live, and
> validated in production; their plans, hand-off prompts, and as-built deviation notes were
> removed once done — `git log` has them if you need the archaeology. What those phases
> established that still matters is folded into the sections below. The only work still in
> flight is the **Phase 9 console arc** at the end.

## What Bodhveda is

An open-source (AGPLv3) notification platform. A customer (developer) creates a
**project**, gets an **API key**, and sends **notifications** to their **recipients**.
Recipients read notifications through an inbox-style API, and per-recipient
**preferences** gate what actually gets delivered.

**Current state:** Bodhveda delivers over **two transports** — the in-app inbox and
**email** (via the customer's own Resend account). Email is shipped, deployed, and
validated end-to-end in production: Resurface (see "Validation target" below) routes all
of its notifications through one Bodhveda send per user, and dropped its own Resend
integration entirely.

Three things about "delivery" that trip people up when reading the code:

- **In-app "delivery" means *persist to the inbox*** — insert a row into `notification`
  that the recipient later pulls via `GET /recipients/{id}/notifications`. Nothing leaves
  the system. Its outcome is the `status` scalar **on the `notification` row**.
- **Email delivery is a real wire send**, and its outcome lives in a **separate
  `notification_delivery` row**, not on the notification. The two can disagree per
  notification (in-app `delivered`, email `muted`), and the console renders both.
- **Email is DIRECT-only. Broadcasts are in-app only** — a hard rule, see the decision log.

## Repo layout (monorepo)

- `api/` — Go backend. Two binaries: `cmd/api` (chi HTTP server, `:1338`) and
  `cmd/worker` (Asynq worker). All logic under `internal/`.
- `console/` — React 19 + Vite + TanStack Router/Query. Dev on `:6970`. Deploys to
  Cloudflare separately from the API.
- `sdk/go/`, `sdk/js/` — SDKs (`sdk/js/core` publishes as `bodhveda`, plus a `react` pkg).
- `migrations/` — Goose SQL migrations. **No runner is wired in** — apply manually with
  `goose -dir migrations postgres "$BODHVEDA_DB_URL" up`. (On goose ≥ v3.26 the CLI wants
  `GOOSE_DRIVER` / `GOOSE_DBSTRING` env vars: `GOOSE_DRIVER=postgres GOOSE_DBSTRING=… goose
  -dir migrations up`.)
- `docs/` — Mintlify site (`docs.json` + MDX under `docs/docs` and `docs/api-reference`).
- `design/multi-medium-delivery.md` — an earlier, pre-BYO design doc. Its SES/reputation/
  suppression apparatus was **deferred, not discarded**: it is the blueprint for a future
  managed-email tier. Do not execute its SES phases; today's decisions (below) supersede it
  for v1.
- `compose.yaml` (base, incl. dev-only console + asynqmon) and `compose.deploy.yaml`
  (prod overlay overriding `image:` on api/worker/migrate).
- `agent-docs/release-email-medium.md`, `agent-docs/release-preference-read-fix.md` —
  publish runbooks (human-executed; npm + Go module tags + Mintlify).

## Backend layering (`api/internal/`)

Strict `handler → service → repository`, wired in `internal/app/app.go` (`APP` singleton
holds DB pool, Asynq client, services, repos).

- `handler/` — chi handlers; decode request, call service, respond via tantra `httpx`.
- `service/` — business logic. Constructors take repos + cross-service deps + Asynq client.
- `pg/` — pgx repository implementations of interfaces in `model/repository/`.
- `model/` — `entity/` (DB rows/domain), `dto/` (request/response), `enum/` (string enums
  + typed errors in `enum/error.go`), `repository/` (interfaces only).
- `email/` — the medium **adapter interface** + the Resend adapter (send, webhook signature
  verification, provider-event normalization) + the unsubscribe token.
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

## Three routing surfaces (`cmd/api/routes.go`)

1. **Developer API** — `Authorization: Bearer <api key>`, permissive CORS (`*`), no
   credentials, 100 req/min/IP (`httprate`). API keys have a `scope`:
   - `full` — can send + do everything; gates `/notifications/send` and recipient CRUD
     via `VerifyAPIKeyHasFullScope`.
   - `recipient` — inbox/preferences/own-contacts only, can't send.
     `/recipients/{recipient_external_id}/…` auto-creates the recipient via
     `CreateRecipientIfNotExists` — so on that surface **the recipient is guaranteed to
     exist** and a 404 branch there is unreachable code.
2. **Console API** — `/console/...`, cookie session (scs/pgxstore), strict CORS to
   `BODHVEDA_WEB_URL` with credentials. Project routes nested under `{project_id}`,
   gated by `VerifyUserOwnsThisProject`.
3. **Public, token/signature-authenticated** — mounted at the **root router**, OUTSIDE
   both groups above and outside the per-group rate limiter, because they are called by
   mail providers and mail clients rather than customers:
   - `POST /webhooks/email/{project_id}` — Resend delivery-status webhooks. **Auth IS the
     Svix signature.**
   - `GET|POST /unsubscribe/email?t=<token>` — RFC 8058 one-click unsubscribe. **Auth IS
     the signed token.**

Handlers/services are shared where sensible (e.g. `Notification.List` vs
`.ListForRecipient`) — but see the ⚠️ in the Console section: those two are **not**
interchangeable.

## The core domain model

### Notification = Target + payload

A notification carries a **`Target` = {channel, topic, event}** plus a free-form JSON
`payload` (16 KB cap, `enum.NotificationMaxPayloadSize`).

> ⚠️ **`Target.Channel` is a categorization label, not a transport medium.** Examples:
> `channel="posts", topic="post_123", event="new_comment"` or
> `channel="announcements", topic="none", event="new_feature"`. This is why "channel"
> is unavailable as a name for email/push transports — hence "medium".
>
> `topic` reserved words: `any` (preferences only — matches all topics in a channel)
> and `none` (rule has no topic). A send `Target` may use `none` but never `any`.

Two send modes (`SendNotificationPayload`, dispatched in `service.NotificationService.Send`):

- **Direct** — `recipient_id` set. Creates one `notification` row (status `enqueued`),
  enqueues `notification:delivery`, and **may also fan out to email** (below).
- **Broadcast** — no recipient, requires a matching **project preference** to exist
  (else 400). Creates a `broadcast` row, enqueues `broadcast:prepare_batches`.
  **In-app only** — an `email` block on a broadcast is a 400.

Notification statuses (`enum`): `enqueued`, `muted`, `delivered`, `quota_exceeded`,
`failed`. Broadcast: `enqueued`, `completed`, `quota_exceeded`, `failed`. This scalar is
the **in-app** outcome only; every other medium's outcome is a `notification_delivery` row.

### Mediums

A **medium** is a delivery transport. `enum/medium.go`:

- `Valid()` — all five (`in_app`, `email`, `sms`, `web_push`, `mobile_push`); matches the
  `preference.medium` CHECK. The non-active ones are scaffolding so the enum, contacts
  table and preference catalog need no re-migration when web-push lands.
- `Active()` / `ActiveMediums()` — **`in_app` + `email`** — the only transports that fire
  in v1. Catalog creation is restricted to these, and the console only renders toggles for
  these.
- `ValidContactMedium()` — email/sms/web_push/mobile_push. **`in_app` is excluded**: its
  "address" is the `recipient_external_id`.
- `DefaultMedium = in_app` — an omitted `medium` on any preference API call means in-app,
  which is what keeps older SDKs working unchanged.

**How a medium fires — all three must hold:**

1. **Sender intent = presence of that medium's content block.** `payload` ⇒ in-app;
   an `email: {subject, html, text}` sibling block ⇒ email is eligible. **No `email` block
   ⇒ no email**, and there is **no payload→email fallback**.
2. **Catalog** — see the ⚠️ under Preferences: the catalog is a *default*, not a gate.
3. **Preference** — the per-medium recipient preference must not disable it.

### Recipients + contacts

Addressed externally by a customer-chosen `external_id` string (**stored lowercase**, so
any filter or lookup keyed on one must lowercase too), never the internal serial `id`.
Recipient-scoped routes use `{recipient_external_id}`.

A recipient's contact addresses live in **`recipient_contact`** — keyed
`(project, recipient, medium, address)`, with `is_primary` + `verified_at`, a medium CHECK
of `email|sms|web_push|mobile_push`, and a partial unique index
(`ux_recipient_contact_one_primary`) enforcing one primary per `(recipient, medium)`. Only
`email` is exercised in v1; the table is deliberately future-proof rather than a bare
`email` column (web-push is the next medium). **A recipient with no primary email contact
gets a `no_contact` delivery outcome, not an error.**

- Email addresses are trimmed + lowercased; other mediums' addresses are only trimmed
  (push tokens are case-sensitive). PATCHing an address to a *different* value nulls
  `verified_at`.
- Dev API scope: POST/GET/PATCH allowed for full **or** recipient-self scope; **DELETE is
  full-scope only**. POST is 409-on-conflict, not idempotent.
- ⚠️ **A literal `/` in an `external_id` fails safe but does not work.** chi routes on the
  raw path, but `chi.URLParam` returns it still-encoded (`a%2Fb`), which misses the DB
  lookup ⇒ 404. Every other hostile character (`+`, space, `#`, `?`, `%`, `@`) round-trips
  correctly. Affects every recipient-scoped route on both surfaces. Recorded, not fixed —
  fixing means `url.PathUnescape` on every recipient route.

### Preferences (`preference` table) — two levels × medium

- **Project-level** (`recipient_external_id NULL`, `label NOT NULL`) — the **catalog** of
  subscribable `(target, medium)` pairs. A broadcast requires one to exist for its target.
- **Recipient-level** (`recipient_external_id NOT NULL`, `label NULL`) — per-recipient
  opt-in/opt-out per medium.
- Uniqueness is enforced by two partial unique indexes, both with **`medium` appended**
  (`recipient_pref_unique`, `project_pref_unique`); a CHECK enforces the label/recipient
  XOR. Those indexes are why the set-based resolver below can use LEFT JOINs safely: each
  cascade rung matches ≤1 row.

**Resolution cascade** (`pg/preference.go`) — the authority on what a send does:

```
recipient-exact → recipient-fallback (topic='any') → project-exact → project-fallback → default
```

> ⚠️ **The default is MEDIUM-DEPENDENT, and the catalog is a DEFAULT, not a GATE.** These
> two facts are the source of a whole family of bugs; both were measured against the real
> gating SQL, twice.
>
> - `in_app` defaults to **deliver** (legacy "deliver unless muted"; no catalog required).
>   Every other medium defaults to **not** deliver — it fires only when cataloged or
>   explicitly enabled. *That default IS the catalog gate for non-in_app transports.*
> - An **explicit recipient row wins before the catalog is ever consulted**. So:
>
>   | situation | resolves to |
>   |---|---|
>   | uncataloged `(T, in_app)`, no recipient row | **delivers** (in_app default is true) |
>   | uncataloged `(T, email)`, no recipient row | does not deliver |
>   | uncataloged `(T, email)`, recipient row `enabled=true` | **delivers** |
>
>   "Uncataloged ⇒ unavailable" is a **lie**. Only the resolved value is honest. The
>   console renders `cataloged` as context, never as a gate.

**Two SQL resolvers of this one cascade exist, deliberately:**

- `ShouldDirectNotificationBeDelivered(…, medium)` — single-cell, on the **send hot path**.
  Plus `DoesProjectPreferenceExist` and `ListEligibleRecipientExtIDsForBroadcast` (both
  called with `enum.MediumInApp` — broadcasts are in-app only).
- `ResolveRecipientPreferences(…)` / `ResolveRecipientPreferenceForTargets(…, targets)` —
  **set-based**, answering every `(target × active medium)` in one round trip with
  `enabled` + `inherited` + `cataloged` + `source`. Powers both the console grid and the
  Dev API preference read/check. Its target universe is **the catalog UNION the recipient's
  own rows** — that union is what makes an uncataloged-but-explicitly-enabled row visible.
- ⚠️ **They are held in step by `TestResolveRecipientPreferencesAgreesWithGating`**, which
  asserts cell-for-cell that the resolver equals the gating query. Change one and not the
  other and it fails — that is the whole point, and it is why a prose comment claiming two
  functions agree is not good enough. (A comment exactly like that *did* go quietly false
  when the default became medium-dependent, and shipped a bug for months.)

**Write path — one convergence point.** Two service entry points exist and are both
legitimate: `UpsertRecipientPreference` (console `PUT`, flat payload, 201) and
`UpdateRecipientPreferenceTarget` (Dev API `PATCH` / unsubscribe / complaint-suppression,
nested, 200). **Both call `repo.Create`**, the recipient-level upsert. That convergence is
*why* the app's settings toggle and the email one-click unsubscribe stay in sync. Do not
add a third path, and do not "unify" the two.

### `notification_delivery` — one row per (notification, medium)

**Email-only in v1.** In-app state stays on the `notification` row; the old design doc's
in_app backfill / dual-write / column-drop was deliberately NOT done, and there is still no
reason to do it.

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
    provider_message_id     TEXT,                 -- correlates inbound webhooks
    provider_response       JSONB,                -- APPENDED array of raw webhook bodies; unbounded
    failure_reason          TEXT,                 -- not_cataloged / preference_disabled / provider_not_configured / …
    attempt                 INT NOT NULL DEFAULT 0,
    sent_at                 TIMESTAMPTZ,
    delivered_at            TIMESTAMPTZ,
    bounced_at              TIMESTAMPTZ,
    complained_at           TIMESTAMPTZ,
    opened_at               TIMESTAMPTZ,          -- soft signal
    clicked_at              TIMESTAMPTZ,          -- soft signal
    read_at                 TIMESTAMPTZ,          -- in_app only; unused in v1
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (notification_id, medium)
);
```

**Indexes** (`migrations/20260713120000_add_notification_delivery.sql`):
`ix_nd_notification (notification_id)`,
`ix_nd_project_recipient (project_id, recipient_external_id, created_at DESC)`,
`ux_nd_provider_message (medium, provider_message_id) WHERE provider_message_id IS NOT NULL`,
and `ix_nd_email_status_time`.

> ⚠️ **`ix_nd_email_status_time` is MISNAMED — it has no `status` column.** It is
> `(project_id, created_at DESC) WHERE medium='email'`. Do not reach for it to serve a
> delivery-status predicate; it cannot. (Measured in 9.4. The console's email filter is
> keyed by `notification_id` and is served by `ix_nd_notification`.)

**`enum.DeliveryStatus`** (matches the CHECK). Set in v1: `pending` (enqueued), `sent`
(provider accepted — the success terminal absent webhooks), `delivered` / `bounced` /
`complained` (from webhooks), `failed`, `muted` (preference/catalog disallows —
`failure_reason` distinguishes `not_cataloged` from `preference_disabled`), `no_contact`.
**Reserved and never written:** `sending`, `suppressed`, `quota_exceeded`, `rejected` — so
a UI must not offer them as filters.

## Delivery pipeline (Asynq, `cmd/worker`)

API enqueues, worker consumes. Task types (`job/task/task.go`):

- `notification:delivery` — one direct send's inbox write. `ShouldDirectNotificationBeDelivered`
  → if muted, status `muted`; else `billingService.CheckAndConsumeUsage` (`quota_exceeded`
  if over) → else `delivered`.
- `email:delivery` — one email. Enqueued **only from the direct-send path**, never
  broadcast. Loads project email settings **fresh** and decrypts per-send, so the provider
  secret never rides through Redis and key rotation is respected. Sends via the Resend
  adapter, then `UpdateResult`s the delivery row → `sent`/`failed`. Asynq retries
  (`MaxRetry(5)`); each attempt updates `attempt`.
- `broadcast:prepare_batches` — lists eligible recipients, consumes usage for the whole set,
  splits into `broadcast_batch` chunks (100–1000, ~len/10), enqueues one delivery per batch.
- `broadcast:delivery` — `BatchCreateTx` inserts a `notification` per recipient in a tx;
  the last batch marks the broadcast `completed`.
- `recipient:delete_data`, `project:delete_data` — async cascading cleanup.

**The email fan-out (`fanOutEmail` in `service/notification.go`) is SYNCHRONOUS on the send
path; the worker only UPDATES the row.** Every outcome is resolved and the row inserted
up-front — terminal skips never enqueue — which is what lets the send return **200 with
per-medium statuses** in `deliveries[]`. Gate order, each recorded as a *visible delivery
outcome* rather than an error:

1. preference/catalog fails ⇒ `muted` (+ `failure_reason` `not_cataloged` vs
   `preference_disabled`)
2. no `project_email_settings` ⇒ `failed` / `provider_not_configured`
3. no primary email contact ⇒ `no_contact`
4. all pass ⇒ `pending`, enqueue `email:delivery`

**A failed email fan-out NEVER rejects the send** — errors are logged, not propagated; the
direct send still returns 200 with the in-app notification. Partial-medium failure ⇒ 200
with per-delivery statuses, never an atomic reject.

`make up` runs **asynqmon** on `:7755` (dev-only, absent from prod on purpose).
`make dev` runs api + worker + console in a tmux session, each hot-reloading.

## Email medium — provider, webhooks, unsubscribe

**Provider config: `project_email_settings`**, one row per project (`project_id` PK,
upserted). Holds `provider` (CHECK `IN ('resend')`), the Resend API key as `secret` BYTEA +
`nonce` (AES-GCM via tantra `cipher` over `env.CipherKey` — same pattern as API-key tokens),
an encrypted `webhook_secret` + `webhook_nonce`, and `from_name`/`from_address`. **Secrets
are never returned in plaintext** — the DTO exposes only `secret_masked` / `webhook_secret_masked`
(last 4). Each secret is re-encrypted only when its plaintext is supplied; blank ⇒ keep the
existing ciphertext. The API key is required on first config; the webhook secret is always
optional. **Console-only surface** (`/console/projects/{id}/email-settings`, GET + PUT).

**Adapter interface (`internal/email/`)** — `NewAdapter(provider, apiKey)` selects by the
`provider` discriminator. `Send`, plus `VerifyWebhookSignature` and `NormalizeWebhookEvent`
so a future provider (or managed SES) slots in without touching the endpoint, the service,
or the console. The Resend adapter calls the REST API directly — **no Resend SDK, no Svix
SDK**. Svix verification is manual: HMAC-SHA256 over `"{svix-id}.{svix-timestamp}.{body}"`
with the base64-decoded `whsec_` key, constant-time compared, ±5-min timestamp tolerance.
Failure ⇒ **401**.

**Resend event → status mapping** (`resendEventKind` + `webhookStatusFor`):

| Resend `type` | status set | `*_at` stamped |
|---|---|---|
| `email.sent` | `sent` | — |
| `email.delivered` | `delivered` | `delivered_at` |
| `email.bounced` | `bounced` (terminal) | `bounced_at` |
| `email.complained` | `complained` (terminal) | `complained_at` |
| `email.opened` | *(unchanged — soft signal)* | `opened_at` |
| `email.clicked` | *(unchanged — soft signal)* | `clicked_at` |
| anything else | *(ignored, 200 ack)* | — |

**Non-regression rules** — webhooks arrive out of order and duplicated, so one guarded SQL
UPDATE (`pg.ApplyWebhookStatus`) matched **by `provider_message_id`** converges them:

- Status advances only when the incoming status **outranks** the current one:
  `pending(0) < sending(1) < sent(2) < delivered(3) < {bounced,complained,failed}(4) < else(5)`.
  A late `delivered` never overwrites `bounced`; among terminals the **first wins** (strict
  `>`), so bounced/complained/failed are **sticky**. `complained` *can* follow `delivered`
  (4 > 3) — "marked spam after receipt" is real.
- Each `*_at` is **first-write-wins** (`COALESCE`); the raw event is **appended** to the
  `provider_response` JSONB array. An event matching no row is acked **200** and logged.

**Unsubscribe (RFC 8058).** Every outbound email carries
`List-Unsubscribe: <{BODHVEDA_API_URL}/unsubscribe/email?t=…>` and
`List-Unsubscribe-Post: List-Unsubscribe=One-Click`. The token is opaque and self-contained:
`base64url(claims) + "." + base64url(HMAC-SHA256(claims, env.HashKey))` — signed with
**`BODHVEDA_API_HASH_KEY`**, *not* the cipher key; claims `{p,r,c,t,e,exp}`; **TTL 180 days**.
Medium is not in the token (this is the email surface). Built in `fanOutEmail` and carried
through the queue — the worker never rebuilds it. Tampered ⇒ 400, expired ⇒ 401. **POST** =
one-click; **GET** = flips *and* renders a small HTML confirmation page. Both idempotent, and
both flip the pref through the same `UpdateRecipientPreferenceTarget` write path.

**A `complained` (spam) webhook auto-unsubscribes** — target-scoped, same write path,
best-effort (an error never fails the webhook ack). **Address-level** suppression across all
targets remains the old design doc's `email_suppression` table, deferred to a managed tier.

## Billing / usage

`service.BillingService` + `pg/usage_*.go`, `user_subscription.go`. Meters the
`notifications` metric per project, consulted on both send paths (`CheckAndConsumeUsage`) to
enforce plan limits. `ErrQuotaExceeded` maps to the `quota_exceeded` statuses.

**Email is NOT metered.** Under BYO the customer pays Resend directly, so an email metric
would be for plan tiers, not cost-recovery — deferred until a managed-sending tier exists.

## Console (`console/src/`)

- TanStack Router, file-based routes under `routes/` (`routeTree.gen.ts` is **generated by
  the Vite plugin** — run `npx vite build`; `npx tsr generate` is not wired up and fails).
  Note `X.tsx` + an `X/` directory makes `X.tsx` a *layout* route needing an `<Outlet/>` —
  that's why `recipients.tsx` became `recipients/index.tsx`.
- TanStack Query; `QueryCache.onError` → `apiErrorHandler` (`lib/api.ts`): toasts
  non-401, silently redirects to sign-in on 401/403.
- Single axios instance, `withCredentials: true`. All endpoint URLs centralized in
  `API_ROUTES` (`lib/api.ts`) — add there, don't hardcode. It `encodeURIComponent`s
  recipient ids.
- **View state lives in the URL** (`lib/search.ts`): `validateViewSearch` for a param that
  picks one of a fixed set (a tab, a kind toggle — always resolves to a concrete value), and
  `optionalEnumSearch`/`optionalStringSearch`/`optionalDaySearch` for optional filters (an
  unrecognized one is *dropped*, since absent is a real answer). Routes own the URL state and
  pass value + onChange down; `replace: true` so tweaking a control doesn't pile up history.
  - ⚠️ **TanStack Router PARSES search params**: `?recipient_search=12` arrives as the
    **number** 12 and `?x=true` as a boolean. A `typeof === "string"` guard silently drops
    them — and recipient ids are very often all digits. `optionalStringSearch` coerces back.
- Features under `src/features/{api_key,auth,billing,email_settings,home,notification,preference,project,recipient}/`
  mirror backend domains. UI lib: `netra`. Tailwind v4.
- Vite reads root `.env` (`envDir: "../"`), exposes only `BODHVEDA_`-prefixed vars.
- **Per-notification delivery status:** the Notifications list's Status column shows the
  in-app outcome and, when the send carried email, the email medium's status + latency on a
  second line — so a diverging outcome (in-app muted, email delivered) is visible per row.
  Since 9.1 that line **explains itself** (prose from `features/notification/delivery_copy.ts`,
  never a raw slug — e.g. `muted` reads "target not cataloged for email" vs "recipient opted
  out"), and a **Details** dialog shows the full lifecycle + provider webhook history, fetched
  on open from `GET /console/projects/{id}/notifications/{id}/deliveries` (kept off the list
  because `provider_response` is unbounded).
- **Recipient detail page** — `routes/projects/$id/recipients/$recipientId.tsx`, netra `tabs`:
  Overview (identity + project-scoped counts), Notifications (that recipient's feed),
  Preferences (an editable per-`(target, medium)` grid of netra `Switch`es, each cell showing
  the **resolved** decision from `ResolveRecipientPreferences` + `inherited`/`cataloged`/
  `source` as context), Contacts. Recipient ids link here from the recipient and notification
  lists.
- **Notification list filters** (9.4) — in-app status, target, email delivery outcome, date
  range, recipient search, all URL-synced. See the Phase 9.4 deviations for the semantics.
- **Email delivery overview** (`email_delivery_overview.tsx`) — a per-status
  `count(*) FILTER (…)` KPI row above the direct table; **self-hides until the project has
  attempted ≥1 email**, and is deliberately **lifetime/unfiltered**. `opened`/`clicked` come
  from the `*_at` columns (they are not statuses). The aggregate pattern to copy for 9.5.

> ⚠️ **`Notification.ListForRecipient` is the RECIPIENT'S INBOX, not an operator view.** It
> filters out `muted`/`quota_exceeded` (`pg/notification.go`) and carries **no email delivery
> data**. It powers the Dev API inbox, where that is correct. Every console view uses
> `ListNotifications` + a `recipient_id` filter instead — an operator asking "why didn't they
> get it?" came for exactly the rows the inbox hides. Don't re-propose it for console work.

## Conventions worth remembering

- API key plaintext token is returned **only** on create; stored encrypted (`token` BYTEA +
  `nonce`), looked up by HMAC `token_hash`. Never log/return the plaintext elsewhere.
- `UserIdentity` carries the password hash — must never be serialized to clients.
- Recipient `external_id` is the external handle (**lowercase**); the serial `id` stays internal.
- **The `notification` table is on the send hot path** — every direct send and every broadcast
  batch inserts into it. It carries exactly one index besides its PK
  (`ix_notification_project_id (project_id, id DESC)`, added in 9.4). Adding another taxes every
  send; measure first. Same instinct as refusing to hang aggregates off `repo.Get`.
- ⚠️ **netra's stylesheet wins on source order, so overriding a netra default class needs `!`.**
  `index.html` links the console's Tailwind in `<head>`; `main.tsx` imports `"netra/styles.css"`
  *after*, so netra's utilities come later and beat the console's at equal specificity (media
  variants add none, so even `sm:*` loses). Worse, netra's `cn` is `twMerge(clsx(…))`, so a
  plain `sm:max-w-2xl` both *strips* netra's `sm:max-w-lg` and then loses to its base
  `max-w-[calc(100%-2rem)]` — removing the cap entirely. Hence `select-text!`, `w-full!`,
  `sm:max-w-3xl!`.
- ⚠️ **This repo is on chi v1** (`github.com/go-chi/chi`) — what `routes.go` and tantra's
  `httpx.ParamInt` use — even though `chi/v5` is also in `go.mod`. Mounting a test router on
  v5 makes every URL param resolve **empty**, surfacing as a misleading `400 Invalid project ID`.
  Import the **v1** path.
- **Test pattern:** real-Postgres integration tests gated on `TEST_DB_URL`, self-cleaning via
  `t.Cleanup`, borrowing a `user_id` from an existing project. Fakes only where a DB adds
  nothing (adapter/httptest, DTO validation).
- **`httpx.DecodeQuery` (gorilla/schema) decodes `*time.Time` from RFC3339 — and a BLANK
  `?created_from=` is a hard 400**, not an ignored param. Callers must omit, not blank.

## Design decisions (the log)

- **BYO-provider adapter over platform-owned email resale (for v1).** Owning email means owning
  sender reputation/deliverability — SES aggregates bounce/complaint across all customers and
  suspends the whole account past ~0.1% complaints / ~5% bounces, so one bad customer =
  platform-wide outage; isolating needs dedicated IPs + warmup (only economical at volume).
  Category peers (Knock/Novu/Courier) are BYO and monetize orchestration, not email bytes.
  Margins come from the notifications/MAR meter. **Resend is the first adapter** (dogfooding via
  its free tier). Managed SES is a later **paid upsell on the same adapter interface** — so
  BYO-first throws nothing away.
- **Email is DIRECT-only — never on broadcast.** Bulk email blasts are the fastest way to wreck
  sender reputation / get suspended, the exact risk BYO-first exists to avoid. Revisit only once
  managed sending + reputation controls exist.
- **Content-block-implies-intent send model (non-breaking).** The sender signals which mediums to
  attempt by which content blocks it includes; no `mediums[]` array, no breaking change, **no
  payload→email fallback**. Chosen over the old doc's explicit-`mediums[]`-breaking model because
  it keeps the send API compatible while still giving per-send control.
- **Subject/body come from the Send API, not from target config.** Real subjects are per-send
  dynamic. Bodhveda is a **pass-through**: the caller renders its own template (e.g.
  `@react-email`) and passes html/text. No templating engine, no variables. Per-target templates
  in the console (the Knock/Courier model) are a legitimate future feature, deferred.
- **`email` is a typed SIBLING block, not `email_*` keys inside `payload`.** `payload` is
  customer free-form JSON for in-app rendering; injecting reserved keys collides and couples
  concerns, and email needs ≥3 typed, validatable fields. A unified `content: {inapp, email}` map
  would be cleaner but renaming `payload` is breaking — deferred to a hypothetical v2.
- **`recipient_contact` table over a bare `email` column.** Web-push is next, so build a schema
  that already supports multiple contacts, primary/fallback and verification, and skip a
  re-migration.
- **`notification_delivery` for email (non-in_app) only.** Leave `in_app` on the `notification`
  row — the old doc's inbox migration/dual-write/column-drop is a big risky change with no
  current payoff.
- **Email "opened" is a SOFT signal.** Apple Mail Privacy Protection pre-fetches pixels
  (inflates); blocked images deflate. It is directional only — **in-app `read` is the trustworthy
  signal**. Never chart or present them as the same kind of fact. (`OPEN_SOFT_SIGNAL_COPY` in
  `delivery_copy.ts` is the single source of that caveat's wording.)
- **Provider knowledge stays SERVER-side.** Normalization lives in the adapter interface, so the
  console never parses a provider's JSON shape and a future adapter isn't also a frontend change.
- **The old design doc is retained as the managed-email tier blueprint**, not discarded.

## Validation target: Resurface (`../resurface`)

Resurface is the real-world app that proved the email medium, and is the live consumer to keep in
mind when changing a public surface. It **dropped its Resend integration entirely**: one Bodhveda
`notifications.send({recipient_id, target, payload, email})` per user fans out to inbox + email.

- Its digest target is `digest/none/sent` (`web/lib/bodhveda-targets.ts`); recipients' emails are
  synced **server-side** as a `recipient_contact` on `/me` (never from the browser).
- Preferences are Bodhveda's, read/written from its settings UI via `@bodhveda/react`
  (`usePreferences`/`useUpdatePreference`) — so the settings toggle and the email's one-click
  unsubscribe stay in sync automatically. That's the point of the convergence at `repo.Create`.
- **`isPro` stays a Resurface entitlement** (Bodhveda has no plan concept): if Pro, include the
  `email` block; Bodhveda then decides email vs in-app from preferences.
- Verified in prod: the daily digest delivers in-app + email for opted-in Pro users, and an
  opted-out recipient has **both** mediums muted, visible per-notification in the console.

## Roadmap — phased delivery (one phase per session)

Each phase is scoped to a single working session and leaves `main` shippable and independently
testable. When a phase completes, update its status here and record what changed the plan.
**Follow the existing layered handler→service→pg pattern; don't refactor domains mid-phase.**

### Status

- **Phases 0–8 — DONE and SHIPPED.** The email medium, end to end: design, recipient contacts,
  per-medium preferences + catalog, project provider settings, email delivery core (Resend adapter
  + `email:delivery` + `notification_delivery`), delivery-status webhooks, RFC 8058 unsubscribe,
  docs + SDK release prep, VPS/Cloudflare deploy, and the Resurface cutover that validated it in
  production. Their plans/prompts/deviation notes lived here until 9.4; everything of theirs that
  still matters is folded into the sections above, and `git log` has the rest.
  - ⚠️ **Unpublished/undeployed:** the 9.3.1 SDK bumps (js core/react `0.2.0`, go `v0.3.0`) —
    runbook `agent-docs/release-preference-read-fix.md`. The **behavior** change shipped with the
    API; the SDK bumps are types + docs only.
- Phase 9.1 — Delivery detail — **DONE** (see deviations below)
- Phase 9.2 — Recipient detail page — **DONE** (see deviations below)
- Phase 9.3 — Recipient preference editing (the per-medium grid) — **DONE** (see deviations below)
- Phase 9.3.1 — Developer API preference read fix — **DONE** (see deviations below)
- Phase 9.4 — Notification list filters — **DONE** (see deviations below). The only Phase 9
  sub-phase to need a **migration** — though not for the reason the plan expected: the
  `notification` table had **no index at all** beyond its PK, so the list was already a seq scan
  before any filter was added.
- Phase 9.5 — Analytics (time-series + per-target/medium breakdowns) — **TODO**

---

## Phase 9 — Console (UI/UX + power features)

The email medium is shipped and validated (Phases 1–8). The console is now the weakest surface:
each email phase bolted on the minimum UI it needed, and **twice deferred work explicitly because
a recipient detail page didn't exist** (Phase 1 → contacts became a modal; Phase 2 → the
per-medium preference grid was skipped). Phase 9 pays that down.

### The shape of the problem (verified against the code, 2026-07-16)

**Rich data is captured and never shown.** This is the theme of the whole arc.
`notification_delivery` stores `failure_reason`, `attempt`, `provider`, `provider_message_id`,
`provider_response` (JSONB — the full raw webhook event history), `address_snapshot`, and five
timestamps (`sent_at`/`delivered_at`/`bounced_at`/`complained_at`/`opened_at`/`clicked_at`).
The console shows **status + one elapsed time**.

> ⚠️ **The bottleneck is the projection, not the schema.** The batch query at
> `pg/notification.go` (~L356, in `List`) selects only
> `notification_id, status, sent_at, delivered_at`; `entity.Notification` carries only
> `EmailStatus`/`EmailSentAt`/`EmailDeliveredAt` (`entity/notification.go:30-32`); and
> `dto.NotificationEmailDelivery` (`dto/notification.go:34`) exposes only those three.
> **No migration is needed for any of Phase 9** — every column already exists (Phase 4 DDL,
> recorded above). The work is widening SELECT → entity → DTO → UI.
>
> Note the asymmetry that makes this obvious: `dto.NotificationDelivery` **does** carry
> `failure_reason`, but it only rides the **send** response (`SendNotificationResult.deliveries[]`),
> which is why `send_notification_modal.tsx:595` can explain a skip and the notifications **list**
> cannot. The same `muted` row is self-explanatory right after you send it and opaque forever after.

**Console state today:**
- Routes are flat: `routes/projects/$id/{home,notifications,recipients/,preferences,api-keys,billing,settings}.tsx`.
  ~~**No detail route for anything**~~ — **since 9.2** there is one: `recipients/$recipientId.tsx`
  (and `recipients.tsx` became `recipients/index.tsx` so it is not treated as a layout route).
  Notifications and broadcasts remain list-only (a notification's detail is 9.1's dialog).
- `features/home/home.tsx` is four lifetime scalars (Recipients / Notifications / Direct /
  Broadcast). There is **no stats endpoint**: it calls `useGetProjects()`, fetches every project,
  and `.find()`s the current one (`home.tsx:41`). No time dimension, no date range, no grouping.
- `email_delivery_overview.tsx` (Phase 5) is the only other analytics — per-status counts,
  project-wide, lifetime, self-hiding until ≥1 email is attempted.
- ~~`recipient_list.tsx:203-205` renders the recipient ID as a plain span; nothing links anywhere.~~
  **Fixed in 9.2:** recipient ids link to the detail page from both the recipient list and the
  notifications list, via the shared `features/recipient/recipient_link.tsx`.
- ~~`dto.ListNotificationsFilters` is `{ProjectID, Pagination, Kind}` + (**9.2**) `RecipientExtID` —
  still no status, target, medium, or date filter.~~ **9.2 switched the repo method to take the
  filters DTO**, and **9.4 duly extended that struct** (status, target, email delivery, date range,
  recipient search) with no signature churn — the seam worked as designed.

**What already exists to build on (reuse — do NOT re-derive):**
- ⚠️ `Notification.ListForRecipient` — powers the Dev API's `GET /recipients/{id}/notifications`.
  **It is the RECIPIENT'S INBOX, not an operator view, and 9.2 deliberately did NOT use it:** it
  filters out `muted`/`quota_exceeded` and carries no email delivery data. The console's
  recipient feed is `ListNotifications` + a `recipient_id` filter instead. Don't re-propose
  `ListForRecipient` for console work — see the 9.2 deviations.
- ⚠️ `repository.ListPreferencesForRecipient` / service `GetRecipientProjectPreferences` — **do NOT
  reach for either for console work.** `GetRecipientProjectPreferences` is a Go exact-match merge
  over the project catalog that **disagrees with what a send actually does** (no `topic='any'`
  fallbacks, no medium-dependent default, and a recipient row on an uncataloged pair is invisible
  to it while still delivering). Since 9.3 the console resolves preferences with
  **`PreferenceRepo.ResolveRecipientPreferences`** (one set-based query, the same cascade as
  `ShouldDirectNotificationBeDelivered`, held in step by a test).
  `GetRecipientProjectPreferences` survives **only** because the Developer API's documented
  `GET /recipients/{id}/preferences` still uses it — **and it is still wrong there**; see the
  Phase 9.3 deviations, which recommend fixing that public surface as its own phase.
- ~~`handler.GetRecipient` is Dev-API only; the console has no single-recipient GET~~ — **9.2 added
  `GET /console/projects/{id}/recipients/{ext_id}`** (`GetRecipientConsole` → `GetWithCounts`,
  counts included; the Dev API's stays lean because `repo.Get` is on the send hot path).
- Console recipient **contacts** CRUD endpoints already exist and are wired — since 9.2 the UI is
  the detail page's Contacts tab (`recipient_contacts_panel.tsx`; the modal is deleted).
- `UpsertRecipientPreferences` — console `PUT /recipients/{id}/preferences` already exists.
  **9.2 added the matching read** (`GET .../preferences`), so 9.3 has both halves.
- 9.1's status cell + dialog trigger are shared components in
  `features/notification/components/notification_cells.tsx` (extracted in 9.2).
- `EmailDeliveryOverviewForProject` (per-status `count(*) FILTER`) — the aggregate pattern to
  copy for Phase 9.5.

**UI lib (`netra`) — confirmed available:**
- **Charts ship with netra**: `ChartContainer`, `ChartTooltipContent`, `ChartLegendContent`,
  `tooltipCursor`, `axisDefaults` are re-exported from the root barrel (`components/chart/chart`),
  wrapping **recharts ^2.15.4** (a netra *dependency*, so it installs transitively). **Do not add a
  charting library.** If you import recharts primitives (`<LineChart>`, `<Bar>`) directly rather
  than through netra's wrapper, add `recharts` to `console/package.json` explicitly instead of
  relying on the transitive install.
- Also available: `tabs`, `dialog`, `date_picker`, `calendar`, `data_table`/`DataTableSmart`,
  `popover`, `scroll_area`, `tag`.
- **No drawer/sheet component exists** — use `dialog` for the delivery detail, matching the
  existing modal-driven console UX.

### Sequencing rationale

9.1 first: it's the cheapest, needs no new endpoint, and answers the single most common support
question ("why didn't my email send?"). 9.2 is the keystone — it unblocks 9.3, which was already
deferred once. 9.4 is independent. 9.5 is last: it's the only sub-phase needing genuinely new
aggregate queries.

---

### Phase 9.1 — Delivery detail

- **Goal:** every value `notification_delivery` already stores is inspectable from the
  notifications list. A `muted` or `failed` email explains itself **in the list**, not only in the
  post-send modal.
- **In scope:** widen the email-delivery projection (`pg/notification.go` batch query → entity →
  `dto.NotificationEmailDelivery`) to carry `failure_reason`, `attempt`, `provider_message_id`,
  `address_snapshot`, and the `bounced_at`/`complained_at`/`opened_at`/`clicked_at` timestamps;
  surface `failure_reason` inline on the list's `MediumStatusLine` (`notifications_list.tsx:282`)
  — at minimum distinguishing `not_cataloged` from `preference_disabled`, the distinction Phase 4
  created this field for; a **delivery detail dialog** (netra `dialog`) per notification row showing
  the full delivery lifecycle — every timestamp, attempt count, provider message id, and the raw
  `provider_response` event history rendered readably (it's a JSONB **array**, appended per webhook
  — see Phase 5 deviations).
- **Out of scope:** new endpoints (widen the existing list response); the recipient page (9.2);
  filters (9.4).
- **Depends on:** nothing.
- **Done when:** a `muted` email row states *why* without leaving the list; the detail dialog shows
  the full webhook history for a delivered/bounced email; no migration was needed.
- **Watch for:** `provider_response` can be large — consider whether it belongs in the list payload
  at all, or behind a per-notification fetch. If the list response gets heavy, prefer a dedicated
  `GET /console/projects/{project_id}/notifications/{notification_id}/deliveries` and keep the list
  lean (decide this deliberately and record it).

```
Read agent-docs/overview.md in full first, esp. "Phase 9 — Console" (the shape of the problem),
the Phase 4 deviations (the final `notification_delivery` schema + the DeliveryStatus enum) and
Phase 5 deviations (the webhook → status mapping, the `provider_response` JSONB append, and the
opened-is-a-soft-signal rule). Implement Phase 9.1 (Delivery detail).

The premise, already verified — do NOT re-derive: `notification_delivery` stores failure_reason,
attempt, provider, provider_message_id, provider_response (JSONB array of raw webhook events),
address_snapshot, and sent/delivered/bounced/complained/opened/clicked timestamps. The console
shows status + one elapsed time. NO MIGRATION IS NEEDED — every column exists. The bottleneck is
the projection: the batch query in `pg/notification.go` (~L356, inside `List`) selects only
`notification_id, status, sent_at, delivered_at`; `entity.Notification` (entity/notification.go:30-32)
carries only EmailStatus/EmailSentAt/EmailDeliveredAt; `dto.NotificationEmailDelivery`
(dto/notification.go:34) exposes only those three. Widen SELECT → entity → DTO → UI.

Then:
1. Surface `failure_reason` inline in the list's status column (`MediumStatusLine`,
   notifications_list.tsx:282). Phase 4 set failure_reason specifically to distinguish
   `not_cataloged` (target has no project-level (target,email) catalog row) from
   `preference_disabled` (recipient opted out) — both share status `muted`. That distinction is
   the whole point; make it readable, not a raw slug dumped on screen.
2. Add a delivery detail dialog (netra `dialog` — netra has NO drawer/sheet) opened from the
   notification row: full lifecycle timestamps, attempt count, provider + provider_message_id,
   address_snapshot, and the provider_response event history. provider_response is an ARRAY
   appended once per webhook (Phase 5) — render it as a readable timeline, not raw JSON dump.
3. Keep the soft-signal framing: `opened` is directional (Apple MPP inflates it), unlike in-app
   `read`. Phase 5 already has a tooltip for this in email_delivery_overview.tsx — reuse its copy.

DECIDE DELIBERATELY and record it: provider_response can be large. Either widen the list response
or add `GET /console/projects/{project_id}/notifications/{notification_id}/deliveries` and keep the
list lean. Do not bloat every list row with full webhook history by reflex.

Console-only + the projection widening. Do NOT touch the send path, the worker, the broadcast
pipeline, or any gating logic. No migration. Follow the layered handler→service→pg pattern. Add
new endpoint URLs to `API_ROUTES` in lib/api.ts, never hardcoded. Update Phase 9.1 status to DONE
and add a "Phase 9.1 — deviations (as built)" section recording the list-vs-endpoint decision.
```

#### Phase 9.1 — deviations (as built)

**No migration** — as predicted, every column already existed (Phase 4 DDL). The work was
exactly the projection widening: SELECT → entity → DTO → UI. Backend follows the layered
`handler → service → pg` split; the send path, worker, broadcast pipeline, and all gating logic
are untouched. `go build`/`go vet` pass; the whole suite (incl. the new real-Postgres tests)
passes; the console typechecks, lints, and builds.

- **THE DECISION (list vs. endpoint): HYBRID, split on BOUNDEDNESS.** Both, deliberately — the
  question isn't "list or endpoint", it's "which columns are bounded".
  - **The list carries every BOUNDED column**: `failure_reason`, `attempt`, `provider`,
    `provider_message_id`, `address_snapshot`, and all six timestamps (`sent_at`/`delivered_at`/
    `bounced_at`/`complained_at`/`opened_at`/`clicked_at`). These are fixed-size scalars — a few
    hundred bytes per row. `failure_reason` **must** be inline (it's the whole point of the
    phase), and once the query is open, the remaining scalars are free and let the dialog render
    instantly with no spinner.
  - **`provider_response` is NOT in the list.** It is the one **unbounded** field: a JSONB array
    that grows by one raw provider event body (~1–3 KB) per webhook, forever. The notifications
    list is paginated, refetched with `keepPreviousData`, and invalidated on every send — so
    putting webhook history there multiplies unbounded payload × page size × every refetch, for
    data that is only ever read **one delivery at a time**. It is served by a dedicated endpoint
    the dialog fetches on open (`enabled: open` on the query — that gating *is* the split).
  - Net: the list response grew by bounded scalars only; the heavy field moved behind an
    explicit, on-demand fetch. This is the "do not bloat every list row with full webhook
    history by reflex" instruction taken literally, without giving up the inline explanation.
- **New endpoint: `GET /console/projects/{project_id}/notifications/{notification_id}/deliveries`**
  (exactly the shape the phase brief proposed), added to `API_ROUTES` as
  `project.notifications.deliveries`. Console-only, under the existing
  `VerifyUserOwnsThisProject` gate. Returns `{deliveries: [...]}` — **empty, not 404**, when the
  send carried no email (in_app has no delivery row in v1; "no rows" is a legitimate answer, and
  the dialog says so in prose).
- **`ListForNotification` was DEAD CODE — repurposed and project-SCOPED rather than duplicated.**
  Phase 4 declared + implemented it on the delivery repo and never called it. It gained a
  `projectID` parameter (`ListForNotification(ctx, projectID, notificationID)`) and a
  `project_id = $2` predicate. **This is a real security boundary, not tidiness:**
  `VerifyUserOwnsThisProject` only proves the caller owns the *project* in the URL — without the
  scope, a guessed `notification_id` from another project would resolve. Covered by a test.
- **The entity's flat `Email*` fields became a struct.** `entity.Notification`'s
  `EmailStatus`/`EmailSentAt`/`EmailDeliveredAt` became `Email *entity.NotificationEmailDelivery`
  (12 flat `Email*`-prefixed fields would have been unreadable). `entity.NotificationDelivery`
  (the full row) gained the Phase 5 timestamp columns + `ProviderResponse`, so the shared
  `notificationDeliveryFields` const + `scanNotificationDelivery` now cover the whole table —
  `Create`'s `RETURNING` widened with them for free.
  - `provider_response` is scanned into `[]byte` (not `json.RawMessage`) so a SQL NULL — the
    normal state of every terminal-skip row — stays nil instead of attempting a decode. Verified.
- **Webhook events are normalized SERVER-side by REUSING Phase 5's adapter normalizer.** The
  detail DTO exposes `events[] = {kind, at, raw}`, built by calling the delivery row's own
  provider adapter — `email.NewAdapter(provider, "")` (no API key needed to normalize, same as
  the webhook path) + `NormalizeWebhookEvent(http.Header{}, raw)`. **Empty headers are fine**:
  headers only supply the Svix `svix-id` idempotency key on the live path, which stored events
  don't need. This deliberately honors Phase 5's "normalization lives in the adapter interface so
  other providers slot in later" — teaching the React console to parse Resend's JSON shape would
  have put provider knowledge on the wrong side of the wire and made a future adapter a
  frontend change too. Normalization is best-effort presentation: an unparseable event degrades
  to `kind: ""` with its raw body intact, never a failed request. **No new adapter method was
  needed.**
- **`failure_reason` is rendered as prose, never as a slug** (`features/notification/
  delivery_copy.ts`). Every one of the nine reasons the backend actually writes
  (`not_cataloged`, `preference_disabled`, `provider_not_configured`, `provider_send_error`,
  `provider_lookup_error`, `contact_lookup_error`, `gating_error`, `secret_decrypt_error`,
  `adapter_init_error`) gets a **short** phrase for the space-constrained list line plus a
  **long** explanation (what happened *and* the fix) in a tooltip and the dialog. The two causes
  of `muted` now read as "target not cataloged for email" vs "recipient opted out" — the
  distinction Phase 4 created the field for. The wording deliberately **matches the post-send
  toast** (`notifyEmailOutcome`, send_notification_modal.tsx:595), so the same skip reads the
  same way whether seen right after sending or a week later in the list — that asymmetry (the
  send response could explain a skip, the list couldn't) was the phase's stated motivation. An
  unrecognized future slug degrades to a de-underscored version of itself rather than vanishing.
  `no_contact` carries no `failure_reason` (the status says it), so the status itself is given
  copy too.
- **Console UI.** The Status column's email line gained the inline reason phrase + `IconInfo`
  tooltip. `MediumStatusLine` stayed the shared component (the broadcast table still uses it)
  and took one optional `reason` prop, so in-app/broadcast rendering is unchanged. The dialog
  (`components/delivery_detail_dialog.tsx`, netra `dialog` — netra has **no** drawer/sheet)
  shows the resolved outcome + explanation, the address snapshot, attempts, provider + message
  id, the full lifecycle grid, and the `provider_response` history as a **timeline** (dot/line
  rail, human event labels, per-event `<details>` for the raw JSON) — not a JSON dump. An empty
  history distinguishes *"waiting on webhooks"* from *"this email never reached the provider"*
  (muted/no_contact/failed). Width is `sm:max-w-3xl!` — see the netra cascade gotcha above.
- **The dialog is NOTIFICATION-scoped, not email-scoped, and the trigger is a row-level
  "Details" column.** Built email-only first (trigger on the Status cell's email line), which
  left the most common notification — in-app-only, no email — with **no** detail affordance at
  all, and made the dialog's own "no email" empty state unreachable. Corrected on review:
  - The dialog now opens with an **In-app section** rendered from the notification row itself
    (status, sent/resolved, read/opened) — **no fetch and no backend change**, because `in_app`
    has no delivery record by design (Phase 4 kept its state on the `notification` row). The
    dialog names that asymmetry instead of hiding it. Email still renders from its delivery
    record; only that half is fetched.
  - It surfaces **`payload`** (the in-app content block as sent) — pretty-printed. It was
    already in the list response and rendered **nowhere** in the console, so "what did I
    actually send?" had no answer. `dto.Notification.payload` is `json.RawMessage` ⇒ it
    serializes as a JSON **object**; the console typed it `payload: string`, which was simply
    wrong (and unused anywhere, so it was safe to correct to `unknown`).
  - The trigger moved out of the Status cell into its own trailing **`details` column** — the
    dialog is per-notification, so a per-medium line was the wrong home; a column also gives
    every row exactly one trigger without adding a third line to every Status cell.
  - **Deliberately NOT done:** in-app `read_at`/`opened_at` **timestamps** (the list shows only
    the `state.read`/`state.opened` booleans). Exposing them means widening `dto.Notification`,
    which is **shared with the Dev API's** recipient inbox response — an additive but public
    surface change that belongs with `openapi.json` (Phase 7), not smuggled into a console
    phase. The booleans are shown instead.
- ⚠️ **Gotcha for ALL future console UI: netra's stylesheet wins on source order, so overriding
  a netra default class needs `!`.** `index.html` links `/src/index.css` (the console's Tailwind)
  in `<head>`, while `main.tsx` imports `"netra/styles.css"` **after** it — so netra's
  precompiled utilities always come LATER in the cascade and beat the console's at equal
  specificity (media-query variants add **no** specificity, so even `sm:*` loses to a netra base
  utility). This is why the console is peppered with `!` classes (`select-text!`, `w-full!`) —
  they are fighting exactly this, not styling whims. It bit the dialog: netra's `DialogContent`
  is `cn("… max-w-[calc(100%-2rem)] … sm:max-w-lg", className)` and its `cn` **is**
  `twMerge(clsx(…))`, so a plain `sm:max-w-2xl` made twMerge drop netra's `sm:max-w-lg` and then
  lost to netra's base `max-w-[calc(100%-2rem)]` — removing the cap and rendering the dialog
  **full-bleed**. Fixed with `sm:max-w-3xl!` (768px — comfortable for the 2-column field grid).
  Note the failure mode is *worse than no override*: you strip the default AND lose.
- **Soft-signal framing preserved.** The Opened/Clicked fields and any opened/clicked timeline
  event carry the Apple-MPP caveat tooltip, **reusing Phase 5's exact copy** — lifted verbatim
  from `email_delivery_overview.tsx` into `OPEN_SOFT_SIGNAL_COPY` (now the single source for
  both). Email `opened` stays directional; in-app `read` stays the trustworthy signal.
- **Tests (real-Postgres, gated on `TEST_DB_URL`, self-cleaning — the Phase 3/5/6 pattern):**
  `internal/service/notification_delivery_detail_test.go` (the widened **list** projection
  carries `failure_reason`/`attempt`/`address_snapshot`/`opened_at`; the detail returns a
  normalized 2-event timeline with the raw body preserved; a NULL `provider_response` yields zero
  events; **another project cannot read the row**) and
  `internal/handler/notification_deliveries_test.go` (the endpoint over real HTTP through a chi
  router mounted exactly as `routes.go` mounts it — proves both URL params reach the handler and
  the JSON serializes as the console expects; plus a non-numeric id ⇒ 400).
  - ⚠️ **Gotcha for future handler tests: this repo is on chi v1** (`github.com/go-chi/chi`),
    which is what `routes.go` and tantra's `httpx.ParamInt` (`chi.URLParam`) use — even though
    `chi/v5` is also in `go.mod` (pulled in elsewhere). Mounting a test router on `chi/v5` makes
    every URL param resolve **empty** (each version stores its route context under its own key),
    which surfaces as a misleading `400 Invalid project ID`. Import the **v1** path.
- **Untouched (as scoped):** the send path, the worker, the broadcast pipeline, all gating logic,
  the SDKs, and `docs/`. No new endpoints beyond the one deliveries read. The Dev API's
  notification surface is unchanged — this is console-only.

### Phase 9.2 — Recipient detail page (the keystone)

- **Goal:** a real answer to "who is this recipient, what have we sent them, and what did they
  actually get?" — the page two earlier phases were deferred for.
- **In scope:** route `routes/projects/$id/recipients/$recipientId.tsx` (TanStack file-based;
  `routeTree.gen.ts` is generated — never hand-edit); **two console endpoints**: a single-recipient
  GET (the Dev API's `handler.GetRecipient` exists but is API-key auth'd — wrong surface) and a
  recipient-scoped notifications list (reuse the existing `Notification.ListForRecipient` service
  method — it already powers the Dev API; this is routing/DTO plumbing, **not** new query logic);
  the page itself as netra `tabs` — Overview (identity, created_at, direct/broadcast counts),
  Notifications (their feed with per-medium status, reusing 9.1's status cell + detail dialog),
  Preferences (read-only here — editing is 9.3, via the existing
  `ListPreferencesForRecipient(ctx, projectID, recipientExtID)` repo method at
  `service/preference.go:171`), and Contacts (**fold `recipient_contacts_modal.tsx` in** — the
  console endpoints already exist; Phase 1 only made it a modal because this page didn't exist);
  **make recipient IDs clickable** in `recipient_list.tsx:203-205` (currently a plain
  `<span className="select-text!">`) and in the notifications list's recipient column.
- **Out of scope:** preference *editing* (9.3); analytics (9.5); notification filters (9.4) beyond
  what this page's own feed needs.
- **Depends on:** 9.1 (reuses its status cell + dialog; sequence after, though not a hard block).
- **Done when:** clicking a recipient ID anywhere lands on their page; the page shows their
  identity, their notification feed with per-medium outcomes, their current preferences, and their
  contacts; the standalone contacts modal is gone (or is now just this page's tab).
- **Watch for:** recipients are addressed by customer-chosen `external_id` **strings**, not the
  serial `id` — they can contain URL-hostile characters. The existing contacts routes already
  `encodeURIComponent` them (`lib/api.ts:51`); the route param must do the same. Keep the internal
  serial `id` internal.

```
Read agent-docs/overview.md in full first, esp. "Phase 9 — Console" and the Phase 1 + Phase 2
deviations — BOTH explicitly deferred work "because the console has no recipient detail page
today" (overview.md:522 → contacts became a modal; overview.md:618-621 → the per-medium preference
grid was skipped). This phase builds that page. It is the keystone of the console arc.

What already exists — reuse, do NOT re-derive:
- `Notification.ListForRecipient` (service) already powers the Dev API's
  GET /recipients/{id}/notifications. The console needs a ROUTE, not a query.
- `repository.ListPreferencesForRecipient(ctx, projectID, recipientExtID)` exists
  (service/preference.go:171).
- Console recipient CONTACTS CRUD endpoints already exist and are wired
  (features/recipient/list/recipient_contacts_modal.tsx).
- `handler.GetRecipient` exists but on the DEV API (API-key auth) — the console surface has NO
  single-recipient GET (list/create/patch/delete only). Add one, console-side, gated by the
  existing VerifyUserOwnsThisProject like every other console project route.

Build:
1. Two console endpoints: single-recipient GET, and a recipient-scoped notifications list (wrap
   ListForRecipient). Follow the layered handler→service→pg pattern; add both to `API_ROUTES` in
   lib/api.ts (never hardcode URLs).
2. Route `console/src/routes/projects/$id/recipients/$recipientId.tsx` (TanStack file-based —
   routeTree.gen.ts is GENERATED, never hand-edit). Page = netra `tabs`:
   - Overview: identity, created_at, direct/broadcast counts (RecipientListItem already carries
     direct_notifications_count / broadcast_notifications_count).
   - Notifications: their feed, reusing 9.1's per-medium status cell + delivery detail dialog.
   - Preferences: READ-ONLY this phase (editing is 9.3).
   - Contacts: FOLD IN recipient_contacts_modal.tsx — endpoints already exist. Phase 1 only made
     it a modal because this page didn't exist. Remove the row-action modal once folded.
3. Make recipient IDs CLICKABLE → this page: recipient_list.tsx:203-205 (currently a plain
   `<span className="select-text!">`) and the notifications list's recipient column.

CRITICAL: recipients are addressed by the customer-chosen `external_id` STRING, not the serial id.
It can contain URL-hostile characters — encodeURIComponent it in the route exactly as the existing
contacts routes do (lib/api.ts:51). The internal serial `id` stays internal (a long-standing repo
convention — see "Conventions worth remembering").

Console + two read endpoints only. No migration. Do NOT touch the send path, worker, gating, or
broadcast pipeline. Do NOT build preference editing (9.3) or analytics (9.5). Update Phase 9.2
status to DONE and add a "Phase 9.2 — deviations (as built)" section.
```

#### Phase 9.2 — deviations (as built)

**No migration**, as scoped. Backend follows the layered `handler → service → pg` split. The send
path, worker, gating logic, and broadcast pipeline are untouched. `go build`/`go vet`/the whole
test suite pass; the console typechecks, lints, and builds; the page was driven **live** in a real
browser against the running API + Postgres (all four tabs, both entry points, the 9.1 dialog).

- ⚠️ **THE BIG ONE — the feed is NOT `ListForRecipient`, and the brief's two instructions were
  mutually exclusive.** The brief said to wrap `Notification.ListForRecipient` ("routing/DTO
  plumbing, not new query logic") *and* to reuse 9.1's per-medium status cell + delivery dialog.
  You cannot do both. `ListForRecipient` is the **recipient-facing inbox**:
  - It filters `status NOT IN ('muted','quota_exceeded')` (`pg/notification.go:132`) — by design,
    since the recipient never received those. But a `muted` row is *exactly* what an operator
    asking "why didn't they get it?" came for.
  - It attaches **no email delivery data**, so 9.1's status cell and dialog would render nothing —
    the reuse the same brief mandated.
  
  So the feed instead reuses the **console list path** (`ListNotifications`), which 9.1 already
  widened with the email-delivery batch query, scoped to one recipient. Verified live: the
  recipient's feed shows a row that is **in-app `delivered` but email `muted` /
  `not_cataloged`** — under `ListForRecipient` that entire email line would not exist.
  `ListForRecipient` is left **completely untouched**; it still powers the Dev API inbox, where
  its filtering is correct.
- **That means NO new notifications endpoint — a `recipient_id` FILTER on the existing list.**
  `dto.ListNotificationsFilters` gained `RecipientExtID *string` (`schema:"recipient_id"`), and
  `enum.NotificationKindAll` ("all") was added so the feed can show direct **and** broadcast in one
  table. This is sanctioned by the brief's own scope line ("filters (9.4) **beyond what this page's
  own feed needs**" are out of scope) and is the seam 9.4 extends. An **omitted** kind still means
  `direct` — the project Notifications list depends on that default, and there is a test pinning it.
  - `repository.NotificationRepository.ListNotifications` now takes the **filters DTO** instead of
    `(projectID, kind, pagination)`. 9.4 adds filters by extending the struct, with no signature
    churn. The service lowercases `RecipientExtID` (external ids are stored lowercase).
- **Endpoint count: two, but NOT the two the brief named.** The brief said "single-recipient GET +
  a recipient-scoped notifications list". Actual:
  1. `GET /console/projects/{project_id}/recipients/{recipient_external_id}` — new (the Dev API's
     `handler.GetRecipient` is API-key auth'd, the wrong surface).
  2. `GET /console/projects/{project_id}/recipients/{recipient_external_id}/preferences` — new, and
     **the brief missed it**: the console could already `PUT` a recipient's preferences but could
     only read them **project-wide**, so the Preferences tab had no read path at all.
  Both are gated by the existing `VerifyUserOwnsThisProject`, like every other console project route.
- **The Preferences tab reads the RESOLVED view, not `ListPreferencesForRecipient`.** The brief
  pointed at that repo method, but it returns only the recipient's *stored* rows. The console GET
  instead reuses the service's existing `GetRecipientProjectPreferences`, which overlays the project
  catalog with the recipient's overrides and marks each row `inherited` — because a recipient with
  no stored row is not "unset", they are following the project default. That distinction is the
  whole point of 9.3's grid, so 9.3 inherits the right read path for free. (`ListPreferencesForRecipient`
  is still used *inside* that service method — the reuse the brief intended, one layer up.)
- 🐛 **Fixed a real cross-project count bug in the query the Overview tab reuses.**
  `pg/recipient.go`'s `findRecipients` joined `notification n ON n.recipient_external_id =
  r.external_id` with **no project predicate**, while only the `WHERE` scoped `r.project_id`.
  `external_id` is unique only **within** a project (`ux: project_id, external_id`), so two projects
  that both have a customer-chosen `"user_1"` counted each other's notifications. Proven against
  Postgres before fixing: project A's recipient reported **3** direct notifications when the truth
  was 0 (all 3 were project B's), and the new test reports **7 vs 2** with the old join. It was
  invisible in dev only because the dev DB had one recipient. Fixed by scoping the join
  (`AND n.project_id = r.project_id`). It leaked a *count*, never content — but the Overview tab
  renders exactly that number, so shipping it knowingly wrong was not an option. Also affected the
  recipient **list**, which has always shown these counts.
- **`repo.Get` was deliberately NOT given the counts.** A new `GetListItem` + service
  `GetWithCounts` was added instead: `repo.Get` sits on the **send hot path** (via
  `CreateIfNotExists` in `sendDirectNotification`), and hanging a `GROUP BY` aggregate off it would
  tax every send to feed one console page. The Dev API's `GetRecipient` stays lean and unchanged.
- **Console structure.** `routes/projects/$id/recipients.tsx` was **moved to
  `recipients/index.tsx`** — TanStack treats `X.tsx` + an `X/` directory as a *layout* route needing
  an `<Outlet/>`, so leaving it would have silently prevented the detail route from rendering. This
  matches the repo's existing convention (`projects.tsx` + `projects/index.tsx`; `projects/$id.tsx`
  is likewise a layout). `routeTree.gen.ts` is regenerated by the **Vite plugin** (`npx vite build`)
  — `npx tsr generate` is not wired up here and fails.
- **9.1's status cell + dialog trigger were EXTRACTED, not duplicated.**
  `NotificationStatusCell`, `MediumStatusLine`, and `DeliveryDetailCell` moved out of
  `notifications_list.tsx` (where they were module-private) into
  `features/notification/components/notification_cells.tsx`, now shared by the project list, its
  broadcast table, and the recipient feed — so a notification reads identically wherever it appears.
- **Contacts modal folded in and DELETED.** `recipient_contacts_modal.tsx` is gone; its body is
  `features/recipient/detail/recipient_contacts_panel.tsx` (the Contacts tab). The row action
  "Contacts" became "View details" (verified live: the dropdown now reads Edit / View details /
  Delete). Phase 1's deviation note — "the console has no recipient *detail page* today, so a
  modal" — is now discharged.
- **Recipient ids link from both places** (`recipient_list.tsx` and the notifications list's
  recipient column) via a shared `features/recipient/recipient_link.tsx`.
- ⚠️ **URL-hostile `external_id`: solved except a literal `/`, which is PRE-EXISTING.** `API_ROUTES`
  `encodeURIComponent`s the id (and `recipients.edit`/`delete` were missing it entirely — added,
  same latent bug); TanStack Router encodes route params on the way out and decodes on read, so the
  id is never string-interpolated into a path. Probed every hostile character against a real chi
  router: `+`, spaces, `#`, `?`, `%`, `@` **all round-trip correctly**. A literal **`/` does not** —
  chi routes on the raw path (so no 404 from mis-segmenting) but `chi.URLParam` hands back the
  still-encoded `a%2Fb`, which then misses the DB lookup. This affects **every** recipient-scoped
  route equally, including the contacts routes the brief cites as the reference implementation, and
  the Dev API. It **fails safe** (a 404, never a wrong-recipient read), so it is recorded, not
  fixed here — fixing it means `url.PathUnescape` on every recipient route, a Dev-API behavior
  change that does not belong in a console phase.
- **Tests (real-Postgres, gated on `TEST_DB_URL`, self-cleaning — the established pattern):**
  `internal/service/recipient_detail_test.go` seeds the **same external id in two projects** (the
  collision the schema permits) and asserts counts + feed are project-scoped, that the feed keeps
  `muted` rows *with* their email delivery, that an unknown/other-project recipient is `ErrNotFound`,
  and that an omitted kind still means direct. `internal/handler/recipient_detail_test.go` drives the
  new endpoint over real HTTP through a chi **v1** router mounted as `routes.go` mounts it (see 9.1's
  chi-version gotcha), including an email-shaped id surviving the URL round trip. The count test was
  confirmed to **fail** (7 vs 2) against the old join before the fix.
- **Untouched (as scoped):** preference *editing* (9.3), analytics (9.5), broader filters (9.4), the
  send path, the worker, all gating logic, the broadcast pipeline, the SDKs, and `docs/`.
  `ListForRecipient` and the Dev API's recipient surface are byte-for-byte unchanged.

### Phase 9.3 — Recipient preference editing (the deferred grid)

- **Goal:** finish what Phase 2 deferred — per-medium (In-App / Email) preference toggles per
  target, on the recipient page 9.2 built.
- **In scope:** turn 9.2's read-only Preferences tab into an editable per-`(target, medium)` grid,
  writing through the **existing** console `PUT /recipients/{recipient_external_id}/preferences`
  (`UpsertRecipientPreferences` — already exists); make the resolved outcome legible — the cascade
  is recipient-exact → recipient-fallback (`topic='any'`) → project-exact → project-fallback →
  default, and the default is **medium-dependent** (`in_app` defaults to deliver; every other
  medium defaults to NOT deliver unless cataloged/enabled). Fixing the READ so it resolves the way
  the send path does is very likely part of this (see the ⚠️ notes in the prompt below).
  - **As built:** the read fix *was* the phase. It landed as a new set-based resolver wired to the
    console read **only** — the old read turned out to be shared with the Developer API, so it
    could not be fixed from a console phase. See the deviations below.
- **Out of scope:** changing any gating semantics (the send path's cascade is correct — it is the
  READ that disagrees with it). New preference *write* endpoints (that path exists); a read-side
  change is expected.
- **Depends on:** **9.2** (hard — this is a tab on that page).
- **Done when:** an operator can flip a recipient's email preference for a target from the console
  and a subsequent send reflects it (`muted` / `failure_reason=preference_disabled`); and every
  cell in the grid states what would ACTUALLY happen on a send, agreeing with
  `ShouldDirectNotificationBeDelivered` — including uncataloged pairs, where "unavailable" is a
  lie (see the measured table in the prompt).
- **Watch for:** do **not** add a parallel write path. Phase 6 established that the authenticated
  toggle, the SDK, and the one-click unsubscribe must all end up writing the *same*
  `(project, recipient, target, medium)` row — that convergence is *why* an unsubscribe and the
  app's settings toggle stay in sync. Breaking it desyncs unsubscribes.
  - ⚠️ **Correction (verified in 9.2):** that convergence is at the **repository** layer, not one
    service method. There are already **two** service entry points, and they are both legitimate:
    the console `PUT` → `PreferenceService.**UpsertRecipientPreference**` (flat payload, checks the
    recipient exists, 201), and the Dev API PATCH / unsubscribe / complaint-suppression →
    `PreferenceService.**UpdateRecipientPreferenceTarget**` (nested `{target, medium, state}`, 200).
    **Both call `repo.Create`**, which is the recipient-level upsert. So the sync property holds.
    9.3 should reuse the console `PUT` as-is and **not** re-route it through
    `UpdateRecipientPreferenceTarget` — that would be churn, not a fix.

```
Read agent-docs/overview.md in full first, esp. the Phase 2 deviations (per-medium preferences +
catalog gating, and the note at overview.md:618-621 that the recipient-facing per-target toggle
GRID was explicitly NOT built for want of a recipient detail page), Phase 6 deviations (the
unsubscribe flip reuses the SAME write path — no parallel disable), and Phase 9.2 (which built the
page this lands on). Implement Phase 9.3.

Build the editable per-(target, medium) preference grid on the recipient detail page's Preferences
tab (9.2 left it read-only).

REUSE, do NOT re-derive (all verified against the code in 9.2):
- **Write path — the console `PUT /console/projects/{project_id}/recipients/{recipient_external_id}/
  preferences` ALREADY EXISTS.** Handler `UpsertRecipientPreferences` → service
  `UpsertRecipientPreference`; body is FLAT `{channel, topic, event, medium, enabled}` (medium
  omitted ⇒ in_app); it 404s if the recipient doesn't exist and returns 201. **One (target, medium)
  per call**, so a grid toggle = one PUT. Use it as-is.
  - Do NOT add a parallel disable/enable path: Phase 6 requires the authenticated toggle, the SDK,
    and the email one-click unsubscribe to all write the SAME `(project, recipient, target, medium)`
    row, which is why unsubscribing and the app's settings toggle stay in sync.
  - ⚠️ The earlier draft of this prompt claimed everything funnels to
    `UpdateRecipientPreferenceTarget`. **It does not.** That is the sibling entry point used by the
    Dev API PATCH / unsubscribe / complaint-suppression (nested `{target, medium, state}`, 200).
    **Both converge on `repo.Create`** (the recipient-level upsert) — the convergence is at the
    REPOSITORY layer. Both are legitimate; do not "unify" them.
- **Read path — `GET /console/projects/{project_id}/recipients/{recipient_external_id}/preferences`
  ALREADY EXISTS** (added in 9.2, `GetRecipientPreferencesConsole` → service
  `GetRecipientProjectPreferences`). It returns the RESOLVED view you need:
  `{preferences: [{target: {channel, topic, event, medium, label?}, state: {enabled, inherited}}]}`
  — the project catalog overlaid with the recipient's overrides. Do NOT drop to
  `ListPreferencesForRecipient` (raw stored rows, no catalog context); the service method already
  uses it internally. Console hook: `useGetRecipientPreferences` (`preference_hooks.ts`); types:
  `RecipientPreferenceTargetState` (`preference_type.ts`).
- **The tab to replace:** `PreferencesTab` in
  `console/src/features/recipient/detail/recipient_detail.tsx` — read-only, renders the resolved
  rows in a `DataTableSmart` with an "Inherited" tag. That is the surface 9.3 makes editable.

Semantics you must render honestly (from Phase 2 — do not re-invent):
- Resolution cascade: recipient-exact → recipient-fallback (topic='any') → project-exact →
  project-fallback → default. The DEFAULT IS MEDIUM-DEPENDENT: in_app defaults to DELIVER (legacy
  "deliver unless muted"); every other medium defaults to NOT deliver unless cataloged/enabled.
  Show the RESOLVED outcome, not just the stored row — a recipient with no stored row is not
  "unset", it's "in_app: on, email: off".
- Only in_app + email are Active() mediums; web_push/sms/mobile_push are scaffolding — don't
  render toggles for mediums that cannot fire.
- ⚠️ **"Uncataloged ⇒ UNAVAILABLE" is WRONG** (earlier drafts of this plan said it; measured
  against the real gating SQL in 9.2):
  | situation | resolves to |
  |---|---|
  | uncataloged `(T, in_app)`, no recipient row | **delivers** (in_app default is true) |
  | uncataloged `(T, email)`, no recipient row | does not deliver |
  | uncataloged `(T, email)`, recipient row `enabled=true` | **delivers** |
  The catalog is a **DEFAULT, not a gate**: an explicit recipient row overrides it (it wins the
  COALESCE). So "unavailable" is a lie for in_app, and a lie for email whenever a recipient row
  exists. Only the resolved value is honest.
- ⚠️ **The read REIMPLEMENTS resolution, more weakly than the send path.**
  `ShouldDirectNotificationBeDelivered` is the authoritative SQL cascade;
  `GetRecipientProjectPreferences` is a **Go exact-match merge** (a `prefKey{Channel,Topic,Event,
  Medium}` map over project prefs) that handles neither the `topic='any'` fallbacks nor the
  medium-dependent default, and **cannot see a recipient row for an uncataloged pair at all** —
  it only iterates project prefs, so such a row vanishes from the response while still
  delivering. A tab labelled "resolved" that disagrees with delivery is the bug this phase exists
  to kill.
- **Recommended fix (decide deliberately, record it):** make the read answer per
  `(target × Active medium)` with `enabled` + `inherited` + `cataloged`, resolved by the SAME
  cascade the send path uses, as **ONE set-based SQL query** in the preference repo. Do NOT loop
  `ShouldDirectNotificationBeDelivered` per cell (20 targets × 2 mediums = 40 round trips), and do
  NOT re-implement the cascade in React — that puts backend semantics on the wrong side of the
  wire (the argument 9.1 used to keep provider normalization server-side). The cost is a second
  SQL resolver to keep in step with the gating one; weigh it and say what you chose.
- **NOT a concern (settled in 9.2):** "revert to inherited" is not a thing to build. A recipient
  preference is just enabled/disabled and defaults to the project default; there is no delete in
  the flow. (`repo.Delete` can remove any row by id, but the resolved read returns no row ids.)
  `inherited` only *matters* if the project default later changes — an explicit `true` keeps
  delivering while an inherited one follows. Display nuance, not a feature.

VERIFY END-TO-END (do not just typecheck): flip a recipient's email preference off in the console,
send a direct notification with an email block to that target, and confirm the delivery row records
`muted` with failure_reason=preference_disabled. That round trip is the "done when".

Console-only. No migration, no new endpoints, no gating-semantics changes. Update Phase 9.3 status
to DONE and add a "Phase 9.3 — deviations (as built)" section.
```

#### Phase 9.3 — deviations (as built)

**No migration**, no new endpoints, no gating-semantics change — as scoped. The send path, worker,
broadcast pipeline, and `ShouldDirectNotificationBeDelivered` itself are byte-for-byte untouched:
this phase changed the READ that disagreed with them. `go build`/`go vet`/the whole suite pass; the
console typechecks, lints (2 pre-existing unrelated warnings), and builds; the grid was driven
**live** in a real browser against the running API + Postgres.

- ⚠️ **THE BIG ONE — "fix the read" could not mean "fix `GetRecipientProjectPreferences`": it is
  SHARED WITH THE DEVELOPER API.** The brief scoped 9.3 console-only and named that service method
  as the broken read. It is also the handler for the Dev API's
  `GET /recipients/{id}/preferences` (`routes.go:118`, API-key auth) — a **documented, SDK-consumed,
  openapi'd** surface. Fixing it in place would have changed the row SET and the resolved values a
  public API returns, from inside a console phase. So:
  - ✏️ **Correction (measured in 9.3.1, against the Resurface checkout):** this section originally
    said the read was "the one Resurface's settings UI reads via `usePreferences()`". **It is not.**
    `usePreferences` appears nowhere in Resurface — the Phase 8 prompt (above) asked for those React
    hooks, but the cutover was built **server-side** instead: `/settings` calls
    `getDigestPreferences()` → two `preferences.**check**()` calls (`web/lib/bodhveda.ts`). The
    read's public-surface argument stands on its own (openapi'd + SDK'd), but the specific Resurface
    claim was wrong, and it propagated into 9.3.1's first draft before being caught. Verify against
    `../resurface`, don't inherit this.
  - **New repo method + new service method + the console handler re-pointed at it.**
    `PreferenceRepo.ResolveRecipientPreferences` → `PreferenceService.ResolveRecipientPreferences`
    → `GetRecipientPreferencesConsole`. The Dev API keeps the old method, unchanged.
  - This follows the repo's own precedent for exactly this situation: 9.2 added `GetListItem`/
    `GetWithCounts` rather than hanging aggregates off `repo.Get` (send hot path), and 9.1
    project-scoped `ListForNotification` rather than duplicating it.
  - 🐛 **The Dev API read is therefore STILL WRONG, on purpose, and recorded here rather than
    quietly fixed.** It has all three defects the brief measured (no `topic='any'` fallbacks, no
    medium-dependent default, uncataloged recipient rows invisible). **This is a real bug with real
    consequences** — it is what a customer's own settings UI renders, so a recipient can be shown a
    toggle state that contradicts what they actually receive. Fixing it is a **public-surface
    change** (the response gains rows and changes values) and belongs with `openapi.json` + an SDK
    bump — i.e. its own phase, sequenced with a version. **Recommend picking this up next**; it is
    the same class of lie 9.3 just killed, one surface over.
  - A characterization test pins the divergence (`the Dev API read still disagrees (why the console
    does not reuse it)`) so nobody re-points the console at it believing they are equivalent. It is
    labelled: **when the Dev API read is fixed, that test's failure is the signal — delete it.**
- **THE DECISION (how to resolve): ONE set-based SQL resolver in the preference repo — the brief's
  recommended option, taken.** `ResolveRecipientPreferences(ctx, projectID, recipientExtID, mediums)`
  answers every `(target × Active medium)` in **one round trip**, with `enabled` + `inherited` +
  `cataloged` (+ `source`, below). Not a per-cell loop (40 round trips), not a React
  re-implementation — the 9.1 argument that backend semantics stay server-side.
  - **Cost paid as promised: a second SQL resolver of one cascade.** The gating query stays
    single-cell because it is on the send hot path. The two are held in step by
    **`TestResolveRecipientPreferencesAgreesWithGating`** (real-Postgres), which seeds a matrix
    hitting every rung and asserts, **cell-for-cell, that the grid equals
    `ShouldDirectNotificationBeDelivered`**. Change one resolver and not the other and it fails —
    that is the whole point, and it is called out in both functions' doc comments.
  - Agreement alone is not enough (both could be wrong the same way), so the test **also pins the
    measured table's literal values** plus `cataloged`/`source` attribution.
  - The LEFT JOINs are safe to write set-based **because** the partial unique indexes
    (`recipient_pref_unique`, `project_pref_unique`, both `medium`-appended since Phase 2) mean each
    cascade rung matches ≤1 row — the structural reason the gating query's `LIMIT 1` needs no
    counterpart. A `no duplicate cells` test guards it if an index is ever dropped.
- **The target universe is the catalog UNION the recipient's own rows** — this is what actually
  fixes "a recipient row on an uncataloged pair is invisible while still delivering". The old read
  iterated project prefs only, so such a row could not appear no matter how the merge was written.
  Verified live (below).
- **Added `source` beyond the brief's `enabled`/`inherited`/`cataloged`** (`recipient_exact` |
  `recipient_any` | `project_exact` | `project_any` | `default`). It is free in the same query and
  it is what makes the cascade *legible* — the phase's own goal. Each cell's tooltip renders it as
  prose ("Following the project's `posts/any/new_comment` default, which covers every topic in
  posts"), so an operator can see the difference between "we set this" and "this is just the
  default". `inherited` is then derived, not stored twice: `source != "recipient_exact"`.
- **`cataloged` is rendered as CONTEXT, never as a gate.** The grid never says "unavailable". An
  uncataloged cell shows a **"Not cataloged"** tag whose tooltip states the honest thing: in-app
  *still delivers* (the catalog is a default), and email is off by default *but an explicit
  preference here overrides that and sends*.
- **New DTO type rather than widening `PreferenceState`.** `dto.ResolvedPreferenceState` is separate
  because `PreferenceState` rides the Dev API's preference responses — adding `cataloged`/`source`
  there would have leaked console fields onto that public surface (the same boundary as the big one
  above). `PreferenceTarget` *is* reused (it already carries `medium` + `label`).
- **`enum.ActiveMediums()` added** (list form of the existing `Active()`), so the resolver
  enumerates mediums from the enum rather than hardcoding the in_app/email pair — when web_push
  becomes active, the grid follows. Only Active mediums get toggles, as scoped.
- **The write path was reused exactly as-is — no third entry point, no re-routing.** A grid toggle
  is one `PUT` (flat `{channel, topic, event, medium, enabled}`). Per the brief's own ⚠️ correction,
  `UpsertRecipientPreference` and `UpdateRecipientPreferenceTarget` were both left alone; they
  converge at `repo.Create`, which is what keeps the settings toggle and the one-click unsubscribe
  writing the same row (Phase 6). Verified live: the console toggle wrote the same
  `(project, recipient, digest/none/sent, email, enabled=f)` row the unsubscribe flips.
  - **No "revert to inherited"**, per 9.2. A cell is enabled/disabled; toggling an inherited cell
    writes an explicit row and its tag flips `Inherited` → `Set`.
- **The console read now 404s for an unknown recipient** (it checks `recipientRepo.Exists`, matching
  the `PUT`'s existing behavior). Previously it returned the bare catalog for a recipient that does
  not exist. Minor, console-only, and symmetric with the write.
- **A write invalidates and re-reads the WHOLE grid rather than patching the toggled cell.** A
  `topic='any'` row decides every exact-topic cell in its channel/event that has no row of its own,
  so one write legitimately moves cells it did not target. Only the server resolves the cascade, so
  re-reading is the honest move. The mutation's `invalidateQueries` is **awaited** in the hook so
  the optimistic switch position is held until the fresh read lands (dropping it any earlier flashes
  the stale value).
- **Console UI.** New `features/recipient/detail/recipient_preferences_panel.tsx` (sibling of 9.2's
  contacts/notifications panels); the read-only `PreferencesTab` in `recipient_detail.tsx` is
  deleted. Rows = targets, columns = In-App | Email, each cell a netra **`Switch`** (radix, from the
  root barrel) + an `Inherited`/`Set` tag + the source tooltip.
  - **A plain `<table>`, NOT `DataTableSmart`.** The 9.2 tab used it because it was a flat list of
    (target, medium) rows; a grid is a *pivot* — one row per target, medium as columns — and the
    cells are interactive, so the sorting/pagination/column machinery buys nothing and fights the
    layout. Grouping the flat API cells by target is done client-side, but that is **presentation
    only**: every resolved value is the server's.
  - No netra `!` override was needed here (nothing fights a netra default class) — the 9.1 gotcha
    still stands for anyone adding one.
- **Honest boundary worth knowing: the grid answers the PREFERENCE rung only.** `fanOutEmail` gates
  preference → provider settings → primary contact, so an email cell that reads ON can still not
  send (`provider_not_configured` / `no_contact`). That is not a lie in the grid — it agrees with
  `ShouldDirectNotificationBeDelivered`, which is the stated contract — and those outcomes have
  their own surfaces (9.1's delivery copy; the Contacts tab already explains `no_contact`). Deciding
  to widen the cell to "would this actually arrive?" would mean joining contacts + settings into the
  resolver and breaking the agreement invariant. Left alone deliberately.

**Live verification (project 3, recipient `123`, against the running API + Postgres + real browser):**
1. The grid's own read immediately exposed the old bug on **real** data: `(digest/none/sent, in_app)`
   resolves **enabled=true, cataloged=false, source=default** — a cell the old read omitted
   *entirely* (no in_app catalog row exists, and it only walked project prefs), while in-app was in
   fact delivering. The old tab showed one row; the honest grid shows two.
2. Flipped **Email OFF** for `digest/none/sent` in the real UI → row written, tag flipped
   `Inherited` → `Set`, toast confirmed.
3. Sent a **direct** notification with an `email` block to that target → response
   `deliveries: [{medium: email, status: muted, failure_reason: preference_disabled}]`; the
   `notification_delivery` row reads `muted`/`preference_disabled`; in-app still `delivered`. Because
   the skip is terminal it never enqueued, so **Resend was never called** (no real email sent).
4. The notifications list renders it in **9.1's prose**: *Email — Muted — "recipient opted out"*,
   visibly distinct from an older row's *"target not cataloged for email"*. The distinction Phase 4
   created `failure_reason` for, end to end.
5. The **measured table reproduced live**, including the two cases that motivated the phase:
   `(uncat/none/ping, email)` with an explicit recipient row → **enabled=true, cataloged=false,
   source=recipient_exact** (uncataloged, invisible before, *delivers*); `(uncat/none/ping, in_app)`
   → enabled=true by default. "Uncataloged ⇒ unavailable" is dead.
6. The dev DB was **restored** afterwards (project 3 had no recipient preferences originally); the
   verification notification remains in history, as sends cannot be unsent.

- **Untouched (as scoped):** the send path, worker, broadcast pipeline, all gating logic, every
  `enum.MediumInApp` call site, the SDKs, and `docs/`. No migration; `routeTree.gen.ts` untouched
  (9.2's route already existed). Analytics (9.5) and broader filters (9.4) are still open.

### Phase 9.3.1 — Developer API preference read fix

- **Goal:** kill the divergence 9.3 deliberately left behind — the Developer API's
  `GET /recipients/{id}/preferences` still answered with the old Go exact-match merge, so a
  customer's own settings UI could render a toggle that contradicted what the recipient received.
- **In scope:** re-point that read (and `/preferences/check`) at the proven resolver; the public
  surface work that makes it a real release — `openapi.json` first, docs prose, SDK bumps + loud
  CHANGELOGs, a publish runbook.
- **Out of scope:** gating semantics (correct), the console (9.3 already honest), 9.4/9.5.
- **Done when:** the Dev API read agrees with `ShouldDirectNotificationBeDelivered` cell-for-cell,
  and 9.3's characterization test — which pinned the divergence — is gone.

#### Phase 9.3.1 — deviations (as built)

**No migration**, no new endpoints, no gating-semantics change. The send path, worker, broadcast
pipeline, and `ShouldDirectNotificationBeDelivered` are byte-for-byte untouched — this phase changed
the READ that disagreed with them, one surface over from 9.3. `go build`/`go vet`/the full suite
pass; the console typechecks; both reads were driven live against the running API + Postgres,
including a real send.

- **THE DECISION (what the public surface exposes): `cataloged` YES, `source` NO.** The brief left
  this open. `cataloged` is exposed because *this change creates the need for it*: the row set now
  includes `(target, medium)` cells the customer never cataloged (they resolve, and can deliver), so
  a settings UI needs to tell a declared catalog entry from a default-resolved cell in order to
  decide what to render. `source` is withheld because it names the resolver's internal cascade rungs
  (`recipient_exact`/`project_any`/…) — publishing it freezes our resolution *algorithm's vocabulary*
  into a permanent contract, and `inherited` already answers the only question a settings UI asks
  ("did the recipient choose this?"). The console keeps `source`: it ships in-repo with the resolver
  and changes alongside it.
  - Shape: `dto.ResolvedPreferenceState{Enabled, Inherited, Cataloged}` is the public read state;
    `dto.ConsoleResolvedPreferenceState` **embeds** it and adds `Source` (embedding keeps the
    console's JSON byte-identical, so no console change was needed). `dto.PreferenceState`
    `{Enabled, Inherited}` now means "the row you just WROTE" and rides the PATCH response only —
    a write describes a row, a read answers a question, and they are no longer the same type.
  - Verified live: the Dev API response carries `cataloged` and no `source`; the console carries
    both; both agree cell-for-cell.
- **The check endpoint had the same bug and a worse one, and is fixed too.**
  `CheckRecipientTargetSubscription` walked stored rows in Go with exact matching: it never
  consulted the `topic='any'` fallbacks, and its step-3 default returned `enabled=true` for **every**
  medium — so an email target that could never fire reported as subscribed. Measured live before/after
  on `brandnew/none/evt`: email `true` → **`false`**; `in_app` stays `true`. Its response gains
  `cataloged`, and now carries the catalog `label` for cataloged pairs (the old code explicitly
  nulled it); both additive.
  - **How it got in, worth remembering:** its step-3 carried a comment asserting *"This must match
    the delivery default in ShouldDirectNotificationBeDelivered, which delivers when no preference is
    found."* That was TRUE when in_app was the only medium. **Phase 2 made the default
    medium-dependent and updated the gating query — but not this hand-rolled mirror of it**, so the
    comment quietly became false while still reading as a justification. A prose invariant pointing
    at another function does not hold anything in step; the cell-for-cell test does. That is the
    argument for one shared cascade, restated by a real bug.
- **A single-target check CANNOT be "the list read, filtered" — this is the one real design
  constraint.** The list's universe is the catalog UNION the recipient's rows; a check must answer
  for **any** `(channel, topic, event)`, including one stored nowhere — which is absent from that
  universe yet still resolves (a project `topic='any'` rule can cover it, and `in_app` delivers by
  default). Filtering would have returned "not found" and forced a re-implemented default in Go —
  the exact bug being fixed. So the repo grew
  `ResolveRecipientPreferenceForTargets(…, targets []dto.Target)`: **the same cascade over a
  caller-named universe.** Both entry points delegate to one unexported `resolvePreferences`; only
  the `target` CTE differs. **Still two SQL resolvers total, not three** — the set-based one and the
  send path's single-cell one — so `TestResolveRecipientPreferencesAgreesWithGating` remains the
  invariant that holds everything in step.
- **No recipient-exists check / 404 on the Dev API read, deliberately.** The console read 404s
  (9.3), and symmetry was tempting. But every Dev API recipient route is behind
  `CreateRecipientIfNotExists` (`routes.go:102`), so the recipient is *guaranteed* to exist there —
  a 404 branch is unreachable code plus a wasted query on every call. Recorded so nobody "fixes" the
  asymmetry.
- **9.3's characterization test is deleted, as its author instructed — and replaced by its
  inverse.** `the Dev API read still disagrees (why the console does not reuse it)` failed on both
  assertions the moment the fix landed (captured before deleting: *"Dev API read now sees uncataloged
  recipient rows"* / *"now models the in_app default"*) — exactly the signal it was written to give.
  Deleting it would have left the fix unpinned, so it became **`the Dev API read agrees with
  gating`** (every returned cell equals `ShouldDirectNotificationBeDelivered`, plus the two cells the
  old read could not express) and **`the Dev API check agrees with gating`** (every known cell, plus
  the unknown-target and `topic='none'` cases that were wrong).
- **🐛 Found + fixed en route: the Go SDK's `inherited` never deserialized.**
  `PreferenceState.Inherit` was tagged `json:"inherit"`; the API sends `inherited`. It has been
  wrong since the SDK's first commit (`8b027dc`) — every Go consumer read `false`, always. Renamed
  to `Inherited` with the right tag. The rename is the point: fixing only the tag would silently
  start feeding real values to code written around a field that was always false, so a compile error
  at the call site is the safer break. Called out in the Go CHANGELOG as source-breaking.
- **Docs carried the same lie and were corrected, not just extended.**
  `docs/concepts/preferences.mdx` said a `(target, medium)` "must be declared before that medium can
  fire" and that email "only fires when the pair is cataloged **and** the recipient enabled it" —
  both contradicted by the measured cascade (an explicit recipient rule wins *before* the catalog is
  consulted, so an uncataloged email rule sends). Replaced with a "How a preference resolves" section
  stating the five rungs, the medium-dependent default, and **the catalog is a default, not a gate**.
  `openapi.json` was updated FIRST (the MDX renders from it), per the established pattern.
- **Versioning: minor across the board** (js core/react `0.1.0`→`0.2.0`, go `sdk/go/v0.2.0`→`v0.3.0`),
  runbook at **`agent-docs/release-preference-read-fix.md`** (a NEW file — `release-email-medium.md`
  is the record of what shipped in Phase 7.5 and was left alone). **Nothing is published or
  deployed**; those steps are irreversible/credential-gated and are the human's.
  - ⚠️ **The SDK bumps are types + docs only. The behavior change ships with the API** — customers
    get it whether or not they upgrade. That asymmetry is why the CHANGELOGs lead with the behavior
    change rather than the type change.
  - **Resurface's measured impact: almost certainly none** — and the reasoning is worth keeping,
    because the *first draft of this section claimed the opposite*. It inherited 9.3's wrong claim
    that Resurface's settings UI reads `usePreferences()` (it does not; the string is absent from
    that codebase — see the correction in the 9.3 deviations). It reads via **`check`**, and old ==
    new for every case it can hit: `topic: none` can't take an `any` rule, and its backfill wrote
    explicit rows for every user, so `check` resolves `recipient_exact` either way. The lesson is
    the phase's own: **a claim about a downstream consumer is not verified until you open that
    consumer's code.** Full analysis in `agent-docs/release-preference-read-fix.md` §6.

**Live verification (project 3 `Resend`, recipient `123`, real API + Postgres + a real send):**
1. **The bug reproduced on untouched real data.** Project 3 catalogs `digest/none/sent` for **email
   only** — no in_app row. The old read returned **one** cell; the fixed read returns **two**, the
   second being `(digest/none/sent, in_app)` → `enabled=true, cataloged=false` — a cell the old read
   omitted entirely *while in-app was actively delivering*. Same shape 9.3 hit; now on the public API.
2. **Read ↔ delivery agreement proven through a real send.** Flipped email off via the Dev API
   `PATCH`; the read said `email: enabled=false, inherited=false, cataloged=true` and
   `in_app: enabled=true`. A direct send with an `email` block then returned
   `deliveries: [{medium: email, status: muted, failure_reason: preference_disabled}]` and the
   notification row went `delivered` — **both mediums exactly as the read predicted.** Project 3 has
   a live Resend secret + a real primary contact (`ceoshikhar@gmail.com`), so the muted path was
   chosen deliberately: the skip is terminal, so it never enqueued and **Resend was never called**
   (no real email sent).
3. **The marquee case, live:** an explicit recipient rule on the **uncataloged**
   `(secret/none/ping, email)` → read shows `enabled=true, cataloged=false` — invisible to the old
   read, and it *delivers*. `(secret/none/ping, in_app)` → `enabled=true` by default.
4. **The check endpoint's medium-dependent default, live:** unknown target `brandnew/none/evt` →
   `email: false` (was `true`), `in_app: true`.
5. The PATCH response was confirmed to still carry `{enabled, inherited}` with **no** `cataloged` —
   it describes the written row, not a resolution.
6. The dev DB was **restored** (project 3's recipient preferences deleted, verification API key
   deleted); the verification notification remains, as sends cannot be unsent.

- **Dead code the fix orphaned was removed, not left lying.** Both rewritten methods were
  `ListPreferencesForRecipient`'s only callers, so it went (repo impl + interface), and with it the
  now-unpopulated `PreferenceSearchFilter.RecipientExtID` field and its branch in `findPreferences`.
  Also dropped `dto.PatchRecipientPreferenceTargetResult`, a type alias that was **already** dead at
  HEAD (nothing ever referenced it). All `internal/`, so no external consumer. `PreferenceTarget`,
  `PreferenceTargetDTOFromPreference` and `PreferenceTargetStateDTOFromPreference` stay — the PATCH
  response still uses them. Verified after removal: the project-preference list (the surviving
  `findPreferences` caller) and both reads still serve correctly.
- **Untouched (as scoped):** the send path, worker, broadcast pipeline, all gating logic, the console
  (its read already shared the resolver; typechecks clean), and every write path — `repo.Create`
  remains the single convergence point that keeps the settings toggle and the one-click unsubscribe
  writing the same row (Phase 6). No migration. Analytics (9.5) and list filters (9.4) still open.

### Phase 9.4 — Notification list filters

- **Goal:** the notifications list is usable on a project with real volume.
- **In scope:** extend `dto.ListNotificationsFilters` (`dto/notification.go:551` — today just
  `{ProjectID, Pagination, Kind}`) and the `pg` query with filters for status, target
  (channel/topic/event), medium + delivery status, date range, and recipient search; matching
  console filter UI (netra `date_picker`/`select`/`toggle_group`); URL-synced filter state via
  TanStack Router search params so a filtered view is shareable/bookmarkable.
- **Out of scope:** analytics aggregates (9.5); saved views.
- **Depends on:** nothing hard (9.1 makes the medium/delivery-status filter more useful).
- **Done when:** an operator can answer "show me every bounced email on `digest/none/sent` last
  week" from the list; filter state survives a page reload and is shareable as a URL.
- **Watch for:** filtering on the email medium means filtering on a **joined** `notification_delivery`
  row, not a `notification` column — check the query plan and index coverage (`ix_nd_email_status_time`
  is a partial index `WHERE medium='email'`, from Phase 4). Don't turn the list into a seq scan.

```
Read agent-docs/overview.md in full first, esp. "Phase 9 — Console" and the Phase 4 deviations
(the notification_delivery schema + its indexes). Implement Phase 9.4 (Notification list filters).

Today `dto.ListNotificationsFilters` (internal/model/dto/notification.go:551) is
`{ProjectID, query.Pagination, Kind, RecipientExtID}` — 9.2 added `RecipientExtID` and switched
`repository.NotificationRepository.ListNotifications` to take the FILTERS DTO instead of a parameter
list, precisely so 9.4 extends the struct with no signature churn. Beyond those, the list is
unfiltered; on a project with real volume it is unusable.

Add filters: status, target (channel/topic/event), medium + delivery status, date range, and
recipient search. Extend the DTO + the pg query + the console UI (netra has date_picker, select,
toggle_group). Sync filter state to the URL via TanStack Router search params so a filtered view is
shareable and survives reload.

PERFORMANCE — this is the actual risk: filtering by email/delivery status means filtering on a
JOINED notification_delivery row, not a notification column. Phase 4 shipped `ix_nd_email_status_time`
as a PARTIAL index (WHERE medium='email'); `ix_nd_notification` and `ix_nd_project_recipient` also
exist. EXPLAIN your query — do not turn the list into a seq scan, and do not add an index without
first checking whether an existing partial one covers the predicate. If you need a new index, that
IS a migration (Goose, applied manually) — the only sub-phase of Phase 9 that might need one.

Note the existing list already attaches email deliveries via a SECOND batch query keyed by
notification ids (pg/notification.go ~L356), not a join in the main query. Filtering by delivery
status may force that into the main query — if so, do it deliberately and record the tradeoff
(the batch-query design exists so a notification with no email simply has no row; a naive INNER
JOIN would silently drop every in-app-only notification).

Follow the layered handler→service→pg pattern. Update Phase 9.4 status to DONE and add a
"Phase 9.4 — deviations (as built)" section recording the final filter set + any index work.
```

#### Phase 9.4 — deviations (as built)

**One migration** — `migrations/20260717120000_add_notification_project_id_index.sql` (Goose,
`NO TRANSACTION` + `CREATE INDEX CONCURRENTLY`; applied manually, no runner is wired in). 9.4 is
the Phase 9 sub-phase that needed one, as the brief guessed — but **not for the reason it gave**
(see the index section). The send path, worker, broadcast pipeline, and all gating logic are
untouched. `go build`/`go vet`/the whole suite pass; the console typechecks, lints (the same 2
pre-existing warnings 9.3 recorded), and builds; every filter was driven **live in a real browser**
against the running API + Postgres, on a seeded 400k-row dataset **and** on the real project 3.

**THE FINAL FILTER SET** (`dto.ListNotificationsFilters`, extended in place — no signature churn,
exactly as 9.2 set it up for):

| filter | param | shape |
|---|---|---|
| in-app status | `status` | one `enum.NotificationStatus` |
| target | `channel`, `topic`, `event` | exact match, each independent |
| email medium + delivery status | `email` | `none` \| `any` \| a `DeliveryStatus` |
| date range | `created_from`, `created_to` | RFC3339 instants, inclusive |
| recipient search | `recipient_search` | case-insensitive substring |
| (9.2, unchanged) | `recipient_id` | exact match |

- **THE DECISION (batch query vs. join): the batch query STAYS, and the filter is an `EXISTS`
  subquery.** The brief expected a delivery-status filter might force the join. It does not, and
  the distinction it drew is the reason why: **narrowing the row set and projecting the email data
  are different jobs.** The `EXISTS` narrows (only when asked); the batch query projects (always,
  and narrows nothing). So the main query still touches `notification` alone, in-app-only rows are
  never dropped by a filter that wasn't about email, and the plan for an unfiltered list is
  byte-for-byte what it was. A `LEFT JOIN` would have been the correct *join* answer but a worse
  one: it changes the plan for every request to serve the minority that filters on email, and it
  re-opens the "what does a NULL delivery mean" question that `EXISTS`/`NOT EXISTS` answers
  structurally.
- **There is deliberately NO `medium` filter, and that is the honest reading of "medium + delivery
  status".** In v1 the two dimensions are not independent: `email` is the only medium with a
  `notification_delivery` row at all — Phase 4 kept the in-app outcome on the `notification` row —
  so a `medium=in_app` control could only ever mean "every notification", i.e. a filter that lies.
  The medium dimension is instead expressed by *which* control you use (`status` is in-app's;
  `email` is email's), and `email=none`/`any` carries the "was email attempted" question. This is
  the same class of fix as 9.3's "uncataloged ⇒ unavailable is a lie": the control set now matches
  what the data can actually answer.
  - **`email=none` is the load-bearing one.** It is the anti-join (`NOT EXISTS`) that makes
    in-app-only notifications — still the common case — **findable**, not merely un-dropped. Proven
    live: on the seeded project it returns exactly 140,000 of 200,000 rows (the 60k that carried
    email excluded), every row rendering an In-app line and no Email line.
- **The console offers only the delivery statuses that can occur; the API accepts every legal one.**
  Four of the twelve `DeliveryStatus` values (`sending`, `suppressed`, `quota_exceeded`, `rejected`)
  are reserved and never written in v1 (Phase 4), so offering them as filters would imply data that
  cannot exist. `enum.EmailDeliveryFilter.Valid()` still accepts them — the API validates against
  the CHECK constraint, the UI against reality.
- **Single-select per filter, not multi.** One `status`, one `email`. Multi-select would mean array
  search params, a bigger URL grammar, and a `= ANY($n)` per filter, to answer questions
  ("failed OR quota_exceeded") nobody has asked yet. Recorded as the cheap thing to revisit.
- **An impossible filter value is a 400, not an empty list.** `dto.ListNotificationsFilters.Validate()`
  rejects a `status`/`email`/`kind` that no row could hold, and an inverted date range. Silently
  returning zero rows for a typo reads as "you have no such notifications", which is a lie of the
  same family this phase exists to remove. It also **normalizes**: blank params (`?status=`) are
  treated as absent, and the external-id filters are lowercased (external ids are stored lowercase —
  the rule 9.2 recorded). The service's old ad-hoc lowercasing moved into it.
  - Blank-is-absent is not hypothetical: `httpx.DecodeQuery` decodes `created_from` into a
    `*time.Time`, and an empty `?created_from=` is a hard **400** from gorilla/schema, not an
    ignored param. The console omits empties for that reason; the DTO collapses the rest.
- **`recipient_search` escapes LIKE wildcards.** This is the repo's **first** use of dbx's
  `AddContainsFilter`, and dbx does not escape `%`/`_`/`\`. External ids are customer-chosen and
  very often contain `_` (`user_1`), which LIKE reads as "any single character" — so searching
  `user_1` also matched `userX1`, and a lone `%` matched everything. Escaped in the pg layer
  (`escapeLikeNeedle`; Postgres' default LIKE escape is a backslash, so no ESCAPE clause is needed).
  It is a precision bug, not injection — the needle was always a bind parameter. Pinned by a test
  that was **confirmed to fail** against the unescaped version.

**INDEX WORK — the brief pointed at the wrong index, and missed a live bug.**

- 🐛 **`notification` had NO index beyond its primary key.** Not on `project_id`, not on anything.
  So *every* console notifications list — the one shipped today, with no filters at all — is a
  sequential scan of the whole table, narrowed to one project afterwards. It hides in dev (a handful
  of rows) and hides in the plan whenever the project being listed happens to own the highest ids,
  because then walking `notification_pkey` backward stops at LIMIT and looks perfect. **It is not
  perfect for any other project:** their rows sit behind that one in id order. Measured on 400k rows
  across two projects, listing project 3 (4 rows, lowest ids):
  | | plan | buffers | time |
  |---|---|---|---|
  | before | Parallel Seq Scan | 4707 | 14.3 ms |
  | after `ix_notification_project_id` | Index Scan | **4** | **0.058 ms** |
  This is a pre-existing bug 9.4 did not introduce — but "the list is usable on a project with real
  volume" is this phase's stated goal, and shipping filters onto a seq scan would not have met it.
  - **Someone had already noticed.** A commented-out
    `CREATE INDEX ... ON notification (id DESC, project_id, recipient_external_id)` had been sitting
    in `pg/notification.go` above `ListForRecipient` for a long time, never applied — because a
    comment is not a migration and no runner is wired in. (Its leading `id DESC` could not seek to a
    project anyway, so it would not have fixed this.) Replaced with a note pointing at the real
    migration, and at the fact that `ListForRecipient` — the Dev API inbox, out of scope here —
    still has no index of its own beyond the `project_id` prefix it can now borrow.
- **The one index added: `ix_notification_project_id (project_id, id DESC)`** — the universal prefix
  of every list query (all of them filter `project_id = ?` and order by `id DESC` with a LIMIT), so
  it serves the scan, the ordering and the early stop at once. `CONCURRENTLY` because `notification`
  is on the **send hot path** and a plain `CREATE INDEX` would take ACCESS EXCLUSIVE and block every
  send for the build (Phase 2's precedent). Verified to apply *and* roll back through goose.
- ⚠️ **`ix_nd_email_status_time` does NOT contain `status`, despite its name.** It is
  `(project_id, created_at DESC) WHERE medium='email'`. The brief (and this doc) cite it as the
  partial index covering a delivery-status predicate; it cannot. **No new delivery index was needed
  anyway** — the `EXISTS` is keyed by `notification_id`, so **`ix_nd_notification` serves it**
  (measured: `email=bounced` → Index Scan Backward on `ix_nd_notification`, 0.085 ms). The brief's
  own instruction — check whether an existing partial index covers the predicate before adding one —
  is what surfaced this: the answer was "no, but a different existing index does". Left as-is; the
  misleading *name* is recorded here rather than renamed, since renaming an index is its own
  migration for zero behavioral gain.
- **Measured and DECLINED: an index on `(channel, topic, event)` or on `created_at`.** Every filter
  answers in ≤ ~45 ms on 400k rows, and most are sub-millisecond; the two slower shapes are a
  deliberately broad old date window (43 ms — walking id DESC to reach an 11-month-old window) and
  the full target+date+email combination (~12 ms). Each additional index taxes **every INSERT on the
  send path** to speed one console filter combination. Same instinct as 9.2 refusing to hang
  aggregates off `repo.Get`. Revisit only with a measurement.
- **Untouched and still true: the paginated `COUNT(*)` is the most expensive part of a large list**
  (~19 ms / 200k rows, a seq scan — it must count every match). Pre-existing to offset pagination,
  not something filters changed. Noted for 9.5, which will face it directly.

**CONSOLE — built on the uncommitted URL-sync work already in the tree.**

- ⚠️ **There was uncommitted WIP when this phase started**, beyond the recipient-detail files the
  brief warned about: a console-wide "sync the selected view to the URL" pass (new untracked
  `console/src/lib/search.ts` with `validateViewSearch`, applied to the notifications `kind` toggle,
  the preferences kind, and the recipient-detail tab). 9.4 **built on it rather than around it** —
  the notifications route's `validateSearch` grew from that one-key helper into
  `validateNotificationSearch` (kind + every filter), and `lib/search.ts` gained the optional-param
  readers. The route-owns-URL-state / feature-takes-value-and-onChange convention is that WIP's; it
  is now also 9.4's.
- **Filters live in the URL; the PAGE does not.** A shared link is a shared *question*, and page 5
  of it is not part of the question. Pagination stays local table state.
- 🐛 **Fixed while verifying: filtering left the pager claiming a page it wasn't on.** Resetting to
  page 1 on a filter change is obvious (otherwise you land on page 5 of a shorter list and read the
  empty table as "no matches"). The non-obvious half: **`DataTableSmart` seeds its pagination from
  `state.pagination` ONCE**, via a `useState` initializer, and thereafter owns it — publishing
  upward through `onStateChange` but never reading the prop again. So the reset reached the *query*
  (which correctly refetched `page=1`) but not the table's own pager, which went on displaying
  "Page 4 of 2500" above page-1 rows. Fixed by keying the table on the filter signature so it
  re-seeds; the key deliberately **excludes** the page, or paging would remount and bounce back to
  page 1 forever (verified: 1→2→3→4 while filtered, filters holding). No other console list hits
  this because nothing else ever moved their page from the outside.
- 🐛 **Fixed while verifying: numeric-looking filters were silently dropped.** TanStack Router
  *parses* the search string, so `?recipient_search=12` arrives as the **number** 12 and
  `?channel=true` as a boolean. The first cut's `typeof value === "string"` guard rejected those as
  malformed and dropped them — and recipient external ids are customer-chosen strings that are very
  often all digits. It was invisible against a project whose row count happened to equal the
  unfiltered count, and only showed up on the **real** project 3, whose one real recipient is
  literally named `123`. `optionalStringSearch` now coerces number/boolean back to text. (TanStack
  then re-serializes the string as `?recipient_search=%2212%22` to preserve the type on round-trip —
  ugly, correct, and it reloads and shares fine. Left alone.)
- 🐛 **Fixed while verifying: a date RANGE could never be picked — only its last day.** netra's
  `DatePicker` wraps `@rehookify/datepicker` in `mode="range"`, where a **two-date selection means
  "complete"** and the next click starts a new range. The first cut closed the range on the first
  click (`to = from`, to keep it a "valid single-day range") and fed that back in as the picker's
  state — so the second click always started over, and `from == to == the day you clicked last`. A
  half-picked range is now an **open-ended `from` with no `to`** ("on or after that day"), which is
  both a legitimate filter and the state the picker needs to complete. Verified live: click 10 →
  `?from=2026-07-10`; click 14 → `?from=2026-07-10&to=2026-07-14`.
- **Dates are calendar DAYS in the URL, INSTANTS on the wire.** `?from=2026-07-10` is readable and
  shareable; the console converts it to the viewer's local start-of-day / end-of-day before calling
  the API, so "last week" means the operator's week, not UTC's. End-of-day is computed as
  next-day-minus-1ms rather than a hardcoded `23:59:59.999`, so a DST transition can't clip an hour.
  Verified live: a `to=Jul 14` range returns rows sent at `Jul 14, 23:35`, which a naive
  `T00:00:00` bound would have excluded. (A link shared across timezones resolves to the reader's
  own days — the right reading of a date picker, and recorded so it isn't mistaken for a bug.)
- **Free-text filters are debounced (300 ms)**; without it every keystroke is a router navigation, a
  refetch, and a history entry (measured: typing `digest` = 6 keystrokes = **1** request). The
  inputs keep a local mirror while typing and re-sync when the prop moves on its own (Clear, back
  button), so the URL stays the source of truth — but the re-sync must ignore **the echo of its own
  write**, or the trimmed value coming back erases a trailing space mid-typing and the next
  character lands in the wrong place (`digest ` + `x` → `digestx`). Guarded by comparing against the
  trimmed local value; verified live.
- **The Phase 5 "Email delivery" KPI row is deliberately NOT filtered.** It is the project's
  lifetime email picture; silently re-scoping it to the current filter would make two different
  numbers look like the same one. (Filtered aggregates are 9.5's job.)
- **Switching kind PRESERVES the filters rather than clearing them.** The broadcast table is served
  by a different endpoint (`/broadcasts`, unfiltered — out of scope for a *notification* list phase)
  and shows no filter bar, so the selection is simply dormant and switching back restores it.
- **`useNotifications` took an options object** instead of growing to a sixth positional parameter —
  the same instinct 9.2 applied to `ListNotifications` on the Go side. Two call sites, both updated.

**Live verification (real browser, real API, real Postgres):**
1. On a **seeded 200k-row project**: unfiltered, `email=none` (140,000 rows, no Email lines),
   `email=bounced` (every row Email/Bounced), and the brief's own question —
   *bounced email on `digest/none/mention`* — all correct.
2. **The filtered URL survives a reload** and, opened in a **fresh browser context**, renders the
   same 10 rows — i.e. it is genuinely shareable, not just restored from local state.
3. A hand-edited `?status=bogus` is **dropped** from the URL and the page renders unfiltered (the
   route drops unrecognized optional params; the API would 400 an actual request).
4. Interactively: typing in Channel updates the URL and the rows; the date picker builds a real
   range; **Clear** ("Clear filter" / "Clear N filters") removes them and restores the full list.
5. On the **real project 3**: every filter behaves, `email=none` correctly returns 0 (all 4 of its
   notifications carried email) and renders "No results." rather than looking broken, and
   `recipient_search=999` returns 0 while `123` returns 4 — which is how the numeric-coercion bug
   above was caught.
6. **The dev DB was restored**: both seeded projects deleted, back to the original 3 projects /
   4 notifications / 4 deliveries / 1 recipient / 1 preference, with the same delivery-status mix.
   The new index remains — it is the migration. (Sequences are advanced; harmless.)

- **Tests:** `internal/service/notification_filters_test.go` (real-Postgres, gated on `TEST_DB_URL`,
  self-cleaning — the established pattern). The fixture gives every filter a match and a non-match,
  and pins the things that would silently rot: **in-app-only rows survive every non-email filter**
  (a JOIN regression fails here), `email=none` finds exactly them, `email=<status>` selects on the
  *delivery* row and not the notification's own (the fixture's bounced row is deliberately in-app
  `delivered`), filters compose as AND, the paginated **total agrees with the rows** (a COUNT that
  loses the filters offers pages that render empty), blank params don't narrow, external ids are
  lowercased, impossible values are rejected, and the `EXISTS` **cannot reach across projects**.
- **Untouched (as scoped):** the send path, worker, broadcast pipeline, all gating logic, the SDKs,
  `docs/`, the Developer API (this is console-only — `ListNotificationsFilters` is not a public
  surface), and the broadcast list. Analytics (9.5) is still open, and now has a measured COUNT
  problem waiting for it.

### Phase 9.5 — Analytics

- **Goal:** replace lifetime scalars with something that shows **trend and breakdown** — the only
  sub-phase needing genuinely new aggregate queries.
- **In scope:** new console aggregate endpoint(s) over `notification` + `notification_delivery`
  grouped by day/status/medium/target with a **date range** (copy the shape of
  `EmailDeliveryOverviewForProject` — per-status `count(*) FILTER (...)` — from Phase 5); rebuild
  `features/home/home.tsx` on real stats (today it is four lifetime scalars sourced by fetching
  **every project** and `.find()`ing the current one at `home.tsx:41` — there is no stats endpoint
  at all); time-series charts (send volume, delivery outcomes over time), per-medium in-app vs email
  comparison, per-target breakdown (which targets actually fire), and delivery-health signals
  (bounce/complaint rate — the numbers that predict a sender-reputation problem, which is the exact
  risk BYO-first exists to manage).
- **Out of scope:** a metrics pipeline/rollup tables (aggregate live first; only add rollups if
  measurements justify it); billing/usage analytics (`BillingService` already meters separately).
- **Depends on:** nothing hard, but sequence last (most backend work; benefits from 9.1's widened
  projection).
- **Done when:** the Home page shows send volume over a selectable range with per-medium and
  per-status breakdowns, and a project with zero email still renders sensibly.
- **Watch for:** the existing `email_delivery_overview.tsx` deliberately **self-hides** until the
  project has attempted ≥1 email — in-app-only projects (still the common case) must not be shown a
  wall of empty email charts. Preserve that instinct. Also: **email `opened` is a soft signal**
  (Apple MPP inflates it, blocked images deflate it) — it is directional only, and in-app `read` is
  the trustworthy number. Never chart them as if they were the same kind of fact.
- **9.4 measured something you inherit:** the paginated `COUNT(*)` over a project's notifications is
  a seq scan (~19 ms / 200k rows) because it must count every match. Aggregates face that head-on.

```
Read agent-docs/overview.md in full first. It is the shared brain across sessions and it now
describes only what is BUILT: the sections above Phase 9 are the architecture, the domain model,
and the decision log — Phases 0–8's plans and deviation notes were removed once shipped, with
everything of theirs that still matters folded up into those sections (`git log` has the rest).

Read especially:
- "The core domain model" → the `notification_delivery` schema + its indexes + the DeliveryStatus
  enum (incl. which statuses are RESERVED and never written — don't chart what cannot exist), and
  the ⚠️ that `ix_nd_email_status_time` is misnamed and has no `status` column.
- "Console" → `email_delivery_overview.tsx` is the per-status `count(*) FILTER (…)` aggregate
  pattern to copy, and the ⚠️ that `ListForRecipient` is the recipient's inbox, not an operator view.
- "Design decisions" → email `opened` is a SOFT signal; in-app `read` is the trustworthy one.
- "Conventions" → the `notification` table is on the SEND HOT PATH (one index; measure before
  adding another), the netra `!` cascade gotcha, chi v1, and the real-Postgres test pattern.
- The Phase 9.1–9.4 deviations — the worked examples that matter. **When the brief and the CODE
  disagree, the CODE wins, and you record the divergence rather than bending the code.** Every one
  of 9.1–9.4 found a real mismatch ONLY by running things against real data.

Implement Phase 9.5 (Analytics). This is the only console sub-phase needing genuinely new
aggregate queries.

Today there is NO stats endpoint. `features/home/home.tsx` renders four LIFETIME scalars and gets
them by calling `useGetProjects()` — fetching EVERY project and `.find()`ing the current one
(home.tsx:41). The only other analytics is `email_delivery_overview.tsx`: per-status counts,
project-wide, lifetime, self-hiding. Nothing has a time dimension.

Build:
1. Console aggregate endpoint(s) over notification + notification_delivery grouped by
   day/status/medium/target, with a DATE RANGE. Copy the shape of
   `EmailDeliveryOverviewForProject` (per-status `count(*) FILTER (...)`) — it's the established
   pattern. Aggregate LIVE; do NOT build rollup tables or a metrics pipeline unless you measure a
   reason to (record the measurement if you do).
2. Rebuild home.tsx on it: send volume over a selectable range, in-app vs email per-medium
   comparison, per-target breakdown (which targets actually fire), and delivery-health
   (bounce/complaint rate — the numbers that predict a sender-reputation problem, the exact risk
   BYO-first exists to manage; see the decision log).

REUSE, do NOT re-derive (verified against the code at 9.4):
- **In-app and email outcomes live in DIFFERENT PLACES, and this is the whole shape of the
  problem.** In-app status is a scalar on the `notification` row; email lives in a
  `notification_delivery` row that only exists when the send carried an `email` block. So
  "notifications by status" and "emails by delivery status" are two different aggregates over two
  different tables — NOT one GROUP BY over a join. A naive join drops every in-app-only
  notification, still the common case. 9.4 hit this exact wall and answered it with EXISTS; read
  its deviations before designing the aggregate.
- **`enum.ActiveMediums()`** is the list of transports that can fire (in_app + email) — enumerate
  from it rather than hardcoding the pair.
- **The date-range plumbing already exists**: `dto.ListNotificationsFilters` carries
  `CreatedFrom`/`CreatedTo` as RFC3339 instants, and the console converts a picked calendar DAY to
  the viewer's local start/end-of-day (`notification_filters.ts`, `lib/search.ts`). Reuse that
  convention — a shared link must mean the same range on reload, and `?created_from=` BLANK is a
  hard 400 (gorilla/schema decodes it into a *time.Time), so omit rather than blank.
- **URL-synced view state**: routes own it via `lib/search.ts` and pass value + onChange down. A
  selected analytics range belongs in the URL for the same reason a filter does.

PERFORMANCE — EXPLAIN your aggregates against realistic volume before shipping; do not trust a
4-row dev DB. 9.4's method, which worked: seed a THROWAWAY project (200k+ notifications, ~30%
with deliveries, a realistic status skew, and `created_at` CORRELATED with `id` the way production
has it), seed a SECOND large project so neither dominates the planner (a single-project table makes
every plan a lie), EXPLAIN (ANALYZE, BUFFERS), then DELETE the seed and confirm the DB matches its
original counts. `notification` now has `ix_notification_project_id (project_id, id DESC)` — the
universal prefix of a project-scoped query. Any new index is a Goose migration (applied manually,
NO runner is wired in) and is paid on every INSERT on the send path: measure, then decide, and
record what you declined.

CHARTS — netra ALREADY SHIPS CHARTING. `ChartContainer`, `ChartTooltipContent`,
`ChartLegendContent`, `tooltipCursor`, `axisDefaults` are re-exported from netra's root barrel
(components/chart/chart), wrapping recharts ^2.15.4 (a netra dependency). DO NOT add a charting
library. If you import recharts primitives directly rather than via netra's wrapper, add `recharts`
to console/package.json explicitly rather than relying on the transitive install — Cloudflare's
fresh `npm ci` is where that kind of drift surfaces (see [[project-console-cloudflare-deploy]]).
Load the `dataviz` skill before writing chart code.

HONESTY RULES (non-negotiable — the data has known biases and the console must not launder them):
- Email `opened` is a SOFT signal: Apple Mail Privacy Protection pre-fetches pixels (inflates),
  blocked images deflate. In-app `read` is the trustworthy signal. Never chart them as the same
  kind of fact; reuse `OPEN_SOFT_SIGNAL_COPY` (delivery_copy.ts) rather than rewording the caveat.
- Preserve the self-hiding instinct: email_delivery_overview.tsx deliberately shows NOTHING until
  the project has attempted ≥1 email. In-app-only projects are still the common case — they must
  not get a wall of empty email charts. A project with zero email must render sensibly.
- Never chart a DeliveryStatus that v1 never writes (`sending`, `suppressed`, `quota_exceeded`,
  `rejected`) — an axis with a permanent zero implies data that cannot exist.
- Decide deliberately whether the existing lifetime KPI row and a new ranged chart can coexist
  without reading as the same number twice. 9.4 left that KPI row UNFILTERED for exactly this
  reason; if 9.5 makes it ranged, say so and make it obvious in the UI.

VERIFY END-TO-END — do not just typecheck. 9.1–9.4 each found real mismatches ONLY by driving the
real thing. Stack: db/redis run in docker (`docker compose up -d db redis`); `.env` has the ports
(db 42070, redis 6776, user `postgres` — do NOT export your own DB/Redis URLs). Check whether
:1338/:6970 are already held — `make dev` runs the api under `air` in tmux and hot-reloads your
changes, but a stale non-air server would serve OLD code and you would "verify" nothing (confirm
with a request that only new code could answer). Console is Google-OAuth-only: borrow a session
token (`select token from sessions where expiry > now() order by expiry desc limit 1`), cookie name
`session`, then curl with `-b "session=$TOKEN"` and/or Playwright with that cookie on domain
`localhost`. Drive the charts in a browser and LOOK at them.
⚠️ Project 3 has a LIVE Resend secret + a real primary email contact (`ceoshikhar@gmail.com`). A
send with an `email` block to an ENABLED email target sends a REAL email. If you need a send, use a
muted/disabled path (terminal skip ⇒ never enqueued ⇒ Resend never called). Restore any data you
seed.

Do NOT touch the send path, worker, broadcast pipeline, or any gating logic. Follow the layered
handler→service→pg pattern; add endpoints to `API_ROUTES` in lib/api.ts, never hardcoded. Update
Phase 9.5 status to DONE and add a "Phase 9.5 — deviations (as built)" section recording the
endpoint shape, whether live aggregation held up (with the measurement), any index work, and the
final chart set.
```
