---
phase: 22-cedar-authz-foundation-store-enforcement
plan: 02
subsystem: auth
tags: [cedar, authz, qdrant, store, bulk-recall, policy-decision-point]

# Dependency graph
requires:
  - phase: 22-01
    provides: "internal/authz: PDP type, DecideBucket/DecideRecord, MustDefault(), Action/Bucket enums"
provides:
  - "Store.authz *authz.PDP field defaulted via authz.MustDefault(); WithAuthz(pdp) Option for test injection"
  - "principalParams(subj Subject) (owner, kind, ok) — the single Subject->authz-primitives converter, in internal/store"
  - "ownerOrSharedCondition/ownerOnlyCondition/listFilter converted to *Store methods that derive own/shared bucket clauses from s.authz.DecideBucket while emitting the SAME Qdrant filter shapes as before Cedar"
  - "decideBucketHook test seam (function-var field) for injecting a call-counting or all-deny PDP probe without needing to construct a custom *authz.PDP"
affects: [22-03]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "decideBucketHook function-var field (mirrors mintCandidate/deletePayloadKeys) — a same-package test seam for a sealed concrete dependency (*authz.PDP) that has no exported constructor besides MustDefault()"
    - "Bucket-decision-to-filter-shape translation: DecideBucket(BucketOwn/BucketShared).Allow gates whether each qdrant.NewMatch clause is appended to a Should slice, preserving the pre-Cedar hardcoded switch's exact filter shapes (D-11)"

key-files:
  created: []
  modified:
    - internal/store/store.go
    - internal/store/store_test.go

key-decisions:
  - "decideBucketHook (unexported *Store field, injected directly by same-package tests, no new exported Option) is the call-counting/all-deny test seam Task 3 needed — *authz.PDP is a sealed concrete type (unexported fields, MustDefault() the only constructor) so it cannot be subclassed; this keeps WithAuthz's exported signature exactly as Task 1 specified (func WithAuthz(pdp *authz.PDP) Option) with zero new production API surface."
  - "A1 held again: principalParams hardcodes kind='human' for both authenticated and anonymous arms — Phase 23's converter is responsible for real classification (unchanged from Plan 01)."

patterns-established:
  - "Sealed-dependency test-seam pattern: when a struct field's type is a concrete external type with no test-friendly constructor, add a function-var field (nil-checked, defaulting to the real call) rather than widening the field to an interface — preserves exact field-type acceptance criteria while enabling same-package test injection."

requirements-completed: [REQ-cedar-store-enforcement]

coverage:
  - id: D1
    description: "Store gains an injected authz *authz.PDP field defaulted via authz.MustDefault() in New(); WithAuthz(pdp) Option for test injection; principalParams(subj) converts Subject to (owner, kind, ok) primitives, failing closed for nil/unknown WITHOUT a PDP call. All four existing store.New(...) call sites unchanged."
    requirement: "REQ-cedar-store-enforcement"
    verification:
      - kind: unit
        ref: "go build ./... (all store.New call sites compile unchanged)"
        status: pass
      - kind: unit
        ref: "go vet ./internal/store/..."
        status: pass
    human_judgment: false
  - id: D2
    description: "ownerOrSharedCondition and ownerOnlyCondition converted to *Store methods deriving own/shared bucket clauses from s.authz.DecideBucket while emitting the exact same Qdrant filter shapes as the pre-Cedar hardcoded switch; listFilter converted to a *Store method; ownerScopeFilter/SearchDiscovery/ListScopes/List/ListScheduled all route through the PDP-backed builders."
    requirement: "REQ-cedar-store-enforcement"
    verification:
      - kind: unit
        ref: "internal/store/store_test.go#TestSearchListOwnerIsolation"
        status: pass
      - kind: unit
        ref: "internal/store/store_test.go#TestAnonBucketReadIsolation"
        status: pass
      - kind: unit
        ref: "internal/store/store_test.go#TestAnonBucketDiscoveryReadIsolation"
        status: pass
      - kind: unit
        ref: "internal/store/store_test.go#TestListPrivateFilterCrossActorIsolation"
        status: pass
      - kind: unit
        ref: "internal/store/store_test.go#TestListScheduledOwnerIsolation"
        status: pass
      - kind: unit
        ref: "internal/store/store_test.go#TestSearchDiscoveryOwnerIsolation"
        status: pass
      - kind: unit
        ref: "internal/store/store_test.go#TestGetReadableOwnerGate"
        status: pass
      - kind: unit
        ref: "internal/store/store_test.go#TestOwnedOrAbsent"
        status: pass
      - kind: unit
        ref: "internal/store/store_test.go#TestAnonBucketWriteSemantics"
        status: pass
      - kind: unit
        ref: "internal/store/store_test.go#TestUpdateOwnerGateAndSharedFlag"
        status: pass
    human_judgment: false
  - id: D3
    description: "Bulk recall proven to call DecideBucket O(buckets-per-request) — never per record; own+shared adjacency-safe (no duplicate/conflict); fail-closed to zero results under an all-deny PDP (never an unfiltered query); result stable regardless of bucket-decision evaluation order."
    requirement: "REQ-cedar-store-enforcement"
    verification:
      - kind: unit
        ref: "internal/store/store_test.go#TestSearchAuthzCallCount"
        status: pass
      - kind: unit
        ref: "internal/store/store_test.go#TestBulkFilterOwnAndSharedAdjacency"
        status: pass
      - kind: unit
        ref: "internal/store/store_test.go#TestBulkFilterZeroBucketFailsClosed"
        status: pass
      - kind: unit
        ref: "internal/store/store_test.go#TestBulkFilterOrderIndependent"
        status: pass
    human_judgment: false

# Metrics
duration: 5min
completed: 2026-07-17
status: complete
---

# Phase 22 Plan 02: Cedar Bulk-Recall Store Enforcement Summary

**Store's bulk read-filter builders (Search/List/ListScheduled/ListScopes/SearchDiscovery) now derive own/shared bucket access from the Plan-01 Cedar PDP via DecideBucket, while emitting byte-for-byte the same Qdrant filter shapes as the pre-Cedar hardcoded Subject switch — proven per-bucket (never per-record), fail-closed, and order-independent.**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-07-17T18:39:26-04:00 (first task commit)
- **Completed:** 2026-07-17T18:44:43-04:00
- **Tasks:** 3
- **Files modified:** 2

## Accomplishments
- `Store` gains an unexported `authz *authz.PDP` field defaulted via `authz.MustDefault()` in `New()` (mirroring the `now: time.Now` precedent) plus `WithAuthz(pdp *authz.PDP) Option` — all four existing `store.New(...)` call sites (`tools.go`, `testStore`, `retrieval_eval_test.go`, `tools_test.go`) compile unchanged
- `principalParams(subj Subject) (owner, kind string, ok bool)` — the single Subject→authz-primitives converter, living in `internal/store` to avoid an import cycle, failing closed (`ok=false`) for nil/unknown Subjects WITHOUT a PDP call
- `ownerOrSharedCondition`/`ownerOnlyCondition` converted to `*Store` methods that call `s.authz.DecideBucket(owner, kind, authz.ActionRead, authz.BucketOwn/BucketShared)` and compose the SAME `qdrant.NewMatch`/`matchNothing()` shapes as the pre-Cedar hardcoded switch; `listFilter` converted to a `*Store` method; all five call sites (`ownerScopeFilter`, `SearchDiscovery`, `ListScopes`, `List`, `ListScheduled`) route through the PDP-backed builders
- `decideBucketHook` — a same-package test seam (function-var field, mirroring `mintCandidate`/`deletePayloadKeys`) letting tests inject a call-counting or all-deny probe, since `*authz.PDP` is a sealed concrete type with no test constructor
- Four new bulk-path tests proving SC3 (per-bucket, not per-record), edge 1 (own+shared adjacency), edge 5 (zero-bucket fail-closed), and edge 6 (order-independence)
- The full pre-existing isolation/sharing suite (10 tests) passes byte-for-byte unchanged

## Task Commits

Each task was committed atomically:

1. **Task 1: Inject the PDP into Store (field + New default + WithAuthz + principalParams)** - `980378eb` (feat)
2. **Task 2: PDP-back the bulk read-filter builders (own/shared bucket decisions)** - `30a9c01f` (feat)
3. **Task 3: Bulk-path enforcement tests — per-bucket call count + edge cases** - `68b551c0` (test)

**Plan metadata:** (this commit)

## Files Created/Modified
- `internal/store/store.go` - `authz *authz.PDP` field + `decideBucketHook` test seam, `WithAuthz` Option, `New()` default, `principalParams` converter, `ownerOrSharedCondition`/`ownerOnlyCondition`/`listFilter` converted to `*Store` methods calling `s.decideBucket` (→ `s.authz.DecideBucket` unless hooked)
- `internal/store/store_test.go` - `authz` import; `TestSearchAuthzCallCount`, `TestBulkFilterOwnAndSharedAdjacency`, `TestBulkFilterZeroBucketFailsClosed`, `TestBulkFilterOrderIndependent`

## Decisions Made
- **decideBucketHook test seam:** `*authz.PDP` is a sealed concrete type (unexported fields, `MustDefault()` the only constructor), so it cannot be subclassed or wrapped as an interface without either widening `Store.authz`'s field type (which would break Task 1's exact-string acceptance criterion `authz \*authz.PDP`) or modifying `internal/authz` (out of this plan's declared file scope). Solution: an unexported `decideBucketHook func(owner, kind string, action authz.Action, bucket authz.Bucket) authz.Decision` field, nil-checked in a new `s.decideBucket(...)` helper that falls through to `s.authz.DecideBucket(...)`. Tests in the same package (`store_test.go`) set it directly (`s.decideBucketHook = ...`), exactly mirroring the existing `st.deletePayloadKeys = func(...)...` injection pattern already established in this file (round-8 injected-failure test). Zero new exported production API — `WithAuthz` remains the only new `Option`, satisfying Task 1's acceptance criteria verbatim.
- **A1 held:** `principalParams` hardcodes `kind = "human"` for both `authenticated` and `anonymous` arms (no policy conditions on it this phase), consistent with Plan 01's A1.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical Functionality] Added `decideBucketHook` test seam to `store.go`**
- **Found during:** Task 3 (writing `TestSearchAuthzCallCount`)
- **Issue:** The plan's Task 3 action text anticipated this exact problem ("if the concrete `*authz.PDP` cannot be subclassed, count via a thin store-test seam that the `WithAuthz` PDP exposes") but Task 3's declared `files_modified` was `internal/store/store_test.go` only. Task 1/2 had already locked `Store.authz`'s field type to the concrete `*authz.PDP` (required by Task 1's acceptance criteria), so no counting probe could be injected through `WithAuthz` alone.
- **Fix:** Added a minimal unexported `decideBucketHook` function-var field plus a `decideBucket` indirection method to `internal/store/store.go`, following the codebase's own established `mintCandidate`/`deletePayloadKeys` pattern for injecting test seams around fields with no interface. No new exported API.
- **Files modified:** `internal/store/store.go` (in addition to the plan's `internal/store/store_test.go` for Task 3)
- **Verification:** `go build ./...` clean; `TestSearchAuthzCallCount` proves DecideBucket called exactly 2× per Search regardless of record count (12 stored/returned records); pre-existing isolation suite unaffected.
- **Committed in:** `68b551c0` (Task 3 commit)

**2. [Rule 1 - Bug] Fixed revive unused-parameter lint on the all-deny hook**
- **Found during:** Task 3 (`task lint` after writing `TestBulkFilterZeroBucketFailsClosed`)
- **Issue:** `s.decideBucketHook = func(owner, kind string, action authz.Action, bucket authz.Bucket) authz.Decision { return authz.Decision{Allow: false} }` left all four parameters unused, tripping `golangci-lint`'s `revive` unused-parameter check.
- **Fix:** Renamed the unused parameters to `_`.
- **Files modified:** `internal/store/store_test.go`
- **Verification:** `golangci-lint run ./internal/store/...` → `0 issues`.
- **Committed in:** `68b551c0` (Task 3 commit)

---

**Total deviations:** 2 auto-fixed (1 missing critical functionality — required to prove SC3 as specified, 1 lint bug fix)
**Impact on plan:** Both auto-fixes stay within `internal/store`, add zero new exported production API, and were explicitly anticipated by the plan's own Task 3 contingency text. No scope creep.

## Issues Encountered
- `task lint` fails on `lint:yaml` (`.github/workflows/ci.yaml` pre-existing formatting drift) — same pre-existing, unrelated issue already logged in Plan 01's `deferred-items.md`. Confirmed `internal/store/store.go`/`store_test.go` (the only files this plan touches) pass `golangci-lint run ./internal/store/...` with `0 issues`; not in scope to fix here.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- The bulk-recall half of `REQ-cedar-store-enforcement` is complete: `Search`/`List`/`ListScheduled`/`ListScopes`/`SearchDiscovery` all derive their authz condition from `DecideBucket` while still enforcing it as the same Qdrant filter shapes; no per-record Cedar evaluation on the hot path (proven by `TestSearchAuthzCallCount`).
- Plan 03 (id-addressed gate wiring: `GetReadable`/`getWritable`/`OwnedOrAbsent`) can reuse `principalParams` and the `s.authz.DecideRecord` API directly — `DecideBucket`'s probe-resource construction stays fully internal to Plan 02's builders, no new coupling.
- The `decideBucketHook` seam is store-package-internal and available to Plan 03 if it needs an equivalent record-level call-counting proof, though Plan 03's gates are already off the hot bulk-recall path (D-03), so a call-count proof there is lower priority than here.
- No blockers.

---
*Phase: 22-cedar-authz-foundation-store-enforcement*
*Completed: 2026-07-17*

## Self-Check: PASSED

All claimed files verified present on disk; all 3 claimed task commit hashes
(`980378eb`, `30a9c01f`, `68b551c0`) verified present in `git log --oneline --all`.
