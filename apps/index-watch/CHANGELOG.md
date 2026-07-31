## 1.1.0 (2026-07-31)

This was a version bump only for index-watch to align it with other projects, there were no code changes.

# 1.0.0 (2026-07-30)

### 🚀 Features

- ⚠️ **index-watch:** add per-subscriber alerts, digests, and recovery notices ([#21](https://github.com/tanjd/core-repository/pull/21))

### ⚠️ Breaking Changes

- **index-watch:** add per-subscriber alerts, digests, and recovery notices ([#21](https://github.com/tanjd/core-repository/pull/21))
  alert_state's schema is now keyed by chat_id (was
  symbol+threshold only), and AlertState's methods gained a leading chat_id
  parameter. The schema migration runs automatically in init_db() on next
  startup; it may cause one duplicate alert per already-triggered threshold
  as a one-time side effect (no manual DB step required).
  Claude-Session: https://claude.ai/code/session_015MuauJJ1vrva2EdABJN1Q9

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.2.0 (2026-07-30)

### 🚀 Features

- **index-watch:** add MSCI World index and configurable history years ([#20](https://github.com/tanjd/core-repository/pull/20))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.1.1 (2026-07-30)

This was a version bump only for index-watch to align it with other projects, there were no code changes.

# CHANGELOG

<!-- carried over from the standalone tanjd/index-watch repo (pre-migration history, python-semantic-release format) -->

## v1.3.0 (2026-03-04)

### Features

- **bot**: Add admin-only /clearcache command ([#2](https://github.com/tanjd/index-watch/pull/2),
  [`4fe3c87`](https://github.com/tanjd/index-watch/commit/4fe3c8702391819850fd5f7e0bee7ecc38daaa34))

## v1.2.0 (2026-02-19)

### Features

- Add rate limiting, enhanced caching, and graceful degradation
  ([`20f2602`](https://github.com/tanjd/index-watch/commit/20f26022baf3d8a90f3f3af51df398968ae67a69))

## v1.1.0 (2026-02-17)

### Features

- Add database-backed subscriber management
  ([`7af564f`](https://github.com/tanjd/index-watch/commit/7af564f9ef22e8a6871762e423b5eed7fcfb1031))

## v1.0.1 (2026-02-16)

### Bug Fixes

- **docker**: Remove data copy, fix entrypoint and drop healthcheck
  ([`0107307`](https://github.com/tanjd/index-watch/commit/0107307994fbf1ea622c8b72499aba03b4190e6e))

## v1.0.0 (2026-02-16)

- Initial Release

<!-- nx release entries continue below this line -->
