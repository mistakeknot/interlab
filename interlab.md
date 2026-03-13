# interlab: State Reconstruction Latency

## Objective

Optimize the hot path of interlab — `ReconstructState()` — which is called on every tool invocation. Current baseline shows ~1.5ms for a 100-entry JSONL file. The goal is to reduce this as much as possible while keeping the stateless crash-recovery property intact.

## Metrics

- **Primary**: reconstruct_100_ns (nanoseconds, lower_is_better) — time to reconstruct state from a 100-entry JSONL
- **Secondary**:
  - write_result_ns — JSONL append latency
  - parse_metrics_ns — METRIC line parsing time
  - test_duration_ms — full test suite time (must not regress >20%)
  - code_loc — lines of Go code (informational)

## How to Run

`bash interlab.sh` — runs Go benchmarks + test suite, outputs METRIC name=value lines

## Files in Scope

- `internal/experiment/state.go` — ReconstructState, WriteConfigHeader, WriteResult, appendJSONL
- `internal/experiment/tools.go` — parseMetrics, truncateTail, tool handlers
- `internal/experiment/ic.go` — ic bridge (only if it affects tool handler hot path)

## Constraints

- All existing tests must pass (`go test ./... -count=1`)
- No new external dependencies
- API contract (tool names, parameters, JSONL schema) must not change
- The stateless property must be preserved — state always reconstructable from JSONL alone
- Path-scoped git safety must be maintained

## What's Been Tried

(Campaign not yet started — awaiting baseline via `/autoresearch`)
