# Ideas Backlog

## Promising

- [ ] Pre-allocate scanner buffer based on file size — avoid repeated growth
- [ ] Use json.RawMessage for first-pass type discrimination instead of map[string]interface{}
- [ ] Avoid double unmarshal (currently: unmarshal to map, then unmarshal to struct) — use type peek
- [ ] Use bufio.Scanner line-by-line with pre-sized byte slice pool
- [ ] Replace json.Encoder with direct append+Marshal for WriteResult (avoid encoder overhead)
- [ ] Cache file descriptor across multiple reads in same process (MCP server is long-lived)
- [ ] Use json.Decoder instead of Scanner+Unmarshal to skip intermediate byte copy
- [ ] Compile regex once at package level for parseMetrics (already done — verify no allocation)
- [ ] Reduce allocations in isBetter/defaults by inlining

## Tried

(none yet)

## Rejected

(none yet)
