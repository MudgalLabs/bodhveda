# Release runbook — email medium (Phase 7 → 7.5)

Human publish steps for the email-medium release prepared in Phase 7. **Everything below is
irreversible or credential-gated** — Claude prepared the artifacts but does NOT run these.
Run them in order, from the repo root unless noted.

Chosen versions (decided in Phase 7):

| Package | npm / module | Old | New |
|---|---|---|---|
| `sdk/js/core` | `bodhveda` (npm) | 0.0.6 | **0.1.0** |
| `sdk/js/react` | `@bodhveda/react` (npm) | 0.0.6 | **0.1.0** (dep `bodhveda@^0.1.0`) |
| `sdk/go` | `github.com/MudgalLabs/bodhveda/sdk/go` | tag `sdk/go/v0.1.9` | **tag `sdk/go/v0.2.0`** |

A minor bump across the board (`0.x` → new minor) because this adds the email feature (email
send block, `deliveries[]`, recipient contacts, per-medium preferences) without breaking the
existing API. `package.json` versions are already bumped in the tree; the Go tag is **not**
created yet (that's step 3 here).

> ⚠️ **npm publishes are effectively permanent.** npm only allows un-publish within 72h and
> under strict conditions; treat a published version as forever. Double-check the version and
> `npm pack --dry-run` contents before publishing. You need `npm login` (an npm account with
> publish rights to `bodhveda` and the `@bodhveda` scope) first.

---

## 0. Pre-flight (safe to run)

```bash
# From repo root.
git switch main && git pull            # release from main
git status                             # clean tree except the release commit

# Confirm builds one more time.
( cd sdk/go && go build ./... && go vet ./... )
( cd sdk/js/core && npm ci && npm run build )
```

`npm login` if not already authenticated:

```bash
npm whoami || npm login
```

## 1. Publish `bodhveda` (JS core) — MUST be first

React depends on `bodhveda@^0.1.0`, so core has to exist on npm before react installs cleanly.

```bash
cd sdk/js/core
npm run build                          # refresh dist/ (also runs on prepublishOnly)
npm pack --dry-run                     # sanity-check the tarball contents (should ship dist/ only)
npm publish                            # publishes bodhveda@0.1.0 (publishConfig.access = public)
cd ../../..
```

Verify: `npm view bodhveda version` → `0.1.0`.

## 2. Publish `@bodhveda/react` (JS react)

```bash
cd sdk/js/react
npm install                            # now resolves bodhveda@^0.1.0 from npm (refreshes package-lock.json)
npm run build                          # tsc → dist/ (also runs on prepublishOnly)
npm pack --dry-run
npm publish                            # publishes @bodhveda/react@0.1.0
cd ../../..
```

> The committed `sdk/js/react/package-lock.json` still references the old core until this
> `npm install` regenerates it against the just-published `bodhveda@0.1.0`. Commit the updated
> lockfile if it changes.

Verify: `npm view @bodhveda/react version` → `0.1.0`.

## 3. Tag the Go module

The Go SDK is versioned by a **git tag on the module subpath** (`sdk/go/vX.Y.Z`). No `go
publish` — pushing the tag makes it resolvable via the Go proxy.

```bash
git tag sdk/go/v0.2.0
git push origin sdk/go/v0.2.0
```

Verify (may lag a minute while the proxy warms):

```bash
GOPROXY=proxy.golang.org go list -m github.com/MudgalLabs/bodhveda/sdk/go@v0.2.0
```

## 4. Deploy the docs (Mintlify)

The docs site (**docs.bodhveda.com**) is **not** built by `.github/workflows/deploy.yml`
(that workflow only builds/pushes the API image + SSH-deploys the VPS). There is no
`mint deploy` step in CI and no Mintlify config beyond `docs/docs.json`.

Mintlify deploys via its **GitHub App integration**: once the `docs/` changes land on the
default branch (`main`), Mintlify auto-builds and publishes docs.bodhveda.com. So:

1.  Merge this branch to `main` (the same merge that ships the SDK/README changes).
2.  Confirm the deploy in the **Mintlify dashboard** (Deployments/Activity). It should trigger
    automatically off the push to `main`.
3.  **If** the GitHub App is not configured for auto-deploy, deploy manually from the docs
    directory instead: `cd docs && mint deploy` (requires the Mintlify CLI + dashboard auth).
    Use `cd docs && mint dev` locally first to preview, and `cd docs && mint broken-links` to
    re-check links.

Verify: https://docs.bodhveda.com shows the new **Mediums** concept page and the **Contacts**
API group.

---

## What Phase 7.5 does with this

Phase 7.5 (deploy the running email medium to the VPS + Cloudflare and verify live) consumes
this runbook: it publishes the SDKs + docs here, then deploys the API/worker image and the
Console, and runs the live end-to-end email verification. See the Phase 7.5 section in
`agent-docs/overview.md`.
