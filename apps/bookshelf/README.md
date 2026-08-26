# Bookshelf

A self-hosted community book-lending app. Members share physical copies of
books they own; others browse the catalogue and request to borrow them. Free,
open source, and built for small trust groups — a building, an office, a
church, a book club, a friend group.

**Live demo:** [bookshelf.tanjd.com](https://bookshelf.tanjd.com)

## Features

- **Catalogue** — search and filter every book the community has shared
- **Share a book** — scan an ISBN barcode or search by title; cover and
  metadata are fetched automatically (Open Library + optional Google Books)
- **Borrowing workflow** — request, accept, hand off, return; owners manage
  copies and loan history
- **Wishlist board** — ask for books nobody has shared yet; requesters are
  notified when a match appears
- **Waitlists** — queue for copies that are already out on loan
- **Notifications & announcements** — in-app alerts plus optional email
- **Admin tools** — user management, community settings, scheduled backups,
  metadata refresh jobs
- **Mobile-first UI** — bottom tab bar, ISBN scanner, thumb-friendly layouts

## Quick start (Docker)

Pre-built images are published to GitHub Container Registry. You do not need
to clone this monorepo to run Bookshelf.

1. Copy the deployment files to a directory on your host (e.g. your NAS):

   ```bash
   mkdir bookshelf && cd bookshelf
   curl -LO https://raw.githubusercontent.com/tanjd/core-repository/main/apps/bookshelf-backend/docker-compose.example.yml
   mv docker-compose.example.yml docker-compose.yml
   curl -LO https://raw.githubusercontent.com/tanjd/core-repository/main/apps/bookshelf-backend/.env.compose.example
   mv .env.compose.example .env
   ```

   Or copy `docker-compose.example.yml` and `.env.compose.example` from this
   repo manually.

2. Generate secrets and paste them into `.env`:

   ```bash
   echo "JWT_SECRET=$(openssl rand -base64 32)"
   echo "ENCRYPTION_SECRET=$(openssl rand -base64 32)"
   ```

   Both are **required**. The backend refuses to start in production
   (`ENV=prd`) if either is still set to a default placeholder.

3. Set `FRONTEND_ORIGIN` to the URL you will use in a browser (e.g.
   `http://192.168.1.10:3000` or `https://books.example.com`).

4. Start the stack:

   ```bash
   docker compose pull && docker compose up -d
   ```

5. Open `FRONTEND_ORIGIN` in a browser. On first run you are redirected to
   `/setup` — create the administrator account, then invite members from
   **Admin → Users**.

Only port **3000** (frontend) is published. The Go API runs on an internal
Docker network; the Next.js server proxies all `/api/*` calls to it, so the
backend never needs to be exposed directly.

### SMTP (recommended for production)

Registration, password reset, and loan notifications use email. Leave
`SMTP_HOST` empty only for solo testing — outbound mail is logged but not
sent, and magic-link registration will not work for real users.

Any SMTP provider with STARTTLS on port 587 works (Brevo, Amazon SES, Mailgun,
Google Workspace, etc.). See [Environment variables](#environment-variables)
below.

### Upgrading

```bash
docker compose pull && docker compose up -d
```

Database migrations run automatically on backend startup. Check
[CHANGELOG.md](./CHANGELOG.md) for release notes and any **Database migrations**
subsection when a release changes the schema (automatic on startup — no manual SQL
in normal cases).

After upgrading, members see an in-app **What's new** notice in the notification
bell; admins can also spot-check `/changelog` and `GET /health` (`schema_version`).

### Backups

Scheduled snapshots (database + cached cover images) are enabled by default
and configurable from **Admin → Jobs** and **Admin → Backups**. Restore is a
documented manual procedure — see the in-app backups page or
[apps/bookshelf-backend/CLAUDE.md](../bookshelf-backend/CLAUDE.md) (Backup and
restore section).

### HTTPS / reverse proxy

The example compose publishes plain HTTP on port 3000. For HTTPS, put a
reverse proxy (Caddy, nginx, Traefik) in front and set `FRONTEND_ORIGIN` to
your public `https://` URL. The backend reads `FRONTEND_ORIGIN` when building
links in emails.

## Environment variables

Docker deployments use `.env` next to `docker-compose.yml`. See
[`.env.compose.example`](../bookshelf-backend/.env.compose.example) for the
full reference.

| Variable               | Required | Description                                                                                        |
| ---------------------- | -------- | -------------------------------------------------------------------------------------------------- |
| `JWT_SECRET`           | yes      | Signs session tokens. Generate with `openssl rand -base64 32`.                                     |
| `ENCRYPTION_SECRET`    | yes      | Encrypts stored secrets (e.g. Google Books API keys). Use a **different** value from `JWT_SECRET`. |
| `FRONTEND_ORIGIN`      | yes      | Public URL of the frontend, used for CORS and email links.                                         |
| `ENV`                  | no       | `dev` (default) or `prd`. Use `prd` in production.                                                 |
| `SMTP_HOST`            | no*      | SMTP server. *Required for real multi-user registration.                                           |
| `SMTP_PORT`            | no       | Default `587`.                                                                                     |
| `SMTP_USERNAME`        | no       | SMTP auth username.                                                                                |
| `SMTP_PASSWORD`        | no       | SMTP auth password.                                                                                |
| `EMAIL_FROM`           | no       | Sender address for outbound mail.                                                                  |
| `DEV_EMAIL_OVERRIDE`   | no       | When `ENV=dev`, route all mail to this inbox (testing only).                                       |
| `GOOGLE_BOOKS_API_KEY` | no       | Optional metadata key(s), comma-separated for round-robin. Open Library is always used.            |

Local development uses
[`.env.example`](../bookshelf-backend/.env.example) in `apps/bookshelf-backend/`
plus `BACKEND_URL=http://localhost:8000` in
`apps/bookshelf/.env.local`.

## Development

Bookshelf lives in the [core-repository](https://github.com/tanjd/core-repository)
monorepo as two apps:

| App                 | Path                     | Role                          |
| ------------------- | ------------------------ | ----------------------------- |
| `bookshelf`         | `apps/bookshelf`         | Next.js frontend              |
| `bookshelf-backend` | `apps/bookshelf-backend` | Go API (Huma + GORM + SQLite) |

```bash
# From the repo root, inside the devcontainer:
cp apps/bookshelf-backend/.env.example apps/bookshelf-backend/.env
echo 'BACKEND_URL=http://localhost:8000' > apps/bookshelf/.env.local

pnpm nx serve bookshelf-backend   # :8000
pnpm nx serve bookshelf           # :3000
```

E2E tests (real servers, seeded auth):

```bash
pnpm nx e2e bookshelf-e2e
```

Build Docker images locally:

```bash
make docker-build APP=bookshelf-backend
make docker-build APP=bookshelf
```

## Project layout

```
apps/bookshelf/              Next.js frontend, landing page, admin UI
apps/bookshelf-backend/      Go API, SQLite, docker-compose.example.yml
apps/bookshelf-e2e/          Playwright end-to-end tests
```

## License

[MIT](./LICENSE)
