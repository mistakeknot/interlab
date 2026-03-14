# interlab-multi: Optimize orchestration layer

## Objective
Optimize interlab's v0.3 orchestration code (`internal/orchestration/`) across two independent dimensions: plan validation speed and status reconstruction latency. This is the first real dogfood of the multi-campaign orchestration tools.

## Campaigns

| # | Name | Metric | Direction | Status | Best |
|---|------|--------|-----------|--------|------|
| 1 | plan-validation-speed | validate_10_ns | lower_is_better | planned | — |
| 2 | status-reconstruction | reconstruct_50_ns | lower_is_better | planned | — |

## File Ownership
- **Campaign 1 (plan-validation-speed)**: `internal/orchestration/plan.go`
- **Campaign 2 (status-reconstruction)**: `internal/orchestration/status.go`, `internal/experiment/state.go`
- **Shared (do not modify)**: `internal/orchestration/beads.go`, `internal/orchestration/register.go`

## Global Constraints
- All existing tests must pass (`go test ./... -count=1`)
- No new external dependencies
- beads.go and register.go must not be modified (shared infrastructure)
- API contract (tool names, parameters) must not change

## Progress Log
(updated as campaigns dispatch, complete, or produce insights)
