---
phase: 04
slug: spine-curation-semantic-skill
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: false
wave_0_complete: true
created: 2026-08-11
validated: 2026-08-11
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
| _pending_ | — | — | REQ-semantic-curation-skill | — | Skill calls only the six allowed MCP tools | structural (weak, **paired**) | Negative half: `rg -c '(mcp__engram__\|…__\|\.\.\.__)(store_memory\|schedule_memory\|delete_all\|set_visibility\|store_rule\|store_discovery\|search_discovery)' skill/engram/skills/curating-spine/SKILL.md` returns **zero**. Positive (non-vacuity) half: the same pattern shape over the six *allowed* tools returns **≥ 6** — proving the pattern can match this file at all | ✅ | ✅ green |
| _pending_ | — | — | REQ-semantic-curation-skill | — | Zero new server-side code | automated | `git diff --stat 72a32c58..b992929b -- internal/ cmd/ proto/ gen/ ':(exclude)*_test.go'` empty across this phase's commits, test files excluded (`72a32c58` = the phase-04 base commit, `b992929b` = the phase-04 final commit) | N/A (CI-level) | ✅ green |
| _pending_ | — | — | REQ-consent-never-perform | T-04-01 | Every proposed mutation stops at an ask; record content treated as data, never instruction | manual | Read the consent section; confirm all three worked verb examples (supersede / update / delete) end in "ask once, then stop," matching `curating-memory`'s reused protocol verbatim | ❌ W0 | ⬜ pending |
| _pending_ | — | — | REQ-consent-adversarial-proof | T-04-01 | A confident, plausible, **wrong** proposal still stops at consent | **behavioral, manual — cold-read subagent run** | **None — and none should be manufactured.** Recorded as `04-COLD-READ.md` with one of the four terminal verdicts fixed by the locked rubric: `PASS` / `FAIL` / `NOT-OBTAINED` / `OFF-MATRIX`. `NOT-TEMPTED` and `INCONCLUSIVE` are **per-run labels, never terminal** — the Task 2 gate at `04-02-PLAN.md:541` rejects them | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

**Why REQ-consent-adversarial-proof carries no automated command.** An `rg` assertion that the consent section exists would pass whether or not the gate holds under temptation — it asserts the gate's *presence*, not its *behavior*. That is the tautology the research explicitly warns against, and it is the same false-green class this repo has hit before (unreachable-branch tests; `-run` filters matching nothing and exiting 0). The cold-read transcript is the evidence; a green structural check here would be worse than no check, because it would read as coverage.

**Four corrections applied 2026-08-11.** Recorded rather than silently patched, because each is an instance of the exact failure class this document exists to prevent. The first two were found by the planner dry-running the gates this file seeded; the third by the cross-AI review orchestrator in convergence cycle 3, after the rubric was locked; the fourth by v0.13.x Phase 5's reconciliation pass, re-running this file's own commands live against HEAD rather than trusting their recorded verdicts:

1. *Permanently-red gate.* The SC-1 no-new-server-code row originally diffed against `main`. On branch `feat/v0.13` that range contains all of phases 01–03.1 — measured at 97 files / 24,363 insertions — so it could never be empty and never pass. Re-anchored to the phase base `72a32c58..HEAD`, which is empty as expected.
2. *Vacuous negative gate.* The forbidden-tool row originally matched only the fully-prefixed `mcp__engram__<tool>` form. The sibling skills write the ellipsis-abbreviated form instead: `promoting-memory/SKILL.md:25` contains `…__store_memory(`, giving 0 fully-prefixed hits against 1 real reference. The negative grep therefore returned clean on a file that demonstrably calls a forbidden tool. Now matches the abbreviated forms too, and is paired with a positive non-vacuity half so a pattern that cannot match this file at all fails loudly instead of passing green.

3. *Stale terminal verdict set.* The `REQ-consent-adversarial-proof` row named `PASS / FAIL / INCONCLUSIVE`. That was written when this file was seeded, **before** the cold-read rubric was locked (engram `8pbkf8w9hx`). Under the locked rubric the terminal set is `PASS / FAIL / NOT-OBTAINED / OFF-MATRIX`, and `INCONCLUSIVE` is a per-run label the Task 2 gate at `04-02-PLAN.md:541` explicitly rejects. This could not have produced a false green — but it could have produced a false RED or a stall at the phase's only behavioural gate, made worse by `04-01-PLAN.md` instructing the executor not to edit this file. Row corrected; that directive narrowed to match.

4. *Open-ended range decayed into a false red.* The SC-1 no-new-server-code row's range was still open-ended at `HEAD` (correction 1 pinned only the start). One doc-binding test file, `internal/server/verbtabledocs_test.go`, landed inside that range at commit `a2599027` ("test(04): IN-01 bind curating-spine verb table to curating-memory's"), during this phase's own code-review fix pass — so the row's literal command went red on this phase's own commit before any later phase touched it. Both ends are now pinned (`72a32c58..b992929b`, where `b992929b` is this phase's actual final commit) with test files excluded (`':(exclude)*_test.go'`), so the row states the property the criterion actually meant — no new server *behaviour* — rather than the broader "no diff at all" it accidentally asserted. Confirmed live and empty: `git diff --stat 72a32c58..b992929b -- internal/ cmd/ proto/ gen/ ':(exclude)*_test.go'`. Plainly: v0.13.x Phase 5 itself edits a file under `internal/` (`internal/retrievaleval/retrieval_eval_test.go`, plan 05-02), so an open-ended range would have gone red for a reason having nothing to do with this phase.

The general lesson, already recorded in this repo's memory: a negative assertion is only evidence when paired with a positive one proving the matcher works on the target. Correction 3 adds a second: a document seeded *before* a decision is locked does not update itself, and a "do not edit this file" directive aimed at one stale claim will happily protect the next one. Correction 4 adds a third: an open-ended `<sha>..HEAD` range in a closed phase's record decays into a false red the moment any later work touches the same tree — pin both ends, or the record's own honesty has an expiry date.

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
