#!/usr/bin/env bash
# scan-plugin-quality.sh — Score all interverse plugins and rank by PQS.
#
# Usage: bash scripts/scan-plugin-quality.sh [--json] [--top=N]
#   --json    Output JSON instead of table
#   --top=N   Only show bottom N plugins (default: 5)
#
# Must be run from the monorepo root (or DEMARCH_ROOT must be set).
set -euo pipefail

DEMARCH_ROOT="${DEMARCH_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || echo ".")}"
BENCHMARK="$DEMARCH_ROOT/interverse/interlab/scripts/plugin-benchmark.sh"
PLUGIN_DIR="$DEMARCH_ROOT/interverse"

JSON_OUTPUT=false
TOP_N=5

for arg in "$@"; do
    case "$arg" in
        --json) JSON_OUTPUT=true ;;
        --top=*) TOP_N="${arg#--top=}" ;;
    esac
done

if [[ ! -f "$BENCHMARK" ]]; then
    echo "Error: plugin-benchmark.sh not found at $BENCHMARK" >&2
    exit 1
fi

# Collect results
results="[]"

for plugin_path in "$PLUGIN_DIR"/*/; do
    plugin_name=$(basename "$plugin_path")

    # Skip non-plugin directories (no plugin.json or .claude-plugin/)
    if [[ ! -f "$plugin_path/.claude-plugin/plugin.json" ]] && [[ ! -f "$plugin_path/plugin.json" ]]; then
        continue
    fi

    # Run benchmark, capture METRIC lines
    metrics=$(bash "$BENCHMARK" "$plugin_path" 2>/dev/null) || metrics=""

    pqs=$(echo "$metrics" | grep -oP 'plugin_quality_score=\K[\d.]+' || echo "0")
    audit_score=$(echo "$metrics" | grep -oP 'audit_score=\K[\d.]+' || echo "0")
    audit_max=$(echo "$metrics" | grep -oP 'audit_max=\K[\d.]+' || echo "19")
    struct_pass=$(echo "$metrics" | grep -oP 'structural_tests_pass=\K[\d.]+' || echo "0")
    struct_total=$(echo "$metrics" | grep -oP 'structural_tests_total=\K[\d.]+' || echo "0")
    build_passes=$(echo "$metrics" | grep -oP 'build_passes=\K[\d.]+' || echo "0")

    # Check for interlab.sh (METRIC-readiness)
    has_interlab=0
    if [[ -f "${plugin_path}interlab.sh" ]]; then
        has_interlab=1
    fi

    results=$(echo "$results" | jq --arg name "$plugin_name" \
        --arg path "$plugin_path" \
        --argjson pqs "${pqs:-0}" \
        --argjson audit "${audit_score:-0}" \
        --argjson audit_max "${audit_max:-19}" \
        --argjson struct_pass "${struct_pass:-0}" \
        --argjson struct_total "${struct_total:-0}" \
        --argjson build "${build_passes:-0}" \
        --argjson has_interlab "$has_interlab" \
        '. + [{
            name: $name,
            path: $path,
            pqs: $pqs,
            audit_score: $audit,
            audit_max: $audit_max,
            structural_pass: $struct_pass,
            structural_total: $struct_total,
            build_passes: $build,
            has_interlab: $has_interlab
        }]')
done

# Sort by PQS ascending (worst first)
sorted=$(echo "$results" | jq 'sort_by(.pqs)')
bottom=$(echo "$sorted" | jq --argjson n "$TOP_N" '.[:$n]')
total=$(echo "$sorted" | jq 'length')
avg_pqs=$(echo "$sorted" | jq '[.[].pqs] | if length > 0 then add / length else 0 end')

if [[ "$JSON_OUTPUT" == true ]]; then
    echo "$sorted" | jq --argjson bottom "$bottom" --argjson total "$total" --argjson avg "$avg_pqs" '{
        total_plugins: $total,
        avg_pqs: $avg,
        bottom: $bottom,
        all: .
    }'
else
    echo "Plugin Quality Scan: $total plugins scored (avg PQS: $(printf '%.3f' "$avg_pqs"))"
    echo ""
    interlab_count=$(echo "$sorted" | jq '[.[] | select(.has_interlab == 1)] | length')
    echo "METRIC-ready: $interlab_count / $total plugins have interlab.sh"
    echo ""
    echo "Bottom $TOP_N (improvement targets):"
    echo "| Plugin | PQS | Audit | Build | Struct | Interlab |"
    echo "|--------|-----|-------|-------|--------|----------|"
    echo "$bottom" | jq -r '.[] | "| \(.name) | \(.pqs | tostring | .[0:5]) | \(.audit_score)/\(.audit_max) | \(.build_passes) | \(.structural_pass)/\(.structural_total) | \(if .has_interlab == 1 then "Y" else "-" end) |"'
    echo ""
    echo "Full ranking:"
    echo "$sorted" | jq -r '.[] | "\(.pqs | tostring | .[0:5]) \(.name)"'
fi
