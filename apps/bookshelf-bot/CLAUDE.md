# CLAUDE.md

Guidance for `apps/bookshelf-bot` specifically — see the repo-root `CLAUDE.md` for cross-cutting
conventions (Nx, Python/uv tooling, deployment, release process).

Telegram companion to `apps/bookshelf` (community book-lending app) and `apps/bookshelf-backend`.
Scaffolded via `make new-bot NAME=bookshelf-bot` (see the `scaffold-telegram-bot` skill), using
`libs/telegram-bot-shared` for health-check server, dev/prod bot-token selection, and logging
setup — same pattern as `index-watch`/`table-talks`/`otobr-buddy`.

Unlike those three bots, this one holds **no database of its own** and is intentionally thin and
stateless — `bookshelf-backend` is the source of truth for who's linked, and sends Telegram
messages directly via `internal/services/telegram.go` rather than routing through this bot (see
`apps/bookshelf/docs/telegram-bot-integration-spec.md`'s "Architecture decision" section for why).
This bot's only job is completing the "Connect Telegram" deep link:

- `/start <token>` (`src/bookshelf_bot/bot.py`'s `cmd_start`) calls `POST
/internal/telegram/confirm-link` on `bookshelf-backend`, authenticated via a shared
  `BOOKSHELF_INTERNAL_TOKEN` (must match the backend's `TELEGRAM_INTERNAL_SECRET`) rather than a
  user session — the bot has no user auth of its own. A bare `/start` (no token arg) and `/help`
  both reply with the same static help text pointing back to the bookshelf profile page.
- `LinkError` wraps every backend-call failure (network error, non-200 response) into one
  human-readable reply telling the member to re-tap "Connect Telegram" for a fresh link, rather
  than surfacing raw HTTP detail.

## Environment

`.env.example` → copy to `.env` in this directory. Needs `BOT_TOKEN`/`BOT_TOKEN_DEV` (see
`telegram_bot_shared.env.select_bot_token`), `BOOKSHELF_BACKEND_URL`, and
`BOOKSHELF_INTERNAL_TOKEN`.

## Deployment

Dockerized (GHCR, `ghcr.io/tanjd/bookshelf-bot`) and versioned independently via `nx release`
(`release.projects` in root `nx.json`), same event chain as every other deployable app — see the
`release-and-deployment` skill.
