# interlab — Vision

**Last updated:** 2026-03-14
**PRD:** [`docs/prds/2026-03-14-multi-campaign-orchestration.md`](prds/2026-03-14-multi-campaign-orchestration.md)
**Roadmap:** [`docs/interlab-roadmap.md`](interlab-roadmap.md)

## The Big Idea

interlab is the **empirical backbone of Demarch** — the system any agent uses to answer "did this change actually help?" with rigor and automation. It makes the scientific method available as infrastructure: form a hypothesis, run experiments, measure results, keep what works, discard what doesn't. Whether you're optimizing nanoseconds off a hot path, comparing two architectural approaches, or training an agent to improve its own behavior — interlab provides the experiment loop, the safety guards, and the institutional memory.

## Design Principles

- **Stateless tools, file-based state.** Every tool call reconstructs state from JSONL. No server-side state machine. Crash recovery is free — any agent can continue any campaign by reading the file.

- **LLM drives, plugin guards.** The intelligence lives in the agent and the skill protocol. interlab provides "dumb tools + smart guards" — circuit breakers, path-scoped git safety, metric consistency. The agent decides what to optimize; the tools ensure experiments are safe.

- **Mechanism, not policy.** interlab is domain-agnostic infrastructure. It doesn't know what you're optimizing, what language you're writing, or what metrics matter. That's the skill's job. One plugin serves unlimited domains.

- **Empirical over theoretical.** When in doubt, measure it. interlab biases toward running experiments rather than debating approaches. The circuit breaker is the safety net — not human approval gates.

- **Learnings compound.** Individual experiments produce isolated results. interlab's job is to turn those results into institutional knowledge that makes future experiments smarter and faster.

## Current State

**Version:** 0.3.0 (published to Interverse marketplace)
**Maturity:** Production-ready for single-campaign optimization. Multi-campaign orchestration tools shipped but not yet battle-tested.

**Components:**
- 7 MCP tools (3 experiment + 4 orchestration)
- 2 skills (`/autoresearch`, `/autoresearch-multi`)
- SessionStart hook for campaign detection
- Campaign archival with structured learnings

**Milestones achieved:**
- v0.1: Full experiment loop with circuit breakers, JSONL persistence, git isolation
- v0.1 dogfood: 22x speedup on ReconstructState (1.54ms → 0.068ms, 10 experiments, 7 kept)
- v0.2: working_directory bug fix (discovered via dogfooding), campaign archival
- v0.3: Multi-campaign orchestration (plan/dispatch/status/synthesize), beads-backed coordination, file conflict detection

## Where We're Going

### Near-term: Battle-test orchestration (v0.4)

The v0.3 orchestration tools exist but haven't been proven with real multi-campaign runs. Before adding features, validate that `plan_campaigns` → `dispatch_campaigns` → `status_campaigns` → `synthesize_campaigns` works end-to-end on a real broad optimization goal. Fix whatever breaks.

Also: add **comparative experiments** — the ability to answer "is A better than B?" with paired runs rather than just "make metric go down." This unlocks the research-infrastructure use case.

### Medium-term: Agent self-improvement (v0.5–v0.6)

Build benchmark adapters that expose agent-level metrics as METRIC lines interlab can consume. An agent should be able to say "optimize my prompt for this task" or "benchmark my tool response time" and have interlab run the loop. Integration points: interspect (performance evidence), interstat (token usage), Clavain (sprint metrics).

Also: **cross-session intelligence** — extract generalizable patterns from completed campaigns and feed them into future campaigns via interknow. "Byte scanning beats JSON parsing" shouldn't be rediscovered in every codebase.

### Long-term: Empirical backbone (v0.7–v1.0)

interlab v1.0 is reached when:
1. Any Demarch agent can invoke interlab to answer an empirical question — optimize, compare, or validate
2. At least one agent has successfully optimized its own behavior through interlab
3. Learnings from past campaigns feed into future ones automatically
4. The tools are battle-tested across 5+ real multi-campaign orchestrations

## Constellation

interlab doesn't exist in isolation. It connects to the Demarch ecosystem:

| Plugin | Relationship | Status |
|--------|-------------|--------|
| **beads** | Coordination layer for multi-campaign orchestration (parent/child beads, dependencies) | Integrated (v0.3) |
| **intercore** | Event emission via `ic events record` for experiment outcomes | Integrated (v0.1, best-effort) |
| **interspect** | Future: agent performance evidence as experiment input | Planned (v0.5) |
| **interstat** | Future: token/cost metrics as benchmark targets | Planned (v0.5) |
| **interknow** | Future: cross-campaign learnings storage and retrieval | Planned (v0.6) |
| **Clavain** | Skill host (`/autoresearch`, `/autoresearch-multi`) | Integrated (v0.1) |

## What We Believe

Explicit bets that, if wrong, would change direction:

- **JSONL scales to v1.0.** Campaigns stay under 1000 experiments. O(n) reconstruction (now 68K ns for 100 entries) is fast enough. If campaigns grow to 10K+ experiments, we'd need an index or incremental state.

- **Agents can self-improve through interlab.** The hypothesis is that agent behavior (prompt quality, tool selection, response latency) is measurable and optimizable via the same experiment loop used for code. If agent metrics prove too noisy or non-deterministic, this bet fails.

- **Beads-backed orchestration is sufficient.** We chose beads over a bespoke manifest format for multi-campaign coordination. If beads becomes a bottleneck (too slow, wrong abstraction, coupling issues), we'd need to revisit.

- **Learnings compound.** We believe that optimization patterns transfer across codebases and domains. "Byte scanning beats JSON parsing" should help any Go project. If learnings are too context-specific to transfer, the cross-session intelligence feature has less value.

- **Three experiment types cover 90% of use cases.** Optimize (make metric go up/down), compare (is A better than B?), and validate (did this regress?) should handle most empirical questions. Tournament-style multi-option evaluation is post-v1.0.
