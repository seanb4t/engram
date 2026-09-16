---
phase: "5"
slug: "apply-time-preserve-gate-documentation"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-16"
---

# Phase 5 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (no third-party test framework anywhere in this repo) |
| **Config file** | none — `go test` is invoked directly, per `Taskfile.yaml`'s `test:go` task |
| **Quick run command** | `go test ./internal/setup/... ./cmd/engram/... -count=1` |
| **Full suite command** | `task test` (lint + `go test ./...`) |
| **Estimated runtime** | ~60 seconds (quick) / ~180 seconds (full; `internal/store` needs Docker) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/setup/... ./cmd/engram/... -count=1`
- **After every plan wave:** Run `task test`
- **Before `/gsd-verify-work`:** Full suite green; `05-POST-RELEASE.md` exists and names every qualifying-release check (D-06)
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| {N}-01-01 | 01 | 1 | REQ-{XX} | T-{N}-01 / — | {expected secure behavior or "N/A"} | unit | `{command}` | ✅ / ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*(Seeded by plan-phase; gsd-planner fills one row per task from the plans it authors.)*

---

## Wave 0 Requirements

- [ ] `internal/setup/apply_test.go` — new tests through the existing `fakeEnvWithRun`/`scriptedRun` harness: `--apply` on a `preserved`-classifying fixture issues zero registration-write `Run` calls (SC1); Claude Code's `mcp remove` argv never appears among recorded calls (SC2); `already-correct` is the same pre-write zero-write case (D-01); `would-write` still runs `plan.Actions` → `wrote` and a second `Apply` converges; `Registered` after a write is rebuilt via `Observe`/`renderObservation`, never raw `displayCapture` (D-02)
- [ ] `internal/setup/apply_test.go` — EXISTING `TestApplyConvergesCodex`/`TestApplyConvergesClaudeCode` retargeted to D-01 semantics (`already-correct` is now a pre-write claim for drift runtimes)
- [ ] `internal/setup/claudecode_test.go` — a `would-write` Claude Code fixture whose observed auth shape is oauth/oauth-client carries the OAuth re-login note in BOTH `Preview` and `Apply` `Notes` (D-03/D-04); a bearer-shaped fixture does NOT
- [ ] `cmd/engram/setup_test.go` — `--apply` at the process boundary: a `preserved` row still populates plugin/skills facets, zero registration writes, exit 0
- [ ] `cmd/engram/install_docs_test.go` — new `agent_setup_docs_test.go`-shaped gate for `install.md` (man pages `man engram-setup`, cask contents, plugin-first pointer)
- [ ] `cmd/engram/agent_setup_docs_test.go` — extended legs: apply gate stated; corrected `already-correct` semantics (the "does not guarantee no write ran" sentence must be GONE); OAuth re-login consequence; manual remediation
- [ ] `cmd/engram/plugin_docs_test.go` (or equivalent) — gate for `plugin.md`'s cross-link to `agent-setup.md` and plugin-first description
- [ ] `.planning/phases/05-apply-time-preserve-gate-documentation/05-POST-RELEASE.md` — NOT a Go test; the D-06 human-handoff deliverable, shaped like `06-POST-RELEASE.md`

*RED-evidence approach: tests referencing not-yet-existing symbols or asserting the new zero-write behavior against the pre-Phase-5 `mutate` branch fail (compile failure or assertion failure — both valid RED in Go). Run each new test individually with `-run '^Name$' -v` before its GREEN commit (gotcha `bsbsvn4hbc`).*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Post-release live observation: `brew install` of the qualifying release; `engram setup` plugin-first + `--header` + `preserved` + the apply gate on a real machine; man pages present in the cask | REQ-docs-setup-v2 | Rule `m45p2b4bp7` — no test invokes a real CLI or touches `$HOME`; the D-10 pattern records the observation after a release, never from code alone | Follow `05-POST-RELEASE.md` after the next release; record `05-RELEASE-<ver>.md` (shape of `06-RELEASE-0.16.0.md`); flip `post_release_status` in `05-VERIFICATION.md` and check off REQ-docs-setup-v2 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
