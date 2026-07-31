# Table Talks

Telegram bot that turns thoughtful questions into simple conversation games.
Choose a topic, draw a card, and connect — one prompt at a time.

## Features

- **Theme-based conversation cards**: pick a theme (e.g. marriage, fun/light,
  faith) and draw questions one at a time, with next/previous navigation.
- **Pluggable data sources**: questions/themes load from tracked CSV files by
  default, with an optional Google Sheets backend (falls back to CSV on any
  error) for editing content without a redeploy.
- **Bot info screen**: shows the running version and recent changelog
  entries, read live from this app's `package.json`/`CHANGELOG.md`.
- **Support/coffee link**: optional "Support Creator" screen.
- **Health check**: stdlib HTTP server on `HEALTH_PORT` (default `9999`),
  used by the Dockerfile's `HEALTHCHECK`.

## Setup

1. Create a bot with [@BotFather](https://t.me/BotFather) and copy the token.
2. Copy `.env.example` to `.env` and set:

```bash
BOT_TOKEN=your_production_bot_token_here
BOT_TOKEN_DEV=your_development_bot_token_here
ENV=dev
```

3. Install and run:

```bash
uv sync
uv run python -m table_talks.index
```

## Environment variables

See `.env.example` for the full reference — required bot tokens, optional
health-check port, optional Buy Me a Coffee link, and optional Google Sheets
integration (`ENABLE_GOOGLE_SHEETS`, `GOOGLE_SHEET_ID`,
`GOOGLE_SERVICE_ACCOUNT_FILE`/`GOOGLE_SERVICE_ACCOUNT_JSON`,
`GOOGLE_SHEET_NAME`, `SHEETS_CACHE_TTL`).

## Data

`data/questions.csv` and `data/themes.csv` are tracked source content, not
generated artifacts. `scripts/csv_to_sheets.py` converts them into a
denormalized CSV suitable for importing into a Google Sheet, for anyone who
wants to switch to the Sheets-backed data source.

## Development

```bash
pnpm nx build table-talks
pnpm nx test table-talks
pnpm nx lint table-talks
pnpm nx serve table-talks
```

## License

See repository license.
