---
phase: 23-service-auth-chain-tenancy-isolation
plan: 03
subsystem: auth
tags: [go, mcpauth, verifier-chain, oidc, static-token]

# Dependency graph
requires:
  - phase: 23-service-auth-chain-tenancy-isolation
    provides: "Plan 01's NewService fail-closed lane and Plan 02's staticTokenVerifier — both mcpauth.TokenVerifier-shaped, consumed as chain lane args"
provides:
  - "chainVerifier(oidcHuman, oidcService, static mcpauth.TokenVerifier) mcpauth.TokenVerifier — the D-01 combinator withAuth (Plan 06) will wrap in place of the lone human verifier"
  - "looksLikeJWT(token string) bool — the D-04 structural (two-dot) discriminator, no parse"
affects: [23-06-service-auth-wiring]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Structural pre-verification routing (discriminate by shape, never 'try all, take first success')"
    - "Nil-mechanism guard: an unconfigured lane (nil TokenVerifier) denies via errors.Join(mcpauth.ErrInvalidToken, ...) rather than being dereferenced"

key-files:
  created:
    - internal/auth/chain.go
    - internal/auth/chain_test.go
  modified: []

key-decisions:
  - "D-02 order and D-03 nil-guards are intrinsic to a correct chainVerifier — Task 1's GREEN implementation already satisfied Task 2's behavioral tests; Task 2 landed as a test-only commit that locks the contract rather than driving new production code (see Deviations)"
  - "Added a laneUnrecognized third enum value (beyond laneOIDC/laneStatic) so the routing switch has an explicit default deny-by-default arm for any future non-binary bearer shape, even though today only the empty string reaches it"

patterns-established:
  - "chainVerifier / verifyOIDCBranch: route-then-verify, stateless closures over mcpauth.TokenVerifier, sentinel errors that never embed the raw bearer"

requirements-completed: [REQ-service-auth-chain]

coverage:
  - id: D1
    description: "looksLikeJWT structurally discriminates JWT-shaped (two dots) from opaque bearers, no parse"
    requirement: "REQ-service-auth-chain"
    verification:
      - kind: unit
        ref: "internal/auth/chain_test.go#TestDiscriminator_LooksLikeJWT"
        status: pass
    human_judgment: false
  - id: D2
    description: "chainVerifier routes JWT-shaped bearers to the OIDC branch only and opaque bearers to the static branch only — the two mechanisms never both run on one token"
    requirement: "REQ-service-auth-chain"
    verification:
      - kind: unit
        ref: "internal/auth/chain_test.go#TestChainVerifier_RoutesJWTToOIDCBranchOnly"
        status: pass
      - kind: unit
        ref: "internal/auth/chain_test.go#TestChainVerifier_RoutesOpaqueToStaticBranchOnly"
        status: pass
    human_judgment: false
  - id: D3
    description: "OIDC branch tries the human verifier before the client-credentials verifier (D-02 order); first success wins"
    requirement: "REQ-service-auth-chain"
    verification:
      - kind: unit
        ref: "internal/auth/chain_test.go#TestChainVerifier_HumanTriedBeforeService"
        status: pass
      - kind: unit
        ref: "internal/auth/chain_test.go#TestChainVerifier_ServiceTriedAfterHumanFails"
        status: pass
    human_judgment: false
  - id: D4
    description: "A nil routed verifier (unconfigured mechanism, D-03) or an unrecognized bearer shape denies via errors.Join(mcpauth.ErrInvalidToken, ...) — never a nil-pointer panic, never a fallthrough identity"
    requirement: "REQ-service-auth-chain"
    verification:
      - kind: unit
        ref: "internal/auth/chain_test.go#TestChainVerifier_NilServiceOnJWTBranchDenies"
        status: pass
      - kind: unit
        ref: "internal/auth/chain_test.go#TestChainVerifier_NilStaticOnOpaqueBranchDenies"
        status: pass
      - kind: unit
        ref: "internal/auth/chain_test.go#TestChainVerifier_UnrecognizedShapeDeniesByDefault"
        status: pass
    human_judgment: false
  - id: D5
    description: "Human-only config (nil service, nil static) is byte-for-byte the pre-chain human-only behavior; the chain is deterministic across repeated verification of the same token"
    requirement: "REQ-service-auth-chain"
    verification:
      - kind: unit
        ref: "internal/auth/chain_test.go#TestChainVerifier_HumanOnlyConfigMatchesPreChainBehavior"
        status: pass
      - kind: unit
        ref: "internal/auth/chain_test.go#TestChainVerifier_Deterministic"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-07-18
status: complete
---

# Phase 23 Plan 03: Verifier Chain Routing Combinator Summary

**`chainVerifier` combinator over `mcpauth.TokenVerifier` with a structural JWT-vs-opaque discriminator, D-02 OIDC try-order, and D-03 nil-mechanism deny-by-default guards — zero new interface, zero new Subject variant.**

## Performance

- **Duration:** 12 min
- **Started:** 2026-07-18T02:09:12Z
- **Completed:** 2026-07-18T02:21:00Z
- **Tasks:** 2
- **Files modified:** 2 (both new)

## Accomplishments
- `looksLikeJWT` — a `strings.Count(token, ".") == 2` structural check, no base64/parse, deciding routing before any verifier runs (D-04).
- `chainVerifier(oidcHuman, oidcService, static mcpauth.TokenVerifier) mcpauth.TokenVerifier` — routes JWT-shaped bearers to the OIDC branch (human tried first, then client-credentials, D-02) and opaque bearers to the static branch, with the two mechanism families never both invoked on one token (anti-Pitfall-9, proven via call-count spies).
- D-03 nil-mechanism guard: any of the three lane verifiers may be `nil` when unconfigured; a routed nil verifier resolves to `errors.Join(mcpauth.ErrInvalidToken, ...)`, never a panic — verified for both the JWT branch (nil service) and the opaque branch (nil static).
- Human-only config (nil service, nil static) proven byte-for-byte the pre-chain human-only behavior, and the chain proven deterministic (same token verified twice yields the same outcome, one call per verification — no per-call state).

## Task Commits

1. **Task 1: structural discriminator + chainVerifier routing (D-04/D-01)** — RED `48e9cdfd`, GREEN `48fdddf6`
2. **Task 2: D-02 order, D-03 nil-mechanism guard, deny-by-default, determinism** — test-only `45896f06` (see Deviations — Task 1's implementation already satisfied this behavior)

**Plan metadata:** (this commit)

## Files Created/Modified
- `internal/auth/chain.go` - `chainVerifier`, `looksLikeJWT`, `discriminate`/`lane` enum, `verifyOIDCBranch`
- `internal/auth/chain_test.go` - routing-isolation, order, nil-guard, human-only-parity, and determinism tests using call-counting stub `mcpauth.TokenVerifier` funcs

## Decisions Made
- **D-06 invariant carried into code:** every chain lane resolves to the existing `authenticated{sub}` `store.Subject` via the unchanged `TokenInfo.Extra[OwnerClaimExtraKey]` contract — `chain.go` introduces zero new Subject-shaped types; it only composes over the `mcpauth.TokenVerifier` function type. This plan is intentionally silent on the *concrete* human/service/static verifier constructors (Plans 01/02) and the config wiring (Plan 04/06) — `chainVerifier`'s three args are opaque `mcpauth.TokenVerifier` closures, keeping this plan fully independent.
- Added a third `laneUnrecognized` enum value alongside `laneOIDC`/`laneStatic` so the routing `switch` carries an explicit `default:` deny arm, rather than relying on a two-way `if/else` with an implicit assumption that every string is either JWT-shaped or opaque. Currently only the empty-string bearer reaches it (a non-empty opaque bearer still routes to `laneStatic` and denies there if `static` is nil) — this keeps the deny-by-default guarantee structurally explicit rather than incidental.

## Deviations from Plan

### Auto-fixed Issues

**1. [Not a Rule 1-4 case — TDD sequencing note] Task 2's tests passed without new implementation**
- **Found during:** Task 2 RED attempt
- **Issue:** Task 2's acceptance criteria (D-02 order, D-03 nil-guard, human-only parity, determinism) require a `chainVerifier` implementation that already has to make an ordering choice and guard nil dereferences to be minimally correct — `verifyOIDCBranch` cannot try "the OIDC branch" at all without deciding whether to try `oidcHuman` or `oidcService` first, and cannot avoid a nil-pointer panic on an unconfigured lane without the guard. Task 1's GREEN commit therefore already implemented the full behavior Task 2's tests describe.
- **Fix:** Followed the plan's fail-fast RED-phase guidance — investigated why the tests passed immediately (the feature already existed, not a broken test) and confirmed via manual code inspection of `chain.go` (committed at `48fdddf6`) that `verifyOIDCBranch`'s try-order and nil-checks were already present. Rather than force an artificial RED (e.g., temporarily deleting logic the plan's own Task 1 action already specified), committed Task 2's tests as a single test-only commit that locks the existing contract in place, and documented the sequencing here per Task boundary transparency.
- **Files modified:** `internal/auth/chain_test.go` (test-only; no `chain.go` change in the Task 2 commit)
- **Verification:** `go test ./internal/auth/... -run Chain -v` — all 9 `TestChainVerifier_*` tests plus `TestDiscriminator_LooksLikeJWT` pass; full `go test ./internal/auth/...` (including Plan 01/02's suites) passes unchanged.
- **Committed in:** `45896f06`

---

**Total deviations:** 1 (TDD sequencing note, not a Rule 1-4 auto-fix — no production-code change)
**Impact on plan:** None on scope or correctness. Task 1 and Task 2's production code are one coherent unit (a routing closure that is order-defined and nil-safe by construction); the two-commit split still gives Task 2 its own dedicated test commit as specified.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - `chainVerifier` is fully wired to its three lane-verifier arguments; wiring the concrete constructors (Plans 01/02) and config (Plan 04) into `withAuth` is explicitly Plan 06's scope, not a stub.

## Threat Flags
None - both `<threat_model>` entries (T-23-02 mechanism-selection spoofing, T-23-08 nil-verifier DoS) are mitigated by the routing/nil-guard implementation itself; no new surface introduced beyond what the threat model already scoped.

## Next Phase Readiness
- `chainVerifier`/`looksLikeJWT` are ready for Plan 06 to wrap in `withAuth`, passing `NewService`'s `TokenVerifier()` (Plan 01), `staticTokenVerifier.TokenVerifier()` (Plan 02), and the existing human `Verifier.TokenVerifier()` as the three lane args.
- No blockers. Plan 04 (config) and Plan 05 (if any) are unaffected by this plan's implementation choices — the three-arg signature is the entire contract this plan exposes.

---
*Phase: 23-service-auth-chain-tenancy-isolation*
*Completed: 2026-07-18*
