# Delivery feedback — design note

**Status:** Unit 1 **BUILT** (2026-07-29, see §2.6 for what shipped). Unit 2 not started.
**Scope:** Unit 1 — infra alerting (Discord). Unit 2 — delivery tree in the console.
**Shelved:** outbound webhooks, public delivery-status API. See §5.

**Context that set the scope:** Bodhveda has no external customers yet — the author is the
only user, via Grahak. So there is no one to receive a webhook and no one to poll a public
delivery API. What is actually needed is: *tell me when my own infra is broken*, and *let me
see what happened to a broadcast*.

---

## 0. The incident, and what it means for the design

For roughly a day the worker archived ~98% of its tasks (asynq `default`: 168 processed /
164 failed, 57 remaining all archived). Grahak noticed nothing: its per-minute job kept
reporting `sent: N`, its logs were clean, no email reached a human. Found by opening the
console by hand.

**What that looked like in the database is not what you would guess.** Since Phase 10 the send
path does one `notification` INSERT and enqueues `notification:delivery`. Everything else —
recipient upsert, in-app gating, billing, the whole email fan-out — happens in the worker.
`fanOutEmail` is what *creates* the `notification_delivery` row.

So when the worker archives the task:

| Send shape | `notification.status` | `notification_delivery` |
|---|---|---|
| mixed (`payload` + `email`) | stays `enqueued` | **no row at all** |
| email-only (no `payload`) | `not_requested` | **no row at all** |

Two consequences that drive everything below:

1. **The incident signature is an ABSENCE, not a failure.** There is no `failed` delivery row
   to find, because the code that writes delivery rows never ran. Anything that only reports
   "deliveries that failed" reports *nothing* during the exact incident it exists to catch.
   This is why the alerting watches the **queue and the worker**, not delivery rows.
2. **The email-only case is the nastier blind spot.** `not_requested` is set at INSERT and
   never resolved — it is terminal-*looking*. An email-only send whose task was archived is,
   on the `notification` row alone, indistinguishable from one that worked.

---

## 1. What exists today (verified in code — don't re-derive)

- Asynq: **one server, one queue** (`default`), `Concurrency: 10`, no `Queues` map.
- **No alerting or queue monitoring existed anywhere in this repo** before Unit 1 (grepped).
- `cmd/worker` already runs a **ticker goroutine** (`runWebhookEventCleanup`) — precedent for
  a periodic in-process job.
- `notification_delivery` — one row per `(notification_id, medium)`. Email-only; in-app state
  stays on the `notification` row.
- Failure reasons written today: `preference_disabled`, `not_cataloged` (→ `muted`);
  `provider_not_configured`, `secret_decrypt_error`, `adapter_init_error`,
  `provider_send_error` (→ `failed`).
- ⚠️ "Webhook" currently means **inbound** (Resend → Bodhveda): `webhook_event`,
  `project_email_settings.webhook_secret`, `/webhooks/email`.

### Broadcast — findings that shape §3

- `broadcast` → `broadcast:prepare_batches` → N × `broadcast:delivery` → one `notification`
  row per recipient. `broadcast_batch.recipients` holds per-batch counts.
- ⚠️ **Broadcast is in-app only.** An `email` block on a broadcast is a 400 (hard rule), so a
  broadcast has **zero `notification_delivery` rows** — every outcome is `notification.status`.
- ⚠️ **Broadcast "muted" recipients are never materialised.**
  `ListEligibleRecipientExtIDsForBroadcast` filters by preference *before* any row is written.
  On a **direct** send `muted` is a visible, clickable row. Same word, two fidelities.
- ⚠️ **No index on `notification.broadcast_id`** — a per-broadcast rollup is a seq scan today.
- The `broadcast` row stores **no audience counts** — not total, not eligible.
- The console has **no broadcast UI at all** (one API route in `lib/api.ts`, no components, no
  route). §3 is greenfield on the frontend.

---

## 2. Unit 1 — infra alerting (the priority)

**Goal:** when Bodhveda's own infra malfunctions, the author finds out from Discord, not from
opening the console a day later.

### 2.1 Where it runs — ⚠️ the one constraint that matters

A monitor that runs as an asynq task on the `default` queue **dies in the incident it exists to
report.** It must not depend on the worker.

The stated scenario is *"the worker is dying or crashing but the main API is still running."*
The API is therefore exactly the right host: it is the surviving process, it already has Redis
and Postgres, and `cmd/worker` already sets the precedent for a ticker goroutine.

**Decision: a ticker goroutine in `cmd/api`.** No new binary, no new compose service. The
earlier draft proposed a separate `cmd/monitor` process; that was sized for a multi-customer
webhook dispatcher and is overkill here. The residual gap — *the API itself is down* — is
covered outside the app by an inbound uptime monitor on `GET /ping` (§2.4).

### 2.2 Checks

`asynq.Inspector` (v0.25.1, confirmed available) exposes both `Servers()` and a `QueueInfo`
carrying `Pending / Active / Retry / Archived / Processed / Failed / Latency`.

| # | Check | Source | Fires when |
|---|---|---|---|
| 1 | **Worker absent** | `Inspector.Servers()` | empty for 2 consecutive ticks |
| 2 | **Queue backing up** | `QueueInfo.Latency` (age of oldest pending task) | > 5 min |
| 3 | **Failure ratio** | `Failed / Processed` (daily counters) | > 25% with `Processed` ≥ 20 |
| 4 | **Tasks being archived** | `QueueInfo.Archived` | increased since last tick |
| 5 | **Stuck sends** | DB: `notification` non-terminal, created > 10 min ago | count > 0 |

**Checks 1–2 catch the scenario you described** (worker crashed/died — nothing consumes, so
`Servers()` empties and pending-task latency climbs).

**Checks 3–4 are the ones that would have caught the actual incident.** During it the worker
was *running*, so `Servers()` was not empty — it was consuming and archiving. But
`Failed/Processed` was 164/168 = **98%**, and 57 tasks were archived. Either check fires
within a tick.

Both failure modes are covered, by different checks. Neither alone is sufficient — worth
keeping in mind before trimming the list.

Check 5 is the backstop: it catches a worker that is alive and reporting success while
notifications nonetheless never reach a terminal state.

### 2.3 Alert hygiene

Firing every tick while a condition holds turns the channel into noise and the alert gets
muted — which is how this feature quietly stops working.

- Each check is a named **condition** with `open` / `resolved` state held in memory.
- Fire once on open, once on resolve. Re-notify at most every 30 min while still open.
- In-memory state is fine for one API instance. A restart re-alerts — acceptable, arguably
  desirable.
- Sink: `BODHVEDA_ALERT_DISCORD_WEBHOOK_URL`. **Absent ⇒ log-only, never fatal**, so local dev
  and anyone running the stack without Discord is unaffected.
- The Discord POST gets a short timeout and its failure is logged, never propagated — a
  monitoring failure must not take down the API.

### 2.4 What covers "the API itself is down"

Everything above dies with the host, so nothing in `internal/monitor` can report it — a system
cannot report its own death.

**Decision (2026-07-29): an inbound uptime monitor, not an outbound heartbeat.** An external
service polls `GET /ping` (already routed, `cmd/api/routes.go`) on a schedule and alerts when
it stops answering. An outbound dead-man's-switch ping was built and then **removed**: the
inbound poll covers strictly more of the realistic failures — it also catches DNS and expired
TLS, which a heartbeat is blind to because the process is alive and pinging happily. The
heartbeat's only unique coverage was "API serving fine but the monitor goroutine wedged",
which is narrow given `Tick` already recovers panics, and it costs an env var plus a
false-alarm mode when VPS egress is blocked.

### 2.5 Layering

This is infra, not a domain, and does not fit handler→service→pg. Proposal: a self-contained
`internal/monitor/` package (checks + Discord sink), started from `cmd/api`. The one DB query
goes through the notification repository (a `CountStuck` method) rather than raw SQL in the
monitor, so the layering rule still holds where it applies.

### 2.6 As built (2026-07-29)

`internal/monitor/` — `condition.go` (Tracker/Alert), `checks.go` (the five checks),
`discord.go` (embed sink), `monitor.go` (the ticker). Wired from `cmd/api/main.go`
via `startMonitor`. `CountStuck` added to `NotificationReader` + `pg/notification.go`.

**No migration.** The stuck-sends query is deliberately unindexed — `notification` is on the
send hot path and a partial index on `status='enqueued'` would add an index write on INSERT
and an index delete on resolve to *every* send, to serve a query that runs once a minute. The
`newerThan` bound keeps the scan proportional to recent volume. Measured locally: seq scan,
0.5 ms. If it ever shows in query stats, add
`CREATE INDEX ... ON notification (created_at) WHERE status = 'enqueued'`.

**Verified against live infra**, not only fakes: run against the dev Redis with no worker
running, `worker_absent` fired and the other four stayed quiet; the `CountStuck` SQL was
EXPLAIN ANALYZE'd against the dev database.

Two traps worth remembering:

- ⚠️ **`NewDiscordSink` returns the `Sink` INTERFACE, not `*DiscordSink`.** Returning a typed
  nil pointer would make `Config.Sink` a non-nil interface holding nil, so the log-only check
  `m.sink == nil` would be false and the first alert would panic on a nil receiver — turning
  "no Discord configured" into a crash on the first real incident. Pinned by
  `TestEmptyDiscordURLYieldsATrulyNilSink`.
- ⚠️ **`tasks_archived` is an EDGE check on the delta, not a level check.** Asynq keeps
  archived tasks until pruned or deleted by hand, so `archived > 0` would latch on forever
  after one bad day. The first observation only primes the baseline, so a restart does not
  alert on a pre-existing backlog.
- ⚠️ **A resolve must not render like a live problem.** `Alert.Summary`/`Fields` describe the
  BROKEN state, so the first version echoed them verbatim and produced a green
  "Resolved: worker absent" card reading *"No Asynq worker is registered — hint: check the
  worker container/process"*. It told the reader to go fix something already fixed. The
  resolve now leads with "Recovered after 1m", marks the old summary as `Was: …`, and drops
  the field block entirely. **Only visible by looking at the rendered card — the state was
  correct and the tests passed.** Pinned by `TestResolvedEmbedDoesNotReadLikeALiveProblem`.

**Smoke test:** `api/internal/monitor/smoke_manual_test.go`, behind the `smoke` build tag so
it stays out of the normal suite (it posts real Discord messages). Fires `worker_absent`
against the live Redis, then resolves it genuinely by registering a real Asynq server:
`go test ./internal/monitor/ -tags smoke -run TestSmoke -v`.

**Deviation from the plan:** none material. Thresholds as designed (5 min queue latency, 25%
failure ratio with a 20-task minimum sample, 10 min stuck age, 6 h lookback, 60 s interval,
30 min re-notify).

---

## 3. Unit 2 — the delivery tree

One UI and one API shape for both send kinds. A direct send is the same tree with a fan-out of
1 — which is the right framing, because it makes the two comparable rather than special-cased.

### 3.1 Shape

**Broadcast:**

```
Broadcast #123 · digest/none/sent · completed · 2026-07-28
└─ Audience  1,000 recipients at send time
   ├─ Excluded by preference   600     ← count only, not expandable (§3.2)
   └─ Eligible                 400
      ├─ in_app                400
      │  ├─ delivered          380
      │  ├─ quota_exceeded      18
      │  ├─ failed               2
      │  └─ enqueued             0     ← non-zero and old = stuck (the §0 signal)
      └─ email                   —     not supported on broadcasts
```

**Direct** — same tree, one branch:

```
Notification #456 · digest/none/sent → user_abc
└─ Recipient  user_abc
   ├─ in_app   delivered
   └─ email    bounced · attempt 1 · resend · re_xyz · bounced_at 10:14
```

### 3.2 Two honesty problems

**(1) "Excluded by preference" is an absence, not a set.** Those recipients were filtered out
before any row existed, so the node shows a *count* and nothing else — there is no list to open.
On a direct send the equivalent (`muted`) *is* a real row you can click. Materialising them
would mean writing N rows per broadcast for people who explicitly opted out: large write
amplification for low value, and the current answer to "who opted out?" lives in the
preferences view anyway (where it is also *current*, not a historical snapshot).
**Recommend: keep it a count, and label it differently from a direct send's `muted` row** so
the UI does not render two different things identically.

**(2) Audience counts must be frozen at send time.** `total − eligible` computed against a live
`COUNT(*) FROM recipient` is wrong the moment anyone signs up or leaves, and can go **negative**.
So: **persist `total_recipients` and `eligible_recipients` on the `broadcast` row** at
`prepare_batches` time. Migration + one write in the processor. Without it the tree shows a
plausible-looking wrong number, which is worse than no number.

### 3.3 Queries and indexing

The rollup is `SELECT status, count(*) FROM notification WHERE broadcast_id = $1 GROUP BY
status`, and **there is no index on `broadcast_id`** — a seq scan today. Migration required.

Coordinate with `overview.md` §Open/next, which already plans a partial unique index on
`(broadcast_id, recipient_external_id)` to fix the `BroadcastDeliveryProcessor` idempotency
bug. That index serves `WHERE broadcast_id = $1` as a prefix, so **one index may satisfy both**
— though the rollup also reads `status`, so `(broadcast_id) INCLUDE (status)` may plan better.
Measure; don't add two indexes to the hot-path `notification` table without checking.

Build the API per-medium from day one (one branch today) so broadcast email — and SMS/push —
slot in without a contract change.

### 3.4 Surfaces

- `GET /console/projects/{pid}/broadcasts/{bid}` — broadcast + frozen audience counts +
  per-medium status rollup + batch progress.
- Direct tree needs no new data — assemble from the notification row plus the existing
  deliveries endpoint.
- Console: new broadcast detail route (greenfield), tree component shared with the direct-send
  detail dialog.
- **Console-only.** No Developer API surface for the tree — there is no external caller, and it
  would lock an opinionated shape into a public contract for no one's benefit.

### 3.5 Shared foundation

`Outcome()` and `Terminal()` as methods on `enum.DeliveryStatus`, so the tree and any later
surface classify identically:

| `outcome` | statuses |
|---|---|
| `pending` | `pending`, `sending` |
| `succeeded` | `sent`, `delivered` |
| `suppressed` | `muted`, `no_contact` — **working as intended, not an error** |
| `failed` | `failed`, `bounced`, `complained`, `quota_exceeded`, `rejected` |

Keeping `suppressed` out of `failed` is the point: an opted-out recipient is not a failure, and
counting it as one makes every failure number permanently wrong.

⚠️ `sent` is terminal only when inbound provider webhooks are *not* configured; with Resend
webhooks wired it still advances to `delivered`/`bounced`.

---

## 4. What this does NOT cover

- ✅ **Worker/queue failure, and Bodhveda infra malfunction** — Unit 1, checks 1–5.
- ✅ **Total outage / the api being down** — an inbound uptime monitor on `GET /ping`, set up
  outside the app (§2.4). Nothing inside Bodhveda can cover this.
- ✅ **Seeing what happened to a send or a broadcast** — Unit 2.
- ❌ **Per-notification failure notifications to an external consumer** — no webhooks (§5).
  Not needed: the only consumer is the author, who gets Discord alerts instead.
- ❌ **Attributing a delivery failure to a cause** (bad Resend key vs Resend outage) — see §5;
  `provider_send_error` is still one opaque bucket.

---

## 5. Shelved, with reasons

**Outbound webhooks — dropped for now.** Nothing to build against: no external customers. And
per §0 they would not have caught the motivating incident anyway, since no delivery row is
written when the worker fails. Revisit when a real customer wants programmatic per-delivery
reactions at volume. The full design (event set, HMAC canonical string, retry/backoff,
auto-disable, SSRF hardening, isolation) is in this file's git history and can be lifted intact.

**Public delivery-status API (`GET /deliveries`, `GET /notifications/{id}/deliveries`) —
deferred.** Same reason: the only user reads the console, which already shows per-notification
delivery detail. The genuinely load-bearing pieces of that design are *kept* and absorbed:
`Outcome()`/`Terminal()` (§3.5) and the stuck-send query (§2.2, check 5) — now an internal
monitor query rather than a public endpoint.

**Provider error classification — deferred, but note the blocker.** `provider_send_error`
collapses a 401 from a revoked API key, a 429 rate limit, and a 503 outage into one string with
the HTTP status discarded. Any future "why did this fail?" feature needs the adapter to return
a typed error first (`email.SendError{StatusCode, ProviderCode, Retryable}`). Recording it here
so it is not rediscovered later.

---

## 6. Build order

1. **Unit 1 — alerting.** `internal/monitor/` (checks + condition state + Discord sink),
   `CountStuck` on the notification repo, ticker wired into `cmd/api`, env vars.
   Tests: each check's fire/clear thresholds, condition state machine (fire-once, re-notify
   window, resolve), Discord payload, absent-webhook-URL is a no-op.
2. **Unit 2 — delivery tree.** Migration (broadcast audience counts + `broadcast_id` index),
   rollup query, broadcast detail endpoint, console tree UI for both shapes.
   Tests: rollup correctness, frozen audience counts, direct-vs-broadcast assembly.
