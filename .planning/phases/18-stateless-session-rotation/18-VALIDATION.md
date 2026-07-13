---
phase: 18
slug: stateless-session-rotation
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-07-13
validated: 2026-07-13
---

# Phase 18 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Audited State A on 2026-07-13: REQ-session-rotation is fully covered by automated
> Go tests (all green); the only manual-only item is the live-browser re-seal
> persistence (SC1) and the ADR doc (SC2). No code-test gaps → `nyquist_compliant: true`.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (stdlib) |
| **Config file** | none — `Taskfile.yaml` orchestrates |
| **Quick run command** | `go test ./internal/webauth/... ./internal/server/... -count=1` |
| **Full suite command** | `task` (lint + test) |
| **Estimated runtime** | ~15s (webauth) + ~12s (server); no live Qdrant needed — pure cookie/interceptor logic |

---

## Sampling Rate

- **After every task commit:** quick run command for the touched package.
- **After every plan wave:** `task` (lint + test).
- **Before `/gsd-verify-work`:** full suite green (`task` clean).
- **Max feedback latency:** ~30 seconds.

---

## Per-Requirement / Success-Criterion Verification Map

REQ-session-rotation (the sole phase requirement) → 4 success criteria + the interceptor
contract + the WR-01 code-review hardening. All automated checks green as of 2026-07-13.

| SC / Concern | Requirement | Automated Test(s) | Type | Command | Status |
|--------------|-------------|-------------------|------|---------|--------|
| **SC1** threshold re-seal (past ½-TTL+skew) | REQ-session-rotation | `TestResealNoopBeforeThreshold`, `TestResealPastThresholdRefreshesSessionCookie` (`internal/webauth/reseal_test.go`) | unit | `go test ./internal/webauth/... -run Reseal -count=1` | ✅ green |
| **SC1** re-seal fires on read AND write | REQ-session-rotation | `TestNewConnectResealInterceptor_FiresOnSuccess` (`internal/server/connectreseal_test.go`) | unit (interceptor) | `go test ./internal/server/... -run Reseal -count=1` | ✅ green |
| **SC1** Set-Cookie reaches the HTTP wire | REQ-session-rotation | `TestConnectResealSetCookieReachesWire` (`internal/server/connectreseal_wire_test.go`) | integration (httptest, real codec) | `go test ./internal/server/... -run Wire -count=1` | ✅ green |
| **SC3** forward-monotonic under concurrency | REQ-session-rotation | `TestResealForwardMonotonicUnderConcurrency` (50 goroutines, `-race`) | unit (race) | `go test ./internal/webauth/... -run Concurren -race -count=1` | ✅ green |
| **SC4** hard expiry strict, zero skew | REQ-session-rotation | `TestResolveHardExpiryHasNoSkewTolerance` (1ns-expired reject), `TestResealNoopOnExpiredCookie` (`resolver_test.go` / `reseal_test.go`) | unit (guard/negative) | `go test ./internal/webauth/... -run 'HardExpiry|Expired' -count=1` | ✅ green |
| **D-08** dual-cookie refresh (session + CSRF) | REQ-session-rotation | `TestResealPastThresholdRefreshesCSRFCookie` + the wire test (both cookies) | unit + integration | `go test ./internal/webauth/... -run CSRF -count=1` | ✅ green |
| **D-01/D-03/D-04** interceptor: best-effort, innermost, skip-on-error/nil, nil-passthrough | REQ-session-rotation | `TestNewConnectResealInterceptor_{SkipsOnError,SkipsOnNilResponse,NilResealIsPassthrough}` | unit (interceptor contract) | `go test ./internal/server/... -run Reseal -count=1` | ✅ green |
| **WR-01** (code-review hardening) no-op on legacy/expired/empty-owner cookie | REQ-session-rotation | `TestResealNoopOn{LegacyVersionCookie,ExpiredCookie,EmptyOwnerCookie}` | unit (negative-space) | `go test ./internal/webauth/... -run Noop -count=1` | ✅ green |

---

## Wave 0 Requirements

*Complete.* All test files landed with their implementation:
- [x] `internal/webauth/reseal_test.go` — threshold, dual-cookie, `-race` concurrency, WR-01 no-op guards
- [x] `internal/webauth/resolver_test.go` — `TestResolveHardExpiryHasNoSkewTolerance` (SC4)
- [x] `internal/server/connectreseal_test.go` — interceptor contract (fires/skips)
- [x] `internal/server/connectreseal_wire_test.go` — real-server `Set-Cookie`-on-the-wire (WR-02)

Framework already present (`go test`); no install needed.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| A real browser persists the re-sealed cookie across a long live session, extending it past the original 12h | SC1 | Real-browser cookie-jar behavior is outside `go test` scope (the wire test proves `Set-Cookie` is emitted; only a live browser proves the jar applies it across requests) | Log in via OIDC, idle past the 6h threshold, issue a read, confirm the response carries a fresh `engram_session` `Set-Cookie` and the session outlives the original 12h |
| The ADR `engram-slr8` accurately documents rotation-under-statelessness + the no-revocation limitation | SC2 | Documentation artifact, not executable behavior | Verified by the code reviewer + security auditor (T-18-01); `docs/adr/engram-slr8-*.md` is Accepted, names the real `ENGRAM_UI_COOKIE_KEY` kill-switch |

---

## Validation Sign-Off

- [x] All requirement/SC rows have an `<automated>` verify (or a documented manual-only reason)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all references
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-07-13

---

## Validation Audit 2026-07-13

| Metric | Count |
|--------|-------|
| Requirements / SCs audited | 8 (SC1×3, SC3, SC4, D-08, interceptor-contract, WR-01) |
| COVERED (automated, green) | 8 |
| PARTIAL | 0 |
| MISSING | 0 |
| Manual-only (documented, not gaps) | 2 (live-browser persistence, ADR doc) |

No code-test gaps found. State A audit: existing tests already cover every automatable
success criterion (including the WR-01 code-review hardening), all green under `-race`.
`nyquist_compliant: true`. No `gsd-nyquist-auditor` spawn required.
