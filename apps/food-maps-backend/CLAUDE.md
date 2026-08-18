# CLAUDE.md

Guidance for `apps/food-maps-backend` specifically — see the repo-root `CLAUDE.md` for
cross-cutting conventions (Nx, deployment, release process).

## Go tooling

- Lint config is `.golangci.yaml` at the repo root — note the full "golangci", not "golanci".
  The installed `golangci-lint` is v2, which requires the v2 config schema (`version: "2"`,
  linter settings under `linters.settings`, not bare top-level keys). Verify a config change is
  actually being picked up with `make golangci-verify` (or `golangci-lint run -v`, looking for
  `[config_reader] Used config file ...`) — a silent fallback to defaults is exactly the bug that
  motivated this note.
- `nx run food-maps-backend:lint` depends on `golangci-lint` (see `nx.json` →
  `targetDefaults.lint.dependsOn`), so a plain `nx affected -t lint` genuinely gates on it, not
  just `go vet`/`go fmt`.
- `repository/sqlite`'s `DB`/`Tx` types were named `SQLiteDB`/`SQLiteTx` until root's
  `.golangci.yaml` gained `revive` (flagged as a stuttering name, `sqlite.SQLiteDB`) — see
  `apps/bookshelf-backend/CLAUDE.md`'s Go tooling notes for the full story of that config change,
  which also fixed a `gosec` G112 finding here (`ReadHeaderTimeout` on the `http.Server` in
  `cmd/main.go`) and four `noctx` findings (`Ping`/`Exec` → `PingContext`/`ExecContext`).

## Known gaps

- Nx is capped below 23.0.0 workspace-wide because of this app: `@nx-go/nx-go` (its Go plugin) has
  zero versions supporting Nx 23+ as of this writing (confirmed by `nx show projects` crashing
  entirely, not just Go targets, under 23.1.0) — currently on 22.7.8 (bumped from 22.2.2 for a
  Dependabot-flagged `nx` CVE; `@nx-go/nx-go`'s `@nx/devkit` peer range was `>= 20 < 23` at the
  time, confirmed via the check below before migrating). Before running `make upgrade-nx` (or
  `nx migrate latest`) again, check `npm view @nx-go/nx-go dependencies` for its `@nx/devkit`
  range first — `nx migrate latest` on its own is not safe here, since "latest" may already be
  23+; pin the migration to a specific 22.x version instead (`nx migrate 22.x.y`).

Dockerized (GHCR) and versioned independently via `nx release` (`release.projects` in root
`nx.json`) — already bootstrapped; every push to `main` versions it automatically.
