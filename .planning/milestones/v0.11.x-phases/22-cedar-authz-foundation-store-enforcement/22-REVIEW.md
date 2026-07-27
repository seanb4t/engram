---
phase: 22-cedar-authz-foundation-store-enforcement
reviewed: 2026-07-18T00:45:00Z
depth: deep
files_reviewed: 13
files_reviewed_list:
  - docs/adr/engram-cdr1-cedar-pdp-decides-predicate-store-enforces-qdrant-filter.md
  - internal/authz/authz.go
  - internal/authz/authz_test.go
  - internal/authz/entities.go
  - internal/authz/policies.go
  - internal/authz/policies/defense_empty_owner.cedar
  - internal/authz/policies/own_records.cedar
  - internal/authz/policies/shared_read.cedar
  - internal/authz/policies/tenant_isolate.cedar
  - internal/authz/policy_corpus_test.go
  - internal/authz/schema.json
  - internal/store/store.go
  - internal/store/store_test.go
findings:
  critical: 0
  warning: 0
  info: 0
  total: 0
status: clean
---

# Phase 22: Code Review Report

**Reviewed:** 2026-07-18T00:45:00Z
**Depth:** deep
**Files Reviewed:** 13
**Status:** clean

## Summary

This is iteration 3 (final) of the fix→re-review loop. Since iteration 2's review, exactly
two commits landed, both of which resolve the two remaining items from that pass:

- **WR-04** (`DeleteAll`'s denied-bucket branch had no regression test) — `d3f6c740` adds
  `TestDeleteAllDeniedBucketDeletesNothing`, which injects an all-deny `decideBucketHook`,
  calls `DeleteAll` for an owned, existing record, and asserts both that `DeleteAll` returns
  `nil` (not an error) and that the record survives (`s.Get` still finds it). This is the exact
  test iteration 2's review proposed, byte-for-byte. Verified: (a) it compiles and passes under
  `-race` in isolation and as part of the full `internal/store` suite (`go test
  ./internal/store/... -race`, 7.18s, all green); (b) it correctly targets the new branch added
  by the WR-01 fix (`internal/store/store.go`'s `DeleteAll`, the `if
  !s.decideBucket(...).Allow { return nil }` guard before the delete filter is built); (c) it
  follows the same injected-hook idiom as its sibling tests
  (`TestBulkFilterZeroBucketFailsClosed`, `TestGetWritableAndOwnedOrAbsentDenyMapsToNotFound`)
  — `t.Cleanup` resets `decideBucketHook` to nil so the probe cannot leak into later tests, and
  `defer s.DeleteAllRaw(...)` cleans up the real Qdrant collection state regardless of outcome.
  With this test, every PDP-denial branch introduced by this phase now has a matching
  regression test — no coverage gap remains.
- **IN-03** (ADR's `DecideBucket` caller list omitted `DeleteAll`) — `4f3c108e` appends a
  clause to the `DecideBucket` bullet in `docs/adr/engram-cdr1-...md` noting that `DeleteAll` —
  "a bulk mutation, not a recall path" — asks the same `BucketOwn` question
  (`ActionDelete`) before building its delete filter. Verified: the wording is accurate (matches
  `DeleteAll`'s actual call `s.decideBucket(owner, kind, authz.ActionDelete,
  authz.BucketOwn)`), correctly placed (end of the existing `DecideBucket` bullet, immediately
  after the bulk-recall filter-builder description it extends), and does not contradict or
  duplicate any other ADR section. Read the full ADR end-to-end; no other stale caller lists or
  cross-references were introduced or left dangling by this edit.

IN-01 (`Decision.diag` unread) remains deliberately-skipped/no-action-this-phase per standing
instruction and is not re-flagged.

Full verification re-run at this pass, all clean:

- `go build ./...` — clean.
- `go vet ./internal/authz/... ./internal/store/...` — clean.
- `golangci-lint run ./internal/authz/... ./internal/store/...` — 0 issues.
- `task license:check` — 0 invalid headers.
- `go test ./internal/authz/... -race -v` — 11/11 pass.
- `go test ./internal/store/... -race` — full suite green (7.18s), including
  `TestDeleteAllDeniedBucketDeletesNothing`, `TestWithAuthzOption`,
  `TestSearchAuthzCallCount`, `TestBulkFilterZeroBucketFailsClosed`, and
  `TestGetWritableAndOwnedOrAbsentDenyMapsToNotFound` run individually and as part of the
  whole-package run — no interaction/ordering regressions from the new test's hook
  install/cleanup.

Cross-file invariants re-confirmed one final time:

- Sole PDP consumer is `internal/store/store.go` (`rg -n "internal/authz"` outside
  `internal/authz` itself returns exactly one hit) — no handler or CLI command touches the PDP
  directly.
- No per-record Cedar evaluation on any bulk-recall path — bucket decisions stay O(buckets),
  never O(records).
- The authz condition is always inside the outer `Must` of every composed filter
  (`Search`/`List`/`SearchDiscovery`/`ListScheduled`/`ListScopes`), and now also gates
  `DeleteAll`'s delete filter before it is built — no path can reach another owner's records.
  `Decision.diag` never reaches a caller-facing error string.

No new findings were surfaced by this pass. Both landed commits are correct, complete, minimal,
and exactly scoped to the two items they targeted — no unrelated changes, no regressions
introduced. All reviewed files meet quality standards.

---

_Reviewed: 2026-07-18T00:45:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
