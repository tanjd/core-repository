# Index Watch

Telegram bot that tracks index drawdowns (e.g. S&P 500, NASDAQ-100, MSCI World), sends daily reports with drawdown metrics and CNN Fear & Greed, and alerts when drawdown crosses 5%/10%/15%/20% with historical frequency context. Built as a crash-buy helper.

## Features

- **Daily report** (configurable cron, default 22:00 UTC Mon–Fri): drawdown metrics per index, Fear & Greed Index, and historical “how often” stats for each threshold.
- **Drawdown alerts**: when current drawdown exceeds 5%, 10%, 15%, or 20%, you get a single digest notification (even if multiple thresholds/indices trigger at once) with how many trading days in history the index closed at or below each drawdown.
- **Recovery / new-ATH notifications**: when an index recovers back to (or sets a new) all-time high, you get notified — folded into the same digest message as any drawdown alerts from that check cycle. Always-on, no opt-out.
- **Per-subscriber settings**: thresholds and tracked indices default to the global config but can be overridden per-subscriber via `/setthresholds` and `/setindices`.
- **Commands**: `/start`, `/subscribe`, `/unsubscribe`, `/status`, `/mystats`, `/daily` (manual report), `/alerts` (show thresholds and config), `/mysettings`, `/setthresholds`, `/myindices`, `/setindices`.

## Setup

1. Create a bot with [@BotFather](https://t.me/BotFather) and copy the token.
2. Get your Telegram chat ID (e.g. send a message to the bot and call `getUpdates` on the Bot API, or use [@userinfobot](https://t.me/userinfobot)).
3. Copy `.env.example` to `.env` and set:

```bash
BOT_TOKEN=your_production_bot_token_here
BOT_TOKEN_DEV=your_development_bot_token_here
ENV=dev
TELEGRAM_CHAT_IDS=123456789,987654321
```

4. Install and run:

```bash
uv sync
make run
```

## Environment variables

| Variable                  | Description                                              | Default                         |
| ------------------------- | -------------------------------------------------------- | ------------------------------- |
| `BOT_TOKEN`               | Production bot token from @BotFather                     | required (when ENV ≠ dev)       |
| `BOT_TOKEN_DEV`           | Development bot token from @BotFather                    | required (when ENV=dev)         |
| `ENV`                     | `dev` → use BOT_TOKEN_DEV; anything else → use BOT_TOKEN | `prd`                           |
| `TELEGRAM_CHAT_IDS`       | Comma-separated chat IDs for reports and alerts          | —                               |
| `DRAWDOWN_THRESHOLDS_PCT` | Space-separated thresholds (e.g. `5 10 15 20`)           | `5 10 15 20`                    |
| `DAILY_REPORT_CRON`       | Cron: minute hour day month weekday (UTC)                | `0 22 * * 1-5`                  |
| `ALERT_CHECK_MINUTES`     | How often to check drawdown for alerts                   | `60`                            |
| `HISTORY_YEARS`           | Years of history for ATH and frequency stats             | `20`                            |
| `INDEX_SYMBOLS`           | Tracked indices, `symbol:Name` pairs separated by `;`    | S&P 500, NASDAQ-100, MSCI World |

## Indices

Default indices (Yahoo Finance symbols), overridable via `INDEX_SYMBOLS`:

- **S&P 500**: `^GSPC`
- **NASDAQ-100**: `^NDX`
- **MSCI World**: `^990100-USD-STRD`

## Data sources

- **Prices**: [Yahoo Finance](https://finance.yahoo.com/) via `yfinance`.
- **Fear & Greed**: [CNN Fear & Greed Index](https://www.cnn.com/markets/fear-and-greed) via the `fear-and-greed` package (scrapes CNN).

## Development

```bash
make setup   # install deps + pre-commit
make test    # pytest
make check   # lint, format, typecheck, test
```

## License

See repository license.
