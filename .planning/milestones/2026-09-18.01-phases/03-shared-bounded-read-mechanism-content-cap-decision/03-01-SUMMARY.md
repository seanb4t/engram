---
phase: 03-shared-bounded-read-mechanism-content-cap-decision
plan: 01
subsystem: config
tags: [content-cap, tags-cap, mcp, connect, validation, D-01, D-09, D-10]

requires:
  - phase: 02-error-classification-resourceexhausted-mapping
    provides: store.ErrResponseTooLarge sentinel and the shared field/hint rejection envelope this plan reuses (no new HintCode)
provides:
  - "ENGRAM_MEMORY_MAX_CONTENT_BYTES (default 65536), ENGRAM_MEMORY_MAX_TAGS (128), ENGRAM_MEMORY_MAX_TAG_BYTES (128) — registry-declared, always-enforced config fields"
  - "memoryWriteCaps{contentBytes,tags,tagBytes} on deps, with resolved() defaulting any zero field, wired through buildDepsFromEnv"
  - "checkContentBytes/checkTags enforced once inside validateStoreArgs, shared by store_memory/schedule_memory/supersede_memory on both MCP and Connect"
affects: [03-02, 03-03, 03-05]

actuals:
  tokens: 11053
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "registry -> MemoryConfig -> Config.Validate -> deps.writeCaps pipeline, mirroring the existing MaxSummaryBytes precedent but diverging on the 0-disables convention (D-09)"
    - "memoryWriteCaps.resolved() as the single place a zero-value cap becomes the documented default, so a bare &deps{} test literal is never uncapped"

key-files:
  created:
    - internal/server/contentcap_test.go
  modified:
    - internal/config/registry.go
    - internal/config/config.go
    - internal/config/validate.go
    - internal/config/validate_test.go
    - internal/config/config_test.go
    - internal/config/service_auth_test.go
    - internal/server/tools.go
    - internal/server/schemarequired_test.go
    - internal/server/argattribution_test.go

key-decisions:
  - "D-01: ENGRAM_MEMORY_MAX_CONTENT_BYTES (default 65536) rejects oversized memory content on every create path with the EXISTING field=content hint=too_long envelope — no new HintCode."
  - "D-09: the three new caps (content, tags, tag bytes) are ALWAYS enforced — Config.Validate rejects 0/negative/non-integer, a deliberate documented divergence from ENGRAM_MEMORY_MAX_SUMMARY_BYTES's 0-disables convention, because plan 03-02's read-side ceiling is derived from these caps."
  - "D-10: ENGRAM_MEMORY_MAX_TAGS (128) and ENGRAM_MEMORY_MAX_TAG_BYTES (128) reject too-many-tags/oversized-tag with the existing field=tags hint=too_many/too_long envelopes, reusing validateCitations' rejection shape."

requirements-completed: [REQ-content-cap-decided]

coverage:
  - id: D1
    description: "ENGRAM_MEMORY_MAX_CONTENT_BYTES flows from config to an MCP store_memory rejection end to end (D-01)"
    requirement: "REQ-content-cap-decided"
    verification:
      - kind: integration
        ref: "internal/server/contentcap_test.go#TestMemoryContentCapFlowsFromConfig"
        status: pass
    human_judgment: false
  - id: D2
    description: "store_memory/schedule_memory/supersede_memory on MCP, and StoreMemory/ScheduleMemory on Connect, all reject oversized content and tags with the existing envelopes and store nothing"
    requirement: "REQ-content-cap-decided"
    verification:
      - kind: integration
        ref: "internal/server/contentcap_test.go#TestMemoryWriteCapsRejectOnEveryCreateLane"
        status: pass
    human_judgment: false
  - id: D3
    description: "Exact boundaries, byte-vs-rune precision, empty-input handling, rejection order, configured non-default caps, and zero-value-deps-is-capped are all proven directly against deps.storeMemory"
    requirement: "REQ-content-cap-decided"
    verification:
      - kind: unit
        ref: "internal/server/contentcap_test.go#TestMemoryWriteCapBoundaries"
        status: pass
    human_judgment: false
  - id: D4
    description: "The three caps cannot be configured to 0 or negative; config defaults are pinned to the registry by test"
    requirement: "REQ-content-cap-decided"
    verification:
      - kind: unit
        ref: "internal/config/validate_test.go#TestMemoryCapsRejectZeroAndNonPositive"
        status: pass
      - kind: unit
        ref: "internal/config/config_test.go#TestMemoryCapDefaultsAndEnv"
        status: pass
      - kind: unit
        ref: "internal/server/contentcap_test.go#TestMemoryWriteCapDefaultsMatchRegistry"
        status: pass
    human_judgment: false
  - id: D5
    description: "No rejection ever echoes the submitted tag value; discovery/rule content bounds and existing validation tests stay unbroken"
    verification:
      - kind: unit
        ref: "internal/server/argattribution_test.go#TestHintNeverEchoesValue"
        status: pass
      - kind: unit
        ref: "internal/server/argattribution_test.go#TestValidationErrorAttributionMatrix"
        status: pass
    human_judgment: false

duration: 35min
completed: 2026-09-19
status: complete
---

# Phase 3 Plan 1: Content/Tags Write Caps Summary

**Registry-declared, always-enforced `ENGRAM_MEMORY_MAX_CONTENT_BYTES`/`ENGRAM_MEMORY_MAX_TAGS`/`ENGRAM_MEMORY_MAX_TAG_BYTES` reject oversized memory writes on every create path (MCP + Connect) with the existing `too_long`/`too_many` envelopes — no new hint code, no silent truncation.**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-09-19T19:30:00Z
- **Completed:** 2026-09-19T20:04:35Z
- **Tasks:** 3 completed
- **Files modified:** 10 (1 created, 9 modified)

## Accomplishments

- Decided and enforced D-01: `ENGRAM_MEMORY_MAX_CONTENT_BYTES` (default 65536 bytes, always enforced) caps memory `content` on `store_memory`, `schedule_memory` and `supersede_memory`, on both the MCP and Connect lanes, via one shared `validateStoreArgs` check reusing the existing `field=content hint=too_long` envelope.
- Decided and enforced D-09: unlike `ENGRAM_MEMORY_MAX_SUMMARY_BYTES`'s "0 disables" convention, the three new caps reject `0`, negative, and non-integer values at config load — `validatePositiveCap` — because plan 03-02's read-side per-record ceiling is derived from these caps.
- Decided and enforced D-10: `ENGRAM_MEMORY_MAX_TAGS` (128) and `ENGRAM_MEMORY_MAX_TAG_BYTES` (128) cap the tag count and per-tag byte length on the same three create paths, reusing `field=tags hint=too_many`/`too_long`.
- `memoryWriteCaps{contentBytes,tags,tagBytes}.resolved()` guarantees a zero-value `deps{}` test literal is never uncapped.
- Hermetic proofs on both lanes: an in-memory MCP session and an `httptest`-backed Connect client against a spy store, plus direct `deps.storeMemory` boundary/order/precision tests — no Qdrant needed.

## Task Commits

Each task was committed atomically:

1. **Task 1: End to end — `ENGRAM_MEMORY_MAX_CONTENT_BYTES` flows to an MCP `store_memory` rejection** - `07a39b75` (feat)
2. **Task 2: The caps are always enforced — validation rejects 0/non-positive, defaults pinned to the registry** - `e8b6a6ad` (feat)
3. **Task 3: Tags caps plus every create lane — MCP + Connect reject oversized content and tags** - `8a01e7a3` (feat)

_All three tasks carried `tdd="true"`; each RED was observed by temporarily reverting the just-added enforcement call, confirming the target test(s) failed, then restoring the fix and re-confirming GREEN (see "TDD Gate Compliance" below)._

## Files Created/Modified

- `internal/config/registry.go` - added `memory.max_content_bytes`/`memory.max_tags`/`memory.max_tag_bytes` registry rows
- `internal/config/config.go` - `MemoryConfig.MaxContentBytes`/`.MaxTags`/`.MaxTagBytes` fields
- `internal/config/validate.go` - `validatePositiveCap`, applied to the three always-enforced caps
- `internal/config/validate_test.go` - `TestMemoryCapsRejectZeroAndNonPositive`; `validConfig()` literal extended
- `internal/config/config_test.go` - `TestMemoryCapDefaultsAndEnv`; test literal extended
- `internal/config/service_auth_test.go` - test literal extended (D-09/D-10 fields)
- `internal/server/tools.go` - `memoryWriteCaps`, `resolved()`, `positiveIntOrDefault`, `memoryWriteCapsFromConfig`, `deps.writeCaps`, `checkContentBytes`, `checkTags`, `validateStoreArgs` signature + the three call sites
- `internal/server/schemarequired_test.go` - 7 `validateStoreArgs(a, 512)` calls updated to the new 3-arg signature
- `internal/server/argattribution_test.go` - 3 new matrix rows (`store_content_too_large`/`store_too_many_tags`/`store_tag_too_long`) + `tag_value_no_echo`
- `internal/server/contentcap_test.go` (new) - `TestMemoryContentCapFlowsFromConfig`, `TestMemoryWriteCapsRejectOnEveryCreateLane`, `TestMemoryWriteCapBoundaries`, `TestMemoryWriteCapDefaultsMatchRegistry`, plus the shared MCP/Connect test harness helpers

## Decisions Made

See `key-decisions` in frontmatter (D-01, D-09, D-10). All three were locked in `03-CONTEXT.md` by the user in discuss-phase on 2026-09-19 — no checkpoint was inserted for any of them (each carries `reversibility rating="one-way"` with the user's prior confirmation recorded).

## Deviations from Plan

None - plan executed exactly as written. All acceptance criteria (grep-based `rg -o` counts, `go test -list`, `go vet`, `golangci-lint`, `task license:check`, `git diff --exit-code -- go.mod go.sum`) pass exactly as specified in each task.

## TDD Gate Compliance

| Task | RED observed | GREEN restored | Commit scope |
|------|---------------|-----------------|---------------|
| 1 | Yes — temporarily removed the `checkContentBytes` call inside `validateStoreArgs`; `TestMemoryContentCapFlowsFromConfig/over` and `/at` both failed (`contentcap_test.go:95: IsError = false, want true`; `contentcap_test.go:126: ... spy holds 2 record(s), want 1`) | Yes — restored the call, both subtests pass | `feat(config,server)` — plumbing + enforcement + test landed together per plan's Task 1 action, RED/GREEN observed via a scoped temporary revert rather than separate commits |
| 2 | Yes — temporarily removed the three `validatePositiveCap` calls; every `0`/`-1`/`abc`/`""` row of `TestMemoryCapsRejectZeroAndNonPositive` failed (18 of 21 subtests) | Yes — restored the calls, all 21 subtests pass | `feat(config)` |
| 3 | Yes — temporarily removed the `checkTags` call; every tag-count/tag-byte row across `TestMemoryWriteCapsRejectOnEveryCreateLane`, `TestMemoryWriteCapBoundaries` failed (`contentcap_test.go:434: storeMemory(tags-over-count): want error, got nil`) | Yes — restored the call, all rows pass | `feat(server)` |

Each RED was produced by a scoped `Edit`/re-`Edit` revert-and-restore cycle (never `git stash`), confirmed via `go vet` + the target test's `-v` output, then reverted before the task's single commit — matching this plan's `tdd="true"` frontmatter without needing separate RED/GREEN commits (the plan's own action text specifies "commit as" a single commit per task, not a RED→GREEN commit pair).

## Issues Encountered

None.

## Known Stubs

None. No hardcoded empty values, placeholder text, or unwired data sources were introduced.

## Threat Flags

None. This plan's own `<threat_model>` register (T-03-01-01 through T-03-01-04, T-03-01-SC) covers every new surface introduced (content/tags size validation, config tampering, cross-lane bypass); no new surface fell outside it.

## User Setup Required

None - no external service configuration required. The three new env vars (`ENGRAM_MEMORY_MAX_CONTENT_BYTES`, `ENGRAM_MEMORY_MAX_TAGS`, `ENGRAM_MEMORY_MAX_TAG_BYTES`) have documented registry defaults and need no operator action unless a non-default value is desired.

## Next Phase Readiness

- The three write caps (`memoryWriteCaps`, always resolved via `resolved()`) are now the exact inputs plan 03-02 needs to derive the per-record read-side byte ceiling.
- `update_memory`'s Connect lane (which bypasses `validateUpdateArgs`), the legacy over-cap record contract, the store-side record ceiling wiring, the CLI end-to-end proof, and all documentation are explicitly out of scope here — they land in plans 03-02, 03-03, and 03-05 per this plan's own objective statement.
- No blockers.

---
*Phase: 03-shared-bounded-read-mechanism-content-cap-decision*
*Completed: 2026-09-19*

## Self-Check: PASSED

- All 10 key files (created + modified) confirmed present on disk.
- All 3 task commit hashes (`07a39b75`, `e8b6a6ad`, `8a01e7a3`) confirmed present in `git log --oneline --all`.
- `go test ./internal/server/... ./internal/config/... -count=1` re-run: `ok`.
- `go test ./internal/keylinks/ -run TestNoEscapedPatternsRepoWide -count=1`: `ok`.
- `go test ./internal/store/ -run TestRedEvidencePatchesAreLive -count=1`: `ok`.
- `task lint` / `task license:check` / `git diff --exit-code HEAD -- go.mod go.sum`: all clean.
