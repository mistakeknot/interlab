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

extract_metric() {
    local metrics="$1"
    local key="$2"
    local type="$3"
    local plugin_name="$4"
    local value

    if ! value=$(printf '%s\n' "$metrics" | awk -v key="$key" '
        $1 == "METRIC" {
            prefix = key "="
            if (index($2, prefix) == 1) {
                value = substr($2, length(prefix) + 1)
                count++
            }
        }
        END {
            if (count == 1) {
                print value
            } else {
                exit 1
            }
        }
    '); then
        echo "Error: plugin $plugin_name emitted a missing or duplicate $key metric" >&2
        return 1
    fi

    case "$type" in
        integer)
            if [[ ! "$value" =~ ^[0-9]+$ ]]; then
                echo "Error: plugin $plugin_name emitted nonnumeric $key metric: $value" >&2
                return 1
            fi
            ;;
        number)
            if [[ ! "$value" =~ ^[0-9]+([.][0-9]+)?$ ]]; then
                echo "Error: plugin $plugin_name emitted nonnumeric $key metric: $value" >&2
                return 1
            fi
            ;;
    esac

    printf '%s\n' "$value"
}

# Collect results
results="[]"

for plugin_path in "$PLUGIN_DIR"/*/; do
    plugin_name=$(basename "$plugin_path")

    # Skip non-plugin directories (no plugin.json or .claude-plugin/)
    if [[ ! -f "$plugin_path/.claude-plugin/plugin.json" ]] && [[ ! -f "$plugin_path/plugin.json" ]]; then
        continue
    fi

    # Run benchmark, capture METRIC lines
    if ! metrics=$(bash "$BENCHMARK" "$plugin_path" 2>/dev/null); then
        echo "Error: benchmark failed for plugin $plugin_name" >&2
        exit 1
    fi

    pqs=$(extract_metric "$metrics" "plugin_quality_score" "number" "$plugin_name") || exit 1
    audit_score=$(extract_metric "$metrics" "audit_score" "integer" "$plugin_name") || exit 1
    audit_max=$(extract_metric "$metrics" "audit_max" "integer" "$plugin_name") || exit 1
    struct_pass=$(extract_metric "$metrics" "structural_tests_pass" "integer" "$plugin_name") || exit 1
    struct_total=$(extract_metric "$metrics" "structural_tests_total" "integer" "$plugin_name") || exit 1
    build_passes=$(extract_metric "$metrics" "build_passes" "integer" "$plugin_name") || exit 1

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
