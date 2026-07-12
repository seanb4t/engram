---
phase: 16-csrf-interceptor
plan: 02
subsystem: auth
tags: [csrf, connect-interceptor, double-submit, hmac, connect-go]

# Dependency graph
requires:
  - phase: 16-csrf-interceptor
    plan: 01
    provides: webauth.CSRFSigner{Token,Verify}, webauth.DeriveCSRFKey, webauth.CSRFCookieName/CSRFHeaderName
  - phase: 15-additive-proto-stub-write-handlers
    provides: the six write RPCs + Connect interceptor chain (subject/validate) this CSRF layer slots into
provides:
  - server.newConnectCSRFInterceptor(verify func(owner, token string) bool) connect.UnaryInterceptorFunc
  - server.csrfWriteProcedures (write-only Procedure allowlist, exactly 6 entries)
  - server.CSRFCookieName / server.CSRFHeaderName (server-side wire-contract constants)
  - mountConnect/Register csrfVerify parameter threaded from serve.go's real webauth.CSRFSigner
affects: [16-03-csrf-cookie-issuance, 17-deps-refactor-wired-handlers, 18-session-rotation]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Write-only Connect interceptor gated on generated Procedure constants (never a hand-maintained path list)"
    - "Independent fail-closed re-check of resolved Subject inside a downstream interceptor (defense-in-depth against future ordering regressions)"

key-files:
  created:
    - internal/server/connectcsrf.go
    - internal/server/connectcsrf_test.go
  modified:
    - internal/server/connectapi.go
    - internal/server/tools.go
    - cmd/engram/serve.go
    - internal/server/connectapi_negative_test.go
    - internal/server/connectapi_cookie_test.go
    - internal/server/connectapi_test.go
    - internal/server/tools_test.go

key-decisions:
  - "TestWriteRPCNegativeMatrix's shared callWrite helper now auto-attaches a matching engram_csrf cookie + X-CSRF-Token header pair whenever an actor is set, using a stub csrfVerify that always returns true — keeps the pre-existing Unimplemented/Unauthenticated/InvalidArgument matrix green without special-casing CSRF in that file"
  - "TestReadRPCsCSRFExempt uses testDeps(t) (real Qdrant via testcontainers, skip-gated) rather than a bare &deps{} — the 5 read handlers dereference deps.st/deps.em directly (unlike the still-Unimplemented write stubs), so a nil-deps httptest call would risk a recovered-but-nondeterministic panic instead of a clean assertable code"
  - "The HMAC-over-Owner verify used by connectcsrf_test.go is a local inline replica (crypto/hmac+sha256 over a fixed test key), not an import of internal/webauth's CSRFSigner — internal/server does not depend on internal/webauth in production code, and the RESEARCH/PLAN explicitly call for this replication to preserve that layering"

requirements-completed: [REQ-connect-csrf]

coverage:
  - id: D-06
    description: "Each of the 6 write RPCs presented with no CSRF cookie/header (but a valid Subject) is rejected with CodePermissionDenied before the stub handler runs — never CodeUnimplemented"
    requirement: "REQ-connect-csrf"
    verification:
      - kind: unit
        ref: "internal/server/connectcsrf_test.go#TestNoAnonymousWrite"
        status: pass
    human_judgment: false
  - id: SC2
    description: "A write RPC with a valid engram_csrf cookie equal to X-CSRF-Token, both verifying for the resolved Owner, passes CSRF and reaches the still-Unimplemented stub; missing header, mismatched cookie/header, and cross-owner cookie are all rejected"
    requirement: "REQ-connect-csrf"
    verification:
      - kind: unit
        ref: "internal/server/connectcsrf_test.go#TestConnectCSRFTokenMatrix"
        status: pass
    human_judgment: false
  - id: D-05
    description: "newConnectCSRFInterceptor independently returns CodePermissionDenied when the resolved Subject has empty Owner, without relying on the subject interceptor to reject first"
    requirement: "REQ-connect-csrf"
    verification:
      - kind: unit
        ref: "internal/server/connectcsrf_test.go#TestConnectCSRFInterceptor_EmptyOwner"
        status: pass
    human_judgment: false
  - id: SC3
    description: "csrfWriteProcedures contains exactly the 6 write Procedure constants and none of the 5 read Procedure constants; the 5 read RPCs are never rejected with PermissionDenied end-to-end"
    requirement: "REQ-connect-csrf"
    verification:
      - kind: unit
        ref: "internal/server/connectcsrf_test.go#TestCSRFWriteProcedureAllowlist"
        status: pass
      - kind: integration
        ref: "internal/server/connectcsrf_test.go#TestReadRPCsCSRFExempt"
        status: pass
    human_judgment: false
  - id: D-02
    description: "The CSRF interceptor is installed in mountConnect between newConnectSubjectInterceptor and newConnectValidateInterceptor"
    requirement: "REQ-connect-csrf"
    verification:
      - kind: unit
        ref: "internal/server/connectapi.go (connect.WithInterceptors order) + internal/server/connectcsrf_test.go#TestConnectCSRFTokenMatrix (happy-path cell proves the ordering end-to-end)"
        status: pass
    human_judgment: false
  - id: SC4
    description: "TestConnectNoCORSHeaders stays green after the mountConnect signature change"
    requirement: "REQ-connect-csrf"
    verification:
      - kind: regression
        ref: "internal/server/connectapi_cookie_test.go#TestConnectNoCORSHeaders"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-07-12
status: complete
---

# Phase 16 Plan 02: Connect CSRF Interceptor Wiring Summary

**Write-only Connect interceptor enforcing a session-bound HMAC double-submit token (CodePermissionDenied) between the subject and validate interceptors, threaded from serve.go's real webauth.CSRFSigner through Register/mountConnect, plus a permanent regression matrix proving D-05/D-06/SC2/SC3.**

## Performance

- **Duration:** ~25 min
- **Completed:** 2026-07-12T01:38:18Z
- **Tasks:** 3
- **Files modified:** 9 (2 new, 7 modified)

## Accomplishments

- `newConnectCSRFInterceptor` (internal/server/connectcsrf.go): gates on the 6 generated write Procedure constants (`csrfWriteProcedures`, D-07), independently re-reads the resolved Subject and fails closed on an empty Owner (D-05), reads the `engram_csrf` cookie via the sanctioned `&http.Request{Header: req.Header()}` idiom, and rejects a missing/mismatched double-submit token — every rejection path maps to `CodePermissionDenied` (D-03) with a fixed generic message, never leaking `err.Error()` detail.
- `mountConnect` installs the interceptor between `newConnectSubjectInterceptor` and `newConnectValidateInterceptor` (D-02); `Register` and `serve.go` thread a `csrfVerify func(owner, token string) bool` parameter down from a real `webauth.CSRFSigner` derived via `webauth.DeriveCSRFKey(key)` — only built when the UI/Connect lane is enabled.
- All 6 pre-existing `mountConnect`/`Register` call sites (5 test, 1 production) updated to the new signature; `go build ./...` is clean and `TestWriteRPCNegativeMatrix` / `TestConnectNoCORSHeaders` / `TestConnectCookieLaneIsolation` all stay green.
- `internal/server/connectcsrf_test.go` adds five permanent regression tests: `TestCSRFWriteProcedureAllowlist` (data-level exact-6 + no-reads assertion), `TestNoAnonymousWrite` (D-06, all 6 write RPCs, cookieless → PermissionDenied, never Unimplemented), `TestConnectCSRFTokenMatrix` (SC2, 4-cell matrix: matching pair → Unimplemented; missing header / mismatched pair / cross-owner cookie → PermissionDenied), `TestReadRPCsCSRFExempt` (SC3, all 5 read RPCs, no CSRF header, never PermissionDenied), `TestConnectCSRFInterceptor_EmptyOwner` (D-05, anonymous-Subject write rejected even with a well-formed token pair).

## Task Commits

1. **Task 1: Create the write-only CSRF token interceptor factory** - `99621795` (feat)
2. **Task 2: Thread csrfVerify through mountConnect + Register + serve.go and fix all call sites** - `7fb960fb` (feat)
3. **Task 3: Regression matrix (D-05/D-06/SC2/SC3/allowlist)** - `96d236a0` (test)

**Plan metadata:** (this commit, docs: complete plan)

## Files Created/Modified

- `internal/server/connectcsrf.go` - `newConnectCSRFInterceptor`, `csrfWriteProcedures`, `CSRFCookieName`/`CSRFHeaderName`
- `internal/server/connectcsrf_test.go` - D-05/D-06/SC2/SC3/allowlist permanent regression tests
- `internal/server/connectapi.go` - `mountConnect` gains `csrfVerify` param; interceptor inserted at D-02 position
- `internal/server/tools.go` - `Register` gains `csrfVerify` param, forwards to `mountConnect`
- `cmd/engram/serve.go` - derives `k_csrf` via `webauth.DeriveCSRFKey`, builds `csrfSigner`, wires `connectCSRFVerify = csrfSigner.Verify` into `server.Register`
- `internal/server/connectapi_negative_test.go` - `callWrite` now attaches a matching CSRF cookie/header pair for authenticated calls (stub `csrfVerify` always true)
- `internal/server/connectapi_cookie_test.go`, `connectapi_test.go`, `tools_test.go` - updated to the new `mountConnect`/`Register` signatures (`nil` csrfVerify where no write RPC is exercised)

## Decisions Made

- `TestWriteRPCNegativeMatrix`'s `callWrite` helper auto-attaches a matching `engram_csrf` cookie + `X-CSRF-Token` header whenever an actor is set, backed by a stub `csrfVerify` that always returns `true` — this keeps the file's pre-existing Unimplemented/Unauthenticated/InvalidArgument assertions valid without threading CSRF-specific logic into that matrix; CSRF-specific rejection behavior is exercised exclusively in `connectcsrf_test.go`.
- `TestReadRPCsCSRFExempt` uses `testDeps(t)` (real Qdrant via testcontainers, skip-gated) instead of a bare `&deps{}`, because the 5 read handlers dereference `deps.st`/`deps.em` directly — unlike the still-`Unimplemented` write stubs, a nil-deps call would risk an httptest-recovered-but-nondeterministic panic instead of a clean, assertable Connect code.
- `connectcsrf_test.go`'s token-matrix verify function is a local inline replica of `webauth.CSRFSigner.Verify` (crypto/hmac+sha256 over a fixed test key) rather than importing `internal/webauth` — matches the RESEARCH/PLAN's explicit instruction to preserve the layering boundary (`internal/server` does not depend on `internal/webauth` in production code today).

## Deviations from Plan

None functionally — plan executed as written. One minor grep-count note: the plan's acceptance criterion `rg -c "EngramService(ListScopes|ListMemories|SearchMemories|GetMemory|SearchDiscoveries)Procedure" internal/server/connectcsrf_test.go` expected exactly 5; the actual file returns 10 because the five read-Procedure constants are legitimately referenced twice — once in `TestCSRFWriteProcedureAllowlist`'s negative-membership check and once in `TestReadRPCsCSRFExempt`'s live-call table — which is a stricter test than a single reference, not a gap.

## Issues Encountered

`task lint:go`, `task license:check`, and `task test` (Go + Python) all green. `task lint:markdown` fails on ~900 pre-existing issues across `.planning/` files unrelated to this plan's changes (same systemic `.rumdl.toml` `.planning`-exclude gap already tracked in STATE.md/16-01-SUMMARY.md as tech debt for Phase 21) — out of scope per the executor's scope-boundary rule, not touched here.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

`internal/server/connectcsrf.go` and the threaded `csrfVerify` seam are ready for plan 03 (cookie issuance: `webauth.Handler.Callback` minting the `engram_csrf` cookie alongside the session cookie via `CSRFSigner.Token`, and the cross-package constant-equality test asserting `server.CSRFCookieName`/`CSRFHeaderName` match `webauth.CSRFCookieName`/`CSRFHeaderName` byte-for-byte). No blockers.

---
*Phase: 16-csrf-interceptor*
*Completed: 2026-07-12*

## Self-Check: PASSED

- FOUND: internal/server/connectcsrf.go
- FOUND: internal/server/connectcsrf_test.go
- FOUND: .planning/phases/16-csrf-interceptor/16-02-SUMMARY.md
- FOUND: 99621795
- FOUND: 7fb960fb
- FOUND: 96d236a0
