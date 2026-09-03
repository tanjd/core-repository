# Benchmark Comparison & Money-Weighted Return (XIRR) — spec

**Status:** Draft — not yet approved for build · **Scope:** `apps/ledger-lens-backend` +
`apps/ledger-lens` · **Depends on:** `Statement`, `Deposit`

Two additions to the Trends/Overview pages that both answer the same underlying question —
_"is this actually working?"_ — better than the metrics already there:

1. **Benchmark comparison** — plot the portfolio's return next to an index (S&P 500 or another
   chosen index) so "+12% this year" has something to be judged against.
2. **Money-weighted return (XIRR)**, presented to the user as their personal annualized return —
   the number that accounts for _when_ money went in and out, not just how the NAV moved.

This spec deliberately scopes to **intent and behavior**, not implementation. Table shapes, route
strings, and file names below are illustrative, not directives — the sections that matter are the
data-source decision, the calculation semantics, and the non-goals.

## Why these two, together

Every return figure this app currently shows — the per-year `twr_pct` on Overview
(`app/parser/sections/nav.py` → `Statement.twr_pct`) and the cumulative-TWR card on
`AllTimeSummaryCards` (`∏(1 + twr_i / 100) − 1`, computed client-side) — is **time-weighted**: it
measures how well the _investments_ performed, deliberately blind to when deposits and
withdrawals happened. That's the right metric for judging a fund manager. It's the wrong one for
answering "am I beating a savings account, or the index, given how _I_ actually moved money in
and out" — a lump sum invested right before a crash and a lump sum invested right before a rally
can post the identical TWR while the investor's actual outcomes are opposite. XIRR is the
standard fix: it weights each cash flow by both size and date.

Benchmark comparison has the same gap from a different angle: TWR alone has no reference point.
"+12%" reads as a win until you learn the index did +24% over the same stretch.

Both features build on data already in the DB — dated `Deposit` rows and annual `twr_pct` —
rather than needing new ingestion of portfolio-side data. The only genuinely new data this spec
introduces is the benchmark's own price history (see "Where does index data come from," below).

## What the existing data does (and doesn't) support

Worth stating plainly before designing either feature, since both are shaped by these constraints:

- **TWR is annual, not daily, and it isn't computed by this app.** `parse_nav()` reads
  `Time Weighted Rate of Return` straight off the IBKR statement — IBKR computes it, we just
  store it (`app/parser/sections/nav.py:24-25`). There is no daily or monthly NAV curve anywhere
  in the schema, only one `twr_pct` per `(account_id, year)` `Statement` row. Any comparison
  against a benchmark is therefore naturally **per-statement-period**, not daily — this matches
  the granularity `TwrByYearChart`/`PortfolioGrowthChart` already show.
- **A `Statement`'s period isn't always a calendar year.** `period_start`/`period_end` capture
  either a full year (`2025-01-01`→`2025-12-31`) or a YTD stub for the in-progress year
  (`Statement` model comment, `app/models/db.py:24-27`). Both features need to compare against
  the _same_ window, not the calendar year, or the current year's numbers will be wrong by
  construction. Legacy rows ingested before this field existed have `period_start = None` —
  those years can't be benchmark-matched and should be flagged, not guessed at.
- **Dated cash flows only exist for IBKR.** `Deposit` rows come from parsing the
  "Deposits & Withdrawals" section of an IBKR Activity Statement
  (`app/parser/sections/cash.py`). Moomoo has no such export — `timeseries_deposits`
  (`app/routers/timeseries.py:82-94`) already works around this for the deposits chart by
  _approximating_ net capital deployed from `-sum(trade.proceeds)`, aggregated per year, not
  dated per transaction. That approximation is fine for a bar chart; it is not fine as an XIRR
  cash-flow input, which needs both a specific date and a specific investor-side sign per
  transaction (see "Broker scope," below).
- **"Current value" means "as of the latest ingested statement," not live.** Nothing in this app
  calls out to a market-data feed — `holdings.py`'s "current" positions are the latest statement's
  year-end marks. Both new features inherit that: an XIRR or benchmark comparison is "as of the
  last upload," not real-time, same as everything else in the dashboard.
- **Mixed-currency totals are a checked error, not silently summed.** `overview.py` and
  `holdings.py` both 409 when statements/positions span more than one currency
  (`f07275d`). Both new endpoints need the same discipline — an XIRR or benchmark run over
  cash flows that quietly span SGD and USD would produce a number-shaped lie. Worth noting in
  passing: `cashflows.py`'s existing `total_usd` field (`app/routers/cashflows.py:27`) sums
  `deposits_withdrawals` regardless of currency and is misleadingly named — not this spec's
  problem to fix, but don't copy that pattern into the new code.

## Feature 1 — Benchmark comparison

### Where does index data come from?

This is the one real open decision in this spec, because it's an architecture choice, not just a
calculation choice. `ledger-lens-backend` has no runtime network dependency today — it's
CSV-drop-and-watch, entirely offline (`README.md`, `app/config.py`; `httpx` is a dev-only
dependency, used only in tests). Three ways to get index prices in:

1. **User uploads a benchmark CSV** (date + close, the shape every free source — Yahoo Finance,
   Stooq, investing.com — exports as), through a small new upload flow that mirrors the existing
   one conceptually but isn't the same pipeline (see below). No new dependency, no outbound
   network call, consistent with everything else this app does. Cost: the user has to go get the
   file and re-upload periodically to keep coverage current, same chore as dropping in a new
   broker statement each year.
2. **Fetch live from a free external API** (e.g., Stooq's no-key CSV endpoint) at request or
   ingest time, cached in SQLite. Zero manual steps once configured. Cost: this becomes the
   app's first outbound network dependency, which matters for a self-hosted service (firewall
   rules, an origin that can go away or rate-limit, a new failure mode on every request unless
   cached carefully).
3. **Bundle a small pinned historical dataset** in the repo. Zero runtime dependency, but it goes
   stale between releases and can't be "a chosen index" — only whatever's bundled.

**Recommendation: (1), CSV upload.** It's the only option that doesn't compromise the "fully
offline, self-hosted" property every other part of this app already has, and it generalizes
cleanly to "a chosen index" — any symbol the user has uploaded prices for becomes selectable,
not just S&P 500. If this turns out to be too much manual friction in practice, (2) can be added
later as an optional convenience _on top of_ the same `BenchmarkPrice` table, without changing
the comparison/chart logic at all — worth keeping in mind as the fallback, but not the v1 design.

### Data model

A new table, independent of the broker-statement ingestion pipeline (this is _not_ another
`Statement` — no account, no year, no broker):

```
BenchmarkPrice
  id            int, pk
  symbol        str            # user-chosen label, e.g. "SPY", "^GSPC" — not validated against a live ticker list
  price_date    date
  close         float
  unique(symbol, price_date)
```

### Ingestion

A dedicated small endpoint (`POST /api/benchmarks/upload?symbol=SPY`, multipart CSV) — not routed
through `app/services/ingestor.py`'s existing broker-detection/sectioned-CSV pipeline, which is
built around IBKR/moomoo's specific multi-section statement format. A benchmark file is just two
columns; reuse would mean bending that pipeline to a shape it wasn't built for. Upsert on
`(symbol, price_date)` so re-uploading an overlapping range to extend coverage is safe. No
filesystem-watcher auto-ingestion for this (`app/services/watcher.py` stays broker-statement-only)
— see Non-goals.

`GET /api/benchmarks` lists distinct symbols with row count and min/max `price_date`, so the
frontend can populate a picker and show coverage gaps ("SPY: 2019-01-01 → 2025-12-31").

### Calculation

For each `Statement` (grouped the same way `timeseries_nav` already does — one row per year,
IBKR's `twr_pct` as the portfolio figure for that period), with a non-null `period_start`:

- `price_start` = latest `BenchmarkPrice.close` with `price_date <= period_start`
- `price_end` = latest `BenchmarkPrice.close` with `price_date <= period_end`
- `benchmark_return_pct` = `(price_end / price_start − 1) × 100`
- Coverage flag: `"full"` if both prices exist and bracket the statement period reasonably (e.g.
  within a handful of days — exact tolerance is an implementation detail), `"missing"` if either
  side has no price at or before it (index history doesn't reach back far enough, or hasn't been
  uploaded yet for a recent period). A `"missing"` year is _shown as a gap_, never silently
  dropped or interpolated — a benchmark line that quietly skips a bad year is worse than one with
  a visible hole in it.

Chain-link both series the same way `AllTimeSummaryCards.computeCumulativeTwr` already does for
the portfolio (`∏(1 + r_i / 100)`, rebased to 100 at the first year both series have `"full"`
coverage), so they're visually comparable on one line chart — a direct sibling of
`PortfolioGrowthChart`. A second chart pairs per-year `twr_pct` against `benchmark_return_pct` as
grouped bars, extending `TwrByYearChart`'s existing shape rather than replacing it (add a new
component alongside it so the no-benchmark-selected case is unaffected).

`GET /api/timeseries/benchmark?symbol=SPY` returns the per-year rows (`year`, `twr_pct`,
`portfolio_cum_index`, `benchmark_return_pct`, `benchmark_cum_index`, `coverage`) the two charts
need. The Trends page gains a symbol picker (populated from `GET /api/benchmarks`) that's absent
entirely — not just disabled — when no benchmark has been uploaded yet, same "don't show an
empty affordance for a feature with nothing behind it" instinct as `TrendsPage`'s existing
`hasData` gate.

### Non-goals (v1)

- **Live/automatic price fetching.** See the recommendation above — deliberately deferred, not
  ruled out.
- **Multiple simultaneous benchmarks on one chart.** One selected symbol at a time; comparing two
  indices against each other isn't this feature's job.
- **Dividend-adjusted (total-return) index data.** Whether "S&P 500" means price return or total
  return depends entirely on which CSV the user sources and uploads — this app has no opinion and
  doesn't attempt to adjust for index dividends itself. Worth a line of UI copy so the comparison
  isn't read as more precise than it is.

## Feature 2 — Money-weighted return (XIRR), presented as annualized personal return

### One metric, not two

The ask names both XIRR and CAGR, but they shouldn't become two separate numbers on the page.
CAGR (`(ending/beginning)^(1/years) − 1`) assumes a single lump sum with no interim cash flows —
exactly the assumption a portfolio with ongoing deposits violates, which is the whole reason XIRR
exists. Computing a second, naive CAGR alongside XIRR would just reintroduce the cash-flow-blind
problem this feature exists to fix, sitting right next to its own solution. Instead: compute XIRR
once, and present it to the user in CAGR's _framing_ — "the constant annual growth rate that would
explain everything you've put in, taken out, and currently hold" — since that's the intuitive
reading most people mean by "CAGR" anyway. One number, framed accessibly.

### Cash flows and sign convention

`Deposit.amount` is signed the way IBKR reports it: positive for a deposit (money added to the
account), negative for a withdrawal (`app/parser/sections/cash.py`) — the same convention
`Statement.deposits_withdrawals` uses. XIRR wants the investor's-eye-view sign, which is the
opposite for contributions: money leaving the investor's pocket into the portfolio is a cash
_outflow_ (negative), money taken back out is an _inflow_ (positive). So per deposit row:
`cash_flow = -amount`. Add one terminal cash flow: `(as_of_date, +nav_current)` — the latest
statement's ending NAV, treated as if the whole portfolio were liquidated on that date.

Solve `Σ cash_flow_i / (1 + rate)^((date_i − date_0)/365) = 0` for `rate`. No numerical library is
in this app's dependencies today (`pyproject.toml` — no numpy/scipy); a small Newton-Raphson
solver with a bisection fallback (standard for XIRR, handles cases where Newton doesn't converge
cleanly) is well within reach without adding one. Guard the degenerate cases explicitly rather
than letting the solver throw: fewer than two distinct cash-flow dates, or zero deposits, means
"not enough history yet" — return null/absent, not a fabricated number.

### Broker scope

**v1 computes XIRR for IBKR only.** Mixing real dated IBKR deposits with moomoo's annual
trade-proceeds _approximation_ (see "What the existing data does," above) into one cash-flow list
would produce a number that looks exactly as precise as a real XIRR while being partly guessed —
worse than not showing one. Concretely: the endpoint takes a `broker` filter (default `ibkr`), and
the frontend simply doesn't render the XIRR card when the active data has no real dated deposits
behind it, rather than rendering a number with an asterisk nobody will read. If moomoo ever gains
a real deposit export, extending this to a combined figure is a natural v2 — the trade-date
approximation could plausibly stand in per-transaction (not just per-year) at that point, but
that's a deliberate future call, not a default to reach for now.

### Currency scope

Same discipline as `overview.py`/`holdings.py`'s existing mixed-currency 409s: if the deposits
feeding a given XIRR run span more than one currency (an IBKR account can hold both SGD and USD
cash movements even under one `base_currency`), reject with 409 rather than summing raw numbers
across currencies. This is a real constraint on this account already, not a hypothetical — it's
exactly the gap `f07275d` closed for NAV and position totals; XIRR shouldn't reopen it.

### Output shape

`GET /api/returns/xirr?broker=ibkr` returns a trailing series, one entry per year the account has
a statement for — for year `Y`, restrict cash flows to `settle_date <= period_end(Y)` and use
that year's `nav_current` as the terminal value. This gives "money-weighted return since
inception, as of each year-end" — a line chart that pairs naturally with the existing
`TwrByYearChart` bars, and its latest point is the headline "Since-Inception XIRR" figure.

That headline figure belongs as a new card in `AllTimeSummaryCards`
(`apps/ledger-lens/src/components/overview/AllTimeSummaryCards.tsx`), alongside the existing
"Cumulative TWR" card it's meant to be read against — same `data-val=""` privacy-mode tagging,
same `pnlColor` treatment every other signed figure on that row already uses.

### Non-goals (v1)

- **Moomoo / combined-broker XIRR.** See "Broker scope" above.
- **Sub-annual (quarterly/monthly) XIRR.** The trailing-per-year series is the granularity this
  data naturally supports; a finer cadence would need finer-grained NAV snapshots this app
  doesn't have.
- **A distinct CAGR metric.** See "One metric, not two," above — deliberate, not an oversight.

## Open decisions to confirm before build

- **Benchmark data source** — recommended: user-uploaded CSV (Feature 1, above). Confirm before
  building the ingestion endpoint, since switching to a live-fetch design later changes the
  storage/caching shape non-trivially.
- **XIRR solver tolerance/iteration bounds** — implementation detail, but worth a deliberate
  choice (e.g., converge to 1e-6, cap at ~100 iterations before falling back to bisection) rather
  than an arbitrary default, since a portfolio with few, widely-spaced cash flows is exactly the
  case where naive Newton-Raphson misbehaves.
