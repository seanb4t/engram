---
phase: "4"
slug: "skills-distribution"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-09"
validated: "2026-09-12"
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
| 4-01-01 | 01 | 1 | REQ-skills-embedded-in-binary | — | Embedded tree is byte-identical to the plugin's shipped skills (D-02 drift gate) | unit | `go test ./internal/skills/ -run 'TestSkillsEmbedMatchesVendored\|TestInventoryIsStructural\|TestInventoryIncludesUnderscoreAndDotPrefixedContent\|TestInventoryIsDeterministic' -count=1` | ✅ | ✅ green |
| 4-01-02 | 01 | 1 | REQ-skills-native-format | — | Native install converges; differing destination overwritten; failures accumulate per skill | unit | `go test ./internal/skills/ -run 'TestInstallNativeConverges\|TestInstallOverwritesDifferingDestination\|TestInstallAccumulatesFailuresAndAttemptsEverySkill' -count=1` | ✅ | ✅ green |
| 4-01-03 | 01 | 1 | REQ-skills-native-format | — | Skills outcome aggregates exhaustively; failure reaches partial exit; report omits content in text lane | unit+integration | `go test ./internal/setup/ ./cmd/engram/ ./internal/skills/ -run 'TestAggregateOutcomeExhaustive\|TestAggregatePrecedenceIsAuthoredNotDerived\|TestSkillsOutcomeExhaustive\|TestSetupSkillsFailureReachesPartialExit\|TestSetupPreviewSkillsJSON\|TestSetupTextRowOmitsSkillContent\|TestSetupPackageIsStdlibOnlyLeaf\|TestSkillsPackageImportsAreGated' -count=1` | ✅ | ✅ green |
| 4-02-01 | 02 | 2 | REQ-skills-embedded-in-binary | — | Every skill carries an index summary; frontmatter parses; import gate holds | unit | `go test ./internal/skills/ -run 'TestParseFrontmatter\|TestEverySkillCarriesIndexSummary\|TestSkillsPackageImportsAreGated' -count=1` | ✅ | ✅ green |
| 4-02-02 | 02 | 2 | REQ-skills-agents-md-fallback | T-04-01 | Hard-fail on any AGENTS.md state other than zero blocks or one well-formed block; zero bytes written | unit | `go test ./internal/skills/ -run 'TestAgentsMdSplice\|TestRenderBlockIsAnIndexNotABody' -count=1` | ✅ | ✅ green |
| 4-02-03 | 02 | 2 | REQ-skills-agents-md-fallback | T-04-02 | Re-run replaces the block; symlinked AGENTS.md written through in place | unit+integration | `go test ./internal/skills/ -run 'TestInstallAgentsMdConverges\|TestAgentsMdPreservesSymlink' -count=1` | ✅ | ✅ green |
| 4-03-02 | 03 | 3 | REQ-skills-native-format | — | Codex/opencode SkillTargets are user-scope, never `$CODEX_HOME`; plan fails when HOME is unresolvable; every runtime authors an explicit format | unit | `go test ./internal/setup/ -run 'TestCodexSkillTarget\|TestCodexSkillTargetIsNotCodexHome\|TestCodexPlanFailsWhenHomeUnresolvable\|TestOpenCodeSkillTarget\|TestOpenCodePlanFailsWhenHomeUnresolvable\|TestEveryRuntimeAuthorsAnExplicitSkillFormat' -count=1` | ✅ | ✅ green |
| 4-04-01 | 04 | 4 | REQ-skills-embedded-in-binary | — | Generic carries the full payload with no destination; would-write in both lanes | unit+integration | `go test ./internal/setup/ ./cmd/engram/ -run 'TestGenericSkillTargetHasNoDestination\|TestGenericSkillsOutcomeIsWouldWriteInBothLanes\|TestSetupReportCoversEveryRuntimeShape' -count=1` | ✅ | ✅ green |
| 4-04-02 | 04 | 4 | REQ-setup-correct-by-reading | — | `--help` names skills installation and every runtime/auth mode | unit | `go test ./cmd/engram/ -run 'TestSetupHelpNamesSkillsInstallation\|TestDestructiveCommandsExactFlagSet\|TestSetupHelpNamesEveryRuntimeAndAuthMode' -count=1` | ✅ | ✅ green |
| 4-05-01 | 05 | 5 | REQ-skills-agents-md-fallback | T-04-03 | Only `fs.ErrNotExist` is the create case; any other index read error preserves AGENTS.md with zero writes (B01 / #559) | unit | `go test ./internal/skills/ -run 'TestInstallPreservesIndexOnReadError\|TestInstallAgentsMdConverges' -count=1` | ✅ | ✅ green |
| 4-05-02 | 05 | 5 | REQ-skills-native-format | — | Index-read failure reaches the codex row and `exitPartial`; claude-code row and native skill writes unaffected | integration | `go test ./cmd/engram/ -run 'TestSetupIndexReadFailureReachesPartialExit' -count=1` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Reconciled by `/gsd-validate-phase` on 2026-09-12 from 04-01..04-05 PLAN/SUMMARY coverage
blocks. 04-03 Tasks 1 and 3 were `checkpoint:decision` / `checkpoint:human-verify` gates with
no automated row by design (see Manual-Only). 04-05 is the B01 / #559 gap closure.*

---

## Wave 0 Requirements

- [x] `internal/skills/drift_test.go` — D-02's set-equality-then-byte-equality gate; landed in
  wave 1 (`TestSkillsEmbedMatchesVendored`) before any other `internal/skills` code.
- [x] `internal/skills/environment_test.go` — the fake filesystem seam every other
  `internal/skills` test builds on.
- [x] Framework install: none — `go test` is already fully configured for this repo.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Codex actually surfaces a skill written to the chosen user-scope destination | REQ-skills-native-format | Official docs name `$HOME/.agents/skills`; a live machine shows a populated `$CODEX_HOME/skills`. Which one Codex's own selector reads is unresolved, and repo rule `m45p2b4bp7` forbids gating on third-party behavior. | Install skills via `engram setup --apply`, start `codex`, and confirm the five curation skills appear in its skill list. Record which destination was written. **Verified 2026-09-10 (Sean):** the five curation skills appear in codex's skill list ("skills are there in codex") — RESEARCH assumption A1 is CONFIRMED, not falsified. Codex's own skill selector reads `$HOME/.agents/skills`; the locked destination and the `codex-native-plus-index` routing stand. |
| Claude Code / opencode tolerate the new `metadata.engram-summary` frontmatter key without warning or dropping the skill | REQ-skills-native-format | Same ownership-boundary rule — this is third-party loader behavior, verifiable but not gateable. | Install skills, start each runtime, confirm all five skills load and no frontmatter warning is emitted. **Verified 2026-09-10 (Sean) — all three runtimes tolerate the key.** codex: the five skills loaded and were listed. opencode: confirmed working by Sean. Claude Code: the five loose skills appeared in a live session's skill list alongside the plugin's `engram:*` entries, namespaced separately — the plugin-plus-loose duplicate case research predicted. In every case the unrecognized `metadata.engram-summary` key did not cause the skill to be rejected or dropped. Warning *output* was not separately captured on any runtime; what is established is non-rejection, which is the property REQ-skills-native-format depends on. |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-09-12 by `/gsd-validate-phase 4`

---

## Validation Audit 2026-09-12

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

All 3 Phase 4 requirements are COVERED by committed tests, including the two 04-05 regressions
that pin the milestone-audit blocker B01 (#559); every SUMMARY-referenced test function exists
and `go test ./internal/skills/ ./internal/setup/ ./cmd/engram/ -count=1` is green at
`e9cf19dd`. The two Manual-Only rows are retained with their 2026-09-10 human observations —
both assert third-party skill-loader behavior, which rule `m45p2b4bp7` keeps out of automated
tests. The narrower "emits no warning" half of the second row remains unobserved and is
tracked as milestone-audit tech debt, not as a coverage gap.
