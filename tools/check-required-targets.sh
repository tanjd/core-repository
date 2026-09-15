#!/usr/bin/env bash
# Verifies every Go/Python project is tagged and carries the Nx targets its
# language requires (currently just update-deps - see root CLAUDE.md's "Nx
# conventions" section). `make upgrade-go`/`make upgrade-python` resolve
# projects by tag (tag:lang:go/tag:lang:python) rather than a hardcoded list,
# so an untagged or under-configured project would otherwise just silently
# never get upgraded instead of erroring anywhere.
#
# Ground truth is the filesystem (go.mod / pyproject.toml), not the tag
# itself - checking "projects tagged lang:go are missing update-deps" alone
# can't catch a Go project that has no tag at all yet.
set -euo pipefail

missing_found=0

check_project() {
	local project_dir="$1" expected_tag="$2" required_target="$3"
	local project_json="${project_dir}/project.json"

	if [ ! -f "$project_json" ]; then
		echo "  - ${project_dir}: no project.json (not registered as an Nx project)"
		missing_found=1
		return
	fi

	if ! jq -e --arg tag "$expected_tag" '(.tags // []) | index($tag) != null' "$project_json" >/dev/null; then
		echo "  - ${project_dir}: missing tag '${expected_tag}'"
		missing_found=1
	fi

	if ! jq -e --arg t "$required_target" '(.targets // {}) | has($t)' "$project_json" >/dev/null; then
		echo "  - ${project_dir}: missing target '${required_target}'"
		missing_found=1
	fi
}

echo "Checking Go projects (go.mod present)..."
while IFS= read -r -d '' gomod; do
	check_project "$(dirname "$gomod")" "lang:go" "update-deps"
done < <(find apps libs -mindepth 2 -maxdepth 2 -name go.mod -print0 2>/dev/null)

echo "Checking Python projects (pyproject.toml present)..."
while IFS= read -r -d '' pyproject; do
	check_project "$(dirname "$pyproject")" "lang:python" "update-deps"
done < <(find apps libs -mindepth 2 -maxdepth 2 -name pyproject.toml -print0 2>/dev/null)

if [ "$missing_found" -eq 0 ]; then
	echo "All Go/Python projects are correctly tagged and carry their required targets."
else
	exit 1
fi
