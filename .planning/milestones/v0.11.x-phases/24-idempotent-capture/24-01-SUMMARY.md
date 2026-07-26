---
phase: 24-idempotent-capture
plan: 01
subsystem: api
tags: [go, qdrant, uuid, sha256, idempotency]

# Dependency graph
requires:
  - phase: 13-embedder-config-identity
    provides: "EmbedderIdentity payload-only stamp shape (json:\"-\" + payload()/fromPayload() codec) mirrored verbatim for IdempotencyFingerprint"
  - phase: 17-owner-claim-hardening
    provides: "internal/auth namespacedOwner injective length-prefix encoding, extended from 2 to 3 components"
provides:
  - "Memory.IdempotencyFingerprint payload-only field + idempotencyFingerprintKey const + payload()/fromPayload() codec"
  - "store.ErrIdempotencyConflict distinct sentinel (not ErrNotFound), pre-wired into connectError -> CodeAlreadyExists"
  - "idempotencyPointID(owner, scope, key) — deterministic, injective, owner-scoped UUIDv5 point-ID derivation"
  - "contentFingerprint(storeArgs) — tag-order-stable, field-sensitive sha256 content fingerprint"
affects: [24-02-handler-wiring, 25-supersession]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Payload-only server-set stamp (json:\"-\", manual payload()/fromPayload() codec) — second instance after EmbedderIdentity"
    - "Injective length-prefixed multi-component hash input (extends namespacedOwner's 2-component discipline to 3)"

key-files:
  created:
    - internal/server/idempotency.go
    - internal/server/idempotency_test.go
  modified:
    - internal/store/store.go
    - internal/store/store_test.go
    - internal/server/connecterror.go

key-decisions:
  - "engramIdempotencyNS fixed at 69fbe3e4-a53b-4d6e-971a-cad2f107e23c (generated via uuidgen) — committed permanently per D-04, changing it would silently un-dedup every keyed record"
  - "idempotencyPointID returns string (not uuid.UUID) matching the plan's artifact signature; callers get a wire-ready UUID string directly"
  - "Added TestIdempotencyPointIDKeySensitive (not in the original behavior list) to give golangci-lint's unparam check a varying key literal across the package — all originally-planned tests used a constant \"k\", tripping unparam since no production call site exists yet in this plan"

patterns-established:
  - "Payload-only content-fingerprint stamp: field json:\"-\", const *Key, unconditional write in payload(), defensive read in fromPayload() (legacy payload missing key -> zero value, no panic)"
  - "Distinct store sentinel error registered in connecterror.go's exhaustive switch even when the Connect lane cannot yet trigger it (pre-positioning)"

requirements-completed: [REQ-idempotent-capture]

coverage:
  - id: D1
    description: "Memory.IdempotencyFingerprint round-trips through payload()/fromPayload() and is wire-invisible (json:\"-\"); a legacy payload missing the key decodes to \"\""
    requirement: "REQ-idempotent-capture"
    verification:
      - kind: unit
        ref: "internal/store/store_test.go#TestPayloadRoundTripsIdempotencyFingerprint"
        status: pass
    human_judgment: false
  - id: D2
    description: "store.ErrIdempotencyConflict is a distinct sentinel, not ErrNotFound, and maps to a Connect code for future parity"
    requirement: "REQ-idempotent-capture"
    verification:
      - kind: unit
        ref: "go build ./... (compile-time proof errors.Is(ErrIdempotencyConflict, ErrNotFound) is false is exercised in the plan's acceptance check; connecterror.go switch row added)"
        status: pass
    human_judgment: false
  - id: D3
    description: "idempotencyPointID is deterministic, injective across owner/scope boundary shifts, owner-scoped, key-sensitive, and emits a valid UUID string"
    requirement: "REQ-idempotent-capture"
    verification:
      - kind: unit
        ref: "internal/server/idempotency_test.go#TestIdempotencyPointIDDeterministic"
        status: pass
      - kind: unit
        ref: "internal/server/idempotency_test.go#TestIdempotencyPointIDBoundaryShiftInjective"
        status: pass
      - kind: unit
        ref: "internal/server/idempotency_test.go#TestIdempotencyPointIDOwnerScoped"
        status: pass
      - kind: unit
        ref: "internal/server/idempotency_test.go#TestIdempotencyPointIDKeySensitive"
        status: pass
      - kind: unit
        ref: "internal/server/idempotency_test.go#TestIdempotencyPointIDValidUUID"
        status: pass
    human_judgment: false
  - id: D4
    description: "contentFingerprint is stable under tag reordering and sensitive to every client-authored field, independent of the idempotency key"
    requirement: "REQ-idempotent-capture"
    verification:
      - kind: unit
        ref: "internal/server/idempotency_test.go#TestContentFingerprintTagOrderStable"
        status: pass
      - kind: unit
        ref: "internal/server/idempotency_test.go#TestContentFingerprintFieldSensitivity"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-07-18
status: complete
---

# Phase 24 Plan 01: Idempotency Primitives Summary

**Payload-only content-fingerprint stamp on `Memory`, a distinct `ErrIdempotencyConflict` sentinel, and two pure helpers — `idempotencyPointID` (deterministic UUIDv5 over owner/scope/key) and `contentFingerprint` (sha256 over client-authored fields) — with zero live call sites yet.**

## Performance

- **Duration:** 12 min
- **Started:** 2026-07-18T15:31:00-04:00
- **Completed:** 2026-07-18T15:43:41-04:00
- **Tasks:** 2
- **Files modified:** 5 (3 modified, 2 created)

## Accomplishments
- `Memory.IdempotencyFingerprint` payload-only field added, mirroring `EmbedderIdentity`'s exact shape (`json:"-"`, unconditional write in `payload()`, defensive read in `fromPayload()`)
- `store.ErrIdempotencyConflict` distinct sentinel added and pre-wired into `connectError` (`CodeAlreadyExists`), never folded into `ErrNotFound`
- `idempotencyPointID(owner, scope, key string) string` — deterministic, injective (length-prefixed, extends `namespacedOwner`'s 2-component discipline to 3), owner-scoped, key-sensitive, valid-UUID output
- `contentFingerprint(a storeArgs) string` — sha256 over a fixed field order with sorted tags, stable under tag reordering, sensitive to every client-authored field, independent of the idempotency key itself
- Both helpers are pure, Qdrant-free, and fully unit-tested; nothing in the live write path calls them yet (Plan 02's scope)

## Task Commits

Each task was committed atomically:

1. **Task 1: Payload-only fingerprint field, ErrIdempotencyConflict sentinel, and codec round-trip** - `e210895e` (feat)
2. **Task 2: Deterministic point-ID and content-fingerprint pure helpers (test-first)** - `334b496c` (test, RED) → `bf1a4fa8` (feat, GREEN)

**Plan metadata:** pending (this commit)

_Note: Task 2 is TDD — RED (`334b496c`, compile-fail on undefined functions) then GREEN (`bf1a4fa8`)._

## Files Created/Modified
- `internal/store/store.go` - `IdempotencyFingerprint` field, `idempotencyFingerprintKey` const, `ErrIdempotencyConflict` sentinel, `payload()`/`fromPayload()` write/read sites
- `internal/store/store_test.go` - `TestPayloadRoundTripsIdempotencyFingerprint`
- `internal/server/connecterror.go` - pre-positioning switch row for `store.ErrIdempotencyConflict` -> `CodeAlreadyExists`
- `internal/server/idempotency.go` - new file: `engramIdempotencyNS`, `idempotencyPointID`, `contentFingerprint`
- `internal/server/idempotency_test.go` - new file: 7 pure-function unit tests (RED then GREEN)

## Decisions Made
- `engramIdempotencyNS` generated fresh via `uuidgen` (`69fbe3e4-a53b-4d6e-971a-cad2f107e23c`) rather than reusing RESEARCH.md's illustrative example value, per the plan's explicit instruction that the example is not to be shipped verbatim.
- `idempotencyPointID` returns `string` (via `.String()`) rather than `uuid.UUID`, matching the plan's `artifacts_this_phase_produces` table signature exactly.
- Both new symbols placed in a new `internal/server/idempotency.go` file (not appended to `tools.go`) per the plan's explicit file list, keeping the two pure primitives cohesive and giving their tests a clean home ahead of Plan 02's handler wiring.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug/Lint] Added a key-sensitivity test to resolve a golangci-lint `unparam` finding**
- **Found during:** Task 2, post-GREEN lint pass
- **Issue:** All six originally-planned behavior tests called `idempotencyPointID` with the same literal `"k"` for the `key` parameter (the boundary-shift test deliberately needs a fixed key, but the other tests didn't vary it). Since Plan 01 introduces no live call site yet, `key` was the only parameter with a single constant value across every caller in the package, and `golangci-lint`'s `unparam` linter flagged it as effectively unused.
- **Fix:** Added `TestIdempotencyPointIDKeySensitive` (differing key, same owner+scope, asserts different IDs) and varied the literal in `TestIdempotencyPointIDValidUUID` from `"k"` to `"check-uuid"`. This is a genuine coverage improvement (key-sensitivity wasn't explicitly pinned before) as well as a lint fix — no `//nolint` directive used, per CLAUDE.md.
- **Files modified:** `internal/server/idempotency_test.go`
- **Verification:** `golangci-lint run ./internal/server/... ./internal/store/...` reports 0 issues; all tests still pass.
- **Committed in:** `bf1a4fa8` (Task 2 GREEN commit, same commit — the lint fix landed before the GREEN commit was made)

---

**Total deviations:** 1 auto-fixed (1 lint/bug-class, Rule 1)
**Impact on plan:** Additive test coverage only; no production behavior changed; no scope creep.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `idempotencyPointID` and `contentFingerprint` are pure, tested, and ready for Plan 02 to wire into `storeArgs`/`toMemory`/`persistAndEnqueue` per D-08's check-before-embed ordering.
- `IdempotencyFingerprint` and `ErrIdempotencyConflict` are in place for Plan 02's `checkIdempotentReplay` helper to read/write without any further store-layer changes.
- No blockers. `go build ./...`, `go vet ./internal/server/...`, `golangci-lint run ./internal/server/... ./internal/store/...`, and `go test ./internal/store/... ./internal/server/...` are all green.

---
*Phase: 24-idempotent-capture*
*Completed: 2026-07-18*

## Self-Check: PASSED

All created/modified files found on disk; all commit hashes (e210895e, 334b496c, bf1a4fa8, 34db9ac8) found in git log.
