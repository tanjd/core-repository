## 1.7.4 (2026-09-04)

This was a version bump only for table-talks to align it with other projects, there were no code changes.

## 1.7.3 (2026-08-26)

This was a version bump only for table-talks to align it with other projects, there were no code changes.

## 1.7.2 (2026-08-19)

This was a version bump only for table-talks to align it with other projects, there were no code changes.

## 1.7.1 (2026-08-19)

This was a version bump only for table-talks to align it with other projects, there were no code changes.

## 1.7.0 (2026-08-16)

This was a version bump only for table-talks to align it with other projects, there were no code changes.

## 1.6.0 (2026-07-31)

This was a version bump only for table-talks to align it with other projects, there were no code changes.

## 1.5.0 (2026-07-31)

This was a version bump only for table-talks to align it with other projects, there were no code changes.

# CHANGELOG

<!-- carried over from the standalone tanjd/table-talks repo (pre-migration history, python-semantic-release format) -->

## v1.5.0 (2026-02-15)

### Features

- Add validation for theme IDs and callback data
  ([`3f46b42`](https://github.com/tanjd/table-talks/commit/3f46b420bd81f4d9450daa5440a3e11185a1731a))

## v1.4.0 (2026-02-15)

### Features

- Add Google Sheets integration with pluggable data sources
  ([`b480a3a`](https://github.com/tanjd/table-talks/commit/b480a3a3f13f3b0d8da97d7caf075f811b53bfa8))

## v1.3.2 (2026-02-14)

### Bug Fixes

- **ci**: Add version list marker to CHANGELOG.md for v10 compatibility
  ([`6c4ebd7`](https://github.com/tanjd/table-talks/commit/6c4ebd74085ee7d554f339b327739e652a2a39bd))

### Continuous Integration

- Migrate to official python-semantic-release GitHub Actions
  ([`075127d`](https://github.com/tanjd/table-talks/commit/075127d19b909d4309cf190a404ccde2917ec390))

## v1.2.0 (2026-02-14)

### Chores

- Add conventional commit validation to pre-commit hook
  ([`06929f1`](https://github.com/tanjd/table-talks/commit/06929f1a800a58f87dbbe27e767e558213d3c458))

### Features

- Add SSH signing for verified release commits
  ([`09b6ca6`](https://github.com/tanjd/table-talks/commit/09b6ca6b57260df31204bd93dd43aa4e5d21bc73))

- Configure SSH signing keys in release workflow - Update commit author to use GitHub private email

  - Enable verified commits badge on GitHub

- Improve changelog formatting for Telegram display
  ([`d8450ff`](https://github.com/tanjd/table-talks/commit/d8450ff3a2a22fad7280533539bc9738e64cf3a4))

- Convert markdown headers to bold text for better readability - Replace list dashes with bullet
  points (•) - Add Markdown parse_mode to bot info and support messages - Make version headers bold
  for emphasis

## v1.1.5 (2026-02-14)

### Bug Fixes

- **ci**: Pin semantic-release to v9 and regenerate changelog
  ([`0c97e07`](https://github.com/tanjd/table-talks/commit/0c97e07a8ba442c1adac84e806f63ffaadca5bc7))

- **ci**: Restore complete changelog after tag recreation
  ([`390a4b7`](https://github.com/tanjd/table-talks/commit/390a4b72144662ccf79039399e78cd074a3bcc33))

Restore CHANGELOG.md with all versions (v1.0.0 - v1.1.4) after recreating tags on rewritten commits.
Next release will be v1.1.5.

- **docker**: Copy pyproject.toml and CHANGELOG.md to runtime image
  ([`0213ab7`](https://github.com/tanjd/table-talks/commit/0213ab794f357cdc724066da89737ffa6f37c7ca))

Add pyproject.toml and CHANGELOG.md to the runtime stage so the bot can read version and changelog
information for the Bot Info screen.

## v1.1.4 (2026-02-14)

### Bug Fixes

- **ci**: Update changelog config to non-deprecated format
  ([`f3a0fd7`](https://github.com/tanjd/table-talks/commit/f3a0fd73d1e3f8053d7f1a5f384be9acbe14bd5f))

Move changelog_file to changelog.default_templates.changelog_file as the old location is deprecated
and will break in v10.

## v1.1.3 (2026-02-14)

### Bug Fixes

- **ci**: Remove non-existent templates dir from semantic-release config
  ([`ea4a715`](https://github.com/tanjd/table-talks/commit/ea4a715507c2fda929b1e0f62d1d2424eb33dabd))

Remove template_dir configuration that pointed to non-existent 'templates' directory, causing
semantic-release to skip changelog generation. Now uses default templates to properly generate
CHANGELOG.md.

- **ci**: Use only 'published' event to prevent duplicate CD runs
  ([`ebe756e`](https://github.com/tanjd/table-talks/commit/ebe756e28cff2ed4766bafff24e51e191ea82276))

Remove 'released' event type from CD workflow trigger. Both 'published' and 'released' events fire
for the same release, causing duplicate workflow runs. Using only 'published' covers all release
types.

## v1.1.2 (2026-02-14)

### Bug Fixes

- **ci**: Configure commit author in semantic-release config
  ([`8e091ca`](https://github.com/tanjd/table-talks/commit/8e091ca5ffc2ce31777958f8bed93b5c7915290b))

Add commit_author setting to pyproject.toml to ensure semantic-release uses the verified
github-actions bot identity for release commits.

- **ci**: Use PAT to enable CD workflow triggering
  ([`19381ce`](https://github.com/tanjd/table-talks/commit/19381ce03d82ebd704d4bc28006074dfc9b83e3a))

Use GHA_TRIGGER_TOKEN (PAT) instead of GITHUB_TOKEN to allow the Release workflow to trigger the CD
workflow. GITHUB_TOKEN prevents triggering other workflows by design, while a PAT with workflow
scope enables this behavior.

## v1.1.1 (2026-02-14)

### Bug Fixes

- **ci**: Add 'released' event type to CD workflow trigger
  ([`9d6c5b2`](https://github.com/tanjd/table-talks/commit/9d6c5b2cb282afd07e1f28bfa82f6da7cfb0b569))

python-semantic-release may trigger 'released' instead of 'published' event. Add both types to
ensure CD workflow triggers for all releases.

- **ci**: Configure semantic-release remote for GitHub releases
  ([`5c45c5e`](https://github.com/tanjd/table-talks/commit/5c45c5eda82e77e677b2770b162c5e47a76a2542))

Add remote.type and remote.name configuration to enable semantic-release publish command to create
GitHub releases, which triggers the CD workflow.

- **ci**: Set git author env vars for verified semantic-release commits
  ([`2dfc55d`](https://github.com/tanjd/table-talks/commit/2dfc55d35672c0e9cdff3a54e951d1ca60432c6e))

Use GIT*AUTHOR*_ and GIT*COMMITTER*_ environment variables to ensure semantic-release commits are
signed with the verified github-actions bot email.

## v1.1.0 (2026-02-14)

### Bug Fixes

- **ci**: Use verified email for github-actions bot commits
  ([`c2d20fe`](https://github.com/tanjd/table-talks/commit/c2d20fed8e8ebd7226d6e7ba8a4c2fabe2b783b2))

Update release workflow to use GitHub Actions bot's verified email format
(41898282+github-actions[bot]@users.noreply.github.com).

This ensures all semantic-release commits appear as "Verified" in GitHub.

### Features

- Add version and changelog display in bot info screen
  ([`03c3376`](https://github.com/tanjd/table-talks/commit/03c337660817113b837292737227194c221d1350))

Implement automated version tracking and changelog display: - Create src/version.py module to parse
version from pyproject.toml and extract last 2 versions from CHANGELOG.md - Update bot info screen
to display current version and recent changes - Remove BOT_VERSION env var (now automatically
reads from pyproject.toml managed by semantic-release) - Update .env.example to document automatic
version/changelog management - Mark changelog system as complete in TODO.md

Version and changelog are now fully automated - no manual configuration needed.

## v1.0.1 (2026-02-14)

### Bug Fixes

- **ci**: Checkout main branch instead of SHA for semantic-release
  ([`298eced`](https://github.com/tanjd/table-talks/commit/298eced273073656e52008670caae378acb3e017))

semantic-release requires being on a branch, not in detached HEAD state. Changed from checking out
the specific SHA to checking out the main branch.

Since the release workflow has concurrency control and only runs after CI passes, this ensures we're
always on the latest validated commit.

### Chores

- **ci**: Add timeouts, concurrency control, and best practices
  ([`32c0b37`](https://github.com/tanjd/table-talks/commit/32c0b3751c8993d34d057477d0db6fd8b222f4de))

CI Workflow: - Add 15-minute timeout to prevent hanging - Add concurrency control to cancel old runs
(faster feedback) - Add explicit read-only permissions

Release Workflow: - Add 10-minute timeout - Add concurrency control (one release at a time, no
cancellation) - Update to checkout@v6 and setup-uv@v7 for consistency - Simplify dependency
install using --extra release from lockfile

CD Workflow: - Add 15-minute timeout for multi-platform Docker builds - Add concurrency control per
release tag (one deploy at a time) - Explicit read-only permissions already present

All workflows now follow GitHub Actions best practices with proper resource management, security
controls, and reproducible builds from lockfile.

- **ci**: Make release workflow dependent on CI success
  ([`2f9a7dd`](https://github.com/tanjd/table-talks/commit/2f9a7dd3c49f3a4b5b8a633e3aa88985c9d85a68))

- Change release.yml trigger from push to workflow_run - Release only runs after CI workflow
  completes successfully - Checkout the same commit SHA that CI tested - Prevents releasing code
  that fails tests or checks

This ensures only validated code gets released and deployed.

## v1.0.0 (2026-02-14)

### Bug Fixes

- **bot**: Use context.args for start payload, generate secret per run
  ([`1b5635c`](https://github.com/tanjd/table-talks/commit/1b5635c005827ab8f32083aa6873d7abe390912d))

- Parse /start argument from context.args for reliable deep link and manual input - Remove
  BOT_SECRET env; secret is always generated at startup and logged

### Chores

- Add make setup and run on postCreate
  ([`6b08176`](https://github.com/tanjd/table-talks/commit/6b08176a748c6a0af9a465380221e0a2e227172f))

- Makefile: setup target runs uv sync and pre-commit install - Devcontainer: postCreateCommand runs
  make setup

- Devcontainer, pre-commit, and app updates
  ([`089bc0e`](https://github.com/tanjd/table-talks/commit/089bc0ebbfd70751c652e7f9e73f195a51e48284))

- Devcontainer: base image, Python+uv feature, pre-commit and docker-in-docker features, VSCode
  settings and extensions (Even Better TOML) - Add .pre-commit-config.yaml (pre-commit-hooks, ruff,
  basedpyright) - Bot and health module updates; pyproject and uv.lock refresh

- Improve .env.example and Makefile documentation
  ([`855a47f`](https://github.com/tanjd/table-talks/commit/855a47f1ee514bc1bb20a0999f8e364f1ff0989b))

- Reorganize .env.example with clear sections (required/optional/reserved) - Add missing HEALTH_PORT
  variable to .env.example - Comment out CREATOR_USER_ID (reserved for future features) - Update
  Makefile to self-documenting pattern with ## comments - Add sectioned help output (Development,
  Testing & Quality, Docker) - Add upgrade target for dependency updates - Suppress recursive make
  directory messages

- Initial commit
  ([`5c6ab04`](https://github.com/tanjd/table-talks/commit/5c6ab0402c9cf1a4dec4d5df2fc7bcb3b26bd18d))

Table Talks Telegram bot with theme-based conversation cards, access control, health check, and
Docker support.

### Features

- Add automated semantic versioning and release workflow
  ([`d914ce2`](https://github.com/tanjd/table-talks/commit/d914ce27e00a6d4f5b8e74dcb9e2e99063ec5278))

- Configure python-semantic-release in pyproject.toml (version 0.0.0) - Add release.yml workflow to
  analyze commits and create releases - Modify cd.yml to deploy only on GitHub releases with version
  tags - Create initial CHANGELOG.md template - Add semantic-release dependencies

Implements Phase 1: Automated version tracking with conventional commits. Releases trigger on
feat/fix/BREAKING commits, auto-generating changelog and deploying Docker images tagged with
semantic versions.

- **bot**: Add end session button and environment-aware config
  ([`59384f9`](https://github.com/tanjd/table-talks/commit/59384f9f1a740b4c178150f82d7e2564b6204aef))

Add "End session" button to allow users to terminate the game gracefully. Implement
environment-aware configuration (ENV=dev/prd) with backward-compatible BOT_TOKEN for production
and BOT_TOKEN_DEV for development.

Also cleanup devcontainer settings and remove unused deployment docs.

>

- **bot**: Add home page with navigation and bot info
  ([`3c6aa04`](https://github.com/tanjd/table-talks/commit/3c6aa0499990244c02b9cfe239efa3ec24068eab))

- Add welcome home page with explanation of how bot works - Add bot info screen showing version and
  deployment time - Add support creator button with Buy Me a Coffee link - Add home button to all
  screens for easy navigation - Add back button to theme selection screen - Rename "Start Session"
  to "Select Theme" for clarity - Remove creator name and source link from bot info (cleaner UX) -
  Update TODO.md to reflect completed features

- **ci**: Add GitHub Actions CI/CD and make check-ci
  ([`86b1e84`](https://github.com/tanjd/table-talks/commit/86b1e84d2fb442104bf4c1628b306438c6b5b630))

- CI: run pre-commit and make check on push/PR to main - CD: build and push Docker image to Docker
  Hub after CI succeeds - Makefile: add check-ci target (pre-commit --all-files then check) - Pin
  Python 3.13 in .python-version

### Refactoring

- Modularize bot code into separate files
  ([`f3cbd43`](https://github.com/tanjd/table-talks/commit/f3cbd43e88934b191889f1adc743eaeb8ea91118))

Split monolithic src/bot.py (364 lines) into organized package structure for better maintainability
and separation of concerns.

New structure: - src/bot/**init**.py: Public API exports - src/bot/constants.py: All callback
constants and messages - src/bot/session.py: Session management and utilities - src/bot/auth.py:
Authorization decorator and helpers - src/bot/keyboards.py: Keyboard builders -
src/bot/handlers.py: Command and callback handlers - src/bot/app.py: Application building and
lifecycle

All handlers now use @require_auth decorator pattern to eliminate code duplication. All checks pass
(ruff, basedpyright, pytest). >

- **bot**: Add auth decorator and improve session management
  ([`d4f86f4`](https://github.com/tanjd/table-talks/commit/d4f86f4baacd775b45e26d65c6466685db1e1c42))

Add @require_auth decorator to eliminate code duplication across callback handlers. Improve shutdown
notifications to only notify chats with active sessions, skipping users who have ended their
session.

Also add environment logging to track which environment (dev/prd) the bot is running in, and update
TODO with new feature ideas and code quality tasks.

Changes: - Add require_auth decorator with proper type annotations - Apply decorator to
theme_chosen, next_card, new_topic, and end_session - Check for active sessions before sending
offline notifications - Pass environment info to build_application and log it - Update TODO with
feature ideas and refactoring tasks >
