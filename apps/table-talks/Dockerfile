# syntax=docker/dockerfile:1
#
# Build context is the repo root:
#   docker build -f apps/table-talks/Dockerfile .
#
# The image mirrors the repo's apps/<name> + libs/<lib> layout under /app (rather
# than flattening apps/table-talks straight to /app) so that table-talks'
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

COPY --link apps/table-talks/pyproject.toml apps/table-talks/uv.lock /app/apps/table-talks/
WORKDIR /app/apps/table-talks
RUN uv sync --frozen --no-dev --no-install-project

COPY --link apps/table-talks/src /app/apps/table-talks/src/
COPY --link apps/table-talks/data /app/apps/table-talks/data/
COPY --link apps/table-talks/.env.example /app/apps/table-talks/
RUN uv sync --frozen --no-dev

# -----------------------------------------------------------------------------
# Stage 2: runtime — copy only venv + app; no uv, no build tools
# -----------------------------------------------------------------------------
FROM python:3.13-slim

WORKDIR /app/apps/table-talks

# Copy venv and app from builder (same layout so venv + editable-install paths are valid)
COPY --from=builder --link /app/apps/table-talks/.venv /app/apps/table-talks/.venv
COPY --from=builder --link /app/apps/table-talks/src /app/apps/table-talks/src
COPY --from=builder --link /app/apps/table-talks/data /app/apps/table-talks/data
COPY --from=builder --link /app/apps/table-talks/.env.example /app/apps/table-talks/.env.example
COPY --from=builder --link /app/libs/telegram-bot-shared/src /app/libs/telegram-bot-shared/src

# Copy version manifest and changelog for the bot's in-chat info display
COPY --link apps/table-talks/package.json apps/table-talks/CHANGELOG.md /app/apps/table-talks/

RUN adduser --disabled-password --gecos "" appuser \
    && mkdir -p /app/apps/table-talks/secrets \
    && chown -R appuser:appuser /app

USER appuser

ENV PYTHONUNBUFFERED=1
ENV HEALTH_PORT=9999
ENV PATH="/app/apps/table-talks/.venv/bin:$PATH"
ENV PYTHONPATH="/app/apps/table-talks"

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:9999/health')" || exit 1

CMD ["python", "-m", "table_talks.index"]
