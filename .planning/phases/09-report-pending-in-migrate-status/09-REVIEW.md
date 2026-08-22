---
phase: 09-report-pending-in-migrate-status
reviewed: 2026-08-22T19:21:47Z
depth: standard
files_reviewed: 4
files_reviewed_list:
  - cmd/engram/migrate_family.go
  - cmd/engram/migrate_family_test.go
  - cmd/engram/migrate_docs_test.go
  - docs-site/src/content/docs/guides/migrate.md
findings:
  critical: 0
  warning: 0
  info: 1
  total: 1
status: clean
---

# Phase 09: Code Review Report

**Reviewed:** 2026-08-22T19:21:47Z
**Depth:** standard
**Files Reviewed:** 4
**Status:** clean

## Summary

This phase adds `Pending uint64` (`json:"pending"`) to `migrateStatusReportDoc`, sourced from
`store.MigrateStatusResult.Pending()`, plus an unconditional headline clause in `statusSummary`,
plus a corrected `pending` row in `guides/migrate.md`. I read all four changed files, cross-checked
against `internal/store/migrate_status.go` (`Pending()`'s real definition and `CurrentVersion`'s
real value), `internal/server/connectapi.go` (the Connect RPC handler), and
`cmd/engram/client_migration_status.go`/`client_common.go` (the other two consumers named in the
doc row), and ran the new tests, `go vet`, `gofmt -l`, and `golangci-lint run ./cmd/engram/...`
against the package — all clean.

I did not stop at "tests pass." Per the adversarial brief I injected two constructed defects
directly into `cmd/engram/migrate_family.go` and re-ran the targeted tests to prove each test
actually goes red against the failure it claims to catch, then reverted (confirmed via `git diff`
showing no residual change):

1. Replaced `Pending: res.Pending()` with an inlined `absent + sum(all buckets)` re-derivation
   (D-02's exact prohibited class). Result: `TestMigrateFamilyStatusReportDocPendingNeverRederived`
   failed at both `equals_store_pending` (`doc.Pending = 137, want res.Pending() = 97`) and
   `renders_last_in_both_lanes` (json missing `"pending":97`). Confirms the test is not vacuous.
2. Reordered `statusSummary` to emit the future clause before the pending clause (D-03's ordering
   requirement). Result: `TestMigrateFamilyStatusSummaryPendingClause/states_pending_value_between_buckets_and_future`
   failed (`got 74, 155, 84` — future token now precedes pending token). Confirms the ordering
   assertion is load-bearing, not decorative.

I also hand-verified the fixture arithmetic in
`TestMigrateFamilyStatusReportDocPendingNeverRederived` against the real `migrate.CurrentVersion`
(`1`, confirmed via `internal/migrate/migrate_test.go`) and the real `Pending()` formula
(`internal/store/migrate_status.go:76-82`): with `Absent=88`, buckets `{v0:9, v1:40}`,
`Future={v2:5}`, `FutureTotal=5`, `Total=142`, the four candidate values are `want=97`,
`absentPlusEveryBucket=137`, `plusFutureTotal=142`, `totalMinusCurrentBucket=102` — all four
pairwise distinct, matching the test's own assertions and the doc comment's claimed numbers
exactly. `TestMigrateFamilyStatusReportDocKeyOrder` confirms `pending` renders last (D-01).
`orderedKeyDiff` (`operator_view_test.go:73`) is unchanged from prior phases — still an exact
length+position check, never relaxed to subset/prefix (D-06).

For the docs gate, `migrateGuidePendingRowViolations`'s two stale-claim anchors
(`"the equivalent number from"`, `"Connect lane only"`) no longer appear anywhere under
`docs-site/src/` (verified repo-wide, not just in the one file the test reads), so this phase's fix
is complete rather than merely locally scoped. The rewritten row's substantive claims all check out
against the actual code: `MigrateStatusResponse.pending` is proto field 7
(`proto/engram/v1/engram.proto:205`); `internal/server/connectapi.go:212` populates it from
`status.Pending()`; `cmd/engram/client_migration_status.go` renders the full proto message (so
`pending` appears in both its `text` and `json` output without any change needed in that file, as
the phase's CONTEXT explicitly anticipated); and `cmd/engram/client_common.go:382` (the advisory
footer) already reads `resp.Msg.GetPending()` off the wire with no re-derivation. The docs test's
positive control (`"clean"` case, `expectViolation: false`) is present and is not itself a
tautology — it exercises the same `migrateGuidePendingRowViolations` function the live-file test
calls, not a parallel reimplementation.

I found no Critical or Warning issues. One Info-level observation on test-design robustness below.

## Info

### IN-01: Docs gate's encoding-paragraph check is document-wide, not row-adjacent

**File:** `cmd/engram/migrate_docs_test.go:80-86`
**Issue:** "Leg 5" of `migrateGuidePendingRowViolations` guards against the protojson/uint64
encoding paragraph being deleted alongside the `pending` row fix, but it checks for the substrings
`"protojson"` and `"uint64"` anywhere in the whole document, not anchored near the `pending` row.
Today this is harmless — `rg` confirms both words occur exactly once in the live guide, in that one
paragraph — but the check would silently stop discriminating if either word were ever introduced
elsewhere in the guide (e.g., in an unrelated future section) while the actual paragraph below the
`pending` row was deleted or reworded away from those terms.
**Fix:** Not urgent given current guide content, and this is prose-gate scope (CLAUDE.md's testing
guidance treats CI/doc gates as "simple, reliable, predictable" rather than exhaustively
adversarial). If tightened, anchor the check to the text immediately following `pendingRowPattern`'s
match rather than the whole document, e.g. by capturing a fixed number of lines after the row match
and searching only within that window.

---

_Reviewed: 2026-08-22T19:21:47Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
