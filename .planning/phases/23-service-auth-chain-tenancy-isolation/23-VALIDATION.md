---
phase: 23
slug: service-auth-chain-tenancy-isolation
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-17
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
| 23-01-01 | 01 | 1 | REQ-service-owner-failclosed | T-23-01 | Authenticated service principal with unresolvable owner claim → explicit fail-closed error (never `owner==""` / anonymous bucket) | unit | `go test ./internal/auth/... -run FailClosed` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Full per-task map is authored by the planner across the phase's PLAN.md files; the FIRST test
(the fail-closed empty-owner proof, SC2 / D-08 / D-10) MUST precede all other service-auth tasks.*

---

## Wave 0 Requirements

- [ ] `internal/auth/chain_test.go` (or equivalent) — fail-closed empty-owner proof + chain-discriminator + isolation-parity stubs for REQ-service-auth-chain / REQ-service-owner-failclosed
- [ ] `internal/auth/statictoken_test.go` — constant-time compare + no-leak + per-owner-map stubs for REQ-static-token-auth
- [ ] `internal/store/store_test.go` — extend the existing isolation/sharing suite with the service-principal cross-tenant private-isolation + `shared`-cross-tenant (D-15/D-16) proofs for REQ-service-principal-isolation

*Existing `go test` infrastructure covers the framework; no install needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live OIDC client-credentials token from a real IdP resolves to a non-empty namespaced owner | REQ-service-auth-chain | Requires a real IdP issuing client-credentials JWTs (claim shape is deployment/IdP-specific: `client_id` vs `azp`) | Configure a service-lane issuer/audience + owner-claim `[client_id, azp]`; mint a client-credentials token; assert recall lands in the service owner bucket, not anonymous |

*Automated tests use synthetic claim maps for the fail-closed + isolation proofs; the live-IdP claim-shape confirmation is the one manual check.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
