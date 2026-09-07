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

# `go run` (and similar wrapper invocations) execs the real program as a
# separate child process rather than exec'ing into it, and doesn't forward
# signals to that child — so killing only the matched wrapper PID leaves the
# actual server running (and its port held) as an orphaned child. Kill each
# matched PID's full descendant tree instead, children first, so nothing gets
# orphaned mid-walk.
kill_tree() {
	local pid="$1"
	local child
	for child in $(pgrep -P "$pid" 2>/dev/null || true); do
		kill_tree "$child"
	done
	kill "$pid" 2>/dev/null || true
}

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
		for pid in $pids; do
			kill_tree "$pid"
		done
	fi
done
echo "Done."
