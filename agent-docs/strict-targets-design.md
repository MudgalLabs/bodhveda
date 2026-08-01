# Strict targets — making the catalog a gateway

> **SHIPPED 2026-08-01.** This document is kept as the reasoning record. Where the
> proposal and the implementation differ, the implementation is right and the
> differences are listed here.
>
> **What changed from the proposal:**
>
> 1. **No `strict_targets` flag (§3.2 is obsolete).** The flag existed to protect
>    live integrations during migration. Measuring production settled it instead:
>    Resurface was already fully cataloged, Grahak prod had sent zero
>    notifications, and the only real breakage was Arthveda's welcome — which
>    `mandatory` fixes directly. A flag nobody would ever set to `false` is a
>    branch to maintain, not a safety net. Correct routing is the default because
>    it is the only mode.
> 2. **Open question §6.1 answered YES, on evidence.** A `topic='any'` catalog row
>    DOES satisfy the gate. Not a lean — 8 production sends match only via the
>    wildcard, all of them Grahak conversation replies. Exact-match gating would
>    have 400'd the product's core function.
> 3. **`mandatory` shipped in the SAME unit (§3.4 upheld).** It was load-bearing,
>    not a follow-up: Arthveda's `marketing/none/welcome` is 796 production sends
>    and is deliberately uncataloged. Without `mandatory` the gate would have
>    broken it on day one.
> 4. **A fourth thing had to ship with it, unforeseen by this doc.** The three
>    broadcast audience queries matched the topic EXACTLY while the direct cascade
>    honoured wildcards, so the same target resolved differently for a direct send
>    and a broadcast. Latent before; the gate made it reachable. They now share one
>    SQL fragment.
> 5. **§6.4 (`rejected` outcome bucket) is moot.** The gate rejects before any row
>    is written, so nothing lands in `suppressed`.
>
> **Still open:** an untargeted send (no `target` at all) is not gated, because it
> names no target to check. Such notifications are also unmutable — the §2.1
> problem in its purest form — but requiring a target on every send is a separate,
> larger breaking change.


**Status:** proposed, not built. Written 2026-07-30.
**Origin:** came out of the delivery-feedback work (`delivery-feedback-design.md`) — building
the delivery tree forced the question "does the catalog actually gate anything?" and the answer
turned out to be *three different things depending on the path*.

**Not urgent, but not cosmetic either:** it changes shipped behaviour on the hot send path, so
it needs its own design pass, its own tests, and a migration story.

---

## 1. What happens today (verified in code, 2026-07-30)

The behaviour is asymmetric, and the code is explicit that this is deliberate:

> `pg.ShouldDirectNotificationBeDelivered`: *"in_app defaults to DELIVER (true), preserving
> legacy behavior — direct in-app notifications deliver unless explicitly muted, no catalog
> required."*

> `pg.ResolveRecipientPreferences`: *"The catalog is a DEFAULT, not a gate."*

| Path | Medium | No catalog row for the target |
|---|---|---|
| Direct | `in_app` | **Delivered.** No gate whatsoever. |
| Direct | `email` | **Muted**, `failure_reason: not_cataloged` |
| Broadcast | `in_app` | **400 at send time** (`NotificationService.Send`) |

The mechanism is one line in the send-path resolver:

```go
defaultDeliver := medium == enum.MediumInApp
```

The cascade is recipient-exact → recipient-fallback (`topic='any'`) → project-exact →
project-fallback → **medium-dependent default**. Only that last step differs by medium.

⚠️ **A fourth inconsistency:** the broadcast guard (`DoesProjectPreferenceExist`) checks row
**existence only, not `enabled`**. So a disabled catalog row passes the gate and the broadcast
is accepted, then reaches nobody. Observed on demo broadcast #31: accepted, `eligible = 0`,
`excluded_not_cataloged = 25`. The gate and the eligibility rule disagree about what the
catalog means.

---

## 2. Why this is worth changing

### 2.1 The strongest argument: uncataloged in-app notifications are un-mutable

`ResolveRecipientPreferences` builds its target universe from **the catalog UNION the
recipient's own existing preference rows**. So a target that

- has no catalog row, and
- has no pre-existing recipient row,

appears **nowhere** in a recipient's preference list. It cannot be discovered or toggled from a
settings UI built on that API — which is every settings UI, including Resurface's.

The Developer API *would* accept an explicit opt-out write for an arbitrary target
(`PATCH /recipients/{id}/preferences` creates a recipient-level row for any target string), so
this is not a hard impossibility. But nothing surfaces the target, so in practice a recipient
has no way to know it exists, let alone turn it off.

**So today's default doesn't merely let typos through — it can create notifications nobody can
switch off.** That is a user-facing problem, not internal hygiene.

### 2.2 A typo is currently indistinguishable from a real send

`notifications.send({target: {channel: "prodcut", ...}})` delivers. It lands in the inbox, it
meters a usage unit, and it is un-mutable per §2.1. Nothing anywhere says the project has no
such target.

### 2.3 The catalog currently isn't a contract

It is advertised as the thing that "defines what a whole project may send"
(`cmd/api/routes.go` says exactly that about the catalog routes), and for `email` and broadcast
it behaves that way. For direct in-app it does not. Documenting the current behaviour honestly
requires the table in §1, which is a sign the model is wrong rather than subtle.

---

## 3. Proposal

### 3.1 ⚠️ The load-bearing decision: 400, not `muted`

**An uncataloged target must be a loud 400 at send time. It must NOT become a `muted`
delivery.**

- `muted` means **the recipient said no**. A legitimate, expected, healthy outcome.
- Uncataloged means **the caller sent something this project does not define**. Almost always a
  typo or a missing setup step.

Conflating them recreates precisely the failure class the delivery-feedback work exists to
close: the caller receives `200`, believes it sent, and nothing reaches a human. It is the
2026-07-27 incident in miniature, per-notification instead of per-queue.

It also corrupts every downstream rollup. `enum.Outcome` maps `muted` → `suppressed`
specifically so that "the recipient opted out" never inflates failure counts. A typo'd target
landing in `suppressed` makes that bucket mean two unrelated things, and the delivery tree then
reports a caller bug as the system working as designed.

**This also means the existing `email` behaviour is wrong and should change with it.** Today an
uncataloged email send produces `muted` + `failure_reason: not_cataloged` — the right
information in the wrong shape, since every UI and aggregate reads `muted` as an opt-out. The
`not_cataloged` reason exists only because the outcome it is attached to is misleading.

### 3.2 Per-project `strict_targets` flag

A hard gate is a breaking change (§4), so it ships behind a per-project flag:

- `project.strict_targets BOOLEAN NOT NULL DEFAULT false`
- **New projects: true.** Set at creation, so the strict model is the default going forward.
- **Existing projects: false.** Flipped deliberately, per project, once its catalog is complete.
- Console toggle, with a pre-flight check that lists targets already sent but not cataloged, so
  turning it on is never a blind switch.

When `strict_targets` is on, `NotificationService.Send` rejects any target with no matching
catalog row **for the medium the send is requesting** — 400, before any row is written, for
both direct and broadcast.

Considered and rejected:

- **Global hard switch.** Cheaper, but breaks live integrations with no migration path (§4).
- **Backfill the catalog from history, then enable globally with no flag.** Tempting — the
  target set is tiny — but it silently *invents* catalog entries for whatever was sent,
  including the typos this feature exists to catch. A typo sent once becomes a permanent
  blessed target.
- **Strict per API key rather than per project.** The catalog is project-scoped; a per-key gate
  would let two keys disagree about what the project can send.

### 3.3 Fix the existence-vs-enabled mismatch

`DoesProjectPreferenceExist` should stay **existence-based**, and that should be explicit
rather than incidental — but the two cases need different errors:

- **No catalog row** → 400 `target_not_cataloged`. The project has no such target.
- **Catalog row exists but `enabled = false`** → accepted, and the broadcast completes with
  `eligible = 0` / `excluded_not_cataloged = N`.

A disabled catalog row means "defined, currently switched off project-wide" — that is a real,
intentional state and a send to it is not a caller error. The delivery tree already reports it
correctly (§3.6 of the delivery-feedback note), so this needs naming and a test, not new
behaviour.

### 3.4 Transactional / mandatory targets — must be designed WITH this, not after

If the catalog is both the gate *and* the preference surface, then everything in it becomes
opt-out-able — including password resets, security alerts, and billing failures. Products
cannot allow that, and the usual workaround (send it outside Bodhveda) defeats the platform.

So `preference` needs a project-level `mandatory BOOLEAN` (name TBD — `required`,
`transactional`): the target is cataloged, therefore sendable, but the recipient-level toggle
is refused (the cascade skips the recipient row) and the console/SDK render it as
non-negotiable with an explanation.

⚠️ This is deliberately called out as in-scope for the *design*, because retrofitting it means
revisiting the cascade — the one piece of SQL that is already duplicated across
`ShouldDirectNotificationBeDelivered` and `ResolveRecipientPreferences` and kept in step by
`TestResolveRecipientPreferencesAgreesWithGating`. Changing it twice is the expensive path.

---

## 4. Migration — measured, not guessed

Query run against the dev DB on 2026-07-30 (direct sends, grouped by target, with whether an
`in_app` catalog row exists):

```
 project_id |      target      | sent | in_app_cataloged
------------+------------------+------+------------------
          3 | digest/none/sent |    4 | f
```

**Resurface's digest target has no `in_app` catalog row.** Under a global hard gate its in-app
digest would start 400ing immediately. Small blast radius in dev, but that is the live
integration, and it is exactly the case the flag exists to protect.

Rollout:

1. Ship the flag, default `false`, new projects `true`. Nothing changes for anyone.
2. Add the console pre-flight: "these N targets have been sent but are not cataloged."
3. Fill in catalogs for Resurface and Grahak by hand — reviewed, not auto-backfilled (§3.2).
4. Flip those projects.
5. Only once no project runs with `strict_targets = false` should the flag be considered for
   removal.

---

## 5. What this is NOT

- Not a replacement for preferences. Strict targets answer *"is this target defined?"*;
  preferences answer *"does this recipient want it?"*. Two questions, currently one table — see
  §6.
- Not a rate limit or an abuse control.
- Not retroactive. Notifications already delivered for uncataloged targets stay as they are;
  rewriting history would erase the evidence that motivated this.

---

## 6. Open questions

1. **Does the `topic='any'` fallback satisfy the gate?** The cascade already treats a
   `topic='any'` catalog row as matching a concrete topic. If it gates too, then one wildcard
   row re-opens the gateway for a whole channel — which may be exactly what a caller with
   per-post topics (`post_123`) needs, or a loophole that makes the feature pointless.
   **Leaning: yes, it satisfies the gate** — otherwise dynamic-topic products cannot use
   strict mode at all — but it needs a deliberate decision and a test.
2. **One flag, or per-medium?** A project might want strict email and lenient in-app while
   migrating. Leaning one flag; per-medium is a second knob that can disagree with itself.
3. **`mandatory` naming and scope** (§3.4) — and whether it lands in the same unit or
   immediately after.
4. **Should `enum.Outcome` gain a `rejected` bucket** for caller-error outcomes, distinct from
   both `failed` (we tried, it did not arrive) and `suppressed` (deliberately not sent)? A 400
   means no row is written at all, so possibly moot — but if any uncataloged path ever records
   a row instead of rejecting, it must not land in `suppressed` (§3.1).
