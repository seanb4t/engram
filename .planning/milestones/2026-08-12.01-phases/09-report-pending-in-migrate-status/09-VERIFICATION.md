---
phase: 09-report-pending-in-migrate-status
verified: 2026-08-22T20:10:00Z
status: passed
score: 9/9 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 9: Report pending in migrate status Verification Report

**Phase Goal:** Close audit items W2 and W3 — `engram migrate status` reports `pending` (the
value the milestone declared canonical) in both `--output text` and `--output json`, sourced
from the single existing `store.MigrateStatusResult.Pending()` definition, and
`docs-site/src/content/docs/guides/migrate.md`'s `pending` row is corrected to match.

**Verified:** 2026-08-22T20:10:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `engram migrate status` reports `pending` in both `--output json` (key) and `--output text` (headline clause) | ✓ VERIFIED | `cmd/engram/migrate_family.go:313` (`Pending uint64 \`json:"pending"\`` — 7th/last field); `statusReportDoc` populates it via `res.Pending()` (line ~336); `statusSummary` emits `"; %d pending"` unconditionally (line ~354). `viewFields`/`renderOperatorView` render struct field order into both lanes, so one field covers both. |
| 2 | `pending` has exactly one arithmetic definition repo-wide; `cmd/engram` adds a call site, never a re-derivation | ✓ VERIFIED | `internal/store/migrate_status.go:76` is the sole `func (r MigrateStatusResult) Pending()`. `rg -v '^\s*//' cmd/engram/migrate_family.go \| rg -c -F 'res.Pending()'` = 2 (statusReportDoc + statusSummary), 0 loops/accumulators found in either function body. |
| 3 | A bucket at `current_version` contributes nothing to `pending`; only `absent` + strictly-below buckets count; `future` excluded | ✓ VERIFIED | `TestMigrateFamilyStatusReportDocPendingNeverRederived/equals_store_pending` and `/fixture_discriminates_every_naive_rederivation` pass (fixture: Absent 88, buckets at cur-1/cur, future at cur+1 → Pending=97 vs naive 137/142/102, all pairwise distinct). Reviewer independently hand-verified the arithmetic in 09-REVIEW.md. |
| 4 | Zero-valued result still marshals `"pending":0`, never null/omitted | ✓ VERIFIED | `zero_value_marshals_pending_zero` subtest passes; `TestMigrateFamilyStatusReportDocNeverMarshalsNull` unchanged and still passes. |
| 5 | `pending` is the last key of the marshalled doc / last row of the text table; pre-existing keys keep their order | ✓ VERIFIED | `TestMigrateFamilyStatusReportDocKeyOrder`'s `want` slice ends `..., "current_version", "pending"`; `orderedKeyDiff` (`cmd/engram/operator_view_test.go:73-91`) is unmodified — still exact length+position, not relaxed to a subset check (D-06 honored). |
| 6 | Text headline emits the pending clause unconditionally, positioned after buckets and before the future clause | ✓ VERIFIED | Source line order confirmed (`for _, bucket` loop → `fmt.Fprintf(&b, "; %d pending", res.Pending())` → `if len(res.Future) > 0`). `TestMigrateFamilyStatusSummaryPendingClause` passes all 3 subtests, including `emitted_unconditionally_at_zero`. Reviewer independently drove this RED via constructed defect (reordering) and confirmed the ordering assertion is load-bearing. |
| 7 | `guides/migrate.md`'s `pending` row no longer claims Connect-lane-only or a CLI-side derivation | ✓ VERIFIED | `rg -o -F 'the equivalent number from' docs-site/` = 0; `rg -o -F 'Connect lane only' docs-site/` = 0 (repo-wide, not just the one file). |
| 8 | The replacement row is itself true: states the strictly-below boundary, names `current_version` and `future`, names all three reporting surfaces and the single `Pending()` definition | ✓ VERIFIED | `docs-site/.../migrate.md:279` row text read directly: states "`absent` plus every bucket **strictly below** `current_version`... every `future` bucket are excluded... Reported by `engram migrate status` (both `text` and `json`), by `engram migration-status`, and by the Connect `MigrateStatusResponse` (field 7); all three read the same server-side `MigrateStatusResult.Pending()`". Cross-checked against `internal/server/connectapi.go:212` (`Pending: status.Pending()`) and `cmd/engram/client_migration_status.go` (protojson full-message render) — both true of the shipped code. |
| 9 | Docs gate asserts zero remaining occurrences (never a conversion count) and ships with a self-discriminating positive control | ✓ VERIFIED | `cmd/engram/migrate_docs_test.go`: `migrateGuidePendingRowViolations` gates on `strings.Count(doc, anchor) != 0` (zero-occurrence, not count-of-N); `TestMigrateGuidePendingRowGateFiresOnInjectedViolation` runs 7 cases including a `clean` (no-violation) case. Both tests pass: `go test ./cmd/engram/... -run 'TestMigrateGuidePendingRow' -v -count=1` → 9 PASS lines, 0 FAIL (independently re-run). |

**Score:** 9/9 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/engram/migrate_family.go` | `migrateStatusReportDoc.Pending` field + `res.Pending()` in both `statusReportDoc` and `statusSummary` | ✓ VERIFIED | Field present, last position; both call sites confirmed; doc comments extended per D-01/D-02/D-03. |
| `cmd/engram/migrate_family_test.go` | Discriminating-fixture gate, summary-clause gate, extended key-order `want` | ✓ VERIFIED | `TestMigrateFamilyStatusReportDocPendingNeverRederived` (5 subtests) and `TestMigrateFamilyStatusSummaryPendingClause` (3 subtests) present and passing; key-order `want` extended. |
| `cmd/engram/migrate_docs_test.go` | Zero-occurrence docs gate + shared violation helper + positive control | ✓ VERIFIED | New file, 151 lines (exceeds `min_lines: 60`), SPDX header present, `migrateGuidePendingRowViolations` has exactly 2 callers (the two tests), no parallel reimplementation. |
| `docs-site/src/content/docs/guides/migrate.md` | Corrected `pending` row | ✓ VERIFIED | Row rewritten; `git diff --numstat` on this file across the phase's commits is a 1-line swap; encoding paragraph below the row byte-unchanged. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `cmd/engram/migrate_family.go` (`statusReportDoc`/`statusSummary`) | `internal/store/migrate_status.go` (`Pending()`) | `res.Pending()` call | ✓ WIRED | 2 call sites confirmed, 0 re-derivations. |
| `cmd/engram/migrate_family.go` (`Pending` field) | `cmd/engram/operator_view.go` (`viewFields`) | json-tag-driven document-order rendering | ✓ WIRED | `renders_last_in_both_lanes` subtest exercises both the text and json lanes off one fixture and passes. |
| `cmd/engram/migrate_docs_test.go` | `docs-site/src/content/docs/guides/migrate.md` | package-relative file read | ✓ WIRED | `TestMigrateGuidePendingRowIsAccurate` reads the live file and passes against the corrected row. |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| REQ-migrate-status-histogram | 09-01, 09-02 | Already Complete (Phase 4) — this phase's histogram-adjacent debt closure | ✓ SATISFIED | `pending` now reported alongside the existing histogram fields in both lanes; no re-opening of the original histogram implementation. |
| REQ-docs-record-state | 09-01, 09-02 | Already Complete (Phase 8) — this phase's doc-accuracy debt closure | ✓ SATISFIED | `guides/migrate.md`'s `pending` row now accurately documents record-state reporting across all three surfaces. |

No orphaned requirements: ROADMAP maps only these two IDs to Phase 9, and both appear in both plans' frontmatter.

### Anti-Patterns Found

None. Scanned all four phase-modified files (`cmd/engram/migrate_family.go`, `cmd/engram/migrate_family_test.go`, `cmd/engram/migrate_docs_test.go`, `docs-site/src/content/docs/guides/migrate.md`) for `TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER|not yet implemented|coming soon` — zero matches.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Code-side gates (W2) all pass together | `go test ./cmd/engram/... -run 'TestMigrateFamilyStatusReportDocPendingNeverRederived\|TestMigrateFamilyStatusReportDocKeyOrder\|TestMigrateFamilyStatusReportDocNeverMarshalsNull\|TestOperatorDocsAreHandDeclared\|TestOperatorOutputEmpty\|TestMigrateFamilyStatusSummaryPendingClause' -v -count=1` | exit 0, 36 PASS lines, 0 FAIL | ✓ PASS |
| Docs gate (W3) passes, including positive control | `go test ./cmd/engram/... -run 'TestMigrateGuidePendingRow' -v -count=1` | exit 0, 9 PASS lines (1 real-file + 1 parent + 7 control cases), 0 FAIL | ✓ PASS |
| Stale anchors removed repo-wide | `rg -o -F 'the equivalent number from' docs-site/` / `rg -o -F 'Connect lane only' docs-site/` | 0 / 0 | ✓ PASS |
| Sole `Pending()` definition | `rg -n 'func (r MigrateStatusResult) Pending' internal/` | 1 match, `internal/store/migrate_status.go:76` | ✓ PASS |
| No scope leakage into out-of-scope files | `git log --oneline -- internal/store/migrate_status.go internal/server/connectapi.go internal/server/tools.go cmd/engram/client_common.go cmd/engram/client_migration_status.go` (since phase start) | empty (no commits touching these files) | ✓ PASS |
| Full build/test/lint (orchestrator-measured, independently spot-confirmed above) | `go build ./...`, `go test ./...`, `task lint` | exit 0 each | ✓ PASS |

### Probe Execution

Not applicable — this is a Go CLI/docs debt-closure phase with no `scripts/*/tests/probe-*.sh` and none declared in the plans. `find scripts -path '*/tests/probe-*.sh' -type f` returns nothing.

### Locked Decisions (09-CONTEXT.md) Honored

| Decision | Check | Result |
|----------|-------|--------|
| D-01 `Pending` appended last, after `CurrentVersion` | Struct field order | ✓ Honored — 7th/final field |
| D-02 single `res.Pending()` call, never re-derived | Call-site count + loop/accumulator scan | ✓ Honored — 2 call sites (converter + summary), 0 re-derivations |
| D-03 unconditional clause, positioned after buckets / before future | Source line ordering + test | ✓ Honored |
| D-04 row rewritten (not deleted/softened) | Row text read directly | ✓ Honored — states arithmetic, all 3 surfaces, single definition |
| D-06 `orderedKeyDiff` not relaxed | `operator_view_test.go:73-91` unmodified | ✓ Honored — exact length+position check intact |
| D-07 zero-occurrence gate, inflection-free anchor, positive control | `migrateGuidePendingRowViolations` + control | ✓ Honored |
| D-08 `operatorViewFixtures()` migrate-status entries re-checked; `TestOperatorOutputEmpty` gains no entry | `TestOperatorOutputEmpty` still passes unmodified | ✓ Honored |

### Scope Fence Held

- Backlog Phases 999.2 (W1 E2E), 999.3 (W4 CLAUDE.md claim), 999.4 (proto `schema_version` typing) were not touched — confirmed no commits or diffs to `internal/e2e/`, `CLAUDE.md`, or `proto/`.
- `internal/store/migrate_status.go` (`Pending()` itself), `internal/server/connectapi.go`, `internal/server/tools.go`, `cmd/engram/client_common.go`, `cmd/engram/client_migration_status.go` all show no commits since the phase began — confirmed via `git log` scoped to those paths.

### Human Verification Required

None. All must-haves resolve to VERIFIED via source inspection and independently re-run automated tests.

### Gaps Summary

None. Both audit items W2 and W3 are closed by the same code change plus the corresponding doc
correction, exactly as the phase goal specified. The single `Pending()` definition remains the only
arithmetic site; the CLI report struct and text headline both consume it; the documentation now
accurately describes all three reporting surfaces. Independent re-verification (test re-run,
source reads, git history scoping, anchor greps) confirms the two SUMMARY.md files' claims rather
than merely trusting them.

---

_Verified: 2026-08-22T20:10:00Z_
_Verifier: Claude (gsd-verifier)_
