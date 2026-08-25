#!/usr/bin/env bash
# Devcontainer postCreateCommand entry point.
#
# CI-vs-local routing:
# - GitHub Actions runners set CI=true in the runner env.
# - devcontainers/ci@v0.3's own `inheritEnv` input defaults to `false`, so that
#   runner-side CI is NOT auto-forwarded into the container.
# - Bridged explicitly in devcontainer.json via:
#     "remoteEnv": { "CI": "${localEnv:CI}" }
#   which propagates the host's CI into the container's postCreate environment.
#   Local IDE builds have CI unset on the host, so it stays unset inside too.
#
# Why: keep a transient GitHub release-assets 5xx on the rtk installer from
# taking down CI (see the 2026-08-12 outage). Set FORCE_DEV_TOOLS=1 to run the
# full setup anyway (e.g. when debugging the CI image locally).
set -euo pipefail

cd "$(dirname "$0")/.."

if [ -n "${CI:-}" ] && [ -z "${FORCE_DEV_TOOLS:-}" ]; then
  echo "post-create.sh: CI detected, running 'make setup-ci' (skipping dev-only tools)"
  exec make setup-ci
fi

echo "post-create.sh: running full 'make setup'"
exec make setup
