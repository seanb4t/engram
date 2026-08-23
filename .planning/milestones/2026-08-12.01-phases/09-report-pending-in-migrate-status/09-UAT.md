---
status: complete
phase: 09-report-pending-in-migrate-status
source: 09-01-SUMMARY.md, 09-02-SUMMARY.md
started: 2026-08-22T19:59:20Z
updated: 2026-08-22T20:17:06Z
---

## Current Test

[testing complete]

## Tests

### 1. migrate status reports pending in --output json
expected: `engram migrate status --output json` emits a `pending` key as the last field of the report doc, sourced from `store.MigrateStatusResult.Pending()` (never re-derived).
result: pass
source: automated
coverage_id: D1
covering_tests: cmd/engram/migrate_family_test.go#TestMigrateFamilyStatusReportDocPendingNeverRederived, cmd/engram/migrate_family_test.go#TestMigrateFamilyStatusReportDocKeyOrder

### 2. migrate status text headline carries an unconditional pending clause
expected: The text headline places a pending clause between the bucket enumeration and the future clause, from the same `res.Pending()` call — present even when pending is zero.
result: pass
source: automated
coverage_id: D2
covering_tests: cmd/engram/migrate_family_test.go#TestMigrateFamilyStatusSummaryPendingClause

### 3. Zero-occurrence docs gate over the migrate guide pending row
expected: `cmd/engram/migrate_docs_test.go` asserts zero occurrences of both stale-claim anchors in guides/migrate.md, plus row survival and token checks, backed by a 7-case positive control that fires on injected violations.
result: pass
source: automated
coverage_id: D1
covering_tests: cmd/engram/migrate_docs_test.go#TestMigrateGuidePendingRowIsAccurate, cmd/engram/migrate_docs_test.go#TestMigrateGuidePendingRowGateFiresOnInjectedViolation

### 4. migrate guide pending row states the real arithmetic
expected: guides/migrate.md's `pending` row states absent-plus-every-bucket-strictly-below-current_version (future excluded), names all three reporting surfaces, and names `MigrateStatusResult.Pending()` as the single shared definition.
result: pass
source: automated
coverage_id: D2
covering_tests: cmd/engram/migrate_docs_test.go#TestMigrateGuidePendingRowIsAccurate, rg zero-occurrence check on both stale anchors

## Summary

total: 4
passed: 4
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none yet]
