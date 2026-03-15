---
name: autoresearch-multi
description: Run a multi-campaign optimization — decompose a broad goal into focused campaigns, dispatch them in parallel via subagents, monitor progress, and synthesize results. Use when optimizing multiple aspects of a module or exploring a hypothesis from multiple angles.
---

# /autoresearch-multi — Multi-Campaign Orchestration

Decompose a broad optimization goal into focused campaigns, dispatch them in parallel via subagents, monitor progress, and synthesize results.

**Announce at start:** "I'm using the autoresearch-multi skill to orchestrate a multi-campaign optimization."

## When to Use

Use this skill when a single `/autoresearch` loop is insufficient because the goal spans multiple dimensions. Examples:
- Optimize both test speed AND binary size for the same module
- Compare multiple approaches to the same problem (e.g., caching vs. batching vs. parallelism)
- Run comparative experiments across different metrics or configurations
- Explore a hypothesis from multiple angles simultaneously
- Broad module optimization where several independent subsystems can be improved in parallel

Do NOT use this skill for simple, single-metric optimization — use `/autoresearch` directly instead.

## Prerequisites

### Required Tools

The interlab MCP tools must be available:
- **Campaign orchestration**: `plan_campaigns`, `dispatch_campaigns`, `status_campaigns`, `synthesize_campaigns`
- **Single-campaign tools**: `init_experiment`, `run_experiment`, `log_experiment`

Verify with a quick mental check: can you see these tools in your tool list? If not, the interlab plugin is not loaded — stop and tell the user.

### Required Capabilities

- Subagent spawning (to dispatch parallel `/autoresearch` loops)
- Access to the target codebase with write permissions

## Phase 1: Analyze

Read the codebase and identify optimization targets.

### Step 1: Understand the Goal

Ask the user (or infer from context):
- **What broad goal** to pursue (e.g., "make the Skaffen agent faster and smaller")
- **Which module(s)** are in scope
- **What constraints** apply globally (tests must pass, no API changes, etc.)
- **How many campaigns** they expect (or let you decompose freely)

### Step 2: Identify Targets

Read the codebase to identify independent optimization dimensions:
- Profile the module for distinct subsystems (build, runtime, tests, etc.)
- Identify metrics that can be optimized independently
- Map file ownership — which files belong to which campaign
- Flag files that multiple campaigns might want to touch (conflict zones)

### Step 3: Write the Living Document

Create `interlab-multi.md` in the working directory:

```markdown
# interlab-multi: <broad goal>

## Objective
<what we're optimizing across multiple campaigns and why>

## Campaigns

| # | Name | Metric | Direction | Status | Best |
|---|------|--------|-----------|--------|------|
| 1 | <name> | <metric> | lower/higher | planned | — |
| 2 | <name> | <metric> | lower/higher | planned | — |

## File Ownership
- **Campaign 1**: <files>
- **Campaign 2**: <files>
- **Shared (conflict zone)**: <files needing coordination>

## Global Constraints
<hard rules that apply across all campaigns>

## Progress Log
<updated as campaigns dispatch, complete, or produce insights>
```

## Phase 2: Plan

### Step 1: Call plan_campaigns

Prepare a decomposition JSON and call `plan_campaigns`:

```json
{
  "goal": "<broad optimization goal>",
  "campaigns": [
    {
      "name": "<campaign-name>",
      "metric_name": "<primary metric>",
      "metric_unit": "<unit>",
      "direction": "lower_is_better | higher_is_better",
      "benchmark_command": "<command>",
      "files_in_scope": ["<file1>", "<file2>"],
      "constraints": ["<constraint1>"]
    }
  ]
}
```

Design campaigns so that:
- Each campaign optimizes **one primary metric**
- File scopes are **disjoint** wherever possible
- Each campaign can run independently without blocking others

### Step 2: Resolve File Conflicts

If two campaigns need to modify the same file:
1. **Prefer splitting** — can the file be refactored so each campaign owns distinct sections?
2. **Serialize** — mark one campaign as `depends_on` the other, so they run sequentially
3. **Shared constraint** — allow both but add a constraint that neither may change the shared file's public API

Document the resolution in `interlab-multi.md` under "File Ownership".

### Step 3: Write Benchmark Scripts

For each campaign, write (or verify) a benchmark script that outputs `METRIC name=value` lines. Name them distinctly:

```
interlab-<campaign-name>.sh
```

Each script must be independent — no shared state between campaign benchmarks.

## Phase 3: Dispatch

### Step 1: Call dispatch_campaigns

Call `dispatch_campaigns` to register all planned campaigns and mark them as ready for execution.

### Step 2: Spawn Subagents

For each campaign in `ready` status, spawn a subagent with instructions to:
1. Run the `/autoresearch` skill
2. Use the campaign-specific benchmark script
3. Respect the campaign's file scope and constraints
4. Write results to the campaign's own `interlab.md` and `interlab.jsonl`

Each subagent operates in isolation — it runs a full `/autoresearch` loop for its assigned campaign.

### Step 3: Update Living Document

Update `interlab-multi.md`:
- Set campaign statuses to `running`
- Log dispatch timestamps
- Note which subagent is handling which campaign

## Phase 4: Monitor

### Step 1: Poll Status

Periodically call `status_campaigns` to check progress across all campaigns:
- Which campaigns are still running
- Which have completed (hit exit conditions)
- Current best metric values per campaign
- Any crashes or stuck campaigns

### Step 2: Update Living Document

After each status check, update `interlab-multi.md`:
- Refresh the campaigns table with current status and best values
- Add progress notes to the log
- Flag any campaigns that appear stuck (no improvement for many iterations)

### Step 3: Handle Completions

When a campaign completes:
1. Read its results (`interlab.jsonl`, `interlab.md`)
2. Check if its results affect other running campaigns
3. If a completed campaign unlocked a `depends_on` campaign, dispatch the dependent
4. Re-dispatch if a campaign crashed and retries are warranted

### Step 4: Cross-Campaign Insights

If one campaign discovers something relevant to another:
- Add the insight to the dependent campaign's `interlab.ideas.md`
- Do NOT modify a running campaign's code or configuration directly — let the subagent pick up the idea on its next iteration

## Phase 5: Synthesize

### Step 1: Call synthesize_campaigns

Once all campaigns have completed (or been stopped), call `synthesize_campaigns` to aggregate results.

### Step 2: Write Final Summary

Update `interlab-multi.md` with:

```markdown
## Final Summary

### Overall Results
- **Campaigns**: <total> (<completed>/<stopped>/<crashed>)
- **Total experiments**: <sum across campaigns>

### Per-Campaign Results
| # | Name | Baseline | Best | Improvement | Experiments |
|---|------|----------|------|-------------|-------------|
| 1 | <name> | <value> | <value> | <delta> (<pct>%) | <count> |

### Cross-Campaign Insights
- <insight that emerged from comparing campaign results>

### Key Wins
- <top changes across all campaigns>

### Recommendations
- <what to do next, what wasn't explored, what needs human review>
```

### Step 3: Archive

For each campaign, archive results to `campaigns/<name>/`:
- Copy `interlab.jsonl` to `campaigns/<name>/results.jsonl`
- Write `campaigns/<name>/learnings.md` with validated insights

Update `campaigns/README.md` index table with all campaign summary rows.

Clean up working directory: remove per-campaign `interlab.jsonl`, `interlab.md`, and benchmark scripts.

Keep `interlab-multi.md` as the permanent multi-campaign record.

### Step 4: Broadcast Aggregate Results (if interlock available)

After synthesis completes, broadcast the campaign results so future sessions benefit:

1. For each campaign that improved its metric, call `broadcast_message` with:
   - `topic`: `"mutation"`
   - `subject`: `"[multi:<parent_bead>] <campaign_name> improved <metric> by <delta>%"`
   - `body`: JSON with the best approach for each campaign (task_type, hypothesis, quality_signal, campaign_id)

2. This is best-effort — failure does not block synthesis completion.

## Exit Conditions

Stop orchestration when ANY of these are true:

- **All campaigns complete**: Every campaign has hit an exit condition (circuit breaker, convergence, ideas exhausted)
- **User interrupts**: User explicitly asks to stop
- **No progress for 3 cycles**: Three consecutive status checks show no metric improvement across ANY campaign
- **Global constraint violated**: A cross-campaign invariant breaks (e.g., combined binary size exceeds budget)

## Resuming

If `interlab-multi.md` already exists when this skill is invoked:

1. Read `interlab-multi.md` for full context
2. Call `status_campaigns` to check current state
3. Re-dispatch any campaigns that were `running` but whose subagents are no longer active
4. Continue monitoring from Phase 4

Do not re-plan. Do not re-dispatch completed campaigns.

## Rules

These are non-negotiable:

1. **One subagent per campaign.** Never run two subagents on the same campaign. Never have one subagent handle multiple campaigns.
2. **Let /autoresearch handle experiments.** This skill orchestrates campaigns — it does not run experiments directly. Each subagent runs its own `/autoresearch` loop.
3. **Update the living document.** Every status change, dispatch, completion, and insight gets logged in `interlab-multi.md`. This is the coordination record.
4. **Don't modify campaign internals.** Never edit a running campaign's code, benchmark script, or JSONL. The subagent owns its campaign's working state.
5. **Disjoint file scopes.** Design campaigns so they don't fight over the same files. When overlap is unavoidable, serialize or add shared constraints.
6. **Cross-pollinate via ideas files.** If campaign A discovers something useful for campaign B, write it to B's `interlab.ideas.md` — don't modify B's code directly.
7. **Never ask to continue.** The orchestration loop runs until an exit condition fires. Trust the per-campaign circuit breakers and the global progress check.

## Common Mistakes

**Overlapping file scopes without coordination**
- Problem: Two campaigns edit the same file, creating merge conflicts or invalidating each other's results
- Fix: Map file ownership during Phase 2, serialize or split conflicting scopes

**Running experiments directly instead of delegating**
- Problem: This skill tries to edit code and run benchmarks itself
- Fix: This skill only orchestrates. Subagents running `/autoresearch` do the actual work.

**Ignoring cross-campaign interactions**
- Problem: Campaign A's optimization breaks campaign B's assumptions
- Fix: Check for interactions during Phase 4 monitoring, propagate insights via ideas files

**Re-dispatching completed campaigns**
- Problem: Wastes compute re-running campaigns that already converged
- Fix: Only re-dispatch if explicitly asked or if new information invalidates prior results

**Forgetting to update interlab-multi.md**
- Problem: Lose track of multi-campaign progress, duplicate work across sessions
- Fix: Update after every status check, dispatch, and completion
