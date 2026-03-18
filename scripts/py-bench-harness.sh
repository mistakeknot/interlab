#!/usr/bin/env bash
set -euo pipefail
# py-bench-harness.sh — wraps Python test/benchmark output into METRIC format.
#
# Usage:
#   bash py-bench-harness.sh --cmd "pytest tests/ -q" --metric test_duration_ms --dir interverse/intercache
#   bash py-bench-harness.sh --cmd "python3 benchmark.py" --metric quality_score --dir interverse/intermem
#
# Modes:
#   --mode timing  (default) — measures wall-clock duration of command in ms
#   --mode output  — parses stdout for "METRIC name=value" lines (passthrough)
#   --mode pytest  — parses pytest output for pass/fail counts + timing

CMD=""
METRIC_NAME="duration_ms"
DIR="."
MODE="timing"
COUNT=3

while [[ $# -gt 0 ]]; do
    case "$1" in
        --cmd) CMD="$2"; shift 2 ;;
        --metric) METRIC_NAME="$2"; shift 2 ;;
        --dir) DIR="$2"; shift 2 ;;
        --mode) MODE="$2"; shift 2 ;;
        --count) COUNT="$2"; shift 2 ;;
        *) echo "Unknown option: $1" >&2; exit 1 ;;
    esac
done

if [[ -z "$CMD" ]]; then
    echo "Usage: $0 --cmd <command> [--metric name] [--dir path] [--mode timing|output|pytest]" >&2
    exit 1
fi

cd "$DIR"

case "$MODE" in
    timing)
        DURATIONS=()
        ERRORS=0
        for ((i=0; i<COUNT; i++)); do
            START_MS=$(($(date +%s%N) / 1000000))
            bash -c "$CMD" >/dev/null 2>&1 || ERRORS=$((ERRORS + 1))
            END_MS=$(($(date +%s%N) / 1000000))
            DURATIONS+=($((END_MS - START_MS)))
        done
        # Median
        IFS=$'\n' SORTED=($(printf '%s\n' "${DURATIONS[@]}" | sort -n))
        MID=$(( ${#SORTED[@]} / 2 ))
        echo "METRIC ${METRIC_NAME}=${SORTED[$MID]}"
        echo "METRIC run_count=${#SORTED[@]}"
        [[ $ERRORS -gt 0 ]] && echo "METRIC error=$ERRORS"
        echo "METRIC benchmark_exit_code=0"
        ;;
    output)
        bash -c "$CMD" 2>/dev/null | grep '^METRIC ' || {
            echo "METRIC ${METRIC_NAME}=-1"
            echo "METRIC error=1"
        }
        ;;
    pytest)
        OUTPUT=$(eval "$CMD" 2>&1) || true
        PASSED=$(echo "$OUTPUT" | grep -oP '\d+ passed' | grep -oP '\d+' || echo "0")
        FAILED=$(echo "$OUTPUT" | grep -oP '\d+ failed' | grep -oP '\d+' || echo "0")
        TOTAL=$((PASSED + FAILED))
        RATE=$(python3 -c "print(f'{$PASSED / $TOTAL:.4f}' if $TOTAL > 0 else '0')")
        echo "METRIC ${METRIC_NAME}=$RATE"
        echo "METRIC tests_passed=$PASSED"
        echo "METRIC tests_failed=$FAILED"
        echo "METRIC tests_total=$TOTAL"
        echo "METRIC benchmark_exit_code=0"
        ;;
esac
