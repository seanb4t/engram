---
phase: 25
slug: supersession-with-history
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-19
---

# Phase 25 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`, `-race`) |
| **Config file** | none — Go modules; requires CGO + real Qdrant for store integration tests (`ENGRAM_REQUIRE_QDRANT`) |
| **Quick run command** | `go test ./internal/store/... ./internal/server/...` |
| **Full suite command** | `task test` |
| **Estimated runtime** | ~30–90 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/store/... ./internal/server/...`
- **After every plan wave:** Run `task test`
- **Before `/gsd-verify-work`:** Full suite must be green (incl. `-race` vs real Qdrant)
- **Max feedback latency:** ~90 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 25-01-T1 | 25-01 | 1 | REQ-supersession-links | T-25-06 | superseded record soft-hidden from Search AND List, still fetchable via Get; codec round-trips *string links | unit | `go test ./internal/store/... -run TestSupersedeRecallGate -count=1` | ❌ W0 | ⬜ pending |
| 25-01-T2 | 25-01 | 1 | REQ-supersession-links | T-25-01/02/03/04/05 | owner-only supersede (ActionWrite); already-superseded rejected; back-stamp preserves content+vector; TOCTOU fail-closed; forward chains | unit | `go test ./internal/store/... -run 'TestSupersede(Stamp\|OwnerGate\|AlreadySuperseded\|ForwardChain\|TOCTOU)' -count=1` | ❌ W0 | ⬜ pending |
| 25-02-T1 | 25-02 | 2 | REQ-supersession-links | T-25-07/08 | supersede_memory routes through the owner write gate; target-not-found re-wrapped with original input | unit | `go test ./internal/server/... -run TestSupersedeMemory -count=1` | ❌ W0 | ⬜ pending |
| 25-02-T2 | 25-02 | 2 | REQ-supersession-links | T-25-09 | ErrAlreadySuperseded maps to CodeFailedPrecondition (sentinel switch exhaustive) | unit | `go test ./internal/server/... -run TestConnectError -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*Planner populates concrete task rows from PLAN.md; validate-phase reconciles.*

---

## Wave 0 Requirements

- [ ] `internal/store/store_test.go` — supersede owner-gate + TOCTOU + already-superseded rejection (mirror `TestSetVisibilityOwnerGate` / `TestSetVisibilityTOCTOU`)
- [ ] `internal/store/store_test.go` — recall-gate exclusion at BOTH Search and List call sites + `get_memory` still fetchable (mirror `TestSearchDateWindow` / `TestListDateWindow`)
- [ ] Existing `go test -race` + real-Qdrant harness covers integration — no new framework install

*Existing infrastructure covers the framework; Wave 0 adds the supersede-specific stubs above.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| — | REQ-supersession-links | All behaviors are automatable via `go test` against real Qdrant | — |

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
