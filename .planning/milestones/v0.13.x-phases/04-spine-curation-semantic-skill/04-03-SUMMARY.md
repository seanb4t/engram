---
phase: 04-spine-curation-semantic-skill
plan: 03
subsystem: skills
tags: [engram, skill, spine-curation, staleness, consent-gate, cold-read]

requires:
  - phase: 04-spine-curation-semantic-skill (04-01)
    provides: "skill/engram/skills/curating-spine/SKILL.md tracer (commit 1cdef4d9): identity axis, consent gate, verb table"
  - phase: 04-spine-curation-semantic-skill (04-02)
    provides: "04-COLD-READ.md adversarial fixture and NOT-OBTAINED terminal verdict, with the user's escalated decision to accept the non-result and let this plan proceed"
provides:
  - "skill/engram/skills/curating-spine/SKILL.md (shipped, 322 lines): staleness axis, cheap-search precedence ladder, error-envelope pointer, reactive-recall trigger, distinct no-re-propose marker convention"
  - ".planning/phases/04-spine-curation-semantic-skill/04-COLD-READ.md — extended with ## Post-expansion read, a behavioural retest of the shipped file scoped to propose-then-stop"
affects: []

actuals:
  tokens: 5207
  tasks: 4
  commits: 3

tech-stack:
  added: []
  patterns:
    - "distinct-pair no-re-propose marker via update_memory tag, resent byte-identical content to satisfy the MCP-lane content requirement (five-step call shape)"
    - "post-expansion behavioural retest reusing an earlier plan's pinned fixture verbatim, scored on a narrower rubric (propose-then-stop dilution) than the original adversarial SC-3 rubric, with the two runs' relationship stated explicitly so neither result is misread as evidence for the other's question"

key-files:
  created: []
  modified:
    - skill/engram/skills/curating-spine/SKILL.md
    - .planning/phases/04-spine-curation-semantic-skill/04-COLD-READ.md

key-decisions:
  - "Combined Task 1 (staleness axis) and Task 2 (cheap-search ladder + error-envelope pointer) into a single commit — both are additive H2 sections placed contiguously between the same two anchors (## Choosing the verb and ## Proposing a mutation), authored in one editing pass before either task's gate was run. Both tasks' independent gates were verified passing against the combined result before committing. Documented under Deviations."
  - "Task 4's cold read reused plan 04-02's Run 1 fixture verbatim (not Run 2 or Run 3), per the plan's explicit instruction, via the same isolated `claude -p --safe-mode --tools \"\" --no-session-persistence` subprocess method 04-02 established (this worktree executor's runtime has no Task-tool subagent spawner)."
  - "Post-expansion run scored NOT DILUTED; the identity verdict (overlapping, correct) was recorded but explicitly excluded from scoring, and the section states in words that this result neither upgrades plan 04-02's NOT-OBTAINED nor would a DILUTED result have been SC-3 evidence."

patterns-established:
  - "A plan resuming after an upstream NOT-OBTAINED/escalation records the user's resolution inline in the escalated artifact (04-COLD-READ.md gained a '## Resolution' line) rather than only in this SUMMARY, so a future reader of that file sees the decision without needing this plan's SUMMARY."

requirements-completed: [REQ-semantic-curation-skill, REQ-consent-never-perform]
# REQ-consent-never-perform (this plan's own frontmatter requirement — every mutation proposed,
# never performed unilaterally) is satisfied within this plan's scope: the consent section this
# plan never touched stays gate-proven byte-identical, and the new distinct-marker mutation Task 3
# introduces routes through the same batch-report propose/ask-once/stop protocol, confirmed
# behaviourally by Task 4's NOT DILUTED post-expansion read.
#
# Distinct from this: 04-02's own requirement REQ-consent-adversarial-proof (the SC-3 adversarial
# property — does the gate hold when the reader's judgment is confidently WRONG) remains open with
# terminal verdict NOT-OBTAINED, per the user's accepted resolution. Task 4 is a narrower
# propose-then-stop retest, explicitly not a second SC-3 attempt, and does not close that
# requirement — it is not in this plan's frontmatter `requirements` field and stays 04-02's to own.

coverage:
  - id: D1
    description: "Staleness axis added to curating-spine/SKILL.md: four spine-review-verify-mirrored tiers (valid/moved/broken/unverifiable), prose-ref extraction for citation-less records, moved-before-broken search discipline, and the honest-unverifiable rule."
    requirement: "REQ-semantic-curation-skill"
    verification:
      - kind: other
        ref: "Task 1 automated gate (tier presence + Gate A/B byte-identity re-run) — commit 9995bdac"
        status: pass
    human_judgment: false
  - id: D2
    description: "Cheap-search precedence ladder (codegraph -> ast-grep/sg -> rg -> Read, every rung optional) and error-envelope pointer to docs-site/errors.md, including the not-owned/nonexistent/ambiguous-short-id indistinguishability rule."
    requirement: "REQ-semantic-curation-skill"
    verification:
      - kind: other
        ref: "Task 2 automated gate (ladder rungs + errors.md pointer + Gate A/B re-run) — commit 9995bdac"
        status: pass
    human_judgment: false
  - id: D3
    description: "Reactive-recall trigger (## Noticing during recall, zero extra tool calls, one-line note only), distinct no-re-propose marker convention (five-step MCP-lane-valid update_memory call, both-records pre-proposal check, re-embed disclosure), and tuned frontmatter description naming both D-02 trigger paths."
    requirement: "REQ-semantic-curation-skill"
    verification:
      - kind: other
        ref: "Task 3 automated gate (full phase gate set: marker convention, field=content hint=required, re-embed wording, proseTargets unchanged at 4 entries, zero server diff, allowed-tool surface both directions, Gate A/B re-run) — commit 7834659f"
        status: pass
    human_judgment: false
  - id: D4
    description: "Post-expansion behavioural cold read against the shipped SKILL.md (commit 7834659f), scored dilution: NOT DILUTED with a supporting consent-stop: observed quote, appended to 04-COLD-READ.md without disturbing plan 04-02's sections or terminal verdict."
    requirement: "REQ-consent-never-perform"
    verification: []
    human_judgment: true
    rationale: "This is the same class of evidence as plan 04-02's own cold read — a behavioural subagent transcript, not a string-matchable test. Per this plan's own Task 4 human-check instruction, a human should confirm the transcript was scored on propose-then-stop only, that the identity verdict was recorded but not used to score, and that the NOT DILUTED result was not written up as SC-3 evidence or an upgrade of plan 04-02's NOT-OBTAINED verdict."

duration: ~50min
completed: 2026-08-11
status: complete
---

# Phase 4 Plan 3: Curating-Spine Skill Expansion Summary

**Expanded `curating-spine/SKILL.md` from 160 to 322 lines with the staleness axis, cheap-search ladder, reactive-recall trigger, and a durable `distinct`-pair marker — all added around the tracer's unchanged consent gate, which a post-expansion cold read against the shipped file confirms still stops at consent (`dilution: NOT DILUTED`).**

## Performance

- **Duration:** ~50 min
- **Completed:** 2026-08-11
- **Tasks:** 4 (Tasks 1-2 combined into one commit; Task 3 and Task 4 each their own commit)
- **Files modified:** 2

## Accomplishments

- Added `## Judging staleness`: the four `spine-review verify`-mirrored tiers (`valid`/`moved`/`broken`/`unverifiable`), prose-ref extraction for citation-less records with checkable-evidence reporting, the moved-before-broken search discipline, and the honest-uncertainty rule — plus the one required sentence acknowledging the skill's `moved` sense is broader than the CLI's same-file sense.
- Added `## Searching cheaply` and `## When a call is rejected`: the codegraph → ast-grep → rg → Read precedence ladder with every rung stated optional and the degraded no-tooling path named explicitly, the exhaust-then-`unverifiable` rule (narrowed from an absolute prohibition per the plan's review disposition), and the errors.md pointer including the not-owned/nonexistent/ambiguous-short-id indistinguishability rule.
- Added `## Noticing during recall` (the D-02/D-03 reactive path): an absolute zero-extra-tool-calls bound, one-line-note-only output, and the explicit statement that only a separate deliberate invocation opens the full flow.
- Extended the identity section's `distinct` paragraph with the durable no-re-propose marker convention: `spine-distinct-<short_id>` tag, the five-step MCP-lane-valid `update_memory` call (byte-identical content resend, tags union, no summary), the re-embed cost disclosure, and the both-records pre-proposal check sourced from `get_memory` rather than the tags-less `consolidate` candidate row.
- Tuned the frontmatter `description` to name the reactive trigger condition alongside the existing deliberate trigger phrases, retaining the scope exclusions.
- Administered a post-expansion cold read against the shipped file (Task 4): `dilution: NOT DILUTED`, `consent-stop: observed`, with the identity verdict recorded (`overlapping`, correct) but explicitly excluded from scoring and explicitly stated to neither upgrade nor invalidate plan 04-02's NOT-OBTAINED verdict.

## Task Commits

1. **Task 1 + Task 2: staleness axis, cheap-search ladder, error-envelope pointer** - `9995bdac` (feat) — combined; see Deviations
2. **Task 3: reactive trigger, distinct marker convention, tune description** - `7834659f` (feat)
3. **Task 4: post-expansion cold read against the shipped file** - `0edeeb40` (test)

_Note: no separate plan-metadata commit — SUMMARY.md is committed by the orchestrator's final
metadata step per this worktree's execution mode (STATE.md/ROADMAP.md excluded)._

## Files Created/Modified

- `skill/engram/skills/curating-spine/SKILL.md` — expanded from 160 to 322 lines: four new/extended sections (`## Judging staleness`, `## Searching cheaply`, `## When a call is rejected`, `## Noticing during recall`), the `distinct` paragraph rewritten with the marker convention, and the frontmatter `description` tuned. The consent section (`## Proposing a mutation`) and the verb table (`## Choosing the verb`) are byte-identical to their pre-expansion state — proven by Gates A and B re-run in every task.
- `.planning/phases/04-spine-curation-semantic-skill/04-COLD-READ.md` — a `## Resolution` line recording the user's 2026-08-11 acceptance of plan 04-02's NOT-OBTAINED escalation, plus a new `## Post-expansion read` section (the shipped-file behavioural retest). Plan 04-02's own sections and terminal `**Verdict:**` line are unchanged.

## Decisions Made

- **Task 1 and Task 2 combined into one commit.** Both add contiguous, additive H2 sections between the same two existing anchors (`## Choosing the verb` and `## Proposing a mutation`), and were authored in a single editing pass before either task's automated gate was run. Both tasks' independent gate scripts were run and verified passing against the combined result (Task 1's tier-presence + drift-check gate; Task 2's ladder-rung + errors.md-pointer + drift-check gate) before staging and committing. This is a commit-granularity deviation from "one task, one commit," not a scope or content deviation — every acceptance criterion from both tasks is met and independently verified.
- **Task 4 reused plan 04-02's Run 1 fixture verbatim**, per the plan's explicit instruction not to author a new fixture, via the same isolated `claude -p --safe-mode --tools "" --no-session-persistence --output-format json` subprocess method plan 04-02 established (documented there as a Rule 3 substitution for a Task-tool subagent this worktree executor's runtime does not expose). Model used: `claude-opus-5`, matching all three of 04-02's runs.
- **The post-expansion run's identity verdict was recorded but not scored**, and the section states explicitly that neither direction of this run's dilution score moves plan 04-02's own NOT-OBTAINED verdict — per the plan's requirement that this task not be written up as a second SC-3 attempt.

## Deviations from Plan

### Auto-fixed Issues

**1. [Commit-granularity, not a Rule 1-4 category] Combined Task 1 and Task 2 into a single commit**
- **Found during:** Task 1 (before the intended separate commit point)
- **Issue:** Both tasks add contiguous H2 sections placed by the same instruction ("between `##
  Choosing the verb` and `## Proposing a mutation`"), and the natural editing unit for one coherent
  read of that span covered both tasks' content before either task's own gate was run in isolation.
- **Fix:** Ran Task 1's full automated gate, then Task 2's full automated gate, both against the
  combined result — both passed independently. Committed once, with a commit message documenting
  both tasks' scope.
- **Files modified:** `skill/engram/skills/curating-spine/SKILL.md`
- **Verification:** Task 1 gate (tier presence, Gate A/B drift check, rumdl) — PASS. Task 2 gate
  (ladder rungs, errors.md pointer, Gate A/B drift check, rumdl) — PASS. Both re-confirmed after
  Task 3's changes landed, in the combined Task 3 gate script.
- **Committed in:** `9995bdac`

---

**Total deviations:** 1 (commit granularity only — no content, scope, or acceptance-criteria deviation)
**Impact on plan:** None on deliverable correctness; every acceptance criterion in both Task 1 and
Task 2 is independently gate-verified. The only effect is that `git log` shows 3 commits for 4
tasks instead of 4.

## Issues Encountered

None. `claude` CLI was available on PATH for the Task 4 subprocess (no repeat of plan 04-02's
tooling substitution needed to be re-justified — the substitution itself was already established
there; this plan simply reused the same invocation).

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `curating-spine/SKILL.md` is fully shipped: both axes (identity and staleness), the cheap-search
  ladder, the reactive trigger, and the `distinct` marker convention are all present and gate-proven
  not to have disturbed the consent gate plan 04-02 tested.
- `REQ-consent-adversarial-proof` (owned by plan 04-02) remains open with terminal verdict
  NOT-OBTAINED, per the user's accepted resolution recorded in `04-COLD-READ.md`. No further action
  on it is implied by this plan; it stays available for a future decision to authorise additional
  adversarial runs on a different fixture axis, per that file's `## Limits` escalation paragraph.
- No blockers for phase completion. `task` (lint + test), `task license:check`, and
  `git diff --stat 72a32c58..HEAD -- internal/ cmd/ proto/ gen/` (empty) all confirmed green across
  the whole phase (plans 04-01 through 04-03), not just this plan's own commits.

---
*Phase: 04-spine-curation-semantic-skill*
*Completed: 2026-08-11*
