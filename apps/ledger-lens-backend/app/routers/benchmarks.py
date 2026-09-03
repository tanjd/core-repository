"""Upload + list routes for benchmark index price history.

POST /api/benchmarks/upload — parse a (date, close) CSV for a symbol, upsert into the DB
GET  /api/benchmarks        — list ingested symbols with coverage
"""

from __future__ import annotations

import csv
import io
from datetime import date

from fastapi import APIRouter, Depends, File, HTTPException, Query, UploadFile
from sqlmodel import Session, col, func, select

from app.database import get_session
from app.models.api import BenchmarkInfo, BenchmarkUploadResponse
from app.models.db import BenchmarkPrice
from app.parser.normalizers import parse_date, parse_float

router = APIRouter()

# Accept either the common Yahoo Finance/Stooq export header names or a plain
# "first two columns are date, close" file with any header.
_DATE_KEYS = ("Date", "date", "DATE")
_CLOSE_KEYS = ("Close", "close", "CLOSE", "Adj Close", "Price", "price")


def _parse_benchmark_csv(content: bytes) -> list[tuple[date, float]]:
    text = content.decode("utf-8-sig")
    reader = csv.DictReader(io.StringIO(text))
    if not reader.fieldnames or len(reader.fieldnames) < 2:
        raise ValueError("CSV must have at least two columns (date, close)")

    date_key = next((k for k in _DATE_KEYS if k in reader.fieldnames), reader.fieldnames[0])
    close_key = next((k for k in _CLOSE_KEYS if k in reader.fieldnames), reader.fieldnames[1])

    rows: list[tuple[date, float]] = []
    for row in reader:
        raw_date = (row.get(date_key) or "").strip()
        raw_close = (row.get(close_key) or "").strip()
        if not raw_date or not raw_close:
            continue
        price_date = parse_date(raw_date)
        close = parse_float(raw_close)
        if price_date is None or close is None:
            continue
        rows.append((price_date, close))
    return rows


@router.post("/benchmarks/upload", response_model=BenchmarkUploadResponse)
async def upload_benchmark(
    symbol: str = Query(..., min_length=1, max_length=32),
    file: UploadFile = File(...),
    session: Session = Depends(get_session),
) -> BenchmarkUploadResponse:
    content = await file.read()
    try:
        rows = _parse_benchmark_csv(content)
    except Exception as exc:
        raise HTTPException(status_code=422, detail=f"Failed to parse CSV: {exc}") from exc

    if not rows:
        raise HTTPException(status_code=422, detail="No valid (date, close) rows found in CSV")

    symbol = symbol.strip().upper()
    for price_date, close in rows:
        existing = session.exec(
            select(BenchmarkPrice).where(
                BenchmarkPrice.symbol == symbol,
                BenchmarkPrice.price_date == price_date,
            )
        ).first()
        if existing:
            existing.close = close
            session.add(existing)
        else:
            session.add(BenchmarkPrice(symbol=symbol, price_date=price_date, close=close))
    session.commit()

    dates = sorted(d for d, _ in rows)
    return BenchmarkUploadResponse(
        symbol=symbol,
        ingested=len(rows),
        first_date=dates[0],
        last_date=dates[-1],
    )


@router.get("/benchmarks", response_model=list[BenchmarkInfo])
def list_benchmarks(session: Session = Depends(get_session)) -> list[BenchmarkInfo]:
    symbols = session.exec(select(BenchmarkPrice.symbol).distinct()).all()
    result = []
    for symbol in sorted(symbols):
        count = session.exec(
            select(func.count(col(BenchmarkPrice.id))).where(BenchmarkPrice.symbol == symbol)
        ).one()
        first_date = session.exec(
            select(func.min(col(BenchmarkPrice.price_date))).where(BenchmarkPrice.symbol == symbol)
        ).one()
        last_date = session.exec(
            select(func.max(col(BenchmarkPrice.price_date))).where(BenchmarkPrice.symbol == symbol)
        ).one()
        result.append(
            BenchmarkInfo(
                symbol=symbol, price_count=count, first_date=first_date, last_date=last_date
            )
        )
    return result
