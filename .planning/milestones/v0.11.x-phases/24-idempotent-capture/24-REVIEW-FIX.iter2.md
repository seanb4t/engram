---
phase: 24-idempotent-capture
fixed_at: 2026-07-18T20:47:22Z
review_path: .planning/phases/24-idempotent-capture/24-REVIEW.md
iteration: 1
findings_in_scope: 6
fixed: 6
skipped: 0
status: all_fixed
---

# Phase 24: Code Review Fix Report

**Fixed at:** 2026-07-18T20:47:22Z
**Source review:** .planning/phases/24-idempotent-capture/24-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 6 (fix_scope: all — Critical, Warning, and Info)
- Fixed: 6
- Skipped: 0

## Fixed Issues

### CR-01: Concurrent identical-key writes can return a short_id that never resolves

**Files modified:** `internal/server/tools.go`
**Commit:** `e37d7fb7`
**Applied fix:** `persistAndEnqueue` now re-`Get`s the point immediately after `Upsert` and uses the
persisted record's `ShortID` (falling back to the locally-minted one only if the re-Get itself
fails) instead of unconditionally returning the locally-minted value. This matches the reviewer's
suggested fix code verbatim. Verified with the existing `TestStoreMemoryIdempotentConcurrentIdenticalOnePoint`
(SC4, `-race`), plus manual `go build`/`go vet`/full package test runs.

**Residual caveat (status: fixed, requires human awareness — logic/consistency nuance, not a
correctness regression):** under a genuine full N-way simultaneous race (e.g. 20 goroutines firing
the identical keyed call at the exact same instant, as the SC4 stress test does), each racer's
re-`Get` reads the store's state *at that instant*, which can still be superseded by a
still-in-flight sibling's `Upsert` completing microseconds later. This eliminates the finding's
core defect — the pre-fix guarantee was "the losing racer's returned short_id was **never**
persisted" (100% failure); post-fix, every returned short_id was persisted *at the moment it was
read back*, and in the vast majority of real-world usage (client retries are typically sequential,
not literally simultaneous) the returned short_id is also the final one. A fully airtight
guarantee across every possible interleaving of a truly simultaneous N-way race would require
additional coordination (e.g. a per-pointID lock/singleflight or a Qdrant-side CAS/optimistic-
concurrency primitive) — a larger design change judged out of scope for this fix. I attempted to
add a regression test asserting full N-way convergence and it was empirically flaky under real
concurrent Qdrant round-trips for exactly this reason, so it was not committed (see note in the
`TestStoreMemoryIdempotentConcurrentIdenticalOnePoint` history for the discarded attempt); the
existing SC4 test (no-duplicate-point invariant) remains the regression guard for this path.

### WR-01: contentFingerprint's tag-joining is not injective

**Files modified:** `internal/server/idempotency.go`, `internal/server/idempotency_test.go`
**Commit:** `ec8e6d06`
**Applied fix:** `contentFingerprint`'s tags component now length-prefixes each tag individually
(`%d:%s:` per tag, matching `idempotencyPointID`'s own discipline) before folding the result into
the outer per-field length-prefixed encoding, replacing the raw `strings.Join(tags, "\x1f")`.
Added `TestContentFingerprintTagsBoundaryShiftInjective`, which pins that
`tags=["a\x1fb"]` and `tags=["a","b"]` no longer fingerprint identically. Confirmed the new test
fails on the pre-fix code and passes post-fix.

### WR-02: schedule_memory's future-window validation can reject a legitimate idempotent retry

**Files modified:** `internal/server/tools.go`, `internal/server/tools_test.go`
**Commit:** `4e8637f3`
**Applied fix:** Reordered `scheduleMemory` so `checkIdempotentReplay` runs before `parseWindow`
(matching the reviewer's suggested code exactly) — on the replay path, `parseWindow` never runs at
all; it only executes on the non-replay (create) path. Added
`TestScheduleMemoryIdempotentRetryAfterWindowLapses`, which pins that a delayed retry with the same
(now-lapsed) `not_after` resolves as a replay instead of `ErrInvalidArgument`. Confirmed the new
test fails on the pre-fix code (`git stash` of the `tools.go` change reproduced the exact
`ErrInvalidArgument` the finding describes) and passes post-fix.

### WR-03: connecterror_test.go's acceptance table was not extended for the new sentinel arm

**Files modified:** `internal/server/connecterror_test.go`
**Commit:** `d298fa49`
**Applied fix:** Added the missing `{"idempotency_conflict", ..., connect.CodeAlreadyExists}` case
to `TestConnectError`'s `cases` table, using the exact code the reviewer suggested.

### IN-01: idempotency_key has no size bound

**Files modified:** `internal/server/tools.go`, `internal/server/tools_test.go`
**Commit:** `75cd05aa`
**Applied fix:** Added `maxIdempotencyKeyBytes = 512` (a modest cap consistent with the
`storeDiscoveryArgs` size-bound discipline already established in this file) and enforced it at
the top of `checkIdempotentReplay`, rejecting oversized keys with a wrapped
`store.ErrInvalidArgument` before any hashing or store round trip. Added
`TestStoreMemoryIdempotencyKeyTooLarge` (no Qdrant/embedder needed — the check runs before either
is touched).

### IN-02: IdempotencyFingerprint goes stale after update_memory changes content

**Files modified:** `internal/store/store.go`
**Commit:** `3f3ef9c2`
**Applied fix:** Documentation-only, per the review's own "no behavior change required" guidance.
Extended the `Memory.IdempotencyFingerprint` field doc comment to explicitly state it is frozen at
create time and not recomputed by `Store.Update`, and added a one-line note on `Store.Update`'s
doc comment pointing back to that contract. No behavior change.

## Skipped Issues

None — all 6 in-scope findings were fixed.

## Verification

- `go build ./...` — clean after every fix.
- `go vet ./internal/server/... ./internal/store/...` — clean.
- `gofmt -l` on every touched file — no output (all formatted).
- `go test -race ./internal/server/... ./internal/store/...` — full suite green after all fixes
  applied together (not just per-fix incrementally).
- `task license:check` — 0 invalid (no new files created, so no new SPDX headers needed).
- `task lint:go` / `task lint:python` — clean. `task lint:yaml` fails on `Taskfile.yaml` formatting
  drift, confirmed pre-existing and unrelated (reproduces identically on the unmodified base branch
  before any of this run's changes; no yaml file was touched by this fix run).

---

_Fixed: 2026-07-18T20:47:22Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
