---
artifact_type: prd
bead: Demarch-meq9
stage: design
---

# PRD: Multi-Campaign Orchestration

## Problem

interlab today handles one campaign at a time — one metric, one benchmark, one serial loop. When an agent wants to broadly optimize a module (e.g., "make interlab faster"), it must manually decompose the goal, run each campaign sequentially, and mentally synthesize results. This cognitive overhead limits interlab to narrow, pre-decomposed tasks.

## Solution

Add an orchestration layer on top of interlab's existing tools. Four new MCP tools handle campaign planning, parallel dispatch, progress monitoring, and result synthesis. A new `/autoresearch-multi` skill drives the full autonomous loop. Beads provide the coordination layer (dependency graphs, status, session attribution). Core tools (init/run/log_experiment) remain unchanged.

## Features

### F1: plan_campaigns tool

**What:** MCP tool that accepts a structured campaign plan from the agent and creates the orchestration infrastructure (beads, directories, benchmark scripts).

**Acceptance criteria:**
- [ ] Accepts JSON input: parent goal, list of campaign specs (name, metric, direction, benchmark_command, files_in_scope), and dependency edges between campaigns
- [ ] Creates a parent bead (epic) for the overall goal if one doesn't exist, or accepts an existing bead ID
- [ ] Creates child beads for each campaign with `bd dep add` for dependency ordering
- [ ] Creates working directories under `campaigns/<name>/` with initialized interlab.jsonl (config header written)
- [ ] Validates no files_in_scope overlap between campaigns that would run in parallel (fail-fast)
- [ ] Returns a structured plan summary: bead IDs, dependency graph, file assignments, estimated parallelism
- [ ] Stores plan metadata on the parent bead via `bd set-state` (campaign_count, campaign_ids, plan_timestamp)

### F2: dispatch_campaigns tool

**What:** MCP tool that reads the campaign plan and spawns execution agents for all unblocked campaigns.

**Acceptance criteria:**
- [ ] Reads parent bead state to discover child campaigns and their dependency status
- [ ] Identifies "ready" campaigns: no unmet dependencies (all blocking campaigns completed)
- [ ] For same-project campaigns: returns dispatch instructions for Claude subagents (campaign bead ID, working directory, benchmark command) — the calling agent spawns them
- [ ] For cross-project campaigns: returns dispatch instructions including project directory for session-level dispatch
- [ ] Marks dispatched campaigns as `in_progress` via `bd update --claim`
- [ ] Returns list of dispatched and waiting campaigns with reasons
- [ ] Idempotent: calling dispatch again after some campaigns complete dispatches newly unblocked ones

### F3: status_campaigns tool

**What:** MCP tool that reconstructs progress across all child campaigns from bead state and JSONL files.

**Acceptance criteria:**
- [ ] Reads parent bead to discover all child campaigns
- [ ] For each campaign: reads bead status (open/in_progress/closed) and JSONL state (run count, best metric, kept/discarded/crashed counts)
- [ ] Returns structured status: per-campaign progress, aggregate stats (total experiments, total kept, total improvement), dependency graph with completion markers
- [ ] Detects stale campaigns (in_progress but no new results for >30 minutes) and flags them
- [ ] Works even if some campaign directories don't exist yet (not all campaigns started)

### F4: synthesize_campaigns tool

**What:** MCP tool that reads completed campaign results and generates a structured cross-campaign report.

**Acceptance criteria:**
- [ ] Reads all child campaign results.jsonl files (from `campaigns/<name>/` directories)
- [ ] Extracts: baseline metric, final metric, improvement percentage, experiment count, top 3 changes per campaign
- [ ] Identifies cross-campaign patterns: did the same optimization technique work across campaigns? Did any campaign's improvements conflict with another's?
- [ ] Produces a structured JSON report with per-campaign summaries and cross-campaign insights
- [ ] Archives the synthesis to `campaigns/<parent-name>/synthesis.md`
- [ ] Closes the parent bead with a summary reason

### F5: /autoresearch-multi skill

**What:** Skill protocol that drives the full orchestration loop autonomously: analyze → plan → dispatch → monitor → synthesize.

**Acceptance criteria:**
- [ ] Agent analyzes the codebase to identify optimization targets (metrics, benchmarks, file scopes)
- [ ] Calls `plan_campaigns` with the decomposition
- [ ] Calls `dispatch_campaigns` to spawn parallel execution
- [ ] Polls `status_campaigns` until all campaigns complete or circuit breakers trip
- [ ] Calls `synthesize_campaigns` for the final report
- [ ] Handles partial success: if some campaigns finish with no improvement, reports them as "explored, no gain" rather than failures
- [ ] Resumes correctly if interrupted mid-orchestration (SessionStart hook detects parent bead)
- [ ] Creates interlab-multi.md living document tracking the overall orchestration state

### F6: File conflict detection

**What:** Validation at plan time that prevents parallel campaigns from modifying the same files.

**Acceptance criteria:**
- [ ] `plan_campaigns` validates files_in_scope across all campaigns that could run in parallel (no dependency between them)
- [ ] If overlap detected: returns error with the conflicting files and campaign names
- [ ] Suggests resolution: add a dependency edge between conflicting campaigns (forces serial execution) or partition the files differently
- [ ] Does not flag overlaps between campaigns that have a dependency (serial execution is safe)

## Non-goals

- **LLM-powered decomposition inside the Go binary** — the agent provides the decomposition, not the tool
- **Dynamic re-planning** — if a campaign discovers new optimization targets mid-run, the agent can plan a follow-up, but the orchestration tools don't auto-expand the plan
- **Cross-machine distribution** — all execution happens on the same machine, just in parallel sessions/subagents
- **Beads-free fallback** — this iteration requires beads. Environments without beads use single-campaign /autoresearch

## Dependencies

- beads CLI (`bd`) — coordination layer
- interlock plugin — file reservation for cross-project campaigns (optional, for safety)
- intermux plugin — session-level parallelism for cross-project campaigns (optional)
- Existing interlab tools (init/run/log_experiment) — unchanged, campaigns use them directly

## Open Questions

1. **Budget allocation** — Each campaign gets its own full circuit breaker budget (default 50 experiments). The parent doesn't impose a global limit. This keeps campaigns independent. If needed later, a `max_total_experiments` field on the plan can cap aggregate work.

2. **Partial success semantics** — Synthesis treats each campaign independently. "3 of 5 improved" is a valid outcome. The synthesis report shows per-campaign results without an aggregate pass/fail.

3. **Campaign priority within the DAG** — Follow dependency order. Among independent campaigns, dispatch all at once (maximize parallelism). No priority-based ordering — all campaigns are equally important within the plan.
