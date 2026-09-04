"""Endpoint-level tests for /api/benchmarks, /api/timeseries/benchmark, /api/returns/xirr."""

from __future__ import annotations

from datetime import date

import pytest
from app.models.db import Deposit, Statement
from sqlmodel import Session

_CHART_PAYLOAD = {
    "chart": {
        "result": [
            {
                "timestamp": [1704110400, 1735632000],  # 2024-01-01, 2024-12-31 (UTC)
                "indicators": {"quote": [{"close": [100.0, 120.0]}]},
            }
        ]
    }
}


def _seed_ibkr_statement(session: Session, *, year: int, nav_current: float, twr_pct: float):
    stmt = Statement(
        account_id="U1",
        account_name="Test",
        year=year,
        period="Annual",
        period_start=date(year, 1, 1),
        period_end=date(year, 12, 31),
        broker="ibkr",
        base_currency="USD",
        twr_pct=twr_pct,
        nav_current=nav_current,
    )
    session.add(stmt)
    session.commit()
    session.refresh(stmt)
    return stmt


def test_list_benchmarks_returns_catalog(client):
    listed = client.get("/api/benchmarks").json()
    symbols = {b["symbol"] for b in listed}
    assert "SPY" in symbols
    assert all("label" in b for b in listed)


def test_timeseries_benchmark_matches_portfolio_twr_against_index(client, monkeypatch):
    import app.database
    import app.services.benchmarks as benchmarks_service

    monkeypatch.setattr(
        benchmarks_service, "_fetch_chart_json", lambda yahoo_symbol: _CHART_PAYLOAD
    )

    with Session(app.database.engine) as session:
        _seed_ibkr_statement(session, year=2024, nav_current=11000.0, twr_pct=10.0)

    resp = client.get("/api/timeseries/benchmark", params={"symbol": "SPY"})
    assert resp.status_code == 200, resp.text
    rows = resp.json()
    assert len(rows) == 1
    row = rows[0]
    assert row["coverage"] == "full"
    assert row["twr_pct"] == 10.0
    assert row["portfolio_cum_index"] == pytest.approx(110.0)
    assert row["benchmark_return_pct"] == pytest.approx(20.0)
    assert row["benchmark_cum_index"] == pytest.approx(120.0)


def test_timeseries_benchmark_second_call_does_not_refetch(client, monkeypatch):
    import app.database
    import app.services.benchmarks as benchmarks_service

    call_count = 0

    def _fake_fetch(yahoo_symbol: str) -> dict:
        nonlocal call_count
        call_count += 1
        return _CHART_PAYLOAD

    monkeypatch.setattr(benchmarks_service, "_fetch_chart_json", _fake_fetch)

    with Session(app.database.engine) as session:
        _seed_ibkr_statement(session, year=2024, nav_current=11000.0, twr_pct=10.0)

    first = client.get("/api/timeseries/benchmark", params={"symbol": "SPY"})
    second = client.get("/api/timeseries/benchmark", params={"symbol": "SPY"})
    assert first.status_code == 200, first.text
    assert second.status_code == 200, second.text
    assert call_count == 1


def test_xirr_endpoint_computes_a_rate(client):
    import app.database

    with Session(app.database.engine) as session:
        stmt = _seed_ibkr_statement(session, year=2024, nav_current=1100.0, twr_pct=10.0)
        session.add(
            Deposit(
                statement_id=stmt.id,  # type: ignore
                year=2024,
                settle_date=date(2024, 1, 1),
                currency="USD",
                amount=1000.0,
            )
        )
        session.commit()

    resp = client.get("/api/returns/xirr", params={"broker": "ibkr"})
    assert resp.status_code == 200, resp.text
    rows = resp.json()
    assert len(rows) == 1
    assert rows[0]["xirr_pct"] == pytest.approx(10.0, abs=0.5)
    assert rows[0]["cash_flow_count"] == 1
