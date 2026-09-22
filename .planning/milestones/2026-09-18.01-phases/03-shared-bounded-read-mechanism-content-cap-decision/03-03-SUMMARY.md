---
phase: 03-shared-bounded-read-mechanism-content-cap-decision
plan: 03
subsystem: server
tags: [content-cap, tags-cap, update-memory, connect, mcp, record-caps, D-01, D-02, D-07, D-09, D-10]

requires:
  - phase: 03-shared-bounded-read-mechanism-content-cap-decision
    provides: "plan 03-01's memoryWriteCaps/checkContentBytes/checkTags (the create-path caps this plan extends to update) and plan 03-02's store.RecordCaps/DefaultRecordCaps/WithRecordCaps (the read-side ceiling this plan wires with production values)"
provides:
  - "Content and tags caps enforced inside deps.updateMemory itself (gated on contentChanged / a changed tag set), closing the one lane (Connect's field-mask UpdateMemory) that bypasses validateUpdateArgs entirely"
  - "recordCapsFromConfig(cfg) — the ONE function building store.RecordCaps from exactly the parsers memoryWriteCapsFromConfig/maxMemorySummaryBytes already use for the write caps, plus the server's citation constants"
  - "storeFromConfig now passes store.WithRecordCaps(recordCapsFromConfig(cfg)) — every production Store (serve, reindex, migrate, prune) reports the configured caps, not silently DefaultRecordCaps()"
affects: [03-04, 03-05, 03-06]

actuals:
  tokens: 15600
  tasks: 3
  commits: 3
  plan_head_before: 61e73902e828e1517ba04bc068b879cc929c1ec1

tech-stack:
  added: []
  patterns:
    - "Gate a mutation-path check on WHAT CHANGED (contentChanged / !slices.Equal(*a.Tags, cur.Tags)), never on presence alone — the D-07 legacy-record contract depends on this: an unchanged over-cap value must never lock its owner out of an otherwise-legitimate update"
    - "A read-side derivation function (recordCapsFromConfig) reuses the EXACT write-side parser (memoryWriteCapsFromConfig) rather than re-parsing the same env vars a second, independent way — the only way the two are provably never allowed to drift"

key-files:
  created:
    - internal/server/updatecap_test.go
    - internal/server/recordcaps_test.go
  modified:
    - internal/server/tools.go

key-decisions:
  - "D-09: the content-cap check on update lives inside deps.updateMemory itself (not validateUpdateArgs), because Connect's UpdateMemory RPC calls deps.updateMemory directly — the exact #360-shaped bypass RESEARCH.md named."
  - "D-10 gating (planner's discretion, following D-09's contentChanged precedent): the tags check on update runs only when the supplied set differs from the stored set (slices.Equal, order-sensitive), so resending a legacy record's own tags is never a rejection."
  - "D-02/D-09 read-side link: recordCapsFromConfig reuses memoryWriteCapsFromConfig + maxMemorySummaryBytes verbatim rather than a second parse, so the store's read ceiling can never silently diverge from what the write side actually enforces."

requirements-completed: [REQ-content-cap-decided, REQ-byte-budget-pages]

coverage:
  - id: D1
    description: "Connect's UpdateMemory field-mask lane rejects an over-cap changed content with the existing field=content hint=too_long envelope, and an at-cap change succeeds and is stored (D-01, D-09)"
    requirement: "REQ-content-cap-decided"
    verification:
      - kind: integration
        ref: "internal/server/updatecap_test.go#TestUpdateMemoryContentCap"
        status: pass
    human_judgment: false
  - id: D2
    description: "The MCP update_memory closure rejects an over-cap changed content the same way, and both lanes reject a changed over-cap tag count/tag byte length while accepting an empty tag list (clear) (D-10)"
    requirement: "REQ-content-cap-decided"
    verification:
      - kind: integration
        ref: "internal/server/updatecap_test.go#TestUpdateMemoryTagsCap"
        status: pass
    human_judgment: false
  - id: D3
    description: "A legacy over-cap record (70000-byte content, 200 tags) stays readable byte-for-byte on both lanes, can be re-shared, re-summarized, and resent unchanged, and can be trimmed by its owner — only a NEW over-cap value is rejected, and a rejection never mutates the stored record (D-07)"
    requirement: "REQ-content-cap-decided"
    verification:
      - kind: integration
        ref: "internal/server/updatecap_test.go#TestUpdateMemoryLegacyOversizedRecord"
        status: pass
    human_judgment: false
  - id: D4
    description: "recordCapsFromConfig derives store.RecordCaps from exactly the write-side parsers, its defaults are test-bound to the registry and the server's citation constants, and the ONE production store construction site (storeFromConfig) actually wires WithRecordCaps with a live Qdrant client"
    requirement: "REQ-byte-budget-pages"
    verification:
      - kind: unit
        ref: "internal/server/recordcaps_test.go#TestRecordCapsFromConfig"
        status: pass
      - kind: unit
        ref: "internal/server/recordcaps_test.go#TestDefaultRecordCapsMatchRegistryDefaults"
        status: pass
      - kind: integration
        ref: "internal/server/recordcaps_test.go#TestStoreFromConfigCarriesRecordCaps"
        status: pass
    human_judgment: false

duration: 10min
completed: 2026-09-19
status: complete
---

# Phase 3 Plan 3: Update-Lane Content/Tags Caps & Production Record-Ceiling Wiring Summary

**Closes the one non-obvious write-path gap (Connect's `UpdateMemory` bypassing `validateUpdateArgs`) by putting the content/tags caps inside `deps.updateMemory` itself, gated on what actually changed, and wires the production store's read-side per-record ceiling (`store.WithRecordCaps`) to exactly those same configured caps via a new `recordCapsFromConfig`.**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-09-19T21:10:00Z
- **Completed:** 2026-09-19T21:20:37Z
- **Tasks:** 3 completed
- **Files modified:** 3 (2 created, 1 modified)

## Accomplishments

- D-01/D-09: `deps.updateMemory` now rejects an over-cap CHANGED `content` with the existing `field=content hint=too_long` envelope, gated on `contentChanged` — the check Connect's field-mask `UpdateMemory` RPC could never reach when it lived only in `validateUpdateArgs` (the exact #360-shaped trap RESEARCH.md names, since Connect calls `deps.updateMemory` directly and never runs `validateUpdateArgs`).
- D-10: the same function now rejects a CHANGED over-cap tag count/tag byte length (`checkTags`, gated on `!slices.Equal(*a.Tags, cur.Tags)`), on both the MCP and Connect lanes; an empty tag list (clear) is always accepted.
- D-07: `TestUpdateMemoryLegacyOversizedRecord` pins the full legacy contract on a record seeded directly into the spy store (bypassing every server cap, the shape a pre-cap record takes in production) — it stays readable byte-for-byte via `get_memory` and `GetMemory`, can be re-shared (`shared`-only mask), have its content resent unchanged with a new summary, have its tags resent unchanged, and be trimmed down by its owner; only a genuinely NEW over-cap content or tag change is rejected, and every rejection leaves the stored record untouched.
- D-02/D-09: new `recordCapsFromConfig(cfg)` builds `store.RecordCaps` from EXACTLY the same parsers (`memoryWriteCapsFromConfig`, `maxMemorySummaryBytes`) the write side already uses, plus the server's own citation constants (`maxDiscoveryCitations`, `maxCitationExcerptBytes`) — never a second, independent config read. `storeFromConfig` — the one production store construction site `serve`, `reindex`, `migrate`, and `prune` all funnel through — now passes `store.WithRecordCaps(recordCapsFromConfig(cfg))`, proven against a live Qdrant client in `TestStoreFromConfigCarriesRecordCaps`.
- `TestDefaultRecordCapsMatchRegistryDefaults` is the drift gate: with the four cap env vars cleared, `recordCapsFromConfig`'s output equals `store.DefaultRecordCaps()` field for field, binding the store's defaults to the registry defaults and the server's citation constants by test, not by reading.

## Task Commits

Each task was committed atomically:

1. **Task 1: End to end — Connect UpdateMemory's field-mask lane rejects over-cap content inside deps.updateMemory** - `c6c56f0b` (feat, tracer)
2. **Task 2: Tags on update, the MCP update lane, and a legacy over-cap record that stays readable and trimmable** - `a0837fd5` (feat)
3. **Task 3: The production store derives its read ceilings from the configured caps** - `cf16ff6c` (feat)

_All three tasks carried `tdd="true"` (Task 1 additionally `type="tracer"`); each RED was observed directly (test written and run BEFORE the corresponding production fix, in the same working tree, before any commit), then the fix was added and GREEN re-confirmed, then committed once. See "TDD Gate Compliance" below._

## Files Created/Modified

- `internal/server/tools.go` - `deps.updateMemory` gains the content-cap check (gated on `contentChanged`) and the tags-cap check (gated on `!slices.Equal(*a.Tags, cur.Tags)`), both placed after `FetchForUpdate`'s owner gate and before `resolveSummaryUpdate`/the embed call; new `recordCapsFromConfig(cfg *config.Config) store.RecordCaps`; `storeFromConfig` now constructs `store.New(qc, cfg.Qdrant.Collection, store.WithRecordCaps(recordCapsFromConfig(cfg)))`
- `internal/server/updatecap_test.go` (new) - `TestUpdateMemoryContentCap` (Connect + MCP over-cap/at-cap changed content), `TestUpdateMemoryTagsCap` (Connect + MCP too-many/too-long/at-cap/empty-clears), `TestUpdateMemoryLegacyOversizedRecord` (8 ordered subtests: get-mcp, get-connect, share-only, resend-same-content, resend-same-tags, change-to-oversized, add-a-tag, trim)
- `internal/server/recordcaps_test.go` (new) - `TestRecordCapsFromConfig`, `TestDefaultRecordCapsMatchRegistryDefaults`, `TestStoreFromConfigCarriesRecordCaps` (live Qdrant)

## Decisions Made

See `key-decisions` in frontmatter (D-09's placement, D-10's `contentChanged`-mirroring gate choice, D-02/D-09's single-parse read-side link). All three were locked or explicitly delegated to planner's discretion in `03-CONTEXT.md`/`03-RESEARCH.md`; no new architectural decision was made in this plan.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `storeFromConfig`'s doc comment accidentally duplicated its own acceptance-criteria grep target**
- **Found during:** Task 3 (acceptance criteria pass)
- **Issue:** The doc comment added above `storeFromConfig` quoted the literal expression `store.WithRecordCaps(recordCapsFromConfig(cfg))` verbatim, which made `rg -o 'store[.]WithRecordCaps[(]recordCapsFromConfig[(]cfg[)][)]' internal/server/tools.go | wc -l` count 2 matches (the doc comment plus the real code) instead of the required 1.
- **Fix:** Reworded the doc comment to describe the wiring without repeating the exact literal expression — no behavior change, doc-only.
- **Files modified:** internal/server/tools.go
- **Verification:** `rg -o ... | wc -l` now prints `1`, matching the acceptance criterion exactly.
- **Committed in:** cf16ff6c (Task 3 commit)

---

**Total deviations:** 1 auto-fixed (1 bug, doc-comment-only). **Impact:** No behavior change; the fix only tightened wording so the plan's own acceptance grep counts correctly.

## TDD Gate Compliance

| Task | RED observed | GREEN restored | Commit scope |
|------|---------------|-----------------|---------------|
| 1 | Yes — `TestUpdateMemoryContentCap` written and run against unmodified `tools.go`; `--- FAIL: TestUpdateMemoryContentCap/connect_over` (`updatecap_test.go:53: UpdateMemory(over-cap content): err = nil, want CodeOutOfRange`) | Yes — added the `contentChanged`-gated `checkContentBytes` call; both `connect_over` and `connect_at_cap` (and the later-added `mcp_over`/`mcp_at_cap`) pass | `feat(server)` — test + fix landed together per the plan's Task 1 action ("test first, observe RED, add the check, observe GREEN"), RED genuinely observed in the working tree before the single commit |
| 2 | Yes — `TestUpdateMemoryTagsCap`/`TestUpdateMemoryLegacyOversizedRecord` written and run against Task 1's committed state (no tags check yet); four subtests failed: `TestUpdateMemoryTagsCap/connect/too_many`, `/connect/too_long`, `/mcp/too_many`, `TestUpdateMemoryLegacyOversizedRecord/add-a-tag` | Yes — added the `slices.Equal`-gated `checkTags` call; all rows across all three tests pass, plus the full `./internal/server/...` suite | `feat(server)` |
| 3 | Yes — `recordcaps_test.go` written (with `recordCapsFromConfig` already added) and run against `storeFromConfig` still calling bare `store.New(qc, cfg.Qdrant.Collection)`; `--- FAIL: TestStoreFromConfigCarriesRecordCaps` | Yes — changed `storeFromConfig`'s return to include `store.WithRecordCaps(recordCapsFromConfig(cfg))`; all three tests pass, plus `./internal/server/...` and `./internal/store/...` under `ENGRAM_REQUIRE_QDRANT=1` | `feat(server)` |

Each RED was produced by writing the test file(s) and running them against the not-yet-fixed production code in the SAME working tree (never `git stash`), confirmed via `go vet` + the target test's `-v` output, then the fix was added and GREEN re-confirmed, then committed once per task — matching this plan's `tdd="true"` frontmatter without a separate RED/GREEN commit pair (the plan's own action text specifies "commit as" a single commit per task).

## Issues Encountered

None.

## Known Stubs

None. No hardcoded empty values, placeholder text, or unwired data sources were introduced.

## Threat Flags

None. This plan's own `<threat_model>` register (T-03-03-01 through T-03-03-04, T-03-03-SC) covers every new surface this plan introduces (the Connect field-mask bypass now closed, the D-07 owner-lockout risk, the owner-gate ordering that prevents information disclosure on rejection, and the read-ceiling/write-cap drift risk); no new surface fell outside it.

## User Setup Required

None - no external service configuration required. Both caps continue to use plan 03-01's existing env vars (`ENGRAM_MEMORY_MAX_CONTENT_BYTES`, `ENGRAM_MEMORY_MAX_TAGS`, `ENGRAM_MEMORY_MAX_TAG_BYTES`); no new env var was introduced.

## Next Phase Readiness

- The write-side content/tags caps now cover every memory write path including `update_memory` on both lanes (D-01/D-09/D-10 fully implemented for this phase's scope).
- The production store now reports the configured `RecordCaps` via `storeFromConfig` → `store.WithRecordCaps`, giving plans 03-04/03-05 (the ordered-page helper wiring and remaining migration work) a live, config-derived read ceiling to build on.
- No red-evidence patches are registered by this plan — RESEARCH.md/this plan's own executor notes confirm plan 03-06 registers Phase 3's patches after the last plan.
- No blockers.

---
*Phase: 03-shared-bounded-read-mechanism-content-cap-decision*
*Completed: 2026-09-19*

## Self-Check: PASSED

- All 3 key files (2 created + 1 modified) confirmed present on disk.
- All 3 task commit hashes (`c6c56f0b`, `a0837fd5`, `cf16ff6c`) confirmed present in `git log --oneline --all`.
- `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... ./internal/store/... -count=1`: `ok`.
- `go test ./internal/keylinks/ -run TestNoEscapedPatternsRepoWide -count=1`: `ok`.
- `task lint` / `task license:check` / `git diff --exit-code HEAD -- go.mod go.sum`: all clean.
- `go vet ./internal/server/`: clean. `golangci-lint run ./internal/server/...`: 0 issues.
