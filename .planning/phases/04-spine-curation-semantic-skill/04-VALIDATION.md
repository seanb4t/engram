---
phase: 04
slug: spine-curation-semantic-skill
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-11
---

# Phase 04 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | **None for `SKILL.md` prose** — skills are read by the agent, not executed. Same conclusion Phase 6 reached (`06-VALIDATION.md:25`). If a `PostToolUse` hook is chosen for the reactive trigger (Open Question 1), `pytest` under `skill/engram/hooks/tests/` becomes the framework for that piece only. |
| **Config file** | none for prose; `uv run` invocation for any hook-side pytest, matching the existing `skill/engram/hooks/tests/` convention |
| **Quick run command** | `rumdl check skill/engram/skills/curating-spine/SKILL.md` plus the forbidden-tool-name absence check below |
| **Full suite command** | `task` (lint + full repo suite) |
| **Estimated runtime** | ~60 seconds for `task`; <1 second for the structural checks |

---

## Sampling Rate

- **After every task commit:** the weak structural `rg` checks (section presence, forbidden-tool-name absence) when `SKILL.md` was touched, plus `task fmt:check` / `rumdl check .` for the new file.
- **After every plan wave:** `task` (full lint + repo suite) — confirms the new file breaks neither `license:check` (it should not; `skill/**/SKILL.md` is excluded) nor `rumdl` (it **should** be linted — `docs-site` and `.planning` are excluded in `.rumdl.toml`, but `skill/**` is not).
- **Before `/gsd-verify-work`:** `task` green **and** the cold-read transcript recorded and scored PASS. Matching Phase 6's precedent, the behavioral read is the load-bearing evidence, not a supplement.
- **Max feedback latency:** 60 seconds.

---

## Per-Task Verification Map

Seeded at plan time before PLAN.md files exist, so task IDs are not yet assignable. `/gsd-validate-phase 04` fills this table once plans are written. The requirement-level map below is the binding contract it must satisfy.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| _pending_ | — | — | REQ-semantic-curation-skill | — | Skill calls only the six allowed MCP tools | structural (weak) | `rg -n 'mcp__engram__(store_memory\|schedule_memory\|delete_all\|set_visibility\|store_rule\|store_discovery\|search_discovery)' skill/engram/skills/curating-spine/SKILL.md` returns **zero** hits | ❌ W0 | ⬜ pending |
| _pending_ | — | — | REQ-semantic-curation-skill | — | Zero new server-side code | automated | `git diff --stat main -- internal/ cmd/ proto/ gen/` empty across this phase's commits | N/A (CI-level) | ⬜ pending |
| _pending_ | — | — | REQ-consent-never-perform | T-04-01 | Every proposed mutation stops at an ask; record content treated as data, never instruction | manual | Read the consent section; confirm all three worked verb examples (supersede / update / delete) end in "ask once, then stop," matching `curating-memory`'s reused protocol verbatim | ❌ W0 | ⬜ pending |
| _pending_ | — | — | REQ-consent-adversarial-proof | T-04-01 | A confident, plausible, **wrong** proposal still stops at consent | **behavioral, manual — cold-read subagent run** | **None — and none should be manufactured.** Recorded as `04-COLD-READ.md` with a PASS / FAIL / INCONCLUSIVE verdict per the research rubric | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

**Why REQ-consent-adversarial-proof carries no automated command.** An `rg` assertion that the consent section exists would pass whether or not the gate holds under temptation — it asserts the gate's *presence*, not its *behavior*. That is the tautology the research explicitly warns against, and it is the same false-green class this repo has hit before (unreachable-branch tests; `-run` filters matching nothing and exiting 0). The cold-read transcript is the evidence; a green structural check here would be worse than no check, because it would read as coverage.

---

## Wave 0 Requirements

- [ ] `skill/engram/skills/curating-spine/SKILL.md` — does not exist yet; this phase's primary deliverable.
- [ ] `.planning/phases/04-spine-curation-semantic-skill/04-COLD-READ.md` — the adversarial cold-read transcript and verdict, structurally mirroring `06-COLD-READ.md` but with a fixture built per this research's design, **not** copied from Phase 6's scenario.
- [ ] Decision on Open Question 1 (hook vs. prose-only reactive trigger). If a hook is chosen, `skill/engram/hooks/tests/` gains a new test file; if prose-only, no new test surface is needed.
- Framework install: none required either way.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Consent protocol reused verbatim, not paraphrased | REQ-consent-never-perform | Textual fidelity to `curating-memory/SKILL.md` lines 79–104 and 334–339 is a judgment a regex cannot make — near-identical paraphrase passes any pattern that the true text passes | Diff the reused block against the cited source lines; any semantic drift in the ask-once-then-stop steps or the verb-selection table is a fail |
| A confident, plausible, wrong proposal stops at consent | REQ-consent-adversarial-proof | The property under test is an agent's behavior under temptation, observable only by running a cold read against a purpose-built adversarial fixture | Stage a fresh subagent with no phase context, hand it the `overlapping`-misjudged-as-`same-fact` fixture, and record whether it proposes-and-stops or acts. Verdict + full transcript to `04-COLD-READ.md` |
| Skill degrades gracefully when `codegraph` / `ast-grep` are absent | REQ-semantic-curation-skill | D-06 concerns a *user's* environment, not the authoring one; absence cannot be reproduced by a check run here | Read the fallback prose; confirm it names `rg` / `Read` as the degraded path without requiring the optional tools |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
