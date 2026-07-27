---
phase: 23
slug: service-auth-chain-tenancy-isolation
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-07-17
validated: 2026-07-17
---

# Phase 23 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Seeded from 23-RESEARCH.md `## Validation Architecture`. The per-task map and Wave 0 list are
> populated by the planner/executor.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`) |
| **Config file** | none — Go toolchain; `Taskfile.yaml` drives `task test` |
| **Quick run command** | `go test ./internal/auth/... ./internal/config/...` |
| **Full suite command** | `task test` (lint + full `go test ./...`, incl. `internal/store` isolation/sharing suite) |
| **Estimated runtime** | ~30–120 seconds (store suite may require Qdrant per `requireQdrant` gate) |

---

## Sampling Rate

- **After every task commit:** Run the quick command for the touched package(s)
- **After every plan wave:** Run `task test`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 23-01 | 01 | 1 | REQ-service-owner-failclosed | T-23-01 | Authenticated service principal with unresolvable owner claim → explicit fail-closed error (never `owner==""` / anonymous bucket) — the FIRST test | unit | `go test ./internal/auth/... -run TestFailClosedRejectsEmptyOwner` | ✅ | ✅ green |
| 23-01 | 01 | 1 | REQ-service-auth-chain | T-23-05 | `NewService` per-lane audience independent of the human lane | unit | `go test ./internal/auth/... -run TestNewServiceIndependentAudience` | ✅ | ✅ green |
| 23-02 | 02 | 1 | REQ-static-token-auth | T-23-03/04/07 | Constant-time full-value compare, per-owner map, token never in error/log/span | unit | `go test ./internal/auth/... -run TestStaticToken` | ✅ | ✅ green |
| 23-03 | 03 | 1 | REQ-service-auth-chain | T-23-02/08 | JWT-vs-opaque discriminator routes before verify; deny-by-default; nil-lane guard | unit | `go test ./internal/auth/... -run TestChainVerifier` | ✅ | ✅ green |
| 23-04 | 04 | 1 | REQ-static-token-auth / REQ-service-auth-chain | T-23-09/10 | Service owner-claims default `client_id,azp`; fatal-when-malformed static-tokens config; two-dot guard | unit | `go test ./internal/config/... -run TestServiceAuth` | ✅ | ✅ green |
| 23-05 | 05 | 1 | REQ-service-principal-isolation | T-23-11/06 | Cross-owner private isolation; cross-tenant `shared`-read intended (D-15) | integration (Qdrant) | `go test ./internal/store/... -run 'TestServicePrincipalIsolation|TestSharedCrossTenantReadIntended'` | ✅ | ✅ green |
| 23-06 | 06 | 2 | REQ-service-auth-chain / REQ-service-principal-isolation | T-23-01/12 | Chain wired at `withAuth`; fail-closed survives composition; human-only path preserved; static-token lane authenticates end-to-end from config | integration | `go test ./cmd/engram/... -run TestWithAuth; go test ./internal/server/... -run TestServiceAuthChainParity` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*The FIRST test (the fail-closed empty-owner proof, SC2 / D-08 / D-10) precedes all other
service-auth tasks and passes. All 4 phase REQ IDs are covered by green automated tests; the
CR-01 code-review fix added the `cmd/engram/serve_test.go` config→withAuth→live-verify wiring
test that closes the parse→wire→verify seam.*

---

## Wave 0 Requirements

- [x] `internal/auth/service_owner_failclosed_test.go` — fail-closed empty-owner proof (the FIRST test) + `NewService` audience for REQ-service-owner-failclosed / REQ-service-auth-chain
- [x] `internal/auth/chain_test.go` — chain discriminator/order/deny-by-default/nil-guard for REQ-service-auth-chain
- [x] `internal/auth/static_token_test.go` — constant-time compare + no-leak + per-owner (token→owner) map + rotation for REQ-static-token-auth
- [x] `internal/config/service_auth_test.go` — config parse + fatal-when-malformed validation + two-dot guard for REQ-static-token-auth / REQ-service-auth-chain
- [x] `internal/store/service_principal_isolation_test.go` — service-principal cross-owner private isolation + `shared`-cross-tenant (D-15/D-16) for REQ-service-principal-isolation
- [x] `cmd/engram/serve_test.go` + `internal/server/connectapi_service_auth_parity_test.go` — end-to-end withAuth wiring + chain-parity for REQ-service-auth-chain

*All Wave 0 test files exist and pass. Existing `go test` infrastructure covered the framework; no install needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live OIDC client-credentials token from a real IdP resolves to a non-empty namespaced owner | REQ-service-auth-chain | Requires a real IdP issuing client-credentials JWTs (claim shape is deployment/IdP-specific: `client_id` vs `azp`) | Configure a service-lane issuer/audience + owner-claim `[client_id, azp]`; mint a client-credentials token; assert recall lands in the service owner bucket, not anonymous |

*Automated tests use synthetic claim maps for the fail-closed + isolation proofs; the live-IdP claim-shape confirmation is the one manual check.*

---

## Validation Audit 2026-07-17

| Metric | Count |
|--------|-------|
| Requirements (phase) | 4 |
| Covered (green automated tests) | 4 |
| Partial | 0 |
| Missing | 0 |
| Gaps found | 0 |
| Resolved | 0 |
| Escalated (manual-only) | 0 |

All 4 phase requirements are COVERED by green automated tests (7 test files); no gaps to fill —
no `gsd-nyquist-auditor` spawn required. State A audit run by `/gsd-validate-phase 23`.

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (none — all covered)
- [x] No watch-mode flags
- [x] Feedback latency < 120s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** verified 2026-07-17
