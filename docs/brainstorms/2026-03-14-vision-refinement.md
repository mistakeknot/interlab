---
artifact_type: brainstorm
bead: none
stage: discover
---

# interlab Vision Refinement

## What interlab Becomes

interlab is the **empirical backbone of Demarch** — the system any agent uses to answer "did this change actually help?" with rigor and automation. It serves three roles:

1. **Optimization engine** — the standard way to make anything faster, smaller, cheaper. Every metric-driven improvement flows through interlab.
2. **Research infrastructure** — general-purpose experiment runner for hypothesis testing, A/B comparisons, and statistical validation. Not just "make metric go down" but "is approach A better than B?"
3. **Agent self-improvement layer** — how agents improve themselves. Benchmark your own tools, optimize your own prompts, evolve your own performance. The self-referential loop is the point.

## What's Been Built (v0.1–v0.3)

- **v0.1**: 3 stateless MCP tools (init/run/log_experiment), JSONL persistence, /autoresearch skill, SessionStart hooks. First dogfood: 22x ReconstructState speedup.
- **v0.2**: working_directory fix (discovered via dogfooding), campaign archival to campaigns/, marketplace publish.
- **v0.3**: 4 orchestration tools (plan/dispatch/status/synthesize_campaigns), /autoresearch-multi skill, beads-backed coordination, file conflict detection.

## Gaps to Close for v1.0

### Gap 1: Non-optimization experiments
Current limitation: interlab only supports `lower_is_better` / `higher_is_better` metric optimization. But research questions often look different:
- "Is streaming faster than batching?" (comparative A/B)
- "Does this change cause a regression?" (null hypothesis testing)
- "Which of these 3 approaches performs best?" (tournament)

**Needed:** New experiment types beyond optimize-a-metric. Comparative experiments (paired runs), hypothesis testing with statistical significance, tournament-style multi-option evaluation.

### Gap 2: Agent self-improvement hooks
No agent can currently say "optimize my own response latency" and have interlab do it. The gap is integration with agent introspection systems:
- **interspect** — agent performance evidence (who's good at what)
- **interstat** — token usage, tool patterns, cost per task
- **Clavain** — sprint metrics, time-to-complete, defect rates

**Needed:** Benchmark adapters that can measure agent-level metrics (not just code metrics). A skill that wraps agent introspection data into METRIC lines interlab can consume.

### Gap 3: Cross-session intelligence
Campaign learnings are trapped in per-campaign `learnings.md` files. No mechanism feeds insights back:
- "Byte scanning beats JSON parsing" (from reconstruct-speed campaign) should inform future optimization campaigns
- "This code path is already at the noise floor" should prevent wasted effort
- Optimization techniques that work across codebases should be surfaced as patterns

**Needed:** A learnings aggregation layer. After synthesis, extract generalizable patterns and store them where future campaigns can discover them (probably via interknow or a dedicated learnings index).

## v1.0 Definition

interlab v1.0 is reached when:
1. **Any Demarch agent** can invoke interlab to answer an empirical question — not just "optimize X" but "is A better than B?", "did this regress?", "which approach wins?"
2. **Agents can improve themselves** — at least one successful campaign where an agent optimized its own behavior (prompt, tool selection, or performance metric)
3. **Learnings compound** — results from past campaigns feed into future ones, preventing repeated work and surfacing cross-cutting patterns
4. **The tools are battle-tested** — at least 5 completed multi-campaign orchestrations with real results, proving the v0.3 orchestration layer works at scale

## Key Decisions

1. **Three experiment types at v1.0**: optimize (existing), compare (A/B), and validate (regression/hypothesis) — tournament is post-v1.0
2. **Agent self-improvement via benchmark adapters** — thin wrappers that expose agent metrics as METRIC lines, keeping interlab's core domain-agnostic
3. **Learnings feed into interknow** — the existing knowledge management plugin, rather than building a bespoke learnings index
4. **v1.0 is about breadth, not depth** — each capability needs to work end-to-end but doesn't need to be perfect. Refinement is post-v1.0
5. **Milestone-based roadmap** — v0.4 (compare experiments), v0.5 (agent self-improvement), v0.6 (cross-session learnings), v0.7-v0.9 (hardening + dogfooding), v1.0 (all three gaps closed + battle-tested)

## Open Questions

1. **Statistical significance** — how rigorous does "is A better than B?" need to be? Full t-test, or "A wins 7/10 paired runs" suffices?
2. **Agent benchmark scope** — which agent metrics are measurable today vs. need new instrumentation?
3. **Learnings format** — what structure makes learnings discoverable by future campaigns? Tags? Embeddings? Simple keyword search?
