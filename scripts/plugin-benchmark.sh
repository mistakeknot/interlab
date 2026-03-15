#!/usr/bin/env bash
set -euo pipefail
# plugin-benchmark.sh — Run structural tests + deterministic audit on a plugin.
# Emits METRIC lines to stdout for interlab consumption.
# All non-metric output goes to stderr.
#
# Usage: bash interverse/interlab/scripts/plugin-benchmark.sh interverse/interlock/
#
# Must be run from the monorepo root.

info() { echo "[plugin-benchmark] $*" >&2; }
warn() { echo "[plugin-benchmark] WARN: $*" >&2; }

PLUGIN_DIR="${1:?Usage: plugin-benchmark.sh <plugin-dir>}"

# Normalize: strip trailing slash, resolve relative to monorepo root
PLUGIN_DIR="${PLUGIN_DIR%/}"

if [[ ! -d "$PLUGIN_DIR" ]]; then
    warn "Plugin directory not found: $PLUGIN_DIR"
    echo "METRIC plugin_quality_score=0"
    echo "METRIC structural_tests_pass=0"
    echo "METRIC structural_tests_total=0"
    echo "METRIC build_passes=0"
    echo "METRIC audit_score=0"
    echo "METRIC audit_max=19"
    exit 0
fi

PLUGIN_NAME=$(basename "$PLUGIN_DIR")
info "Benchmarking plugin: $PLUGIN_NAME ($PLUGIN_DIR)"

# ─── Phase 1: Structural tests ───────────────────────────────────────────────

structural_pass=0
structural_total=0

STRUCT_TEST="$PLUGIN_DIR/tests/structural/test_structure.py"
if [[ -f "$STRUCT_TEST" ]]; then
    info "Running structural tests..."
    PYPROJECT="$PLUGIN_DIR/tests/pyproject.toml"
    if [[ -f "$PYPROJECT" ]]; then
        # Run pytest with machine-readable output, capture results
        PYTEST_OUTPUT=$(cd "$PLUGIN_DIR/tests" && uv run pytest -q --tb=no 2>&1) || true
        info "Pytest output:"
        echo "$PYTEST_OUTPUT" >&2

        # Parse pytest summary line: "N passed, M failed" or "N passed"
        if echo "$PYTEST_OUTPUT" | grep -qE '[0-9]+ passed'; then
            structural_pass=$(echo "$PYTEST_OUTPUT" | grep -oE '[0-9]+ passed' | grep -oE '[0-9]+')
        fi
        failed=0
        if echo "$PYTEST_OUTPUT" | grep -qE '[0-9]+ failed'; then
            failed=$(echo "$PYTEST_OUTPUT" | grep -oE '[0-9]+ failed' | grep -oE '[0-9]+')
        fi
        errors=0
        if echo "$PYTEST_OUTPUT" | grep -qE '[0-9]+ error'; then
            errors=$(echo "$PYTEST_OUTPUT" | grep -oE '[0-9]+ error' | grep -oE '[0-9]+')
        fi
        structural_total=$(( structural_pass + failed + errors ))
    else
        warn "No pyproject.toml found, running pytest directly..."
        PYTEST_OUTPUT=$(cd "$PLUGIN_DIR/tests" && python3 -m pytest structural/ -q --tb=no 2>&1) || true
        echo "$PYTEST_OUTPUT" >&2
        if echo "$PYTEST_OUTPUT" | grep -qE '[0-9]+ passed'; then
            structural_pass=$(echo "$PYTEST_OUTPUT" | grep -oE '[0-9]+ passed' | grep -oE '[0-9]+')
        fi
        failed=0
        if echo "$PYTEST_OUTPUT" | grep -qE '[0-9]+ failed'; then
            failed=$(echo "$PYTEST_OUTPUT" | grep -oE '[0-9]+ failed' | grep -oE '[0-9]+')
        fi
        errors=0
        if echo "$PYTEST_OUTPUT" | grep -qE '[0-9]+ error'; then
            errors=$(echo "$PYTEST_OUTPUT" | grep -oE '[0-9]+ error' | grep -oE '[0-9]+')
        fi
        structural_total=$(( structural_pass + failed + errors ))
    fi
else
    info "No structural tests found at $STRUCT_TEST"
fi

echo "METRIC structural_tests_pass=$structural_pass"
echo "METRIC structural_tests_total=$structural_total"

# ─── Phase 2: Build correctness ──────────────────────────────────────────────

build_passes=0

if [[ -f "$PLUGIN_DIR/go.mod" ]]; then
    info "Detected Go plugin, running go test..."
    if (cd "$PLUGIN_DIR" && go test ./... -count=1 -timeout=60s) >&2 2>&1; then
        build_passes=1
        info "Go tests passed"
    else
        warn "Go tests failed"
    fi
elif [[ -f "$PLUGIN_DIR/pyproject.toml" ]]; then
    info "Detected Python plugin, running uv run pytest..."
    if (cd "$PLUGIN_DIR" && uv run pytest -q --tb=short) >&2 2>&1; then
        build_passes=1
        info "Python tests passed"
    else
        warn "Python tests failed"
    fi
elif [[ -f "$PLUGIN_DIR/package.json" ]]; then
    info "Detected Node plugin, running npm test..."
    if (cd "$PLUGIN_DIR" && npm test --if-present) >&2 2>&1; then
        build_passes=1
        info "Node tests passed"
    else
        warn "Node tests failed"
    fi
else
    # No recognized build system — count as pass (plugin may be pure config/scripts)
    info "No recognized build system, treating build as pass"
    build_passes=1
fi

echo "METRIC build_passes=$build_passes"

# ─── Phase 3: Deterministic audit (19-point interskill checklist) ─────────────
#
# Structure (6): YAML frontmatter, name, description, <500 lines, one-level refs, compact
# Invocation Control (4): disable-model-invocation, user-invocable, allowed-tools, context
# Content Quality (5): markdown headings, triggers, examples, Quick Start, no vague
# Anti-Patterns (4): no XML, no deep nesting, no punting, no broad descriptions
#
# We check what can be verified deterministically. Items requiring LLM judgment
# (e.g., "examples are concrete", "no vague guidance") are skipped and counted
# as neither pass nor fail — reducing audit_max accordingly.

audit_score=0
audit_max=0

# Collect all SKILL.md files in the plugin
SKILL_FILES=()
if [[ -d "$PLUGIN_DIR/skills" ]]; then
    while IFS= read -r -d '' f; do
        SKILL_FILES+=("$f")
    done < <(find "$PLUGIN_DIR/skills" -name "SKILL.md" -print0 2>/dev/null)
fi

if [[ ${#SKILL_FILES[@]} -eq 0 ]]; then
    info "No skills found — audit checks on skill quality skipped"
    # Still check plugin-level items
fi

# Helper: check YAML frontmatter in a file
has_yaml_frontmatter() {
    local file="$1"
    [[ -f "$file" ]] || return 1
    head -1 "$file" | grep -q '^---$' || return 1
    # Check for closing ---
    tail -n +2 "$file" | grep -qm1 '^---$' || return 1
    return 0
}

# Helper: extract YAML frontmatter value
frontmatter_value() {
    local file="$1" key="$2"
    if [[ ! -f "$file" ]]; then echo ""; return; fi
    # Extract between first and second ---
    local in_fm=false
    while IFS= read -r line; do
        if [[ "$line" == "---" ]]; then
            if $in_fm; then break; fi
            in_fm=true
            continue
        fi
        if $in_fm; then
            if echo "$line" | grep -qE "^${key}:"; then
                echo "$line" | sed "s/^${key}:[[:space:]]*//" | sed 's/^["'"'"']//' | sed 's/["'"'"']$//'
                return
            fi
        fi
    done < "$file"
    echo ""
}

# ── Plugin-level checks ──

# plugin.json exists and is valid JSON
audit_max=$((audit_max + 1))
PLUGIN_JSON="$PLUGIN_DIR/.claude-plugin/plugin.json"
if [[ -f "$PLUGIN_JSON" ]] && python3 -c "import json; json.load(open('$PLUGIN_JSON'))" 2>/dev/null; then
    audit_score=$((audit_score + 1))
    info "PASS: plugin.json is valid JSON"
else
    warn "FAIL: plugin.json missing or invalid"
fi

# Required root files
for reqfile in CLAUDE.md AGENTS.md PHILOSOPHY.md README.md LICENSE .gitignore; do
    audit_max=$((audit_max + 1))
    if [[ -f "$PLUGIN_DIR/$reqfile" ]]; then
        audit_score=$((audit_score + 1))
    else
        warn "FAIL: Missing required file: $reqfile"
    fi
done

# plugin.json has required fields (name, version, description, author, skills)
if [[ -f "$PLUGIN_JSON" ]]; then
    for field in name version description; do
        audit_max=$((audit_max + 1))
        if python3 -c "
import json, sys
d = json.load(open('$PLUGIN_JSON'))
v = d.get('$field', '')
if not v:
    sys.exit(1)
if '$field' == 'description' and len(v) < 10:
    sys.exit(1)
" 2>/dev/null; then
            audit_score=$((audit_score + 1))
        else
            warn "FAIL: plugin.json missing or empty '$field'"
        fi
    done
fi

# ── Per-skill checks ──

for skill_file in "${SKILL_FILES[@]}"; do
    skill_dir=$(dirname "$skill_file")
    skill_name=$(basename "$skill_dir")
    info "Auditing skill: $skill_name"

    # Structure 1: Valid YAML frontmatter with --- delimiters
    audit_max=$((audit_max + 1))
    if has_yaml_frontmatter "$skill_file"; then
        audit_score=$((audit_score + 1))
        info "  PASS: YAML frontmatter"
    else
        warn "  FAIL: Missing YAML frontmatter in $skill_name"
    fi

    # Structure 2: name field present (lowercase, hyphens, max 64 chars)
    audit_max=$((audit_max + 1))
    name_val=$(frontmatter_value "$skill_file" "name")
    if [[ -n "$name_val" ]] && echo "$name_val" | grep -qE '^[a-z][a-z0-9-]{0,63}$'; then
        audit_score=$((audit_score + 1))
        info "  PASS: name field valid ($name_val)"
    elif [[ -n "$name_val" ]]; then
        warn "  FAIL: name field format invalid: $name_val"
    else
        warn "  FAIL: name field missing in $skill_name"
    fi

    # Structure 3: description field present and specific (>10 chars)
    audit_max=$((audit_max + 1))
    desc_val=$(frontmatter_value "$skill_file" "description")
    if [[ -n "$desc_val" ]] && [[ ${#desc_val} -gt 10 ]]; then
        audit_score=$((audit_score + 1))
        info "  PASS: description field"
    else
        warn "  FAIL: description field missing or too short in $skill_name"
    fi

    # Structure 4: SKILL.md under 500 lines
    audit_max=$((audit_max + 1))
    line_count=$(wc -l < "$skill_file")
    if [[ "$line_count" -lt 500 ]]; then
        audit_score=$((audit_score + 1))
        info "  PASS: SKILL.md is $line_count lines (<500)"
    else
        warn "  FAIL: SKILL.md is $line_count lines (>=500)"
    fi

    # Structure 6: SKILL-compact.md exists if SKILL.md > 200 lines
    if [[ "$line_count" -gt 200 ]]; then
        audit_max=$((audit_max + 1))
        if [[ -f "$skill_dir/SKILL-compact.md" ]]; then
            audit_score=$((audit_score + 1))
            info "  PASS: SKILL-compact.md exists (skill > 200 lines)"
        else
            warn "  FAIL: SKILL-compact.md missing (skill is $line_count lines > 200)"
        fi
    fi

    # Content Quality 1: Uses standard markdown headings (NOT XML tags)
    audit_max=$((audit_max + 1))
    # Check body (after frontmatter) for headings
    body=$(sed -n '/^---$/,/^---$/d; p' "$skill_file" | tail -n +2)
    if echo "$body" | grep -qE '^#{1,6} '; then
        audit_score=$((audit_score + 1))
        info "  PASS: Uses markdown headings"
    else
        warn "  FAIL: No markdown headings found in $skill_name"
    fi

    # Content Quality 4: Has Quick Start, Instructions, Overview, or Workflow section
    audit_max=$((audit_max + 1))
    if grep -qiE '^#+\s*(Quick Start|Instructions|Usage|How to|Overview|The Workflow|Recovery|Steps|Getting Started)' "$skill_file"; then
        audit_score=$((audit_score + 1))
        info "  PASS: Has actionable section (Quick Start/Instructions/Overview/Workflow)"
    else
        warn "  FAIL: No actionable section in $skill_name"
    fi

    # Anti-Pattern 1: No XML tags in body
    audit_max=$((audit_max + 1))
    # Check for XML-style tags (but not markdown code blocks)
    # Use grep -c safely — it exits 1 on no match, so capture separately
    xml_tags=$(sed -n '/^---$/,/^---$/d; p' "$skill_file" | grep -cE '<[a-zA-Z][a-zA-Z0-9]*[^>]*>' 2>/dev/null) || xml_tags=0
    code_fences=$(grep -cE '^```' "$skill_file" 2>/dev/null) || code_fences=0
    # Allow XML inside code blocks — rough heuristic: if xml_tags > code_fences, likely real XML
    if [[ "$xml_tags" -le "$code_fences" ]]; then
        audit_score=$((audit_score + 1))
        info "  PASS: No XML tags in body"
    else
        warn "  FAIL: Found $xml_tags XML-like tags in $skill_name"
    fi

    # Anti-Pattern 4: No overly broad descriptions
    audit_max=$((audit_max + 1))
    if echo "$desc_val" | grep -qiE 'helps with things|general purpose|does stuff|various tasks'; then
        warn "  FAIL: Overly broad description in $skill_name"
    else
        audit_score=$((audit_score + 1))
        info "  PASS: Description is not overly broad"
    fi
done

echo "METRIC audit_score=$audit_score"
echo "METRIC audit_max=$audit_max"

# ─── Phase 4: Compute PQS ────────────────────────────────────────────────────
#
# PQS = (structural_pass / structural_total) * build_passes * (audit_score / audit_max)
# Handle division: if denominator is 0, that factor is 1.0 (neutral)

pqs=$(python3 -c "
structural_pass = $structural_pass
structural_total = $structural_total
build_passes = $build_passes
audit_score = $audit_score
audit_max = $audit_max

structural_ratio = structural_pass / structural_total if structural_total > 0 else 1.0
audit_ratio = audit_score / audit_max if audit_max > 0 else 1.0

pqs = structural_ratio * build_passes * audit_ratio
print(f'{pqs:.4f}')
")

echo "METRIC plugin_quality_score=$pqs"

info "Done. PQS=$pqs (structural=$structural_pass/$structural_total, build=$build_passes, audit=$audit_score/$audit_max)"

exit 0
