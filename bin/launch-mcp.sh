#!/usr/bin/env bash
# Launcher for interlab-mcp: probes known binary paths before falling back to go build.
# Probe order: cache-local → source-tree (dev) → ~/.local/bin → go build.
# Sidesteps envs where `go` is missing from the MCP subprocess PATH.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BINARY="${SCRIPT_DIR}/interlab-mcp"

for candidate in \
    "$BINARY" \
    "/home/mk/projects/Sylveste/interverse/interlab/bin/interlab-mcp" \
    "${HOME}/.local/bin/interlab-mcp"
do
    if [[ -x "$candidate" ]]; then
        exec "$candidate" "$@"
    fi
done

# Fallthrough: attempt build if toolchain available
if ! command -v go &>/dev/null; then
    echo '{"error":"go not found — cannot build interlab-mcp. Install Go 1.23+."}' >&2
    exit 0
fi
cd "$PROJECT_ROOT"
go build -o "$BINARY" ./cmd/interlab-mcp/ 2>&1 >&2
exec "$BINARY" "$@"
