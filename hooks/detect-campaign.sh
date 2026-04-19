#!/usr/bin/env bash
# Detect active interlab campaign and prompt resume.
# SessionStart hook — runs at the start of each Claude Code session.
set -uo pipefail
trap 'exit 0' ERR

JSONL="interlab.jsonl"

# Check if interlab.jsonl exists in cwd
if [[ ! -f "$JSONL" ]]; then
    exit 0
fi

# Determine the current campaign state by walking the JSONL: find the last
# `config` line, then verify no `campaign_closed` event follows it. A closed
# campaign must not trigger a resume prompt — the loop has already met its
# target (or been abandoned).
CAMPAIGN_NAME=$(python3 - "$JSONL" <<'PY' 2>/dev/null || echo "unknown"
import json, sys

path = sys.argv[1]
last_config = None
last_config_idx = -1
closed_after_config = False

with open(path) as f:
    for idx, line in enumerate(f):
        line = line.strip()
        if not line:
            continue
        try:
            data = json.loads(line)
        except json.JSONDecodeError:
            continue
        t = data.get("type")
        if t == "config":
            last_config = data.get("name", "unknown")
            last_config_idx = idx
            closed_after_config = False
        elif t == "campaign_closed" and last_config_idx >= 0:
            closed_after_config = True

if last_config is None or closed_after_config:
    print("unknown")
else:
    print(last_config)
PY
)

if [[ "$CAMPAIGN_NAME" == "unknown" || -z "$CAMPAIGN_NAME" ]]; then
    exit 0
fi

echo "Active interlab campaign detected: '$CAMPAIGN_NAME'. Read interlab.md and resume the experiment loop with /autoresearch."
