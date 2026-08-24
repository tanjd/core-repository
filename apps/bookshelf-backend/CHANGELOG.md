## 0.14.3 (2026-08-24)

### 🩹 Fixes

- **bookshelf-backend:** stop cover-only hit from blocking description lookup ([#68](https://github.com/tanjd/core-repository/pull/68), [#66](https://github.com/tanjd/core-repository/issues/66))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.14.2 (2026-08-22)

### 🩹 Fixes

- **bookshelf,bookshelf-backend:** surface per-book results for cover-backfill job ([#66](https://github.com/tanjd/core-repository/pull/66))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.14.1 (2026-08-22)

### 🩹 Fixes

- **bookshelf,bookshelf-backend:** backfill book covers/descriptions and add My Books sort/filter ([#65](https://github.com/tanjd/core-repository/pull/65))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.14.0 (2026-08-22)

### 🚀 Features

- **bookshelf,bookshelf-backend:** cross-edition description backfill ([#64](https://github.com/tanjd/core-repository/pull/64))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.13.0 (2026-08-21)

### 🚀 Features

- **bookshelf,bookshelf-backend,bookshelf-e2e:** magic-link registration ([#63](https://github.com/tanjd/core-repository/pull/63))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.12.0 (2026-08-21)

### 🚀 Features

- **bookshelf,bookshelf-backend,bookshelf-e2e:** add fuzzy title/author match to book import ([#61](https://github.com/tanjd/core-repository/pull/61))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.11.1 (2026-08-21)

### 🚀 Features

- **bookshelf,bookshelf-e2e:** redesign mobile nav, cover-load fallback ([#60](https://github.com/tanjd/core-repository/pull/60))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.11.0 (2026-08-20)

### 🚀 Features

- **bookshelf,bookshelf-backend:** catalog UX polish + My Books search ([#59](https://github.com/tanjd/core-repository/pull/59))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.10.0 (2026-08-20)

### 🚀 Features

- **bookshelf,bookshelf-backend,bookshelf-e2e:** export and import books ([#58](https://github.com/tanjd/core-repository/pull/58))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.9.0 (2026-08-20)

### 🚀 Features

- **bookshelf,bookshelf-backend,bookshelf-e2e:** require phone at registration when community demands it ([#57](https://github.com/tanjd/core-repository/pull/57))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.8.0 (2026-08-20)

### 🚀 Features

- **bookshelf-backend:** delete orphaned keyless books on last-copy removal ([#56](https://github.com/tanjd/core-repository/pull/56))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.7.0 (2026-08-20)

### 🚀 Features

- **bookshelf,bookshelf-backend,bookshelf-e2e:** magic-link auth, book cover fallback ([#55](https://github.com/tanjd/core-repository/pull/55))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.6.1 (2026-08-20)

### 🚀 Features

- **bookshelf,bookshelf-e2e:** landing page for logged-out visitors, e2e against real servers ([#54](https://github.com/tanjd/core-repository/pull/54))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.6.0 (2026-08-19)

### 🚀 Features

- **bookshelf,bookshelf-backend:** ISBN barcode scanning to sharing flow ([#53](https://github.com/tanjd/core-repository/pull/53))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.5.0 (2026-08-19)

### 🚀 Features

- **bookshelf:** add password reset flow
- **bookshelf:** add contact preferences and loan return tracking
- **bookshelf:** add automatic backups and refresh admin console

Manually versioned: PR [#50](https://github.com/tanjd/core-repository/pull/50) squash-merged with
the non-conventional title "Add password reset, contact prefs, loan-return tracking, and backups",
so nx release's conventional-commits detection silently skipped this project despite the PR
touching `apps/bookshelf-backend` (password-reset endpoints, backup service/handlers, contact-pref
fields). See `.github/workflows/pr-title.yml`.

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.4.6 (2026-08-19)

### 🚀 Features

- **bookshelf:** add wishlist board with catalog auto-fulfillment ([#49](https://github.com/tanjd/core-repository/pull/49))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.4.5 (2026-08-19)

This was a version bump only for bookshelf-backend to align it with other projects, there were no code changes.

## 0.4.4 (2026-08-19)

### 🚀 Features

- **bookshelf:** add admin-authored announcements with notification panel redesign ([#47](https://github.com/tanjd/core-repository/pull/47))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.4.3 (2026-08-18)

### 🩹 Fixes

- **bookshelf:** harden backend, fix admin UI and notification bugs ([#46](https://github.com/tanjd/core-repository/pull/46))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.4.2 (2026-08-18)

### 🩹 Fixes

- **bookshelf:** close SMTP injection, setup race, missing headers ([#45](https://github.com/tanjd/core-repository/pull/45))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.4.1 (2026-08-18)

### 🚀 Features

- **bookshelf:** verify email and phone before registration completes ([#43](https://github.com/tanjd/core-repository/pull/43))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.4.0 (2026-08-18)

### 🚀 Features

- **bookshelf-backend:** rate-limit registration and OTP-send endpoints ([#41](https://github.com/tanjd/core-repository/pull/41))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.3.2 (2026-08-18)

### 🩹 Fixes

- **bookshelf:** close session-revocation gap, harden login and password rules ([#40](https://github.com/tanjd/core-repository/pull/40))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.3.1 (2026-08-18)

### 🚀 Features

- **bookshelf:** gate registration behind admin approval, fix audit findings ([#39](https://github.com/tanjd/core-repository/pull/39))

### ❤️ Thank You

- Jeddy Tan @tanjd

## 0.3.0 (2026-08-17)

### 🚀 Features

- admin dashboard stats, user management, and app settings endpoints ([#31](https://github.com/tanjd/core-repository/pull/31))
- SMTP-based email notifications ([#31](https://github.com/tanjd/core-repository/pull/31))
- pending email-change confirmation flow ([#33](https://github.com/tanjd/core-repository/pull/33))

This release was cut manually: PRs #31 and #33 used non-Conventional-Commit
squash titles ("Bookshelf: ..." instead of "feat(bookshelf-backend): ..."),
so `nx release`'s conventional-commits detection never picked them up and
`bookshelf-backend` stayed on 0.2.0 while the code on `main` moved ahead —
this is what broke the admin dashboard in production (404 on
`/admin/dashboard`, a route that didn't exist in the 0.2.0 image).

## 0.2.0 (2026-08-16)

### 🚀 Features

- migrate bookshelf into the monorepo ([#30](https://github.com/tanjd/core-repository/pull/30))

### ❤️ Thank You

- Jeddy Tan @tanjd
