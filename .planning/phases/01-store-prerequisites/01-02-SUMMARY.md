---
phase: 01-store-prerequisites
plan: 02
subsystem: database
tags: [qdrant, facet, recall-gate, store]

# Dependency graph
requires:
  - phase: 01-store-prerequisites
    provides: "01-01's Store package conventions (getWritable, ownerScopeFilter, span+telemetry wrapper shape) that this plan's ListTags reuses"
provides:
  - "Store.ListTags(ctx, subj, scope, limit) — exact, recall-visible, authz-scoped per-tag counts over a filtered Qdrant Facet"
  - "Store.facetTags — the package's ONE filtered Facet call site, reusable by plan 01-04's RelatedMemories rarity weights (D-07)"
  - "Store.recallVisibleFilter — the shared read filter (ownerScopeFilter + the three recall-gate conditions) for caller-facing aggregate reads added this milestone"
  - "a tags keyword payload index in ensureIndexes (D-16), reaching existing collections idempotently on next boot"
  - "the recall gate (schemaversion_recallgate_test.go) widened in all four vocabulary lists for Facet and the ListTags entry point"
affects:
  - "Phase 3 — the ListTags Connect RPC and list_tags MCP tool (RPC-04) wrap this method directly"
  - "Plan 01-04 — RelatedMemories' shared-tag rarity weighting (D-07) reuses facetTags so both features read the same numbers"
  - "Phase 5 — the console tag cloud renders ListTags' counts"

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
actuals:
  tokens: 8872
  tasks: 3
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Filtered-Facet aggregate: recallVisibleFilter composes ownerScopeFilter plus the three recall-gate conditions (active window, not-superseded, not-archived) once; facetTags is the ONLY Facet call in the package that carries that filter, contrasted explicitly with Store.MigrateStatus's deliberately unfiltered operator-tier Facet."
    - "Truncation via over-ask: FacetResponse carries no truncation flag, so facetTags requests Limit: limit+1 and derives more := len(out) > limit — the same idiom ListScopes uses for its own (out, more, err) shape."

key-files:
  created:
    - internal/store/listtags.go
    - internal/store/listtags_test.go
  modified:
    - internal/store/store.go
    - internal/store/schemaversion_recallgate_test.go

key-decisions:
  - "D-13..D-16 implemented exactly as CONTEXT.md locked them: recall-visible-only counts, cross-spine empty scope, exact Facet counts with a limit+more truncation signal, and a tags keyword index in ensureIndexes."
  - "recallVisibleFilter and facetTags are new shared primitives (not named in D-13..D-16 but required by the interfaces section) so plan 01-04's RelatedMemories can reuse the identical filtered Facet for its rarity weighting (D-07) without a second Facet call site to keep in sync."
  - "Task 2's five contract tests all passed against Task 1's tracer implementation on first run — no GREEN-phase fix to listtags.go was needed, recorded rather than inventing a RED phase that did not occur."
  - "Widened all four recall-gate vocabulary lists (recallEmissionMethods, recognizedFilterCarryingRequestMethods, recallCaptureInterceptor's type switch, recallEntryPointSeeds) plus recallTransmitters and operatorMigrationEmitters' Store.MigrateStatus justification — D-15 named only two of the four, RESEARCH.md's Pitfall 1 caught the other two."

requirements-completed: [STORE-03]

coverage:
  - id: D1
    description: "D-16: ensureIndexes creates a tags keyword payload index idempotently"
    requirement: STORE-03
    verification:
      - kind: unit
        ref: "internal/store/listtags_test.go#TestEnsureIndexesCreatesTagsIndex"
        status: pass
    human_judgment: false
  - id: D2
    description: "Tracer: Store.ListTags returns exact per-tag counts through the filtered Facet for one owner end to end (SC4 core)"
    requirement: STORE-03
    verification:
      - kind: unit
        ref: "internal/store/listtags_test.go#TestListTagsCountsOwnedTags"
        status: pass
    human_judgment: false
  - id: D3
    description: "D-13: counts cover recall-visible records only — archived, superseded, expired, and not-yet-active scheduled records are never counted"
    requirement: STORE-03
    verification:
      - kind: unit
        ref: "internal/store/listtags_test.go#TestListTagsRecallVisibleOnly"
        status: pass
    human_judgment: false
  - id: D4
    description: "V4 authz read filter: another owner's private tags are never counted, shared records count for an authenticated reader, anonymous counts only the ownerless bucket, nil Subject fails closed to a non-nil empty slice"
    requirement: STORE-03
    verification:
      - kind: unit
        ref: "internal/store/listtags_test.go#TestListTagsReadFilter"
        status: pass
    human_judgment: false
  - id: D5
    description: "D-14: a named scope counts only that scope; an empty scope counts across every scope the caller can read"
    requirement: STORE-03
    verification:
      - kind: unit
        ref: "internal/store/listtags_test.go#TestListTagsScope"
        status: pass
    human_judgment: false
  - id: D6
    description: "D-13 parity / transparency prohibition: every returned count matches List's total for the same tag filter, over a mixed fixture and two scope modes; ListTags is read-only (idempotent)"
    requirement: STORE-03
    verification:
      - kind: unit
        ref: "internal/store/listtags_test.go#TestListTagsMatchesListTotals"
        status: pass
    human_judgment: false
  - id: D7
    description: "D-15: exact limit/more boundary — truncation below the distinct-tag count, no truncation at exactly the limit, default 100 for limit 0, rejection above MaxRecallLimit naming \"limit\""
    requirement: STORE-03
    verification:
      - kind: unit
        ref: "internal/store/listtags_test.go#TestListTagsLimitAndMore"
        status: pass
    human_judgment: false
  - id: D8
    description: "D-15 gate requirement: the recall gate's static AST scan and live interceptor both recognize the filtered Facet and ListTags as a seeded caller-facing recall entry point"
    requirement: STORE-03
    verification:
      - kind: unit
        ref: "internal/store/schemaversion_recallgate_test.go#TestRecallEmissionSetIsCompleteAndClassified"
        status: pass
      - kind: unit
        ref: "internal/store/schemaversion_recallgate_test.go#TestSchemaVersionNeverGatesRecall"
        status: pass
    human_judgment: false
  - id: D9
    description: "Concurrency: ListTags counts are an exact point-in-time Qdrant facet snapshot under the caller's read + recall-gate filter — concurrent writes may shift counts between calls but can never expose a record the caller cannot read"
    requirement: STORE-03
    verification: []
    human_judgment: true
    rationale: "Flagged in the plan's own must_haves as verification: backstop (spec-less fallback, flagged-unverified) — the filter-travels-on-the-request structural argument is sound (same mechanism TestListTagsReadFilter proves for a single snapshot), but no dedicated interleaving test forces a concurrent write mid-Facet the way TestArchiveSurvivesConcurrentUpdate does for plan 01-01's writes. Flagging for the phase verifier rather than silently claiming full proof."
duration: 23min
completed: 2026-09-26
status: complete
---

# Phase 1 Plan 2: ListTags Facet Counts Summary

**`Store.ListTags` returns exact, authz-scoped per-tag counts through one new filtered Qdrant Facet over a `tags` keyword index, with the recall gate widened to see it.**

## Performance

- **Duration:** 23 min
- **Started:** 2026-09-26T02:40:00Z (approximate)
- **Completed:** 2026-09-26T03:02:39Z
- **Tasks:** 3
- **Files modified:** 4 (2 created, 2 modified)

## Accomplishments

- `internal/store/listtags.go` — `TagCount{Tag, Count}`, `listTagsDefaultLimit` (100), `recallVisibleFilter` (the shared read filter: `ownerScopeFilter` plus the three recall-gate conditions), `facetTags` (the package's ONE filtered `Facet` call site), and `Store.ListTags(ctx, subj, scope, limit) (out []TagCount, more bool, err error)`.
- `internal/store/store.go`'s `ensureIndexes` gained a `tags` keyword payload index (D-16) — idempotent by the same `AlreadyExists`-tolerant loop every other index uses, reaching existing collections on next boot with no separate backfill.
- `internal/store/listtags_test.go` — the tracer (`TestListTagsCountsOwnedTags`) plus six contract tests: `TestEnsureIndexesCreatesTagsIndex`, `TestListTagsRecallVisibleOnly` (D-13), `TestListTagsReadFilter` (V4 authz), `TestListTagsScope` (D-14), `TestListTagsMatchesListTotals` (D-13 parity + idempotency), `TestListTagsLimitAndMore` (D-15). All five Task 2 tests passed against Task 1's implementation on the first run.
- `internal/store/schemaversion_recallgate_test.go` widened in all four places that name a Qdrant method or a recall entry point (`recallEmissionMethods`, `recognizedFilterCarryingRequestMethods`, the interceptor's type switch, `recallEntryPointSeeds`), plus a new `recallTransmitters` row for `Store.facetTags` and an amended `Store.MigrateStatus` justification — the static AST scan and the live interceptor both now see `ListTags`' filtered `Facet` and continue to prove `schema_version` never gates recall.
- Tag counts are provably exact: `TestListTagsMatchesListTotals` cross-checks every returned `TagCount` against `Store.List`'s own total for the same tag filter across a fixture spanning live, archived, superseded, expired, scheduled, another owner's private, and a shared record.

## Task Commits

Each task was committed atomically:

1. **Task 1: ListTags end to end — tracer** - `393f2f2e` (feat)
2. **Task 2: The counting contract** - `afaf2593` (test)
3. **Task 3: Widen the recall gate** - `32c2fa8b` (test)
4. **Deviation fix** - `4577e910` (fix)

**Plan metadata:** (this commit, pending) — SUMMARY + STATE/ROADMAP

## Files Created/Modified

- `internal/store/listtags.go` - `TagCount`, `recallVisibleFilter`, `facetTags`, `Store.ListTags`
- `internal/store/listtags_test.go` - 7 tests (1 tracer + 6 contract/index tests)
- `internal/store/store.go` - `ensureIndexes` gained the `tags` keyword index
- `internal/store/schemaversion_recallgate_test.go` - recall gate widened for the filtered Facet and the `ListTags` entry point

## Decisions Made

- D-13..D-16 implemented exactly as CONTEXT.md locked them (see frontmatter `key-decisions`).
- `recallVisibleFilter`/`facetTags` are new shared primitives beyond the two names D-13..D-16 fix, deliberately built so plan 01-04's `RelatedMemories` can reuse the same filtered Facet for its rarity weighting (D-07) without a second call site to keep in sync.
- All four recall-gate vocabulary lists widened, not just the two D-15 named — RESEARCH.md's Pitfall 1 flagged the other two (`recallEmissionMethods`'s static scan and `recallEntryPointSeeds`/`recallTransmitters`), and this plan's own acceptance criteria pinned all four.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed a malformed second package doc comment flagged by golangci-lint**
- **Found during:** plan-level verification (`golangci-lint run ./internal/store/...`), after Task 3 was already committed
- **Issue:** `listtags.go`'s descriptive file comment sat directly above `package store` with no blank line, so Go/revive treated it as a second package doc comment (`store.go` already carries the canonical one) — `package-comments: package comment should be of the form "Package store ..."`.
- **Fix:** Removed the floating comment; its content already lives in the `TagCount` and `facetTags` doc comments, so nothing was lost.
- **Files modified:** `internal/store/listtags.go`
- **Verification:** `golangci-lint run ./internal/store/...` reports 0 issues; `go test ./internal/store/ -count=1` still green (108.5s); acceptance-criteria greps for Task 1 re-verified unchanged.
- **Commit:** `4577e910`

---

**Total deviations:** 1 auto-fixed (1 bug — Rule 1, lint-only, no behavior change).
**Impact on plan:** Cosmetic; required to keep the repo's `task lint` gate green. No scope creep, no behavior change to any store method.

## Issues Encountered

None beyond the deviation above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `Store.ListTags` is ready for Phase 3's `ListTags` Connect RPC and `list_tags` MCP tool (RPC-04) to wrap directly.
- `Store.facetTags` and `Store.recallVisibleFilter` are ready for plan 01-04's `RelatedMemories` rarity weighting (D-07) to reuse without a second Facet call site.
- Plan 01-01's `ArchiveAs`/`RestoreAs` and this plan's `ListTags` share no files (`files_modified` lists were disjoint), confirmed by the full `go test ./internal/store/ -count=1` pass covering both plans' tests together.

---

*Phase: 01-store-prerequisites*
*Completed: 2026-09-26*
