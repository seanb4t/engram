---
phase: "4"
slug: "skills-distribution"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-09"
---

# Phase 4 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go's built-in `testing` package (`go test`) — same as every other package in this repo; no new framework |
| **Config file** | none — `go test ./...` via `Taskfile.yaml`'s `test:go` task |
| **Quick run command** | `go test ./internal/skills/... ./internal/setup/... ./cmd/engram/... -count=1` |
| **Full suite command** | `task` (lint + test) |
| **Estimated runtime** | ~30 seconds (quick) / ~180 seconds (full) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/skills/... ./internal/setup/... ./cmd/engram/... -count=1`
- **After every plan wave:** Run `task`
- **Before `/gsd-verify-work`:** Full suite must be green, plus a confirmation of the Codex
  destination open question (RESEARCH.md § Open Questions 1)
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | 0 | REQ-skills-embedded-in-binary | — | N/A | unit | `go test ./internal/skills/... -count=1 -run TestSkillsEmbedMatchesVendored` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-skills-embedded-in-binary | — | N/A | unit | `go test ./cmd/engram/... -count=1 -run TestSetupPreviewSkillsJSON` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-skills-native-format | — | N/A | unit | `go test ./internal/skills/... -count=1 -run TestInstallNativeConverges` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-skills-native-format | — | N/A | unit | `go test ./internal/skills/... -count=1 -run TestInventoryIsStructural` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-skills-agents-md-fallback | T-04-01 | Hard-fail on any AGENTS.md state other than zero blocks or one well-formed block; name offsets in `Reason`; write zero bytes | unit | `go test ./internal/skills/... -count=1 -run TestAgentsMdSplice` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-skills-agents-md-fallback | T-04-02 | A symlinked AGENTS.md is written THROUGH via in-place `os.WriteFile`, never replaced by stage-and-rename | unit | `go test ./internal/skills/... -count=1 -run TestAgentsMdPreservesSymlink` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Task ID / Plan / Wave columns are filled by the planner at Step 8 and reconciled by
`/gsd-validate-phase`; the Requirement → command mapping above is derived from
`04-RESEARCH.md` § Validation Architecture and is authoritative for coverage.*

---

## Wave 0 Requirements

- [ ] `internal/skills/drift_test.go` — D-02's set-equality-then-byte-equality gate; the single
  highest-value new test in the phase. Must land in the FIRST wave, before any other
  `internal/skills` code, so every subsequent change is drift-checked from the start.
- [ ] `internal/skills/environment_test.go` — the fake filesystem seam every other
  `internal/skills` test builds on.
- [ ] Framework install: none — `go test` is already fully configured for this repo.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Codex actually surfaces a skill written to the chosen user-scope destination | REQ-skills-native-format | Official docs name `$HOME/.agents/skills`; a live machine shows a populated `$CODEX_HOME/skills`. Which one Codex's own selector reads is unresolved, and repo rule `m45p2b4bp7` forbids gating on third-party behavior. | Install skills via `engram setup --apply`, start `codex`, and confirm the five curation skills appear in its skill list. Record which destination was written. |
| Claude Code / opencode tolerate the new `metadata.engram-summary` frontmatter key without warning or dropping the skill | REQ-skills-native-format | Same ownership-boundary rule — this is third-party loader behavior, verifiable but not gateable. | Install skills, start each runtime, confirm all five skills load and no frontmatter warning is emitted. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
