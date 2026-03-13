---
artifact_type: brainstorm
bead: none
stage: discover
---
# interlab: Initial Goals

## v0.1 Release

Ship interlab as an installable Interverse plugin with all 3 tools working end-to-end:
- `init_experiment` — campaign setup with JSONL config header and experiment branch
- `run_experiment` — benchmark execution with circuit breaker and METRIC parsing
- `log_experiment` — keep/discard/crash with path-scoped git and ic events

## Dogfood Campaign

Run interlab to optimize itself — prove the loop works with real experiments:
- Candidate metrics: JSONL write throughput, state reconstruction latency, plugin startup time
- Validates: full init→run→log cycle, circuit breaker, living document pattern, ideas backlog
- Produces: real `interlab.md` + `interlab-learnings.md` with experiment history

## Clavain Integration

Make `/autoresearch` skill available for any Demarch agent:
- SKILL.md encodes the loop protocol, living-document template, ideas backlog pattern
- SessionStart hook detects active campaigns and prompts resume
- Any agent with interlab installed can run autonomous optimization loops

## Key Design Principles

- **Stateless tools + file-based state** — crash recovery for free
- **LLM drives, plugin guards** — intelligence in the skill, safety in the tools
- **Mechanism, not policy** — domain-agnostic infrastructure
- **Path-scoped safety** — structural git safety guarantee
