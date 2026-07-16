---
phase: 16-csrf-interceptor
verified: 2026-07-12T01:55:30Z
status: passed
score: 9/9 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 16: CSRF Interceptor Verification Report

**Phase Goal:** The write lane's primary defense against cross-site request forgery exists in the Connect interceptor chain before any write RPC does real work.
**Verified:** 2026-07-12T01:55:30Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| SC1 | Every state-changing Connect RPC rejects a cross-origin caller via Go 1.26 `net/http.CrossOriginProtection` as the primary defense | VERIFIED | `cmd/engram/csrf.go` `newCrossOriginProtection()` builds `http.NewCrossOriginProtection()`; `cmd/engram/serve.go:214` sets `httpSrv.Handler = newCrossOriginProtection().Handler(mux)` — wraps the fully-assembled top-level mux (after all `mux.Handle` registrations). `TestCrossOriginProtectionRejectsCrossOrigin` (cmd/engram/csrf_test.go:18) passes. |
| SC2 | A session-bound double-submit CSRF token (HMAC over session identity, non-HttpOnly cookie, echoed as header) is required and validated on every write RPC before handler logic runs; never a bare random value; never checked without reference to the resolved Subject | VERIFIED | `internal/webauth/csrf.go`: `CSRFSigner.Token(owner)` = `base64(HMAC-SHA256(k_csrf, owner))` — deterministic, not random. `internal/server/connectcsrf.go` `newConnectCSRFInterceptor` calls `subjectFromConnectContext(ctx)` to obtain the resolved Subject/Owner and calls `verify(subj.Owner(), c.Value)` before comparing to the header. `TestConnectCSRFTokenMatrix` (4 sub-cases) passes. Cookie minted non-HttpOnly (`internal/webauth/handlers.go` `setReadableCookie`, `HttpOnly: false, Secure: h.secure, SameSite: Lax`). |
| SC3 | The five read RPCs are provably unaffected — no CSRF header required, verified by a regression test enumerating each read RPC against the write-only allowlist | VERIFIED | `csrfWriteProcedures` (connectcsrf.go) contains exactly the 6 write Procedure constants; `TestCSRFWriteProcedureAllowlist` asserts len==6 and asserts none of the 5 read Procedures are present. `TestReadRPCsCSRFExempt` drives all 5 read RPCs with no CSRF header over the real interceptor chain and asserts none returns PermissionDenied. Both tests pass. |
| SC4 | `TestConnectNoCORSHeaders` remains green — no `Access-Control-Allow-Origin` ever emitted from the Connect mux — permanent CI gate | VERIFIED | `internal/server/connectapi_cookie_test.go:98` `TestConnectNoCORSHeaders` passes (`go test -run TestConnectNoCORSHeaders ./internal/server/...`). |

### Observable Truths (PLAN.md must_haves, D-decisions)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| D-02 | CSRF interceptor sits between subject and validate | VERIFIED | `internal/server/connectapi.go:262-267`: `connect.WithInterceptors(otelIc, accessLog, subject, newConnectCSRFInterceptor(csrfVerify), validate)`. |
| D-03 | CSRF failure returns `connect.CodePermissionDenied` | VERIFIED | All three rejection paths in `newConnectCSRFInterceptor` return `connect.NewError(connect.CodePermissionDenied, ...)`. |
| D-04 | `CrossOriginProtection` deny handler is normalized — Connect-shaped `permission_denied`/403, fixed generic message (not raw error text) | VERIFIED | `cmd/engram/csrf.go` `SetDenyHandler` writes `Content-Type: application/json`, `403`, body `{"code":"permission_denied","message":"cross-origin request rejected"}` — a fixed string, never `err.Error()`. `TestCrossOriginDenyHandlerEnvelope` passes. |
| D-05 | Interceptor independently fails closed on empty/absent Owner | VERIFIED | `newConnectCSRFInterceptor` re-reads Subject via `subjectFromConnectContext` and rejects if `err != nil \|\| subj.Owner() == ""`, before checking the cookie. `TestConnectCSRFInterceptor_EmptyOwner` (nil-TokenInfo resolver, well-formed cookie/header) passes with PermissionDenied. |
| D-06 | Permanent no-anonymous-write regression test across all 6 write RPCs, never `Unimplemented` | VERIFIED | `TestNoAnonymousWrite` enumerates StoreMemory/StoreDiscovery/UpdateMemory/DeleteMemory/SetVisibility/ScheduleMemory against a cookieless authenticated request, asserts `PermissionDenied` and explicitly asserts code is never `Unimplemented`. Passes. |
| D-07 | Write-only allowlist keys on generated Procedure constants; CrossOriginProtection wraps the whole server handler | VERIFIED | `csrfWriteProcedures` map keys are `engramv1connect.EngramService*Procedure` constants (generated). `httpSrv.Handler = newCrossOriginProtection().Handler(mux)` wraps everything registered on `mux` (Connect + `/auth/*` + `/ui/`). `TestCrossOriginProtectionAllowsSafeAndNoOrigin` confirms GET and no-Origin (MCP) pass through untouched. |
| D-08 | HMAC over Owner only (not Owner+Expiry); key is HKDF sub-key of `ui.cookie_key` with distinct info label | VERIFIED | `webauth.DeriveCSRFKey` uses `hkdf.Key(sha256.New, cookieKey, nil, "engram-csrf-v1", 32)` — distinct label from the session-seal derivation. `CSRFSigner.Token(owner)` takes only `owner`, never touches `Expiry`. `TestCSRFSigner_StableAcrossExpiry` and `TestDeriveCSRFKey_DeterministicAndDistinct` pass. |

**Score:** 9/9 truths verified (0 present-but-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/webauth/csrf.go` | HKDF sub-key derivation + HMAC-over-Owner double-submit signer | VERIFIED | Exists, substantive (71 lines, real crypto), wired (used by `webauth.NewHandler`, `serve.go`, `server.Register`) |
| `internal/webauth/csrf_test.go` | D-08 stability, tamper rejection, key guard tests | VERIFIED | 4 test functions, all pass |
| `internal/server/connectcsrf.go` | Connect write-only CSRF interceptor | VERIFIED | Exists, wired into `mountConnect`'s `WithInterceptors` chain at the correct position |
| `internal/server/connectcsrf_test.go` | D-06/SC2/SC3/D-05 regression matrix | VERIFIED | 5 test functions (`TestCSRFWriteProcedureAllowlist`, `TestNoAnonymousWrite`, `TestConnectCSRFTokenMatrix`, `TestReadRPCsCSRFExempt`, `TestConnectCSRFInterceptor_EmptyOwner`), all pass |
| `cmd/engram/csrf.go` | CrossOriginProtection + Connect-shaped deny handler | VERIFIED | Exists, wired into `httpSrv.Handler` in serve.go |
| `cmd/engram/csrf_test.go` | SC1/D-04/cross-package wire-name-match tests | VERIFIED | 4 test functions, all pass, including `TestCSRFWireNamesMatch` confirming `server.CSRFCookieName == webauth.CSRFCookieName` and header-name equality |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `cmd/engram/serve.go` | `webauth.DeriveCSRFKey` / `webauth.NewCSRFSigner` | derives `k_csrf` from `ui.cookie_key`, builds signer (serve.go:143-147) | WIRED | Signer's `.Verify` passed as `csrfVerify` into `server.Register` (serve.go:166) |
| `server.Register` | `mountConnect` | `csrfVerify` threaded through, mirrors existing `connectResolver` plumbing | WIRED | `Register(... resolve connectResolver, csrfVerify func(owner, token string) bool)` at tools.go:1078; `mountConnect` signature matches |
| `mountConnect` | `newConnectCSRFInterceptor` | installed in `WithInterceptors` list between subject and validate | WIRED | connectapi.go:262-267 |
| `webauth.Handler.Callback` | CSRF cookie minting | `h.setReadableCookie(w, CSRFCookieName, h.signer.Token(owner), sessionTTL)` | WIRED | handlers.go:186, alongside existing session cookie mint; `TestCallbackMintsCSRFCookie` passes |
| `cmd/engram/serve.go` | `newCrossOriginProtection().Handler(mux)` | wraps `httpSrv.Handler` after all mux registrations | WIRED | serve.go:214 |

### Behavioral Spot-Checks / Test Execution

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Build | `go build ./...` | clean | PASS |
| Full test suite (single run) | `go test ./...` | all packages `ok` | PASS |
| CSRF-specific regression matrix | `go test ./internal/webauth/... ./internal/server/... ./cmd/engram/... -run 'CSRF\|CrossOrigin\|NoAnonymousWrite\|ReadRPCsCSRFExempt\|ConnectNoCORSHeaders' -v` | all 14 named tests PASS (see subtests) | PASS |
| Lint | `task lint:go` | `0 issues` | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| REQ-connect-csrf | 16-01, 16-02, 16-03 | CSRF-protect all state-changing Connect RPCs via CrossOriginProtection primary + double-submit token defense-in-depth; reads untouched; no permissive CORS | SATISFIED | All 4 roadmap SCs and all D-decisions verified above; REQUIREMENTS.md line 45 already marked `[x]`, consistent with actual codebase state |

No orphaned requirements: REQUIREMENTS.md maps only REQ-connect-csrf to Phase 16, and all 3 plans declare it.

### Anti-Patterns Found

None. Scanned all 11 phase-modified files (`internal/webauth/csrf.go`, `csrf_test.go`, `handlers.go`, `handlers_test.go`, `internal/server/connectcsrf.go`, `connectcsrf_test.go`, `connectapi.go`, `tools.go`, `cmd/engram/csrf.go`, `csrf_test.go`, `serve.go`) for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER`/"not yet implemented" — zero matches. No `err.Error()` leaked into wire messages (confirmed fixed generic strings in both D-03 and D-04 rejection paths). No `==` string comparison for token verification (confirmed `hmac.Equal` only).

### Human Verification Required

None. All must-haves are either static/deterministic crypto properties or exercised by passing automated regression tests running the real interceptor chain over `httptest.NewServer`.

### Gaps Summary

None. Phase goal fully achieved: `CrossOriginProtection` is the primary same-origin defense wrapping the whole server handler with a normalized deny envelope; the double-submit HMAC-over-Owner token is a second, independently fail-closed layer correctly positioned in the Connect interceptor chain, gated to exactly the 6 write RPCs via generated Procedure constants; the 5 read RPCs and `TestConnectNoCORSHeaders` are provably unaffected. All 8 locked decisions (D-01–D-08) are honored in source, not just claimed in SUMMARY.md.

---

_Verified: 2026-07-12T01:55:30Z_
_Verifier: Claude (gsd-verifier)_
