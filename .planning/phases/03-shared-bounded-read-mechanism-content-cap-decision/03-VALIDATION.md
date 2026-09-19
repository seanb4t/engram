---
phase: "3"
slug: "shared-bounded-read-mechanism-content-cap-decision"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-19"
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test + `internal/store/storetest` (Phase 1) |
| **Config file** | none |
| **Quick run command** | `go test ./internal/store/... ./internal/server/... ./internal/config/... -short -count=1` |
| **Full suite command** | `task` |
| **Estimated runtime** | ~300 seconds (full, real Qdrant) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/store/... ./internal/server/... ./internal/config/... -short -count=1`
- **After every plan wave:** Run `ENGRAM_REQUIRE_QDRANT=1 task`
- **Before `/gsd-verify-work`:** `task` green
- **Max feedback latency:** 120 seconds (quick run)

---

## Per-Task Verification Map

Requirement-level rows seeded from RESEARCH.md § Validation Architecture; the planner refines them
to task IDs and real test names (re-resolve every `-run` against `go test -list`, durable record
`bsbsvn4hbc`).

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | REQ-byte-budget-pages | — | N/A | integration (real Qdrant) | ordered-page helper stops on bytes AND count | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-byte-budget-pages | — | N/A | integration (real Qdrant) | `scrollAllPoints` byte-budget extension | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-byte-budget-pages (D-07) | — | never skip a record | integration (raw `package store` write) | batch-of-1 fallback → `ErrResponseTooLarge` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-content-cap-decided | — | oversized write rejected on every lane | integration | content/tags cap rejection, all write paths, MCP + Connect + CLI | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-content-cap-decided | — | N/A | integration | existing over-cap records stay readable | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-content-cap-decided | — | N/A | unit | config validation rejects 0 / non-positive caps | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] ordered-page helper test file
- [ ] `scrollAllPoints` byte-budget test coverage
- [ ] raw-write legacy over-cap fixture helper in `package store`
- [ ] content/tags cap rejection tests (all write paths, both lanes, CLI)
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
