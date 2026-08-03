---
phase: 06-rule-capture-investigation-fix
plan: 01
subsystem: docs
tags: [skill-prose, rule-capture, curating-memory, docs-site, claude-md, agent-behavior]

# Dependency graph
requires: []
provides:
  - "### Proposing a rule subsection in curating-memory/SKILL.md — permission stated as a bare imperative, two observable triggers, an inline propose-then-consent protocol, and a category:decision/rule-declined decline record"
  - "Amended skill frontmatter description cueing the two triggers and the store_rule/list_rules load surface"
  - "Reconciled store_rule prose in docs-site/reference/tools.md, pointing to curating-memory instead of duplicating the protocol"
  - "Reconciled Rule tools paragraph in CLAUDE.md carrying the same permission clause"
  - "REQ-rule-capture-investigation closed by citation to 06-CONTEXT.md D-01/D-02"
affects: [06-02-rule-capture-investigation-fix, 06-03-rule-capture-investigation-fix]

# Actuals (#2632)
actuals:
  tokens: 2753
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Notice-then-propose-then-consent shape for agent-initiated actions gated on explicit user blessing — no server-side enforcement, prose is the entire control (see threat_model in 06-01-PLAN.md)"

key-files:
  created: []
  modified:
    - skill/engram/skills/curating-memory/SKILL.md
    - docs-site/src/content/docs/reference/tools.md
    - CLAUDE.md
    - .planning/REQUIREMENTS.md

key-decisions:
  - "The permission clause was removed from the ## Rules paragraph entirely rather than reworded in place (06-RESEARCH.md Pitfall 1) — a permission sandwiched between two prohibitions still reads as a prohibition on a skim pass"
  - "Both D-05 triggers are phrased as conditions observable in the agent's own tool traffic (a search_memory hit resurfacing, normative phrasing about to be written) — never as a belief, closing the D-02 root cause directly"
  - "The normative-phrasing trigger's scope (record content at write time, not conversation/quotes/restated requirements) is stated in the same paragraph as the trigger itself, not a footnote, per Pitfall 2"
  - "The decline record is category:decision, not category:gotcha — filing it as a gotcha would make it re-enumerable by 06-02's backfill sweep and re-proposed, converting the anti-nag mechanism into a nag"
  - "docs-site's store_rule entry now points to curating-memory for the triggers and protocol instead of growing a second copy that could drift from the skill's wording"

patterns-established:
  - "When a prose gate protects a security-relevant invariant with no server-side enforcement, state the permission as the first sentence, in the imperative, before any qualification — house style demonstrated in curating-memory's other subsections (## Routing, ### When not to use cross-spine)"

requirements-completed: [REQ-rule-capture-investigation]

coverage:
  - id: D1
    description: "### Proposing a rule reads as an instruction to notice and offer, not as a list of prohibitions, on a single cold read"
    requirement: "REQ-rule-capture-intervention"
    verification:
      - kind: manual_procedural
        ref: "Task 3's six-check cold read (06-01-PLAN.md) — NOT YET RUN"
        status: unknown
    human_judgment: true
    rationale: "Per 06-CONTEXT.md D-14, this is the phase's actual acceptance test and cannot be self-administered by the agent that wrote the prose. It is reassigned to a fresh subagent spawned by the orchestrator with zero phase context — pending as of this SUMMARY."
  - id: D2
    description: "All three prose surfaces (skill, tool reference, CLAUDE.md) state the same gate: an agent may notice and propose, store_rule is called only after explicit user consent"
    verification:
      - kind: manual_procedural
        ref: "Side-by-side read of SKILL.md:47-121, docs-site/reference/tools.md:351-358, CLAUDE.md:126-135 — performed this session"
        status: pass
    human_judgment: false

duration: ~20min
completed: 2026-08-01
status: complete
---

# Phase 6 Plan 1: Rule Capture Fix (Tasks 1-2) Summary

**Restructured the buried "propose a rule" permission into its own subsection with two observable triggers, an inline consent-gated protocol, and a decline record — then mirrored the corrected gate into the tool reference and CLAUDE.md, and closed the investigation requirement by citation.**

**Task 3 (the cold-read acceptance checkpoint) was NOT executed by this agent.** Per 06-CONTEXT.md D-14 (added after this plan was written), the checkpoint is reassigned to a fresh subagent spawned by the orchestrator with zero phase context — neither this executor nor the orchestrator can self-administer a genuine cold read after having written or reviewed the corrected prose. That check remains open; see "Task 3 — Pending" below.

## Performance

- **Duration:** ~20 min
- **Tasks:** 2 of 3 (Task 3 reassigned per D-14, not executed here)
- **Files modified:** 4
- **Commits:** 2

## Accomplishments

- `skill/engram/skills/curating-memory/SKILL.md`: split the definition/prohibition from the permission in `## Rules`, and added `### Proposing a rule` — opening with a bare-imperative permission, two triggers stated as observable tool-traffic conditions (never a belief), a four-step inline propose-then-consent protocol naming `store_rule` only in the accept branch, a `category:decision`/`rule-declined` decline record, and a short "when not to propose" guardrail list.
- Amended the skill's frontmatter `description` to cue the repeat-hit and normative-phrasing triggers and the `store_rule`/`list_rules` load surface, keeping it a single paragraph.
- `docs-site/src/content/docs/reference/tools.md`'s `## store_rule` entry: restated the buried-permission sentence as two separate, equal facts (agent may notice+propose; `store_rule` called only after consent) and pointed to `curating-memory` for the triggers/protocol instead of duplicating them.
- `CLAUDE.md`'s `Rule tools:` paragraph: added the proposal-on-notice clause it previously omitted entirely (it carried two prohibitions and no permission at all — the purest instance of the defect per D-13).
- `.planning/REQUIREMENTS.md`: closed `REQ-rule-capture-investigation` (`[ ]` → `[x]`) with a citation to `06-CONTEXT.md` D-01/D-02, naming the cause as friction (not mechanical), evidenced by the three D-03 gotcha records. The other two `REQ-rule-*` checkboxes are untouched, per plan instruction — they close in 06-03.

## Task Commits

1. **Task 1: End-to-end "an agent notices a rule candidate and offers it"** - `ff44bdc1` (feat)
2. **Task 2: Reconcile the two other surfaces carrying the same defect, and close the investigation by citation** - `baa8c932` (docs)

## Files Created/Modified

- `skill/engram/skills/curating-memory/SKILL.md` — split `## Rules`, added `### Proposing a rule` (lines 57-121), extended the frontmatter `description`
- `docs-site/src/content/docs/reference/tools.md` — `## store_rule` entry (lines 351-358)
- `CLAUDE.md` — `Rule tools:` paragraph (lines 126-135)
- `.planning/REQUIREMENTS.md` — `REQ-rule-capture-investigation` checkbox and closure citation

## Decisions Made

See `key-decisions` in frontmatter. No decisions departed from the plan's `<discretion_calls>` or `<action>` instructions — both tasks were executed as written.

## Deviations from Plan

None - plan executed exactly as written for Tasks 1 and 2. Task 3 is not a deviation; it is an explicit, pre-authorized reassignment documented in 06-CONTEXT.md D-14 and restated in this agent's objective.

## Verification Run This Session

- `rg -n '^### Proposing a rule$' skill/engram/skills/curating-memory/SKILL.md` — one hit (line 57).
- `store_rule` occurs exactly once inside the `### Proposing a rule` subsection (line 93, step 4's accept branch); the subsection spans lines 57-121 (next `##` heading is `## Tagging` at line 122).
- `rg -n '^- \[x\] \*\*REQ-rule-capture-investigation\*\*' .planning/REQUIREMENTS.md` — one hit; the other two `REQ-rule-*` entries remain `[ ]`.
- `task lint:markdown` — exits 0 (125 files, no issues) — both after Task 1 and after Task 2.
- `task` (full lint + test suite: actionlint, yamlfmt, golangci-lint, rumdl, ruff check/format, pytest hooks suite, `go test ./...`) — exits 0, all green.
- `git diff --exit-code -- '*.go' go.mod go.sum` — exits 0 (clean) against the phase base (`ad922f27`) — confirmed after each task and again after both commits.
- `rg -n 'errRuleImmutable' internal/server/tools.go` — three hits, unchanged (`updateMemory`'s un-share guard, `setVisibility`, `supersedeMemory`) — confirms the Go-side rule guards this plan relies on were not touched.
- Manual side-by-side read of `SKILL.md`'s `## Rules`/`### Proposing a rule`, `docs-site`'s `## store_rule`, and `CLAUDE.md`'s `Rule tools:` paragraph: all three state the same gate — an agent may notice and propose, `store_rule` is called only after the user says yes, never unilaterally. None of the three, read alone, would leave a reader believing an agent must never raise the subject.
- Manual read confirming the three D-03 candidates (`r3bjakymtz`, `z4mgz3a4ab`, `478rhhmhb0` — all NEVER/ALWAYS-phrased gotchas) would each trip the normative-phrasing trigger as written.

## Task 3 — Pending (per D-14, reassigned to orchestrator)

Task 3 (`type="checkpoint:human-verify"`, the six-check cold read of `### Proposing a rule` against the D-01/D-02 defect) was **not executed by this agent**. 06-CONTEXT.md D-14, added after this plan was written, routes that check to a fresh subagent spawned by the orchestrator with zero phase context — the only way to get a genuine cold read on prose this agent (and the orchestrator, having produced the phase context) helped write. This SUMMARY documents Tasks 1-2 only; the orchestrator is expected to administer Task 3's cold read separately and record its six per-check answers (06-01-PLAN.md's `<output>` instruction: "Record the cold-read checkpoint's actual answers per check, not a summary verdict — 06-03's demonstration record cites them") before treating `REQ-rule-capture-intervention` as satisfied or advancing to 06-02.

No STATE.md/ROADMAP.md plan-completion advancement was performed by this agent for that reason — the plan's checkpoint gate remains open.

## Next Phase Readiness

- Tasks 1-2's prose changes are committed, lint-clean, and verified to touch no Go code.
- 06-02 (rule hygiene) and 06-03 (demonstration + remaining REQ closures) both depend on Task 3's cold read landing successfully — a negative result there means restructuring `### Proposing a rule`, not proceeding.
- Blocker for the orchestrator: administer Task 3's cold read via a fresh, phase-context-free subagent per D-14, then resume the plan-completion state updates (STATE.md, ROADMAP.md via `roadmap.update-plan-progress`, and the final metadata commit) that this agent deliberately did not run.

---
*Phase: 06-rule-capture-investigation-fix*
*Plan: 01 (Tasks 1-2 of 3)*
*Completed: 2026-08-01*
