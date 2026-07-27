---
phase: 23-service-auth-chain-tenancy-isolation
plan: 06
subsystem: auth
tags: [oidc, client-credentials, static-token, chain-verifier, tenancy-isolation, cedar, adr, docs]

# Dependency graph
requires:
  - phase: 23-service-auth-chain-tenancy-isolation
    provides: "auth.NewService (Plan 01), newStaticTokenVerifier (Plan 02), chainVerifier combinator (Plan 03), ServiceAuthConfig + ParseServiceStaticTokens (Plan 04), TestSharedCrossTenantReadIntended (Plan 05)"
provides:
  - "withAuth (cmd/engram/serve.go) composes up to three independently-enabled verifiers into auth.ChainVerifier at the single MCP bearer call site"
  - "auth.ChainVerifier and auth.NewStaticTokenVerifier exported so cmd/engram can wire them"
  - "TestServiceAuthChainParity proving D-07 (identical owner resolution regardless of which chain lane answered) and the D-08 fail-closed reject surviving composition"
  - "docs/adr/engram-svct-service-tenant-global-shared-read.md recording the D-15 global cross-tenant shared-read decision"
  - "reference/auth.md + guides/configure.md document ENGRAM_SERVICE_AUTH_* config, the fail-closed guarantee, static-token safety, and the no-revocation kill-switch"
affects: [24-idempotent-capture, service-auth-connect-lane-followon]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Config-presence-gated verifier construction in withAuth: each of the three lanes is built only when its own config field is non-empty, then composed via auth.ChainVerifier — no single 'mode enum'"
    - "Exported package-internal combinators (auth.ChainVerifier, auth.NewStaticTokenVerifier) only when a cross-package call site needs them, keeping everything else in internal/auth unexported"

key-files:
  created:
    - internal/server/connectapi_service_auth_parity_test.go
    - docs/adr/engram-svct-service-tenant-global-shared-read.md
  modified:
    - cmd/engram/serve.go
    - cmd/engram/serve_test.go
    - internal/auth/chain.go
    - internal/auth/chain_test.go
    - internal/auth/static_token.go
    - internal/auth/static_token_test.go
    - docs-site/src/content/docs/reference/auth.md
    - docs-site/src/content/docs/guides/configure.md

key-decisions:
  - "Exported internal/auth's Wave-1 chainVerifier -> ChainVerifier and newStaticTokenVerifier/staticTokenVerifier -> NewStaticTokenVerifier/StaticTokenVerifier so cmd/engram (package main) can call them from withAuth; internal test files renamed in lockstep, behavior unchanged."
  - "Added two withAuth-level tests (TestWithAuth_NoLaneConfigured_ReturnsHandlerUnchanged, TestWithAuth_HumanOnlyConfig_RejectsUnauthenticated) beyond the plan's files_modified list, to directly observe the acceptance criteria's behavior-preservation claim at the wiring layer rather than relying solely on the lower-level chain_test.go proofs."
  - "ADR docs/adr/engram-svct-service-tenant-global-shared-read.md is hand-authored with NO SPDX header comment, matching the repo's existing docs/adr/*.md convention (the .licenserc.yaml paths-ignore list explicitly excludes docs/adr/*.md from the SPDX check — ADR markdown provenance is covered by the repo LICENSE, mirroring engram-cdr1 and engram-slr8's hand-authored, no-source=bd: precedent)."

patterns-established:
  - "The auth-chain wiring pattern (build N verifiers conditionally on config presence, compose with a combinator, pass the single result to the one downstream call site) is now the template for any future auth-lane addition."

requirements-completed: [REQ-service-auth-chain, REQ-service-principal-isolation]

coverage:
  - id: D1
    description: "withAuth composes the human, client-credentials, and static-token verifiers into auth.ChainVerifier at the ONE call site (cmd/engram/serve.go), each built only when its own config is present"
    requirement: "REQ-service-auth-chain"
    verification:
      - kind: unit
        ref: "cmd/engram/serve_test.go#TestWithAuth_NoLaneConfigured_ReturnsHandlerUnchanged"
        status: pass
      - kind: unit
        ref: "cmd/engram/serve_test.go#TestWithAuth_HumanOnlyConfig_RejectsUnauthenticated"
        status: pass
      - kind: unit
        ref: "internal/auth/chain_test.go#TestChainVerifier_HumanOnlyConfigMatchesPreChainBehavior"
        status: pass
    human_judgment: false
  - id: D2
    description: "Owner-claim resolution and isolation are identical regardless of which chain verifier (human OIDC, client-credentials OIDC, or static token) answered"
    requirement: "REQ-service-principal-isolation"
    verification:
      - kind: unit
        ref: "internal/server/connectapi_service_auth_parity_test.go#TestServiceAuthChainParity"
        status: pass
    human_judgment: false
  - id: D3
    description: "The D-08 service-lane empty-owner fail-closed reject survives chain composition (T-23-01)"
    verification:
      - kind: unit
        ref: "internal/server/connectapi_service_auth_parity_test.go#TestServiceAuthChainParity_EmptyOwnerFailsClosedPostComposition"
        status: pass
    human_judgment: false
  - id: D4
    description: "Service-auth config surface, no-revocation kill-switch, and the global cross-tenant shared-read decision are documented (ADR + reference + config guide)"
    verification:
      - kind: other
        ref: "rumdl check docs/adr/engram-svct-service-tenant-global-shared-read.md; task license:check"
        status: pass
    human_judgment: false

# Metrics
duration: 20min
completed: 2026-07-17
status: complete
---

# Phase 23 Plan 06: Wire the Service-Auth Verifier Chain into withAuth Summary

**withAuth (cmd/engram/serve.go) now composes the human OIDC, client-credentials OIDC, and static-token verifiers into a single auth.ChainVerifier at the ONE call site, proven behavior-preserving and lane-independent, with the global shared-read decision recorded as an ADR.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-07-17T22:27:59-04:00 (approx, based on prior plan's last commit)
- **Completed:** 2026-07-17T22:40:43-04:00
- **Tasks:** 3
- **Files modified:** 9 (2 created, 7 modified)

## Accomplishments

- `withAuth` builds the human (`auth.New`), client-credentials (`auth.NewService`), and static-token (`auth.NewStaticTokenVerifier`) verifiers independently, only when each lane's own config is present, and wraps the result with `auth.ChainVerifier` before passing it to `mcpauth.RequireBearerToken` — the single call site this phase's whole verifier chain wires into (D-01/D-03).
- Exported `internal/auth`'s Wave-1 `chainVerifier`/`newStaticTokenVerifier` as `ChainVerifier`/`NewStaticTokenVerifier` so `cmd/engram` (package `main`) can construct the chain — the only cross-package surface change needed; all pre-existing package-internal tests renamed in lockstep with zero logic change.
- `TestServiceAuthChainParity` (new) proves D-07: a human-OIDC-shaped, client-credentials-shaped, and static-token TokenInfo each resolve, through the SAME `SubjectFromTokenInfo`/`callerFromTokenInfo` choke point every tool handler uses, to a stable non-empty owner — the static-token lane exercises the REAL `auth.NewStaticTokenVerifier`, not a stub. A companion test proves the D-08 empty-owner fail-closed reject survives composition (T-23-01).
- Two `withAuth`-level tests (`TestWithAuth_NoLaneConfigured_ReturnsHandlerUnchanged`, `TestWithAuth_HumanOnlyConfig_RejectsUnauthenticated`) directly observe the acceptance criteria's behavior-preservation claim at the wiring layer: no lane configured passes the inner handler through untouched; a human-only config gates it behind a 401 for unauthenticated requests.
- New ADR `docs/adr/engram-svct-service-tenant-global-shared-read.md` records D-15: `shared`-visibility records remain readable by ANY authenticated caller, including a service principal from a different service tenant, for v0.11.x — pinned by `internal/store.TestSharedCrossTenantReadIntended` (Plan 05) — with per-tenant scoping explicitly deferred to a future full-ABAC milestone.
- `reference/auth.md` gains a "Service principals (machine-to-machine auth)" section (chain order, `ENGRAM_SERVICE_AUTH_*` vars, fail-closed guarantee, static-token constant-time-compare/no-leak discipline, the no-revocation kill-switch, and a link to the new ADR); `guides/configure.md` gains the matching env-var table.

## Task Commits

Each task was committed atomically:

1. **Task 1: wire chainVerifier into withAuth (the ONE call site, D-01/D-03)** - `9c459033` (feat)
2. **Task 2: TestServiceAuthChainParity — same resolution/isolation regardless of lane (D-07)** - `13245a73` (test)
3. **Task 3: document service-auth config, no-revocation, and the cross-tenant shared-read decision (D-13/D-15)** - `19f970df` (docs)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `cmd/engram/serve.go` - `withAuth` now takes `config.ServiceAuthConfig`, builds up to three verifiers conditionally, composes via `auth.ChainVerifier`
- `cmd/engram/serve_test.go` - `TestWithAuth_NoLaneConfigured_ReturnsHandlerUnchanged`, `TestWithAuth_HumanOnlyConfig_RejectsUnauthenticated`, `newServeTestOIDCServer` fixture
- `internal/auth/chain.go` - `chainVerifier` exported as `ChainVerifier` (behavior unchanged)
- `internal/auth/chain_test.go` - references updated to `ChainVerifier`
- `internal/auth/static_token.go` - `staticTokenVerifier`/`newStaticTokenVerifier` exported as `StaticTokenVerifier`/`NewStaticTokenVerifier` (behavior unchanged)
- `internal/auth/static_token_test.go` - references updated to `NewStaticTokenVerifier`
- `internal/server/connectapi_service_auth_parity_test.go` - new `TestServiceAuthChainParity` + fail-closed-post-composition test
- `docs/adr/engram-svct-service-tenant-global-shared-read.md` - new ADR recording D-15/D-16
- `docs-site/src/content/docs/reference/auth.md` - new "Service principals" section
- `docs-site/src/content/docs/guides/configure.md` - new "Service principals (machine-to-machine)" env-var table

## Decisions Made

- Exported `chainVerifier`/`newStaticTokenVerifier` rather than adding parallel wrapper functions in `internal/auth` — a straight rename keeps one canonical implementation and matches the plan's acceptance criteria text ("auth.chainVerifier(... or the exported constructors from Plans 01/02").
- Followed the plan's Task 2 guidance literally: stub verifiers for the human/client-credentials OIDC lanes (fast, deterministic, no OIDC discovery server needed for the parity proof itself — the human/service lane's OWN unit tests in `internal/auth` already prove the real `auth.New`/`auth.NewService` behavior against a live `oidctest` server), and the REAL `auth.NewStaticTokenVerifier` for the static lane.
- ADR carries no leading SPDX comment, matching `docs/adr/*.md`'s explicit `.licenserc.yaml` exclusion and the repo's existing hand-authored-ADR precedent (`engram-cdr1`, `engram-slr8`).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added withAuth-level regression tests not listed in the plan's `files_modified`**
- **Found during:** Task 1
- **Issue:** The plan's acceptance criteria explicitly require "a human-only config path... still constructs a chain containing only the human verifier, and a fully-empty config still returns the handler with the 'validation DISABLED' warning" to be true, but `files_modified` only lists `cmd/engram/serve.go` — leaving that specific wiring-layer claim unverified by any test (the pre-existing `chain_test.go` proves the composition function's behavior, not `withAuth`'s config-to-verifier wiring).
- **Fix:** Added `TestWithAuth_NoLaneConfigured_ReturnsHandlerUnchanged` and `TestWithAuth_HumanOnlyConfig_RejectsUnauthenticated` to `cmd/engram/serve_test.go`, using a minimal local `oidctest` fixture mirroring `internal/auth/service_owner_failclosed_test.go`'s pattern.
- **Files modified:** `cmd/engram/serve_test.go`
- **Verification:** `go test ./cmd/... -run WithAuth -v` — both pass.
- **Committed in:** `9c459033` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 missing critical test coverage)
**Impact on plan:** Necessary to close a verification gap the plan's own acceptance criteria created; no scope creep beyond proving the explicitly-stated behavior-preservation claim.

## Issues Encountered

- `task` (the full `lint`+`test` gate) fails at `lint:yaml` (`yamlfmt -lint .github/workflows/ci.yaml`) — confirmed **pre-existing and unrelated to this plan**: stashing all of this plan's changes and re-running `yamlfmt -lint .github/workflows/ci.yaml` directly still fails identically (a local `yamlfmt` binary version-formatting drift on a file this plan never touches). `task lint:go` (golangci-lint), `rumdl check` on every doc/ADR this plan created or touched, `task license:check`, `gofmt -l` on every Go file this plan touched, and the full `go test ./...` suite (with `ENGRAM_REQUIRE_QDRANT=1`, real Qdrant via testcontainers) are all clean. Left unfixed per the deviation-rules scope boundary (only fix issues directly caused by this task's changes).

## User Setup Required

None - no external service configuration required. Operators who want to enable the service-auth lanes set `ENGRAM_SERVICE_AUTH_OIDC_ISSUER`/`_OIDC_AUDIENCE`/`_OWNER_CLAIMS` and/or `ENGRAM_SERVICE_AUTH_STATIC_TOKENS` per `reference/auth.md`'s new "Service principals" section — no code change or migration required, and a deployment with none of these set is unchanged.

## Next Phase Readiness

- Phase 23 (Service Auth Chain & Tenancy Isolation) is complete: the auth chain is live at the one production call site, tenancy isolation and the global shared-read decision are both proven and documented, and `REQ-service-auth-chain`/`REQ-service-principal-isolation` are satisfied.
- Deferred/out-of-scope for a future milestone (per this plan's ADR and `23-CONTEXT.md`): per-tenant `shared`-read scoping (full ABAC), service auth on the Connect write lane (MCP-first per REQUIREMENTS.md), SPIFFE/SPIRE workload identity, bcrypt/argon2 hashing of static tokens at rest.
- No blockers for the next phase.

---
*Phase: 23-service-auth-chain-tenancy-isolation*
*Completed: 2026-07-17*

## Self-Check: PASSED

All key files confirmed present on disk; all three task commits (`9c459033`, `13245a73`, `19f970df`) confirmed present in git history.
