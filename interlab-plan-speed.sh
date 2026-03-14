#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

# Benchmark plan_campaigns validation + conflict detection
BENCH_OUTPUT=$(command go test ./internal/orchestration/ \
    -bench='BenchmarkValidatePlan/campaigns_10|BenchmarkDetectFileConflicts/campaigns_10' \
    -benchtime=500x -count=1 -run='^$' 2>&1)

validate_ns=$(echo "$BENCH_OUTPUT" | grep 'ValidatePlan/campaigns_10' | awk '{print $3}')
echo "METRIC validate_10_ns=$validate_ns"

conflicts_ns=$(echo "$BENCH_OUTPUT" | grep 'DetectFileConflicts/campaigns_10' | awk '{print $3}')
echo "METRIC conflicts_10_ns=$conflicts_ns"

# Test suite time
TEST_START=$(date +%s%N)
command go test ./internal/orchestration/ -count=1 > /dev/null 2>&1
TEST_END=$(date +%s%N)
TEST_DURATION_MS=$(( (TEST_END - TEST_START) / 1000000 ))
echo "METRIC test_duration_ms=$TEST_DURATION_MS"

# Lines of code in plan.go
LOC=$(wc -l < internal/orchestration/plan.go)
echo "METRIC plan_loc=$LOC"
