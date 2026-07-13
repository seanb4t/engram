---
phase: 18
slug: stateless-session-rotation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-13
---

# Phase 18 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Test dimensions derive from 18-RESEARCH.md § Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (stdlib) |
| **Config file** | none — `Taskfile.yaml` orchestrates |
| **Quick run command** | `go test ./internal/webauth/... ./internal/server/... -count=1` |
| **Full suite command** | `task` (lint + test) |
| **Estimated runtime** | ~30–60 seconds (no live Qdrant needed — pure cookie/interceptor logic) |

---

## Sampling Rate

- **After every task commit:** Run the quick run command for the touched package.
- **After every plan wave:** Run `task` (lint + test).
- **Before `/gsd-verify-work`:** Full suite must be green (`task` clean).
- **Max feedback latency:** ~60 seconds.

---

## Per-Task Verification Map

*Filled by the planner/executor. Anchored to REQ-session-rotation and the 4 ROADMAP success criteria (SC1 threshold re-seal; SC2 ADR; SC3 forward-monotonic concurrency; SC4 strict hard-expiry + threshold-only skew).*

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 18-01-01 | 01 | 1 | REQ-session-rotation | — | Re-seal emits forward-only `now+TTL` expiry once past ½-TTL threshold | unit | `go test ./internal/webauth/... -run Reseal -count=1` | ❌ W0 | ⬜ pending |
| 18-01-02 | 01 | 1 | REQ-session-rotation | T-18 (SC3) | N concurrent near-expiry re-seals are all forward-monotonic (no shortening) | unit (concurrency, `nowUTC` seam) | `go test ./internal/webauth/... -run Concurren -race -count=1` | ❌ W0 | ⬜ pending |
| 18-01-03 | 01 | 1 | REQ-session-rotation | T-18 (SC4) | `Resolver.Resolve` hard-expiry check stays byte-for-byte strict (no skew) | unit (guard) | `go test ./internal/webauth/... -run HardExpiry -count=1` | ❌ W0 | ⬜ pending |
| 18-02-01 | 02 | 2 | REQ-session-rotation | — | Interceptor sets `Set-Cookie` on the response for authenticated read+write; skips on error/below-threshold | integration (httptest chain) | `go test ./internal/server/... -run Reseal -count=1` | ❌ W0 | ⬜ pending |

---

## Wave 0 Requirements

- [ ] `internal/webauth/reseal_test.go` — threshold, forward-monotonic concurrency, CSRF-cookie Max-Age refresh (REQ-session-rotation SC1/SC3)
- [ ] `internal/webauth/resolver_test.go` (extend) — hard-expiry-unchanged guard (SC4)
- [ ] `internal/server/connectreseal_test.go` — end-to-end `Set-Cookie`-on-response over the real interceptor chain (httptest), read+write, error/below-threshold no-op

*Framework already present (`go test`); no install needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Browser actually persists the re-sealed cookie across a long live session | REQ-session-rotation SC1 | Real-browser cookie-jar behavior is out of `go test` scope | Log in via OIDC, idle past the ½-TTL threshold, issue a read, confirm the response carries a fresh `engram_session` `Set-Cookie` and the session outlives the original 12h |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
