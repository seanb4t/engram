---
phase: 22-cedar-authz-foundation-store-enforcement
reviewed: 2026-07-17T23:36:00Z
depth: standard
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
  warning: 2
  info: 1
  total: 3
status: issues_found
---

# Phase 22: Code Review Report

**Reviewed:** 2026-07-17T23:36:00Z
**Depth:** standard
**Files Reviewed:** 13
**Status:** issues_found

## Summary

Reviewed the Cedar PDP foundation (`internal/authz`: PDP wrapper, entity converters, embedded
policy loader, 4 `.cedar` policies, schema doc) and its wiring into `internal/store`'s bulk
recall filter builders (`ownerOrSharedCondition`/`ownerOnlyCondition` via `DecideBucket`) and
id-addressed gates (`GetReadable`/`getWritable`/`OwnedOrAbsent` via `DecideRecord`).

Traced the four invariants called out in the phase context by hand against the actual Cedar
policy text (not just the Go wrapper):

- **Fail-closed on nil/unknown Subject** — confirmed. `principalParams`'s default arm returns
  `ok=false`; every bulk builder and id-gate checks this BEFORE calling the PDP and short-circuits
  to `matchNothing()`/`ErrNotFound` without a `cedar.Authorize` call. Directly exercised by
  `TestNilSubjectFailsClosed`.
- **authz condition stays the outer `Must`** — confirmed across `ownerScopeFilter`, `listFilter`,
  `SearchDiscovery`, `ListScheduled` (via `ownerOnlyCondition`): the own/shared `Should` clause is
  always one atomic condition inside the outer `Must` array, never merged into a caller-supplied
  `Should`/`MustNot`.
- **`Decision.diag` never reaches a caller-facing error** — confirmed by grep (`.diag` is written
  in `DecideRecord`/never read anywhere in `internal/authz` or `internal/store`) and by
  `TestGetReadableDenyMapsToNotFound`, which asserts the error string is byte-identical to the
  plain missing-id form.
- **`defense_empty_owner` is correctly scoped (not a blanket forbid)** — hand-verified the Cedar
  `forbid...when...unless` semantics: the forbid fires iff `resource.owner != ""` AND
  `principal.owner == ""`, so it never collaterally denies the legitimate anonymous
  (`owner==""`) bucket. Backed by `TestPolicyCorpus_EmptyOwnerDenyAll` and
  `TestPolicyCorpus_AnonOwnBucketReachable`, and by the `DecideBucket` table test's
  `own-bucket-anonymous-*` cases.
- **`shared_read` grants read only (DEC-kyz)** — confirmed both in the policy text (`action ==
  Action::"read"` scope) and by `TestPolicyCorpus_SharedReadOnly`, which asserts Deny for
  write/delete/share/schedule against the real embedded policy bytes.
- **Behavior preservation** — diffed the pre-Cedar hand-rolled type switches (git history) against
  the new `decideRecord`/`decideBucket` calls for `GetReadable`/`getWritable`; the permit/deny
  outcomes are identical for every case the old switch covered, including the pre-isolation
  (missing-owner-key) record ambiguity in the id-addressed path, which was already present before
  Cedar and is unchanged by this phase (not a regression).

Ran `go test -race -cover ./internal/authz/... ./internal/store/...` and `golangci-lint run` —
all green, no race, 87.1%/90.4% coverage. No BLOCKER-level defects found. Two WARNING-level gaps
and one INFO item below.

## Warnings

### WR-01: `DeleteAll` bypasses the PDP entirely — duplicate, unaudited ownership logic

**File:** `internal/store/store.go:1734-1770`
**Issue:** Every other write/read gate this phase touches (`GetReadable`, `getWritable`,
`OwnedOrAbsent`, and the bulk read-filter builders) now routes its authorization decision through
`s.decideRecord`/`s.decideBucket` → the Cedar corpus. `DeleteAll` still hand-rolls the identical
"anonymous → owner==''; authenticated → owner==sub; else → deny" logic via a raw `switch
sj := subj.(type)` (lines 1753-1760) and never calls into `internal/authz`. Functionally it is
equivalent to `own_records.cedar` today (a delete of the caller's own scope-restricted bucket),
so there is no live authorization bug — but it is now the *only* mutating/reading store entry
point not covered by the policy-corpus regression suite
(`internal/authz/policy_corpus_test.go`). A future policy change (e.g. a tenant-scoped forbid, or
narrowing `own_records` for a new action) will silently NOT apply to `DeleteAll`, and nothing in
the test suite would catch the drift — it would need its own bespoke assertion, defeating the
"one policy corpus, every enforcement point" goal this phase establishes.
**Fix:** Either route `DeleteAll` through `s.decideRecord(owner, kind, authz.ActionDelete, owner,
"", "", "")` (mirroring `DecideBucket`'s own-bucket probe shape) before building the delete
filter, or explicitly document in `store.go` why `DeleteAll` is intentionally exempt from the PDP
(e.g. "bulk self-delete is definitionally always permitted, no Cedar consultation needed") so a
future reader doesn't mistake the omission for an oversight.

### WR-02: `getWritable`/`OwnedOrAbsent` Deny→ErrNotFound mapping untested for an existing, denied record

**File:** `internal/store/store_test.go:3415-3482`
**Issue:** `TestGetReadableDenyMapsToNotFound` proves the Cedar-Deny→`ErrNotFound` mapping (DEC-xa6)
for `GetReadable` against a record that **exists** and is owned by the caller. The companion test,
`TestIdAddressedAbsentShortCircuit`, only exercises `getWritable`/`OwnedOrAbsent` against an
**absent** id (where `decideRecord` is never even reached) — it does not prove that an all-deny
`decideRecordHook` maps to the same uniform `ErrNotFound` for `getWritable` or `OwnedOrAbsent` on
a record that actually exists. The two write-path gates share the same one-line
`if !s.decideRecord(...).Allow { return ..., ErrNotFound }` shape as `GetReadable`, so the risk of
a live bug is low, but it is currently unverified by any test — a future edit to either function
(e.g. adding a debug log with the Decision before the ErrNotFound wrap) could leak Diagnostic
content on the write path without any test catching it, even though the read path is covered.
**Fix:** Extend `TestGetReadableDenyMapsToNotFound` (or add a sibling test) to assert the same
plain-`ErrNotFound`-with-no-Diagnostic-content property for `getWritable` and `OwnedOrAbsent`
against an existing, owned record under the all-deny `decideRecordHook`.

## Info

### IN-01: `Decision.diag` is populated on every call but has zero readers

**File:** `internal/authz/authz.go:48-51`, `internal/authz/authz.go:69-70`
**Issue:** Every `DecideRecord` call captures `cedar.Authorize`'s `Diagnostic` into the unexported
`Decision.diag` field. A repo-wide grep for `.diag` turns up zero read sites — the field is
currently pure write-only overhead on the hot bulk-recall path (two `DecideBucket` calls per
`Search`/`List`/etc.), with no debug-logging or OTel-span consumer wired up yet despite the
doc comment's stated intent ("exists solely for future debug-level logging / OTel span
attachment").
**Fix:** No action required now — this is explicitly deferred per the doc comment — but flag it
for the phase that actually wires up operator diagnostics, since carrying an unused
`cedar.Diagnostic` per decision is otherwise dead weight on a path this phase itself calls out as
performance-sensitive (O(buckets) per request).

---

_Reviewed: 2026-07-17T23:36:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
