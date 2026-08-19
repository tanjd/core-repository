## 1.7.3 (2026-08-19)

This was a version bump only for ledger-lens to align it with other projects, there were no code changes.

## 1.7.2 (2026-08-19)

This was a version bump only for ledger-lens to align it with other projects, there were no code changes.

## 1.7.1 (2026-08-17)

### 🩹 Fixes

- **docker:** serve standalone Next.js static assets from the correct path ([#34](https://github.com/tanjd/core-repository/pull/34))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 1.7.0 (2026-08-16)

### 🚀 Features

- migrate bookshelf into the monorepo ([#30](https://github.com/tanjd/core-repository/pull/30))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 1.6.0 (2026-07-31)

### 🚀 Features

- migrate ledger-lens into the monorepo ([#25](https://github.com/tanjd/core-repository/pull/25))

### ❤️ Thank You

- Jeddy Tan @tanjd

# CHANGELOG

<!-- carried over from the standalone tanjd/ledger-lens repo (pre-migration history, python-semantic-release format) -->

## v1.5.0 (2026-03-12)

### Bug Fixes

- **backend**: Skip already-ingested files on watcher startup
  ([#5](https://github.com/tanjd/ledger-lens/pull/5),
  [`066f339`](https://github.com/tanjd/ledger-lens/commit/066f3397c0a4734bb6943361aa74f7c3de4ad717))

- **frontend**: Add data-val attribute to summary card subtitle
  ([#5](https://github.com/tanjd/ledger-lens/pull/5),
  [`066f339`](https://github.com/tanjd/ledger-lens/commit/066f3397c0a4734bb6943361aa74f7c3de4ad717))

- **frontend**: Center dialog with flex wrapper ([#5](https://github.com/tanjd/ledger-lens/pull/5),
  [`066f339`](https://github.com/tanjd/ledger-lens/commit/066f3397c0a4734bb6943361aa74f7c3de4ad717))

### Chores

- **backend**: Bump package version to 1.4.0 ([#5](https://github.com/tanjd/ledger-lens/pull/5),
  [`066f339`](https://github.com/tanjd/ledger-lens/commit/066f3397c0a4734bb6943361aa74f7c3de4ad717))

### Continuous Integration

- Bump setup-buildx-action from v3 to v4 ([#5](https://github.com/tanjd/ledger-lens/pull/5),
  [`066f339`](https://github.com/tanjd/ledger-lens/commit/066f3397c0a4734bb6943361aa74f7c3de4ad717))

### Features

- **frontend**: Add broker branding to topbar and sidebar nav
  ([#5](https://github.com/tanjd/ledger-lens/pull/5),
  [`066f339`](https://github.com/tanjd/ledger-lens/commit/066f3397c0a4734bb6943361aa74f7c3de4ad717))

- **frontend**: Add IBKR header and SGD conversion to Moomoo overview
  ([#5](https://github.com/tanjd/ledger-lens/pull/5),
  [`066f339`](https://github.com/tanjd/ledger-lens/commit/066f3397c0a4734bb6943361aa74f7c3de4ad717))

## v1.4.0 (2026-03-11)

### Chores

- **ci**: Bump docker/build-push-action to v7 ([#4](https://github.com/tanjd/ledger-lens/pull/4),
  [`e1cf9d3`](https://github.com/tanjd/ledger-lens/commit/e1cf9d36d27b5be5eb0311d7b74c5bea38874ddb))

- **ci**: Move .python-version into backend/ ([#3](https://github.com/tanjd/ledger-lens/pull/3),
  [`5537dc1`](https://github.com/tanjd/ledger-lens/commit/5537dc10a08dc74b161deb5e268607720906629e))

- **ci**: Read node version from package.json engines field
  ([#3](https://github.com/tanjd/ledger-lens/pull/3),
  [`5537dc1`](https://github.com/tanjd/ledger-lens/commit/5537dc10a08dc74b161deb5e268607720906629e))

- **deps**: Sync uv.lock version to 1.3.0 ([#3](https://github.com/tanjd/ledger-lens/pull/3),
  [`5537dc1`](https://github.com/tanjd/ledger-lens/commit/5537dc10a08dc74b161deb5e268607720906629e))

- **devcontainer**: Add basedpyright extraPaths for backend
  ([#4](https://github.com/tanjd/ledger-lens/pull/4),
  [`e1cf9d3`](https://github.com/tanjd/ledger-lens/commit/e1cf9d36d27b5be5eb0311d7b74c5bea38874ddb))

### Continuous Integration

- Harden release pipeline and pin runtime versions
  ([#3](https://github.com/tanjd/ledger-lens/pull/3),
  [`5537dc1`](https://github.com/tanjd/ledger-lens/commit/5537dc10a08dc74b161deb5e268607720906629e))

- Verify Docker builds before semantic release ([#3](https://github.com/tanjd/ledger-lens/pull/3),
  [`5537dc1`](https://github.com/tanjd/ledger-lens/commit/5537dc10a08dc74b161deb5e268607720906629e))

### Features

- Mobile-friendly nav drawer and privacy chart blur fix
  ([#4](https://github.com/tanjd/ledger-lens/pull/4),
  [`e1cf9d3`](https://github.com/tanjd/ledger-lens/commit/e1cf9d36d27b5be5eb0311d7b74c5bea38874ddb))

- **frontend**: Add mobile nav drawer and fix privacy chart blur
  ([#4](https://github.com/tanjd/ledger-lens/pull/4),
  [`e1cf9d3`](https://github.com/tanjd/ledger-lens/commit/e1cf9d36d27b5be5eb0311d7b74c5bea38874ddb))

## v1.3.0 (2026-03-07)

### Features

- Add upload history log and ingest summary ([#2](https://github.com/tanjd/ledger-lens/pull/2),
  [`3c1a3af`](https://github.com/tanjd/ledger-lens/commit/3c1a3af2ed036c393c28a1d9bbe51a0c50edfbd8))

## v1.2.0 (2026-03-07)

### Chores

- Sync frontend version to 1.1.0 [skip ci]
  ([`cb8f177`](https://github.com/tanjd/ledger-lens/commit/cb8f177ded5ce8a37ff33e7e021089f6cf34ea52))

### Continuous Integration

- Use version_variables to sync frontend package.json on release
  ([`54b32c9`](https://github.com/tanjd/ledger-lens/commit/54b32c9fa13e242aad0de71de7030d8e83e19809))

### Features

- Add multi-broker support with Moomoo (Futu SG) integration
  ([`bf984a0`](https://github.com/tanjd/ledger-lens/commit/bf984a05f431bff920e5fd2388bd871457628b52))

## v1.1.0 (2026-03-05)

### Documentation

- Add pre-commit and devcontainer badges and setup instructions
  ([`3693b19`](https://github.com/tanjd/ledger-lens/commit/3693b19c470d6df5bdf92955e8b504ecd9a60594))

- Note version display in sidebar
  ([`e2bf2dc`](https://github.com/tanjd/ledger-lens/commit/e2bf2dc77f9d482b34bbfc5bb03277b6513b9a0f))

### Features

- Display frontend and backend versions in sidebar
  ([`2307b0a`](https://github.com/tanjd/ledger-lens/commit/2307b0a80a3dae9ad9e2431d3fe1d427585f8d39))

- **frontend**: Support multi-file import with per-file preview and status
  ([`241f753`](https://github.com/tanjd/ledger-lens/commit/241f753001aedda92da93af4f385d1bd37b48feb))

## v1.0.2 (2026-03-04)

### Bug Fixes

- Replace Next.js rewrites with runtime proxy route handler and fix CORS env parsing
  ([`d329ccd`](https://github.com/tanjd/ledger-lens/commit/d329ccddfb8f4013dd017708d2e7a7731bf8a014))

## v1.0.1 (2026-03-04)

### Bug Fixes

- **frontend**: Remove --link from runtime COPY steps to fix chown appuser in multi-platform build
  ([`7b90d4f`](https://github.com/tanjd/ledger-lens/commit/7b90d4f3418cb6d64cec89b1ac2ff914d7479f0f))

## v1.0.0 (2026-03-04)

- Initial Release

<!-- nx release entries continue below this line -->
