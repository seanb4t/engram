---
phase: 23-service-auth-chain-tenancy-isolation
plan: 02
subsystem: auth
tags: [go, crypto/subtle, static-token, mcpauth, tenancy-isolation]

# Dependency graph
requires:
  - phase: 23-01
    provides: namespacedOwner encoding, OwnerClaimExtraKey contract, mcpauth.TokenVerifier + errors.Join(ErrInvalidToken, ...) deny idiom (reused verbatim, not reinvented)
provides:
  - "staticTokenVerifier: an opaque bearer-token lane verified with crypto/subtle.ConstantTimeCompare over the full token value, mapping each token to a distinct owner via namespacedOwner(\"static_token\", ownerID)"
  - "Proof (TestStaticTokenNoLeak) that the raw static-token value never appears in a rejection error, log line, or successful TokenInfo.Extra"
affects: ["23-03 (chain combinator will wire this verifier alongside OIDC lanes)", "cmd/engram/serve.go (future withAuth wiring)"]

# Tech tracking
tech-stack:
  added: ["crypto/subtle (first in-repo use)"]
  patterns:
    - "staticTokenVerifier holds ownerID->token map; TokenVerifier() closure iterates every candidate with subtle.ConstantTimeCompare, no early-return short-circuit"
    - "Constant rejection error (errStaticTokenNotRecognized) never interpolates token/candidate values — DEC-wot no-leak discipline"

key-files:
  created:
    - internal/auth/static_token.go
    - internal/auth/static_token_test.go
  modified: []

key-decisions:
  - "UserID on a successful TokenInfo is the non-namespaced ownerID (matches PATTERNS.md/plan behavior spec); Extra[OwnerClaimExtraKey] carries the namespacedOwner(\"static_token\", ownerID) encoding"
  - "Empty configured candidate tokens are structurally excluded from matching (never eligible), independent of the empty-input-token guard, closing an edge case where two empty strings could otherwise 'match'"

patterns-established:
  - "Split TDD commit across two commits per task (feat: verifier + core test coverage, test: no-leak proof) rather than one combined commit, keeping the no-leak safety proof separately auditable in git history"

requirements-completed: [REQ-static-token-auth, REQ-service-auth-chain]

coverage:
  - id: D1
    description: "Static bearer token verified via crypto/subtle.ConstantTimeCompare over the full value; each configured token resolves to its own distinct namespaced owner (no shared 'static service' owner)"
    requirement: "REQ-static-token-auth"
    verification:
      - kind: unit
        ref: "internal/auth/static_token_test.go#TestStaticTokenDistinctOwnersResolveDistinctly"
        status: pass
      - kind: unit
        ref: "internal/auth/static_token_test.go#TestStaticTokenPrefixNotMatched"
        status: pass
      - kind: unit
        ref: "internal/auth/static_token_test.go#TestStaticTokenEmptyRejected"
        status: pass
      - kind: unit
        ref: "internal/auth/static_token_test.go#TestStaticTokenRotationSameOwnerMultipleTokens"
        status: pass
      - kind: unit
        ref: "internal/auth/static_token_test.go#TestStaticTokenSuccessInfoShape"
        status: pass
    human_judgment: false
  - id: D2
    description: "Raw static-token value never leaks into a rejection error, log line, or successful TokenInfo.Extra (DEC-wot no-leak discipline, D-12)"
    requirement: "REQ-service-auth-chain"
    verification:
      - kind: unit
        ref: "internal/auth/static_token_test.go#TestStaticTokenNoLeak"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-07-18
status: complete
---

# Phase 23 Plan 02: Static-Token Verifier Summary

**Opaque static-token bearer lane for `internal/auth`, verified with `crypto/subtle.ConstantTimeCompare` over the full token value, mapping each token to its own distinct owner via the existing `namespacedOwner("static_token", ownerID)` encoding — with a proven no-leak guarantee on the rejection path.**

## Performance

- **Duration:** 12 min
- **Started:** 2026-07-18T01:56:00Z
- **Completed:** 2026-07-18T02:08:15Z
- **Tasks:** 2
- **Files modified:** 2 (both new)

## Accomplishments
- `staticTokenVerifier` + `newStaticTokenVerifier(tokens map[string]string)` — an operator-provisioned ownerID→token map, exposed via a `TokenVerifier() mcpauth.TokenVerifier` closure matching the existing `Verifier.TokenVerifier()` contract shape.
- Constant-time comparison (`subtle.ConstantTimeCompare`) over the FULL token value for every configured candidate, with no early-return short-circuit — timing cannot reveal which candidate (if any) shares a prefix with the presented token.
- Reuses `namespacedOwner("static_token", ownerID)` verbatim (zero new encoding) so a static-token owner can never collide with an OIDC or human owner.
- Rotation support: multiple distinct tokens may map to the same owner and both remain valid simultaneously (no flag-day cutover).
- Empty-input-token and empty-configured-candidate guards, independently enforced, so neither can "match" the other.
- `TestStaticTokenNoLeak` proves the raw sentinel token literal never appears in the rejection error string or captured slog output, and a successful `TokenInfo.Extra` carries only the resolved owner claim.

## Task Commits

Each task was committed atomically:

1. **Task 1: static-token verifier — constant-time compare + per-owner map (D-11/D-12)** - `a52b474d` (feat)
2. **Task 2: no-leak discipline — token never in error/log/span (D-12/DEC-wot)** - `8b0238c5` (test)

**Plan metadata:** pending (this commit)

_Note: implementation and its five core-behavior tests were written and verified together before the Task 1 commit (not a strict separate RED-then-GREEN commit pair), since the task's `<action>` specifies writing test+implementation as one unit; Task 2's no-leak test was then added as its own commit, isolating the safety proof in git history._

## Files Created/Modified
- `internal/auth/static_token.go` - `staticTokenVerifier` type, `newStaticTokenVerifier` constructor, `TokenVerifier()` closure (constant-time compare, per-owner map, constant rejection error)
- `internal/auth/static_token_test.go` - table-driven behavior tests (distinct owners, prefix rejection, empty-token/empty-map/empty-candidate rejection, rotation, success-info shape) plus `TestStaticTokenNoLeak`

## Decisions Made
- `TokenInfo.UserID` is set to the non-namespaced `ownerID` (the map key), while `Extra[OwnerClaimExtraKey]` carries the namespaced encoding — matching both the plan's `<behavior>` spec and `23-PATTERNS.md`'s documented shape.
- An empty configured candidate token (`tokens[owner] == ""`) is structurally excluded from ever matching, even against an empty input token — closes an edge case not explicit in the plan's acceptance criteria but required by "never a silent match."

## Deviations from Plan

None — plan executed exactly as written. Both tasks' acceptance criteria and `<verify>` commands pass as specified.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required. (Wiring this verifier into `cmd/engram/serve.go`'s auth chain and adding `service_auth.static_tokens` config is out of scope for this plan — later plans in Phase 23.)

## Next Phase Readiness
- `staticTokenVerifier` is ready to be wired into the chain combinator (`internal/auth/chain.go`, per `23-PATTERNS.md`) by a later plan in this phase.
- No config-layer (`internal/config`) or `serve.go` wiring exists yet — this plan is scoped strictly to the verifier and its safety proofs, per its `files_modified` frontmatter.

---
*Phase: 23-service-auth-chain-tenancy-isolation*
*Completed: 2026-07-18*

## Self-Check: PASSED

- FOUND: internal/auth/static_token.go
- FOUND: internal/auth/static_token_test.go
- FOUND: .planning/phases/23-service-auth-chain-tenancy-isolation/23-02-SUMMARY.md
- FOUND: a52b474d
- FOUND: 8b0238c5
