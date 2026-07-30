# Delivery feedback — design note

**Status:** Unit 1 **BUILT** (2026-07-29, §2.6). Unit 2 **BUILT** — backend §3.6, console UI §3.8,
detail pages §3.9.
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

### 3.6 As built — backend (2026-07-29)

**Two migrations.** `20260729120000` adds four nullable audience columns to `broadcast`;
`20260729130000` adds `ix_notification_broadcast (broadcast_id) INCLUDE (status) WHERE
broadcast_id IS NOT NULL`, CONCURRENTLY. Verified the rollup now plans as an **Index Only
Scan** — the `INCLUDE` makes it covering, so the aggregate never touches the heap. The index
is PARTIAL so direct sends (NULL `broadcast_id`, the bulk of inserts) never touch it, which is
what makes it acceptable on the hot-path `notification` table.

**Four audience columns, not two.** The plan said `total` + `eligible`. Reading
`ListEligibleRecipientExtIDsForBroadcast` showed the eligibility rule folds together two
*completely different* exclusions:

- `excluded_disabled` — the RECIPIENT opted out. Healthy, nothing to fix.
- `excluded_not_cataloged` — the PROJECT never offered the target (no catalog row, or a
  disabled one). A config mistake, and **the usual reason a broadcast silently reaches
  nobody**.

Collapsing them into one "excluded" number would hide the single most useful thing the tree
can tell an operator. They deliberately reuse the vocabulary the direct-send path already
writes to `notification_delivery.failure_reason`, so the console has one vocabulary for both
send kinds.

**`enum.Outcome`** (`model/enum/outcome.go`) with `Outcome()`/`Terminal()` on both
`DeliveryStatus` and `NotificationStatus`. `suppressed` is its own bucket, separate from
`failed`. An unrecognised status classifies as `failed`, never `succeeded` — a value that
reached the DB without reaching the switch is a bug, and a green number would hide it.

**Layering:** `handler/broadcast.go` → `service/broadcast.go` → `pg/{broadcast,notification,
preference}.go`. New route `GET /console/projects/{id}/broadcasts/{broadcast_id}/tree`.
`BroadcastService` now takes the notification repo (`app.go` updated).

Traps worth remembering:

- ⚠️ **`SetAudience` is separate from `Update`.** `Update` is called with a whole entity —
  including from the quota-exceeded path in `PrepareBroadcastBatchesProcessor`, where the
  broadcast was deserialised from the Asynq payload and carries no audience. Putting these
  columns in `Update`'s SET clause would blank them on every such call.
- ⚠️ **Ownership is checked in the service, not the rollup query.** The rollup is keyed by
  `broadcast_id` alone so it stays index-covered; without the service check, another
  project's broadcast id would return zero counts and render as a legitimately empty
  broadcast. `TestBroadcastDeliveryTreeRejectsCrossProjectAccess` pins the 404.
- ⚠️ **`eligible` is stored from the fan-out list, not the aggregate.** They are separate
  queries, so a recipient created between them would make the stored count disagree with the
  notifications actually written. The list is ground truth.
- ⚠️ **Audience is nil, not zero, when unrecorded.** Scanned through `*int`. A broadcast that
  legitimately reached nobody and one whose audience was never measured are different facts.
- The audience write is **best-effort** — it is reporting, and a failure there must never fail
  a fan-out.

### 3.7 Two pre-existing bugs found by dogfooding (fixed 2026-07-30)

Seeding demo broadcasts through the real pipeline surfaced two bugs that predate this work.
Neither was in Unit 2's scope, but Unit 2 could not ship honestly without them.

**(1) A broadcast matching nobody crashed its worker task and was ARCHIVED.**
`prepare_batches` called `CheckAndConsumeUsage` with `Amount: 0`, and `usage_log` has
`CHECK (amount > 0)` — SQLSTATE 23514, Asynq retried, then archived. Silent work loss with a
failing queue: *the exact shape of the 2026-07-27 incident*, reproducible by broadcasting to a
target nobody has enabled. Compounding it, with no recipients there are no batches, and it is
the **last batch** that marks a broadcast `completed` — so even without the crash it sat in
`enqueued` forever. Fixed with an early return that records the audience, completes the
broadcast, and touches neither billing nor the queue.

**(2) EVERY broadcast notification sat permanently in `enqueued`.**
`entity.NewNotification` defaults to `enqueued` — correct for a DIRECT send, where
`notification:delivery` still resolves preferences and billing. A broadcast has no such second
step: `prepare_batches` already did that work, so the row is in the inbox the moment it is
written. But `BroadcastDeliveryProcessor` never updated the status.

Invisible for as long as nothing read it — `recipientFeedVisible` treats `enqueued` as
visible, so inboxes always looked right. It became load-bearing the instant both new units
existed: the tree reported every broadcast as **100% pending**, and `stuck_sends` would have
counted all of them as stalled and alerted every 30 minutes forever — the precise
alert-fatigue failure §2.3 exists to prevent. **90 of 90 broadcast rows were affected.**

Fixed at the write path (`delivered` + `completed_at`), plus backfill migration
`20260730120000`. ⚠️ The backfill is scoped to `broadcast_id IS NOT NULL`: a DIRECT
notification in `enqueued` is a *genuinely* stalled send and must keep showing as one.

Pinned by `TestBroadcastDeliveryWritesDeliveredNotifications` (asserts nothing non-terminal
remains) and `TestPrepareBatchesCompletesEmptyBroadcast` (passes nil asynq client and billing
service, so reaching either panics).

### 3.8 As built — console UI (2026-07-30)

`features/notification/components/delivery_tree.tsx` (the renderer) +
`broadcast_tree_dialog.tsx` (dialog and the row trigger). Opened from a `Details` cell on the
broadcast table, mirroring `DeliveryDetailCell` on the direct table so both kinds open the same
way. The tree is fetched only when the dialog opens — it is an aggregate over every
notification in the broadcast and has no business riding every list refetch.

`DeliveryTreeView` takes a `DeliveryTree` and nothing else, so the direct-send detail dialog
can reuse it unchanged when that is wired up.

Presentation decisions that carry meaning:

- ⚠️ **`suppressed` renders NEUTRAL, never as an error.** It means the recipient opted out —
  the system working. Red would make a healthy project look broken and train the reader to
  ignore real failures. Same for `not_requested`.
- **`excluded_not_cataloged` renders AMBER, with copy naming it a project-config problem**
  ("no enabled in-app catalog entry … add or enable it in Preferences"), while
  `excluded_disabled` renders muted with "working as intended". Same tree level, opposite
  meanings, deliberately different weight.
- **The Excluded node carries a tooltip saying it is a count only** when `expandable` is false.
  Honest about a drill-down that cannot exist rather than silently offering none.
- **"Audience not recorded" is distinct from "reached nobody"** — different facts, different
  copy.
- ⚠️ **Raw statuses nest under an outcome bucket only when they add information.** A bucket
  holding one status whose label matches the bucket's renders as just the bucket — otherwise
  every healthy broadcast read "Delivered 20 → Delivered 20", which looks like a rendering
  fault. `Failed → Quota Exceeded / Failed` is the case that earns children. Caught by looking
  at the rendered page; `statusChildren` holds the rule.

**Verified in the browser** (Playwright + a borrowed session, per the console's OAuth-only
auth): the healthy broadcast, the reaches-nobody broadcast, and a temporarily-injected
mixed-status broadcast to exercise the nesting branch (reverted after). No console-side test
infrastructure exists in this repo, so that was the available verification.

### 3.9 As built — notification detail pages (2026-07-30)

The modals could not be linked, and a notification id is exactly the thing you paste into an
incident note. Since this whole surface exists for debugging, a stable URL is the point.

**⚠️ Two routes, not one.** `/projects/$id/notifications/$notificationId` and
`/projects/$id/broadcasts/$broadcastId`. `notification.id` and `broadcast.id` are independent
SERIAL sequences whose ranges **overlap** (measured: notifications 1–750563, broadcasts 23–31),
so a single `/notifications/:id` path cannot tell them apart. Both routes render the same
`notification_detail.tsx` component, so the two views cannot drift into unrelated screens.
`notifications.tsx` became `notifications/index.tsx` to make room, matching the `recipients/`
precedent.

**New console endpoints:** `GET …/notifications/{id}`, `GET …/notifications/{id}/tree`,
`GET …/broadcasts/{id}`. The notification tree wires up `dto.EmailMediumFromDelivery`, which had
been written and tested but never called. A direct send's tree has **no Audience node** — it
names its recipient, so there is no audience to resolve, and a zeroed one would be a lie.

**The modals stay, reduced to `Peek` + `Open`.** Peek opens the dialog for triage while scanning
a filtered list (navigating away would lose filters and scroll position); Open goes to the page.
Two affordances because they are genuinely different jobs.

Two copy bugs caught by looking at the rendered page, not from tests:

- ⚠️ **"Email 1 notification"** — a notification is the thing being sent, an email is one way it
  went out. On a direct send every branch is 1, so the mismatched noun is the only text on the
  line and all you notice. Units are now per-medium (`MEDIUM_UNIT`).
- ⚠️ **"No provider events recorded yet"** on a muted email. There will never be any — the
  provider was never contacted — and "yet" leaves the reader waiting for something that is not
  coming. Same trap as a resolved alert echoing broken-state copy: technically-empty is not the
  same as pending. `NEVER_SENT_STATUSES` gates the wording.

Verified in the browser: both pages cold-loaded by direct URL (proving the links work, which is
the entire point), the reaches-nobody broadcast, and a direct send whose in-app **delivered**
while its email was **suppressed/muted** — the diverging-outcome case the shared shape exists
for. No browser console errors.

### 3.10 Detail page redesign + full-row navigation (2026-07-30)

**The verdict band is the page's signature.** The first version was a flat run of label/value
pairs with the tree floating underneath, which meant the reader had to compute the outcome
themselves from five nested counts. The tree is *evidence*; the verdict is the *conclusion*, and
it is what an operator opening the page actually wants — so it leads, with an accent rail
coloured by outcome as the only saturated element on the page.

`delivery_verdict.ts` derives it. ⚠️ Broadcast and direct get **different verdict shapes** on
purpose: "20 of 25 recipients reached" is right for a fan-out, but for a fan-out of one it would
always read "1 of 1" and say nothing — so a direct send states per-medium outcomes instead
("In-app delivered · Email suppressed"). `suppressed` maps to a NEUTRAL tone, never error.

Layout is verdict → fan-out → payload, with metadata in a 260px rail. The old page put metadata
first and the answer last.

**Full-row navigation, replacing Peek/Open.** Two verbs per row on a debugging table is a
decision the reader must make before they can look at anything, and the dialog they were choosing
between showed strictly less than the page. Both dialogs are now DELETED
(`delivery_detail_dialog.tsx`, 448 lines, and `broadcast_tree_dialog.tsx`) — unreachable code is
worse than none.

⚠️ netra's `DataTable` renders its own `<tr>` and exposes no `onRowClick`, so the mechanism is an
absolutely-positioned `<Link>` covering the row (`row_nav.tsx`). Rejected alternatives: a `<Link>`
per cell gives eight tab stops per row; an `onClick` per cell has no keyboard path and breaks
cmd-click. One link per row gets whole-row hit area, one tab stop, and normal browser link
behaviour. Interactive cell content must be wrapped in `RowNavShield` (`relative z-10`) or the
overlay swallows its clicks — the recipient link needs it.

`row_nav.tsx` exports CLASS CONSTANTS rather than a `<RowNavLink>` component: TanStack Router
types `to` and `params` together per route, so a wrapper taking `to: string` would throw that
checking away and fail to compile. Each cell builds its own typed `<Link>`.

Two things the build caught that `tsc --noEmit` did not:

- `recipient_notifications_panel.tsx` was a **second consumer** of the deleted dialog. It now
  navigates to the page too.
- The notifications route requires search params, so the breadcrumb needed one. It now returns to
  the tab you came from — a broadcast page goes back to the Broadcast list.

⚠️ `index.css` styles `pre` globally as an **inline-block chip** with its own background and
radius. Wrapping payload in a card produced a box inside a box sized to its content instead of
the column; the `<pre>` is now the container, overridden to block/full-width/scrollable.

Verified in the browser: all three verdict tones, full-row click on both tables landing on the
right URL, and the shielded recipient link still reaching the recipient page.

**Tests.** `TestNotificationDeliveryTree` covers the direct tree: in-app-only (one branch, no
audience node), email-only (`not_requested` its own bucket, neither success nor failure), muted
(suppressed not failed), the in-app-delivered/email-bounced divergence, and cross-project 404.

**Tests.** `TestBroadcastAudienceMatchesEligibleList` is the load-bearing one: the aggregate
and the list duplicate the same eligibility expression in two shapes, so it cross-checks them
against a live Postgres across every recipient/catalog preference combination and asserts the
four buckets partition the total exactly. Plus outcome-folding unit tests, the cross-project
404, and the nil-audience case. Full suite green with `TEST_DB_URL` set.

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
