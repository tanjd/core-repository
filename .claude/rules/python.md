---
paths:
  - "apps/index-watch/**"
  - "apps/table-talks/**"
  - "apps/ledger-lens-backend/**"
  - "apps/bookshelf-bot/**"
  - "libs/telegram-bot-shared/**"
  - "tools/generators/telegram-bot/**"
---

## Python (uv + ruff)

`apps/index-watch`, `apps/table-talks`, `apps/otobr-buddy`, and
`apps/ledger-lens-backend` (all migrated) are the only Python apps in the
repo so far; further apps are created on demand via `make new-bot` (see the
`scaffold-telegram-bot` skill) or migrated in like these were.

- **uv**, not Poetry — matches the real standalone bot repos and eases their
  eventual migration into this monorepo (see the `scaffold-telegram-bot`
  skill for detail).
- **Ruff** for lint + format. Shared rules live in a root-level `ruff.toml`;
  each app's `pyproject.toml` inherits them via
  `[tool.ruff] extend = "../../ruff.toml"` and can add per-app overrides
  below the `extend` line — ruff merges rather than replaces. Current shared
  rules: `target-version = "py313"`, `line-length = 100`,
  `select = ["E", "F", "I", "N", "W", "UP"]`, `quote-style = "double"`.
  **Important**: ruff's `src` setting does _not_ propagate through `extend`
  (confirmed by reproduction, not just docs) — every app must also set its
  own `src` below the `extend` line, relative to that app's actual package
  layout (see each app's own `CLAUDE.md` for its specific value), or
  first-party imports silently fail isort's import-grouping.
- Lint target (`uv run ruff check . && uv run ruff format --check .`) and
  test target (`uv run pytest`) are cached `nx:run-commands` targets — same
  non-plugin approach as Go, since there's no official Nx Python plugin
  (`@nxlv/python` was removed; see the `scaffold-telegram-bot` skill).
- **Shared code** (`libs/telegram-bot-shared`, used by both Telegram bots and the
  `telegram-bot` generator) is consumed via a `uv` local **path dependency**, not a uv
  workspace — each app keeps its own independent `uv.lock` and independent `nx release`
  versioning, consistent with every other Python app here. The convention: the consuming
  app's `pyproject.toml` declares the lib in `dependencies` and adds
  `[tool.uv.sources] <lib-name> = { path = "../../libs/<lib-name>", editable = true }`.
  Nx has no Python import-graph plugin, so the dependency edge is declared explicitly via
  `implicitDependencies` in the consuming app's `project.json` (same mechanism
  `apps/food-maps-e2e` uses for its non-import dependency on `food-maps`) — this is what
  lets `nx affected` correctly flag consumers when the shared lib changes.
  **Operational gotcha**: this doesn't auto-relock consumers — whenever
  `libs/telegram-bot-shared`'s source changes, run `uv lock` in every consuming app before
  its next `--frozen` sync (Docker build or CI) will succeed. A consuming app's Dockerfile
  also can't flatten `apps/<name>` straight to `/app` the way non-lib-consuming Python apps
  do — the `../../libs/<lib-name>` path has to resolve identically inside the image as it
  does on the host, so the image mirrors the `apps/<name>` + `libs/<lib-name>` depth under
  `/app` instead (see `apps/index-watch/Dockerfile` or `apps/table-talks/Dockerfile`).
