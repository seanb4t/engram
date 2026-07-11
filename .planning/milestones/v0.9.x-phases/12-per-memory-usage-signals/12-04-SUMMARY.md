---
phase: 12-per-memory-usage-signals
plan: 04
subsystem: observability
tags: [opentelemetry, otel, qdrant, tracing, go]

# Dependency graph
requires:
  - phase: 12-per-memory-usage-signals
    provides: "12-01's Memory.AccessCount/LastAccessedAt fields and store.IncrementAccess primitive (this plan is independent of that payload-counter work but shares the same store.go file)"
provides:
  - "recallIDs(out []Memory, max int) bounded helper capping span attribute cardinality at recallIDCap=50"
  - "engram.recall.ids / engram.recall.count span attributes on store.Search, store.List, and store.Get success paths"
affects: [12-per-memory-usage-signals remaining plans, ClickStack analytics dashboards]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "D-06 hybrid analytics: record-set membership rides existing OTLP recall spans (store layer), never a storage write or a payload counter mutation"
    - "attribute.StringSlice for bounded-cardinality span attributes, always paired with a package-level cap constant"

key-files:
  created: []
  modified:
    - internal/store/store.go
    - internal/store/instrument_test.go

key-decisions:
  - "recallIDCap = 50, a package-level const, per RESEARCH/PATTERNS guidance (no requirement to pick a different value)."
  - "Get's attributes are set inline right before its single success return rather than via the defer, matching the plan's literal insertion point and avoiding a defer-scoped closure over a not-yet-final named return."

patterns-established:
  - "Any future recall-adjacent span attribute addition should follow the same store-layer-only rule (D-06/Pitfall 4): never instrumentTools, which is MCP-only and lacks typed result ids."

requirements-completed: [REQ-usage-signals]

coverage:
  - id: D1
    description: "store.Search, store.List, and store.Get spans carry engram.recall.ids (bounded) and engram.recall.count on their success paths"
    requirement: "REQ-usage-signals"
    verification:
      - kind: integration
        ref: "internal/store/instrument_test.go#TestStoreSearchEmitsRecallIDs"
        status: pass
      - kind: integration
        ref: "internal/store/instrument_test.go#TestStoreListEmitsRecallIDs"
        status: pass
      - kind: integration
        ref: "internal/store/instrument_test.go#TestStoreGetEmitsRecallIDs"
        status: pass
    human_judgment: false
  - id: D2
    description: "engram.recall.ids is capped at a bounded package constant (recallIDCap) even when the true result count exceeds it"
    requirement: "REQ-usage-signals"
    verification:
      - kind: integration
        ref: "internal/store/instrument_test.go#TestStoreListRecallIDsCappedAtLimit"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-07-10
status: complete
---

# Phase 12 Plan 04: Recall-Span Analytics Attributes Summary

**Bounded `engram.recall.ids`/`engram.recall.count` OTel span attributes on store.Search/List/Get, riding the existing OTLP pipeline with zero storage change**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-07-10T17:00:00Z (approx)
- **Completed:** 2026-07-10T17:21:17Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Added the D-06 analytics leg: `recallIDs(out []Memory, max int) []string`, a pure, bounded helper capped by the package const `recallIDCap = 50`, used to populate `engram.recall.ids`.
- `store.Search` and every `store.List` success path now set `engram.recall.ids` (capped) + `engram.recall.count` (true count) alongside the existing `engram.result_count`.
- `store.Get`, which previously had no result-count attribute at all, now emits `engram.recall.ids=[id]` and `engram.recall.count=1` on its success path for ClickStack data-completeness.
- Added span-recorder tests (`withSpanRecorder`/`spanByName` idiom) proving presence and correctness of both attributes on Search/List/Get, plus a dedicated cap test seeding `recallIDCap+5` records and asserting the ids slice truncates to `recallIDCap` while `engram.recall.count` still reports the true total.

## Task Commits

Each task was committed atomically:

1. **Task 1: recallIDs helper + D-06 span attributes on Search/List/Get** - `df8c207` (feat)
2. **Task 2: span-recorder tests for recall-id attributes** - `9fa001e` (test)

**Plan metadata:** (this commit)

## Files Created/Modified
- `internal/store/store.go` - added `recallIDCap`/`recallIDs`; wired `engram.recall.ids`/`engram.recall.count` into the `Search`/`List` defers and `Get`'s success return
- `internal/store/instrument_test.go` - added `spanAttr`, `seedRecallSpanRecords`, `assertRecallAttrs` test helpers plus 4 new tests (`TestStoreSearchEmitsRecallIDs`, `TestStoreListEmitsRecallIDs`, `TestStoreGetEmitsRecallIDs`, `TestStoreListRecallIDsCappedAtLimit`)

## Decisions Made
- Kept the cap value at 50 (RESEARCH/PATTERNS-suggested default) since nothing in CONTEXT/REQUIREMENTS mandated a different bound.
- Used a dedicated `List`-backed cap test rather than duplicating it for `Search`, since both paths share the identical `recallIDs(out, recallIDCap)` call and `List`'s offset/"all" mode makes seeding an exact oversized total straightforward.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required. Attributes are a no-op when OTLP is unconfigured (existing idiom, unchanged).

## Next Phase Readiness

- `store.Search`/`List`/`Get` recall spans now carry ClickStack-consumable record-set membership; no further store-layer work needed for D-06.
- `internal/server/instrument.go` (MCP-only middleware) was intentionally left untouched, per D-06/Pitfall 4.
- Remaining Phase 12 plans (payload counter wiring, config gate, proto/`recallView` exposure) are independent of this plan's changes.

---
*Phase: 12-per-memory-usage-signals*
*Completed: 2026-07-10*

## Self-Check: PASSED
- FOUND: internal/store/store.go
- FOUND: internal/store/instrument_test.go
- FOUND: .planning/phases/12-per-memory-usage-signals/12-04-SUMMARY.md
- FOUND commit: df8c207
- FOUND commit: 9fa001e
