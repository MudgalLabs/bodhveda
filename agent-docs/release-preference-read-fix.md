# Release runbook — SDK rename + preference read fix (Phase 9.3.1)

Human publish steps for this release. **Everything below is irreversible or credential-gated** —
Claude prepared the artifacts but does NOT run these. Run them in order, from the repo root unless
noted. Most steps are wrapped in `make` targets (see the "SDK release" block in the `Makefile`); the
raw commands are shown too.

This release does two things at once:

1. **Renames the JS core package** `bodhveda` → `@bodhveda/js`, so both JS packages live under the
   `@bodhveda` scope (matching `@bodhveda/react`). The old `bodhveda` package is deprecated.
2. **Ships the preference-read fix** (`list()` / `check()` now resolve honestly) that had been
   prepared as `bodhveda@0.2.0` but was never published.

This follows the same shape as `release-email-medium.md` (the Phase 7 → 7.5 release), which remains
the record of what shipped then. Don't edit that file for this release.

## Versions — re-baselined so all three SDKs share one number

`@bodhveda/js` is a brand-new name on npm with no version history, so it can start at any number for
free. We use that to re-baseline: from now on **one version number = one feature set across all
three SDKs**, and every release tags all three together.

| Package | npm / module | Old | New |
|---|---|---|---|
| `sdk/js/core` | `bodhveda` → **`@bodhveda/js`** (npm) | 0.1.0 (as `bodhveda`) | **0.3.0** |
| `sdk/js/react` | `@bodhveda/react` (npm) | 0.1.0 | **0.3.0** (dep `@bodhveda/js@^0.3.0`) |
| `sdk/go` | `github.com/MudgalLabs/bodhveda/sdk/go` | tag `sdk/go/v0.2.0` | **tag `sdk/go/v0.3.0`** |

Why 0.3.0 and not 0.2.0: Go was already at v0.2.0 for the email medium (which JS shipped as 0.1.0),
so the SDKs were a minor apart for the *same* features. v0.3.0 is Go's next number for the
preference fix; matching JS to it aligns everything. The never-published `bodhveda@0.2.0` is simply
skipped. This is a **minor** bump (`0.x` → new minor), and under 0.x semver a minor may break —
this one does, in three ways:

1. **Package rename (JS core).** The import path changes: `from "bodhveda"` → `from "@bodhveda/js"`.
   The API surface is otherwise identical.
2. **Behavioral (all SDKs).** `preferences.list()` / `.check()` return a *resolved* state now.
   Values change and the row set grows. A customer's settings screen will render differently —
   correctly, but differently. This is the headline; the CHANGELOGs lead with it.
3. **Source-level (Go only).** `PreferenceState.Inherit` → `PreferenceState.Inherited`. The old
   field was tagged `json:"inherit"` while the API sends `inherited`, so it never deserialized —
   anyone reading `.Inherit` was reading a value that was always `false`. Fixing the tag without
   renaming would silently start returning real data to code written around the broken one, so the
   rename is deliberate: it forces a compile error at the call site.

> ⚠️ **npm publishes are effectively permanent.** npm only allows un-publish within 72h and under
> strict conditions; treat a published version as forever. Double-check the version and
> `npm pack --dry-run` contents before publishing. You need `npm login` first.

---

## 0. Pre-flight (safe to run)

```bash
git switch main && git pull
git status

make sdk_build     # go build+vet; npm ci+build core; build react against LOCAL core
```

`sdk_build` verifies all three even before anything is published — the react step links the local
`../core` with `--no-save`, so it never touches `package.json` or the lockfile.

`npm login` if not already authenticated:

```bash
npm whoami || npm login
```

## 1. Publish `@bodhveda/js` (JS core) — MUST be first

React depends on `@bodhveda/js@^0.3.0`, so core has to exist on npm before react installs cleanly.
This is the **first publish of the new name** — confirm the tarball's `name` is `@bodhveda/js` and
`version` is `0.3.0` in the dry-run before publishing.

```bash
make sdk_publish_js_core
# = cd sdk/js/core && npm pack --dry-run && npm publish
```

Verify: `npm view @bodhveda/js version` → `0.3.0`.

## 2. Publish `@bodhveda/react` (JS react)

The committed `sdk/js/react/package-lock.json` still references the old `bodhveda@0.1.0` — it can
only be regenerated once `@bodhveda/js@0.3.0` is live (step 1). The publish target runs
`npm install` first, which resolves `@bodhveda/js@^0.3.0` from npm and refreshes the lockfile.

```bash
make sdk_publish_js_react
# = cd sdk/js/react && npm install && npm run build && npm pack --dry-run && npm publish
```

**Commit the refreshed `sdk/js/react/package-lock.json`** — it now points at `@bodhveda/js@0.3.0`.

Verify: `npm view @bodhveda/react version` → `0.3.0` and its `dependencies` show `@bodhveda/js`.

## 3. Deprecate the old `bodhveda` package

Freeze the old name and point stragglers at the new one. This warns on install for all versions
(only `0.1.0` exists) and does not remove anything.

```bash
npm deprecate bodhveda "Renamed to @bodhveda/js. Install @bodhveda/js instead (same API)."
```

Verify: `npm view bodhveda deprecated` shows the message. Do not publish to `bodhveda` again.

## 4. Tag all three SDKs (versions ↔ tags)

The JS packages have **never been git-tagged** — this release starts that. Tag all three at the same
version so a tag always corresponds to a published SDK.

```bash
make sdk_tag_go                      # tags + pushes sdk/go/v0.3.0
git tag sdk/js/core/v0.3.0  && git push origin sdk/js/core/v0.3.0
git tag sdk/js/react/v0.3.0 && git push origin sdk/js/react/v0.3.0
```

Verify the Go tag reached origin:

```bash
git ls-remote --tags origin | grep sdk/go/v0.3.0
```

> ⚠️ **Do not verify via the public Go proxy** — the `MudgalLabs/bodhveda` repo is **private**, so
> `proxy.golang.org` returns 404 for every version of this module (the old `sdk/go/v0.2.0` 404s too).
> That is expected, not a release failure. Consumers resolve the private module with authenticated
> git: `GOPRIVATE=github.com/MudgalLabs/* go get github.com/MudgalLabs/bodhveda/sdk/go@v0.3.0`.

## 5. Deploy the docs (Mintlify)

Mintlify auto-builds off the default branch via its GitHub App. Merging to `main` publishes
docs.bodhveda.com. Confirm the deploy in the Mintlify dashboard; if the App is not configured for
auto-deploy, `cd docs && mint deploy` (preview first with `mint dev`).

Verify: the **Preferences** concept page shows the new "How a preference resolves" section, the
list/check endpoints document `cataloged`, and the SDKs page/READMEs reference `@bodhveda/js`.

## 6. Deploy the API

The read fix is server-side — **the SDK bumps are only types/docs. Customers get the new behavior
the moment the API ships, whether or not they upgrade.** That is the whole reason the CHANGELOGs
are loud. Deploy via the usual `.github/workflows/deploy.yml` path (push to `main` builds + pushes
the image and SSH-deploys the VPS).

## 7. Resurface — measured impact: almost certainly none

Checked against the `../resurface` checkout in 9.3.1, because an earlier draft of this file claimed
(wrongly, inheriting it from the 9.3 deviations) that Resurface's settings UI reads
`usePreferences()`. **It does not — `usePreferences` appears nowhere in Resurface.**

What it actually does: `/settings` (a server component) calls `getDigestPreferences()` → two
`preferences.**check**()` calls (in_app + email) on `digest/none/sent` → renders the toggles
(`web/lib/bodhveda.ts`, `web/app/(app)/settings/page.tsx`). So the path to their UI is the **check**
endpoint, not the list read.

Old and new `check()` **agree** for every case Resurface can hit:

- Their target is `topic: none`, so the `topic='any'` fix is a no-op for them by definition.
- The medium-dependent default only fires when nothing matches. `web/scripts/bodhveda-backfill.ts`
  wrote explicit `in_app` + `email` rows for **every** user ⇒ `check` resolves `recipient_exact` ⇒
  same stored value, before and after.
- `bodhveda-targets.ts` requires `digestSent` to be cataloged for in_app + email ⇒ even an
  un-backfilled user resolves `project_exact` ⇒ identical.

The only divergence: a user with **no explicit email row AND email not cataloged** — old `true`, new
`false`. There the old value was the lie (the page rendered "Email digest: ON" while the send path,
unchanged since Phase 2, refused to send). So the only visible change is a toggle that stops
contradicting the inbox.

Worth passing on (Resurface's own code, not ours): `settings/page.tsx` falls back to
`{ inApp: true, email: true }` when Bodhveda is unreachable, commented as "matches the catalog
default". That hardcodes the same wrong assumption this release removes — email's default is
`false`. It only fires during an outage, but it is the same bug's last hiding place.

Separately, Resurface still installs the old `bodhveda` package. It keeps working (deprecated, not
removed), but the clean-up is `npm uninstall bodhveda && npm install @bodhveda/js` and swapping the
import path in `web/lib/bodhveda.ts`. Not urgent; worth doing next time that file is touched.
