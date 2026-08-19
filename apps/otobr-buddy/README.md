# otobr-buddy

Telegram bot that supports one-to-one Bible reading partnerships at church:
tracks what was read, reminds pairs to schedule their next session, keeps a
history of past reading partners, and shows lightweight stats.

See [CLAUDE.md](CLAUDE.md) for the full product spec, data model, and
architecture notes.

## Commands

- `/pair` — add the bot to a group chat with your reading partner and both
  run this there to form a partnership (no code to share)
- `/invite` / `/join <code>` — pair by DM instead, if you'd rather not share
  a group chat
- `/partners` — list your active reading partnerships
- `/log <text>` — log a reading session, e.g. `/log Romans 8:1-17`
- `/setfrequency` — set a reminder schedule (`interval <days>` or `weekly <day> <HH:MM>`)
- `/link` — link the current group chat to a DM-formed partnership
- `/end` — end a partnership (moves it to history)
- `/history` — view past partnerships
- `/stats` — session counts, streaks, and recent coverage

If you have more than one active partnership, prefix a command with which one
(`#1`, `#2`, ...) as shown by `/partners`, e.g. `/log #2 Romans 8:1-17`.

## Development

```bash
pnpm nx serve otobr-buddy   # run the bot locally (uses ENV=dev -> BOT_TOKEN_DEV)
pnpm nx test otobr-buddy    # run pytest
pnpm nx lint otobr-buddy    # ruff check + format --check
```

Copy `.env.example` to `.env` and fill in `BOT_TOKEN_DEV` from @BotFather before running.

## Deployment

Built as a Docker image (see `Dockerfile`, build context is the repo root) and
versioned independently via `nx release`. Mount a volume at
`apps/otobr-buddy/data` so the sqlite database persists across restarts.
