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
