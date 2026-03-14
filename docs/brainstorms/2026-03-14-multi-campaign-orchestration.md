---
artifact_type: brainstorm
bead: Demarch-meq9
stage: discover
---

# Multi-Campaign Orchestration for interlab

## What We're Building

Evolve interlab from a single-experiment loop into a multi-campaign orchestrator that can take a broad optimization request (e.g., "make this module faster"), decompose it into multiple focused campaigns, dispatch them in parallel via subagents, and synthesize results into a unified report.

**Use cases (priority order):**

1. **Optimize a whole module** — "make interlab faster" decomposes into reconstruct latency, write throughput, parse speed, startup time campaigns
2. **Cross-project campaigns** — run experiments across different projects/modules, synthesize into a unified report
3. **Research exploration** — "is streaming faster than batching?" becomes multiple experiments testing the hypothesis from different angles
4. **Multi-metric single target** — already partially supported via secondary metrics, but orchestration enables true multi-objective optimization

## Why This Approach

### Agent-driven decomposition

The calling agent (Claude) does the thinking — analyzes the codebase, identifies optimization targets, selects metrics. interlab provides the orchestration infrastructure (campaign creation, dispatch, monitoring, synthesis). This keeps the MCP tools simple and testable while letting the agent's intelligence drive strategy.

**Rationale:** Tool-driven decomposition would require embedding LLM calls or complex heuristics inside the Go binary, making it harder to test and maintain. Skill-driven decomposition (markdown protocol only) would work but can't provide structured coordination across campaigns.

### Beads-backed orchestration

Use beads as the coordination layer instead of inventing a new manifest format:

- **Parent bead** = the broad optimization request (epic)
- **Child beads** = individual campaigns (features/tasks)
- **Dependencies** = `bd dep add` for serial ordering
- **Status** = bead state (open → in_progress → closed)
- **Session attribution** = interstat tracks tokens per campaign bead

**Rationale:** Beads already handle dependency graphs, status tracking, session attribution, and parallel work management. Building a new JSONL manifest would duplicate this infrastructure. The coupling to beads is acceptable because interlab is a Demarch plugin — beads is part of the platform.

### True parallel execution

- **Same-project campaigns with non-overlapping files:** Dispatch via Claude subagents (Agent tool). Each subagent runs a standard /autoresearch loop against its campaign bead.
- **Cross-project campaigns:** Dispatch via session-level parallelism (intermux/interlock). Full isolation — each session operates in its own project directory.
- **Campaigns with shared files:** Serial execution with dependency ordering via `bd dep add`.

**Rationale:** Interleaved serial execution is simpler but loses the performance benefit. The user prioritized true parallelism. Subagents for same-project work is the natural Claude Code pattern; session-level parallelism extends to cross-project work.

## Key Decisions

1. **Core tools unchanged** — init_experiment, run_experiment, log_experiment stay as-is. The orchestration is a new layer on top, not a rewrite. Each sub-campaign uses standard interlab.jsonl in its own working directory.

2. **New MCP tools for orchestration:**
   - `plan_campaigns` — Agent provides the decomposition (campaign specs + dependency graph), tool creates beads and working directories
   - `dispatch_campaigns` — Spawns subagents or sessions for each ready campaign
   - `status_campaigns` — Reconstructs progress across all child campaigns from bead state + JSONL files
   - `synthesize_campaigns` — Reads completed campaign results and generates a unified report

3. **Campaign isolation** — Each sub-campaign gets its own subdirectory with its own interlab.jsonl, interlab.md, and interlab.ideas.md. No shared JSONL state between campaigns.

4. **Synthesis is structured** — The synthesize tool reads all child campaign results.jsonl files, extracts key metrics and learnings, and produces a structured report. The agent can then write the narrative interpretation.

5. **Beads are the source of truth for coordination** — Campaign topology, dependencies, status, and session attribution all live in beads. interlab only owns the experiment data (JSONL).

6. **New skill: /autoresearch-multi** — Drives the full orchestration loop: analyze codebase → plan campaigns → dispatch → monitor → synthesize. The existing /autoresearch skill continues to work for single campaigns.

## Open Questions

1. **File conflict detection** — How to prevent two parallel campaigns from modifying the same file? Options: (a) fail-fast at plan time if files_in_scope overlap, (b) use interlock reservations, (c) trust the agent to partition correctly.

2. **Budget allocation** — Should the orchestrator split the circuit breaker budget across campaigns (e.g., 50 total experiments / 5 campaigns = 10 each), or give each campaign its own full budget?

3. **Campaign priority** — When campaigns have dependencies, should the orchestrator run high-impact campaigns first? Or just follow the dependency DAG?

4. **Partial success** — If 3 of 5 campaigns complete but 2 hit circuit breakers with no improvement, what does the synthesis report look like? Is this a success or failure?

5. **Cross-session resume** — If a multi-campaign orchestration is interrupted, how does a new session discover and resume it? The SessionStart hook detects single campaigns via interlab.md — does it need to detect multi-campaign plans too?

6. **beads coupling** — Should there be a fallback for environments without beads? Or is this strictly a Demarch-ecosystem feature?
