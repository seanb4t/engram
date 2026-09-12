---
phase: 2
slug: setup-command-core
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-29
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (stdlib) |
| **Config file** | none — golden fixtures under `cmd/engram/testdata/` |
| **Quick run command** | `go test ./cmd/engram/... ./internal/setup/... -run TestSetup` |
| **Full suite command** | `task test` |
| **Estimated runtime** | ~60 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/setup/... ./cmd/engram/... -run <newly-added-test-name>`
- **After every plan wave:** Run `task test`
- **Before `/gsd-verify-work`:** Full suite green, plus `task surfaces:gen` run and its golden diff committed
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | REQ-setup-detects-runtimes | — | N/A | unit | `go test ./internal/setup/... -run TestDetect` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-setup-previews-by-default | — | No mutation without `--apply` | unit | `go test ./cmd/engram/... -run TestSetupPreview` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-setup-idempotent | — | N/A | unit | `go test ./internal/setup/... -run TestOutcome` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-setup-non-interactive | — | N/A | unit | `go test ./cmd/engram/... -run TestSetupNonInteractive` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-setup-partial-failure-legible | — | N/A | unit | `go test ./cmd/engram/... -run TestSetupExitCodes` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-setup-correct-by-reading | — | Preview redacts secret value (D-16) | golden | `go test ./cmd/engram/... -run TestHelpGolden` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Task IDs are filled in by `/gsd-validate-phase` once PLAN.md files exist.*

---

## Wave 0 Requirements

- [ ] `internal/setup/detect_test.go` — covers REQ-setup-detects-runtimes via an injectable `Environment` fake (never a real `PATH` mutation; never asserts third-party CLI behavior, per rule `m45p2b4bp7`)
- [ ] `cmd/engram/setup_test.go` — covers preview/apply/non-interactive/exit-code criteria; modeled on `cmd/engram/prune_test.go`
- [ ] `destructive_test.go` `TestMutatingCommandNamesMembership` — add `"setup": true` to the pinned `want` map in the SAME commit as the `toolclass.go` row
- [ ] `catalog_test.go` `wantExitCodes` / `nonConnectProducedCodes` — add the two new exit-code entries in the SAME commit as the new consts
- [ ] `testdata/catalog.golden`, `testdata/help.golden` — regenerate via `task surfaces:gen` AFTER `setup` is wired; never hand-edit

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real-machine runtime detection against actually-installed `claude`/`codex`/`opencode` binaries | REQ-setup-detects-runtimes | Asserting on third-party binaries' presence would violate rule `m45p2b4bp7`; automated tests use a fake `Environment` | Run `engram setup` on a machine with a known runtime mix; confirm the report matches reality |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
