# interlab

> See `AGENTS.md` for full development guide.

## Overview

MCP server providing 3 stateless experiment-loop tools (init_experiment, run_experiment, log_experiment) with JSONL persistence, git branch isolation, and ic events bridge.

## Quick Commands

```bash
# Build binary
go build -o bin/interlab-mcp ./cmd/interlab-mcp/

# Run Go tests
go test ./...

# Validate structure
python3 -c "import json; json.load(open('.claude-plugin/plugin.json'))"
```

## Design Decisions (Do Not Re-Ask)

- Go binary for MCP server (mark3labs/mcp-go), bash for hooks
- Stateless tools — state reconstructed from JSONL on each call (crash recovery for free)
- Path-scoped git staging — never `git add -A`, always explicit file paths
- Circuit breaker guards: max experiments (50), max consecutive crashes (3), max no-improvement (10)
- Experiment branches: `interlab/<name>` as documented exception to trunk-based development
