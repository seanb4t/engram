---
phase: 07-console-cli-state-surfacing
reviewed: 2026-08-21T00:00:00Z
depth: standard
files_reviewed: 47
files_reviewed_list:
  - cmd/engram/client_common.go
  - cmd/engram/client_common_test.go
  - cmd/engram/client_get.go
  - cmd/engram/client_get_test.go
  - cmd/engram/client_list.go
  - cmd/engram/client_list_test.go
  - cmd/engram/client_migration_status.go
  - cmd/engram/client_migration_status_test.go
  - cmd/engram/client_search.go
  - cmd/engram/client_search_test.go
  - cmd/engram/clienttest_test.go
  - cmd/engram/cmdwalk.go
  - cmd/engram/cmdwalk_test.go
  - cmd/engram/memory_state.go
  - cmd/engram/memory_state_test.go
  - cmd/engram/operator_output_test.go
  - cmd/engram/operator_view.go
  - cmd/engram/operator_view_test.go
  - internal/server/connectapi.go
  - internal/server/connectapi_test.go
  - internal/server/connectdescriptor_test.go
  - internal/server/fakestore_test.go
  - internal/server/store_iface.go
  - internal/server/tools.go
  - internal/store/migrate_status.go
  - internal/store/migrate_status_test.go
  - internal/store/migratebacklog.go
  - internal/store/store.go
  - internal/store/store_test.go
  - internal/surfaces/toolclass.go
  - proto/engram/v1/engram.proto
  - ui/src/lib/components/AppShell.svelte
  - ui/src/lib/components/AppShell.browser.test.ts
  - ui/src/lib/components/MemoryDetail.svelte
  - ui/src/lib/components/MemoryDetail.browser.test.ts
  - ui/src/lib/components/MemoryRow.svelte
  - ui/src/lib/components/MemoryRow.browser.test.ts
  - ui/src/lib/components/MigrationBanner.svelte
  - ui/src/lib/components/MigrationBanner.browser.test.ts
  - ui/src/lib/components/ScopesSidebar.svelte
  - ui/src/lib/components/ScopesSidebar.browser.test.ts
  - ui/src/lib/errors.ts
  - ui/src/lib/errors.test.ts
  - ui/src/lib/memorystate.ts
  - ui/src/lib/memorystate.test.ts
  - ui/src/lib/queries.ts
  - ui/src/lib/queries.test.ts
  - ui/src/routes/+layout.svelte
  - ui/src/routes/observe/+page.svelte
findings:
  critical: 0
  warning: 0
  info: 1
  total: 1
status: clean
---

# Phase 07: Code Review Report

**Reviewed:** 2026-08-21
**Depth:** standard
**Files Reviewed:** 47
**Status:** clean

## Summary

This review traced the diff (`68cc585c..HEAD`) for every file in scope against the seven areas of
scrutiny named in the task: recall-gate relaxation correctness, authorization orthogonality, the
dual (Go/TS) state-word derivation, terminal-injection sanitization, the new `MigrateStatus` RPC,
Svelte 5 correctness, and `queries.ts` URL/cache-key round-tripping. Each was verified against the
actual source, not the summaries' claims about it.

**Recall-gate scoping (area 1) holds exactly as specified.** `Store.Search` and `Store.List` each
gained three independent `if !opts.IncludeX { f.Must = append(...) }` guards, one per gate condition
(`archived_at` IsEmpty, `superseded_by` IsEmpty, `activeWindowConditions` as one atomic unit for
`IncludeScheduled`). `Store.SearchDiscovery` (no `SearchOptions` parameter at all — structurally
incapable of accepting the flags) and `Store.ListScheduled` (accepts a `ListOptions` value but its
own inline filter hardcodes `IsEmpty("superseded_by")`/`IsEmpty("archived_at")`, never reading
`opts.IncludeArchived`/`opts.IncludeSuperseded`) are untouched by the relaxation, matching the
declared 2-of-4 scope. `TestArchiveRecallGateListScheduled`'s negative assertion (all three flags set
true, archived record still excluded) is a real, non-vacuous check of this boundary.

**Authorization orthogonality (area 2) holds.** In both `Store.Search` and `Store.List`, the
owner/scope filter (`s.ownerScopeFilter(...)` / `s.listFilter(...)`) is constructed and appended to
`f.Must` *before* any of the three conditional include-blocks run; the include-blocks only ever add
or withhold state-gate conditions, never touch or reorder the authz condition.
`TestSearchAndListAuthorizationOrthogonalToState` is a genuine cross-owner test: sub-B with all three
flags set sees zero of sub-A's private records (live or archived) and *does* see sub-A's shared
archived/superseded records — proving state relaxation neither widens nor narrows authorization. The
new `MigrateStatus` Connect handler checks `subjectFromConnectContext` (auth) before calling the
store and discards the resolved subject, matching D-06's "any authenticated caller, no owner filter"
design exactly, mirroring `ListScopes`' existing shape.

**State-word derivation (area 3) agrees between `cmd/engram/memory_state.go` and
`ui/src/lib/memorystate.ts`.** Both implement the identical boundary semantics: `not_before <= now`
is active (no "scheduled" word, inclusive `Lte`), `not_after > now` is active (exclusive `Gt` bound,
so `not_after == now` yields "expired"), `expired` is evaluated before `scheduled` and suppresses it
on an inverted window, and canonical order is `archived, superseded, expired, scheduled` in both. This
matches `internal/store`'s `activeWindowConditions` (`Lte` on `not_before`, `Gt` on `not_after`)
exactly, so the derived words agree with what the store gate itself relaxes. Both surfaces carry
dedicated, non-trivial boundary-equality and inverted-window test cases.

**Terminal-injection sanitization (area 4) is closed structurally.** `renderOperatorView`'s headline
write is now `fmt.Fprintln(w, sanitizeViewValue(headline))` — the single write site every headline
producer (all 17, including the two new ones from this phase) funnels through, so the fix applies
uniformly rather than per-producer. Field values were already sanitized before this phase
(`viewScalar` calls `sanitizeViewValue` on every JSON string scalar); the phase's new record-content
consumers (`engram get`'s full `Memory`, `engram migration-status`'s histogram) render exclusively
through this same path. `Citation`'s fields (the one repeated-message field on `Memory` that could
carry stored free text, e.g. `excerpt`) are all plain string scalars, so the documented
nested-array/object sanitization gap (WR-02, pre-existing, out of this phase's scope) does not apply
to any field this phase newly exposes.

**`MigrateStatus` RPC (area 5)** returns only aggregate counts (buckets, absent, future, future_total,
total, current_version, pending) — no record content, scope, or owner — matching D-06's stated
disclosure boundary. Error mapping goes through the existing `connectError` classifier and is proven
by a dedicated failure-injection test (`migrateStatusFailStore`) that also asserts no response is
returned alongside the error. The empty/zero-value case is proven to serialize `"buckets":[]` /
`"future":[]` rather than `null`.

**Svelte correctness (area 6):** `MigrationBanner` derives its two counts from `query.data?.field ??
0n`, which collapses loading, a rejected fetch, and a genuine zero/zero response into the identical
"render nothing" branch — verified by three separate tests (zero, loading, rejected-fetch-through-the-
production-`handleQueryError`-wiring), the last of which asserts `console.error` fires exactly once
and the global `errorBanner` store stays `null`. The state badge is never given the row's
`opacity-60` dimming class (verified structurally in the diff and by dedicated
`MemoryRow.browser.test.ts` cases), satisfying the accessibility carve-out. No `@html` or unsanitized
interpolation was introduced by this phase's new/changed template code (the pre-existing
`{@html bodyHtml}` markdown-render call in `MemoryDetail.svelte` is untouched by this diff).

**`queries.ts` round-tripping (area 7):** `INCLUDE_STATES` is the single recognized-value list shared
by `parseObserveParams` (filter-against-known-set) and `observeSearch` (iterate-and-append), so parse
and encode cannot drift out of the same vocabulary; an unrecognized `inc` value is silently dropped
(mirroring the existing `cat` pattern); an all-false `ObserveParams` produces a URL string with zero
`inc` parameters, verified byte-identical to the pre-phase output by an explicit test; `listMemoriesKey`
includes all three booleans as distinct trailing array elements (not a stringified composite), so no
two distinct flag combinations can collide on one cache key.

No Critical or Warning findings were identified in any of the seven scrutiny areas, in the recall-gate
diff itself, in the MigrateStatus RPC and its handler, in the CLI advisory-footer bounded-context
logic, or in the console's error-routing/query wiring. Test files reviewed (store_test.go additions,
memory_state_test.go, memorystate.test.ts, client_get_test.go, connectapi_test.go additions,
client_common_test.go footer/ceiling tests, MigrationBanner.browser.test.ts, errors.test.ts,
queries.test.ts, ScopesSidebar.browser.test.ts, AppShell.browser.test.ts) all assert on concrete
values, DOM ordering, or timing bounds rather than bare identifiers or comments — none were found to
be vacuous or false-green.

## Info

### IN-01: `STATE_WORD_ORDER` exported but unused outside its own test file — RESOLVED

**File:** `ui/src/lib/memorystate.ts:15`
**Issue:** `STATE_WORD_ORDER` is exported and documents the canonical compound-state order, but no
production code imports it — `MemoryRow.svelte` and `MemoryDetail.svelte` both rely on
`memoryStateWords`' own push order (which does match `STATE_WORD_ORDER`'s values) rather than
importing and using this constant directly. It is referenced only by `memorystate.test.ts`, which
asserts `STATE_WORD_ORDER` equals its own literal value — a self-referential check that pins the
constant's value but does not prove any production code actually orders by it. This is not a
correctness defect (the two orderings are independently verified to agree by the derivation-order
tests), just an unused piece of public surface area.
**Fix:** Either have `MemoryRow`/`MemoryDetail` order their rendered state list by iterating
`STATE_WORD_ORDER` and filtering `stateWords.includes(word)` (making the shared constant load-bearing
rather than incidental), or drop the export and inline the ordering rationale as a comment on
`memoryStateWords` alone, mirroring the Go surface (which has no equivalent exported order constant).

**Resolution (2026-08-21, commit e01bf695):** took the first option. `memoryStateWords`
now collects applicable words into a set and returns `STATE_WORD_ORDER` filtered by it, so
the constant drives every consumer's order rather than agreeing with it by coincidence. The
self-referential test assertion was replaced by a sweep over all 16 combinations of the four
state-bearing wire fields asserting the output is a subsequence of `STATE_WORD_ORDER`;
mutation-proved RED against a push-order regression. 263 console tests pass.

---

_Reviewed: 2026-08-21_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
