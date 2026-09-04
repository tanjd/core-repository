"""GET /api/benchmarks — list the benchmark indices available for comparison.

Prices themselves aren't served here — they're fetched/cached on demand by
`app.services.benchmarks.ensure_benchmark_prices`, called from the
`GET /api/timeseries/benchmark` handler once a symbol + date range is known.
"""

from __future__ import annotations

from fastapi import APIRouter

from app.models.api import BenchmarkOption
from app.services.benchmarks import BENCHMARK_CATALOG

router = APIRouter()


@router.get("/benchmarks", response_model=list[BenchmarkOption])
def list_benchmarks() -> list[BenchmarkOption]:
    return [
        BenchmarkOption(symbol=symbol, label=label)
        for symbol, (_stooq_symbol, label) in sorted(BENCHMARK_CATALOG.items())
    ]
