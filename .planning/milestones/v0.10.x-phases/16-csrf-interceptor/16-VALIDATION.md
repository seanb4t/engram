---
phase: 16
slug: csrf-interceptor
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-07-11
validated: 2026-07-16
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
| 16-SC1 | 16 | 1 | SC1 / REQ-connect-csrf | T-16 CSRF | Cross-origin `Origin`/`Sec-Fetch-Site` write request rejected before Connect parses body | integration (httptest) | `go test ./cmd/engram/ -run TestCrossOriginProtectionRejectsCrossOrigin -v` | ✅ `cmd/engram/csrf_test.go:18` (renamed from planned `TestServeCrossOrigin`; `…AllowsSafeAndNoOrigin` covers the allow case) | ✅ green |
| 16-SC2 | 16 | 1 | SC2 / REQ-connect-csrf | T-16 CSRF | Write RPC w/ valid session but missing/mismatched CSRF cookie+header → `PermissionDenied`; matching → passes CSRF layer | integration (real interceptor chain) | `go test ./internal/server/ -run TestConnectCSRFTokenMatrix -v` | ✅ `connectcsrf_test.go:227` | ✅ green |
| 16-SC3 | 16 | 1 | SC3 / REQ-connect-csrf | — | Each read Procedure accepts a request with NO `X-CSRF-Token` and is not rejected by CSRF layer | integration table | `go test ./internal/server/ -run TestReadRPCsCSRFExempt -v` | ✅ `connectcsrf_test.go:302` | ✅ green |
| 16-SC4 | 16 | 1 | SC4 / REQ-connect-csrf | T-16 CORS | No `Access-Control-Allow-Origin` ever emitted from Connect mux after CSRF wiring | regression | `go test ./internal/server/ -run TestConnectNoCORSHeaders -v` | ✅ `connectapi_cookie_test.go:98` | ✅ green |
| 16-D04 | 16 | 1 | D-04 | T-16 InfoDisc | Rejected cross-origin response body is a JSON `permission_denied` envelope | unit/integration | `go test ./cmd/engram/ -run TestCrossOriginDenyHandlerEnvelope -v` | ✅ `cmd/engram/csrf_test.go:70` | ✅ green |
| 16-D05 | 16 | 1 | D-05 | T-16 CSRF | `newConnectCSRFInterceptor` with empty-owner Subject rejects `PermissionDenied`, independent of subject interceptor | unit (interceptor direct) | `go test ./internal/server/ -run TestConnectCSRFInterceptor_EmptyOwner -v` | ✅ `connectcsrf_test.go:389` | ✅ green |
| 16-D06 | 16 | 1 | D-06 | T-16 CSRF | All write RPCs, cookieless/empty-owner → rejected before handler (`Unimplemented` never observed) | regression table | `go test ./internal/server/ -run TestNoAnonymousWrite -v` | ✅ `connectcsrf_test.go:193` | ✅ green |
| 16-D08 | 16 | 1 | D-08 | T-16 Replay | `CSRFSigner.Token(owner)` identical across two `Session`s differing only in `Expiry` | unit | `go test ./internal/webauth/ -run TestCSRFSigner_StableAcrossExpiry -v` | ✅ `webauth/csrf_test.go:30` | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

**Extra coverage found beyond the strategy** (all green): `TestCSRFWriteProcedureAllowlist`, `TestCSRFWireNamesMatch`, `TestCSRFSigner_TamperRejected`, `TestDeriveCSRFKey_DeterministicAndDistinct`, `TestNewCSRFSigner_KeyGuard`, `TestResealPastThresholdRefreshesCSRFCookie`, `TestCallbackMintsCSRFCookie`.

---

## Wave 0 Requirements

*All Wave-0 test files were delivered during execution and are green — retroactively confirmed 2026-07-16:*

- [x] `internal/server/connectcsrf_test.go` — SC2, SC3, D-05, D-06 (interceptor unit + integration) ✅
- [x] `internal/webauth/csrf_test.go` — D-08 (HMAC stability) + tamper rejection ✅
- [x] `cmd/engram/csrf_test.go` — SC1 (`TestCrossOriginProtectionRejectsCrossOrigin`) + D-04 (deny-handler envelope) ✅
- [x] SC4 covered by the existing `internal/server/connectapi_cookie_test.go` (`TestConnectNoCORSHeaders`) ✅
- Framework install: none — stdlib `testing` already present

---

## Manual-Only Verifications

*All phase behaviors have automated verification.* (CSRF is a transport-layer mechanism fully exercisable via `httptest` + direct interceptor invocation; no browser/manual step required this phase — the client that attaches the token is Phase 19.)

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (all delivered — retroactively confirmed)
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-07-16

---

## Validation Audit 2026-07-16

Retroactive Nyquist audit (State A) during v0.10.x milestone close. The strategy was authored
but its `draft`/`pending` frontmatter was never reconciled after execution.

| Metric | Count |
|--------|-------|
| Strategy behaviors (SC1–4, D-04/05/06/08) | 8 |
| COVERED (real passing test) | 8 |
| MISSING | 0 |
| Gaps generated this pass | 0 |

**No gaps — no auditor spawn, no new tests generated.** All 8 strategy behaviors already had real,
passing tests (7 by exact name; SC1 under the renamed `TestCrossOriginProtectionRejectsCrossOrigin`),
plus 7 extra CSRF tests beyond the strategy. Ran the full set 2026-07-16 → all green
(`internal/server`, `internal/webauth`, `cmd/engram`). Flipped `nyquist_compliant: false → true`
(reconciliation only).
