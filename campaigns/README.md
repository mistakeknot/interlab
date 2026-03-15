# Completed Campaigns

Archived experiment campaigns. Each directory contains:

- `results.jsonl` — full experiment log (config header + result entries)
- `learnings.md` — validated insights, dead ends, and generalizable patterns

## Index

| Campaign | Date | Metric | Before | After | Change |
|----------|------|--------|--------|-------|--------|
| [interlab-reconstruct-speed](interlab-reconstruct-speed/) | 2026-03-13 | reconstruct_100_ns | 1,540K ns | 68K ns | -96% (22x) |
| [interflux-self-review](interflux-self-review/) | 2026-03-15 | agent_quality_score | — | — | pilot (ready) |

## Multi-Plugin Improvement

Scan all plugins, find the lowest-scoring, and run parallel improvement campaigns:

```bash
# 1. Scan all plugins for quality scores (from monorepo root)
bash interverse/interlab/scripts/scan-plugin-quality.sh

# 2. Generate campaign spec for bottom 5
bash interverse/interlab/scripts/generate-campaign-spec.sh --top=5 > /tmp/campaigns.json

# 3. Launch via /autoresearch-multi with the generated spec
# Pass /tmp/campaigns.json to plan_campaigns MCP tool
```

The mutation store (v0.4.0) automatically records approach provenance via `/autoresearch` integration.
Task type for all plugin campaigns: `plugin-quality` (queryable cross-campaign).
