---
phase: 09-report-pending-in-migrate-status
plan: 01
subsystem: cli
tags: [migrate, operator-cli, histogram, pending, go]

# Dependency graph
requires:
  - phase: 04-migration-cli-first-customer
    provides: migrateStatusReportDoc / statusReportDoc / statusSummary and store.MigrateStatusResult.Pending()
provides:
  - "pending" json key in `engram migrate status --output json`
  - unconditional pending clause in `engram migrate status --output text` headline
  - discriminating regression tests proving the value is never re-derived
affects: [09-02-fix-migrate-guide-pending-row]

# Actuals (#2632)
actuals:
  tokens: 2659
  tasks: 2
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "cur-relative discriminating test fixtures (cur := int(migrate.CurrentVersion)) that stay valid after CurrentVersion advances"

key-files:
  created: []
  modified:
    - cmd/engram/migrate_family.go
    - cmd/engram/migrate_family_test.go

key-decisions:
  - "Pending uint64 (json:\"pending\") appended as the 7th and final field of migrateStatusReportDoc, never inserted next to Absent — preserves the struct's documented first-five-fields-mirror-the-store-type invariant (D-01)"
  - "statusReportDoc populates Pending via a single res.Pending() call — no second loop, accumulator, or inlined copy of the store method's arithmetic (D-02)"
  - "statusSummary's pending clause is emitted unconditionally (present at zero pending too), positioned between the per-bucket enumeration and the conditional future clause (D-03)"
  - "Clause wording chosen: '; %d pending' — satisfies the two constraints (contains the lowercase token 'pending', introduces no digits besides the pending count itself) while staying terse; wording carries no stability contract per D-04"

patterns-established:
  - "Discriminating-fixture test design: a fixture chosen so the correct arithmetic and every plausible naive rederivation are pairwise-distinct, so a copy-paste regression fails loudly instead of silently passing on a degenerate fixture"

requirements-completed: [REQ-migrate-status-histogram, REQ-docs-record-state]

coverage:
  - id: D1
    description: "engram migrate status reports pending in --output json (migrateStatusReportDoc.Pending, json tag pending, last field) sourced from store.MigrateStatusResult.Pending()"
    requirement: "REQ-migrate-status-histogram"
    verification:
      - kind: unit
        ref: "cmd/engram/migrate_family_test.go#TestMigrateFamilyStatusReportDocPendingNeverRederived"
        status: pass
      - kind: unit
        ref: "cmd/engram/migrate_family_test.go#TestMigrateFamilyStatusReportDocKeyOrder"
        status: pass
    human_judgment: false
  - id: D2
    description: "engram migrate status text headline carries an unconditional pending clause between the bucket enumeration and the future clause, sourced from the same res.Pending() call"
    requirement: "REQ-migrate-status-histogram"
    verification:
      - kind: unit
        ref: "cmd/engram/migrate_family_test.go#TestMigrateFamilyStatusSummaryPendingClause"
        status: pass
    human_judgment: false

# Metrics
duration: 20min
completed: 2026-08-22
status: complete
---

# Phase 9 Plan 1: Report pending in migrate status Summary

**`engram migrate status` now reports `pending` in both the `--output json` document (last key) and the `--output text` headline (unconditional clause), sourced from `store.MigrateStatusResult.Pending()` with zero re-derivation — closing audit item W2.**

## Performance

- **Duration:** 20 min
- **Started:** 2026-08-22T18:36:12Z
- **Completed:** 2026-08-22T18:55:08Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Added `Pending uint64` (`json:"pending"`) as the final field of `migrateStatusReportDoc`, populated by a single `res.Pending()` call in `statusReportDoc` — no second loop or accumulator anywhere in `cmd/engram`.
- Extended `TestMigrateFamilyStatusReportDocKeyOrder`'s `want` slice to end with `"pending"`, keeping the exact-length-and-position key-order gate intact (D-06).
- Added `TestMigrateFamilyStatusReportDocPendingNeverRederived`: a `cur`-relative discriminating fixture (Absent 88, buckets at `cur-1`/`cur` counts 9/40, future at `cur+1` count 5, Total 142) proving `Pending()` (97) is pairwise-distinct from three plausible naive rederivations (137, 142, 102), that the converter is pure/idempotent, that a zero value marshals `"pending":0` (never null/omitted), and that the field renders last in both the text and json lanes.
- Added an unconditional pending clause to `statusSummary`'s headline, positioned between the per-bucket enumeration and the conditional future clause, reading the same `res.Pending()` value.
- Added `TestMigrateFamilyStatusSummaryPendingClause`, asserting on values and position only (never wording): the three ordering tokens (`40`, `97`, `5`) each occur exactly once and in order, and the clause is present even for a zero-valued result.
- Performed both plan-mandated constructed-defect observations: swapping `res.Pending()` for a naive sum in `statusReportDoc` drove the new gate RED with a verbatim assertion message; wrapping the new clause in `if res.Pending() > 0 { ... }` drove `emitted_unconditionally_at_zero` RED with a verbatim message. Both were reverted and reconfirmed green before committing.

## Task Commits

Each task was committed atomically (plus two follow-up lint fixes, split back to their owning commit's scope):

1. **Task 1: `pending` lands end-to-end — store method through the converter into both render lanes** - `cebc82de` (feat)
2. **Task 2: unconditional pending clause in the `migrate status` headline** - `d2419cc4` (feat)
3. **Lint fix for Task 1's test (staticcheck QF1011 — redundant type)** - `9dfb07bc` (fix)
4. **Lint fix for Task 2's test (staticcheck QF1001 — De Morgan's law)** - `77e0ea5d` (fix)

**Plan metadata:** committed alongside this SUMMARY.

## Files Created/Modified

- `cmd/engram/migrate_family.go` - `migrateStatusReportDoc.Pending` field, `statusReportDoc`'s `res.Pending()` call, `statusSummary`'s unconditional pending clause, extended doc comments
- `cmd/engram/migrate_family_test.go` - extended key-order `want` slice, `TestMigrateFamilyStatusReportDocPendingNeverRederived`, `TestMigrateFamilyStatusSummaryPendingClause`

## Verify-Command Evidence

Per `<output>`'s requirement, the exact final echo line from each task's `<verify>` command, run clean after the plan's lint fixes:

- **Task 1:** `exit=0 new=6 pre=6 fails=0`
- **Task 2:** `exit=0 new=4 prior=6 order=1 fails=0`

## Constructed-Defect Evidence (verbatim)

**Task 1** — temporarily replaced `res.Pending()` in `statusReportDoc` with the naive absent-plus-every-bucket sum, ran the task's verify target, observed:

```
migrate_family_test.go:469: doc.Pending = 137, want res.Pending() = 97
```

then hand-reverted only that one line (confirmed via `git diff` that `Pending: res.Pending()` was restored) and reconfirmed green.

**Task 2** — temporarily wrapped the new `Fprintf` in `if res.Pending() > 0 { ... }`, ran `TestMigrateFamilyStatusSummaryPendingClause`, observed:

```
migrate_family_test.go:625: zero-valued statusSummary missing pending clause: migrate status: 0 record(s) total, 0 absent (never migrated)
```

then removed the wrapper (confirmed via `diff` against the pre-defect file that the restore was byte-identical) and reconfirmed green.

## Headline Clause Wording (discretionary, D-04)

`"; %d pending"`, e.g. `migrate status: 142 record(s) total, 88 absent (never migrated), 9 at v0, 40 at v1; 97 pending; 5 record(s) at a version newer than this binary's current target (v1)`. Satisfies the two hard constraints (contains the lowercase token `pending`; introduces no decimal digit besides the pending count itself) with minimal added prose.

## Decisions Made

- All four `must_haves.truths` for this plan (adjacency, empty, ordering, idempotency) are directly encoded as subtests in `TestMigrateFamilyStatusReportDocPendingNeverRederived`; the concurrency truth is a documented backstop (no new concurrency surface — pure function over an already-fetched result) requiring no new test.
- No `"migrate status"` entry was added to `TestOperatorOutputEmpty` (D-08, unchanged from plan) — confirmed by re-checking `operatorViewFixtures()`'s existing `"migrate status"` entries build through `statusReportDoc` and inherit the new field automatically, and by confirming no test in `cmd/engram` compares rendered migrate-status output against a hand-written expected block or fixed field count.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Two staticcheck lint violations in the new test code**
- **Found during:** running the plan's own `<verification>` block (`task lint`) after both tasks were committed
- **Issue:** `staticcheck` QF1011 flagged a redundant explicit `uint64` type on `absentPlusEveryBucket` (inferrable from `res.Absent`); QF1001 flagged a De Morgan's-law simplification opportunity on the ordering assertion's negated conjunction
- **Fix:** dropped the redundant type annotation; rewrote `!(a && b)` as `a-negated || b-negated`
- **Files modified:** `cmd/engram/migrate_family_test.go`
- **Verification:** `task lint` exits 0 with `0 issues`; both owning tests re-run green
- **Committed in:** `9dfb07bc` (Task 1's test), `77e0ea5d` (Task 2's test) — split back into two commits so each lint fix stays attached to the task whose test it corrects, rather than landing as one commit spanning both tasks

**2. [Documented gate note, no code deviation] One acceptance-criteria literal-string check does not match gofmt's column alignment**
- **Found during:** Task 1's acceptance criteria verification
- **Issue:** the criterion `rg -c -F 'Pending uint64' cmd/engram/migrate_family.go` returns 0, not 1, because gofmt right-aligns struct field types to the longest field name in the block (`CurrentVersion`, 14 chars), so the actual source reads `Pending        uint64` (8 spaces) rather than a single space. The literal fixed-string pattern as written cannot match gofmt-aligned Go source for any field name shorter than the block's longest name.
- **Resolution:** verified the same fact (field named `Pending`, type `uint64`, positioned as the last field immediately before the struct's closing brace) with a whitespace-tolerant equivalent: `rg -cP 'Pending\s+uint64' cmd/engram/migrate_family.go` returns 1, and `sed -n '309,318p'` confirms the field is the final struct member. No code change was needed or made — this is a mechanical blind spot in the literal verification command, not a defect in the implementation. Not fixed in the plan itself since acceptance criteria are read-only checks, not source files.

---

**Total deviations:** 1 auto-fixed (2 lint violations, same rule), 1 documented gate note (no code impact).
**Impact on plan:** Both lint fixes are cosmetic staticcheck simplifications with no behavior change; both were verified not to affect any test outcome. The gate note affects only how the acceptance criterion was manually verified — the underlying implementation fact it checks is true and was confirmed by an equivalent command.

## Issues Encountered

- `task test:go` intermittently failed on an UNRELATED pre-existing test, `TestRedEvidencePatchesAreLive/.../03-04-red-2-trust-error-signal.patch` (`internal/store`), on the first full-suite run after committing both tasks. This test applies a Phase-3-authored regression patch to `internal/store` and shells out to `go test` as a subprocess; it has no dependency on `cmd/engram/migrate_family.go` or `_test.go`. Confirmed pre-existing and flaky, not caused by this plan: (1) `git worktree add --detach` at `HEAD~2` (before either of this plan's task commits) ran the same test and it also intermittently passed/failed across repeated runs; (2) re-running `task test:go` at this plan's final HEAD came back fully green (`ok github.com/seanb4t/engram/internal/store 102.097s`). Per the deviation-rules scope boundary ("pre-existing warnings, linting errors, or failures in unrelated files are out of scope"), this was not investigated further or fixed — it is orthogonal to W2/pending closure. Not logged to `deferred-items.md` as a new item since it is an existing, already-uncommitted-to test-infra flake, not something this plan introduced.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `pending` is now live end-to-end (json key + text headline) via the single `store.MigrateStatusResult.Pending()` definition, closing audit item W2.
- Ready for `09-02-fix-migrate-guide-pending-row`, which corrects `guides/migrate.md`'s documentation of this same value (audit item W3) and can now describe an accurate, shipped `pending` surface rather than an aspirational one.

## Self-Check: PASSED

- FOUND: `cmd/engram/migrate_family.go`
- FOUND: `cmd/engram/migrate_family_test.go`
- FOUND: `.planning/phases/09-report-pending-in-migrate-status/09-01-SUMMARY.md`
- FOUND commits: `cebc82de`, `d2419cc4`, `9dfb07bc`, `77e0ea5d`
- Re-ran Task 1 verify: `exit=0 new=6 pre=6 fails=0`
- Re-ran Task 2 verify: `exit=0 new=4 prior=6 order=1 fails=0`
- Re-ran plan-level `go test ./cmd/engram/...`: `exit=0 fails=0`
- Re-ran `task lint`: `exit=0`
- Re-ran `task license:check`: `exit=0`

---
*Phase: 09-report-pending-in-migrate-status*
*Completed: 2026-08-22*
