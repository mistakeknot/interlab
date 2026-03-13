#!/usr/bin/env bash
set -euo pipefail
# interlab dogfood benchmark — measures state reconstruction and JSONL write performance.
# Outputs METRIC lines for the autoresearch loop.

cd "$(dirname "$0")"

# Run Go benchmarks with enough iterations for stable results
BENCH_OUTPUT=$(go test ./internal/experiment/ \
    -bench='BenchmarkReconstructState/entries_100|BenchmarkWriteResult|BenchmarkParseMetrics' \
    -benchtime=500x -count=1 -run='^$' 2>&1)

# Extract primary metric: ReconstructState latency for 100-entry JSONL (ns/op)
reconstruct_ns=$(echo "$BENCH_OUTPUT" | grep -E '^\s*BenchmarkReconstructState/entries_100' | awk '{print $3}')
echo "METRIC reconstruct_100_ns=$reconstruct_ns"

# Extract secondary metrics
write_ns=$(echo "$BENCH_OUTPUT" | grep -E '^\s*BenchmarkWriteResult' | awk '{print $3}')
echo "METRIC write_result_ns=$write_ns"

parse_ns=$(echo "$BENCH_OUTPUT" | grep -E '^\s*BenchmarkParseMetrics' | awk '{print $3}')
echo "METRIC parse_metrics_ns=$parse_ns"

# Also run full test suite to capture test count + duration
TEST_START=$(date +%s%N)
go test ./... -count=1 > /dev/null 2>&1
TEST_END=$(date +%s%N)
TEST_DURATION_MS=$(( (TEST_END - TEST_START) / 1000000 ))
echo "METRIC test_duration_ms=$TEST_DURATION_MS"

# Total lines of code (Go only, excluding tests)
LOC=$(find internal/ cmd/ -name '*.go' ! -name '*_test.go' | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}')
echo "METRIC code_loc=$LOC"
