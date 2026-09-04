# syntax=docker/dockerfile:1
#
# Build context is the repo root:
#   docker build -f apps/bookshelf-bot/Dockerfile .
#
# The image mirrors the repo's apps/<name> + libs/<lib> layout under /app (rather
# than flattening apps/bookshelf-bot straight to /app) so that this app's
# pyproject.toml `[tool.uv.sources]` path-dependency on ../../libs/telegram-bot-shared
# resolves identically here and on the host — see libs/telegram-bot-shared/README.md.

# -----------------------------------------------------------------------------
# Stage 1: builder - install deps and app with uv; venv stays in .venv
# -----------------------------------------------------------------------------
FROM python:3.14-slim AS builder

WORKDIR /app

RUN pip install --no-cache-dir --upgrade pip \
    && pip install --no-cache-dir uv

COPY --link libs/telegram-bot-shared/pyproject.toml libs/telegram-bot-shared/README.md /app/libs/telegram-bot-shared/
COPY --link libs/telegram-bot-shared/src /app/libs/telegram-bot-shared/src/

COPY --link apps/bookshelf-bot/pyproject.toml apps/bookshelf-bot/uv.lock /app/apps/bookshelf-bot/
WORKDIR /app/apps/bookshelf-bot
RUN uv sync --frozen --no-dev --no-install-project

COPY --link apps/bookshelf-bot/src /app/apps/bookshelf-bot/src/
COPY --link apps/bookshelf-bot/README.md /app/apps/bookshelf-bot/
RUN uv sync --frozen --no-dev

# -----------------------------------------------------------------------------
# Stage 2: runtime - copy only venv + app; no uv, no build tools
# -----------------------------------------------------------------------------
FROM python:3.14-slim

WORKDIR /app/apps/bookshelf-bot

COPY --from=builder --link /app/apps/bookshelf-bot/.venv /app/apps/bookshelf-bot/.venv
COPY --from=builder --link /app/apps/bookshelf-bot/src /app/apps/bookshelf-bot/src
COPY --from=builder --link /app/libs/telegram-bot-shared/src /app/libs/telegram-bot-shared/src

RUN adduser --disabled-password --gecos "" appuser \
    && chown -R appuser:appuser /app

USER appuser

ENV PYTHONUNBUFFERED=1
ENV PATH="/app/apps/bookshelf-bot/.venv/bin:$PATH"
ENV PYTHONPATH="/app/apps/bookshelf-bot"

EXPOSE 8080

CMD ["python", "-m", "bookshelf_bot"]
