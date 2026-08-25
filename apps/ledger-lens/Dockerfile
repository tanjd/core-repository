# -----------------------------------------------------------------------------
# Stage 1: deps — full pnpm workspace install (ledger-lens' own package.json
# is a bare version manifest with no dependencies of its own — every real
# dependency lives in the root package.json, so there's no way to scope this
# install to just this app).
# -----------------------------------------------------------------------------
FROM node:24-slim AS deps

WORKDIR /app

# Pin pnpm to the same version the devcontainer/CI use (.devcontainer/devcontainer.json's
# features["ghcr.io/devcontainers/features/node:2.1.0"].pnpmVersion) — corepack
# defaults to latest otherwise, which can enforce newer supply-chain policies
# (e.g. minimum package release age) the lockfile wasn't resolved against.
# Hardcoded rather than read from devcontainer.json because .dockerignore
# excludes .devcontainer/ entirely; keep this in sync if that version bumps.
RUN corepack enable && corepack prepare pnpm@11.23.0 --activate

COPY --link package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY --link apps/ledger-lens/package.json ./apps/ledger-lens/
RUN pnpm install --frozen-lockfile

# -----------------------------------------------------------------------------
# Stage 2: builder — build via Nx (ledger-lens has no build script of its
# own; the build target lives in project.json, not package.json)
# -----------------------------------------------------------------------------
FROM node:24-slim AS builder

WORKDIR /app

RUN corepack enable && corepack prepare pnpm@11.23.0 --activate

# Nx computes the whole workspace's project graph before building any single
# project, which includes @nx-go/nx-go's dependency analysis for
# food-maps-backend (a Go project also present in this repo) — that plugin
# shells out to `go list -m -json`, so `go` must be on PATH here even though
# this image only ships the frontend. Borrowed from the same golang image
# food-maps-backend's own Dockerfile builds against, not apt-get'd, to avoid
# a heavier/differently-pinned Go toolchain.
COPY --from=golang:1.26-bookworm /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"

COPY --from=deps --link /app/node_modules ./node_modules
COPY --link . .

ENV NEXT_TELEMETRY_DISABLED=1

RUN pnpm exec nx build ledger-lens --configuration=production

# -----------------------------------------------------------------------------
# Stage 3: runtime — minimal image with standalone output
# -----------------------------------------------------------------------------
FROM node:24-slim

WORKDIR /app

RUN adduser --disabled-password --gecos "" appuser

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

# Next's standalone tracer preserves the workspace-relative path for a
# monorepo project — server.js lands at apps/ledger-lens/server.js, not at
# the standalone root, with node_modules hoisted to the root alongside it.
# server.js does process.chdir(__dirname) (apps/ledger-lens/) and then resolves
# the baked-in distDir ("./../../dist/apps/ledger-lens/.next") relative to that
# cwd — i.e. it reads static assets from dist/apps/ledger-lens/.next/static, not
# apps/ledger-lens/.next/static. public/ isn't affected: Next resolves it
# relative to `dir` (apps/ledger-lens/) directly, not through distDir.
COPY --from=builder --chown=appuser:appuser /app/dist/apps/ledger-lens/.next/standalone ./
COPY --from=builder --chown=appuser:appuser /app/dist/apps/ledger-lens/.next/static ./dist/apps/ledger-lens/.next/static
COPY --from=builder --chown=appuser:appuser /app/apps/ledger-lens/public ./apps/ledger-lens/public

USER appuser

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD node -e "require('http').get('http://localhost:3000/api/health', (r) => process.exit(r.statusCode === 200 ? 0 : 1))"

CMD ["node", "apps/ledger-lens/server.js"]
