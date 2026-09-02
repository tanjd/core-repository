MAKEFLAGS += --no-print-directory

.PHONY: help setup setup-ci install-deps install-dev-tools install-goimports install-rtk install-govulncheck verify affected new-bot docker-build golangci-verify nx-reset upgrade-nx prune-devcontainer-cache

.DEFAULT_GOAL := help

help: ## Show this help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "\n\033[36m%-16s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

##@ Development

setup: install-deps install-dev-tools install-govulncheck ## Full local dev setup (deps + Playwright + goimports + rtk + govulncheck)

setup-ci: install-deps install-govulncheck ## CI setup: deps + Playwright + govulncheck, skipping the dev-only Husky/Claude/Cursor tooling that has no business running in CI

install-deps: ## Install workspace deps and Playwright's chromium (needed by CI and local dev alike)
	pnpm install --frozen-lockfile
	pnpm exec playwright install --with-deps chromium

install-dev-tools: install-goimports install-rtk ## Install local-only dev tools (goimports for Husky, rtk for Claude/Cursor)

install-goimports: ## Install goimports (used by Husky lint-staged on staged .go files)
	go install golang.org/x/tools/cmd/goimports@latest

install-rtk: ## Install rtk + init Claude/Cursor token-saving hooks
	curl -fsSL --retry 5 --retry-all-errors --retry-delay 5 \
		https://raw.githubusercontent.com/rtk-ai/rtk/master/install.sh | RTK_VERSION=v0.44.1 sh
	mkdir -p $(HOME)/.claude
	rtk init -g --auto-patch
	mkdir -p $(HOME)/.cursor
	rtk init -g --agent cursor --auto-patch

install-govulncheck: ## Install govulncheck (Go vulnerability scanner, used by the govulncheck Nx target)
	go install golang.org/x/vuln/cmd/govulncheck@latest

verify: ## Build, lint, and test every project (full local CI equivalent)
	pnpm nx run-many -t build lint test

affected: ## Lint and test only what changed vs main (what CI actually runs)
	pnpm nx affected -t lint test

nx-reset: ## Clear the Nx cache (use when a target result looks stale)
	pnpm nx reset

##@ Telegram Bots

new-bot: ## Scaffold a new bot: make new-bot NAME=my-bot
	@if [ -z "$(NAME)" ]; then echo "Usage: make new-bot NAME=my-bot"; exit 1; fi
	pnpm nx g ./tools/generators/telegram-bot/generators.json:telegram-bot $(NAME)

##@ Docker

docker-build: ## Build an app's Docker image: make docker-build APP=food-maps-backend
	@if [ -z "$(APP)" ]; then echo "Usage: make docker-build APP=<app-name>"; exit 1; fi
	docker build -f apps/$(APP)/Dockerfile -t $(APP) .

##@ Go

golangci-verify: ## Confirm golangci-lint actually loaded .golangci.yaml (not silently defaulting)
	cd apps/food-maps-backend && golangci-lint run -v 2>&1 | grep "Used config file"

##@ Maintenance

upgrade-nx: ## Upgrade the monorepo's Nx version
	npx nx migrate latest
	npx nx migrate --run-migrations

prune-devcontainer-cache: ## Reclaim disk from stale entries in the persistent devcontainer caches (pnpm store, Go build cache, old Playwright browser versions)
	pnpm store prune
	go clean -cache
	@browsers_dir="$${PLAYWRIGHT_BROWSERS_PATH:-$$HOME/.cache/ms-playwright}"; \
	keep=$$(pnpm exec playwright install chromium --dry-run 2>/dev/null | grep 'Install location:' | awk '{print $$NF}' | xargs -n1 basename); \
	for dir in "$$browsers_dir"/*/; do \
		[ -d "$$dir" ] || continue; \
		name=$$(basename "$$dir"); \
		if ! printf '%s\n' "$$keep" | grep -qxF "$$name"; then \
			echo "Removing stale Playwright browser: $$name"; \
			rm -rf "$$dir"; \
		fi; \
	done
