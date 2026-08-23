---
status: complete
phase: 05-connect-record-state-parity
source: [05-01-SUMMARY.md, 05-02-SUMMARY.md, 05-03-SUMMARY.md]
started: 2026-08-16
updated: 2026-08-16
---

## Current Test

[testing complete]

## Tests

### 1. Memory carries fields 23-30 exactly matching D-04 as amended by D-14
expected: engramv1.Memory carries fields 23-30 exactly matching D-04's table as amended by D-14 (three new scalars with `optional`, four Timestamps, one repeated); fields 1-22 untouched.
result: pass
source: automated
coverage_id: 05-01/D1

### 2. memoryToProto populates all eight new fields on a real handler round trip
expected: memoryToProto populates all eight new fields from a real store.Memory, proven via a Qdrant-backed Connect GetMemory handler round trip (not the mapper in isolation, not shapeProtoMemories).
result: pass
source: automated
coverage_id: 05-01/D2

### 3. SummaryEgressAt comment repair (05-01 D3)
expected: The third 05-01 deliverable, auto-covered by its declared verification refs.
result: pass
source: automated
coverage_id: 05-01/D3

### 4. unmappedStoreFields detector rejects by exact byte equality only
expected: unmappedStoreFields exists exactly once, is called by the real assertion, the permanent negative fixture, the near-miss fixture, and the determinism check; it rejects by exact byte equality only (no prefix/substring/case matching), returns sorted output, and is pinned by a whole-map-equality assertion on the one-entry alias map.
result: pass
source: automated
coverage_id: 05-02/D1

### 5. Auto-filled store.Memory maps with every proto field populated and decoding back
expected: A reflection auto-filled store.Memory (every json-visible field pairwise-distinct, including repeated and message-valued fields) maps to a proto message on which every field reports Has()==true, decodes back to its source under an exhaustiveness-gated comparator, and preserves supersedes order.
result: pass
source: automated
coverage_id: 05-02/D2

### 6. Zero-value source still assigns schema_version and summary_model
expected: For a zero-value store.Memory, memoryToProto still ASSIGNS schema_version and summary_model (Has(fd) true) while leaving superseded_by unassigned (Has(fd) false) — D-14 §3's assign-always guarantee.
result: pass
source: automated
coverage_id: 05-02/D3

### 7. Boundary-second bounds round outward and agree on both read lanes
expected: A not_before submitted with a sub-second component comes back floored to the containing whole second and a not_after comes back ceiled to it, identically on the MCP and Connect read lanes, from one write; a bound already on a whole second comes back unchanged; the MCP lane is read out of the record's serialized json form, proven RED against a not_before json-tag rename.
result: pass
source: automated
coverage_id: 05-03/D1

### 8. engram list --output json renders schema_version:0 and omits it when unassigned
expected: engram list --output json renders schema_version:0 for an ASSIGNED-zero Memory field as a JSON number, and OMITS the key for an UNASSIGNED field — the permanent negative fixture proving the presence assertion can fail. Number-not-string rendering and Timestamp-absent-not-null behavior also pinned.
result: pass
source: automated
coverage_id: 05-03/D2

### 9. Operator console still loads with the rebuilt UI bundle
expected: |
  Start the engram server and open the operator console in a browser. The console loads
  without a blank page and without 404s on hashed `_app/immutable/**` chunks, and memory
  records render as before. This phase regenerated the Connect TypeScript types and re-ran
  `task ui:build`, which rewrote `internal/webauth/static/index.html` and every hashed chunk
  filename — a stale hash reference would surface as a blank console, not as a failing Go test.
result: issue
reported: "we should have an e2e test for this"
severity: minor
rationale: |
  Added as a fail-safe human checkpoint, NOT derived from a coverage entry. Commit
  33a6a8c5 ("rebuild embedded web UI for regenerated Connect TS types") is a
  plan-mandated drift commit that no `coverage:` entry in any of the three SUMMARYs
  claims. The Go suite cannot observe it: the bundle is a static asset served by the
  server, so a broken chunk reference stays green in `go test ./...`. The console is
  also one of the two consumers of the Connect lane this phase changed, which is the
  reason #482 existed in the first place.

## Summary

total: 9
passed: 8
issues: 1
pending: 0
skipped: 0
blocked: 0

## Gaps

```yaml
- gap_id: G-05-9
  truth: "The embedded operator-console bundle is internally consistent: every asset index.html references under _app/immutable/** exists in the embedded FS and is actually served, so a `task ui:build` regeneration cannot ship a stale chunk reference."
  status: failed
  reason: "User reported: we should have an e2e test for this. Commit 33a6a8c5 rebuilt the bundle and rewrote index.html plus every hashed chunk filename; no automated test observes that rewrite. The Go suite stays green because the bundle is a static asset, so the failure surfaces only as a blank console at runtime."
  severity: minor
  test: 9
  artifacts:
    - internal/webauth/static.go
    - internal/webauth/static/index.html
    - internal/e2e/
  missing:
    - "A test that parses index.html out of the embedded FS, extracts every _app/immutable/** reference, asserts the extracted set is NON-EMPTY (a zero-reference parse must fail, not vacuously pass), and requests each one through the real http.FileServer(http.FS(sub)) handler asserting 200 and a non-empty body."
  prior_art: "GH #106 — a bare `//go:embed static` excluded _app/ and left the SPA unable to mount. The `all:` prefix in internal/webauth/static.go:18 is the fix for that incident, and nothing currently guards against it regressing."
```
