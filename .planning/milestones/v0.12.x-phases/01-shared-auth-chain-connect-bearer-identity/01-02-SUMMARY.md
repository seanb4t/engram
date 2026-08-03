---
phase: 01-shared-auth-chain-connect-bearer-identity
plan: 02
subsystem: auth
tags: [connect, connectrpc, bearer-token, reseal, mcpauth, go-sdk, parity]

# Dependency graph
requires:
  - phase: 01-shared-auth-chain-connect-bearer-identity/01-01
    provides: "auth.Lane, server.NewConnectResolver, connectLaneKey/withConnectLane/laneFromConnectContext, three-return connectResolver"
provides:
  - "internal/server/connectreseal.go: newConnectResealInterceptor gated on auth.LaneCookie (D-09) — a bearer-authenticated request never re-seals a session cookie it did not authenticate with"
  - "internal/server/connectapi_bearer_parity_test.go: TestBearerLaneParity and siblings, proving MCP-vs-Connect bearer identity/actor-attribution/rejection parity"
  - "internal/server/connectapi_service_auth_parity_test.go: stubOIDCVerifier fixture now carries a real future Expiration, reusable under auth.EnforceExpiry"
affects: ["01-03 (headless mount / connect.headless config, docs)"]

# Actuals (#2632)
actuals:
  tokens: 5793
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Lane-gated cookie-lane side effects: the reseal interceptor's skip condition now joins the CSRF exemption in reading laneFromConnectContext exclusively — never request headers or cookie presence — to decide whether a cookie-lane-only side effect runs."
    - "Single-verifier-value parity tests: a shared verifier is constructed exactly once per test case and passed by reference into both the MCP middleware round-trip and the Connect mount, so a drift between the two call sites cannot hide behind two independently-correct-looking stubs."

key-files:
  created:
    - internal/server/connectapi_bearer_parity_test.go
  modified:
    - internal/server/connectreseal.go
    - internal/server/connectreseal_test.go
    - internal/server/connectapi_service_auth_parity_test.go

key-decisions:
  - "D-09: newConnectResealInterceptor's existing err/resp/reseal-nil disjunction gains exactly one clause (laneFromConnectContext(ctx) != auth.LaneCookie) — no restructure, mirroring the D-08 CSRF gate's shape so both cookie-lane side effects now read the same single source of truth."
  - "REVIEWS.md MED-9: stubOIDCVerifier (connectapi_service_auth_parity_test.go, a v0.11.x fixture) now sets an explicit future Expiration, since 01-02's parity tests are the first callers to wrap it in auth.EnforceExpiry, which hard-rejects the zero value (D-05). All pre-existing callers of the fixture are unaffected."
  - "The zero-Expiration parity case (TestBearerLaneParityRejectsZeroExpirationOnBothLanes) deliberately builds its own stub rather than reusing stubOIDCVerifier, per the plan's explicit instruction not to rely on the fixture's former accidental zero value."

requirements-completed:
  - REQ-connect-lane-provenance
  - REQ-connect-bearer-identity

coverage:
  - id: D1
    description: "A bearer-authenticated Connect request never re-seals a session cookie it did not authenticate with, even when a valid session cookie is also present; cookie-lane requests re-seal exactly as before, and every pre-existing skip condition still holds."
    requirement: REQ-connect-lane-provenance
    verification:
      - kind: unit
        ref: "internal/server/connectreseal_test.go#TestResealGatesOnCookieLane"
        status: pass
      - kind: unit
        ref: "internal/server/connectreseal_test.go#TestResealStillRunsForCookieLane"
        status: pass
      - kind: unit
        ref: "internal/server/connectreseal_test.go#TestResealSkippedForUnknownLane"
        status: pass
      - kind: unit
        ref: "internal/server/connectreseal_test.go#TestResealSkipConditionsUnchanged"
        status: pass
    human_judgment: false
  - id: D2
    description: "The same bearer token, verified by the same verifier value, produces the same actor and owner on the MCP and Connect lanes, including the UserID-empty owner-fallback case."
    requirement: REQ-connect-bearer-identity
    verification:
      - kind: unit
        ref: "internal/server/connectapi_bearer_parity_test.go#TestBearerLaneParity"
        status: pass
      - kind: unit
        ref: "internal/server/connectapi_bearer_parity_test.go#TestBearerLaneParityActorFallback"
        status: pass
    human_judgment: false
  - id: D3
    description: "A token rejected on the MCP lane (past or zero Expiration) is rejected on the Connect lane too, with byte-for-byte identical MCP 401 wire bodies to what the go-sdk emitted before this phase."
    requirement: REQ-connect-bearer-identity
    verification:
      - kind: unit
        ref: "internal/server/connectapi_bearer_parity_test.go#TestBearerLaneParityRejectsExpiredOnBothLanes"
        status: pass
      - kind: unit
        ref: "internal/server/connectapi_bearer_parity_test.go#TestBearerLaneParityRejectsZeroExpirationOnBothLanes"
        status: pass
      - kind: unit
        ref: "internal/server/connectapi_bearer_parity_test.go#TestBearerLaneParityRejectionBodiesMatch"
        status: pass
    human_judgment: false
  - id: D4
    description: "The shared stubOIDCVerifier test fixture carries a real future Expiration so it is reusable under auth.EnforceExpiry, and every pre-existing test that consumes it is still green (REVIEWS.md MED-9)."
    verification:
      - kind: unit
        ref: "internal/server/connectapi_bearer_parity_test.go#TestStubOIDCVerifierCarriesFutureExpiration"
        status: pass
      - kind: unit
        ref: "internal/server/connectapi_service_auth_parity_test.go#TestServiceAuthChainParity"
        status: pass
      - kind: unit
        ref: "internal/server/connectapi_service_auth_parity_test.go#TestServiceAuthChainParity_EmptyOwnerFailsClosedPostComposition"
        status: pass
    human_judgment: false

duration: 13min
completed: 2026-07-31
status: complete
---

# Phase 01 Plan 02: Shared Auth Chain & Connect Bearer Identity — Reseal Lane Gate + Bearer Parity Summary

**The reseal interceptor now gates on the same lane stamp the CSRF exemption reads (D-09), and a new MCP-vs-Connect parity suite proves the same bearer token resolves to the identical actor/owner on both lanes and is rejected identically when expired — retiring RESEARCH.md Assumption A1 by measurement.**

## Performance

- **Duration:** ~13 min
- **Completed:** 2026-07-31
- **Tasks:** 2 completed
- **Files modified:** 4 (1 created, 3 modified)

## Accomplishments

- Extended `newConnectResealInterceptor`'s existing `err != nil || resp == nil || reseal == nil` skip condition with one additional clause, `laneFromConnectContext(ctx) != auth.LaneCookie` — a bearer-authenticated Connect request carrying a valid session cookie no longer triggers a session re-seal, closing the narrow both-credentials case D-01 creates. Cookie-lane requests re-seal exactly as before; every pre-existing skip condition (nil `resealFunc`, upstream error, nil response) still holds.
- Proved (not assumed) RESEARCH.md Assumption A1: `TestBearerLaneParity` and `TestBearerLaneParityActorFallback` construct a single verifier value, wrap it once in `auth.EnforceExpiry`, and drive it through the MCP tool path (`mcpauth.RequireBearerToken` middleware round-trip) and a real Connect `StoreMemory` write (mounted via `d.mountConnect` with `NewConnectResolver(verify, nil)`) — both lanes' stored records carry an identical `Actor` and owner. **Disposition: A1 CONFIRMED.** Bearer-caller actor attribution on Connect matches the MCP lane exactly, including the `UserID`-empty owner-fallback case, because both lanes flow through the same `callerFromTokenInfo` choke point.
- Proved cross-lane rejection agreement (SC1, D-05): `TestBearerLaneParityRejectsExpiredOnBothLanes` and `TestBearerLaneParityRejectsZeroExpirationOnBothLanes` show a token rejected on one lane is rejected on the other for both a past `Expiration` and a zero `Expiration`, mapping to `connect.CodeUnauthenticated` on the Connect side.
- Proved wire-body parity (REVIEWS.md MED-8): `TestBearerLaneParityRejectionBodiesMatch` asserts the MCP lane's 401 body is exactly `token expired` / `token missing expiration` (`==`, not `Contains`) — byte-for-byte what the go-sdk emitted before `auth.EnforceExpiry` existed, guarding against the doubled-message regression D-04's planner note forbids.
- Fixed the shared `stubOIDCVerifier` test fixture (REVIEWS.md MED-9): it now returns a `TokenInfo` with an explicit `Expiration: time.Now().Add(time.Hour)` instead of the zero value, so it is reusable under `auth.EnforceExpiry` (which hard-rejects a zero `Expiration`, D-05). `TestStubOIDCVerifierCarriesFutureExpiration` guards the field going forward. All pre-existing tests in `connectapi_service_auth_parity_test.go` (`TestServiceAuthChainParity`, `TestServiceAuthChainParity_EmptyOwnerFailsClosedPostComposition`) remain green — they never wrap the fixture in `EnforceExpiry`, so the added field is inert for them.

## Task Commits

Each task was committed atomically:

1. **Task 1: Reseal only for requests that authenticated on the cookie lane** - `fab5a04f` (feat, tdd)
2. **Task 2: Prove MCP-vs-Connect bearer parity, including actor attribution** - `9fa57df5` (test, tdd)

_Both tasks carry `tdd="true"`; RED evidence for both was captured via a temporary revert/restore cycle (see "Deviations from Plan" below), matching the process note recorded in the 01-01 SUMMARY._

## Files Created/Modified

- `internal/server/connectreseal.go` - `newConnectResealInterceptor`'s skip condition gains the `laneFromConnectContext(ctx) != auth.LaneCookie` clause (D-09); doc comment updated to explain why this closes the narrow both-credentials case rather than the common bearer case
- `internal/server/connectreseal_test.go` - `TestResealGatesOnCookieLane`, `TestResealStillRunsForCookieLane`, `TestResealSkippedForUnknownLane`, `TestResealSkipConditionsUnchanged`; the pre-existing `TestNewConnectResealInterceptor_FiresOnSuccess` now stamps `auth.LaneCookie` on its context so it still exercises the positive path under the new gate
- `internal/server/connectapi_bearer_parity_test.go` - New file: `TestStubOIDCVerifierCarriesFutureExpiration`, `TestBearerLaneParity`, `TestBearerLaneParityActorFallback`, `TestBearerLaneParityRejectsExpiredOnBothLanes`, `TestBearerLaneParityRejectsZeroExpirationOnBothLanes`, `TestBearerLaneParityRejectionBodiesMatch`, plus the `bearerParityMCPContext` / `mountBearerParityConnect` / `bearerParityStoreMemory` test helpers
- `internal/server/connectapi_service_auth_parity_test.go` - `stubOIDCVerifier` gains an explicit future `Expiration` field with a comment recording why it is load-bearing (MED-9)

## Decisions Made

- Followed the plan's D-09 one-clause-extension shape exactly, per `01-PATTERNS.md`'s pinned "current gate" / "D-09 addition" diff — no restructuring of `newConnectResealInterceptor`.
- Drove the Connect side of the bearer parity tests through a real `d.mountConnect` + `httptest.NewServer` + generated client round-trip (rather than calling the handler function directly) so the test exercises the full interceptor chain (subject → CSRF exemption → reseal), not just `callerFromTokenInfo` in isolation — matching the plan's explicit instruction to mount via `d.mountConnect(...)`.
- Used `newSpyDeps()` (no Qdrant dependency) for both lanes in the parity tests, consistent with `TestWriteParity`'s existing fixture convention, so these tests run unconditionally rather than being Qdrant-skip-gated.

## Deviations from Plan

None — plan executed exactly as written. One process note, not a deviation from the shipped code:

**RED-first evidence captured via temporary revert/restore rather than a separate `test(...)` commit**, following the same process the 01-01 plan used and documented. Both tasks carry `tdd="true"`.

- Task 1 RED (the `laneFromConnectContext` clause temporarily reverted to the pre-D-09 condition, with a placeholder `_ = auth.LaneCookie` to keep the file compiling under `go vet` while capturing the failure — `go build` alone does not catch `_test.go` compile breaks, per the carried-context note):
  ```
  === RUN   TestResealGatesOnCookieLane
  --- FAIL: TestResealGatesOnCookieLane (0.00s)
  === RUN   TestResealSkippedForUnknownLane
  --- FAIL: TestResealSkippedForUnknownLane (0.00s)
  ```
  Task 1 GREEN (real clause restored, diff identical to pre-revert):
  ```
  === RUN   TestResealGatesOnCookieLane
  --- PASS: TestResealGatesOnCookieLane (0.00s)
  === RUN   TestResealStillRunsForCookieLane
  --- PASS: TestResealStillRunsForCookieLane (0.00s)
  === RUN   TestResealSkippedForUnknownLane
  --- PASS: TestResealSkippedForUnknownLane (0.00s)
  === RUN   TestResealSkipConditionsUnchanged
  --- PASS: TestResealSkipConditionsUnchanged (0.00s)
  ```
- Task 2 RED (`stubOIDCVerifier`'s `Expiration` field temporarily reverted to the fixture's original zero value): exactly the failure pattern the plan predicted — the happy-path tests that reuse the fixture under `auth.EnforceExpiry` fail, while the two rejection tests (which build their own stubs, never `stubOIDCVerifier`) are unaffected:
  ```
  === RUN   TestStubOIDCVerifierCarriesFutureExpiration
  --- FAIL: TestStubOIDCVerifierCarriesFutureExpiration (0.00s)
  === RUN   TestBearerLaneParity
  --- FAIL: TestBearerLaneParity (0.00s)
  === RUN   TestBearerLaneParityActorFallback
  --- FAIL: TestBearerLaneParityActorFallback (0.00s)
  === RUN   TestBearerLaneParityRejectsExpiredOnBothLanes
  --- PASS: TestBearerLaneParityRejectsExpiredOnBothLanes (0.00s)
  === RUN   TestBearerLaneParityRejectsZeroExpirationOnBothLanes
  --- PASS: TestBearerLaneParityRejectsZeroExpirationOnBothLanes (0.00s)
  === RUN   TestBearerLaneParityRejectionBodiesMatch
  --- PASS: TestBearerLaneParityRejectionBodiesMatch (0.00s)
  ```
  Task 2 GREEN (real `Expiration` field restored, `git diff --stat` identical to pre-revert — 8 lines added, no test function added/removed/renamed):
  ```
  === RUN   TestStubOIDCVerifierCarriesFutureExpiration
  --- PASS: TestStubOIDCVerifierCarriesFutureExpiration (0.00s)
  === RUN   TestBearerLaneParity
  --- PASS: TestBearerLaneParity (0.01s)
  === RUN   TestBearerLaneParityActorFallback
  --- PASS: TestBearerLaneParityActorFallback (0.00s)
  ```

## Issues Encountered

- `task fmt` (dprint) reformatted several unrelated pre-existing files outside this task's scope (`.claude/settings.json`, `docs-site/package.json`, `internal/webauth/static/_app/version.json`, `ui/tsconfig.json`) as a side effect of a repo-wide formatting pass. Reverted with `git checkout --` before each commit per the scope-boundary rule — not part of this plan's diff.
- Git commit signing (1Password SSH agent) transiently failed three times in a row ("failed to fill whole buffer", "agent returned an error") while committing Task 2, coinciding with a 1Password app auto-update/restart observed in the process list. Resolved by retrying after a short wait; no code or config change was needed, and no signing was bypassed.

## User Setup Required

None - no external service configuration required. This plan adds no new production symbol beyond one clause in `connectreseal.go`; no operator-visible behavior changes.

## Next Phase Readiness

- D-09 is now enforced identically to D-08: both cookie-lane side effects (CSRF exemption, session reseal) read `laneFromConnectContext(ctx)` exclusively, so 01-03's headless-mount work inherits a fully lane-gated Connect surface with no remaining cookie-lane special case.
- RESEARCH.md Assumption A1 is confirmed in writing (see Accomplishments above); 01-03 can rely on Connect-bearer actor attribution matching the MCP lane without re-verifying it.
- No blockers.

---
*Phase: 01-shared-auth-chain-connect-bearer-identity*
*Completed: 2026-07-31*

## Self-Check: PASSED

All created/modified files verified present on disk; both task commits (`fab5a04f`, `9fa57df5`) verified present in `git log --oneline --all`.
