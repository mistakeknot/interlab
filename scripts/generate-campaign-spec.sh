#!/usr/bin/env bash
# generate-campaign-spec.sh — Generate /autoresearch-multi campaign spec from scan results.
#
# Usage: bash scripts/generate-campaign-spec.sh [--top=N]
#   Scans plugins, outputs campaign spec JSON for plan_campaigns.
#
# Must be run from the monorepo root.
set -euo pipefail

DEMARCH_ROOT="${DEMARCH_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || echo ".")}"
SCANNER="$DEMARCH_ROOT/interverse/interlab/scripts/scan-plugin-quality.sh"
BENCHMARK="$DEMARCH_ROOT/interverse/interlab/scripts/plugin-benchmark.sh"

TOP_N=5
for arg in "$@"; do
    case "$arg" in
        --top=*) TOP_N="${arg#--top=}" ;;
    esac
done

# Get scan results
scan=$(bash "$SCANNER" --json --top="$TOP_N" 2>/dev/null)
bottom=$(echo "$scan" | jq '.bottom')
count=$(echo "$bottom" | jq 'length')

if [[ "$count" -eq 0 ]]; then
    echo "No plugins to improve." >&2
    exit 0
fi

# Generate campaign spec for plan_campaigns
echo "$bottom" | jq --arg benchmark "$BENCHMARK" '[
    .[] | {
        name: ("pqs-improve-" + .name),
        description: ("Improve plugin quality score for " + .name + " (current PQS: " + (.pqs | tostring) + ")"),
        metric_name: "plugin_quality_score",
        metric_unit: "score",
        direction: "higher_is_better",
        benchmark_command: ("bash " + $benchmark + " " + .path),
        working_directory: .path,
        task_type: "plugin-quality",
        files_in_scope: [
            (.path + "skills/"),
            (.path + ".claude-plugin/"),
            (.path + "hooks/"),
            (.path + "agents/")
        ]
    }
]'
