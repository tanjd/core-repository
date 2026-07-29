MAKEFLAGS += --no-print-directory

.PHONY: help setup verify affected new-bot docker-build golangci-verify nx-reset upgrade-nx

.DEFAULT_GOAL := help

help: ## Show this help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "\n\033[36m%-16s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

##@ Development

setup: ## Install deps (frozen lockfile), goimports, and rtk (Claude/Cursor token-saving hooks)
	pnpm install --frozen-lockfile
	go install golang.org/x/tools/cmd/goimports@latest
	curl -fsSL https://raw.githubusercontent.com/rtk-ai/rtk/master/install.sh | RTK_VERSION=v0.44.1 sh
	rtk init -g --auto-patch
	mkdir -p $(HOME)/.cursor
	rtk init -g --agent cursor --auto-patch

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
