---
phase: 06-rule-capture-investigation-fix
plan: 03
subsystem: docs
tags: [skill-prose, rule-capture, rule-hygiene, curating-memory, requirements, roadmap]

# Dependency graph
requires:
  - phase: 06-rule-capture-investigation-fix plan 01
    provides: "### Proposing a rule — the trigger and protocol 06-COLD-READ.md exercised"
  - phase: 06-rule-capture-investigation-fix plan 02
    provides: "### Rule hygiene and ### One-time rule backfill sweep — the cadence paragraph this plan extends and the procedure the still-pending live sweep will follow"
provides:
  - "Milestone-completion cadence clause in ### Rule hygiene, citing rule 7smp8vy9hr's verified live content"
  - "06-DEMONSTRATION.md — a scaffold recording that the live backfill sweep is orchestrator-administered, with a marked section for its result"
  - "REQ-rule-capture-intervention closed by citation to 06-COLD-READ.md's PASS verdict"
  - "REQUIREMENTS.md traceability table and ROADMAP.md Phase 6 section/progress row updated to 3/3 Complete"
affects: []

# Actuals (#2632)
actuals:
  tokens: 8850
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Live-MCP-dependent execution steps are reassigned to the orchestrator when the executing environment has no MCP access, rather than simulated or silently skipped — same shape as 06-01 Task 3's D-14 cold-read reassignment, now applied a second time to 06-03's Task 1 live sweep."

key-files:
  created:
    - .planning/phases/06-rule-capture-investigation-fix/06-DEMONSTRATION.md
  modified:
    - skill/engram/skills/curating-memory/SKILL.md
    - .planning/REQUIREMENTS.md
    - .planning/ROADMAP.md

key-decisions:
  - "REQ-rule-capture-intervention closes on 06-COLD-READ.md's PASS, not on a live sweep this environment cannot run — the cold read is independent behavioral evidence the intervention changed agent behavior, and per D-14's own precedent a fresh-subagent cold read is a legitimate acceptance test on its own."
  - "06-DEMONSTRATION.md is written as an explicit scaffold rather than omitted or fabricated: it states plainly what already stands as evidence (the cold read), what is still pending (the live sweep against the three D-03 candidates), and leaves a clearly-marked section for the orchestrator to fill in with real per-candidate rows."
  - "The milestone-completion cadence clause is restored using the orchestrator's live-fetched content for rule 7smp8vy9hr, cited by what the rule actually says (extract-before-delete, one milestone summary, never touch reusable codebase facts) — not broadened into a claim that the rule itself mandates rule-hygiene checks."
  - "Phase 6 and its progress row are marked 3/3 Complete on the strength of all three requirements closing, even though the live sweep (the strongest, live-store form of roadmap criterion 3) remains pending with the orchestrator — an explicit scope decision stated in this plan's objective, not a default assumption."

patterns-established: []

requirements-completed: [REQ-rule-capture-intervention]

coverage:
  - id: D1
    description: "REQ-rule-capture-intervention closed by citation to 06-COLD-READ.md's PASS verdict — a fresh subagent with zero phase context unprompted named the trigger, proposed via the corrected protocol, and stopped at consent."
    requirement: "REQ-rule-capture-intervention"
    verification:
      - kind: manual_procedural
        ref: "06-COLD-READ.md, administered by the orchestrator per D-14 — Verdict: PASS"
        status: pass
    human_judgment: false
  - id: D2
    description: "The milestone-completion cadence clause is restored in ### Rule hygiene, citing rule 7smp8vy9hr's verified live content accurately (extract-before-delete procedure), not paraphrased broader."
    verification:
      - kind: manual_procedural
        ref: "Read of skill/engram/skills/curating-memory/SKILL.md's When to run these checks paragraph against the orchestrator-supplied verified rule content in this plan's objective — performed this session"
        status: pass
    human_judgment: false
  - id: D3
    description: "Every phase-close gate is green on the final tree: task, go vet ./..., license:check, chart:validate, proto:lint, proto:gen/ui:build zero-drift, both git diff --exit-code prohibition gates against the phase base."
    requirement: "REQ-rule-capture-intervention"
    verification:
      - kind: other
        ref: "Task; go vet ./...; task license:check; task chart:validate; task proto:lint; task proto:gen (zero drift vs gen/); task ui:build (zero drift vs web/); git diff --exit-code ad922f27 -- go.mod go.sum; git diff --exit-code ad922f27 -- '*.go' internal/ cmd/ — see Phase-Close Gate Results table"
        status: pass
    human_judgment: false
  - id: D4
    description: "The live backfill sweep against the three D-03 candidates (r3bjakymtz, z4mgz3a4ab, 478rhhmhb0), which doubles as the phase's live-store demonstration of roadmap criterion 3, is orchestrator-administered and not yet run."
    verification: []
    human_judgment: true
    rationale: "This execution environment has no MCP access (no .mcp.json, no engram MCP tool in the tool list), so the live sweep cannot be simulated or self-administered — it is reassigned to the orchestrator per the objective's explicit scope change, same pattern as D-14's cold-read reassignment. A human (Sean, via the orchestrator) must bless or decline each candidate; that consent cannot be automated or pre-judged."

duration: ~25min
completed: 2026-08-02
status: complete
---

# Phase 6 Plan 3: Rule-Capture Close & Milestone-Completion Cadence Summary

**Restored the milestone-completion cadence clause in `### Rule hygiene` citing rule `7smp8vy9hr`'s verified live content, closed `REQ-rule-capture-intervention` by citation to `06-COLD-READ.md`'s PASS, and closed the phase with a full green gate set — the live backfill sweep against the three D-03 candidates is reassigned to the orchestrator as a scaffolded `06-DEMONSTRATION.md`.**

## Performance

- **Duration:** ~25 min
- **Tasks:** 2 (cadence restoration; demonstration scaffold + requirement/roadmap close + phase-close gates)
- **Files modified:** 4 (1 created, 3 modified)

## Accomplishments

- `skill/engram/skills/curating-memory/SKILL.md`'s `### Rule hygiene` cadence paragraph now names a third trigger moment — milestone completion — citing rule `7smp8vy9hr`'s verified live content (extract reusable facts from per-phase lifecycle records first, write one authoritative milestone summary, only then delete the collapsed per-phase records, never touch reusable codebase facts) accurately, without broadening it into a claim the rule itself mandates rule hygiene. This is the third cadence moment 06-02 honestly dropped for lack of live MCP access; the orchestrator supplied the verified content this session.
- `.planning/phases/06-rule-capture-investigation-fix/06-DEMONSTRATION.md` created as an explicit scaffold: states why the live sweep is orchestrator-administered (no MCP access in this execution environment, same D-14 reassignment pattern as 06-01's cold read), what already stands as evidence independent of the sweep (`06-COLD-READ.md`'s PASS), and leaves a clearly-marked baseline/per-candidate-table section for the orchestrator to fill in with the real sweep result.
- `.planning/REQUIREMENTS.md`: `REQ-rule-capture-intervention` flipped `[ ]` → `[x]` with a closure note citing `06-COLD-READ.md`'s PASS as behavioral evidence the intervention changed agent behavior (not merely that prose was edited), and naming that the user-blessed gate (`errRuleImmutable`, 3 hits, unchanged) remains intact. The traceability table row updated `Pending` → `Complete`. All three phase requirements are now closed.
- `.planning/ROADMAP.md`: Phase 6's checklist checkbox, the `06-03-PLAN.md` plan checkbox (rewritten to describe what actually happened — cadence restoration + citation-close + reassigned sweep, not the plan's original "run the sweep live" description), and the progress-table row (`3/3`, `Complete`, `2026-08-01`) all hand-edited and diffed before committing.
- Full phase-close gate set run and green (see table below) — the phase touched no Go code and added no dependency across all three plans.

## Task Commits

1. **Task (cadence restoration): restore the milestone-completion cadence clause** - `3cb3a3bd` (feat)
2. **Task 2: record the demonstration scaffold and close the phase** - `be21fdbd` (docs)

## Files Created/Modified

- `skill/engram/skills/curating-memory/SKILL.md` — `### Rule hygiene`'s cadence paragraph, third trigger moment added
- `.planning/phases/06-rule-capture-investigation-fix/06-DEMONSTRATION.md` — created, scaffold
- `.planning/REQUIREMENTS.md` — `REQ-rule-capture-intervention` checkbox, closure note, traceability row
- `.planning/ROADMAP.md` — Phase 6 checklist checkbox, 06-03 plan checkbox, progress-table row

## Decisions Made

See `key-decisions` in frontmatter.

## Deviations from Plan

**1. [Scope reassignment, pre-authorized by the objective] Task 1's live sweep reassigned to the orchestrator.**
- **Found during:** reading 06-03-PLAN.md's Task 1 (`type="checkpoint:human-verify"`, requires `mcp__engram__list_rules`/`store_rule`/etc.)
- **Issue:** this execution environment has no engram MCP tool available — no `.mcp.json`, no running server, no `mcp__engram__*` tool in the session's tool list (same gap 06-02 hit and documented for D-11/D-15's live-check precondition).
- **Fix:** the orchestrator's objective explicitly reassigned the live sweep to itself (same D-14 pattern as 06-01's cold read) and directed this agent to write `06-DEMONSTRATION.md` as a scaffold rather than simulate or omit it. Not a Rule 1-4 auto-fix — an explicit, pre-authorized scope change stated in the spawning objective.
- **Files modified:** `.planning/phases/06-rule-capture-investigation-fix/06-DEMONSTRATION.md` (created as scaffold, not completed record)
- **Committed in:** `be21fdbd`

**2. [Objective-directed addition] Restored the milestone-completion cadence clause 06-02 dropped.**
- **Found during:** the orchestrator's objective, which supplied rule `7smp8vy9hr`'s verified live content (fetched by the orchestrator, since this environment has no MCP access either) and directed the clause be added.
- **Issue:** 06-02 dropped the third cadence moment because it could not verify the rule's actual content live; `06-CONTEXT.md`'s Established Patterns and `06-RESEARCH.md`'s Assumption A1 both flagged it as unverified.
- **Fix:** added the third trigger moment to `### Rule hygiene`'s cadence paragraph, citing the rule's actual extract-before-delete procedure rather than a broadened paraphrase.
- **Files modified:** `skill/engram/skills/curating-memory/SKILL.md`
- **Committed in:** `3cb3a3bd`

---

**Total deviations:** 2 (both pre-authorized scope changes stated in the spawning objective, not autonomous Rule 1-4 fixes)
**Impact on plan:** The plan's Task 1 (live sweep) did not execute as written; its evidence role is filled by `06-COLD-READ.md` (already landed in 06-01) for requirement closure, with the live sweep itself left as a pending, clearly-scaffolded follow-up. No scope creep — both changes were directed by the spawning objective, not discovered mid-execution.

## Phase-Close Gate Results

All gates run after both of this plan's commits landed, on the final tree.

| Gate | Command | Result |
|------|---------|--------|
| Lint + full suite | `task` | PASS — golangci-lint, rumdl (125 files), yamlfmt, actionlint, ruff check+format (0 issues), pytest (33 passed), `go test ./...` all packages ok |
| Vet | `go vet ./...` | PASS (exit 0) |
| License headers | `task license:check` | PASS — 241 valid, 0 invalid |
| Chart drift + render | `task chart:validate` | PASS — default render omits CronJob, `cronjob.enabled=true` render emits it with `Forbid`/daily schedule, `helm lint` clean, `EXPECTED_CHECKSUM` matches |
| Proto lint | `task proto:lint` | PASS — buf lint clean, no `NO_SIDE_EFFECTS` idempotency level |
| Proto codegen drift | `task proto:gen` then `git diff --exit-code -- gen/` | PASS — zero drift |
| UI build drift | `task ui:build` then `git diff --exit-code -- web/` | PASS — zero drift |
| Zero new Go deps (strongest prohibition gate) | `git diff --exit-code ad922f27 -- go.mod go.sum` | PASS (exit 0) — phase base per `06-01-SUMMARY.md` |
| No Go file changed all phase | `git diff --exit-code ad922f27 -- '*.go' internal/ cmd/` | PASS (exit 0) |
| Skill headings present | `rg -n '^### Proposing a rule$\|^### Rule hygiene$\|^### One-time rule backfill sweep$' skill/engram/skills/curating-memory/SKILL.md` | PASS — 3 lines (weak: existence only) |
| Rule guards intact | `rg -n 'errRuleImmutable' internal/server/tools.go` | PASS — 3 hits, unchanged |

### Manual acceptance criteria

| Criterion | Result |
|-----------|--------|
| Cadence clause cites rule `7smp8vy9hr` accurately (not broadened) | PASS — quotes the extract-before-delete procedure, states the rationale, does not claim the rule mandates rule-hygiene checks itself |
| `.planning/ROADMAP.md` / `.planning/REQUIREMENTS.md` diffs show only intended edits | PASS — confirmed via `git diff` before each commit; no structural churn |
| `REQ-rule-capture-intervention` closure cites real evidence, not an assertion of success | PASS — cites `06-COLD-READ.md`'s PASS verdict and the specific behaviors observed |
| `06-DEMONSTRATION.md` states plainly what it does and does not establish | PASS — explicit "What this scaffold does NOT do" and pending-result sections |

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- **v0.12.x Phase 6 (Rule Capture — Investigation & Fix) is COMPLETE — all 3 plans, all 3 requirements** (`REQ-rule-capture-investigation`, `REQ-rule-capture-intervention`, `REQ-rule-curation-hygiene`).
- **This is also the last phase of v0.12.x.** All six phases of the milestone are now complete: Phase 1 (Shared Auth Chain & Connect Bearer Identity), Phase 2 (Headless CLI Client), Phase 3 (Cross-Spine Memory Recall), Phase 4 (Diagnosability), Phase 5 (Operator Config & Reindex Correctness), Phase 6 (Rule Capture — Investigation & Fix).
- **One open item for the orchestrator:** run the live backfill sweep against the three D-03 candidates (`r3bjakymtz`, `z4mgz3a4ab`, `478rhhmhb0`) per `06-DEMONSTRATION.md`'s scaffold, presenting each proposal to Sean for a per-candidate bless/decline, then fill in the scaffold's result section. This is valuable live-store evidence but does not block the phase or milestone close — `REQ-rule-capture-intervention` and roadmap criterion 3 are already satisfied by `06-COLD-READ.md`.
- v0.12.x's milestone-lifecycle steps (ship/complete-milestone) are the orchestrator's to run next.

---
*Phase: 06-rule-capture-investigation-fix*
*Plan: 03*
*Completed: 2026-08-02*
