---
phase: "1"
slug: "test-harness-fixture-helper"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-18"
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`) + `testcontainers-go/modules/qdrant` |
| **Config file** | none — harness behavior lives in each package's `TestMain` (moving to `internal/store/storetest` this phase) |
| **Quick run command** | `go test -short ./internal/store/... ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/...` |
| **Full suite command** | `task test` |
| **Estimated runtime** | ~300 seconds (full, real Qdrant) |

---

## Sampling Rate

- **After every task commit:** Run `go test -short ./internal/store/... ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/...`
- **After every plan wave:** Run `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/... -count=1`
- **Before `/gsd-verify-work`:** `task test` must be green, INCLUDING `TestRedEvidencePatchesAreLive` (no `-short`)
- **Max feedback latency:** 120 seconds (quick run)

---

## Per-Task Verification Map

Requirement-level rows seeded from RESEARCH.md § Validation Architecture; the planner/executor
refines these to task IDs.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | REQ-oversized-fixture-helper | — | N/A | integration (real Qdrant) | `go test ./internal/store/... -run 'TestListScopesFullPayloadsOverGRPCLimit' -count=1 -v` plus the new seeder's own test | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-test-client-parity | — | N/A | AST/static | new D-11 convergence gate test (`go test ./internal/store/... -run <gate> -count=1 -v`) | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | (existing gate) | — | N/A | AST/static | `go test ./internal/store/... -run TestQdrantClientIsHeldOnlyByStorePackage -count=1 -v` | ✅ | ⬜ pending |
| TBD | TBD | TBD | (existing gate, currently RED) | — | N/A | integration (mutates+reverts tree) | `go test ./internal/store/... -run TestRedEvidencePatchesAreLive -count=1 -v` | ✅ | ⬜ pending |
| TBD | TBD | TBD | (CI pinned invariant) | — | N/A | integration | `go test -run '^TestSharedQdrantAddressHonored$' -v -count=1 ./internal/store/... ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/...` (3 PASS + 1 SKIP unchanged) | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `store.NewQdrantClient` in `internal/store/store.go`
- [ ] `internal/store/storetest/` — `Dial` helper, oversized seeder (two shapes), container lifecycle
- [ ] D-11 AST convergence gate
- [ ] `.planning/phases/01-test-harness-fixture-helper/red-evidence/` + `redEvidenceDirs` registration

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
