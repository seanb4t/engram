---
phase: 22-cedar-authz-foundation-store-enforcement
plan: 03
subsystem: auth
tags: [cedar, authz, store, id-addressed-gate, adr, policy-decision-point]

# Dependency graph
requires:
  - phase: 22-01
    provides: "internal/authz: PDP type, DecideRecord/DecideBucket, MustDefault(), Action/Bucket enums"
  - phase: 22-02
    provides: "Store.authz *authz.PDP field, WithAuthz Option, principalParams(subj) converter, decideBucketHook test-seam precedent"
provides:
  - "GetReadable/getWritable/OwnedOrAbsent (and FetchForUpdate via getWritable) consult s.authz.DecideRecord in the record-found branch, replacing the inner Subject type-switch"
  - "decideRecordHook test seam (mirrors decideBucketHook) for injecting an all-deny per-record probe without a real *authz.PDP construction"
  - "engram-cdr1 ADR — hand-authored, refines engram-cgb, reaffirms engram-xa6/engram-kyz/engram-12c"
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "decideRecordHook function-var field (mirrors decideBucketHook/mintCandidate/deletePayloadKeys) — same-package test seam for the sealed *authz.PDP's per-record decision, used identically to the Plan-02 bucket-decision seam"

key-files:
  created:
    - docs/adr/engram-cdr1-cedar-pdp-decides-predicate-store-enforces-qdrant-filter.md
  modified:
    - internal/store/store.go
    - internal/store/store_test.go

key-decisions:
  - "decideRecordHook (unexported *Store field + decideRecord indirection method, mirroring Plan 02's decideBucketHook) is the all-deny test seam Task 2 needed — *authz.PDP has no exported constructor besides MustDefault(), so a real all-deny PDP cannot be built from internal/store's test package. This keeps the acceptance-criteria grep ('s.authz.DecideRecord' present in store.go) satisfied — the literal call lives inside the decideRecord indirection method, exactly as s.authz.DecideBucket lives inside decideBucket."
  - "The Diagnostic-attach-to-span optional enhancement (action's final paragraph) was NOT implemented: authz.Decision.diag is fully unexported with no accessor, so internal/store has no way to read it even for owner-only debug logging without a new exported API in internal/authz, which was out of this plan's declared file scope (internal/store/store.go only). This is a no-op, not a deviation — the action text explicitly marked it 'Optional.'"

patterns-established: []

requirements-completed: [REQ-cedar-store-enforcement]

coverage:
  - id: D1
    description: "GetReadable/getWritable/OwnedOrAbsent replace the inner Subject type-switch with a single s.decideRecord (-> s.authz.DecideRecord) call in the record-found branch; the leading s.Get short-circuit is unchanged so Cedar is never consulted for an absent record; a nil/unknown Subject fails closed to ErrNotFound without a PDP call (principalParams ok=false). FetchForUpdate inherits the change via getWritable, unmodified."
    requirement: "REQ-cedar-store-enforcement"
    verification:
      - kind: unit
        ref: "go build ./... && go vet ./internal/store/..."
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
  - id: D2
    description: "A Cedar Deny on an existing, owned record maps to the exact same fmt.Errorf('%w: %s', ErrNotFound, id) used for a genuinely missing id (DEC-xa6); the Diagnostic never leaks (asserted via exact error-message equality, since authz.Decision.diag is fully unexported and unreachable from internal/store)."
    requirement: "REQ-cedar-store-enforcement"
    verification:
      - kind: unit
        ref: "internal/store/store_test.go#TestGetReadableDenyMapsToNotFound"
        status: pass
    human_judgment: false
  - id: D3
    description: "The s.Get -> ErrNotFound short-circuit precedes DecideRecord in every id-addressed gate: an absent id yields ErrNotFound (GetReadable/getWritable) or nil (OwnedOrAbsent) even under an all-deny PDP, proving Cedar is never consulted for a record that doesn't exist (Pattern 4)."
    requirement: "REQ-cedar-store-enforcement"
    verification:
      - kind: unit
        ref: "internal/store/store_test.go#TestIdAddressedAbsentShortCircuit"
        status: pass
    human_judgment: false
  - id: D4
    description: "engram-cdr1 ADR committed, hand-authored (no adr-render provenance header), refining engram-cgb and reaffirming engram-xa6/engram-kyz/engram-12c."
    requirement: "REQ-cedar-store-enforcement"
    verification:
      - kind: manual
        ref: "docs/adr/engram-cdr1-cedar-pdp-decides-predicate-store-enforces-qdrant-filter.md present; task license:check passes"
        status: pass
    human_judgment: false

# Metrics
duration: 3min
completed: 2026-07-17
status: complete
---

# Phase 22 Plan 03: Cedar Id-Addressed Store Enforcement Summary

**GetReadable/getWritable/OwnedOrAbsent now decide via the Plan-01 Cedar PDP's DecideRecord in their record-found branch — a Deny is byte-for-byte indistinguishable from a missing id — and a new hand-authored ADR (engram-cdr1) documents the refinement, completing REQ-cedar-store-enforcement and Phase 22.**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-07-17T19:24:42-04:00 (first task commit)
- **Completed:** 2026-07-17T19:26:33-04:00
- **Tasks:** 3
- **Files modified:** 3 (1 created, 2 modified)

## Accomplishments
- `GetReadable`/`getWritable`/`OwnedOrAbsent` replace the inner `switch sj := subj.(type)` block with `principalParams(subj)` + a single `s.decideRecord(...)` call (indirecting to `s.authz.DecideRecord`), preserving the exact `fmt.Errorf("%w: %s", ErrNotFound, id)` deny mapping (DEC-xa6) and the leading `s.Get` absent-record short-circuit (Pattern 4); `FetchForUpdate` inherits the change unmodified via `getWritable`
- `decideRecordHook` — a same-package test seam (function-var field, mirroring `decideBucketHook`) letting tests inject an all-deny per-record probe, since `*authz.PDP` is a sealed concrete type with no test constructor
- Two new gate tests: `TestGetReadableDenyMapsToNotFound` (Deny on an OWNED, EXISTING record still yields the plain missing-id error, no Diagnostic leak — SC4) and `TestIdAddressedAbsentShortCircuit` (absent id keeps its pre-Cedar contract across all three gates even under an all-deny PDP)
- The full pre-existing id-addressed gate suite (`TestGetReadableOwnerGate`, `TestOwnedOrAbsent`, `TestAnonBucketWriteSemantics`, `TestUpdateOwnerGateAndSharedFlag`, `TestAnonBucketReadIsolation`) passes byte-for-byte unchanged
- `docs/adr/engram-cdr1-cedar-pdp-decides-predicate-store-enforces-qdrant-filter.md` — hand-authored ADR (no dead `adr-render` provenance header, per D-12), refining `engram-cgb` and reaffirming `engram-xa6`/`engram-kyz`/`engram-12c`
- Whole-phase behavior-preservation oracle green: `task test` (full suite, all packages) passes; `golangci-lint run ./internal/store/... ./internal/authz/...` clean; `task license:check` clean

## Task Commits

Each task was committed atomically:

1. **Task 1: PDP-back the id-addressed gates (GetReadable / getWritable / OwnedOrAbsent)** - `fdc7982f` (feat)
2. **Task 2: Id-addressed enforcement tests — Deny maps to ErrNotFound, no Diagnostic leak** - `ddcb3f6d` (test)
3. **Task 3: Author the DEC-cdr1 refinement ADR** - `222f485f` (docs)

**Plan metadata:** (this commit)

## Files Created/Modified
- `internal/store/store.go` - `GetReadable`/`getWritable`/`OwnedOrAbsent` record-found branches call `s.decideRecord(owner, kind, action, m.Owner, m.Category, m.Visibility, m.Scope)`; `decideRecordHook` field + `decideRecord` indirection method (mirrors `decideBucketHook`/`decideBucket`)
- `internal/store/store_test.go` - `TestGetReadableDenyMapsToNotFound`, `TestIdAddressedAbsentShortCircuit`
- `docs/adr/engram-cdr1-cedar-pdp-decides-predicate-store-enforces-qdrant-filter.md` - new hand-authored ADR

## Decisions Made
- **decideRecordHook test seam** — see `key-decisions` in frontmatter. Same rationale and shape as Plan 02's `decideBucketHook` deviation: the plan's own read-first pointed at reusing "the all-deny / call-counting WithAuthz PDP seam from Plan 02," and since `*authz.PDP` remains a sealed concrete type with no exported constructor besides `MustDefault()`, the store-package-internal function-var seam is the only way to inject an all-deny per-record decision from `store_test.go` without adding new exported `internal/authz` API (out of this plan's declared `internal/store/store.go` file scope).
- **Diagnostic-attach-to-span left unimplemented** — the task action's optional final paragraph asked for attaching the Diagnostic to the existing OTel span at debug level; `authz.Decision.diag` is fully unexported with zero accessor, so `internal/store` has no way to read it. Not a deviation (explicitly optional in the plan text), just a hard capability boundary — a future phase would need to add an exported accessor to `internal/authz` first.

## Deviations from Plan

None — plan executed as written, modulo the anticipated test-seam addition (`decideRecordHook`), which the plan's own Task 2 `read_first` pointed toward reusing.

## Issues Encountered
- `task lint` (`lint:yaml` → `yamlfmt -lint .`) fails on the same pre-existing `.github/workflows/ci.yaml` formatting drift already logged in `.planning/phases/22-cedar-authz-foundation-store-enforcement/deferred-items.md` during Plan 01 — unrelated to any file this plan touches. Confirmed `golangci-lint run ./internal/store/... ./internal/authz/...` (the packages this plan modifies) returns `0 issues`, and `task license:check` (922 files checked) passes cleanly.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Both halves of `REQ-cedar-store-enforcement` are now complete: Plan 02 (bulk recall, `DecideBucket`) and Plan 03 (id-addressed gates, `DecideRecord`). Phase 22 is fully done.
- The `engram-cdr1` ADR is committed and cross-references `engram-cgb`/`engram-xa6`/`engram-kyz`/`engram-12c` by decision id, giving Phase 23 (service auth chain + tenancy isolation) a documented foundation to build service-principal `Tenant` bucket policies on.
- `decideRecordHook` and `decideBucketHook` are both store-package-internal test seams available to Phase 23 if it needs an equivalent per-tenant-bucket call-counting or all-deny proof.
- No blockers.

---
*Phase: 22-cedar-authz-foundation-store-enforcement*
*Completed: 2026-07-17*

## Self-Check: PASSED

All 3 claimed files verified present on disk; all 3 claimed task commit hashes
(`fdc7982f`, `ddcb3f6d`, `222f485f`) verified present in `git log --oneline --all`.
