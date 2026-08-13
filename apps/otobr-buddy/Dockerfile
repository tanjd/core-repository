# syntax=docker/dockerfile:1
#
# Build context is the repo root:
#   docker build -f apps/otobr-buddy/Dockerfile .
#
# The image mirrors the repo's apps/<name> + libs/<lib> layout under /app (rather
# than flattening apps/otobr-buddy straight to /app) so that otobr-buddy's
# pyproject.toml `[tool.uv.sources]` path-dependency on ../../libs/telegram-bot-shared
# resolves identically here and on the host — see libs/telegram-bot-shared/README.md.

# -----------------------------------------------------------------------------
# Stage 1: builder — install deps and app with uv; venv stays in .venv
# -----------------------------------------------------------------------------
FROM python:3.13-slim AS builder

WORKDIR /app

RUN pip install --no-cache-dir --upgrade pip \
    && pip install --no-cache-dir uv

COPY --link libs/telegram-bot-shared/pyproject.toml libs/telegram-bot-shared/README.md /app/libs/telegram-bot-shared/
COPY --link libs/telegram-bot-shared/src /app/libs/telegram-bot-shared/src/

COPY --link apps/otobr-buddy/pyproject.toml apps/otobr-buddy/uv.lock /app/apps/otobr-buddy/
WORKDIR /app/apps/otobr-buddy
RUN uv sync --frozen --no-dev --no-install-project

COPY --link apps/otobr-buddy/src /app/apps/otobr-buddy/src/
COPY --link apps/otobr-buddy/data /app/apps/otobr-buddy/data/
COPY --link apps/otobr-buddy/.env.example /app/apps/otobr-buddy/
RUN uv sync --frozen --no-dev

# -----------------------------------------------------------------------------
# Stage 2: runtime — copy only venv + app; no uv, no build tools
# -----------------------------------------------------------------------------
FROM python:3.13-slim

WORKDIR /app/apps/otobr-buddy

# Copy venv and app from builder (same layout so venv + editable-install paths are valid)
COPY --from=builder --link /app/apps/otobr-buddy/.venv /app/apps/otobr-buddy/.venv
COPY --from=builder --link /app/apps/otobr-buddy/src /app/apps/otobr-buddy/src
COPY --from=builder --link /app/apps/otobr-buddy/data /app/apps/otobr-buddy/data
COPY --from=builder --link /app/apps/otobr-buddy/.env.example /app/apps/otobr-buddy/.env.example
COPY --from=builder --link /app/libs/telegram-bot-shared/src /app/libs/telegram-bot-shared/src

RUN adduser --disabled-password --gecos "" appuser \
    && chown -R appuser:appuser /app

USER appuser

ENV PYTHONUNBUFFERED=1
ENV HEALTH_PORT=9999
ENV PATH="/app/apps/otobr-buddy/.venv/bin:$PATH"
ENV PYTHONPATH="/app/apps/otobr-buddy"

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:9999/health')" || exit 1

# Declare data directory as volume for SQLite persistence
VOLUME ["/app/apps/otobr-buddy/data"]

CMD ["python", "-m", "otobr_buddy"]
