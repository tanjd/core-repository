# bookshelf-bot

Telegram companion to `apps/bookshelf` (community book-lending app) — links a member's
Telegram account to their bookshelf account so `apps/bookshelf-backend` can push
notifications (loan requests, approvals, returns, wishlist matches, due-date reminders)
directly to Telegram. See
[`apps/bookshelf/docs/telegram-bot-integration-spec.md`](../bookshelf/docs/telegram-bot-integration-spec.md)
for the full design.

This bot is intentionally thin and stateless: it holds no database of its own.
`bookshelf-backend` is the source of truth for who's linked, and sends Telegram
messages directly (not through this bot) — the only thing this bot does is complete the
"Connect Telegram" deep link (`/start <token>`, calling
`POST /internal/telegram/confirm-link` on the backend) and answer `/help`.

## Development

```bash
# Run tests
pnpm nx test bookshelf-bot

# Lint
pnpm nx lint bookshelf-bot

# Run the bot locally (reads .env in this directory)
pnpm nx serve bookshelf-bot

# Build a Docker image
pnpm nx docker-build bookshelf-bot
```

Copy `.env.example` to `.env` and fill in `BOT_TOKEN` (or `BOT_TOKEN_DEV` for local
development), `BOOKSHELF_BACKEND_URL`, and `BOOKSHELF_INTERNAL_TOKEN` (must match
`apps/bookshelf-backend`'s `TELEGRAM_INTERNAL_SECRET`) before running.

## Deployment

Pushing to `main` with changes affecting this app triggers
`.github/workflows/release.yml`, which builds this app's `Dockerfile` and
pushes it to `ghcr.io/tanjd/bookshelf-bot`.
