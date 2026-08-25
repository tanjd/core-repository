# syntax=docker/dockerfile:1
#
# Build context is the repo root:
#   docker build -f apps/index-watch/Dockerfile .
#
# The image mirrors the repo's apps/<name> + libs/<lib> layout under /app (rather
# than flattening apps/index-watch straight to /app) so that index-watch's
# pyproject.toml `[tool.uv.sources]` path-dependency on ../../libs/telegram-bot-shared
# resolves identically here and on the host — see libs/telegram-bot-shared/README.md.

# -----------------------------------------------------------------------------
# Stage 1: builder — install deps and app with uv; venv stays in .venv
# -----------------------------------------------------------------------------
FROM python:3.14-slim AS builder

WORKDIR /app

RUN pip install --no-cache-dir --upgrade pip \
    && pip install --no-cache-dir uv

COPY --link libs/telegram-bot-shared/pyproject.toml libs/telegram-bot-shared/README.md /app/libs/telegram-bot-shared/
COPY --link libs/telegram-bot-shared/src /app/libs/telegram-bot-shared/src/

COPY --link apps/index-watch/pyproject.toml apps/index-watch/uv.lock /app/apps/index-watch/
WORKDIR /app/apps/index-watch
RUN uv sync --frozen --no-dev --no-install-project

COPY --link apps/index-watch/src /app/apps/index-watch/src/
COPY --link apps/index-watch/.env.example /app/apps/index-watch/
RUN uv sync --frozen --no-dev

# -----------------------------------------------------------------------------
# Stage 2: runtime — copy only venv + app; no uv, no build tools
# -----------------------------------------------------------------------------
FROM python:3.14-slim

WORKDIR /app/apps/index-watch

# Copy venv and app from builder (same layout so venv + editable-install paths are valid)
COPY --from=builder --link /app/apps/index-watch/.venv /app/apps/index-watch/.venv
COPY --from=builder --link /app/apps/index-watch/src /app/apps/index-watch/src
COPY --from=builder --link /app/apps/index-watch/.env.example /app/apps/index-watch/.env.example
COPY --from=builder --link /app/libs/telegram-bot-shared/src /app/libs/telegram-bot-shared/src

RUN adduser --disabled-password --gecos "" appuser \
    && mkdir -p /app/apps/index-watch/data \
    && chown -R appuser:appuser /app

USER appuser

ENV PYTHONUNBUFFERED=1
ENV PATH="/app/apps/index-watch/.venv/bin:$PATH"
ENV PYTHONPATH="/app/apps/index-watch"

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8080/health')" || exit 1

# Declare data directory as volume for SQLite persistence
VOLUME ["/app/apps/index-watch/data"]

CMD ["python", "-m", "index_watch"]
