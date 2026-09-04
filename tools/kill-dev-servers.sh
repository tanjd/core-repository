#!/usr/bin/env bash
# Kills locally running `nx serve` dev servers (Next.js, Vite, Go, uv/Python
# bots) and their child processes.
#
# Deliberately a standalone script rather than an inline Makefile recipe:
# `pkill -f <pattern>` matches against a process's *entire* command line, and
# a multi-line Makefile recipe is invoked as one `/bin/sh -c "<the whole
# recipe text>"` process — so if the search patterns are written inline in
# the recipe, that invoking shell's own /proc/pid/cmdline contains every
# pattern as a literal substring and can match (and kill) itself, or a
# freshly-forked-but-not-yet-exec'd descendant that briefly inherits the
# same cmdline. Keeping the patterns out of the invoking process's own argv
# (they only ever appear as arguments to short-lived pgrep/kill children,
# which self-exclude) avoids that footgun entirely.
set -u

patterns=(
	"nx run"
	"nx serve"
	"next-server"
	"next dev"
	"vite/bin/vite.js"
	"go run cmd/"
	"uv run uvicorn"
	"uv run python -m"
)

echo "Killing dev servers..."
for pattern in "${patterns[@]}"; do
	# pgrep already excludes its own PID; no need to filter $$ here since the
	# pattern text never appears in this script's own invocation argv.
	pids=$(pgrep -f "$pattern" || true)
	if [ -n "$pids" ]; then
		echo "  $pattern: $(echo "$pids" | tr '\n' ' ')"
		kill $pids 2>/dev/null || true
	fi
done
echo "Done."
