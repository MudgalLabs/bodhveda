.PHONY: up dev down logs db build build_console build_api \
	sdk_build sdk_build_js_core sdk_build_js_react sdk_build_go \
	sdk_publish_js_core sdk_publish_js_react sdk_tag_go \
	sdk_pack_js_core sdk_pack_js_react

up:
	docker compose up db redis asynqmon -d

down:
	docker compose down

logs:
	docker compose logs -f

db:
	docker compose exec db psql -U postgres -d postgres

dev:
	$(MAKE) up
	tmux new-session -d -s bodhveda \
		"cd console && npm run dev" \; \
		split-window -v -t 0 "cd api && air -c air.toml" \; \
		split-window -h -t 1 "cd api && air -c air.worker.toml" \; \
		select-pane -t 0 \; \
		split-window -h -t 0 "bash" \; \
		select-pane -t 1 \; \
		send-keys "clear" C-m

	tmux attach -t bodhveda

kill:
	@docker compose down && tmux kill-session -t bodhveda;

compose:
	@docker compose build
	@docker compose up 


build:
	@echo ""
	@echo "🔨 Building api ..."
	@if $(MAKE) build-api; then \
		echo "✅ api build succeeded"; \
	else \
		echo "❌ api build failed"; exit 1; \
	fi
	@echo ""
	@echo "🔨 Building console ..."
	@if $(MAKE) build-console; then \
		echo "✅ console build succeeded"; \
	else \
		echo "❌ console build failed"; exit 1; \
	fi
	@echo ""

build_console:
	@cd console && npm run build

build_api:
	@cd api && go build -o ./bin/bodhveda ./cmd/api

# ---------------------------------------------------------------------------
# SDK release
#
# Three packages ship in lockstep under one version number:
#   @bodhveda/js     (sdk/js/core)  — npm
#   @bodhveda/react  (sdk/js/react) — npm, depends on @bodhveda/js
#   sdk/go           — Go module, released by git tag
#
# Publish is irreversible (npm) or credential-gated (git push): the publish/
# tag targets are for a human to run, in the order below, one package at a
# time. Build/pack targets are safe to run anytime.
#
#   make sdk_build              # build all three (verify they compile)
#   make sdk_publish_js_core    # 1. publish @bodhveda/js   (must be first)
#   make sdk_publish_js_react   # 2. publish @bodhveda/react (needs core on npm)
#   make sdk_tag_go             # 3. tag + push the Go module
# Then tag the JS packages to match (see agent-docs runbook).
# ---------------------------------------------------------------------------

sdk_build: sdk_build_go sdk_build_js_core sdk_build_js_react
	@echo "✅ all SDKs built"

sdk_build_go:
	@echo "🔨 building sdk/go ..."
	@cd sdk/go && go build ./... && go vet ./...

sdk_build_js_core:
	@echo "🔨 building @bodhveda/js (sdk/js/core) ..."
	@cd sdk/js/core && npm ci && npm run build

# Verifies react against the LOCAL core (--no-save: never touches package.json /
# lockfile), so it builds even before @bodhveda/js is on npm. The real registry
# install + lockfile refresh happens in sdk_publish_js_react, after core is live.
sdk_build_js_react:
	@echo "🔨 building @bodhveda/react (sdk/js/react) against local core ..."
	@cd sdk/js/react && npm install --no-save ../core && npm run build

# Dry-run the publish tarballs — safe, shows exactly what would ship.
sdk_pack_js_core: sdk_build_js_core
	@cd sdk/js/core && npm pack --dry-run

sdk_pack_js_react: sdk_build_js_react
	@cd sdk/js/react && npm pack --dry-run

# --- Publish (irreversible; run one at a time, in order) ---
sdk_publish_js_core: sdk_build_js_core
	@npm whoami >/dev/null || (echo "run 'npm login' first" && exit 1)
	@cd sdk/js/core && npm pack --dry-run && npm publish

# Real registry install (resolves @bodhveda/js@^0.3.0 published in the step above
# and refreshes package-lock.json — commit it after), then build + publish.
sdk_publish_js_react:
	@npm whoami >/dev/null || (echo "run 'npm login' first" && exit 1)
	@cd sdk/js/react && npm install && npm run build && npm pack --dry-run && npm publish

# Tag + push the Go module. VERSION defaults to the sdk/go CHANGELOG's top entry.
sdk_tag_go:
	@ver=$${VERSION:-$$(grep -m1 -oE 'v[0-9]+\.[0-9]+\.[0-9]+' sdk/go/CHANGELOG.md)}; \
	echo "tagging sdk/go/$$ver"; \
	git tag "sdk/go/$$ver" && git push origin "sdk/go/$$ver"
