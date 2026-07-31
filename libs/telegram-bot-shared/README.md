# telegram-bot-shared

Shared boilerplate for this repo's Telegram bots (`apps/index-watch`, `apps/table-talks`, and new
bots scaffolded by `tools/generators/telegram-bot`):

- `health` — a minimal stdlib-only `GET /health` HTTP server for container liveness probes.
- `env` — dev/prod bot-token selection from `ENV`/`BOT_TOKEN`/`BOT_TOKEN_DEV`.
- `logging_setup` — the standard `logging.basicConfig` startup call these bots share.

Consumed via a local `uv` path dependency (`[tool.uv.sources]`), not a uv workspace — each
consuming app keeps its own independent `uv.lock`. Re-run `uv lock` in a consuming app after this
lib's source changes.
