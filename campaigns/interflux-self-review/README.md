# Campaign: interflux-self-review

**Task Type:** `agent-quality`
**Target:** interflux review agent `.md` definitions
**Metric:** `agent_quality_score` (0.0 - 1.0, higher is better)
**Benchmark:** `bash scripts/agent-quality-benchmark.sh <agent.md>`

## Purpose

flux-drive review agents review each other's `.md` definitions for structural quality, prompt clarity, and completeness. This is the first meta-improvement campaign — the review system reviewing and improving itself.

## How to Run

```bash
# From interverse/interflux/ directory:
# 1. Pick a target agent
TARGET="agents/review/fd-architecture.md"

# 2. Launch campaign via /autoresearch
# Metric: agent_quality_score
# Direction: higher_is_better
# Benchmark: bash ../interlab/scripts/agent-quality-benchmark.sh $TARGET
```

## Target Files

All agent definitions in `interverse/interflux/agents/review/`:
- fd-architecture.md
- fd-safety.md
- fd-correctness.md
- fd-quality.md
- fd-user-product.md
- fd-performance.md
- fd-game-design.md
- fd-systems.md
- fd-decisions.md
- fd-people.md
- fd-resilience.md
- fd-perception.md

## Expected Improvements

- Better YAML frontmatter (description accuracy, trigger specificity)
- Clearer when-to-use examples matching real usage patterns
- More specific output format requirements
- Removal of vague language ("consider", "as needed")
- Addition of scope boundaries (what NOT to review)
