---
phase: "1"
slug: "executor-correctness-man-pages"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-13"
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go stdlib `testing`, no third-party framework) |
| **Config file** | none — `go test ./...` via `task test:go` (`Taskfile.yaml`) |
| **Quick run command** | `go test ./internal/setup/... ./cmd/engram/... -run <TestName> -v` |
| **Full suite command** | `task` (== `task lint` + `task test`) |
| **Estimated runtime** | ~60 seconds (quick runs < 10s) |

---

## Sampling Rate

- **After every task commit:** Run the specific `-run` command(s) for the file(s) touched by that commit
- **After every plan wave:** Run `task test:go` (`go test ./...`)
- **Before `/gsd-verify-work`:** Full suite must be green (`task`), plus `git diff --exit-code go.mod go.sum` (no automated test asserts the empty go.mod diff)
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 1-01-01 | 01 | 1 | REQ-osrun-deadline-error | — | N/A | unit (real subprocess, no third-party CLI) | `go test ./internal/setup/... -run TestOsRunReportsContextDeadlineExceeded -v` | ❌ W0 | ⬜ pending |
| 1-01-02 | 01 | 1 | REQ-osrun-deadline-error | — | N/A | unit (fake `Environment.Run`) | `go test ./internal/setup/... -run TestDriftReportedLegibly -v` | ✅ | ⬜ pending |
| 1-02-01 | 02 | 1 | REQ-manpages-generated | — | N/A | unit | `go test ./cmd/engram/... -run TestManPagesByteStable -v` | ❌ W0 | ⬜ pending |
| 1-02-02 | 02 | 1 | REQ-manpages-generated | — | N/A | unit | `go test ./cmd/engram/... -run TestManPagesMatchAvailableCommands -v` | ❌ W0 | ⬜ pending |
| 1-02-03 | 02 | 1 | REQ-manpages-cask-installed | T-1-01 | Hook paths are `#{HOMEBREW_PREFIX}`-relative; `rm_f` file-scoped, never `rm_rf` | static (string/ordering assertions over `.goreleaser.yaml`) | `go test ./cmd/engram/... -run TestReleaseConfigCaskInstallGate -v` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Task IDs are provisional until the planner assigns plan/task numbers; the planner MUST reconcile this table against the emitted PLAN.md files.*

---

## Wave 0 Requirements

- [ ] `internal/setup/environment_test.go` — new file; real-subprocess deadline test for REQ-osrun-deadline-error (`osRun` has zero existing coverage)
- [ ] `cmd/engram/man_test.go` — new file; byte-stability + expected-file-set tests for REQ-manpages-generated
- [ ] Framework install: none — `go test` already configured; no new test dependency

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| — | — | — | — |

*All phase behaviors have automated verification. (Live cask install/uninstall on a real Homebrew is a post-release observation, not a phase gate — rule `m45p2b4bp7`.)*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
