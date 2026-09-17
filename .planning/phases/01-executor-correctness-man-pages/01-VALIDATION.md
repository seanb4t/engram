---
phase: "1"
slug: "executor-correctness-man-pages"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
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
| 01-01-01 | 01 | 1 | REQ-osrun-deadline-error | T-01-01, T-01-02, T-01-05 | `osRun` returns `RunResult{}` + `ctx.Err()` for a deadline-killed/cancelled child (no partial capture surfaced); the only real child is the re-exec'd test binary, never a runtime CLI or `$HOME` | unit (real subprocess via `os.Args[0]`, no third-party CLI) — `TestOsRunReportsContextDeadlineExceeded`, `TestOsRunReportsContextCanceled`, `TestOsRunNonzeroExitStaysNilError` | `go test ./internal/setup/ -run '^TestOsRun' -count=1 -v` | ✅ | ✅ green |
| 01-01-02 | 01 | 1 | REQ-osrun-deadline-error | T-01-03, T-01-04 | Row names the timeout (`timed out after 20s: context deadline exceeded`); `Canceled` passes through unwrapped; probe seam error never proceeds to the write action | unit (fake `Environment.Run`) — subtests `probe-seam-deadline-exceeded-names-timeout`, `probe-seam-canceled-passes-through-unwrapped` | `go test ./internal/setup/ -run '^TestDriftReportedLegibly$' -count=1 -v` | ✅ | ✅ green |
| 01-01-03 | 01 | 1 | REQ-osrun-deadline-error | T-01-SC | Zero go.mod/go.sum drift; SPDX header on the new test file; order-independent, non-flaky suite | gate (lint + test + license + shuffle/count) | `task && task license:check && git diff --exit-code -- go.mod go.sum && go test ./internal/setup/ -count=1 -shuffle=on && go test ./internal/setup/ -run '^TestOsRun' -count=5` | ✅ | ✅ green |
| 01-02-01 | 02 | 1 | REQ-manpages-generated | T-02-03, T-02-05, T-02-07 | Pinned `.TH` header, no `HISTORY` footer, byte-stable; hidden/deprecated/help pages absent, `completion` subtree present; live command tree left unchanged (no grafted `help` children → `TestHelpGolden` stays green) | unit — `TestManPagesByteStable`, `TestManPagesMatchAvailableCommands`, `TestManCmdHiddenExactArgs`, `TestManGenerationLeavesCommandTreeUnchanged` + shuffled whole-package run | `go test ./cmd/engram -run '^TestMan' -count=1 -v && go test ./cmd/engram -count=1 -shuffle=on` | ✅ | ✅ green |
| 01-02-02 | 02 | 1 | REQ-manpages-cask-installed | T-02-01, T-02-02 | Hook paths are `#{HOMEBREW_PREFIX}`-relative; `man` step strictly after completions, exactly once; uninstall glob exactly once, `rm_f` file-scoped, never `rm_rf`; declarative stanza keyword absent | static (string/ordering assertions over `.goreleaser.yaml`) — extended `TestReleaseConfigCaskInstallGate` | `go test ./cmd/engram -run '^TestReleaseConfigCaskInstallGate$' -count=1 -v && task lint:yaml && task release:check` | ✅ | ✅ green |
| 01-02-03 | 02 | 1 | REQ-manpages-generated, REQ-manpages-cask-installed | T-02-SC | Zero go.mod/go.sum drift; SPDX headers on both new files; goldens untouched | gate (lint + test + license + goldens + goreleaser check) | `task && task license:check && git diff --exit-code -- go.mod go.sum && test -z "$(git status --porcelain -- cmd/engram/testdata)" && task release:check` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Task IDs reconciled by the planner on 2026-09-13 against `01-01-PLAN.md` (3 tasks) and `01-02-PLAN.md` (3 tasks); the provisional `1-02-01`/`1-02-02` rows collapsed into plan 02 Task 1, which owns both man-page tests. Threat refs point at each plan's `<threat_model>` register. Note: `TestCollectFlagsIsDepthAware` already fails under `-count=2` at HEAD (pre-existing, pflag redefinition) — never run `cmd/engram` with `-count>1`; use `-count=1 -shuffle=on` for order-independence.*

---

## Wave 0 Requirements

- [x] `internal/setup/environment_test.go` — new file; real-subprocess deadline test for REQ-osrun-deadline-error (`osRun` has zero existing coverage)
- [x] `cmd/engram/man_test.go` — new file; byte-stability + expected-file-set tests for REQ-manpages-generated
- [x] Framework install: none — `go test` already configured; no new test dependency

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| — | — | — | — |

*All phase behaviors have automated verification. (Live cask install/uninstall on a real Homebrew is a post-release observation, not a phase gate — rule `m45p2b4bp7`.)*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 60s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-09-17

---

## Validation Audit 2026-09-17

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

Retroactive reconciliation by `/gsd-validate-phase 1` at `main` HEAD `fbcf587f` (post-merge of PR #569): every
Per-Task Verification Map command was re-run and is green — `^TestOsRun` (3 tests, real re-exec'd child),
`TestDriftReportedLegibly`, `^TestMan` + shuffled `cmd/engram`, `TestReleaseConfigCaskInstallGate` + `task lint:yaml`
+ `task release:check`, `go.mod`/`go.sum` byte-clean, `cmd/engram/testdata` untouched, `-shuffle=on` and `-count=5`
runs stable. The full gate (`task` == lint + test, and `task license:check`) ran once at this HEAD and passed;
both gate rows cite that run. Wave 0 files exist and are the tests named above. No manual-only behaviors.
