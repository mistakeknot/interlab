#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

# Benchmark status_campaigns hot path — this is called repeatedly during monitoring.
# Since status_campaigns calls bdShow/bdGetState (external CLI), we benchmark
# the in-process parts: experiment.ReconstructState across multiple campaigns.

BENCH_OUTPUT=$(command go test ./internal/experiment/ \
    -bench='BenchmarkReconstructState/entries_50$' \
    -benchtime=500x -count=1 -run='^$' 2>&1)

reconstruct_ns=$(echo "$BENCH_OUTPUT" | grep -E '^\s*BenchmarkReconstructState/entries_50-' | awk '{print $3}')
echo "METRIC reconstruct_50_ns=$reconstruct_ns"

# Also benchmark orchestration validation (shared hot path)
ORCH_OUTPUT=$(command go test ./internal/orchestration/ \
    -bench='BenchmarkValidatePlan/campaigns_10' \
    -benchtime=500x -count=1 -run='^$' 2>&1)

validate_ns=$(echo "$ORCH_OUTPUT" | grep 'ValidatePlan/campaigns_10' | awk '{print $3}')
echo "METRIC validate_10_ns=$validate_ns"

# Test suite time
TEST_START=$(date +%s%N)
command go test ./... -count=1 > /dev/null 2>&1
TEST_END=$(date +%s%N)
TEST_DURATION_MS=$(( (TEST_END - TEST_START) / 1000000 ))
echo "METRIC test_duration_ms=$TEST_DURATION_MS"

# Lines of code in status.go
LOC=$(wc -l < internal/orchestration/status.go)
echo "METRIC status_loc=$LOC"
