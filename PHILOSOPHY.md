# interlab — Philosophy

## Design Principles

- **Stateless tools, file-based state.** Every tool call reconstructs state from JSONL. No server-side state machine. This gives crash recovery for free and lets any agent continue any campaign.
- **LLM drives, plugin guards.** The intelligence is in the skill (SKILL.md) and the LLM's reasoning. The plugin provides "dumb tools + smart guards" — circuit breakers, path scoping, metric consistency.
- **Mechanism, not policy.** interlab is domain-agnostic infrastructure. It doesn't know what you're optimizing — that's the skill's job. One plugin serves unlimited optimization domains.
- **Graceful degradation.** Missing `ic` CLI? Skip event emission. Git fails? Log the error, continue. The experiment data in JSONL is always the source of truth.
- **Path-scoped safety.** Never `git add -A`. The plugin enforces that only declared `files_in_scope` are staged. This is a structural safety guarantee, not a policy request.

## Key Goals

- **v0.1 release:** Publish as an installable Interverse plugin with all 3 tools working end-to-end
- **Dogfood campaign:** Run interlab to optimize itself (JSONL write throughput or state reconstruction latency) — prove the loop works with real experiments
- **Clavain integration:** `/autoresearch` skill available for any Demarch agent to invoke

## Tradeoffs

Explicit bets we're making:

- **JSONL over SQLite** — Simpler, appendable, human-readable, version-controllable. Costs: no random access, O(n) reconstruction. Bet: campaigns are <1000 experiments, so linear scan is fine.
- **Experiment branches over trunk commits** — Isolates experiment noise from main history. Squash-merge on completion. Costs: documented exception to trunk-based development.
- **Best-effort ic integration over hard dependency** — Plugin works without intercore. Costs: no events emitted if ic is missing. Bet: most value comes from JSONL + git, not the event pipeline.
- **Single benchmark command over structured test harness** — Keep it simple: `bash -c <command>`, parse METRIC lines from stdout. Costs: benchmarks must be self-contained scripts. Bet: this is flexible enough for 90% of use cases.
