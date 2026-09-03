"""Endpoint-level tests for /api/benchmarks, /api/timeseries/benchmark, /api/returns/xirr."""

from __future__ import annotations

import io
from datetime import date

import pytest
from app.models.db import Deposit, Statement
from sqlmodel import Session


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


def test_benchmark_upload_and_list(client):
    csv_bytes = b"Date,Close\n2024-01-02,100\n2024-12-31,110\n"
    resp = client.post(
        "/api/benchmarks/upload?symbol=SPY",
        files={"file": ("spy.csv", io.BytesIO(csv_bytes), "text/csv")},
    )
    assert resp.status_code == 200, resp.text
    body = resp.json()
    assert body["symbol"] == "SPY"
    assert body["ingested"] == 2

    listed = client.get("/api/benchmarks").json()
    assert listed == [
        {
            "symbol": "SPY",
            "price_count": 2,
            "first_date": "2024-01-02",
            "last_date": "2024-12-31",
        }
    ]


def test_timeseries_benchmark_matches_portfolio_twr_against_index(client, monkeypatch):
    import app.database

    with Session(app.database.engine) as session:
        _seed_ibkr_statement(session, year=2024, nav_current=11000.0, twr_pct=10.0)

    csv_bytes = b"Date,Close\n2024-01-01,100\n2024-12-31,120\n"
    up = client.post(
        "/api/benchmarks/upload?symbol=SPY",
        files={"file": ("spy.csv", io.BytesIO(csv_bytes), "text/csv")},
    )
    assert up.status_code == 200, up.text

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
