---
phase: 21-ci-maintenance-hygiene
plan: 01
subsystem: ci
tags: [rumdl, lint, markdown, taskfile, roadmap, requirements]

# Dependency graph
requires: []
provides:
  - "`.rumdl.toml` excludes `.planning` — `task lint:markdown` and `task` default are unblocked"
  - "Corrected Phase 21 SC2 (real IN-01 finding) and SC3 (no stale count, plain exclude form) in ROADMAP.md and REQUIREMENTS.md"
affects: [21-02-phase-11-residuals, 21-03-renovate-self-heal]

# Tech tracking
tech-stack:
  added: []
  patterns: ["rumdl exclude entries are plain directory names with a same-line why-comment, appended in chronological order — never alphabetized, never globbed"]

key-files:
  created: []
  modified:
    - .rumdl.toml
    - .planning/ROADMAP.md
    - .planning/REQUIREMENTS.md

key-decisions:
  - "D-09: plain `.planning` exclude entry (not `.planning/**` glob), matching the convention of existing neighbors (.beads, .agents, docs-site)"
  - "D-00b/D-12: SC3's stale '331-failure' figure replaced with qualitative language ('systemic planning-doc noise') rather than a new hardcoded count, since the real count drifts daily"

patterns-established: []

requirements-completed: [REQ-lint-planning-exclude]

coverage:
  - id: D1
    description: "`.rumdl.toml` excludes `.planning`, unblocking `task lint:markdown` (and therefore `task` default)"
    requirement: "REQ-lint-planning-exclude"
    verification:
      - kind: other
        ref: "task lint:markdown (rumdl check .) exits 0"
        status: pass
    human_judgment: false
  - id: D2
    description: "Shipped Markdown outside `.planning/` remains linted (exclude did not overreach)"
    requirement: "REQ-lint-planning-exclude"
    verification:
      - kind: other
        ref: "scope probe: printf '## x\\n## x\\n' > rumdl-scope-probe.md; rumdl check rumdl-scope-probe.md exits 1 (4 issues reported)"
        status: pass
    human_judgment: false
  - id: D3
    description: "ROADMAP.md and REQUIREMENTS.md corrected: SC2/REQ-p11-review-residuals name the real IN-01 finding (storeMemory/scheduleMemory duplicated Upsert-then-enqueue block, not a phantom depth-gauge duplication); SC3/REQ-lint-planning-exclude drop the stale 331-failure figure and the .planning/** glob form"
    verification:
      - kind: other
        ref: "grep -n 'depth-gauge' .planning/ROADMAP.md .planning/REQUIREMENTS.md exits 1 (no match); grep -n '331-failure' ... exits 1 (no match)"
        status: pass
    human_judgment: false

duration: 6min
completed: 2026-07-16
status: complete
---

# Phase 21 Plan 01: Rumdl Planning Exclude + Acceptance-List Correction Summary

**Added a plain `.planning` exclude entry to `.rumdl.toml` (unblocking `task lint:markdown`/`task` default, blocked since Phase 20) and corrected two factual errors in the Phase 21 ROADMAP/REQUIREMENTS acceptance list.**

## Performance

- **Duration:** 6 min
- **Started:** 2026-07-16T14:36:00Z
- **Completed:** 2026-07-16T14:42:08Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- `.rumdl.toml`'s `exclude` array now excludes `.planning` (plain directory name, why-comment, appended last per convention) — `task lint:markdown` exits 0 for the first time since Phase 20.
- Verified the exclude does not overreach: a scope probe (deliberate MD022/MD024/MD041 violations in a repo-root `.md` file) is still reported by rumdl (4 issues, exit 1), then removed before commit.
- Corrected ROADMAP.md SC2 and REQUIREMENTS.md's `REQ-p11-review-residuals` line: IN-01 no longer described as "duplicate depth-gauge registration" (a defect that does not exist — the gauge is registered exactly once at `internal/server/tools.go:255`); now names the real finding, the `storeMemory`/`scheduleMemory` duplicated Upsert-then-enqueue block.
- Corrected ROADMAP.md SC3 and REQUIREMENTS.md's `REQ-lint-planning-exclude` line: removed the stale "331-failure" figure (measured 1505→1514→1566 across three reads on 2026-07-15/16 — no fixed count is durable) in favor of qualitative language, and replaced the `.planning/**` glob form with the plain `.planning` form Task 1 actually shipped.

## Task Commits

Each task was committed atomically:

1. **Task 1: Exclude .planning from rumdl (D-09, D-10)** - `6dbfa8bb` (fix)
2. **Task 2: Correct the Phase 21 acceptance list (D-00a, D-00b, D-12)** - `5a4bd691` (docs)

**Plan metadata:** committed together with STATE.md/ROADMAP.md updates below.

## Files Created/Modified
- `.rumdl.toml` - Appended `".planning"` to the `[global].exclude` array with a why-comment
- `.planning/ROADMAP.md` - Phase 21 SC2 (real IN-01 finding) and SC3 (no stale count, plain exclude form) corrected
- `.planning/REQUIREMENTS.md` - `REQ-p11-review-residuals` and `REQ-lint-planning-exclude` lines mirrored to match

## Decisions Made
- Used the plain `.planning` directory name for the rumdl exclude entry (D-09), not a `.planning/**` glob — matches every existing neighbor entry's convention and was confirmed to work (`task lint:markdown` exits 0).
- Did not substitute a new hardcoded failure count into SC3; the real count drifts daily (1505→1514→1566 observed), so a qualitative description ("systemic planning-doc noise") is used instead, per D-00b/D-12.

## Deviations from Plan

**1. [Framework protocol] Two per-task commits instead of the plan's suggested "exactly one atomic commit"**
- **Found during:** Task 1/2 commit step
- **Issue:** The plan's `<success_criteria>` suggests squashing both tasks into a single commit (`chore(lint): exclude .planning from rumdl and correct phase 21 acceptance list`). The executor's standard `task_commit_protocol` mandates committing immediately after each task completes, and CLAUDE.md prohibits amend/rebase unless explicitly requested by the user.
- **Resolution:** Followed the executor's standard per-task commit protocol — `6dbfa8bb` (Task 1, `fix(lint)`) and `5a4bd691` (Task 2, `docs(21)`). Both are individually atomic, correctly typed, and Conventional Commits-compliant; no history rewriting was performed.
- **Impact:** None on outcome — both commits are clean, logically scoped, and traceable. All other plan content (tasks, verification, acceptance criteria) executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `task lint:markdown` now exits 0, and `task` default (lint + test) is reachable again — Plans 21-02 and 21-03 can use the full local gate instead of granular subtargets.
- ROADMAP.md and REQUIREMENTS.md now match reality; the downstream phase-completion verifier will check Plans 02/03 against a correct acceptance list.
- No blockers for 21-02 (Phase-11 residuals) or 21-03 (Renovate self-heal) — both are file-disjoint from this plan's changes.

---
*Phase: 21-ci-maintenance-hygiene*
*Completed: 2026-07-16*

## Self-Check: PASSED

- FOUND: `.planning/phases/21-ci-maintenance-hygiene/21-01-SUMMARY.md`
- FOUND: `6dbfa8bb` (Task 1 commit)
- FOUND: `5a4bd691` (Task 2 commit)
- FOUND: `9fd5884e` (SUMMARY commit)
