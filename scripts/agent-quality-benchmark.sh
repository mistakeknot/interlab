#!/usr/bin/env bash
# agent-quality-benchmark.sh — Score an agent .md file on structural quality.
# Emits METRIC lines compatible with interlab's experiment loop.
#
# Usage: bash agent-quality-benchmark.sh <agent.md>
# Output: METRIC agent_quality_score=N.NNNN (0.0 to 1.0)
set -uo pipefail

if [[ $# -lt 1 ]]; then
    echo "Usage: agent-quality-benchmark.sh <agent.md>" >&2
    exit 1
fi

AGENT_FILE="$1"
if [[ ! -f "$AGENT_FILE" ]]; then
    echo "Error: file not found: $AGENT_FILE" >&2
    exit 1
fi

total_checks=0
passed_checks=0

pass() { total_checks=$((total_checks + 1)); passed_checks=$((passed_checks + 1)); }
fail() { total_checks=$((total_checks + 1)); }

line_count=$(wc -l < "$AGENT_FILE")

# --- Structural Quality (6 checks) ---

# 1. Has YAML frontmatter (---)
head -1 "$AGENT_FILE" | grep -q '^---' && pass || fail

# 2. Has description field in frontmatter
sed -n '1,/^---$/p' "$AGENT_FILE" | grep -qi 'description:' && pass || fail

# 3. Has when_to_use or trigger section
grep -qiE '(when.to.use|trigger|use.*when|examples)' "$AGENT_FILE" && pass || fail

# 4. Has tools or allowed-tools section
grep -qiE '(tools:|allowed.tools|Tools:)' "$AGENT_FILE" && pass || fail

# 5. Under 500 lines
[[ "$line_count" -le 500 ]] && pass || fail

# 6. Has markdown headings (structure)
heading_count=$(grep -c '^#' "$AGENT_FILE" 2>/dev/null || echo 0)
[[ "$heading_count" -ge 2 ]] && pass || fail

# --- Prompt Quality (4 checks) ---

# 7. Has examples section with concrete examples
grep -qiE '(example|<example>)' "$AGENT_FILE" && pass || fail

# 8. No vague instructions ("as needed", "if appropriate", "consider")
vague_count=$(grep -ciE '(as needed|if appropriate|consider doing|you might want|perhaps)' "$AGENT_FILE" 2>/dev/null || true)
[[ -z "$vague_count" ]] && vague_count=0
[[ "$vague_count" -le 2 ]] && pass || fail

# 9. Has clear output format specification
grep -qiE '(output|return|respond|format|result)' "$AGENT_FILE" && pass || fail

# 10. Has scope boundary (what NOT to do)
grep -qiE '(do not|don.t|never|avoid|not for|skip)' "$AGENT_FILE" && pass || fail

# --- Completeness (3 checks) ---

# 11. Non-trivial content (>20 lines)
[[ "$line_count" -ge 20 ]] && pass || fail

# 12. Has system prompt or role definition
grep -qiE '(you are|your role|system.prompt|agent.*that|specialized)' "$AGENT_FILE" && pass || fail

# 13. References specific file paths or code patterns
grep -qE '(/|\.go|\.py|\.ts|\.md|\.sh|\.yaml)' "$AGENT_FILE" && pass || fail

# Compute composite score
if [[ "$total_checks" -gt 0 ]]; then
    score=$(awk "BEGIN {printf \"%.4f\", $passed_checks / $total_checks}")
else
    score="0.0000"
fi

echo "METRIC agent_quality_score=$score"
echo "METRIC agent_quality_passed=$passed_checks"
echo "METRIC agent_quality_total=$total_checks"
