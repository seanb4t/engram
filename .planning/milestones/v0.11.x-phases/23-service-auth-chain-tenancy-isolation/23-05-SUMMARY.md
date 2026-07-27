---
phase: 23-service-auth-chain-tenancy-isolation
plan: 05
subsystem: testing
tags: [store, authz, qdrant, tenancy-isolation, cedar, namespacedOwner]

# Dependency graph
requires:
  - phase: 22-cedar-authz-foundation-store-enforcement
    provides: "the Cedar-backed store Search/List filter (owner==X OR shared) this plan proves against, unchanged"
provides:
  - "TestServicePrincipalIsolation — SC4/D-07 proof: a namespacedOwner-encoded service-principal owner is isolated to its own bucket for private records, never collides with a human owner or the anonymous bucket, recalls empty (not an error) when it owns nothing, and isolation is insertion-order-independent"
  - "TestSharedCrossTenantReadIntended — permanent SC5/D-15/D-16 decision test: global shared-read across service-tenant boundaries is intended v0.11.x behavior, not a leak"
affects: [23-06-docs, future-full-abac-milestone]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Reused the existing store_test.go mk(id,owner,vis) + Search/List(Authenticated(owner)) assertion shape verbatim for a new owner-literal shape (namespacedOwner-encoded service owners) — zero new store code, zero new test infra"

key-files:
  created:
    - internal/store/service_principal_isolation_test.go
  modified: []

key-decisions:
  - "Used literal namespacedOwner-shaped strings (e.g. \"9:client_id:6:svc-aa\") directly in the store-package test rather than importing internal/auth's unexported namespacedOwner helper — the store treats owner as an opaque authz key by design (DEC-cgb), so this keeps the test in-package and dependency-free while still exercising the real encoding shape."
  - "The empty-input (owns-nothing) assertion needed a DEDICATED scope with zero shared records — the first attempt reused the main scope, which already had a serviceB shared record that D-15's global shared-read grant correctly surfaced to serviceC, producing a false test failure (not a store bug). Split into a separate emptyScope to isolate the two properties (owns-nothing-is-empty vs. shared-is-globally-readable) rather than conflating them."

requirements-completed: [REQ-service-principal-isolation]

coverage:
  - id: D1
    description: "A service principal cannot read another human's or another service principal's PRIVATE records via Search or List; a service owner string never equals a human owner or the empty anonymous owner; a service principal with no records recalls empty (not an error); isolation holds regardless of insertion order (SC4/D-07)"
    requirement: "REQ-service-principal-isolation"
    verification:
      - kind: unit
        ref: "internal/store/service_principal_isolation_test.go#TestServicePrincipalIsolation"
        status: pass
    human_judgment: false
  - id: D2
    description: "SC5/D-15/D-16 permanent decision test: two service-tenant owners, one with a shared record — the OTHER tenant's principal CAN read it (global shared-read is the intended, documented v0.11.x behavior)"
    requirement: "REQ-service-principal-isolation"
    verification:
      - kind: unit
        ref: "internal/store/service_principal_isolation_test.go#TestSharedCrossTenantReadIntended"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-07-17
status: complete
---

# Phase 23 Plan 05: Service Principal Tenancy Isolation Tests Summary

**Proved SC4/D-07 tenancy isolation and pinned the SC5/D-15/D-16 global-shared-read decision with two permanent tests against the unchanged Phase-22 store filters — zero new production code.**

## Performance

- **Duration:** 20 min
- **Started:** 2026-07-17T22:20:00Z
- **Completed:** 2026-07-17T22:28:00Z
- **Tasks:** 2
- **Files modified:** 1 (new file)

## Accomplishments
- `TestServicePrincipalIsolation` proves a namespacedOwner-encoded service-principal owner (client-credentials or static-token shaped) is isolated to its own bucket for private records: never sees another service principal's or a human's private records, never collides with a human owner or the anonymous (`""`) bucket, recalls zero hits (not an error) when it owns nothing, and the outcome is unaffected by insertion order.
- `TestSharedCrossTenantReadIntended` permanently pins the SC5/D-15/D-16 decision: two distinct service-tenant owners, one with a `shared` record — the OTHER tenant's principal CAN read it. This is a positive/must-read assertion documenting the intended v0.11.x global-shared-read behavior, not a restriction, so the decision can never be silently reinterpreted.
- Confirmed the pre-existing `internal/store/store_test.go` isolation/sharing suite passes unchanged (behavior-preservation oracle) — this plan added zero store-layer code.

## Task Commits

Each task was committed atomically:

1. **Task 1: TestServicePrincipalIsolation — private-record isolation + no anon/human collision (SC4/D-07)** - `2be2a1a1` (test)
2. **Task 2: TestSharedCrossTenantReadIntended — permanent SC5/D-15/D-16 decision test** - `a8b1af89` (test)

**Plan metadata:** (this commit)

## Files Created/Modified
- `internal/store/service_principal_isolation_test.go` - New test file (package `store`, Apache-2.0 SPDX header) with `TestServicePrincipalIsolation` and `TestSharedCrossTenantReadIntended`; reuses the existing `testStore(t)` + `mk(id,owner,vis)` + `Authenticated(owner)`/`Search`/`List` assertion shape from `store_test.go`

## Decisions Made
- Constructed service-owner test literals in the exact `namespacedOwner("claim", "value")` shape (`len(claim):claim:len(value):value`, e.g. `"9:client_id:6:svc-aa"`, `"12:static_token:6:svc-bb"`) directly as string constants in the store-package test, rather than importing `internal/auth`'s unexported helper — consistent with the store treating `owner` as an opaque authz key (DEC-cgb) and avoiding any store↔auth package coupling in tests.
- Split the "owns nothing recalls empty" (SC4 empty-input) assertion into its own dedicated scope with zero `shared` records. The first draft reused the main isolation scope, which legitimately contains a `serviceB` shared record — under D-15's correct global shared-read grant, `serviceC` (an authenticated caller) is entitled to see that shared record even though it owns nothing itself, so the original assertion of "0 hits" was actually testing the wrong property and failed. This was not a store bug; it was a test-design error conflating "owns nothing" with "sees nothing," which are distinct under global shared-read. Fixed by isolating the empty-input scope from any shared records so the two properties (isolation-when-empty vs. shared-cross-tenant-visibility) are each proven cleanly by their own scope.

## Deviations from Plan

None — plan executed exactly as written. The scope-isolation fix above was a test-authoring correction made while proving the acceptance criteria (caught by running the test itself before any commit), not a deviation from the plan's design; both tasks' acceptance criteria and `<done>` conditions are met exactly as specified, with zero production code touched.

## Issues Encountered
- Docker/testcontainers-go booted the pinned `qdrant/qdrant:v1.18.2` image cleanly; both new tests and the full `internal/store` suite ran against it with no `ENGRAM_QDRANT_TEST_ADDR` override needed. No Qdrant-skip occurred.
- `task lint` fails at the `lint:yaml` step in this working tree; reproduced identically with the new test file absent, confirming it is a pre-existing, out-of-scope issue unrelated to this plan (SCOPE BOUNDARY — not touched). `task license:check` (SPDX header enforcement, the lint concern this plan's file is actually subject to) passes cleanly (204 valid, 0 invalid). `go vet ./internal/store/...` and `golangci-lint run ./...` (via `task lint:go`, which completes before the unrelated `lint:yaml` failure) both report 0 issues.
- 1Password SSH-signing was locked when creating both task commits; per the sequential-execution note, retried each with `git -c commit.gpgsign=false commit ...` (per-commit override only, not a config change).

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- SC4 and SC5 are both closed for the store layer: isolation is proven, and the cross-tenant shared-read decision is explicit, written (this SUMMARY + inline test doc comments citing D-15/D-16), and permanently tested.
- Plan 06 (docs) can now reference this plan's tests as the executable oracle for the global-shared-read decision when writing the operator-facing guide.
- No blockers. This plan's tests are independent of the auth chain (Plans 01-04) — they exercise the store directly with literal owner strings, so they do not need to be re-run once the real client-credentials/static-token verifiers land; they already prove the store-side contract those verifiers will produce owners into.

---
*Phase: 23-service-auth-chain-tenancy-isolation*
*Completed: 2026-07-17*

## Self-Check: PASSED
- FOUND: internal/store/service_principal_isolation_test.go
- FOUND: .planning/phases/23-service-auth-chain-tenancy-isolation/23-05-SUMMARY.md
- FOUND commit: 2be2a1a1
- FOUND commit: a8b1af89
