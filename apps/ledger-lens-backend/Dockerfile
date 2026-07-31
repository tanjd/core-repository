# -----------------------------------------------------------------------------
# Stage 1: builder — install deps with uv; venv stays in /app/.venv
# -----------------------------------------------------------------------------
FROM python:3.14-slim AS builder

WORKDIR /app

RUN pip install --no-cache-dir --upgrade pip \
    && pip install --no-cache-dir uv

COPY --link apps/ledger-lens-backend/pyproject.toml apps/ledger-lens-backend/uv.lock ./
RUN uv sync --frozen --no-dev --no-install-project

COPY --link apps/ledger-lens-backend/app ./app/
RUN uv sync --frozen --no-dev

# -----------------------------------------------------------------------------
# Stage 2: runtime — copy only venv + app; no uv, no build tools
# -----------------------------------------------------------------------------
FROM python:3.14-slim

WORKDIR /app

COPY --from=builder --link /app/.venv /app/.venv
COPY --from=builder --link /app/app /app/app
COPY --from=builder --link /app/pyproject.toml /app/pyproject.toml
COPY --link apps/ledger-lens-backend/package.json /app/package.json

RUN adduser --disabled-password --gecos "" appuser \
    && mkdir -p /app/data \
    && chown -R appuser:appuser /app

USER appuser

ENV PYTHONUNBUFFERED=1
ENV PYTHONPATH="/app"
ENV PATH="/app/.venv/bin:$PATH"
ENV LEDGER_DATA_DIR="/app/data"

EXPOSE 8000

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8000/health')" || exit 1

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
