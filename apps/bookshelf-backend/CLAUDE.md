# CLAUDE.md

Guidance for `apps/bookshelf-backend` specifically — see the repo-root `CLAUDE.md` for
cross-cutting conventions (Nx, deployment, release process).

## Migration history

Squash-imported from the standalone `tanjd/bookshelf` repo's `backend/` directory (personal
book-lending app backend, Go + Huma v2 + GORM/SQLite), no preserved history — same convention as
every other app migration in this repo. Paired with `apps/bookshelf` (the frontend, migrated from
the same source repo's `frontend/`).

- Go module renamed from `github.com/tanjd/bookshelf` to
  `github.com/tanjd/core-repository/apps/bookshelf-backend` to match `food-maps-backend`'s
  convention (module path mirrors its location in the monorepo); all internal imports were
  updated accordingly.
- Dockerfile rewritten for the repo-root build context (`docker build -f
apps/bookshelf-backend/Dockerfile .`) and switched from the source repo's `golang:1.25-alpine`
  to `golang:1.25-bookworm` + `debian:bookworm-slim` — same fix `food-maps-backend` already
  needed for the same dependency: `mattn/go-sqlite3`'s cgo build references `off64_t`/`pread64`/
  `pwrite64`, which musl (Alpine) doesn't expose the way glibc does. `wget` is installed in the
  runtime stage (absent from `debian:bookworm-slim` by default) so the `docker-compose.example.yml`
  healthcheck command, carried over from the source repo, keeps working unchanged.
- Uses the shared root `.golangci.yaml` only — no per-app override, same as `food-maps-backend`.
  The root config enables `gocognit` (min complexity 15), which the standalone repo's own config
  never had; 15 ported functions initially exceeded it (`loan_requests.go`'s `createLoanRequest`
  peaked at 54 — a Huma handler doing copy-availability checks, borrower-eligibility checks
  spanning four admin settings, request construction, and auto-approve, all inline). Rather than
  raise the threshold or carry a per-app override, each was split into small, single-purpose
  helper methods (e.g. `getRequestableCopy`, `checkBorrowerEligibility`,
  `checkVerificationRequirements`, `buildLoanRequest`, `finalizeLoanRequest`) — pure extraction, no
  behavior changes. `mergeGroup` in `metadata_consolidate.go` also moved from twelve sequential
  `if merged.X == "" { merged.X = r.X }` lines to a generic `firstNonEmpty`/`firstNonZero` helper
  taking a field accessor, since that's the same "first non-empty value in priority order" pattern
  repeated per field.
- The standalone repo's own `.golangci.yaml` also enabled `gosec`, `revive`, `noctx`, `misspell`,
  and `gocritic`, which root didn't at migration time. Rather than drop them, they were measured
  against both `bookshelf-backend` and `food-maps-backend` (raw finding counts, not just what
  survived existing `//nolint` comments) and added to the _root_ config once real findings were
  fixed rather than suppressed: `NewMetadataHandler`/`NewInMemoryMetadataCache` reordered `ctx` to
  the first parameter (`revive`'s `context-as-argument`), the `CopyRepository` interface/impl and
  one local var renamed `copy` → `bookCopy` (`revive`'s `redefines-builtin-id`, shadowing the
  builtin), and three `score += 1` became `score++`. `food-maps-backend` needed its own fixes for
  the same pass: a `ReadHeaderTimeout` added to its `http.Server` (`gosec` G112, Slowloris),
  `sqlite.SQLiteDB`/`sqlite.SQLiteTx` renamed to `sqlite.DB`/`sqlite.Tx` (`revive`'s stutter check),
  a comment justifying its blank `mattn/go-sqlite3` import, and four `Ping`/`Exec` calls switched
  to `PingContext`/`ExecContext` (`noctx`). See the repo-root `CLAUDE.md`'s Go tooling notes — this
  now genuinely widens linting for every Go app in the workspace, not just this one.

## Go tooling

- Uses `golang-migrate/migrate/v4`-style SQL migration pairs under
  `internal/db/migrations/*.{up,down}.sql` — `make migrate-new NAME=...` from the source repo's
  `Makefile` isn't ported yet; add a new pair by hand (zero-padded 6-digit sequence number) until
  it is.
- ORM is GORM with SQLite (`mattn/go-sqlite3`, cgo) — `DB_PATH` defaults to `./data/bookshelf.db`
  (see `.env.example`); `data/` is gitignored via the root `.gitignore`'s catch-all `*.db` rule.
- Cached book cover images are written to `./data/covers` at runtime (created on boot in
  `cmd/server/main.go`) — also covered by the `data/` directory, not tracked.
- `nx run bookshelf-backend:lint` depends on `golangci-lint` (see `nx.json` →
  `targetDefaults.lint.dependsOn`), so a plain `nx affected -t lint` genuinely gates on it, not
  just `go vet`/`go fmt`.
- `./data/screenshot-seed.db` (gitignored, same `*.db` rule) is a pre-seeded DB for taking
  `apps/bookshelf` landing-page screenshots against real-looking data instead of an empty
  catalogue: 10 real books (real Open Library/Google Books covers/metadata, via
  `/books/metadata/search`) split across two accounts —
  `admin@bookshelf.local` / `jamie@bookshelf.local`, both password `ScreenshotDemo42!` — so the
  catalogue reads as a multi-person community rather than one account's shelf. Point `DB_PATH` at
  it (`DB_PATH=./data/screenshot-seed.db go run ./cmd/server`) instead of reseeding from scratch;
  copy the `.db`/`-shm`/`-wal` trio together since it was last stopped mid-WAL rather than cleanly
  checkpointed.

## Wishlist board

`WishlistRequest` (`internal/models/models.go`, migration `000003_create_wishlist_requests`) lets
a member post "does anyone have X" for a book nobody's added to the catalog yet — book identity
comes from the same metadata-search external keys `/books`' `createBook` uses (ISBN/OL
key/Google Books ID), so a title always carries a resolvable key even before it exists as a
`Book` row.

- **Auto-fulfillment**: `BookHandler.createBook` calls `WishlistWorkflow.OnBookCreated`
  (`internal/services/wishlist_workflow.go`) after a genuinely new `Book` insert — never on the
  upsert-return-existing path. It matches by `OLKey`/`GoogleBooksID` only (`ISBN` isn't indexed
  for auto-match, only used in the create-time dedupe check below) and fulfills _every_ open
  request sharing the key, since multiple members can separately be looking for the same book.
  `BookHandler.wishlistWorkflow` is a nil-safe field so existing tests constructing a
  `BookHandler` without one keep working.
- **Single write site**: `fulfill()` in `wishlist_workflow.go` is the only place an open request
  transitions to `fulfilled` — both the auto-match path and the manual "link to an existing
  catalog book" path (`POST /wishlist/{id}/fulfill`, for a different edition/ISBN that didn't
  auto-match) route through it. It sets status/`FulfilledBookID`/`FulfilledAt`, creates a
  `wishlist_fulfilled` notification, and best-effort emails the requester (email failure doesn't
  fail the fulfillment).
- **Dedupe check**: `WishlistRequestRepository.FindOpenMatch` (checks ISBN/OL key/Google Books ID,
  unlike the auto-match pair above) powers a create-time "this is already on someone's wishlist"
  prompt so members join an existing request instead of posting a duplicate.
- **Route ordering**: `/wishlist/mine` and `/wishlist/check` are registered before the
  `/wishlist/{id}` wildcard in `WishlistHandler.RegisterRoutes` — same care as `notifications.go`,
  otherwise huma's wildcard swallows the literal paths.

`LoanRequestRepository.ListActiveByBorrowerID` (added alongside this feature, for the frontend's
"currently borrowed" cards) sorts with
`Order("expected_return_date IS NULL, expected_return_date ASC, requested_at ASC")` — a
SQLite-specific NULLS-last idiom relying on boolean expressions evaluating to 0/1, not portable
as-is to a different SQL dialect. `ListByBorrowerIDPaginated` also gained an optional
`statuses []string` filter (empty/nil = no filter) so the frontend can split "current" vs.
"history" tabs without new endpoints.

## Backup and restore

`internal/services/backup.go` (`BackupService`) creates a snapshot of the database plus the
cover-image cache, bundled into one `bookshelf-backup-<UTC-timestamp>.tar.gz` archive per run —
`bookshelf.db` (built via SQLite's `VACUUM INTO`, so it's a clean, WAL-safe single file with no
`-wal`/`-shm` sidecars to also copy) plus everything under `data/covers/`. Snapshots live in
`data/backups/`, created at boot the same way as `data/covers/` — this is **inside the existing
`bookshelf-data` Docker volume**, so no `docker-compose.example.yml`/`compose/*.yml` change was
needed to add it.

- Runs as a second job on the existing `Scheduler` (`internal/services/scheduler.go`), alongside
  cover-refresh — `AppSetting` keys `backup_interval` (default `24h`) and
  `backup_retention_count` (default `7`) control it, editable from the admin UI's Jobs/Backups
  pages same as `cover_refresh_interval`. `BackupService.CreateSnapshot` prunes down to the
  retention count after every run (keeps the newest N, deletes the rest).
- `internal/handlers/backup.go` exposes `GET /admin/backups` (list), `DELETE
/admin/backups/{filename}` (delete), and `GET /admin/backups/{filename}/download` (streams the
  archive via `huma.StreamResponse` — the one place this app returns a raw binary body through
  huma rather than JSON). Creating a snapshot on demand and configuring the schedule intentionally
  reuse the existing `POST /admin/jobs/backup/run` and `PATCH /admin/settings` endpoints rather
  than duplicating that surface.
- No DB table tracks backup history — `data/backups/` on disk is the source of truth (`ListSnapshots`
  reads the directory), matching how cover images have no DB-backed inventory either. Every
  filename-taking operation (download/delete) routes through `BackupService.ResolvePath`, which
  validates the name against a strict `bookshelf-backup-YYYYMMDDTHHMMSSZ.tar.gz` regex before
  touching the filesystem — the single guard against path traversal on this admin-controlled file
  I/O surface.
- **Restore is a manual, documented ops procedure — there is no in-app restore endpoint.**
  Swapping a live SQLite DB out from under a running server safely (in-flight requests, WAL state)
  isn't worth the risk for a single-admin self-hosted app; stopping the container first is simpler
  and safer:
  1. `docker compose stop bookshelf-backend` (the frontend can stay up; it'll show fetch errors
     until the backend restarts).
  2. Extract the chosen archive on the host: `tar -xzf bookshelf-backup-<ts>.tar.gz -C /tmp/restore`.
  3. Copy the volume's current contents aside as a safety net — it's a named volume, not a bind
     mount: `docker run --rm -v bookshelf-data:/data -v $(pwd)/pre-restore-backup:/backup alpine
cp -r /data/. /backup/`.
  4. Replace `bookshelf.db` and `covers/` inside the volume with the extracted ones, same
     `docker run -v bookshelf-data:/data ...` pattern as step 3.
  5. `docker compose start bookshelf-backend`.
  6. Verify via `GET /health`, then spot-check book/user counts against expectations in the admin
     dashboard.

## Known gaps

Dockerized (GHCR) and versioned independently via `nx release` (`release.projects` in root
`nx.json`) — this is its first release; the version starts at `0.1.0` rather than continuing the
source repo's own version history (that history wasn't preserved either, per the migration
decision above).
