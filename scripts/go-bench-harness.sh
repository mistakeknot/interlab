#!/usr/bin/env bash
set -euo pipefail

# Interlab harness: wraps `go test -bench` output into METRIC key=value format.
#
# Usage in interlab campaign YAML:
#   benchmark_command: bash interverse/interlab/scripts/go-bench-harness.sh \
#     --pkg ./priompt/ --bench BenchmarkRender100 --metric ns_per_op --dir masaq
#
# Runs the benchmark with -count=5 and reports the median ns/op as the primary
# metric. Secondary metrics: allocs_per_op, bytes_per_op.
#
# Options:
#   --pkg <path>       Go package path (e.g., ./priompt/)
#   --bench <pattern>  Benchmark name regex (e.g., BenchmarkRender100$)
#   --metric <name>    Primary metric name for METRIC output (default: ns_per_op)
#   --dir <path>       Working directory for go test (default: .)
#   --count <n>        Number of runs for median (default: 5)
#   --benchtime <dur>  Time per benchmark (default: 1s)

PKG=""
BENCH=""
METRIC_NAME="ns_per_op"
DIR="."
COUNT=5
BENCHTIME="1s"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --pkg) PKG="$2"; shift 2 ;;
        --bench) BENCH="$2"; shift 2 ;;
        --metric) METRIC_NAME="$2"; shift 2 ;;
        --dir) DIR="$2"; shift 2 ;;
        --count) COUNT="$2"; shift 2 ;;
        --benchtime) BENCHTIME="$2"; shift 2 ;;
        *) echo "Unknown option: $1" >&2; exit 1 ;;
    esac
done

if [[ -z "$PKG" || -z "$BENCH" ]]; then
    echo "Usage: $0 --pkg <package> --bench <pattern> [--metric name] [--dir path] [--count n]" >&2
    exit 1
fi

cd "$DIR"

# Run benchmark, capture raw output (|| true to prevent set -e from exiting)
RAW=$(go test -bench="$BENCH" -benchmem -count="$COUNT" -benchtime="$BENCHTIME" -run='^$' "$PKG" 2>&1) || EXIT=$?
EXIT=${EXIT:-0}

if [[ $EXIT -ne 0 ]]; then
    echo "METRIC ${METRIC_NAME}=-1"
    echo "METRIC benchmark_exit_code=$EXIT"
    echo "METRIC error=1"
    echo "$RAW" >&2
    exit 0  # don't crash interlab — report failure as metric
fi

# Parse benchmark lines: BenchmarkName-N  <iters>  <ns/op>  <bytes/op>  <allocs/op>
# Collect ns/op values across all runs, compute median
NS_VALUES=()
BYTES_VALUES=()
ALLOCS_VALUES=()

while IFS= read -r line; do
    # Match lines like: BenchmarkRender100-32    38670    34275 ns/op    132224 B/op    56 allocs/op
    if [[ "$line" =~ ^Benchmark.*[[:space:]]+([0-9]+)[[:space:]]+([0-9.]+)[[:space:]]ns/op ]]; then
        ns="${BASH_REMATCH[2]}"
        NS_VALUES+=("$ns")

        # Extract B/op if present
        if [[ "$line" =~ ([0-9]+)[[:space:]]B/op ]]; then
            BYTES_VALUES+=("${BASH_REMATCH[1]}")
        fi

        # Extract allocs/op if present
        if [[ "$line" =~ ([0-9]+)[[:space:]]allocs/op ]]; then
            ALLOCS_VALUES+=("${BASH_REMATCH[1]}")
        fi
    fi
done <<< "$RAW"

if [[ ${#NS_VALUES[@]} -eq 0 ]]; then
    echo "METRIC ${METRIC_NAME}=-1"
    echo "METRIC error=1"
    echo "METRIC parse_error=no_benchmark_lines_found"
    echo "Raw output:" >&2
    echo "$RAW" >&2
    exit 0
fi

# Compute median of sorted values
median() {
    local -a sorted
    IFS=$'\n' sorted=($(printf '%s\n' "$@" | sort -n))
    local n=${#sorted[@]}
    local mid=$((n / 2))
    echo "${sorted[$mid]}"
}

NS_MEDIAN=$(median "${NS_VALUES[@]}")
echo "METRIC ${METRIC_NAME}=${NS_MEDIAN}"
echo "METRIC run_count=${#NS_VALUES[@]}"

if [[ ${#BYTES_VALUES[@]} -gt 0 ]]; then
    BYTES_MEDIAN=$(median "${BYTES_VALUES[@]}")
    echo "METRIC bytes_per_op=${BYTES_MEDIAN}"
fi

if [[ ${#ALLOCS_VALUES[@]} -gt 0 ]]; then
    ALLOCS_MEDIAN=$(median "${ALLOCS_VALUES[@]}")
    echo "METRIC allocs_per_op=${ALLOCS_MEDIAN}"
fi

echo "METRIC benchmark_exit_code=0"
