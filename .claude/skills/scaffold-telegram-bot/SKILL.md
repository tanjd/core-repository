---
name: scaffold-telegram-bot
description: Scaffold a new Telegram bot app in this monorepo via the local Nx generator (make new-bot NAME=...) — what it produces, how it wires up libs/telegram-bot-shared, and known gotchas with the generator invocation.
---

# Scaffolding a new Telegram bot

```bash
make new-bot NAME=<bot-name>
```

Which wraps (and is easier to remember than):

```bash
pnpm nx g ./tools/generators/telegram-bot/generators.json:telegram-bot <bot-name>
```

(The shorter `nx g ./tools/generators/telegram-bot <name>` form doesn't resolve
in this Nx version — local generator collections need either a registered
project or an explicit path to `generators.json`.)

Produces `apps/<bot-name>/` with a `python-telegram-bot` skeleton (health-check
endpoint, bot-token selection, and logging setup all wired in from
`libs/telegram-bot-shared` — see `.claude/rules/python.md`), `Dockerfile`,
`.env.example`, `README.md`, and a `project.json` tagged `lang:python` with
`build`/`serve`/`test`/`lint`/`update-deps`/`docker-build` targets
that shell out to `uv` (`nx:run-commands`, same pattern `food-maps-backend`
uses for Go — no language-specific Nx plugin). Uses **uv**, not Poetry, to
match the standalone bot repos this pattern was proven against
(`index-watch`, `table-talks`, both now migrated). `@nxlv/python` was
removed as a dependency — nothing here uses it.

The generated tag and `update-deps` target (`uv lock --upgrade && uv sync`) need no further setup:
`make upgrade-python` resolves projects by `tag:lang:python` rather than a hardcoded list, so the
new bot is picked up automatically. See root `CLAUDE.md`'s "Nx conventions" section, and
`make check-required-targets` (`tools/check-required-targets.sh`, also run by Husky's pre-commit)
if either is ever missing on a hand-added project.

The devcontainer's `uv` feature (`ghcr.io/va-h/devcontainers-features/uv`) is
required for the generator's post-generation `uv lock` step and for the
generated targets to run at all.
