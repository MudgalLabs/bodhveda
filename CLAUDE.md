# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

[Bodhveda](https://bodhveda.com) is an open-source notification platform (AGPLv3) by Mudgal Labs. It sends direct or broadcast notifications to recipients, respects per-recipient preferences, and exposes an inbox-style API. The repo is a monorepo containing the backend API + worker, the web Console, the SDKs, the docs site, and the deployment compose files.

## Top-level layout

- `api/` — Go backend. Two binaries (`cmd/api` HTTP server on `:1338`, `cmd/worker` Asynq worker). Shares all code under `internal/`.
- `console/` — React 19 + Vite + TanStack Router/Query frontend. Dev server on `:6970`. Deployed to Cloudflare (see `wrangler.toml`).
- `sdk/go/` — Go SDK (importable as `github.com/MudgalLabs/bodhveda/sdk/go`). Self-contained `go.mod`.
- `sdk/js/` — JS SDKs: `core` (vanilla, published as `bodhveda` on npm) and `react`.
- `migrations/` — Goose-flavored SQL migrations (`-- +goose Up`/`StatementBegin`).
- `docs/` — Mintlify docs site (`docs.json`, MDX under `docs/` and `api-reference/`).
- `compose.yaml` — local dev stack (db, redis, asynqmon). `deploy.compose.yaml` — production stack (api, worker, db, redis; no asynqmon).
- `Makefile` — orchestrates local dev via `tmux`.

## Common commands

```bash
# Local dev: starts db+redis+asynqmon in docker, then a tmux session with console (npm run dev) and api (air hot-reload).
make dev
make kill          # tear down docker + tmux session
make up            # just start db/redis/asynqmon
make down          # stop docker stack
make logs
make db            # psql shell into the dev db

# Build everything (api + console).
make build
make build_api     # go build -o ./bin/bodhveda ./cmd/api
make build_console # cd console && npm run build

# API alone (from api/)
air -c air.toml          # hot-reload http server (cmd/api)
go build ./cmd/api       # build api binary
go build ./cmd/worker    # build worker binary
go test ./...            # run tests (none currently exist)
go test ./internal/service -run TestName   # run a single test

# Console alone (from console/)
npm run dev      # vite on :6970
npm run build    # tsc -b && vite build
npm run lint     # eslint
npm run preview

# Production-like compose (builds local images from api.dockerfile / console.dockerfile)
make compose
```

Migrations are Goose-format SQL but no migration runner is wired into the Makefile or app startup — apply them manually (e.g. `goose -dir migrations postgres "$BODHVEDA_DB_URL" up`) when iterating on schema.

## Environment

A single `.env` at the repo root feeds both Go and Vite. Vite reads it via `envDir: "../"` in dev and exposes vars prefixed `BODHVEDA_` (see `console/vite.config.ts`). Go reads it via `godotenv.Load("../.env")` from `api/internal/app/app.go`. Required vars are enumerated in `api/internal/env/env.go` and `compose.yaml` — notably `BODHVEDA_DB_URL`, `BODHVEDA_REDIS_URL`, `BODHVEDA_WEB_URL`, the `BODHVEDA_GOOGLE_*` OAuth trio, and `BODHVEDA_API_CIPHER_KEY` / `BODHVEDA_API_HASH_KEY` (used for encrypting/HMACing API key tokens at rest). The console expects `BODHVEDA_API_URL` (defaults to `http://localhost:1338` in dev).

## Architecture

### Backend layering (`api/internal/`)

The Go backend follows a strict handler → service → repository layering wired up in `internal/app/app.go`:

- `handler/` — chi HTTP handlers. Each handler is a function that takes a service and returns `http.HandlerFunc`. Handlers do request decoding, call a service, and respond.
- `service/` — business logic. Constructors take repositories + cross-service deps + the Asynq client (for enqueuing background work).
- `pg/` — concrete pgx repository implementations. Implement interfaces declared in `model/repository/`.
- `model/` — split into `entity/` (DB rows / domain types), `dto/` (request/response shapes), `enum/` (string enums + `enum/error.go` for typed errors), `repository/` (interfaces only).
- `feature/user_identity/` and `feature/user_profile/` — newer "feature-folder" pattern (core + service + repository in one package). All other domains still use the layered split above. **When extending an existing domain, follow its existing pattern; don't refactor a layered domain into a feature folder mid-task.**
- `middleware/` — auth (`AuthMiddleware` for console session auth, `APIKeyBasedAuthMiddleware` for developer API), scope checks (`VerifyAPIKeyHasFullScope`), ownership checks (`VerifyUserOwnsThisProject`), implicit recipient creation (`CreateRecipientIfNotExists`), logging, timezones.
- `job/` — Asynq plumbing. `task/task.go` defines the task type constants; `processor/` holds the handlers; the api enqueues, the worker consumes.
- `env/`, `app/` — process-wide config and the `APP` singleton (DB pool, Asynq client, services, repositories).

There are two routing surfaces in `cmd/api/routes.go`:
1. **Developer API** (`/notifications`, `/recipients/...`) — auth via `Authorization` Bearer API key, permissive CORS (`*`), no credentials. API keys have a `scope` (`full` vs `recipient`) — full-scope routes are gated by `VerifyAPIKeyHasFullScope`. Recipient-scoped routes auto-create the recipient via `CreateRecipientIfNotExists`.
2. **Console API** (`/console/...`) — auth via cookie session (scs/pgxstore), strict CORS to `BODHVEDA_WEB_URL` with credentials, all project routes nested under `{project_id}` and gated by `VerifyUserOwnsThisProject`.

Both surfaces share the same handlers + services where it makes sense (e.g. `Notification.List` vs `Notification.ListForRecipient`).

### Background jobs

The api enqueues, the worker (`cmd/worker/main.go`) consumes. Task types live in `internal/job/task`:
- `notification:delivery` — fan-out for a direct send
- `broadcast:prepare_batches` — split a broadcast into batches
- `broadcast:delivery` — deliver one batch
- `recipient:delete_data` / `project:delete_data` — async cascading cleanup

In dev, `make up` starts `asynqmon` on `:7755` for queue introspection. The worker is a separate process — `make dev` does not start it; run `go run ./cmd/worker` from `api/` if you need to exercise jobs locally.

### Notification model (the core domain)

A notification is `{channel, topic, event}` (a `Target`) plus a free-form JSON `payload`. Two send modes:
- **Direct** — `recipient_external_id` is set; one row in `notification`.
- **Broadcast** — no recipient; creates a `broadcast` row, then the worker pipeline (`prepare_batches` → `delivery`) fans out to matching recipients in `broadcast_batch` chunks.

Preferences (`preference` table) gate delivery and exist at two levels:
- **Project-level** (recipient_external_id NULL, label NOT NULL) — defines the catalog of subscribable targets.
- **Recipient-level** (recipient_external_id NOT NULL, label NULL) — per-recipient opt-in/opt-out. Uniqueness for each level is enforced by partial unique indexes (see `migrations/20250801205117_init.sql`).

Billing (`service.BillingService`, `internal/pg/usage_*.go`, `user_subscription.go`) meters usage per project and is consulted on send-paths to enforce plan limits.

### External dependency: `tantra`

`github.com/mudgallabs/tantra` is a sibling shared library (logger, dbx pgx pool helper, httpx response helpers, oauth, session manager). When you see `httpx.SuccessResponse` / `httpx.ErrorResponse`, `logger.Get()`, `session.Manager`, or `oauth.InitGoogle`, those come from there — don't reimplement.

### Console (`console/src/`)

- Routing: TanStack Router with file-based routes under `routes/` (auto-generated `routeTree.gen.ts` — don't edit by hand). Auth context is injected into router context in `App.tsx`.
- Data: TanStack Query with a `QueryCache.onError` that funnels everything through `apiErrorHandler` (`lib/api.ts`), which toasts non-401 errors and silently redirects to `/auth/sign-in` on 401/403.
- API client: a single axios instance with `withCredentials: true` (session cookies). All endpoint URLs are centralized in `API_ROUTES` in `lib/api.ts` — add new endpoints there rather than hardcoding strings in components.
- Features under `src/features/{api_key,auth,billing,home,notification,preference,project,recipient}/` mirror the backend domains.
- UI lib: `netra` (Mudgal Labs' component lib — `SidebarProvider`, `TooltipProvider`, `toast`). Tailwind v4.

## Conventions worth knowing

- API key tokens are stored encrypted (`token` BYTEA + `nonce`) and indexed by HMAC (`token_hash`) — never log or return the plaintext token outside of the create-response path.
- The `UserIdentity` struct (`feature/user_identity/core.go`) carries the password hash; per the comment in that file, it must never be serialized to clients.
- Recipients are addressed externally by `external_id` (a string the customer chooses), not the internal serial `id`. Recipient-scoped routes use `{recipient_external_id}` in the URL.
- The Developer API is rate-limited to 100 req/min/IP via `httprate` middleware (`cmd/api/routes.go`).
- Console env vars must be prefixed `BODHVEDA_` to be exposed to the Vite client (see `envPrefix` in `vite.config.ts`).
