# Ledger Lens Backend

FastAPI backend for [Ledger Lens](../ledger-lens), a multi-broker portfolio
analysis dashboard supporting Interactive Brokers (IBKR) and Moomoo. Watches
a data directory for CSV statements, parses and upserts them into SQLite,
and serves the API the frontend consumes.

## Tech Stack

Python 3.14 · FastAPI · SQLModel · SQLite · Watchdog · uv · Ruff ·
basedpyright · pytest

## Setup

```bash
cp .env.example .env
pnpm nx serve ledger-lens-backend
```

Drop CSV files into `data/` (or the path set via `LEDGER_DATA_DIR`) — the
backend picks them up automatically via a filesystem watcher. See the
[frontend README](../ledger-lens/README.md) for supported CSV formats and
the full dashboard feature set.

## Environment variables

| Variable              | Description                                                  | Default                 |
| --------------------- | ------------------------------------------------------------ | ----------------------- |
| `LEDGER_DATA_DIR`     | Directory where CSVs are watched and the SQLite DB is stored | `./data`                |
| `LEDGER_CORS_ORIGINS` | Comma-separated CORS origins allowed to call the API         | `http://localhost:3000` |

## API Endpoints

- `POST /api/upload` — upload a new CSV, triggers ingest
- `GET /api/upload-history` — log of all ingest events, newest first
- `GET /api/years`, `/api/brokers`, `/api/broker-info` — ingested-data metadata
- `GET /api/overview`, `/api/holdings`, `/api/trades`, `/api/income`,
  `/api/cashflows`, `/api/performance` — per-year dashboard data
- `GET /api/timeseries/{nav,deposits,dividends,pnl,dca,commissions}` —
  multi-year time series
- `GET /health` — health check
- `GET /api/version` — running app version

## Dev Commands

```bash
pnpm nx serve ledger-lens-backend        # run with reload on :8000
pnpm nx test ledger-lens-backend         # pytest
pnpm nx lint ledger-lens-backend         # ruff check + format --check
pnpm nx run ledger-lens-backend:docker-build
```

## License

MIT
