---
name: autoresearch
description: Run a continuous optimization loop — edit code, benchmark, keep or discard, repeat. Use when systematically optimizing a metric through iterative code changes.
---

# /autoresearch — Autonomous Experiment Loop

Run a continuous optimization loop: edit code, benchmark, keep or discard, repeat.

**Announce at start:** "I'm using the autoresearch skill to run an autonomous experiment loop."

## When to Use

Use this skill when you want to systematically optimize a metric by iterating through code changes. Examples:
- Optimize test suite speed
- Reduce binary size
- Improve benchmark throughput
- Minimize memory allocation
- Tune configuration for performance

## Prerequisites

The interlab MCP tools must be available: `init_experiment`, `run_experiment`, `log_experiment`.

Verify with a quick mental check: can you see these tools in your tool list? If not, the interlab plugin is not loaded — stop and tell the user.

## Setup Phase

If no `interlab.md` exists in the working directory:

### Step 1: Determine the Goal

Ask the user (or infer from context):
- **What metric** to optimize (e.g., "test_duration", "binary_size", "throughput")
- **Which direction** — `lower_is_better` or `higher_is_better`
- **What command** to benchmark (must output `METRIC name=value` lines)
- **Which files** are in scope for modification
- **What constraints** apply (tests must pass, no new deps, API stability, etc.)

### Step 2: Create the Benchmark Script

Write `interlab.sh` (or use an inline command) that outputs metrics in the format:

```
METRIC <name>=<value>
```

Multiple METRIC lines are supported (one primary + optional secondary metrics). The script must be deterministic enough to measure real changes — avoid metrics that fluctuate >5% between identical runs.

Example:

```bash
#!/usr/bin/env bash
set -euo pipefail

# Run the thing being measured
START=$(date +%s%N)
go test ./... > /dev/null 2>&1
END=$(date +%s%N)

DURATION_MS=$(( (END - START) / 1000000 ))
echo "METRIC test_duration=$DURATION_MS"
echo "METRIC test_count=$(go test ./... -v 2>&1 | grep -c '^--- PASS')"
```

### Step 3: Initialize the Campaign

Call `init_experiment` with:
- `name`: short campaign name (e.g., "skaffen-test-speed")
- `metric_name`: primary metric to optimize (must match a METRIC line name)
- `metric_unit`: unit (ms, bytes, ops/s, etc.)
- `direction`: `"lower_is_better"` or `"higher_is_better"`
- `benchmark_command`: command to run (e.g., `"bash interlab.sh"`)
- `working_directory`: project root (omit to use cwd)

This creates `interlab.jsonl` and checks out a branch `interlab/<name>`.

### Step 4: Write the Living Document

Create `interlab.md` in the working directory:

```markdown
# interlab: <goal>

## Objective
<what we're optimizing and why>

## Metrics
- **Primary**: <name> (<unit>, <direction>)
- **Secondary**: <names if any>

## How to Run
`bash interlab.sh` — outputs METRIC name=value lines

## Files in Scope
<list of files the agent may modify>

## Constraints
<hard rules: tests must pass, no new deps, API must not change, etc.>

## What's Been Tried
<updated after each experiment — key wins, dead ends, insights>
```

### Step 5: Run Baseline

1. Call `run_experiment` to establish the starting metric value.
2. Call `log_experiment` with `decision: "keep"` and `description: "baseline measurement"`.
3. Update `interlab.md` with the baseline value under "What's Been Tried".

### Step 6: Query Prior Mutations (if mutation store available)

After establishing baseline, check for prior approaches on this task type:

1. Call `mutation_query` with:
   - `task_type`: the campaign's task type (e.g., "agent-quality", "plugin-quality")
   - `is_new_best`: true (only successful approaches)
   - `limit`: 10

2. If results returned, add to `interlab.md` under "## Prior Approaches (from mutation store)":
   - List each prior approach: hypothesis, quality_signal, campaign_id
   - Mark which ones are "known good" (is_new_best=true) and "known dead ends" (recorded but not new_best)
   - These seed the agent's hypothesis generation — avoid re-exploring dead ends

3. If `mutation_query` fails or returns empty: continue normally. The mutation store is optional.

#### Cross-Session Discovery (if interlock available)

In addition to the local mutation store, check for broadcasts from parallel sessions:

1. Call `list_topic_messages` with `topic: "mutation"` to get recent mutation broadcasts from other agents.

2. For each broadcast message:
   - Parse the JSON body to extract task_type, hypothesis, quality_signal, is_new_best
   - If `task_type` matches the current campaign's task type, add to the "Prior Approaches" section
   - Mark these as "cross-session" to distinguish from local mutation store results

3. If `list_topic_messages` fails or is unavailable: continue with local mutation store only.

This enables compound learning: Agent A's discovery feeds Agent B's hypothesis generation, even when they're running in different sessions.

## Loop Phase

**LOOP FOREVER. Never ask "should I continue?" Never pause to check in. The circuit breaker is the safety net — trust it.**

Each iteration:

### 1. Read Context

Read `interlab.md` to refresh on what's been tried, what works, and what constraints apply. If `interlab.ideas.md` exists, check for untried ideas.

### 2. Generate an Idea

Look at the code, the metrics, and past attempts. Think about what single change could improve the primary metric. Prioritize:
- Ideas from the backlog (`interlab.ideas.md`) first
- Low-risk, high-expected-impact changes
- Changes that are orthogonal to previous attempts

### 3. Edit Code

Make **ONE focused change**. Small, targeted edits beat large rewrites. You need to isolate what caused any metric shift.

If the campaign has a test constraint, run tests before proceeding to step 4. If tests fail, fix them or revert and try a different approach.

### 4. Run the Benchmark

Call `run_experiment`. Read the output carefully:
- Primary metric value and delta vs. best
- Secondary metrics (if any)
- Exit code (non-zero = crash)
- Output tail for errors or warnings

### 5. Decide

| Condition | Decision | Action |
|-----------|----------|--------|
| Primary improved AND secondaries acceptable | `"keep"` | Changes committed automatically |
| Primary regressed | `"discard"` | Changes reverted automatically |
| Secondary degraded >20% even if primary improved | `"discard"` | Changes reverted automatically |
| Benchmark crashed (non-zero exit, timeout, error) | `"crash"` | Changes reverted automatically |

Call `log_experiment` with the decision and a description of what you changed and why.

### 5b. Record Mutation

After each `log_experiment` call, record the mutation for provenance tracking:

1. Call `mutation_record` with:
   - `task_type`: campaign's task type
   - `hypothesis`: the description passed to log_experiment
   - `quality_signal`: the metric value from run_experiment
   - `campaign_id`: the campaign name
   - `inspired_by`: if the hypothesis was explicitly inspired by a prior approach from the mutation query, include that session_id

2. Note whether `is_new_best` was true — this signals a meaningful improvement.

3. If `mutation_record` fails: log a warning but do NOT stop the campaign. Mutation recording is best-effort.

### 5c. Broadcast Mutation (if interlock available)

After recording the mutation, broadcast it so parallel sessions can discover this approach:

1. Call `broadcast_message` with:
   - `topic`: `"mutation"`
   - `subject`: `"[<campaign_name>] <keep|discard|crash>: <hypothesis summary>"`
   - `body`: JSON string with: `{"task_type": "<type>", "hypothesis": "<description>", "quality_signal": <value>, "is_new_best": <bool>, "campaign_id": "<name>", "session_id": "<id>"}`

2. If `broadcast_message` fails or is unavailable (interlock not loaded): continue silently. Broadcasting is best-effort.

**Important:** `log_experiment` handles git operations. On "keep", it stages in-scope files and commits. On "discard" or "crash", it reverts in-scope files. Do NOT run git commands yourself.

### 6. Update Documents

- **`interlab.md`**: Append to "What's Been Tried" with the result (1-2 lines per experiment).
- **`interlab.ideas.md`**: If you discovered new optimization ideas during this iteration, add them. Mark completed ideas as tried.

### 7. Continue

Go back to step 1. Do not pause. Do not ask the user.

## Exit Conditions

Stop the loop when ANY of these are true:

- **Circuit breaker trips**: `run_experiment` returns an error about limits (max experiments: 50, max consecutive crashes: 3, max no-improvement streak: 10)
- **Ideas exhausted**: You've tried everything in the backlog AND cannot generate new plausible ideas
- **Metric converged**: Last 5 experiments show <1% variance from the best value
- **Hard constraint violated**: Tests broken in a way you can't fix, or API contract changed

When stopping:

### 1. Write Final Summary

Add to `interlab.md`:

```markdown
## Final Summary
- **Starting**: <baseline metric value>
- **Ending**: <best metric value>
- **Improvement**: <absolute and percentage>
- **Experiments**: <total> (<kept>/<discarded>/<crashed>)
- **Key wins**: <top 2-3 changes that moved the needle>
- **Key insights**: <what you learned about this codebase/metric>
```

### 2. Archive the Campaign

Completed campaigns are saved to `campaigns/<name>/` for future reference:

```bash
mkdir -p campaigns/<name>
cp interlab.jsonl campaigns/<name>/results.jsonl
```

Write `campaigns/<name>/learnings.md` with validated insights, dead ends, and generalizable patterns (see template in Learnings Document section below).

Update `campaigns/README.md` index table with the campaign summary row.

### 3. Clean Up

Delete `interlab.jsonl` and `interlab.md` from the working directory — the archived copies in `campaigns/` are the permanent record. The next campaign starts fresh.

## Resuming a Campaign

If `interlab.md` already exists when this skill is invoked:

1. Read `interlab.md` for full context on the campaign
2. Read `interlab.ideas.md` if it exists — prune completed or invalid ideas
3. Continue the loop from step 1 of the Loop Phase

Do not re-run baseline. Do not re-initialize. The JSONL has all the history.

## Ideas Backlog

Maintain `interlab.ideas.md` as a lightweight holding pen:

```markdown
# Ideas Backlog

## Promising
- [ ] <idea> — <expected impact>

## Tried
- [x] <idea> — <result>

## Rejected
- [-] <idea> — <why not>
```

Keep this file lean. One line per idea. Move ideas between sections as they're attempted.

## Learnings Document

After significant discoveries (not every iteration — only genuine insights), update `interlab-learnings.md`:

```markdown
# interlab Learnings: <campaign>

## Validated Insights
- <insight> — proved by experiment #N, delta <X>%
  - Evidence: <what changed, what metrics showed>

## Dead Ends
- <approach> — tried in experiment #N, no improvement because <reason>

## Patterns
- <general pattern discovered> — applies beyond this campaign
```

## Rules

These are non-negotiable:

1. **One change per experiment.** Never bundle multiple changes. You need to know what caused the metric shift.
2. **Always run tests first** (if the campaign has a test constraint) before calling `run_experiment`.
3. **Path-scoped changes only.** Only modify files listed in "Files in Scope" in `interlab.md`.
4. **No manual git operations.** Let `log_experiment` handle all git staging, committing, and reverting.
5. **Secondary metrics matter.** If a secondary metric degrades >20%, discard even if the primary improved.
6. **Never ask to continue.** The loop runs until an exit condition is met. The circuit breaker exists for safety.
7. **Update the living document.** Every experiment result gets logged in `interlab.md`. This is how future sessions (and humans) understand what happened.

## Common Mistakes

**Bundling multiple changes**
- Problem: Metric improves but you don't know which change helped
- Fix: One change per iteration, always

**Ignoring secondary metrics**
- Problem: Optimize speed but memory usage doubles
- Fix: Check ALL metrics in `run_experiment` output before deciding

**Forgetting to update interlab.md**
- Problem: Next session repeats failed experiments
- Fix: Write to "What's Been Tried" after every single experiment

**Running git commands manually**
- Problem: Conflicts with log_experiment's automatic staging/reverting
- Fix: Let the tool handle git. You handle code edits only.

**Pausing to ask the user**
- Problem: Breaks the autonomous loop, wastes human attention
- Fix: Trust the circuit breaker. Keep going until an exit condition fires.
