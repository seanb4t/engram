---
phase: 1
slug: shared-auth-chain-connect-bearer-identity
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
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
| 01-01 T1 | 01-01 | 1 | REQ-connect-token-expiry | T-01-03 — Token replay past expiry | Stub verifier returns `TokenInfo{Expiration: past}`, `err==nil` → Connect rejects | unit | `go test ./internal/auth/... -run TestEnforceExpiry -v` | ✅ `internal/auth/bearer_test.go` | ✅ green |
| 01-01 T1 | 01-01 | 1 | REQ-connect-token-expiry | T-01-03 — Token replay past expiry | Zero `Expiration` also rejects (D-05) | unit | `go test ./internal/auth/... -run TestEnforceExpiryZero -v` | ✅ `internal/auth/bearer_test.go` | ✅ green |
| 01-03 T2 | 01-03 | 3 | REQ-connect-bearer-identity | T-03-03 — Lane drift | Same verifier value reaches both mount sites (D-06) | unit/structural | `go test ./cmd/engram/... -run TestAuthChainSharedBetweenLanes -v` | ✅ `cmd/engram/serve_test.go` | ✅ green |
| 01-02 T2 | 01-02 | 2 | REQ-connect-bearer-identity | T-02-03 — Lane drift | Token accepted on MCP is accepted on Connect and vice versa | unit | `go test ./internal/server/... -run TestBearerLaneParity -v` | ✅ `internal/server/connectapi_bearer_parity_test.go` | ✅ green |
| 01-01 T2 | 01-01 | 1 | REQ-connect-lane-provenance | T-01-01 — CSRF | Cookie caller omitting `X-CSRF-Token` still rejected (write FIRST) | unit | `go test ./internal/server/... -run TestCSRFCookieCallerOmittingHeaderIsStillRejected -v` | ✅ `internal/server/connectcsrf_lane_test.go` | ✅ green |
| 01-01 T2 | 01-01 | 1 | REQ-connect-lane-provenance | T-01-01 — CSRF bypass | Cookie caller cannot self-declare bearer lane via a garbage `Authorization` header | unit | `go test ./internal/server/... -run TestCSRFCookieCallerCannotSelfDeclareBearerLane -v` | ✅ `internal/server/connectcsrf_lane_test.go` | ✅ green |
| 01-01 T1 | 01-01 | 1 | REQ-connect-lane-provenance | T-01-02 — Confused deputy | Bearer verification failure never falls through to cookie | unit | `go test ./internal/server/... -run TestBearerFailureNeverFallsThroughToCookie -v` | ✅ `internal/server/connectbearer_test.go` | ✅ green |
| 01-01 T2 | 01-01 | 1 | REQ-connect-lane-provenance | T-01-04 — Fail-open on unknown lane | Unstamped/unknown lane on a write RPC → `PermissionDenied`, no CSRF check attempted (D-08) | unit | `go test ./internal/server/... -run TestCSRFLaneUnstampedFailsClosed -v` | ✅ `internal/server/connectcsrf_lane_test.go` | ✅ green |
| 01-02 T1 | 01-02 | 2 | REQ-connect-lane-provenance | T-02-01 — Session fixation | Reseal skipped for a non-`LaneCookie` request (D-09) | unit | `go test ./internal/server/... -run TestResealGatesOnCookieLane -v` | ✅ `internal/server/connectreseal_test.go` | ✅ green |
| 01-03 T3 | 01-03 | 3 | REQ-connect-headless-mount | T-03-02 — Surface regression on upgrade | UI disabled AND `connect.headless` unset → Connect unmounted, byte-for-byte today's behavior | unit | `go test ./internal/server/... -run TestMountConnectDefaultOffWithoutUIOrHeadlessFlag -v` | ✅ `internal/server/connectmount_test.go` | ✅ green |
| 01-03 T3 | 01-03 | 3 | REQ-connect-headless-mount | T-03-01 — Unauthenticated write surface | headless + zero configured auth lanes → startup refusal (D-11) | unit | `go test ./cmd/engram/... -run TestHeadlessRefusesStartWithoutAuthLane -v` | ✅ `cmd/engram/serve_test.go` | ✅ green |
| 01-03 T1 | 01-03 | 3 | REQ-connect-headless-mount | T-03-04 — Config default drift | `connect.headless` config-loader zero-value case | unit | `go test ./internal/config/... -run TestConnectHeadlessDefault -v` | ✅ `internal/config/connect_test.go` | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

All twelve rows above were Wave 0 gaps at plan time; execution closed every one. Each file below
exists and its contracted tests are green (verified `2026-08-02` — 12/12 `--- PASS:`, 0 fail, 0 skip).

- [x] `internal/auth/bearer_test.go` — `EnforceExpiry` unit tests (zero, past, valid, error-passthrough). *Landed here rather than the planned `expiry_test.go`: `EnforceExpiry` ships in `internal/auth/bearer.go`, so the test file follows the source file's name.*
- [x] `internal/server/connectbearer_test.go` — new bearer adapter unit tests (mirrors `webauth/resolver_test.go`)
- [x] `internal/server/connectcsrf_lane_test.go` — the four lane-provenance negative tests
- [x] `internal/server/connectreseal_test.go` extension — the `LaneCookie`-gate test (D-09)
- [x] `cmd/engram/serve_test.go` — the D-06 structural/parity test and the D-11 startup-refusal test
- [x] `internal/config/connect_test.go` — the `connect.headless` zero-value test

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

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Every targeted `-run` command proven with `-v` RUN/PASS pairs (repo false-green guard)
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-08-02 — retroactive audit, no gaps found

---

## Validation Audit 2026-08-02

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

All twelve contracted rows resolved to existing, compiling, passing tests — no auditor
intervention and no new test files were required. Evidence:

- `go test -list` over the twelve contracted names returned 12/12, proving each test exists
  **and compiles** (a stronger check than a source text match).
- `go test -count=1 -v -run '<the twelve>' ./internal/auth/... ./internal/server/... ./cmd/engram/... ./internal/config/...`
  → 12 `--- PASS:`, 0 `--- FAIL:`, 0 `--- SKIP:`. Counting `--- PASS:` lines rather than trusting
  package-level `ok` is required here: a `-run` filter matching nothing still exits 0.
- Both Fail-Closed-First obligations above were honored during execution and are recorded in the
  plan artifacts — `TestEnforceExpiry` ("RED observed before the body existed", 01-01-PLAN T-01-03)
  and `TestCSRFCookieCallerCannotSelfDeclareBearerLane` (01-01-PLAN T-01-01, fail-first proof
  against the header-presence-keyed variant).
- Execution delivered substantially more coverage than the twelve contracted rows; the full
  per-decision test inventory lives in the `coverage:` blocks of `01-01`–`01-04-SUMMARY.md`.
