# syntax=docker/dockerfile:1
#
# Build context is the repo root:
#   docker build -f apps/table-talks/Dockerfile .

# -----------------------------------------------------------------------------
# Stage 1: builder — install deps and app with uv; venv stays in /app/.venv
# -----------------------------------------------------------------------------
FROM python:3.13-slim AS builder

WORKDIR /app

RUN pip install --no-cache-dir --upgrade pip \
    && pip install --no-cache-dir uv

COPY --link apps/table-talks/pyproject.toml apps/table-talks/uv.lock ./
RUN uv sync --frozen --no-dev --no-install-project

COPY --link apps/table-talks/src ./src/
COPY --link apps/table-talks/data ./data/
COPY --link apps/table-talks/.env.example ./
RUN uv sync --frozen --no-dev

# -----------------------------------------------------------------------------
# Stage 2: runtime — copy only venv + app; no uv, no build tools
# -----------------------------------------------------------------------------
FROM python:3.13-slim

WORKDIR /app

# Copy venv and app from builder (same base so venv paths are valid)
COPY --from=builder --link /app/.venv /app/.venv
COPY --from=builder --link /app/src /app/src
COPY --from=builder --link /app/data /app/data
COPY --from=builder --link /app/.env.example /app/.env.example

# Copy version manifest and changelog for the bot's in-chat info display
COPY --link apps/table-talks/package.json apps/table-talks/CHANGELOG.md /app/

RUN adduser --disabled-password --gecos "" appuser \
    && mkdir -p /app/secrets \
    && chown -R appuser:appuser /app

USER appuser

ENV PYTHONUNBUFFERED=1
ENV HEALTH_PORT=9999
ENV PATH="/app/.venv/bin:$PATH"
ENV PYTHONPATH="/app"

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:9999/health')" || exit 1

CMD ["python", "-m", "table_talks.index"]
