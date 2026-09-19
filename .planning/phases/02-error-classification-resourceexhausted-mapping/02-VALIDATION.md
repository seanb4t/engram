---
phase: "2"
slug: "error-classification-resourceexhausted-mapping"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-19"
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`, table-driven) + real-Qdrant integration via `internal/store/storetest` |
| **Config file** | none — `go.mod` / `Taskfile.yaml` |
| **Quick run command** | `go test ./internal/store/... ./internal/server/... ./cmd/engram/... -short -count=1` |
| **Full suite command** | `task` |
| **Estimated runtime** | ~300 seconds (full, real Qdrant) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/store/... ./internal/server/... ./cmd/engram/... -short -count=1`
- **After every plan wave:** Run `ENGRAM_REQUIRE_QDRANT=1 task`
- **Before `/gsd-verify-work`:** `task` must be green, plus a live-observed RED/GREEN pair for each lane's new test (store, Connect, MCP, CLI)
- **Max feedback latency:** 120 seconds (quick run)

---

## Per-Task Verification Map

Requirement-level rows seeded from RESEARCH.md § Validation Architecture; the planner/executor
refines these to task IDs and concrete test names (re-resolve every `-run` against `go test -list`).

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | REQ-exhausted-sentinel | — | non-matching ResourceExhausted NOT relabeled | unit (synthetic statuses, 3 receive shapes + negatives) | `go test ./internal/store/ -run '^Test<Classifier>$' -v -count=1` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-exhausted-sentinel | — | N/A | integration (real Qdrant overflow) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^Test<E2E>$' -v -count=1` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-exhausted-connect | — | no raw gRPC/Qdrant text, no byte ceiling on the wire | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^Test<Connect>$' -v -count=1` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-exhausted-mcp | — | envelope only; raw error logged server-side once | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^Test<MCP>$' -v -count=1` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-exhausted-cli-docs | — | N/A | unit (existing table) | `go test ./cmd/engram/ -run '^TestExitCodeForConnectErrTable$' -v -count=1` | ✅ | ⬜ pending |
| TBD | TBD | TBD | REQ-exhausted-cli-docs | — | N/A | unit (existing mechanical gate) | `go test ./cmd/engram/ -run '^TestCatalogExitCodesMatchMapper$' -v -count=1` | ✅ | ⬜ pending |
| TBD | TBD | TBD | REQ-exhausted-cli-docs | — | N/A | integration (binary + server + Qdrant) | `go test ./internal/e2e/ -run '^TestCLIExitCodes$' -v -count=1` | ✅ (subtest TBD) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] classifier unit test (`internal/store`)
- [ ] end-to-end `Store.List` overflow test (`internal/store`, `storetest.SeedOversized`)
- [ ] Connect-lane and MCP-lane overflow tests (`internal/server`)
- [ ] exit-10 subtest (`internal/e2e`)
- [ ] this phase's `redEvidenceDirs` entry + hand-verified patches

---

## Manual-Only Verifications

All phase behaviors have automated verification.

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
