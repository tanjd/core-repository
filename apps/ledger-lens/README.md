# Ledger Lens

Multi-broker portfolio analysis dashboard supporting Interactive Brokers
(IBKR) and Moomoo. Upload CSV statements per year per broker and get a
multi-year view of your portfolio's NAV, P&L, dividends, trades, and cash
flows. Talks to the [ledger-lens-backend](../ledger-lens-backend) FastAPI
API.

## Features

- **Overview** — NAV history, time-weighted return, asset allocation, year-over-year summary; broker-aware in multi-broker mode
- **Holdings** — open positions with cost basis, unrealized P&L, and portfolio allocation chart; Combined/IBKR/Moomoo tabs in multi-broker mode
- **Trades** — stock and forex trade history with direction badges, commission breakdown, and broker badges
- **Income** — dividends and withholding tax by symbol (IBKR)
- **Cash Flows** — deposit/withdrawal timeline with monthly bar chart and cumulative NAV vs invested chart (IBKR)
- **P&L Analysis** — realized/unrealized P&L (short-term + long-term), mark-to-market, corporate actions (IBKR)
- **Trends** — multi-year portfolio growth, TWR by year, deposit vs growth breakdown (IBKR)
- **Upload History** — persistent log of every import (manual upload or file-drop), with per-file counts and status
- **Privacy mode** — blur all monetary values with a single toggle
- **Upload dialog** — 2-step preview → import flow with duplicate detection and inline result summary

## Tech Stack

Next.js · React · Tailwind CSS · shadcn/ui · Recharts · SWR

## Setup

```bash
cp .env.example .env
cp ../ledger-lens-backend/.env.example ../ledger-lens-backend/.env
pnpm nx dev ledger-lens   # starts both the backend (:8000) and this app (:3000)
```

Or run each side in its own terminal (e.g. to watch their logs separately):

```bash
pnpm nx serve ledger-lens-backend   # in one terminal, :8000
pnpm nx serve ledger-lens           # in another, :3000
```

Open [http://localhost:3000](http://localhost:3000), then upload your IBKR
or Moomoo CSV via the upload button.

## Supported CSV Formats

| Broker | File type                 | Filename pattern                                       |
| ------ | ------------------------- | ------------------------------------------------------ |
| IBKR   | Annual activity statement | `U1234567_2024_2024.csv`                               |
| IBKR   | YTD statement             | `U1234567_20240101_20240930.csv`                       |
| Moomoo | Trade history             | `History-Margin Account(123456)-20240101-120000.csv`   |
| Moomoo | Positions snapshot        | `Positions-Margin Account(123456)-20240101-120000.csv` |

Export from IBKR: **Reports → Tax Documents / Activity → Annual Activity
Statement → CSV**.

Re-uploading the same year is safe — the ingestor fully replaces the
previous data. Every import is recorded in the Upload History page.

## Dev Commands

```bash
pnpm nx dev ledger-lens          # start this app + ledger-lens-backend together
pnpm nx serve ledger-lens        # start this app's development server only
pnpm nx build ledger-lens        # build for production
pnpm nx lint ledger-lens         # run linting
pnpm nx run ledger-lens:docker-build
```

## License

MIT
