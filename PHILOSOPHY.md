# interlab — Philosophy

## Vision

interlab is the **empirical backbone of Demarch** — the system any agent uses to answer "did this change actually help?" with rigor and automation. It serves three roles: optimization engine, research infrastructure, and agent self-improvement layer. See [`docs/interlab-vision.md`](docs/interlab-vision.md) for the full vision.

## Design Principles

- **Stateless tools, file-based state.** Every tool call reconstructs state from JSONL. No server-side state machine. Crash recovery is free — any agent can continue any campaign by reading the file.
- **LLM drives, plugin guards.** The intelligence lives in the agent and the skill protocol. interlab provides "dumb tools + smart guards" — circuit breakers, path scoping, metric consistency. The agent decides what to optimize; the tools ensure experiments are safe.
- **Mechanism, not policy.** interlab is domain-agnostic infrastructure. It doesn't know what you're optimizing, what language you're writing, or what metrics matter. That's the skill's job. One plugin serves unlimited domains.
- **Empirical over theoretical.** When in doubt, measure it. interlab biases toward running experiments rather than debating approaches. The circuit breaker is the safety net — not human approval gates.
- **Graceful degradation.** Missing `bd` CLI? Skip bead coordination. Missing `ic`? Skip event emission. Git fails? Log and continue. The experiment data in JSONL is always the source of truth.
- **Path-scoped safety.** Never `git add -A`. The plugin enforces that only declared `files_in_scope` are staged. This is a structural safety guarantee, not a policy request.

## Milestones

- **v0.1** (shipped): 3 stateless MCP tools, JSONL persistence, `/autoresearch` skill, first dogfood campaign (22x ReconstructState speedup)
- **v0.2** (shipped): working_directory fix (discovered via dogfooding), campaign archival with structured learnings
- **v0.3** (shipped): 4 orchestration tools (plan/dispatch/status/synthesize), `/autoresearch-multi` skill, beads-backed coordination, file conflict detection
- **v1.0** (target): Any agent can optimize, compare, or validate empirically. Agents can self-improve. Learnings compound across campaigns.

## Tradeoffs

Explicit bets we're making:

- **JSONL over SQLite** — Simpler, appendable, human-readable, version-controllable. Costs: no random access, O(n) reconstruction. Bet: campaigns stay <1000 experiments, and reconstruction is fast (68K ns for 100 entries after optimization).
- **Beads-backed orchestration over bespoke manifest** — Multi-campaign coordination uses beads (parent/child beads, dependencies). Costs: couples interlab to beads. Bet: beads is part of the Demarch platform and provides dependency graphs, status tracking, and session attribution for free.
- **Experiment branches over trunk commits** — Isolates experiment noise from main history. Squash-merge on completion. Costs: documented exception to trunk-based development.
- **Best-effort external integration** — Plugin works without `ic`, `bd`, or any external CLI. Costs: no events/coordination if tools are missing. Bet: most value comes from JSONL + git, not the event pipeline.
- **Single benchmark command over structured test harness** — Keep it simple: `bash -c <command>`, parse METRIC lines from stdout. Costs: benchmarks must be self-contained scripts. Bet: this is flexible enough for 90% of use cases.
