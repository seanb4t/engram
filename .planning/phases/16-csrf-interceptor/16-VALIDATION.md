---
phase: 16
slug: csrf-interceptor
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-11
---

# Phase 16 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `16-RESEARCH.md` § Validation Architecture. Task IDs are filled
> in by the planner; behaviors/commands below are the fixed test contract.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (`go test` / `task test`) |
| **Config file** | none — plain `go test ./...`; `internal/server`, `internal/webauth`, `cmd/engram` are the touched packages |
| **Quick run command** | `go test ./internal/server/... ./internal/webauth/... -run 'CSRF|CrossOrigin|NoAnonymousWrite|ReadRPCsCSRFExempt' -v` |
| **Full suite command** | `task` (lint + test, full repo) |
| **Estimated runtime** | ~30 seconds (targeted server + webauth packages) |

---

## Sampling Rate

- **After every task commit:** Run the targeted `-run` subset covering the just-touched behavior (quick run command, scoped narrower per task)
- **After every plan wave:** Run `go test ./internal/server/... ./internal/webauth/... ./cmd/engram/...`
- **Before `/gsd-verify-work`:** `task` (full lint + test) must be green
- **Max feedback latency:** ~30 seconds

---

## Per-Task Verification Map

*Task IDs assigned by the planner; rows are keyed by Success Criterion / Decision until then.*

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | — | — | SC1 / REQ-connect-csrf | T-16 CSRF | Cross-origin `Origin`/`Sec-Fetch-Site` write request rejected with 403 before Connect parses body | integration (httptest) | `go test ./cmd/engram/... -run TestServeCrossOrigin -v` | ❌ W0 | ⬜ pending |
| TBD | — | — | SC2 / REQ-connect-csrf | T-16 CSRF | Write RPC w/ valid session but missing/mismatched CSRF cookie+header → `PermissionDenied`; matching → passes CSRF layer | integration (real interceptor chain) | `go test ./internal/server/... -run TestConnectCSRFTokenMatrix -v` | ❌ W0 | ⬜ pending |
| TBD | — | — | SC3 / REQ-connect-csrf | — | Each of 5 read Procedures accepts a request with NO `X-CSRF-Token` and is not rejected by CSRF layer (assert NOT `PermissionDenied`) | integration table | `go test ./internal/server/... -run TestReadRPCsCSRFExempt -v` | ❌ W0 | ⬜ pending |
| TBD | — | — | SC4 / REQ-connect-csrf | T-16 CORS | No `Access-Control-Allow-Origin` ever emitted from Connect mux after CSRF wiring | regression (existing) | `go test ./internal/server/... -run TestConnectNoCORSHeaders -v` | ✅ `connectapi_cookie_test.go:96` | ⬜ pending |
| TBD | — | — | D-04 | T-16 InfoDisc | Rejected cross-origin response body is `{"code":"permission_denied","message":"..."}`, `Content-Type: application/json` | unit/integration | `go test ./cmd/engram/... -run TestCrossOriginDenyHandlerEnvelope -v` | ❌ W0 | ⬜ pending |
| TBD | — | — | D-05 | T-16 CSRF | Invoking `newConnectCSRFInterceptor` with empty-owner Subject rejects `PermissionDenied`, independent of subject interceptor | unit (interceptor direct) | `go test ./internal/server/... -run TestConnectCSRFInterceptor_EmptyOwner -v` | ❌ W0 | ⬜ pending |
| TBD | — | — | D-06 | T-16 CSRF | All 6 write RPCs, cookieless/empty-owner → rejected before handler (assert `Unimplemented` is NEVER observed) | regression table | `go test ./internal/server/... -run TestNoAnonymousWrite -v` | ❌ W0 | ⬜ pending |
| TBD | — | — | D-08 | T-16 Replay | `CSRFSigner.Token(owner)` identical across two `Session`s differing only in `Expiry` | unit | `go test ./internal/webauth/... -run TestCSRFSigner_StableAcrossExpiry -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/server/connectcsrf_test.go` — new file; SC2, D-05, D-06 (interceptor unit + integration, extends the `writeRPCCase`/`callWrite[Req,Resp]` harness in `connectapi_negative_test.go`)
- [ ] `internal/server/connectapi_negative_test.go` (or sibling) — extend with SC3's read-RPC CSRF-exemption table
- [ ] `internal/webauth/csrf_test.go` — new file; D-08 (HMAC stability) + tamper rejection
- [ ] `cmd/engram/serve_test.go` (or existing wiring test) — SC1 (whole-server wrap) + D-04 (deny-handler envelope). **Verify first** whether `cmd/engram` tests `serve.go` wiring today; if not, extract handler-wiring into a testable helper (mirror `mountMCPRoutes`/`resolveMCPPath` in `mcproute.go`) rather than testing inside `runServe`/`main`
- Framework install: none — stdlib `testing` already present

---

## Manual-Only Verifications

*All phase behaviors have automated verification.* (CSRF is a transport-layer mechanism fully exercisable via `httptest` + direct interceptor invocation; no browser/manual step required this phase — the client that attaches the token is Phase 19.)

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
