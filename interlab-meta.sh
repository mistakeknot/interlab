#!/usr/bin/env bash
set -euo pipefail
# interlab-meta.sh — measures interlab's own optimization effectiveness.
# Primary: campaign_success_rate (fraction of campaigns producing improvement)

DIR="$(cd "$(dirname "$0")" && pwd)"
CAMPAIGNS_DIR="$DIR/campaigns"

# Count campaigns with positive improvement
total=0
improved=0

# Check both interlab.jsonl (standard) and results.jsonl (legacy)
for campaign_dir in "$CAMPAIGNS_DIR"/*/; do
    jsonl=""
    [[ -f "$campaign_dir/interlab.jsonl" ]] && jsonl="$campaign_dir/interlab.jsonl"
    [[ -z "$jsonl" && -f "$campaign_dir/results.jsonl" ]] && jsonl="$campaign_dir/results.jsonl"
    [[ -z "$jsonl" ]] && continue
    total=$((total + 1))
    # Check if any experiment was kept (decision=keep)
    if grep -q '"decision":"keep"' "$jsonl" 2>/dev/null; then
        improved=$((improved + 1))
    fi
done

if [[ $total -gt 0 ]]; then
    RATE=$(python3 -c "print(f'{$improved / $total:.4f}')")
else
    RATE="0"
fi

echo "METRIC campaign_success_rate=$RATE"
echo "METRIC campaigns_total=$total"
echo "METRIC campaigns_improved=$improved"

# Mutation store metrics (if available)
MUTATIONS_DB="$HOME/.local/share/interlab/mutations.db"
if [[ -f "$MUTATIONS_DB" ]]; then
    TOTAL_MUTATIONS=$(sqlite3 "$MUTATIONS_DB" "SELECT COUNT(*) FROM mutations" 2>/dev/null || echo "0")
    BEST_MUTATIONS=$(sqlite3 "$MUTATIONS_DB" "SELECT COUNT(*) FROM mutations WHERE is_new_best=1" 2>/dev/null || echo "0")
    SEEDED=$(sqlite3 "$MUTATIONS_DB" "SELECT COUNT(*) FROM mutations WHERE inspired_by IS NOT NULL AND inspired_by != ''" 2>/dev/null || echo "0")
    echo "METRIC mutation_total=$TOTAL_MUTATIONS"
    echo "METRIC mutation_best_count=$BEST_MUTATIONS"
    echo "METRIC mutation_seeded_count=$SEEDED"
fi

echo "METRIC benchmark_exit_code=0"
