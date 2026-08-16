# Core Repository

An Nx monorepo (TypeScript/Next.js + Go + Python/uv), managed with pnpm and
developed inside a devcontainer for a consistent environment across
contributors. Long-term home for side projects, starting with Telegram bots.

## 🛠 Tech Stack

- **Build System**: [Nx](https://nx.dev/) (v22.2.2, capped — see "Known gaps"
  in `CLAUDE.md`)
- **Package Manager**: [pnpm](https://pnpm.io/) (pinned via
  `.devcontainer/devcontainer.json`, no `packageManager` field in
  `package.json`)
- **Languages & Frameworks**:
  - Python 3.13 (uv + Ruff)
  - Go 1.26 (huma + chi + SQLite via `mattn/go-sqlite3`, cgo)
  - Next.js / React
- **Development Environment**: DevContainers
- **CI/CD**: GitHub Actions (`ci.yml` on every push/PR to `main`,
  `release.yml` on push to `main` for versioning, `publish.yml` on release
  for image publishing)

## 📁 Repository Structure

```
.
├── apps/
│   ├── food-maps/              # Next.js frontend
│   ├── food-maps-backend/      # Go API (huma + chi + SQLite)
│   ├── food-maps-e2e/          # Playwright E2E tests for food-maps
│   ├── index-watch/            # Telegram bot (index drawdown tracker), Python/uv
│   ├── table-talks/            # Telegram bot (conversation card game), Python/uv
│   ├── ledger-lens-backend/    # FastAPI portfolio-analysis API, Python/uv
│   ├── ledger-lens/            # Next.js portfolio-analysis dashboard
│   ├── bookshelf-backend/      # Go API (huma + GORM/SQLite), book-lending app
│   └── bookshelf/              # Next.js frontend for the above
├── libs/
│   └── food-maps-data/         # Shared TS lib (@tanjd/food-maps-data path alias)
├── tools/
│   └── generators/
│       └── telegram-bot/       # Local Nx generator for scaffolding new bots
└── ...
```

## 🚀 Getting Started

### Prerequisites

1. [Docker](https://www.docker.com/get-started)
2. [VS Code](https://code.visualstudio.com/)
3. [VS Code Remote - Containers extension](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers)

### Development Setup

1. Clone the repository:

   ```bash
   git clone <repository-url>
   cd core-repository
   ```

2. Open in VS Code:

   ```bash
   code .
   ```

3. When prompted, click "Reopen in Container" or:
   - Press `F1`
   - Type "Reopen in Container"
   - Press Enter

4. The DevContainer will automatically install all required tools (Node.js,
   Python, Go, Docker-in-Docker, GitHub CLI, etc.) and run `make setup`.

### Development Commands

Prefer the `make` targets below over raw `pnpm nx` invocations — run
`make help` for the full, current list.

```bash
make setup      # pnpm install --frozen-lockfile, goimports, rtk init (Claude/Cursor)
make verify     # nx run-many -t build lint test (full local check)
make affected   # nx affected -t lint test (what CI actually runs)
make nx-reset   # pnpm nx reset, when a cached result looks stale
make new-bot NAME=foo                   # scaffold a new Telegram bot
make docker-build APP=food-maps-backend # docker build -f apps/<app>/Dockerfile .
make golangci-verify                    # confirm .golangci.yaml is actually being loaded
```

For anything not covered by a `make` target, use Nx directly:

```bash
pnpm nx show projects          # list all projects
pnpm nx <target> <project>     # e.g. pnpm nx test food-maps-backend
```

Git commits run Husky (`.husky/pre-commit`): lint-staged (ESLint + Prettier /
`gofmt` + `goimports` on staged files), the Python `pre-commit` framework's
generic sanity checks, and a full `pnpm nx affected -t lint test` — the same
gate CI runs, so failures surface locally before a push.

## 🧱 Project Management

### Adding New Projects

#### Python Project (Telegram Bot)

```bash
make new-bot NAME=my-new-bot
```

Scaffolds `apps/<bot-name>/` with a `python-telegram-bot` skeleton, `Dockerfile`,
`.env.example`, `README.md`, and Nx targets that shell out to `uv`.

#### Go Project

```bash
pnpm nx g @nx-go/nx-go:project my-new-go-project
```

#### Next.js Project

```bash
pnpm nx g @nx/next:app my-new-next-app
```

### Running Projects

```bash
pnpm nx <target> <project>
# Example: pnpm nx test food-maps-backend
```

## 🚢 Deployment

Any deployable app owns its own `Dockerfile` under `apps/<name>/`, built with
the repo root as build context. An app opts into deployment by declaring
empty `docker-build`/`docker-push` targets in its `project.json` — the
shared `docker buildx build` command lives once in `nx.json`'s
`targetDefaults`. Currently dockerized: `food-maps-backend`, `index-watch`
(`food-maps` is not).

- `ci.yml` runs `nx affected -t docker-build` (build-only) on every push/PR
  to `main`.
- `release.yml` runs `nx release --skip-publish` on push to `main`, computing
  each affected project's next version from Conventional Commits and cutting
  a GitHub Release (versioning only, no image push).
- `publish.yml` reacts to that release, checks out the released tag, and
  runs `nx run <project>:docker-push`, tagging `latest`, the commit SHA, and
  the released `v<semver>`, pushed to `ghcr.io/tanjd/<app-name>`.

See "Release versioning" in `CLAUDE.md` for the full versioning/publishing
pipeline, including the PAT-based anti-recursion workaround.

## 🛠 DevContainer Features

- 🐍 Python 3.13 with uv
- 🟦 Node.js 24 with pnpm
- 🔷 Go 1.26 with golangci-lint 2.12.2
- 🐳 Docker-in-Docker with buildx
- 🔧 Pre-configured VS Code extensions (Ruff, Prettier, Go, even-better-toml, …)
- 🐚 ZSH with helpful plugins, shell history, GitHub CLI

## 🤝 Contributing

1. Create a new branch for your feature
2. Make your changes
3. Run `make verify` (or at minimum `make affected`)
4. Submit a pull request — CI runs `nx affected -t lint test` and
   `nx affected -t docker-build` against `main`

## 📚 Additional Resources

- [Nx Documentation](https://nx.dev/getting-started/intro)
- [DevContainers Documentation](https://code.visualstudio.com/docs/remote/containers)
- [uv Documentation](https://docs.astral.sh/uv/)
- [Go Documentation](https://golang.org/doc/)

See `CLAUDE.md` for the full, detailed reference (Nx conventions, per-language
setup, deployment internals, known gaps).
