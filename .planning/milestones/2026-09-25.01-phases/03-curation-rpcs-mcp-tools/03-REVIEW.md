---
phase: 03-curation-rpcs-mcp-tools
reviewed: 2026-09-27T00:00:00Z
depth: standard
files_reviewed: 47
files_reviewed_list:
  - .claude/skills/engram-connect-client/SKILL.md
  - CLAUDE.md
  - docs-site/src/content/docs/guides/upgrade.md
  - docs-site/src/content/docs/reference/errors.md
  - docs-site/src/content/docs/reference/memory-record.md
  - docs-site/src/content/docs/reference/tools.md
  - internal/e2e/boot_test.go
  - internal/server/archive.go
  - internal/server/archive_test.go
  - internal/server/connectapi.go
  - internal/server/connectapi_parity_test.go
  - internal/server/connectapi_write_parity_test.go
  - internal/server/connectcsrf.go
  - internal/server/connectcsrf_lane_test.go
  - internal/server/connectcsrf_test.go
  - internal/server/connectdescriptor_test.go
  - internal/server/fakestore_test.go
  - internal/server/listrules_test.go
  - internal/server/protoconv.go
  - internal/server/registertools_test.go
  - internal/server/related.go
  - internal/server/related_test.go
  - internal/server/rules.go
  - internal/server/scheduled_test.go
  - internal/server/schemarequired_test.go
  - internal/server/store_iface.go
  - internal/server/supersedepreview.go
  - internal/server/supersedepreview_test.go
  - internal/server/surfaces_test.go
  - internal/server/tags.go
  - internal/server/tags_test.go
  - internal/server/tools.go
  - internal/server/tools_test.go
  - internal/skills/data/curating-memory/SKILL.md
  - internal/skills/data/curating-spine/SKILL.md
  - internal/store/concurrent_gates_test.go
  - internal/store/listscheduled_oversized_test.go
  - internal/store/recallmax_oversized_test.go
  - internal/store/relatedmemories.go
  - internal/store/relatedmemories_test.go
  - internal/store/schemaversion_compat_test.go
  - internal/store/schemaversion_recallgate_test.go
  - internal/store/store.go
  - internal/store/store_test.go
  - internal/surfaces/toolclass.go
  - internal/surfaces/toolclass_test.go
  - proto/engram/v1/engram.proto
  - skill/engram/skills/curating-memory/SKILL.md
  - skill/engram/skills/curating-spine/SKILL.md
findings:
  critical: 0
  warning: 0
  info: 2
  total: 2
status: clean
---

# Phase 3: Code Review Report

**Reviewed:** 2026-09-27
**Depth:** standard
**Files Reviewed:** 47 (of 49 listed; the two vendored-SPA build-output diffs, `internal/webauth/static/index.html` and the `ui/build/immutable/**` chunk renames, are generated artifacts out of scope for source review)
**Status:** clean

## Summary

This phase adds seven Connect RPCs (`ArchiveMemory`, `RestoreMemory`,
`SupersedeMemory` validate_only, `ListScheduled` cross_spine/cursor,
`ListRules` all-scopes, `RelatedMemories`, `ListTags`) plus four new MCP
tools, following the established thin-adapter pattern (`deps.*` core shared
by both lanes, `protoconv.go` mapping, `connectError` for rejections). I
traced every write Procedure through the CSRF allowlist and interceptor
tests, checked owner-gate/isolation proofs for archive/restore, related, list
tags, and list rules against overlapping-scope, cross-actor fixtures run
against a real Qdrant testcontainer, verified the `ListScheduled` cursor
change reuses the exact `listByCursor` decode/validate pattern (same
`ErrInvalidArgument` on a malformed or oversized cursor, same `Direction_Desc`
ordering), checked the `RelatedEdge` oneof↔flat mapping and its parity test,
and confirmed the batch-archive resource bounds (1 to 1000 ids, 256 bytes per
entry) are validated once in a shared core so both lanes emit byte-identical
`field=/hint=` envelopes. `go build ./...` and `go vet ./internal/...` are
clean.

No correctness, security, or authorization defects found. The two findings
below are quality-only: a stale doc-comment count with no behavioral effect,
and a pre-existing (but phase-touched) inconsistency in how the
`supersede_memory` MCP closure handles an error return compared to every
sibling write tool this phase added.

## Info

### IN-01: `TestRegisterToolsEnumerable` doc comment undercounts the tool set

**File:** `internal/server/registertools_test.go:75`
**Issue:** The function doc comment says `"proves registerTools registers the real, full 17-tool set"`, but `wantRegisteredToolNames` (same file, lines 25-30) lists 19 entries (the pre-existing 15 plus this phase's `archive_memory`, `restore_memory`, `related_memories`, `list_tags`) — and the preceding comment on `wantRegisteredToolNames` itself correctly says "19 tool registrations". The test's actual assertion (`reflect.DeepEqual(got, want)` against the 19-entry `want` map) is unaffected — the count only appears in prose — but the stale number will mislead the next reader trying to sanity-check the inventory.
**Fix:**
```go
// TestRegisterToolsEnumerable proves registerTools registers the real,
// full 19-tool set — with real, non-empty Descriptions — against a bare
```

### IN-02: `supersede_memory` MCP closure builds a nonsensical success message on a validate_only preflight failure

**File:** `internal/server/tools.go:3093-3097`
**Issue:** Every other write tool this phase added (`archive_memory`, `restore_memory`, and the pre-existing `store_memory`/etc. this one still mirrors) either checks `err != nil` before building its result, or the convention relies on the MCP SDK discarding `result`/`structuredResult` whenever a non-nil `error` is also returned. `supersede_memory`'s closure does the latter — it never checks `err` before branching on `out.Validated`:
```go
out, err := d.supersede(ctx, c, a)
if out.Validated {
    return textResult("validated: would supersede " + strings.Join(out.Supersedes, ", ")), ..., err
}
return textResult(fmt.Sprintf("stored %s, superseding %s", out.ID, strings.Join(a.Supersedes, ", "))), ..., err
```
When `a.ValidateOnly` is true and `d.validateSupersede`'s preflight rejects (e.g. an unowned or ambiguous target), `d.supersede` returns a zero-value `supersedeOutcome{}` plus the error. `out.Validated` is then `false`, so the closure falls into the second `return`, producing the text `"stored , superseding <targets>"` — a confusing, semantically wrong message for a call that neither validated nor stored anything — alongside the real error. This is a pre-existing pattern (the un-modified `store_memory` closure has the identical shape), so it is very likely dead/harmless in practice (the go-sdk appears to prefer the returned `error` over `result` when both are non-nil, matching every other handler in this file relying on the same convention) — but this phase added a *second* branch to the same function that inherits the same latent trap, and the new "stored %s, superseding %s" text is actively misleading if that assumption about the SDK ever changes or if any test starts asserting on `CallToolResult` content on an error path.
**Fix:** Check the error first, matching `archive_memory`/`restore_memory`/`related_memories`/`list_tags`:
```go
out, err := d.supersede(ctx, c, a)
if err != nil {
    return nil, nil, err
}
if out.Validated {
    return textResult("validated: would supersede " + strings.Join(out.Supersedes, ", ")), map[string]any{"validated": true, "supersedes": out.Supersedes, "targets": shapeRecall(out.Targets, false, d.summaryMaxChars)}, nil
}
return textResult(fmt.Sprintf("stored %s, superseding %s", out.ID, strings.Join(a.Supersedes, ", "))), map[string]string{"id": out.ID, "short_id": out.ShortID}, nil
```

---

_Reviewed: 2026-09-27_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
