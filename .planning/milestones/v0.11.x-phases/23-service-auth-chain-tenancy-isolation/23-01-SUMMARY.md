---
phase: 23-service-auth-chain-tenancy-isolation
plan: 01
subsystem: auth
tags: [oidc, go-oidc, client-credentials, fail-closed, tenancy-isolation]

# Dependency graph
requires:
  - phase: 22-cedar-authz-foundation-store-enforcement
    provides: Cedar PDP defense-in-depth (`forbid ... unless principal.owner != ""`) and the
      store's owner-bucket enforcement this plan's fail-closed reject backstops.
provides:
  - "Verifier.failClosed field on internal/auth.Verifier — false (human/no-issuer lane,
    default) preserves fail-open-to-anonymous; true (service lane) hard-rejects an
    empty-owner resolution at the TokenVerifier boundary."
  - "NewService(ctx, issuer, audience, ownerClaims) constructor — a second, independently
    audience-configured *Verifier for the client-credentials service lane (failClosed=true)."
  - "The FIRST proven test of the phase (D-10/SC2): an authenticated service principal whose
    owner claim resolves empty is rejected with errors.Is(err, mcpauth.ErrInvalidToken) and a
    nil TokenInfo — never a TokenInfo carrying owner==\"\"."
affects: [23-02, 23-03, 23-04, 23-05, 23-06]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Per-lane *Verifier construction for independent OIDC audience (D-14) — go-oidc bakes
      ClientID/SkipClientIDCheck into oidc.Config at construction time; there is no per-call
      audience override, so two lanes always need two Verifier objects."
    - "oidctest.Server + oidctest.SignIDToken (already used in internal/webauth) as the
      white-box test fixture for exercising the real TokenVerifier/ClaimIdentity code path
      against a genuinely signature-verified token, instead of hand-faking oidc.IDToken's
      unexported claims field."

key-files:
  created:
    - internal/auth/service_owner_failclosed_test.go
  modified:
    - internal/auth/auth.go

key-decisions:
  - "Reused ClaimIdentity/namespacedOwner verbatim for the service lane's owner resolution —
    zero new owner-encoding logic (D-05/D-06)."
  - "Built NewService as a second oidc.NewProvider(...) call (per-issuer discovery) rather than
    adding NewFromProvider — the plan left this optional planner discretion, and a second plain
    discovery is simplest with zero new exported surface."
  - "Split implementation across two commits per the plan's two tasks (field+reject+first-test,
    then NewService+its tests), even though both touch the same two files — temporarily trimmed
    Task 2 content, committed Task 1, then restored it, to keep task-level atomic commits."

patterns-established:
  - "Verifier.failClosed as the single behavioral switch between the human (fail-open) and
    service (fail-closed) lanes — no new type, no new interface."

requirements-completed: [REQ-service-owner-failclosed, REQ-service-auth-chain]

coverage:
  - id: D1
    description: "A failClosed=true Verifier hard-rejects an authenticated service principal
      whose owner claim resolves empty, at the TokenVerifier boundary (401 via
      errors.Is(err, mcpauth.ErrInvalidToken)), never as a TokenInfo carrying owner==\"\"."
    requirement: "REQ-service-owner-failclosed"
    verification:
      - kind: unit
        ref: "internal/auth/service_owner_failclosed_test.go#TestFailClosedRejectsEmptyOwner"
        status: pass
    human_judgment: false
  - id: D2
    description: "A client-credentials-shaped claims map (client_id present, no email) resolves
      through the existing ClaimIdentity to a non-empty namespaced owner via the service
      owner-claim order, and the same map flows through a failClosed Verifier to a TokenInfo
      carrying that owner."
    requirement: "REQ-service-owner-failclosed"
    verification:
      - kind: unit
        ref: "internal/auth/service_owner_failclosed_test.go#TestFailClosedResolvesClientCredentialsOwner"
        status: pass
    human_judgment: false
  - id: D3
    description: "The human/no-issuer lane (failClosed=false) is behavior-preserving: resolving
      an empty owner still returns a TokenInfo (nil error) with an empty owner_claim, exactly as
      before this plan."
    requirement: "REQ-service-owner-failclosed"
    verification:
      - kind: unit
        ref: "internal/auth/service_owner_failclosed_test.go#TestFailClosedDoesNotAffectHumanLane"
        status: pass
    human_judgment: false
  - id: D4
    description: "NewService(ctx, issuer, audience, ownerClaims) constructs a failClosed=true
      Verifier with its own independently-configured audience (D-14); New's signature and
      fail-open behavior are unchanged."
    requirement: "REQ-service-auth-chain"
    verification:
      - kind: unit
        ref: "internal/auth/service_owner_failclosed_test.go#TestNewServiceSetsFailClosed"
        status: pass
      - kind: unit
        ref: "internal/auth/service_owner_failclosed_test.go#TestNewServiceIndependentAudienceFromHumanLane"
        status: pass
    human_judgment: false

duration: 14min
completed: 2026-07-17
status: complete
---

# Phase 23 Plan 01: Service-Lane Fail-Closed Empty-Owner Reject Summary

**A `Verifier.failClosed` field plus a `NewService` constructor in `internal/auth` that hard-rejects an authenticated service principal resolving to an empty owner at the OIDC verifier boundary, proven as the phase's first test (SC2/D-08/D-09/D-10).**

## Performance

- **Duration:** 14 min
- **Started:** 2026-07-17T21:47:47-04:00 (previous plan commit)
- **Completed:** 2026-07-17T22:01:13-04:00
- **Tasks:** 2
- **Files modified:** 2 (1 modified, 1 created)

## Accomplishments

- `Verifier.failClosed bool` field: the human/no-issuer lane (default `false`) keeps its
  existing fail-open-to-anonymous behavior on an unresolved owner claim; a fail-closed
  (service) lane hard-rejects the same resolution at the `TokenVerifier` boundary with
  `errors.Join(mcpauth.ErrInvalidToken, ...)` before any `TokenInfo` is constructed.
- `NewService(ctx, issuer, audience, ownerClaims) (*Verifier, error)`: mirrors `New` exactly
  but sets `failClosed: true` and builds its own `*oidc.IDTokenVerifier` with its own
  `ClientID`/`SkipClientIDCheck`, independent of the human lane's audience (D-14) — go-oidc has
  no per-call audience override, so the two lanes structurally cannot share a `*Verifier`.
- The phase's FIRST test (D-10) proves the milestone's #1 risk closed: an authenticated
  service principal whose configured owner claims (`client_id`, `azp`) are both absent from
  an otherwise-valid, signature-verified token is rejected at the verifier boundary
  (`errors.Is(err, mcpauth.ErrInvalidToken)`, nil `TokenInfo`) — never silently mapped to the
  anonymous `owner==""` bucket.
- A client-credentials-shaped claims map (`client_id` present) is proven to resolve through the
  existing `ClaimIdentity` to a non-empty `namespacedOwner("client_id", ...)`-encoded owner, and
  the same map flows through a `failClosed` `Verifier` to a `TokenInfo` carrying that owner.

## Task Commits

Each task was committed atomically:

1. **Task 1: FIRST TEST — service-lane empty-owner fail-closed reject (SC2/D-08/D-10)** - `57a7874b` (feat)
2. **Task 2: NewService per-lane audience constructor (D-14)** - `afb2a260` (feat)

_Note: RED (build failure without the `failClosed` field/branch) confirmed against
`git show HEAD:internal/auth/auth.go` before implementing, per TDD discipline — no separate RED
commit was created since the plan grouped test+implementation into one task with a single
`tdd="true"` action block._

## Files Created/Modified

- `internal/auth/auth.go` — added `Verifier.failClosed`, the fail-closed reject branch inside
  `TokenVerifier()`, `NewService(...)`, and updated doc comments (package + `Verifier` + `New`)
  to name the two lanes.
- `internal/auth/service_owner_failclosed_test.go` — new file: `newFailClosedTestFixture`
  (an `oidctest.Server`-backed real-signature-verified OIDC fixture, mirroring
  `internal/webauth/oidc_exchange_test.go`'s existing pattern) plus five tests covering the
  reject, the resolve, the human-lane regression, and `NewService`'s `failClosed=true`/
  independent-audience guarantees.

## Decisions Made

- **Test fixture approach:** rather than hand-constructing an `*oidc.IDToken` (its `claims`
  field is unexported, so a hand-rolled `fakeIDV` stub — the pattern the plan's `read_first`
  pointed at — cannot carry arbitrary claims across the package boundary), tests use
  `oidc.NewVerifier`/`oidctest.Server`/`oidctest.SignIDToken` to build a genuinely
  signature-verified token with real claims. This is the exact fixture pattern already
  established in `internal/webauth/oidc_exchange_test.go`, so it introduces no new test
  convention and exercises the real `TokenVerifier`→`ClaimIdentity` path end to end rather than
  a partial mock.
- **NewFromProvider omitted:** the plan flagged this same-issuer JWKS-reuse optimization as
  optional planner discretion; `NewService` does a plain second `oidc.NewProvider` discovery
  instead, keeping the exported surface minimal (zero new exported symbols beyond `NewService`
  itself).
- **Two-commit split for a two-file task pair:** both tasks' `files` lists were identical
  (`auth.go` + the new test file). To keep true per-task atomic commits, Task 2's
  content (`NewService`, its doc-comment additions, its two tests) was written in full, then
  temporarily trimmed out before Task 1's commit, then restored for Task 2's commit — so each
  commit's diff maps exactly to its task's stated deliverable.

## Deviations from Plan

None — plan executed exactly as written. The optional `NewFromProvider` same-issuer
optimization (explicitly left to planner discretion) was not added; `NewService` performing its
own discovery is the simpler of the two equally-correct options the plan named.

## Issues Encountered

- The plan's `read_first` pointed at `auth_test.go`'s `fakeIDV` stub as "the white-box
  construction pattern," but that stub cannot carry claims (the real `*oidc.IDToken.claims`
  field is unexported and existing tests never populate it). Resolved by using the
  `oidctest`-based real-signature fixture already established in `internal/webauth` instead —
  a stronger test (exercises real JWT verification, not just the stub's pass-through) with an
  equally-established in-repo precedent.

## User Setup Required

None — no external service configuration required. `NewService` is not yet wired into any
call site (`cmd/engram/serve.go`'s `withAuth`); that wiring is a later plan in this phase.

## Next Phase Readiness

- `internal/auth` now exposes both lane constructors and the fail-closed guarantee the rest of
  Phase 23 builds on (the `chainVerifier` combinator, the static-token verifier, and
  `withAuth`'s wiring in subsequent plans all depend on `NewService`/`failClosed` existing
  exactly as built here).
- No blockers. The human-lane regression suite (`internal/auth`'s full existing test suite)
  passes unchanged, confirming this plan's behavior-preservation guarantee for the no-issuer/
  human lane.

---
*Phase: 23-service-auth-chain-tenancy-isolation*
*Completed: 2026-07-17*
