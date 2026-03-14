# interlab Roadmap

**Version:** 0.3.0
**Last updated:** 2026-03-14
**Vision:** [`docs/interlab-vision.md`](interlab-vision.md)
**PRD:** [`docs/prds/2026-03-14-multi-campaign-orchestration.md`](prds/2026-03-14-multi-campaign-orchestration.md)

## Where We Are

**v0.3.0** — 7 MCP tools (3 experiment + 4 orchestration), 2 skills, 1 completed campaign, marketplace published.

**What's Working:**
- Single-campaign optimization loop (`/autoresearch`) — full init → run → log cycle with circuit breakers
- State reconstruction from JSONL: 68K ns for 100 entries (22x faster than v0.1 baseline)
- Campaign archival with structured learnings (`campaigns/<name>/`)
- Multi-campaign orchestration tools (plan/dispatch/status/synthesize) — shipped in v0.3
- File conflict detection at plan time between parallel campaigns
- Beads-backed coordination for multi-campaign dependency graphs
- SessionStart hook for campaign detection and resume prompts

**What's Not Working Yet:**
- Multi-campaign orchestration untested with real parallel runs (tools exist, no dogfood yet)
- Only `lower_is_better` / `higher_is_better` optimization — no comparative or validation experiments
- No agent self-improvement integration (interspect, interstat)
- Campaign learnings trapped in per-campaign files — no cross-campaign knowledge transfer
- `Demarch-g6i` (v0.1 epic) still open despite v0.3 being shipped

## Roadmap

### Now — v0.4: Battle-test + Compare (next release)

| Item | Description | Source |
|------|-------------|--------|
| Dogfood orchestration | Run `/autoresearch-multi` on a real broad goal, fix what breaks | Vision: battle-test |
| Comparative experiments | New experiment type: paired A/B runs with "which is better?" semantics | Vision: Gap 1 |
| Close stale epics | Close Demarch-g6i and any other stale interlab beads | Housekeeping |
| PHILOSOPHY.md refresh | Update from v0.1 goals to v0.3 reality + v1.0 vision | Docs drift |

### Next — v0.5–v0.6: Agent self-improvement + Learnings

| Item | Description | Source |
|------|-------------|--------|
| Benchmark adapters | Thin wrappers exposing agent metrics (latency, token usage, quality scores) as METRIC lines | Vision: Gap 2 |
| interspect integration | Feed agent performance evidence into interlab campaigns | Vision: Gap 2 |
| interstat integration | Use token/cost metrics as benchmark targets | Vision: Gap 2 |
| Validation experiments | New type: "did this change cause a regression?" with null hypothesis testing | Vision: Gap 1 |
| Cross-session learnings | Extract generalizable patterns from completed campaigns, feed into interknow | Vision: Gap 3 |
| Learnings discovery | Future campaigns auto-discover relevant past learnings before starting | Vision: Gap 3 |

### Later — v0.7–v1.0: Hardening + Ecosystem

| Item | Description | Source |
|------|-------------|--------|
| 5+ real multi-campaign runs | Battle-test orchestration at scale across different projects | Vision: v1.0 criteria |
| Agent self-optimization dogfood | At least one successful campaign where an agent optimized its own behavior | Vision: v1.0 criteria |
| Tournament experiments | Multi-option evaluation: "which of these N approaches wins?" | Vision: post-v1.0 |
| Statistical rigor | Formal significance testing for comparative experiments (t-test or similar) | Brainstorm: open question |
| Performance at scale | Validate JSONL model at 1000+ experiments per campaign | Vision: bet |

## Research Agenda

| Thread | Status | Key Question |
|--------|--------|-------------|
| Comparative experiment primitives | Open | Paired runs vs. interleaved? How many repetitions for confidence? |
| Agent metric measurability | Open | Which agent behaviors produce stable, measurable metrics? |
| Learnings transfer | Open | Tags vs. embeddings vs. keyword search for cross-campaign discovery? |
| Statistical significance | Open | Full t-test needed, or "A wins 7/10 paired runs" suffices? |

## Companion Status

| Plugin | Role | Status |
|--------|------|--------|
| beads | Coordination layer (parent/child beads, deps) | Integrated (v0.3) |
| intercore | Event emission (`ic events record`) | Integrated (v0.1) |
| Clavain | Skill host (`/autoresearch`, `/autoresearch-multi`) | Integrated (v0.1) |
| interspect | Agent performance evidence | Planned (v0.5) |
| interstat | Token/cost metrics | Planned (v0.5) |
| interknow | Cross-campaign learnings | Planned (v0.6) |

## Open Beads

| ID | Title | Priority | Status |
|----|-------|----------|--------|
| Demarch-g6i | interlab v0.1: autonomous experiment loop plugin | P1 | Open (stale — v0.1 shipped) |

*Note: Feature beads from v0.3 sprint (Demarch-yne9 through Demarch-anql, Demarch-meq9) are all closed. New beads for v0.4+ work haven't been created yet.*

## Keeping Current

Run `/interpath:roadmap interlab` to regenerate from current project state.
