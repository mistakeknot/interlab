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

# Extract the campaign name from the last config line
CAMPAIGN_NAME=$(grep '"type":"config"' "$JSONL" | tail -1 | python3 -c "
import json, sys
for line in sys.stdin:
    line = line.strip()
    if line:
        data = json.loads(line)
        print(data.get('name', 'unknown'))
        break
" 2>/dev/null || echo "unknown")

if [[ "$CAMPAIGN_NAME" == "unknown" || -z "$CAMPAIGN_NAME" ]]; then
    exit 0
fi

# Check if there's an active segment (config without a subsequent "end" marker)
# For now, presence of interlab.jsonl with a config line means active
echo "Active interlab campaign detected: '$CAMPAIGN_NAME'. Read interlab.md and resume the experiment loop with /autoresearch."
