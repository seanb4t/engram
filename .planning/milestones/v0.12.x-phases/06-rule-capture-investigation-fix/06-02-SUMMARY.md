---
phase: 06-rule-capture-investigation-fix
plan: 02
subsystem: docs
tags: [skill-prose, rule-hygiene, curating-memory, docs-site, rule-capture]

# Dependency graph
requires:
  - phase: 06-rule-capture-investigation-fix plan 01
    provides: "### Proposing a rule subsection — the trigger, protocol, and rule-declined decline record this plan's correction table and backfill sweep both reuse by reference"
provides:
  - "### Rule hygiene subsection in curating-memory/SKILL.md — cadence, honestly-priced (D-11/D-15) contradiction check, a code-verified four-row correction table (D-09/D-09a), D-10's user-blessed deletion gate stated in the same sentence as delete_memory, and a Keep/Merge/Flag/Retire disposition vocabulary"
  - "### One-time rule backfill sweep subsection — D-12's five-step procedure reusing the inline trigger's normative-phrasing test and proposal protocol, excluding rule-declined records, keeping source-record deletion as a separate consent, and rejecting batched consent"
  - "Mirrored hygiene invariants in docs-site/reference/tools.md's store_rule, list_rules, and delete_memory entries"
  - "REQ-rule-curation-hygiene closed"
affects: [06-03-rule-capture-investigation-fix]

# Actuals (#2632)
actuals:
  tokens: 2641
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Rule correction is dispatched by what changed, not treated as a single delete-then-re-store shape: update_memory for same-constraint rewording, store_rule(id=...) for a full-text rewrite that must keep its short_id, delete_memory+store_rule only for retiring/reversing the constraint itself"
    - "Cost of a full-text discipline check is priced as one bulk call (list_rules(full=true)), gated behind named trigger moments, rather than an unconditional per-session or per-record fetch"

key-files:
  created: []
  modified:
    - skill/engram/skills/curating-memory/SKILL.md
    - docs-site/src/content/docs/reference/tools.md
    - .planning/REQUIREMENTS.md

key-decisions:
  - "D-09's decision_conflict block was followed verbatim: the correction table has four rows (refine/replace-in-place/retire/cannot-unshare), not two, because store_rule(id=...) and update_memory both offer in-place paths D-09's original wording didn't account for. Collapsing to just 'retire' would have contradicted the code (rules.go:103-146, tools.go:1501-1513)."
  - "The cadence paragraph names only two trigger moments (about to bless a new rule; list_rules' curation-smell advisory fires), not the three the plan's action text describes. The third — hooking onto rule 7smp8vy9hr's milestone-completion checkpoint — requires reading that rule's live content via mcp__engram__get_memory, and no engram MCP tool was available in this execution environment (confirmed: no .mcp.json, no running server, no mcp__engram__* tool in the available tool list). 06-RESEARCH.md's Assumption A1 flags this same rule's content as never independently re-read. Per the plan's own explicit fallback instruction ('if the rule's actual text does not establish such a cadence, drop the milestone-completion clause... Do not cite a cadence you have not read'), the milestone-completion clause was dropped rather than cited unverified. This is not a deviation — it is the plan's own authorized fallback path, taken because the live-check precondition could not be satisfied."
  - "The disposition vocabulary is four-way (Keep/Merge/Flag/Retire), following promoting-memory's three-way idiom (Promote/Keep/Drop) extended by one entry to cover the contradiction case (Flag), which promoting-memory's shape has no analogue for."
  - "docs-site's three mirrored sections stay reference-shaped (facts and constraints) rather than restating the skill's procedure, per the task's own instruction — no table rows were added or removed, only prose beneath existing tables."

patterns-established:
  - "A 'when NOT enough' pricing paragraph (index carries no content; full-text needs an explicit full=true call) is now a second precedent alongside cross-spine's 'when not to' subsection for stating a capability's real cost next to its convenience."

requirements-completed: [REQ-rule-curation-hygiene]

coverage:
  - id: D1
    description: "### Rule hygiene distinguishes duplicates (catchable from summaries), contradictions (not catchable from summaries, needs full text), and rot (checked against the current tree), each with a disposition"
    requirement: "REQ-rule-curation-hygiene"
    verification:
      - kind: manual_procedural
        ref: "Read of skill/engram/skills/curating-memory/SKILL.md:122-193 against 06-RESEARCH.md Pitfalls 3-4 — performed this session"
        status: pass
    human_judgment: false
  - id: D2
    description: "Correction table's four rows (refine/replace-in-place/retire/cannot-unshare) each match the code they cite"
    requirement: "REQ-rule-curation-hygiene"
    verification:
      - kind: unit
        ref: "rg -n 'errRuleImmutable' internal/server/tools.go — 3 hits, unchanged"
        status: pass
      - kind: manual_procedural
        ref: "Read of internal/server/rules.go:103-146 (store_rule id-replace, short_id carry-forward) and tools.go:1501-1513 (update_memory rule guard) against the table's first two rows — performed this session"
        status: pass
    human_judgment: false
  - id: D3
    description: "D-10's user-blessed deletion gate is in the same sentence as delete_memory; no history survives a rule deletion is stated explicitly"
    requirement: "REQ-rule-curation-hygiene"
    verification:
      - kind: manual_procedural
        ref: "Read of SKILL.md:175-186 — 'call `delete_memory` only after the user has explicitly blessed it' is one sentence; 'No history survives a rule deletion' is its own bolded lead — performed this session"
        status: pass
    human_judgment: false
  - id: D4
    description: "The contradiction check is priced as one list_rules(full=true) call gated behind named moments, not a get_memory per rule"
    requirement: "REQ-rule-curation-hygiene"
    verification:
      - kind: manual_procedural
        ref: "Read of SKILL.md:149-155 — names 'one list_rules call with full: true', explicitly contrasts with 'not a get_memory per rule' — performed this session"
        status: pass
    human_judgment: false
  - id: D5
    description: "### One-time rule backfill sweep reuses the inline trigger's test and protocol by reference, excludes rule-declined records, keeps source-record deletion as a separate consent, and rejects batched consent"
    requirement: "REQ-rule-curation-hygiene"
    verification:
      - kind: unit
        ref: "rg -n '^\\| `categories` \\| string' docs-site/src/content/docs/reference/tools.md — 2 hits"
        status: pass
      - kind: manual_procedural
        ref: "Read of SKILL.md:195-240, plus the D-03 candidate trace below — performed this session"
        status: pass
    human_judgment: false
  - id: D6
    description: "Tool reference (store_rule/list_rules/delete_memory) mirrors the hygiene invariants without duplicating the discipline; no argument-table rows changed"
    requirement: "REQ-rule-curation-hygiene"
    verification:
      - kind: other
        ref: "git diff --stat -- docs-site/src/content/docs/reference/tools.md, read in full — 19 insertions/5 deletions, confined to prose beneath existing tables"
        status: pass
    human_judgment: false

duration: ~35min
completed: 2026-08-01
status: complete
---

# Phase 6 Plan 2: Rule Curation Hygiene Summary

**Added `### Rule hygiene` (duplicates/contradictions/rot with a code-verified four-row correction table and D-10's user-blessed deletion gate) and `### One-time rule backfill sweep` (D-12's reuse-the-trigger procedure) to `curating-memory/SKILL.md`, then mirrored the same invariants into the tool reference.**

## Performance

- **Duration:** ~35 min
- **Tasks:** 3 of 3
- **Files modified:** 3 (2 shipped + REQUIREMENTS.md)
- **Commits:** 3 task commits (this metadata commit is the 4th)

## Accomplishments

- `skill/engram/skills/curating-memory/SKILL.md`: added `### Rule hygiene` immediately after `### Proposing a rule` — names why a rotted rule is worse than a rotted memory, states the cadence (two named trigger moments, not every session), prices the index honestly (D-11: `ruleView` carries no `content`; duplicates are free from summaries, contradictions and rot are not), prices the real check as one `list_rules(full=true)` call rather than a `get_memory` per rule (D-15), states the curation-smell advisory's volume-only limits in the same paragraph it's introduced, gives a code-verified four-row correction table honoring the plan's `<decision_conflict>` block (refine via `update_memory`, replace-in-place via `store_rule(id=...)`, retire via delete-then-re-store, cannot-unshare — none), states D-10's user-blessed deletion gate in the same sentence as `delete_memory`, states that no history survives a rule deletion, and closes with a Keep/Merge/Flag/Retire disposition vocabulary.
- Added `### One-time rule backfill sweep` immediately after — D-12's five-step procedure (confirm scopes → enumerate `category: gotcha` records via `list_memory(categories=[gotcha], full=true)`, paging on `cursor`, skipping `rule-declined` records → apply `### Proposing a rule`'s normative test by reference → decide per candidate, bless/decline, with source-record deletion as its own separate consent question → report a summary), closing with an explicit rejection of batched consent.
- `docs-site/src/content/docs/reference/tools.md`: mirrored the invariants into `store_rule` (short_id preservation on replace; supersede and un-share both closed, so retire means delete), `list_rules` (compact `ruleView` carries no `content`; advisory's volume-only limits in the same paragraph), and `delete_memory` (rule deletion is server-permitted, instruction-gated only, never enforced) — prose only, no argument-table rows touched.
- `.planning/REQUIREMENTS.md`: closed `REQ-rule-curation-hygiene` (`[ ]` → `[x]`) with a citation to this SUMMARY, and updated its traceability-table row from Pending to Complete.

## Task Commits

1. **Task 1: Add `### Rule hygiene`** - `f5cc26fd` (feat)
2. **Task 2: Add `### One-time rule backfill sweep`** - `fcab5ff5` (feat)
3. **Task 3: Mirror the hygiene invariants into the tool reference** - `4ac8ad74` (docs)

## Files Created/Modified

- `skill/engram/skills/curating-memory/SKILL.md` — `### Rule hygiene` (lines 122-193), `### One-time rule backfill sweep` (lines 195-240)
- `docs-site/src/content/docs/reference/tools.md` — `store_rule` prose (short_id preservation + closed paths), `list_rules` prose (no-content note + advisory limits), `delete_memory` prose (rule-deletion permitted, instruction-gated)
- `.planning/REQUIREMENTS.md` — `REQ-rule-curation-hygiene` checkbox, closure citation, traceability-table status

## Decisions Made

See `key-decisions` in frontmatter. The one departure from the plan's literal action text is documented there: the cadence paragraph cites two trigger moments instead of the described three, because the third (rule `7smp8vy9hr`'s milestone-completion cadence) required a live `mcp__engram__get_memory` call this execution environment could not make (no engram MCP tool available — confirmed by absence of `.mcp.json`, no running `engram serve` process, and no `mcp__engram__*` tool in this session's tool list). The plan's own `<read_first>` instruction explicitly authorizes this exact fallback when the live check cannot be performed, so this is not a deviation from the plan — it is the plan's specified contingency, taken because its precondition (live MCP access) was unmet.

## Deviations from Plan

None (Rules 1-4) — the two-moment cadence is the plan's own authorized fallback, not an unplanned fix; see Decisions Made above.

## D-03 Candidate Trace (Task 2's final manual criterion — for 06-03's comparison)

Per the plan's `<output>` instruction, tracing the three D-03 candidates through `### One-time
rule backfill sweep` as written, on paper:

| Candidate | Category | Content (as characterized in 06-CONTEXT.md D-03) | Step 2 (enumerated?) | Step 3 (passes normative test?) |
|-----------|----------|---------------------------------------------------|----------------------|----------------------------------|
| `r3bjakymtz` | gotcha | "NEVER run two GSD executor agents concurrently in the SAME working directory" | Yes — `categories: [gotcha]` matches; not tagged `rule-declined` | Yes — contains NEVER |
| `z4mgz3a4ab` | gotcha | "ALWAYS `git diff .planning/ROADMAP.md` and hand-correct before committing" | Yes | Yes — contains ALWAYS |
| `478rhhmhb0` | gotcha | "gate store tests on `--- PASS:`, never exit status" | Yes | Yes — bare imperative ("gate store tests...") reinforced by "never" |

All three are `category: gotcha` (per D-03), so step 2's `list_memory(categories=[gotcha],
full=true)` enumeration surfaces all three; none carry a `rule-declined` tag (no decline record for
any of them exists in the store as of this session), so none are skipped by step 2's exclusion.
All three pass step 3's normative-phrasing test on their content alone. All three reach step 4 as
live candidates for bless/decline — the actual disposition (user says yes/no per candidate) is
06-03's job to execute for real, not this plan's.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `### Rule hygiene` and `### One-time rule backfill sweep` are committed, lint-clean (`task
  lint:markdown` exits 0), and verified to touch no Go code (`git diff --exit-code -- '*.go'
  go.mod go.sum` clean against phase base `ad922f27`).
- `task` (full lint + test suite) exits 0.
- `REQ-rule-curation-hygiene` is closed. `REQ-rule-capture-intervention` remains open per 06-01's
  SUMMARY — it closes in 06-03, citing 06-COLD-READ.md's PASS verdict.
- 06-03 can execute the backfill sweep for real against the three D-03 candidates traced above,
  which doubles as the phase's criterion-3 demonstration.
- One open note for 06-03 or a future session: rule `7smp8vy9hr`'s actual content (does it really
  establish a milestone-completion curation cadence, per 06-CONTEXT.md's Established Patterns and
  06-RESEARCH.md's unverified Assumption A1) was still not independently read in this session — no
  engram MCP tool access. Whoever next has live MCP access should `get_memory(7smp8vy9hr)` once,
  if hooking rule hygiene onto that cadence is still wanted.

## Self-Check: PASSED

- `skill/engram/skills/curating-memory/SKILL.md` — FOUND
- `docs-site/src/content/docs/reference/tools.md` — FOUND
- `.planning/phases/06-rule-capture-investigation-fix/06-02-SUMMARY.md` — FOUND
- Commit `f5cc26fd` (Task 1) — FOUND
- Commit `fcab5ff5` (Task 2) — FOUND
- Commit `4ac8ad74` (Task 3) — FOUND

---
*Phase: 06-rule-capture-investigation-fix*
*Plan: 02*
*Completed: 2026-08-01*
