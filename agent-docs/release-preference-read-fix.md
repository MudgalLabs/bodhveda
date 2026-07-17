# Release runbook — preference read fix (Phase 9.3.1)

Human publish steps for the preference-read fix. **Everything below is irreversible or
credential-gated** — Claude prepared the artifacts but does NOT run these. Run them in order,
from the repo root unless noted.

This follows the same shape as `release-email-medium.md` (the Phase 7 → 7.5 release), which
remains the record of what shipped then. Don't edit that file for this release.

Chosen versions:

| Package | npm / module | Old | New |
|---|---|---|---|
| `sdk/js/core` | `bodhveda` (npm) | 0.1.0 | **0.2.0** |
| `sdk/js/react` | `@bodhveda/react` (npm) | 0.1.0 | **0.2.0** (dep `bodhveda@^0.2.0`) |
| `sdk/go` | `github.com/MudgalLabs/bodhveda/sdk/go` | tag `sdk/go/v0.2.0` | **tag `sdk/go/v0.3.0`** |

A **minor** bump across the board (`0.x` → new minor). Under 0.x semver a minor may break, and
this one does, in two ways worth being deliberate about:

1. **Behavioral (all SDKs).** `preferences.list()` / `.check()` return a *resolved* state now.
   Values change and the row set grows. A customer's settings screen will render differently —
   correctly, but differently. This is the headline; the CHANGELOGs lead with it.
2. **Source-level (Go only).** `PreferenceState.Inherit` → `PreferenceState.Inherited`. The old
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

( cd sdk/go && go build ./... && go vet ./... )
( cd sdk/js/core && npm ci && npm run build )
```

`npm login` if not already authenticated:

```bash
npm whoami || npm login
```

## 1. Publish `bodhveda` (JS core) — MUST be first

React depends on `bodhveda@^0.2.0`, so core has to exist on npm before react installs cleanly.

```bash
cd sdk/js/core
npm run build
npm pack --dry-run
npm publish
cd ../../..
```

Verify: `npm view bodhveda version` → `0.2.0`.

## 2. Publish `@bodhveda/react` (JS react)

```bash
cd sdk/js/react
npm install                            # resolves bodhveda@^0.2.0 from npm; refreshes the lockfile
npm run build
npm pack --dry-run
npm publish
cd ../../..
```

Commit the updated `sdk/js/react/package-lock.json` if it changes.

Verify: `npm view @bodhveda/react version` → `0.2.0`.

## 3. Tag the Go module

```bash
git tag sdk/go/v0.3.0
git push origin sdk/go/v0.3.0
```

Verify (may lag a minute while the proxy warms):

```bash
GOPROXY=proxy.golang.org go list -m github.com/MudgalLabs/bodhveda/sdk/go@v0.3.0
```

## 4. Deploy the docs (Mintlify)

Mintlify auto-builds off the default branch via its GitHub App. Merging to `main` publishes
docs.bodhveda.com. Confirm the deploy in the Mintlify dashboard; if the App is not configured for
auto-deploy, `cd docs && mint deploy` (preview first with `mint dev`).

Verify: the **Preferences** concept page shows the new "How a preference resolves" section, and
the list/check endpoints document `cataloged`.

## 5. Deploy the API

The read fix is server-side — **the SDK bumps are only types/docs. Customers get the new behavior
the moment the API ships, whether or not they upgrade.** That is the whole reason the CHANGELOGs
are loud. Deploy via the usual `.github/workflows/deploy.yml` path (push to `main` builds + pushes
the image and SSH-deploys the VPS).

## 6. Tell Resurface

Resurface's settings UI reads `usePreferences()`. After this ships its toggles resolve honestly —
which is the point, but it means the rendered state can change without Resurface deploying
anything. Worth an explicit look at that screen post-deploy. See the Phase 9.3.1 deviations in
`agent-docs/overview.md`.
