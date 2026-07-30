# syntax=docker/dockerfile:1
#
# Build context is the repo root:
#   docker build -f apps/index-watch/Dockerfile .

# -----------------------------------------------------------------------------
# Stage 1: builder — install deps and app with uv; venv stays in /app/.venv
# -----------------------------------------------------------------------------
FROM python:3.13-slim AS builder

WORKDIR /app

RUN pip install --no-cache-dir --upgrade pip \
    && pip install --no-cache-dir uv

COPY --link apps/index-watch/pyproject.toml apps/index-watch/uv.lock ./
RUN uv sync --frozen --no-dev --no-install-project

COPY --link apps/index-watch/src ./src/
COPY --link apps/index-watch/.env.example ./
RUN uv sync --frozen --no-dev

# -----------------------------------------------------------------------------
# Stage 2: runtime — copy only venv + app; no uv, no build tools
# -----------------------------------------------------------------------------
FROM python:3.13-slim

WORKDIR /app

# Copy venv and app from builder (same base so venv paths are valid)
COPY --from=builder --link /app/.venv /app/.venv
COPY --from=builder --link /app/src /app/src
COPY --from=builder --link /app/.env.example /app/.env.example

RUN adduser --disabled-password --gecos "" appuser \
    && mkdir -p /app/data \
    && chown -R appuser:appuser /app

USER appuser

ENV PYTHONUNBUFFERED=1
ENV PATH="/app/.venv/bin:$PATH"
ENV PYTHONPATH="/app"

# Declare data directory as volume for SQLite persistence
VOLUME ["/app/data"]

CMD ["python", "-m", "index_watch"]
