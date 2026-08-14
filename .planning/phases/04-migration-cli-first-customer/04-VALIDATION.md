---
phase: 4
slug: migration-cli-first-customer
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-14
---

# Phase 4 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go 1.x toolchain, per go.mod) |
| **Config file** | none required — `go.mod` + existing suite conventions |
| **Quick run command** | `go test ./internal/migrate/... ./internal/store/... ./cmd/engram/...` |
| **Full suite command** | `task` (lint + `go test ./...` + skill-plugin pytest) |
| **Estimated runtime** | ~60–120 seconds (full); Qdrant-dependent specs use testcontainers (strict variant: `task test:strict` with `ENGRAM_REQUIRE_QDRANT=1` — never silently skip) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/migrate/... ./cmd/engram/...` (per-plan scope)
- **After every plan wave:** Run `task` (full lint + test)
- **Before `/gsd-verify-work`:** Full suite must be green, including `task test:strict` for sweep/histogram/facet paths that touch Qdrant
- **Max feedback latency:** 120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 4-TBD-01 | TBD | TBD | REQ-migrate-command | TBD | bare `migrate` previews only; `--apply` is the write choke point | unit | `go test ./cmd/engram/ -run Migrate` | ❌ W0 | ⬜ pending |
| 4-TBD-02 | TBD | TBD | REQ-migrate-status-histogram | TBD | mixed-version collection renders a distribution, not a scalar | unit+integration | `go test ./internal/store/ -run MigrateStatus` | ❌ W0 | ⬜ pending |
| 4-TBD-03 | TBD | TBD | REQ-migrate-preview-apply-parity | TBD | apply set == preview set ∩ fresh re-derivation | unit | `go test ./cmd/engram/ -run PreviewApply` | ❌ W0 | ⬜ pending |
| 4-TBD-04 | TBD | TBD | REQ-backfill-shortids-first-step | TBD | standalone alias delegates; `--dry-run` removed; upgrade-guide entry gated by test | unit | `go test ./cmd/engram/ -run Backfill` | ❌ W0 | ⬜ pending |
| 4-TBD-05 | TBD | TBD | REQ-migrate-revert | TBD | whole-range preflight refusal names every irreversible step + snapshot path | unit | `go test ./internal/store/ -run Revert` | ❌ W0 | ⬜ pending |
| 4-TBD-06 | TBD | TBD | REQ-migrate-never-automatic | TBD | startup never migrates; at most a non-blocking warning | unit | `go test ./cmd/engram/ -run Startup` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky — rows are seeded per requirement; plan IDs/waves fill in when PLAN.md files land, per RESEARCH.md §Validation Architecture.*

---

## Wave 0 Requirements

- [ ] Pin-update: `internal/migrate` `TestCurrentVersionValue` must move 0→1 when the v0→v1 step registers (RESEARCH: currently pinned at 0)
- [ ] `TestMigrateConvergesWithoutLock` re-run with an ordinary record at Target=0 (PA-10a item 3 — BLOCKING per RESEARCH)

*Otherwise: existing Go test infrastructure (incl. Phase 1's de-flaked Qdrant testcontainer) covers phase requirements.*

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
