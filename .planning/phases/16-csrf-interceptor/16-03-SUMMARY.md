---
phase: 16-csrf-interceptor
plan: 03
subsystem: auth
tags: [csrf, cross-origin-protection, webauth, cookie-issuance, go-stdlib-crypto]

# Dependency graph
requires:
  - phase: 16-csrf-interceptor
    plan: 01
    provides: webauth.CSRFSigner{Token,Verify}, webauth.CSRFCookieName/CSRFHeaderName
  - phase: 16-csrf-interceptor
    plan: 02
    provides: server.newConnectCSRFInterceptor, csrfSigner threaded into serve.go, server.CSRFCookieName/CSRFHeaderName
provides:
  - cmd/engram/newCrossOriginProtection() — the HTTP-layer primary CSRF defense (D-01 layer 1), wrapping the whole assembled mux
  - webauth.Handler.Callback mints the engram_csrf cookie (non-HttpOnly, Secure, SameSite=Lax) alongside the session cookie
  - webauth.NewHandler(auth, codec, secure, signer) — signer is now a required, fail-fast dependency
affects: [17-deps-refactor-wired-handlers, 18-session-rotation, 19-console-write-ux]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Whole-server http.Handler wrap with a normalized SetDenyHandler emitting the same Connect wire-error envelope as the interceptor-layer rejection (D-04)"
    - "Non-HttpOnly Secure SameSite=Lax cookie minted alongside an HttpOnly session cookie, deliberately readable by same-origin JS for double-submit echo (SC2)"

key-files:
  created:
    - cmd/engram/csrf.go
    - cmd/engram/csrf_test.go
  modified:
    - cmd/engram/serve.go
    - internal/webauth/handlers.go
    - internal/webauth/handlers_test.go

key-decisions:
  - "Minted the CSRF cookie now (in Callback), not deferred to Phase 19, per RESEARCH.md's resolved Open Question 1 / Assumption A1 — SC2 is live end-to-end this phase, not just synthetically testable"
  - "Reworded the newCrossOriginProtection doc comment to avoid literally naming AddTrustedOrigin/AddInsecureBypassPattern, so the plan's negative grep acceptance criterion (no trusted-origin exemption registered) matches cleanly against the file"
  - "Added a setReadableCookie helper on webauth.Handler mirroring setCookie's shape but HttpOnly:false, rather than inlining the Set-Cookie call in Callback — keeps the non-HttpOnly rationale documented in one place"

requirements-completed: [REQ-connect-csrf]

coverage:
  - id: SC1
    description: "A cross-origin unsafe-method (POST) request to the wrapped top-level handler is rejected with HTTP 403 and never reaches the inner handler"
    requirement: "REQ-connect-csrf"
    verification:
      - kind: unit
        ref: "cmd/engram/csrf_test.go#TestCrossOriginProtectionRejectsCrossOrigin"
        status: pass
    human_judgment: false
  - id: D-07
    description: "A safe-method GET and a request with neither Origin nor Sec-Fetch-Site (the MCP transport's shape) pass the wrapper untouched"
    requirement: "REQ-connect-csrf"
    verification:
      - kind: unit
        ref: "cmd/engram/csrf_test.go#TestCrossOriginProtectionAllowsSafeAndNoOrigin"
        status: pass
    human_judgment: false
  - id: D-04
    description: "The CrossOriginProtection deny response has Content-Type application/json and a body deserializing to {code: permission_denied, message: <fixed generic string>}"
    requirement: "REQ-connect-csrf"
    verification:
      - kind: unit
        ref: "cmd/engram/csrf_test.go#TestCrossOriginDenyHandlerEnvelope"
        status: pass
    human_judgment: false
  - id: T-16-10
    description: "server.CSRFCookieName/CSRFHeaderName agree byte-for-byte with webauth.CSRFCookieName/CSRFHeaderName (no cross-package wire-contract drift)"
    requirement: "REQ-connect-csrf"
    verification:
      - kind: unit
        ref: "cmd/engram/csrf_test.go#TestCSRFWireNamesMatch"
        status: pass
    human_judgment: false
  - id: SC2
    description: "webauth.Handler.Callback issues a non-HttpOnly, Secure, SameSite=Lax cookie named engram_csrf whose value is CSRFSigner.Token(owner), alongside the existing session cookie"
    requirement: "REQ-connect-csrf"
    verification:
      - kind: integration
        ref: "internal/webauth/handlers_test.go#TestCallbackMintsCSRFCookie"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-07-12
status: complete
---

# Phase 16 Plan 03: CSRF HTTP-Layer Wrap + Cookie Issuance Summary

**Go 1.26 stdlib CrossOriginProtection wraps the whole assembled mux with a Connect-shaped permission_denied/403 deny handler, and webauth.Handler.Callback now mints the non-HttpOnly engram_csrf double-submit cookie end-to-end this phase.**

## Performance

- **Duration:** ~20 min
- **Completed:** 2026-07-12
- **Tasks:** 3
- **Files modified:** 5 (2 new, 3 modified)

## Accomplishments

- `newCrossOriginProtection` (`cmd/engram/csrf.go`): constructs `http.NewCrossOriginProtection()` with a custom `SetDenyHandler` emitting the same `{"code":"permission_denied","message":"..."}` + `Content-Type: application/json` + HTTP 403 envelope the Connect interceptor's D-03 rejection uses (D-04) — the message is a fixed generic string, never the underlying cross-origin error text (T-16-05). No `AddTrustedOrigin`/`AddInsecureBypassPattern` registered — strict same-origin default.
- `cmd/engram/serve.go`'s `httpSrv.Handler` is now `newCrossOriginProtection().Handler(mux)`, set after every route registration (D-07 whole-server wrap, verified safe against the Go 1.26.5 stdlib source per RESEARCH.md): covers the Connect handler, `/auth/*`, `/ui/`, and the MCP transport in one place.
- `webauth.NewHandler` gains a required `*CSRFSigner` parameter (fail-fast panic on nil, matching the existing auth/codec guards); `Handler.Callback` mints the `engram_csrf` cookie via a new `setReadableCookie` helper (non-HttpOnly, Secure, SameSite=Lax) immediately after sealing the session cookie, valued `CSRFSigner.Token(owner)` (D-08 Owner-only binding, survives the Phase-18 sliding re-seal). `cmd/engram/serve.go` passes the same `csrfSigner` instance whose `Verify` feeds the Connect CSRF interceptor — issuance and verification share one key.
- Four new `cmd/engram/csrf_test.go` tests (SC1 cross-origin rejection before the inner handler runs; D-07 safe-method/no-Origin pass-through; D-04 envelope shape; wire-name cross-package equality) and one new `internal/webauth/handlers_test.go` test (`TestCallbackMintsCSRFCookie`, driving a full fake-OIDC Callback and asserting the Set-Cookie shape/value) prove every `must_haves.truths` claim.

## Task Commits

1. **Task 1: CrossOriginProtection with a Connect-shaped deny handler, wrapping the whole mux (D-07, D-04)** - `560aaa03` (feat)
2. **Task 2: Mint the non-HttpOnly engram_csrf cookie in Callback and wire the signer through NewHandler (D-08, SC2)** - `42e0f664` (feat)
3. **Task 3: Tests — SC1 wrap rejects cross-origin, D-04 envelope, cross-package name equality, cookie-mint shape** - `48b97dc2` (test)

**Plan metadata:** (this commit, docs: complete plan)

## Files Created/Modified

- `cmd/engram/csrf.go` - `newCrossOriginProtection`, the Connect-shaped deny handler
- `cmd/engram/csrf_test.go` - SC1/D-07/D-04/wire-name regression tests
- `cmd/engram/serve.go` - `httpSrv.Handler` wraps `mux` with `newCrossOriginProtection().Handler(...)`; `webHandler = webauth.NewHandler(authr, codec, true, csrfSigner)`
- `internal/webauth/handlers.go` - `NewHandler` gains a `signer *CSRFSigner` param + nil guard; `Handler.signer` field; new `setReadableCookie` helper; `Callback` mints the `engram_csrf` cookie
- `internal/webauth/handlers_test.go` - `testSigner` helper; `NewHandler` call sites updated to the new signature (including a `nil signer` panic case); `TestCallbackMintsCSRFCookie`

## Decisions Made

- Minted the CSRF cookie now (in `Callback`), not deferred to Phase 19 — RESEARCH.md's Open Question 1 / Pitfall 5 explicitly recommended this, and CONTEXT.md's "issuance timing lands with the Phase-19 client" wording refers to the client-side ATTACH + silent-retry behavior, not cookie minting. This makes SC2 live end-to-end this phase via a real fake-OIDC Callback test, not just a synthetic-cookie unit test.
- Reworded `newCrossOriginProtection`'s doc comment to avoid literally spelling `AddTrustedOrigin`/`AddInsecureBypassPattern` — the plan's acceptance criterion greps for those exact identifiers and expects zero matches (proving no exemption is registered); a doc comment mentioning them by name would have false-positived that check even though no call is actually made. Reworded to "no trusted-origin or bypass-pattern exemption is registered" — same meaning, doesn't collide with the grep.
- Added a small `setReadableCookie` helper on `webauth.Handler` rather than inlining a second `http.SetCookie` call directly in `Callback` — keeps the non-HttpOnly rationale (SC2: same-origin JS must read it to echo the header) documented in exactly one place, mirroring `setCookie`'s existing shape.

## Deviations from Plan

None functionally — plan executed as written. One cosmetic adjustment: the plan's example doc comment for `newCrossOriginProtection` (in `<action>`) mentioned "Do NOT register any AddTrustedOrigin/AddInsecureBypassPattern" as prose guidance; the acceptance criterion greps the *file* for those literal strings and expects none, so the shipped doc comment paraphrases instead of naming the APIs — same intent, passes the grep.

## Issues Encountered

`task lint:go`, `task license:check`, and `task test` (Go + Python, full suite) all green. `task lint:markdown` fails on ~900 pre-existing issues across `.planning/` files unrelated to this plan's changes — the same systemic `.rumdl.toml` `.planning`-exclude gap already tracked in STATE.md and 16-01/16-02-SUMMARY.md as tech debt for Phase 21. Out of scope per the executor's scope-boundary rule; not touched here.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Phase 16's full CSRF mechanism (primary same-origin defense + double-submit token + live cookie issuance) is now installed end-to-end against the still-`Unimplemented` write stubs from Phase 15. Phase 17 (write-handler business logic + authz parity) can build directly on this transport-layer CSRF gate without further wiring. No blockers.

---
*Phase: 16-csrf-interceptor*
*Completed: 2026-07-12*

## Self-Check: PASSED

- FOUND: cmd/engram/csrf.go
- FOUND: cmd/engram/csrf_test.go
- FOUND: .planning/phases/16-csrf-interceptor/16-03-SUMMARY.md
- FOUND: 560aaa03
- FOUND: 42e0f664
- FOUND: 48b97dc2
