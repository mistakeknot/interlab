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

- **Baseline**: ~1,540K ns (median of 3 runs: 1421K, 1540K, 1650K)
- **#1 KEEP**: Replace `map[string]interface{}` type peek with `typeOnly` struct → ~1,062K ns (-31%). Eliminates full map allocation per JSONL line.
- **#2 KEEP**: Byte-level type detection (`bytes.Contains` instead of JSON unmarshal) → ~779K ns (-27%). Zero JSON parsing for type discrimination.
- **#3 KEEP**: Size scanner buffer to file (`f.Stat()`) → ~576K ns (-26%). Avoids 1MB allocation for small files.
- **#4 DISCARD**: Replace json.Encoder with json.Marshal in appendJSONL — no measurable change (write path, not read path).
- **#5 KEEP**: Lightweight result struct (only decode decision, metric_value, secondary_metrics) → ~463K ns (-20%).
- **#6 DISCARD**: Pre-allocate byte patterns as package vars — no measurable change (compiler optimizes `[]byte("literal")`).
- **#7 KEEP**: Byte-scan decisions, skip JSON parse entirely for discard/crash results → ~360K ns (-22%). Only parse metric_value for keeps.
- **#8 KEEP**: Extract metric_value via bytes.Index+ParseFloat → ~82K ns (-77%!). Zero json.Unmarshal for result lines. **17x faster than baseline.**
- **#9 KEEP**: Replace Scanner with os.ReadFile + bytes.Split → ~70K ns (-15%). Simpler code, fewer syscalls.
- **#10 KEEP**: Scan newlines in-place with bytes.IndexByte → ~68K ns (-3%). Avoids [][]byte allocation, tighter variance.

## Final Summary

- **Starting**: ~1,540K ns (1.54ms)
- **Ending**: ~68K ns (0.068ms)
- **Improvement**: -1,472K ns absolute, **-96% (22x faster)**
- **Experiments**: 10 total (7 kept / 3 discarded / 0 crashed)
- **Key wins**:
  1. (#8) Byte-scan metric_value extraction — eliminated last json.Unmarshal, -77% alone
  2. (#1+#2) Type discrimination via bytes.Contains instead of JSON parse — -50% combined
  3. (#3) Right-sized scanner buffer — -26%
- **Key insights**:
  - `json.Unmarshal` dominates even for tiny structs — it tokenizes the entire line
  - When you control the serialization format, byte-level scanning beats structured parsing
  - Go compiler optimizes `[]byte("literal")` — no need for package-level vars
  - Write-path optimizations don't affect read-path benchmarks (obvious in hindsight)
