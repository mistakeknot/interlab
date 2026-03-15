# Agent Quality Scoring Rubric

**Script:** `scripts/agent-quality-benchmark.sh`
**Score Range:** 0.0 to 1.0
**Direction:** higher_is_better

## Sub-scores (13 checks, equal weight)

### Structural Quality (6/13)
1. YAML frontmatter present
2. Description field in frontmatter
3. When-to-use / trigger section
4. Tools or allowed-tools declaration
5. Under 500 lines
6. Has 2+ markdown headings

### Prompt Quality (4/13)
7. Contains examples
8. Minimal vague language (<=2 instances of "as needed", "consider", etc.)
9. Output format specification
10. Scope boundaries (what NOT to do)

### Completeness (3/13)
11. Non-trivial content (>20 lines)
12. Role/identity definition
13. References specific file paths or code patterns

## Interpretation

| Score | Quality |
|-------|---------|
| >= 0.85 | Excellent — production-ready agent |
| 0.70 - 0.84 | Good — minor improvements possible |
| 0.50 - 0.69 | Needs work — missing key sections |
| < 0.50 | Poor — significant structural gaps |
