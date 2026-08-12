---
phase: 02-interface-discoverability
plan: 04
subsystem: api
tags: [go, mcp, go-sdk, tool-annotations, codegen]

# Dependency graph
requires:
  - phase: 02-interface-discoverability
    plan: 02
    provides: "internal/server/registertools_test.go#registeredTools(t) — the in-memory-transport ListTools round trip this plan's both-directions gate reuses rather than re-enumerating the 15 tools by hand."
  - phase: 02-interface-discoverability
    plan: 03
    provides: "The widened internal/surfaces registry and its conformance-gate test files, the most recent shape of internal/surfaces this plan extends with a second, unrelated declared table."
provides:
  - "internal/surfaces/toolclass.go: Class/Operation, Operations(), ClassForTool/ClassForCommand, ValidateOperations() — the shared, stdlib-only blast-radius table for all 15 MCP tools plus 7 CLI-only operations, keyed by MCP tool name and CLI command name so plan 02-05's catalog lane reads the SAME table."
  - "internal/server/toolannotations.go: annotationsFor(name) *mcp.ToolAnnotations, boolPtr — the translation from the shared Class to go-sdk v1.7.0's mcp.ToolAnnotations wire shape (bare bool ReadOnlyHint/IdempotentHint, *bool DestructiveHint/OpenWorldHint)."
  - "All 15 mcp.AddTool registrations in tools.go carry Annotations: annotationsFor(\"<name>\")."
  - "internal/server/toolannotations_test.go#TestToolAnnotationsBothDirections — the D-10 both-directions gate, proven fail-first in three ways this session."
  - "The tool-blast-radius generated region in docs-site/reference/tools.md, emitted by internal/surfacesgen from surfaces.Operations()."
affects: [02-05, 02-06]

# Actuals (#2632)
actuals:
  tokens: 11281
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "A second, independent declared table (Class/Operation) lives alongside internal/surfaces' existing ConditionalRule registry in the same leaf package — same declared-slice/built-once-derived-map/exported-lookup-function shape as internal/config/registry.go, proven reusable for an unrelated taxonomy."
    - "annotationsFor returns nil (never a zero-valued &mcp.ToolAnnotations{}) on a missing table entry, so an incomplete table surfaces as a nil Annotations pointer the both-directions gate's second assertion loop catches — the same 'absence over false-positive' discipline ConditionalRule's TagForm/Sentence checks already use."
    - "WriteRegion/ReadRegion's anchor machinery is generic over the region id string, not exclusively bound to a surfaces.ConditionalRule — the tool-blast-radius region reuses it under a distinct, hand-authored anchor pair with no corresponding declared rule."

key-files:
  created:
    - internal/surfaces/toolclass.go
    - internal/surfaces/toolclass_test.go
    - internal/server/toolannotations.go
    - internal/server/toolannotations_test.go
  modified:
    - internal/server/tools.go
    - internal/surfacesgen/main.go
    - docs-site/src/content/docs/reference/tools.md

key-decisions:
  - "delete_memory/delete_all classified Idempotent: true under REST-DELETE-style idempotency reasoning: repeating a delete against an already-removed target has NO ADDITIONAL effect on the environment, even though the repeat itself is rejected as not-found. Documented in a row comment since the reasoning (effect-on-environment, not response code) is not obvious from the tool's Description alone."
  - "supersede_memory classified Idempotent: true — NOT via an idempotency_key (the tool explicitly supports none) but structurally: the single-live-head invariant rejects a second call targeting an already-superseded record, so an identical repeat creates no second record. This is the one case where D-09's 'holds under every valid invocation' resolves to true through a structural guarantee rather than a dedup key."
  - "set_visibility classified Destructive: false, diverging from update_memory's Destructive: true — it only flips a boolean visibility flag; content, tags, and the vector are untouched, and the toggle is always reversible by calling again, unlike update_memory's in-place content replace."
  - "store_discovery and store_rule classified Idempotent: false despite both supporting an optional id-to-replace-in-place path, because D-09's 'every valid invocation' also covers the omit-id (create) path, which always produces a new record even called identically twice."
  - "The bare `engram` self-describe invocation is keyed as CLICommand: \"engram\" (root.Use) rather than empty string, since ValidateOperations rejects a row with both key columns empty and this operation has no MCP counterpart."
  - "The five mutating CLI-only operator commands (reindex, migrate-remap-owner, prune-expired, summarize-missing, backfill-short-ids) are all classified Idempotent: true, grounded in each command's own doc comment/Short text confirming it skips already-processed records (verified by reading reindex.go/prune.go/summarize.go/backfill.go/migrate.go source, not assumed)."
  - "tool-blast-radius is a NEW anchor pair hand-authored in docs-site/reference/tools.md under a '## Blast radius' section, not tied to any surfaces.ConditionalRule — WriteRegion/ReadRegion key on an arbitrary region-id string, proven reusable beyond the ConditionalRule registry this session."

patterns-established:
  - "A missing table entry must resolve to a nil pointer/absent value the gate's own assertions catch — never a zero-valued struct that could pass a shallow nil check. annotationsFor's nil-on-miss return is the pattern; TestAnnotationsForMissingEntryReturnsNil pins it directly."
  - "Non-obvious blast-radius classifications get a one-line row comment explaining the reasoning (REST-DELETE idempotency, structural single-live-head enforcement, omit-id-path non-idempotency) rather than a bare bool literal — future rows should follow the same discipline."

requirements-completed: [REQ-mcp-tool-annotations]

coverage:
  - id: D1
    description: "Every one of the 15 registered MCP tools declares readOnlyHint/destructiveHint/idempotentHint/openWorldHint — none left to the client's default interpretation."
    requirement: "REQ-mcp-tool-annotations"
    verification:
      - kind: unit
        ref: "internal/server/toolannotations_test.go#TestToolAnnotationsBothDirections"
        status: pass
    human_judgment: false
  - id: D2
    description: "openWorldHint is set explicitly to false on every tool rather than omitted."
    requirement: "REQ-mcp-tool-annotations"
    verification:
      - kind: unit
        ref: "internal/surfaces/toolclass_test.go#TestEveryOperationIsClosedWorld"
        status: pass
      - kind: unit
        ref: "internal/server/toolannotations_test.go#TestToolAnnotationsBothDirections"
        status: pass
    human_judgment: false
  - id: D3
    description: "Hint values follow the conservative stance — the three worked examples (store_memory/schedule_memory not idempotent, update_memory destructive, supersede_memory not destructive) resolve as CONTEXT.md specifies."
    requirement: "REQ-mcp-tool-annotations"
    verification:
      - kind: unit
        ref: "internal/surfaces/toolclass_test.go#TestConservativeStanceWorkedExamples"
        status: pass
    human_judgment: false
  - id: D4
    description: "The remaining 11 tools' and 7 CLI-only operations' conservative-stance classifications (delete idempotency via REST-DELETE reasoning, supersede idempotency via structural single-live-head enforcement, set_visibility's non-destructive divergence from update_memory, store_discovery/store_rule's non-idempotent create path, operator-command idempotency grounded in source doc comments)."
    requirement: "REQ-mcp-tool-annotations"
    verification: []
    human_judgment: true
    rationale: "These are judgment calls beyond the three worked examples CONTEXT.md names by hand. Each is documented with a one-line reasoning comment on its row in internal/surfaces/toolclass.go and in this SUMMARY's key-decisions — a human should read and confirm the reasoning is sound, since no external spec dictates the 'correct' answer for e.g. whether a rejected-on-retry delete counts as idempotent."
  - id: D5
    description: "The annotations come from one central table keyed by operation, gated in BOTH directions: every registered tool has a table entry, and every table entry naming an MCP tool names a registered one."
    requirement: "REQ-mcp-tool-annotations"
    verification:
      - kind: unit
        ref: "internal/server/toolannotations_test.go#TestToolAnnotationsBothDirections"
        status: pass
    human_judgment: false
  - id: D6
    description: "The table lives in the internal/surfaces leaf package with no go-sdk dependency, so the CLI lane can read the same taxonomy without importing internal/server."
    requirement: "REQ-mcp-tool-annotations"
    verification:
      - kind: other
        ref: "go list -deps ./internal/surfaces | rg 'modelcontextprotocol/go-sdk' -> no match (exit 1)"
        status: pass
    human_judgment: false
  - id: D7
    description: "The both-directions gate reads the REAL registered tool set through the in-memory transport round trip, never a hand-duplicated list; a nil Annotations pointer or a nil DestructiveHint/OpenWorldHint pointer, or a zero-valued &mcp.ToolAnnotations{}, fails the gate."
    requirement: "REQ-mcp-tool-annotations"
    verification:
      - kind: unit
        ref: "internal/server/toolannotations_test.go#TestToolAnnotationsBothDirections, #TestAnnotationsForMissingEntryReturnsNil"
        status: pass
      - kind: manual_procedural
        ref: "Session transcript: three fail-first demonstrations (missing table row, extra table row naming a nonexistent tool, zero-valued &mcp.ToolAnnotations{}) — see 'Fail-first proofs' below."
        status: pass
    human_judgment: true
    rationale: "The fail-first demonstrations are observed, one-time session events (temporarily mutate, observe RED, revert, observe GREEN), not repeatable automated assertions beyond what TestToolAnnotationsBothDirections already pins — a human should confirm the transcript below matches the claim."
  - id: D8
    description: "docs-site/reference/tools.md states each tool's four hint values, generated from the same table rather than hand-maintained; idempotent regeneration produces no drift."
    requirement: "REQ-mcp-tool-annotations"
    verification:
      - kind: integration
        ref: "go run ./internal/surfacesgen && git diff --exit-code -- docs-site/ (run twice, byte-identical)"
        status: pass
    human_judgment: false

duration: ~15min
completed: 2026-08-05
status: complete
---

# Phase 2 Plan 4: MCP Tool Annotations — the Shared Blast-Radius Table Summary

**All 15 registered MCP tools now advertise readOnlyHint/destructiveHint/idempotentHint/openWorldHint from one internal/surfaces table gated in both directions against the real registration, published to docs-site/reference/tools.md via internal/surfacesgen, and demonstrated fail-first in three distinct ways this session.**

## Performance

- **Duration:** ~15 min
- **Tasks:** 2
- **Files modified:** 7 (4 created, 3 modified)

## Accomplishments

- **The shared table** (`fb8b5db6`). `internal/surfaces/toolclass.go` declares `Class`/`Operation`,
  a package-level `operations` slice covering all 15 MCP tools plus 7 CLI-only operations
  (`reindex`, `migrate-remap-owner`, `prune-expired`, `summarize-missing`, `backfill-short-ids`,
  `version`, the bare `engram` self-describe invocation), `Operations()`, `ClassForTool`/
  `ClassForCommand`, and `ValidateOperations()` — the same declared-slice/built-once-derived-map/
  exported-lookup-function shape `internal/config/registry.go`'s `flagToDefault`/`FlagDefault`
  uses. Zero go-sdk dependency, confirmed via `go list -deps`.
- **D-09's conservative stance, applied to all 22 rows** (`fb8b5db6`). The three worked examples
  CONTEXT.md names by hand — `store_memory`/`schedule_memory` not idempotent, `update_memory`
  destructive, `supersede_memory` not destructive — are spot-checked in
  `TestConservativeStanceWorkedExamples`. The remaining 11 MCP rows and 7 CLI-only rows are
  resolved by the same rule, each with a one-line row comment where the reasoning is not obvious
  from the tool's Description alone (see key-decisions).
- **Annotation wiring + the both-directions gate, TDD** (`56f4c787`). `TestToolAnnotationsBothDirections`
  was written first and observed failing to compile (`annotationsFor` undefined), then failing at
  runtime against un-annotated registrations, before `internal/server/toolannotations.go`'s
  `annotationsFor(name)` and `boolPtr` were implemented and all 15 `mcp.AddTool` calls in `tools.go`
  gained `Annotations: annotationsFor("<name>")` — nothing else about any registration changed.
  The gate copies `TestCatalogExitCodesMatchMapper`'s shape: a `map[string]bool` built from
  `registeredTools(t)` (the real, in-memory-transport `ListTools` round trip from plan 02-02)
  compared via `reflect.DeepEqual` against a `map[string]bool` built from `surfaces.Operations()`'s
  non-empty `MCPTool` values, plus a second assertion loop proving every tool's `Annotations` is
  non-nil, its `DestructiveHint`/`OpenWorldHint` pointers are non-nil, `OpenWorldHint` dereferences
  to `false`, and all four values equal `surfaces.ClassForTool(tool.Name)`.
- **Published taxonomy** (`56f4c787`). `internal/surfacesgen/main.go` gained
  `renderToolBlastRadius()` and a `tool-blast-radius` region write, reusing the existing
  `engram:rule:start`/`engram:rule:end` anchor machinery under a distinct, hand-authored anchor
  pair in a new "## Blast radius" section of `docs-site/reference/tools.md` — proving
  `WriteRegion`/`ReadRegion` generalize beyond `surfaces.ConditionalRule`. Regeneration is
  idempotent (run twice, byte-identical MD5) and drift-free against the committed tree.

## Fail-first proofs (Task 2, observed this session)

**(a) Missing table row** — temporarily removed the `store_memory` row from `surfaces.operations`:
```
--- FAIL: TestToolAnnotationsBothDirections (0.01s)
    toolannotations_test.go:43: registered tool names = [... store_memory ...], surfaces.Operations() MCPTool entries = [... (no store_memory) ...]
    toolannotations_test.go:49: tool "store_memory": Annotations is nil
```
Both the map-equality half and the nil-Annotations half failed independently. Reverted; `go test`
returned to `PASS`.

**(b) Extra table row naming a nonexistent tool** — temporarily added a `no_such_tool_probe` row:
```
--- FAIL: TestToolAnnotationsBothDirections (0.00s)
    toolannotations_test.go:43: registered tool names = [15 real names], surfaces.Operations() MCPTool entries = [15 real names + no_such_tool_probe]
```
Reverted; `go test` returned to `PASS`.

**(c) Zero-valued `&mcp.ToolAnnotations{}`** — temporarily replaced `store_memory`'s
`Annotations: annotationsFor("store_memory")` with a bare `&mcp.ToolAnnotations{}`:
```
--- FAIL: TestToolAnnotationsBothDirections (0.00s)
    toolannotations_test.go:53: tool "store_memory": Annotations.DestructiveHint is nil
    toolannotations_test.go:56: tool "store_memory": Annotations.OpenWorldHint is nil
```
Reverted; `go test` returned to `PASS`. All three probes confirmed the gate genuinely fails on
each of the three failure modes it claims to catch, not merely on a subset.

## Task Commits

1. **Task 1: Declare the shared blast-radius table for all 15 tools** — `fb8b5db6` (feat)
2. **Task 2: Attach the annotations at registration and gate the table in both directions** —
   `56f4c787` (test — TDD: RED compile failure → RED runtime failure → GREEN in one commit,
   copying the RED/GREEN discipline into a single reviewable diff since the test file and its
   subject were authored together)

**Plan metadata:** commit pending (this SUMMARY + STATE.md/ROADMAP.md update)

## Files Created/Modified

- `internal/surfaces/toolclass.go` — `Class`, `Operation`, the 22-row `operations` registry,
  `Operations()`, `ClassForTool`/`ClassForCommand`, `ValidateOperations()`
- `internal/surfaces/toolclass_test.go` — `TestValidateOperations` and its three failure-mode
  siblings, `TestOperationsCoverEveryTool`, `TestConservativeStanceWorkedExamples`,
  `TestEveryOperationIsClosedWorld`, `TestClassForCommandKnowsCLIOnlyOperations`
- `internal/server/toolannotations.go` — `annotationsFor(name) *mcp.ToolAnnotations`, `boolPtr`
- `internal/server/toolannotations_test.go` — `TestToolAnnotationsBothDirections`,
  `TestAnnotationsForMissingEntryReturnsNil`
- `internal/server/tools.go` — all 15 `mcp.AddTool` registrations gained
  `Annotations: annotationsFor("<name>")`; nothing else changed
- `internal/surfacesgen/main.go` — `renderToolBlastRadius()`, the `tool-blast-radius` region write,
  a `surfaces.ValidateOperations()` call alongside the existing `ValidateRules()` call
- `docs-site/src/content/docs/reference/tools.md` — new "## Blast radius" section with the
  generated per-tool hint table inside the `tool-blast-radius` anchor pair

## Decisions Made

See `key-decisions` in frontmatter for the full list. Highlights: `delete_memory`/`delete_all`
classified idempotent under REST-DELETE-style reasoning (effect-on-environment, not response
code); `supersede_memory` classified idempotent via the single-live-head structural guarantee
rather than an idempotency_key; `set_visibility` classified non-destructive (diverging from
`update_memory`) since it only flips a reversible boolean flag; `store_discovery`/`store_rule`
classified non-idempotent because their omit-id create path always produces a new record.

## Deviations from Plan

None — plan executed exactly as written. All required read_first sources (the vendored
go-sdk v1.7.0 `ToolAnnotations` shape, `CLAUDE.md`'s memory contract, `catalog_test.go`'s
both-directions gate, `registertools_test.go`'s helper) were read directly and matched what the
plan described; no compile-time surprise or architectural gap was found.

## Issues Encountered

`go build ./internal/surfacesgen/...` without `-o` left a stray `surfacesgen` binary in the repo
root during manual verification. Caught before staging via `git status --short` and removed; never
committed.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `internal/surfaces.Operations()`/`ClassForTool`/`ClassForCommand` are the single source plan
  02-05's `engram catalog` blast-radius column reads — no second literal, no import of
  `internal/server` needed (the leaf package has zero go-sdk dependency).
- The `tool-blast-radius` anchor-region pattern (a generated region with no corresponding
  `surfaces.ConditionalRule`) is now proven reusable — a future generated region needs no new
  anchor mechanism, only a new region id and a render function in `internal/surfacesgen`.
- `.planning/ROADMAP.md`/`REQUIREMENTS.md` still do not reflect D-10's `openWorldHint` scope
  expansion (per 02-CONTEXT.md's standing note and rule `8dfdhfs5nn`) — that roadmap edit remains
  the user's outstanding action via `/gsd-phase`, not performed by this plan.

## Self-Check: PASSED

- FOUND: `internal/surfaces/toolclass.go`
- FOUND: `internal/surfaces/toolclass_test.go`
- FOUND: `internal/server/toolannotations.go`
- FOUND: `internal/server/toolannotations_test.go`
- FOUND commit `fb8b5db6` in `git log --oneline --all`
- FOUND commit `56f4c787` in `git log --oneline --all`

---
*Phase: 02-interface-discoverability*
*Completed: 2026-08-05*
