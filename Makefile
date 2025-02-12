.FORCE:
.DEFAULT_GOAL := help

.PHONY: help
help:
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m\033[0m\n\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "\033[36m%-10s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)


.PHONY: setup
setup: ## setup repo
	pnpm install --frozen-lockfile

upgrade-nx: ## upgrade monorepo
	npx nx migrate latest
	npx nx migrate --run-migrations
