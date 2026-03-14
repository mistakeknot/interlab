# interlab Learnings: interlab-reconstruct-speed

Campaign: Optimize `ReconstructState()` hot path
Date: 2026-03-13
Result: 1,540K ns → 68K ns (-96%, 22x faster)

## Validated Insights

- **json.Unmarshal dominates even for tiny structs** — proved by experiment #8, delta -77%
  - Evidence: Replacing a 1-field struct unmarshal with `bytes.Index` + `strconv.ParseFloat` cut 280K ns. The tokenizer walks the entire JSON line regardless of how few fields you need.

- **Byte-level type detection beats structured parsing** — proved by experiments #1+#2, delta -50% combined
  - Evidence: `bytes.Contains(line, "type":"config")` is ~3x faster than `json.Unmarshal` into a typeOnly struct, which is itself ~2x faster than unmarshal into `map[string]interface{}`.

- **Buffer sizing matters for small files** — proved by experiment #3, delta -26%
  - Evidence: Default 1MB scanner buffer for a 20KB file wastes allocation time. Sizing to file via `f.Stat()` cut 200K ns.

- **Skip work you don't need** — proved by experiment #7, delta -22%
  - Evidence: Discard/crash results don't need metric_value parsing. Byte-scanning the decision string and skipping JSON entirely for non-keep results saved 100K ns.

## Dead Ends

- **json.Encoder → json.Marshal for writes** — tried in experiment #4, no improvement because it's the write path, not the read path being benchmarked. Obvious in hindsight.

- **Pre-allocated package-level byte patterns** — tried in experiment #6, no improvement because Go compiler already optimizes `[]byte("literal")` conversions to avoid allocation.

## Patterns

- **When you control the serialization format, avoid generic parsers** — byte scanning is 10-20x faster than JSON parsing for known-format data. Applies to any hot-path JSONL reader.

- **Optimize the common case, not the general case** — result lines are ~99% of a campaign JSONL. Config lines (1 per segment) can stay slow. Discard/crash results (often 30-50%) can skip metric parsing entirely.

- **Benchmark variance on cloud VMs is ~15%** — use median of 3+ runs, not single measurements. Changes under 10% are noise.

- **Dogfooding finds real bugs** — the `working_directory` cwd bug would never have been caught by unit tests. Only running the actual MCP server from a different directory exposed it.
