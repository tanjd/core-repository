---
name: create-pr
description: Create a pull request based on the branch's commits and diff, following this repo's PR title convention. Overrides the global create-pr skill for core-repository.
---

Create a pull request based on the branch's commits and diff.

## Why the title format matters here

This repo only allows squash merging (`allow_merge_commit`/`allow_rebase_merge`
are both off), and the squash commit's message comes from the **PR title**
whenever the PR has more than one commit (`squash_merge_commit_title:
COMMIT_OR_PR_TITLE` — see `gh api repos/tanjd/core-repository`). That squash
commit is exactly what `nx release`'s `conventionalCommits` engine reads on
`main` to decide whether a project gets a version bump. A PR titled like a
plain sentence (no `type(scope):` prefix) produces a squash commit `nx
release` can't parse — the project silently gets **no version bump, no
changelog entry, no published image**, even though real code shipped to
`main`. This already happened: PRs #31 and #33 shipped admin
dashboard/SMTP/email-change features to `bookshelf-backend` under
non-conventional titles, `nx release` never bumped it past 0.2.0, and
production ran a stale image with a 404 on a route that existed in source
but not in the shipped image. Fixed by manually cutting
`bookshelf-backend@0.3.0` — see `apps/bookshelf-backend/CHANGELOG.md`'s 0.3.0
entry and the `release-and-deployment` skill.

`.github/workflows/pr-title.yml` enforces this format as a check on every PR, and its `validate`
job is a **required** status check on `main`'s branch protection (alongside `main` and
`docker-build`) — a non-conventional title blocks merging outright. `enforce_admins` is off
though, so it's still bypassable manually; don't rely on the check catching a bad title, get it
right before creating/updating the PR.

## Steps

1. Run the following in parallel to gather context:
   - `git branch --show-current` — get branch name
   - `git log main...HEAD --oneline` — list all commits on this branch
   - `git diff main...HEAD --stat` — files changed summary
   - `git diff main...HEAD` — full diff for deeper analysis
   - `gh pr view 2>/dev/null || true` — check if a PR already exists

2. PR Title: Conventional Commits format, describing the outcome
   - `type(scope): imperative-mood summary` (≤72 chars total, no period).
   - **type**: `feat`, `fix`, `refactor`, `test`, `chore`, `docs`, or `perf`
     (same set the `commit` skill uses).
   - **scope**: every nx project with real code changes across all commits on the branch,
     comma-separated (e.g. `feat(bookshelf,bookshelf-backend): ...`) — not just the most
     obviously-affected one. `nx release`'s `conventionalCommits.useCommitScope` defaults to
     `true`, so a project touched by the PR but not named in its scope only gets an indirect
     patch-level bump, never the real `feat`/`fix`-implied bump, even though the change is real
     (this has happened repeatedly — see `pr-title.yml`'s comment block for the specific PRs).
     Don't scope a project only touched by docs/CLAUDE.md updates supporting another project's
     change. Omit scope entirely only if the change spans many areas with no clear owning
     project(s).
   - Say what the change achieves, not what you did while coding.
   - If the branch's commits already use a `type(scope):` prefix (check `git log`), match that
     scope rather than re-deriving it — the title should describe the same change the commits do.

3. If a PR already exists, ask the user whether to update its title/description or stop.

4. PR Description (PR Message)
   - The description should help the reviewer understand the change quickly.
   - Problem: What was wrong?
   - Solution: What approach did you take?

5. Show the full draft PR title and body to the user and ask for confirmation before creating.

6. On confirmation:
   - Push the branch if not already pushed: `git push -u origin HEAD`
   - Write the PR body to a temp file, then create the PR:
     ```
     cat > /tmp/pr_body.md << 'EOF'
     <body content>
     EOF
     gh pr create --title "..." --base main --body-file /tmp/pr_body.md
     ```
   - Return the PR URL.
