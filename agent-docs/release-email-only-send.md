# Release runbook — email-only direct send (Phase 10)

Human publish/deploy steps for this release. **Everything below is irreversible or
credential-gated** — Claude prepared and verified the artifacts but does NOT run these. Run them in
order, from the repo root unless noted.

Same shape as `release-preference-read-fix.md` (Phase 9.3.1). Don't edit that file for this release.

## What ships

`payload` becomes **optional on a direct send**. Omit it and Bodhveda delivers the email without
creating an in-app notification. See the "Phase 10" section of `overview.md` for the design and why
`payload`-optional was chosen over a `mediums: []` allow-list or an `in_app: false` flag.

| Package | npm / module | Old | New | Status |
|---|---|---|---|---|
| `sdk/js/core` | `@bodhveda/js` | 0.5.0 | **0.6.0** | ✅ published |
| `sdk/js/react` | `@bodhveda/react` | 0.5.0 | **0.6.0** (dep `@bodhveda/js@^0.6.0`) | ✅ published |
| `sdk/go` | `github.com/MudgalLabs/bodhveda/sdk/go` | tag `sdk/go/v0.5.0` | **tag `sdk/go/v0.6.1`** | ⚠️ see below |

### ⚠️ `sdk/go/v0.6.0` was mis-tagged — skip it, do not delete it

`make sdk_tag_go` was run before the Go SDK changes were committed, so
`sdk/go/v0.6.0` points at `b1e2e78` (the react-lockfile commit) and ships **byte-identical code to
`v0.5.0`**. `proxy.golang.org` fetched and cached it within minutes.

**It was deliberately left in place.** Deleting and re-pushing the tag is the instinctive fix and
the one genuinely harmful option: the checksum database has recorded `v0.6.0`, so re-pointing it
gives every consumer that already resolved it a `checksum mismatch` — an error indistinguishable
from a supply-chain compromise. A wrong tag is recoverable; a mismatched one is not.

Impact is mild: `v0.6.0` breaks nothing, and a Go consumer on it can already make email-only sends
(a nil `Payload` marshals to `"payload": null`, which the API treats as absent). The real changes
ship as **`v0.6.1`**.

**Lesson for next time:** `make sdk_tag_go` tags `HEAD`. Commit and push the SDK changes *first*,
then tag. The Makefile target does not check that the working tree is clean — worth adding.

**Backwards compatible.** `payload` only became *optional*, so every existing call behaves
identically. This is a minor bump purely because it adds capability.

## ⚠️ Read this before sequencing anything

**The SDK publish does NOT gate any consumer.** The wire protocol change is what matters, and it is
purely additive — the API now accepts a send with no `payload`. Verified against the real API:
`@bodhveda/js@0.5.0` can already make an email-only send today by passing `payload: undefined`
(the key is dropped by `JSON.stringify`, and the API treats an explicit `null`/absent payload
identically). It typechecks on 0.5.0 because `unknown` admits `undefined`.

```ts
// works on @bodhveda/js@0.5.0, once the API is deployed
await bodhveda.notifications.send({
    recipient_id: "vikram",
    target: { channel: "conversation", topic: "thread_7", event: "reply" },
    payload: undefined,            // <- the escape hatch
    email: { subject: "3 new messages", html: "<p>…</p>" },
});
```

So the **only** thing gating a consumer is **step 1 (deploy the API)**. The SDK bump is an
ergonomics and documentation upgrade: on 0.6.0 you simply omit the key, which is what the feature
actually means.

Consequence for ordering: do step 1, unblock consumers immediately, then do steps 2–4 at leisure.

---

## 1. Deploy the API (the only step that gates consumers)

Merging to `main` is the whole deploy — `.github/workflows/deploy.yml` builds and pushes the image,
then on the VPS runs `git pull`, pulls images, **runs migrations**, and restarts `api` then
`worker`. Migrations run *before* the restart, which is the order this release needs.

```bash
git checkout main && git merge dev     # or merge the PR
git push origin main
```

Then watch the run. The migration is
`migrations/20260727120000_make_notification_payload_nullable.sql` —
`ALTER TABLE notification ALTER COLUMN payload DROP NOT NULL`. It takes a brief ACCESS EXCLUSIVE
lock but does **no table rewrite**, so it is effectively instant even on a large `notification`
table.

**Rollback safety.** The migration is backwards compatible with the *old* code: old code always
supplies a payload, so a nullable column changes nothing for it. If you need to roll the API back,
roll it back and leave the migration applied — do NOT run the Down, which backfills `'{}'` over
every email-only row's NULL payload (it has to; NOT NULL cannot be restored otherwise).

**Verify after deploy** (any full-scope key on a real project):

```bash
curl -s -X POST https://api.bodhveda.com/notifications/send \
  -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
  -d '{"recipient_id":"you","target":{"channel":"c","topic":"t","event":"e"},
       "email":{"subject":"deploy check","html":"<p>hi</p>"}}'
# -> 200, notification.status == "not_requested", payload == null

curl -s -X POST https://api.bodhveda.com/notifications/send \
  -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
  -d '{"recipient_id":"you","target":{"channel":"c","topic":"t","event":"e"}}'
# -> 400 "Nothing to send"
```

## 2. Publish the JS SDKs

Order matters: core must be on npm before react, because `sdk_publish_js_react` does a **real
registry install** to resolve `@bodhveda/js@^0.6.0` and refresh `package-lock.json`.

```bash
npm login                    # if not already
make sdk_publish_js_core     # 1. @bodhveda/js@0.6.0
make sdk_publish_js_react    # 2. @bodhveda/react@0.6.0 (refreshes its lockfile)
```

⚠️ **`sdk/js/react/package-lock.json` is intentionally stale right now** — it still pins
`@bodhveda/js@^0.5.0` while `package.json` says `^0.6.0`, because 0.6.0 does not exist on npm yet.
`npm ci` in `sdk/js/react` will fail until step 2 runs. This is the same two-step the repo has done
before (`e79b41c chore(sdk): refresh react lockfile for @bodhveda/js@0.5.0`). **Commit the
refreshed lockfile after step 2:**

```bash
git add sdk/js/react/package-lock.json
git commit -m "chore(sdk): refresh react lockfile for @bodhveda/js@0.6.0"
git push origin main
```

## 3. Tag the Go module

⚠️ **Go proxy tags are IMMUTABLE, and `make sdk_tag_go` tags whatever `HEAD` is.** Commit and push
the SDK changes **first**, confirm `git status` is clean, and only then tag. Getting this wrong is
what produced the stray `v0.6.0` above.

```bash
git status --short           # MUST be clean for sdk/go/
git log --oneline -1         # HEAD must contain the Go SDK changes
make sdk_tag_go              # reads v0.6.1 from sdk/go/CHANGELOG.md
```

Verify the tag contains the change before anyone consumes it:

```bash
git show sdk/go/v0.6.1:sdk/go/types.go | grep 'json:"payload'
# must show: Payload json.RawMessage `json:"payload,omitempty"`
```

Then confirm it resolves through the public proxy (the repo is public, so no GOPRIVATE):

```bash
GOPROXY=https://proxy.golang.org go list -m github.com/MudgalLabs/bodhveda/sdk/go@v0.6.1
```

## 4. Console + docs

**Console** (Cloudflare, deployed separately from `deploy.yml`, which is VPS/API only). It builds
clean locally (`cd console && npm run build`). Deploy however you normally do — the console changes
here are display-only (the new `not_requested` status, the analytics bucket, an optional payload in
the send modal) and nothing breaks if it lags the API.

**Docs** (Mintlify) publish from `docs/` on merge. `openapi.json`, the Mediums and Notifications
concept pages, and the send-notification reference are all updated.

---

## Grahak integration

Grahak uses `@bodhveda/js` + `@bodhveda/react` (`apps/console/package.json`, both `^0.5.0`) against
`https://api.bodhveda.com` — including from **local dev**, which is why step 1 is what unblocks it.

**After step 1, with no SDK change**, Grahak can debounce email independently of in-app:

- instant, per reply → send with `payload` only (unchanged)
- debounced, per 5-min window per conversation → send with `email` only and `payload: undefined`

The debounced send no longer drops a duplicate row into the feed, which is the workaround Grahak
shipped around (it currently debounces both mediums together and gives up instant in-app).

**After step 2**, bump Grahak to `^0.6.0` and drop the `payload: undefined` line entirely.

Prerequisites for the email actually to send — unchanged, and both already report visibly rather
than silently:

1. the `(target, email)` pair is in the project's catalog — Grahak's
   `setup-bodhveda-catalog.ts` already emits per-medium entries, so re-run it for any new target;
   otherwise the delivery row reads `muted` / `not_cataloged`
2. the recipient has a **primary email contact**; otherwise `no_contact`

Read either back with `bodhveda.notifications.get(id)` → `.email.status`.

## Still open (not in this release)

- **Broadcast email fan-out** — broadcasts remain in-app only. See "Open / next" in `overview.md`.
- ⚠️ **`BroadcastDeliveryProcessor` is not idempotent** — a live bug that today produces duplicate
  in-app rows on a retried batch, and would produce duplicate *emails* once broadcast email ships.
  Must be fixed before that work.
- **Quota does not gate email on a mixed send** — email-only sends are gated (else they would
  bypass plan limits entirely), but a send carrying both blocks still emails when over quota,
  because step 3 has always been independent of step 2.
