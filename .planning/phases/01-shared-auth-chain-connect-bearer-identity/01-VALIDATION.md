---
phase: 1
slug: shared-auth-chain-connect-bearer-identity
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-31
---

# v0.12.x Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Seeded from `01-RESEARCH.md` § Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing`, table-driven — existing convention across `internal/server`, `internal/auth`, `internal/webauth` |
| **Config file** | none — no test-framework config beyond `go.mod` / `go vet` / `golangci-lint` |
| **Quick run command** | `go test ./internal/auth/... ./internal/server/... ./internal/webauth/... ./cmd/engram/... ./internal/config/...` |
| **Full suite command** | `task` (lint + test, per `Taskfile.yaml`) |
| **Estimated runtime** | ~30s quick / ~2–4 min full |

---

## Sampling Rate

- **After every task commit:** Run the quick run command above
- **After every plan wave:** Run `task` (full lint + test)
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~30 seconds

> **Repo-specific false-green guard:** `go test -run X ./pkg/...` matching nothing exits `0`
> with `ok … [no tests to run]`. Every targeted `-run` invocation MUST be proven with `-v` and a
> visible `=== RUN` / `--- PASS` pair. Re-point every test command whenever a package moves.

---

## Per-Task Verification Map

Task IDs are assigned by the planner; this table is the requirement→test contract the plans must
satisfy. `validate-phase` fills the Task ID / Plan / Wave columns after execution.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | REQ-connect-token-expiry | Token replay past expiry | Stub verifier returns `TokenInfo{Expiration: past}`, `err==nil` → Connect rejects | unit | `go test ./internal/auth/... -run TestEnforceExpiry -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-connect-token-expiry | Token replay past expiry | Zero `Expiration` also rejects (D-05) | unit | `go test ./internal/auth/... -run TestEnforceExpiryZero -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-connect-bearer-identity | Lane drift | Same verifier value reaches both mount sites (D-06) | unit/structural | `go test ./cmd/engram/... -run TestAuthChainSharedBetweenLanes -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-connect-bearer-identity | Lane drift | Token accepted on MCP is accepted on Connect and vice versa | unit | `go test ./internal/server/... -run TestBearerLaneParity -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-connect-lane-provenance | CSRF | Cookie caller omitting `X-CSRF-Token` still rejected (write FIRST) | unit | `go test ./internal/server/... -run TestCSRFCookieCallerOmittingHeaderIsStillRejected -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-connect-lane-provenance | CSRF bypass | Cookie caller cannot self-declare bearer lane via a garbage `Authorization` header | unit | `go test ./internal/server/... -run TestCSRFCookieCallerCannotSelfDeclareBearerLane -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-connect-lane-provenance | Confused deputy | Bearer verification failure never falls through to cookie | unit | `go test ./internal/server/... -run TestBearerFailureNeverFallsThroughToCookie -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-connect-lane-provenance | Fail-open on unknown lane | Unstamped/unknown lane on a write RPC → `PermissionDenied`, no CSRF check attempted (D-08) | unit | `go test ./internal/server/... -run TestCSRFLaneUnstampedFailsClosed -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-connect-lane-provenance | Session fixation | Reseal skipped for a non-`LaneCookie` request (D-09) | unit | `go test ./internal/server/... -run TestResealGatesOnCookieLane -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-connect-headless-mount | Surface regression on upgrade | UI disabled AND `connect.headless` unset → Connect unmounted, byte-for-byte today's behavior | unit | `go test ./internal/server/... -run TestMountConnectDefaultOffWithoutUIOrHeadlessFlag -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-connect-headless-mount | Unauthenticated write surface | headless + zero configured auth lanes → startup refusal (D-11) | unit | `go test ./cmd/engram/... -run TestHeadlessRefusesStartWithoutAuthLane -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-connect-headless-mount | Config default drift | `connect.headless` config-loader zero-value case | unit | `go test ./internal/config/... -run TestConnectHeadlessDefault -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

No existing test file covers any of the twelve rows above — all are Wave 0 gaps.

- [ ] `internal/auth/expiry_test.go` — `EnforceExpiry` unit tests (zero, past, valid, error-passthrough)
- [ ] `internal/server/connectbearer_test.go` — new bearer adapter unit tests (mirrors `webauth/resolver_test.go`)
- [ ] `internal/server/connectcsrf_lane_test.go` — the four lane-provenance negative tests
- [ ] `internal/server/connectreseal_test.go` extension — the `LaneCookie`-gate test (D-09)
- [ ] `cmd/engram/serve_test.go` — the D-06 structural/parity test and the D-11 startup-refusal test
- [ ] `internal/config` test extension — the `connect.headless` zero-value test

**No new test framework or fixture infrastructure needed.** The existing
`newConnectAPITestMux` / stub-resolver / `csrfTestVerify` helper patterns in `connectapi_test.go`,
`connectcsrf_test.go`, and `connectapi_service_auth_parity_test.go` cover every shape required.

---

## Fail-Closed-First Obligations

Per the v0.11.x precedent (three real defects shipped through a fully green suite; one executor
wrote a *passing* test asserting a bug), these two negative tests MUST be written and MUST be
observed failing before the code that makes them pass exists:

1. `TestEnforceExpiry` — a past-`Expiration` token is rejected on the Connect lane.
2. `TestCSRFCookieCallerCannotSelfDeclareBearerLane` — a cookie session cannot obtain the bearer
   CSRF exemption by attaching a garbage `Authorization` header.

A green run of either before its implementation lands is evidence the test is wrong, not that the
behavior is already correct.

---

## Manual-Only Verifications

*All phase behaviors have automated verification.* This phase introduces no external service,
runtime, or environment dependency (no Qdrant/testcontainers requirement).

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Every targeted `-run` command proven with `-v` RUN/PASS pairs (repo false-green guard)
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
