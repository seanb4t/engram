---
phase: 03-plugin-first-delivery
plan: 02
subsystem: skills
tags: [go, skills, symlink, agents-md, plugin-delivery]

requires:
  - phase: 03-plugin-first-delivery
    provides: "plan 03-01's PluginRuntime/PluginPreview/PluginApply lane this plan reports alongside"
provides:
  - "Environment.Lstat func field mirroring os.Lstat, OSEnvironment.Lstat = os.Lstat, additive and unused by Install"
  - "DetectPresence(env, target, list) Presence — read-only count of shipped skills present at a native destination, symlink-vs-copy classification, and AGENTS.md index-block presence"
affects: [03-03-report-integration]

actuals:
  tokens: 3422
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Presence check reuses the package's own scanBlock classifier for index-block state rather than a second marker scan"
    - "Report-only seam: DetectPresence touches only Lstat/ReadFile, mechanically enforced in the test by leaving WriteFile/MkdirAll nil on the fake Environment so any write attempt panics"

key-files:
  created:
    - internal/skills/presence.go
    - internal/skills/presence_test.go
  modified:
    - internal/skills/environment.go

key-decisions:
  - "Used testSkills() (the existing 2-skill fixture from install_test.go) rather than a new literal five-skill list — the plan's own <behavior> offered either, and reusing the fixture keeps DetectPresence's tests independent of the real embedded Inventory() exactly like the rest of this package's tests."
  - "lstat-permission-error-counts-absent subtest uses os.ErrPermission specifically (not just os.ErrNotExist) to prove the 'any error counts absent, report-only' rule is not accidentally narrowed to not-exist alone."

requirements-completed: []  # REQ-plugin-skips-skills-copy is shared with 03-03 (not yet executed) — shared-ID gate (#2388) blocks marking it Complete until every declaring plan finishes; verified via `gsd-tools query requirements.ready-ids` returning 0/1 ready.

coverage:
  - id: D1
    description: "Environment.Lstat seam added, additive-only; Install byte-unchanged"
    verification:
      - kind: unit
        ref: "internal/skills/install_test.go (full existing suite, unchanged, still green)"
        status: pass
    human_judgment: false
  - id: D2
    description: "DetectPresence reports skill count, symlink-vs-copy, and AGENTS.md index-block presence, reading only through Lstat/ReadFile"
    requirement: REQ-plugin-skips-skills-copy
    verification:
      - kind: unit
        ref: "internal/skills/presence_test.go#TestDetectPresence (12 subtests incl. real-filesystem)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Full repo gate stays green: task, task license:check, empty go.mod/go.sum diff, keylinks, shuffled internal/skills"
    verification:
      - kind: other
        ref: "task && task license:check && git diff --exit-code -- go.mod go.sum && go test ./internal/keylinks/ -count=1 && go test ./internal/skills/ -count=1 -shuffle=on"
        status: pass
    human_judgment: false

duration: 22min
completed: 2026-09-14
status: complete
---

# Phase 3 Plan 2: Skills Presence Detection Summary

Read-only `DetectPresence` in `internal/skills` reports shipped-skill count, symlink-vs-copy, and Codex `AGENTS.md` index-block presence at a runtime's native destination through a new `Environment.Lstat` seam — never writing, creating, or removing anything.

## Performance

- **Duration:** 22 min
- **Started:** 2026-09-14T (session start)
- **Completed:** 2026-09-14T (session end)
- **Tasks:** 2
- **Files modified:** 3 (2 created, 1 modified)

## Accomplishments

- `Environment` gained a fourth func field, `Lstat func(name string) (os.FileInfo, error)`, with `OSEnvironment.Lstat = os.Lstat` in production — a purely additive change; every existing `Install` test passes unchanged and `Install` itself never references the new field (`rg -n Lstat internal/skills/install.go` is empty).
- New `internal/skills/presence.go` declares `Presence{Skills, Symlink, IndexBlock}` and `DetectPresence(env Environment, target Target, list []Skill) Presence`: `FormatNone` short-circuits to the zero value with zero `Environment` calls; a directory or any counted skill entry carrying `os.ModeSymlink` sets `Symlink`; any `Lstat` error (not-exist, permission, or otherwise) on a skill entry counts it absent rather than erroring; `FormatAgentsMD`'s index-block check reuses the package's own `scanBlock` classifier so a well-formed OR malformed block both report `IndexBlock == true`, and only `blockAbsent` reports `false`.
- `TestDetectPresence` covers 12 subtests: none-present, copies, per-skill-symlinks (the maintainer's real `~/.agents/skills` case), symlinked-dir, partial, four AGENTS.md-block states, format-none (with a fake `Environment` whose `ReadFile`/`Lstat` both call `t.Errorf` if invoked), a permission-error-counts-absent case, and a real-filesystem subtest under `t.TempDir()` using an actual `os.Symlink`.
- Full repo gate (`task`, `task license:check`, empty `go.mod`/`go.sum` diff, `internal/keylinks`, shuffled `internal/skills`) is green, including the full `task` run across every package (lint + all Go/Python tests, `internal/store`'s red-evidence harness included).

## Task Commits

Each task was committed atomically:

1. **Task 1: Lstat seam + DetectPresence** - `0fe08234` (feat) — RED observed first (`presence_test.go` in place, `presence.go` absent: `go test` failed naming `DetectPresence`/`Presence` undefined across 10+ lines), then GREEN after adding `environment.go`'s field and `presence.go`.
2. **Task 2 gate fixups** - `da7ad0d7` (chore) — golangci-lint's `revive` flagged a local `real` variable shadowing the builtin in the real-filesystem subtest; renamed to `realDir`.

**Plan metadata:** committed separately after this SUMMARY.

_Task 1 was TDD (`tdd="true"`): the test file was written first and observed to fail to compile before the two production edits were made. See "TDD Gate Compliance" below._

## Files Created/Modified

- `internal/skills/environment.go` - Added `Lstat func(name string) (os.FileInfo, error)` field + `OSEnvironment.Lstat = os.Lstat`
- `internal/skills/presence.go` - `Presence` struct + `DetectPresence` (read-only: `Lstat`/`ReadFile` only)
- `internal/skills/presence_test.go` - `TestDetectPresence`, `fakeLstatEnv`, `fakeFileInfo`

## Decisions Made

- Reused `testSkills()` (the existing 2-skill fixture in `install_test.go`) instead of authoring a new five-skill literal — the plan explicitly offered either option, and this keeps `DetectPresence`'s tests decoupled from the real embedded `Inventory()`, matching every other test in this package.
- The permission-error subtest deliberately uses `os.ErrPermission`, distinct from the not-exist case used everywhere else, to prove the "any error → absent, report-only" rule generalizes rather than being accidentally narrowed to `os.ErrNotExist`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] golangci-lint `revive` finding: builtin shadow in test**
- **Found during:** Task 2 (`task` full lint+test run)
- **Issue:** `TestDetectPresence`'s real-filesystem subtest declared a local variable named `real`, shadowing Go's builtin `real()` function (`redefines-builtin-id`).
- **Fix:** Renamed the variable to `realDir` at all four use sites within the subtest.
- **Files modified:** `internal/skills/presence_test.go`
- **Verification:** `task` (full lint+test) exits 0; `go test ./internal/skills/ -count=1` still green.
- **Committed in:** `da7ad0d7`

---

**Total deviations:** 1 auto-fixed (1 lint/Rule 1).
**Impact on plan:** Cosmetic rename only; no behavior change; no scope creep.

## Issues Encountered

None.

## TDD Gate Compliance

| Task | RED observed | GREEN | REFACTOR | Notes |
|------|---------------|-------|----------|-------|
| Task 1 | Yes — `presence_test.go` in place, `presence.go` absent and `environment.go` unmodified: `go test ./internal/skills/ -run '^TestDetectPresence$' -count=1` failed to compile, naming `DetectPresence` and `Presence` undefined (10+ compile errors) | Yes — full Task 1 verify command green after `environment.go` + `presence.go` additions, all 12 `TestDetectPresence` subtests pass including `real-filesystem` | N/A (no refactor commit needed beyond the Task 2 lint fixup, tracked separately as a deviation) | Single feat commit `0fe08234` per the plan's own commit-scope instruction (test+prod authored together) |

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `Presence`/`DetectPresence` are ready for plan 03-03 to render onto the operator-facing report row's `skills_native` field (D-08/D-09).
- `REQ-plugin-skips-skills-copy` stays `Pending` in REQUIREMENTS.md: it is shared with plan 03-03, which has not yet run, so the shared-ID gate (#2388) correctly withholds marking it `Complete` (verified: `requirements.ready-ids` reports `0/1 requirement(s) ready`). 03-03's own `update_requirements` step will mark it once both plans have summaries.
- `internal/skills` remains a stdlib-only leaf (`TestSkillsPackageImportsAreGated` green); `go.mod`/`go.sum` unchanged.
- No blockers for 03-03/03-04.

## Self-Check: PASSED

- `internal/skills/presence.go` and `internal/skills/presence_test.go` exist on disk: confirmed via `[ -f ]`.
- Commits `0fe08234` and `da7ad0d7` both found via `git log --oneline --all`.
- All plan-level `<acceptance_criteria>` re-run and PASS (all `rg` greps and the full `TestDetectPresence` run shown above).
- Plan-level `<verification>` commands re-run and PASS: `go test ./internal/skills/ -run '^TestDetectPresence$' -count=1 -v` (12/12 subtests PASS incl. `real-filesystem`), `rg -n -e 'WriteFile|MkdirAll|Remove' internal/skills/presence.go` (empty), `task && task license:check && git diff --exit-code -- go.mod go.sum && go test ./internal/keylinks/ -count=1` (all exit 0).

---
*Phase: 03-plugin-first-delivery*
*Completed: 2026-09-14*
