# Interlab Campaign Templates

Reusable templates for common autoresearch campaign patterns. Copy a template, replace `{{PLACEHOLDERS}}`, and reference from your campaign's interlab configuration.

## Templates

| Template | Pattern | When to use |
|----------|---------|-------------|
| `go-bench.sh.tmpl` | Go benchmark → METRIC | Optimizing Go code with `go test -bench` |
| `cost-baseline.sh.tmpl` | Cost query → METRIC | Measuring cost-per-landable-change impact |
| `config-sweep.sh.tmpl` | YAML value sweep → METRIC | Testing multiple config values to find optimum |

## Usage

```bash
# 1. Copy template to your campaign directory
cp interverse/interlab/templates/go-bench.sh.tmpl campaigns/my-campaign/benchmark.sh

# 2. Replace placeholders
sed -i 's|{{PKG}}|./priompt/|; s|{{BENCH}}|BenchmarkRender100$|; s|{{METRIC}}|render_ns|; s|{{DIR}}|masaq|' campaigns/my-campaign/benchmark.sh

# 3. Test it
bash campaigns/my-campaign/benchmark.sh
```

## METRIC Format

All templates output `METRIC key=value` lines that interlab consumes:

```
METRIC render_ns=30000
METRIC allocs_per_op=22
METRIC bytes_per_op=96312
```

The existing `go-bench-harness.sh` in `scripts/` is the canonical Go benchmark runner — templates wrap it with campaign-specific configuration.
