---
phase: 15-additive-proto-stub-write-handlers
plan: 02
subsystem: build
tags: [taskfile, github-actions, buf, protobuf, ci, csrf]

requires:
  - phase: 15-additive-proto-stub-write-handlers (plan 01)
    provides: the six additive write RPCs and buf.validate annotations in proto/engram/v1/engram.proto, gen/ regenerated
provides:
  - task proto:lint grep-ban on idempotency_level = NO_SIDE_EFFECTS anywhere under proto/
  - CI buf job inline mirror of the same ban (no task binary on the runner)
affects: [15-03-PLAN.md, 15-04-PLAN.md, future proto edits]

tech-stack:
  added: []
  patterns:
    - "Grep-ban build gate: anchored regex, POSIX character class, scoped to proto/, mirrored verbatim between Taskfile and CI rather than shared as a script (per this repo's bare-runner CI convention)"

key-files:
  created: []
  modified:
    - Taskfile.yaml
    - .github/workflows/ci.yaml

key-decisions:
  - "Duplicate the grep regex verbatim in Taskfile.yaml and ci.yaml rather than extracting a shared script — CI mirrors Taskfile commands inline per repo convention (ci.yaml:12-14), and Plan 04's descriptor test is the defense-in-depth backstop if the two copies ever drift (accepted per 15-REVIEWS.md Codex LOW finding)"

patterns-established:
  - "Idempotency-level ban: task proto:lint and the CI buf job both fail the build the moment any RPC under proto/ carries idempotency_level = NO_SIDE_EFFECTS (GET-reachable + CSRF risk, PITFALLS.md Pitfall 2)"

requirements-completed: [REQ-connect-write-rpcs]

coverage:
  - id: D1
    description: "task proto:lint gains a grep-ban step that fails the build on any NO_SIDE_EFFECTS idempotency_level annotation under proto/, and stays green on the clean Phase-15 proto"
    requirement: "REQ-connect-write-rpcs"
    verification:
      - kind: other
        ref: "task proto:lint (manual run, this session) — green on current proto/"
        status: pass
      - kind: other
        ref: "isolation test: injected `option idempotency_level = NO_SIDE_EFFECTS;` onto GetMemory in proto/engram/v1/engram.proto, ran `go tool buf lint` alone (exit 0, confirms buf lint does not catch it), ran the grep-ban command alone (non-zero exit, PITFALLS.md Pitfall 2 message printed), then restored the original file (git diff clean)"
        status: pass
    human_judgment: false
  - id: D2
    description: "CI buf job gains an inline 'idempotency ban (no side-effect-free RPC)' step mirroring the Taskfile grep exactly, with no setup-task/task binary added to the bare runner"
    requirement: "REQ-connect-write-rpcs"
    verification:
      - kind: other
        ref: "grep -q 'idempotency ban' .github/workflows/ci.yaml && ! grep -q 'setup-task' .github/workflows/ci.yaml (pass)"
        status: pass
      - kind: other
        ref: "actionlint .github/workflows/ci.yaml && yamlfmt -lint .github/workflows/ci.yaml (both clean)"
        status: pass
    human_judgment: false

duration: 6min
completed: 2026-07-11
status: complete
---

# Phase 15 Plan 02: Idempotency-Level Ban Build Gate Summary

**Grep-ban build gate in both `task proto:lint` and the CI `buf` job blocking any RPC from ever being annotated `idempotency_level = NO_SIDE_EFFECTS` (the GET-reachable, CSRF-exploitable annotation per PITFALLS.md Pitfall 2)**

## Performance

- **Duration:** 6 min
- **Started:** 2026-07-11T22:12:00Z (approx.)
- **Completed:** 2026-07-11T22:18:00Z (approx.)
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- `task proto:lint` now runs `go tool buf lint` followed by an anchored grep-ban over `proto/` for `idempotency_level = NO_SIDE_EFFECTS`, failing the build with a PITFALLS.md Pitfall 2 pointer on any match, and staying green on the clean Phase-15 proto
- The CI `buf` job gained an inline mirror step ("idempotency ban (no side-effect-free RPC)") using the identical regex, emitting a `::error::` GitHub annotation and exiting 1 on match — added directly as a `run:` block with no `setup-task`/`task` binary install, matching every other step in the job
- Isolation-proved the grep gate (per 15-REVIEWS.md Finding #7, codex MEDIUM): injected the banned option onto the real `GetMemory` RPC in a working copy, confirmed `go tool buf lint` alone stays green on the modified file, confirmed the grep-ban command alone (not the full `proto:lint` chain) exits non-zero with the Pitfall 2 message, then restored the original file (`git diff` on `proto/` empty afterward)

## Task Commits

Each task was committed atomically:

1. **Task 1: Add the idempotency-level ban to Taskfile proto:lint** - `b19e5cd8` (feat)
2. **Task 2: Mirror the ban as an inline step in the CI buf job** - `ac45e635` (feat)

_No TDD tasks in this plan._

## Files Created/Modified
- `Taskfile.yaml` - `proto:lint` gained a second `cmds:` entry: the grep-ban shell block
- `.github/workflows/ci.yaml` - `buf` job gained a new "idempotency ban (no side-effect-free RPC)" step after "generated-code drift"

## Decisions Made
- Duplicated the grep regex verbatim between `Taskfile.yaml` and `.github/workflows/ci.yaml` instead of extracting a shared script/action. This repo's CI convention (documented at `ci.yaml:12-14`) mirrors each Taskfile target's underlying shell command directly on bare runners rather than invoking the `task` binary, so a shared script would deviate from every other job in the file. Accepted per 15-REVIEWS.md (Codex LOW finding) with Plan 04's descriptor test as the defense-in-depth backstop against regex drift.

## Deviations from Plan

None - plan executed exactly as written. Both tasks matched 15-PATTERNS.md's verbatim recommended commands with no adjustments needed.

## Issues Encountered

None. `task fmt` (run to confirm the CI YAML diff needed no reformatting) touched four unrelated files (`.claude/settings.json`, `docs-site/package.json`, `internal/webauth/static/_app/version.json`, `ui/tsconfig.json`) with pre-existing formatting drift outside this plan's scope; these were reverted via `git checkout --` before committing, per the scope-boundary rule (out-of-scope discoveries are not auto-fixed).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- The build-time gate (SC2) is now permanent and enforced both locally (dev parity) and in CI, ahead of Plan 03/04's runtime interceptor and descriptor-test work in this phase
- No blockers for 15-03-PLAN.md or 15-04-PLAN.md

---
*Phase: 15-additive-proto-stub-write-handlers*
*Completed: 2026-07-11*

## Self-Check: PASSED
