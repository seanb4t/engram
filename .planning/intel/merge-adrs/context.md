# Context & Traceability — merge-adrs companion set

## Cross-reference graph

3-color DFS cycle detection run over the `cross_refs` of all 31 ADRs (depth cap 50).

**In-set edges** (both endpoints among the 31 ingested ADRs):
- engram-8q3 → engram-u9v
- engram-m4s8 → engram-ddiw

Both targets are terminal (no outgoing in-set edges). No back-edges encountered.
**Result: no cycles.** Max traversal depth = 2, well under the cap.

**Dangling cross-refs** (targets outside the 31-doc ingest set — recorded for traceability,
do NOT block; several point at existing baseline locks or non-id strings):

- engram-1h3k → engram-s2ao (out-of-set id)
- engram-2xl → engram-0lu (baseline lock)
- engram-3nas → engram-kyz (baseline lock DEC-kyz)
- engram-6gb → engram-uxh (baseline lock DEC-uxh)
- engram-8q3 → engram-1xv (out-of-set id)
- engram-d386 → engram-ambu (baseline lock DEC-ambu)
- engram-lzz → engram-e38 (out-of-set id)
- engram-no3 → engram-0lu (baseline lock)
- engram-om5b → "two-tier vitest decision" (prose ref to DEC-1h3k, not an id)
- engram-tdk → engram-ew7 (out-of-set id)
- engram-vxk → engram-0lu (baseline lock)
- engram-wtw → engram-edv, engram-mbnw (out-of-set ids)
- engram-50b → 2026-06-02-generalize-engram-client-config, PR #16, v0.3.0 (change-log / non-id strings)

## Refinement map (companion → baseline lock)

These 31 ADRs are the fine-grained companions omitted from the original 50-doc bootstrap.
Each refines or implements a scope already governed at a coarser grain by an existing lock.
This is expected and complementary, not contradictory:

- Rules index / session-start:  d386, m4s8 → iedk, ambu
- Temporal recall gate:         ufz, c0m → y1g
- Telemetry seams/attrs:        6gb, f7p, tdk, wot, 7qd, 9tj → dwi, uxh
- Session-cookie sealing / BFF:  8q3, u9v, bgj → g37x
- SPA internals:                2xl, 3nas, c4y, lzz, no3, vxk, 4ag → 0lu, 8xe
- Docs-site hosting/deploy:     1w7, u5h → ttb
- Test tiers:                   1h3k, om5b → vitest-browser direction
- Discovery tools/trust:        0gy, 3l0 → discovery baseline
- Memory summary/update:        4y7p, ddiw → summary/update baseline
- Config validation:            wtw, d24 → config baseline
- Plugin packaging:             50b → bundled skill/engram plugin baseline (see below)

## 50b adjudication (recorded)

engram-50b states the engram plugin ships NO bundled MCP server and that `/engram-setup` is
the sole registration path (remove the bundled `.mcp.json`). The Phase-7 baseline states the
memory-curator client plugin was relocated into the repo as the *bundled* `skill/engram` plugin.

These operate on different axes and are consistent:
- "bundled plugin" = the `skill/engram` plugin is vendored/shipped inside the repo (packaging).
- "no bundled MCP server" = that plugin does not ship a `.mcp.json` that auto-registers the MCP
  server; registration is performed explicitly by `/engram-setup` via `claude mcp add`.

A bundled plugin can register its MCP server through `/engram-setup` rather than through a
shipped `.mcp.json`. No mutually-exclusive knob is set to two incompatible values. Classified as
INFO (complementary), not a BLOCKER. See INGEST-CONFLICTS.md.
