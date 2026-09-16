---
phase: 05-apply-time-preserve-gate-documentation
plan: 04
subsystem: docs
tags: [post-release, d-06, handoff, github-issue, phase-gate]

# Dependency graph
requires:
  - phase: 05-apply-time-preserve-gate-documentation
    plan: 01
    provides: "the shipped apply-time preserve gate this handoff's checklist points a future human observer at"
  - phase: 05-apply-time-preserve-gate-documentation
    plan: 02
    provides: "the agent-setup.md docs-gate this handoff's checklist replaces the Unreleased notice on"
  - phase: 05-apply-time-preserve-gate-documentation
    plan: 03
    provides: "the install.md/plugin.md docs-gates this handoff's checklist replaces the Unreleased notices on"
provides:
  - ".planning/phases/05-apply-time-preserve-gate-documentation/05-POST-RELEASE.md — the D-06 human handoff in the 06-POST-RELEASE.md precedent's exact shape (phase/status/tracker frontmatter; four precedent headings), status: pending"
  - "GitHub tracking issue https://github.com/seanb4t/engram/issues/567 for the Phase 5 post-release observation"
  - "A green full phase-gate run recorded at the plan's final commit (task, license, key-links, shuffle, cmd/engram, surfacesgen drift, docs-site build, generated-path cleanliness, no replace-registration flag)"
affects: []

# Actuals (#2632)
actuals:
  tokens: 1379
  tasks: 2
  commits: 1
  plan_head_before: 1ea119f73aff7c06298e913dc587e862378c03b2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "D-06/D-10 precedent reuse: 05-POST-RELEASE.md copies 06-POST-RELEASE.md's exact frontmatter key set and heading set, values filled only — no invented structure in a tool-owned/audit-parsed planning artifact"

key-files:
  created:
    - .planning/phases/05-apply-time-preserve-gate-documentation/05-POST-RELEASE.md
  modified: []

key-decisions:
  - "The tracking issue was opened FIRST (before writing the handoff file) so its real URL could be written into the frontmatter verbatim, per the plan's own task ordering — no placeholder URL was ever written."
  - "The trailing '## Current disposition' section the 06-POST-RELEASE.md precedent later grew was deliberately NOT authored here — the plan explicitly notes it was added AFTER the human observation, and this handoff is still open."

patterns-established: []

requirements-completed: []  # REQ-docs-setup-v2 deliberately NOT checked off — see below (D-06)

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "05-POST-RELEASE.md exists with exactly the three precedent frontmatter keys (phase/status/tracker), status: pending, a live OPEN tracking issue URL, and the precedent's four headings (# Post-release handoff; ## Trigger and ownership; ## Handoff checklist; ## Existing evidence) — no invented key or heading"
    requirement: "REQ-docs-setup-v2"
    verification:
      - kind: unit
        ref: "task-level acceptance shell chain (frontmatter key count, heading count, gh issue view state/title, checklist token presence) — reproduced verbatim below"
        status: pass
    human_judgment: false
  - id: D2
    description: "The handoff checklist names every D-06 qualifying-release check as a concrete, falsifiable step: an actual brew install, man pages present in the cask, engram setup observed with plugin-first delivery, --header, a preserved row, and the apply gate (byte-identical registration, plugin facet still delivered, already-correct re-run, OAuth re-login note), the three guides' Unreleased notices replaced, 05-RELEASE-<ver>.md recorded, and post_release_status/REQ-docs-setup-v2 flipped only then"
    requirement: "REQ-docs-setup-v2"
    verification:
      - kind: unit
        ref: "task-level acceptance shell chain (5 numbered steps; required tokens: plugin-first, --header, --apply, already-correct, log in, oauth-client, 05-RELEASE-, Unreleased as of v0.16.1; Do not mint a tag / Do not infer issue closure sentences)"
        status: pass
    human_judgment: false
  - id: D3
    description: "REQ-docs-setup-v2 stays [ ] in REQUIREMENTS.md and no 05-RELEASE-*.md exists in the phase directory at the end of this plan — an interrupted or never-completed closeout leaves the open handoff visible to the milestone audit, never silently complete"
    requirement: "REQ-docs-setup-v2"
    verification:
      - kind: unit
        ref: "grep -qF -- '- [ ] **REQ-docs-setup-v2**' .planning/REQUIREMENTS.md; ls .planning/phases/05-apply-time-preserve-gate-documentation/05-RELEASE-*.md (absent)"
        status: pass
    human_judgment: false
  - id: D4
    description: "The tracking issue exists on GitHub, OPEN, titled for the Phase 5 post-release observation, unassigned, no labels, with a body linking the handoff file and listing the same checks"
    requirement: "REQ-docs-setup-v2"
    verification:
      - kind: unit
        ref: "gh issue view https://github.com/seanb4t/engram/issues/567 --json state,title,assignees"
        status: pass
    human_judgment: false
  - id: D5
    description: "The whole phase gate is green at the final commit: task (lint+test incl. internal/store's live-Docker red-evidence harness), task license:check, go.mod/go.sum/docs-site lockfile diff clean, keylinks, setup shuffle, cmd/engram, surfacesgen --check-setup, docs-site frozen-lockfile install+build, generated-path porcelain clean, zero replace-registration-flag occurrences"
    requirement: "REQ-docs-setup-v2"
    verification:
      - kind: integration
        ref: "task; task license:check; git diff --exit-code -- go.mod go.sum docs-site/package.json docs-site/pnpm-lock.yaml; go test ./internal/keylinks/ -count=1; go test ./internal/setup/ -count=1 -shuffle=on; go test ./cmd/engram/ -count=1; go run ./internal/surfacesgen --check-setup; pnpm --dir docs-site install --frozen-lockfile && pnpm --dir docs-site build"
        status: pass
    human_judgment: false

# Metrics
duration: ~15min
completed: 2026-09-16
status: complete
---

# Phase 5 Plan 4: Post-Release Handoff and Phase Gate Summary

**`05-POST-RELEASE.md` records the D-06 human handoff in the `06-POST-RELEASE.md` precedent's exact shape with `status: pending` and tracking issue #567, and the full Phase 5 gate — `task`, license, key-links, shuffle, `cmd/engram`, surfaces drift, docs-site build — is green at the final commit, including `internal/store`'s live-Docker red-evidence harness.**

## Performance

- **Duration:** ~15 min (estimated — session scope: reading required context/precedent files, opening the tracking issue, authoring the handoff file, and running the full phase gate, including a 96-second `internal/store` live-Docker suite)
- **Started:** ~2026-09-16T22:30:00Z (estimated)
- **Completed:** 2026-09-16T22:43:56Z
- **Tasks:** 2
- **Files modified:** 1 (created)

## Accomplishments

- Opened GitHub tracking issue [#567](https://github.com/seanb4t/engram/issues/567), "Phase 5 post-release observation: apply-time preserve gate, plugin-first, --header, man pages (D-06 handoff)" — OPEN, unassigned, no labels — before writing the handoff file, so the real URL landed in the frontmatter verbatim.
- Authored `.planning/phases/05-apply-time-preserve-gate-documentation/05-POST-RELEASE.md` in `06-POST-RELEASE.md`'s exact precedent shape: three frontmatter keys (`phase`, `status: pending`, `tracker`), and four headings (`# Post-release handoff`, `## Trigger and ownership`, `## Handoff checklist`, `## Existing evidence`) — no invented key, heading, or section. The checklist enumerates all five D-06 steps: refresh release metadata; a real `brew install` with man pages verified under `share/man/man1`; `engram setup` observed with plugin-first delivery, `--header`, a `preserved` row, and the apply gate (byte-identical registration, plugin facet still delivered, `already-correct` convergence, the OAuth re-login note, and an opportunistic `oauth-client` capture); replacing the three guides' `Unreleased as of v0.16.1` notices and recording `05-RELEASE-<ver>.md`; and only then flipping `post_release_status`/`REQ-docs-setup-v2`.
- Ran the full Phase 5 gate at the final commit and confirmed every command exits 0 — including `internal/store`'s `TestRedEvidencePatchesAreLive`, which passed cleanly reproducing all 22 registered RED patches across phases 1-4 (no Phase 5 entries were expected yet; the orchestrator registers those after this plan, per the plan's own note, and their absence caused no failure here).
- Confirmed the D-06/T-05-15 invariant holds: `REQUIREMENTS.md` still reads `- [ ] **REQ-docs-setup-v2**`, no `05-RELEASE-*.md` exists in the phase directory, and no commit in this plan touched `ROADMAP.md`, `STATE.md`, `REQUIREMENTS.md`, or `internal/store/redevidence_harness_test.go`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Author `05-POST-RELEASE.md` and open the tracking issue** — `c38016d5` (docs)
2. **Task 2: Phase gate at the final commit** — verification-only, no files modified, no commit (as declared in the plan's `<files>` for this task)

**Plan metadata:** committed alongside this SUMMARY.

## Files Created/Modified

- `.planning/phases/05-apply-time-preserve-gate-documentation/05-POST-RELEASE.md` (created) — the D-06 human handoff: trigger/ownership, five-step qualifying-release checklist, existing pre-merge evidence; `status: pending`, `tracker: https://github.com/seanb4t/engram/issues/567`

## Decisions Made

- **Issue opened before the handoff file was written**, so the frontmatter's `tracker:` key carries the real, live issue URL from the first draft — never a placeholder later swapped in.
- **No `## Current disposition` section was authored.** The `06-POST-RELEASE.md` precedent grew that trailing section only after its own human observation was recorded; this plan's own read_first instruction says explicitly not to author one now, since this handoff is still open.
- **Task 2 produced no commit**, matching its own `<files>` declaration ("none modified — verification-only"). All gate output is recorded in this SUMMARY rather than a file.

## Deviations from Plan

None - plan executed exactly as written. No auto-fixes were needed: the phase gate was fully green on the first run, including `internal/store`'s live-Docker suite (no flaky or environmental failures encountered).

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required. `gh auth status` was green for account `seanb4t` throughout (the precondition Task 1 required).

## Phase Gate Results (Task 2, verbatim exit-code record)

| Command | Result |
|---|---|
| `task` (lint + test, incl. `internal/store` live-Docker suite, 95.19s for `TestRedEvidencePatchesAreLive`) | PASS (exit 0) |
| `task license:check` | PASS — 1931 files checked, 415 valid, 0 invalid |
| `git diff --exit-code -- go.mod go.sum docs-site/package.json docs-site/pnpm-lock.yaml` | PASS — clean, no dependency drift |
| `go test ./internal/keylinks/ -count=1` | PASS |
| `go test ./internal/setup/ -count=1 -shuffle=on` | PASS |
| `go test ./cmd/engram/ -count=1` | PASS |
| `go run ./internal/surfacesgen --check-setup` | PASS — no drift |
| `pnpm --dir docs-site install --frozen-lockfile && pnpm --dir docs-site build` | PASS — 21 pages built |
| `git status --porcelain -- skill/ internal/setupgen/ cmd/engram/testdata/catalog.golden release-please-config.json` | PASS — empty |
| `rg -o -F -e '--replace-registration' cmd/engram internal/setup \| wc -l` | PASS — 0 occurrences |
| `REQUIREMENTS.md` still unchecked / no `05-RELEASE-*.md` | PASS |

`task` did NOT fail on `internal/store`'s `TestRedEvidencePatchesAreLive` — the orchestrator-owned registration step is still pending (Phase 5 has no entry in `redEvidenceDirs` yet), but the test's empty-map-per-phase design means an unregistered phase causes no failure, only silence for that phase's own patches. This is the expected state; the orchestrator still needs to register the phase's suggested patch set (the union of 05-01's five, 05-02's three, and 05-03's two) in `internal/store/redevidence_harness_test.go`'s `redEvidenceDirs` before `/gsd-verify-work`, per this plan's own `<verification>` note — this plan did not and must not edit that file itself.

## Verification-record note for `/gsd-verify-work` (D-06, restated verbatim per the plan)

**Phase 5's verification PASSES with `post_release_status: pending` and `post_release_tracker: https://github.com/seanb4t/engram/issues/567` in `05-VERIFICATION.md`. `REQ-docs-setup-v2` remains `[ ]` in REQUIREMENTS.md until a human records `05-RELEASE-<ver>.md` after the qualifying release; the milestone audit must treat the open handoff as expected, not as a gap.** `REQ-apply-preserve-gate` and `REQ-apply-rewrite-consequence` are code-verified by plans 05-01/05-02 and may be checked off (both plans finished; the shared-ID gate already marked them complete during those plans' own `update_requirements` steps).

No task in this plan ran a `roadmap` write verb (gotcha `yzmfesbsg0`) — the ROADMAP progress row is filled by hand by the orchestrator.

## Next Phase Readiness

- This is the last plan of Phase 5. `05-POST-RELEASE.md` and tracking issue #567 are the durable artifacts a human (or a future agent, prompted by the human) will use to close the loop after the milestone's release ships.
- **Orchestrator follow-up required before `/gsd-verify-work`:** register this phase's red-evidence patch set (05-01's five, 05-02's three, 05-03's two — ten total, none new from this plan since it ships no production code) in `internal/store/redevidence_harness_test.go`'s `redEvidenceDirs` under `.planning/phases/05-apply-time-preserve-gate-documentation/red-evidence/`.
- No blockers. `REQ-docs-setup-v2` stays open by design until the post-release observation lands.

## Self-Check: PASSED

- `.planning/phases/05-apply-time-preserve-gate-documentation/05-POST-RELEASE.md` confirmed present on disk (`FOUND`).
- Commit `c38016d5` confirmed present via `git log --oneline`.
- Plan-level Task 1 `<automated>` verify chain re-run in full immediately before this SUMMARY was written: frontmatter key count = 3, `phase`/`status: pending` lines present, tracker URL resolves via `gh issue view` to `OPEN Phase 5 post-release observation…`, all four headings present with exactly 3 `## ` headings, all required substrings present, no SPDX header, commit touches exactly 1 file.
- Plan-level Task 2 `<automated>` verify chain re-run in full: all ten commands/checks pass (see Phase Gate Results table above).
- `git diff --stat 1ea119f7..HEAD` touches exactly the 1 file declared: `05-POST-RELEASE.md` — no scope creep.
- `ls .planning/phases/05-apply-time-preserve-gate-documentation/05-RELEASE-*.md` prints nothing; `grep -qF -- '- [ ] **REQ-docs-setup-v2**' .planning/REQUIREMENTS.md` exits 0.
- `git log --format=%H 1ea119f7..HEAD -- .planning/ROADMAP.md .planning/STATE.md .planning/REQUIREMENTS.md internal/store/redevidence_harness_test.go` prints nothing — no forbidden file was touched by this plan.

---
*Phase: 05-apply-time-preserve-gate-documentation*
*Completed: 2026-09-16*
