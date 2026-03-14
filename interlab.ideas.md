# Ideas Backlog

## Promising

- [ ] Cache file descriptor across multiple reads in same process (MCP server is long-lived)
- [ ] Read file into memory in one shot (`os.ReadFile`) then scan bytes — avoids buffered I/O overhead
- [x] Extract metric_value via byte scan — **-77%** (#8, biggest single win)
- [ ] Use `bufio.Reader` with larger buffer instead of Scanner for fewer syscalls

## Tried

- [x] Use typeOnly struct for type discrimination — **-31%** (#1)
- [x] Byte-level type detection with bytes.Contains — **-27%** (#2)
- [x] Size scanner buffer to file size — **-26%** (#3)
- [x] Replace json.Encoder with json.Marshal in appendJSONL — no change (#4, discarded)
- [x] Lightweight result struct (3 fields only) — **-20%** (#5)
- [x] Pre-allocate byte patterns as package vars — no change (#6, discarded)
- [x] Byte-scan decisions, skip JSON for discard/crash — **-22%** (#7)

## Tried

(none yet)

## Rejected

(none yet)
