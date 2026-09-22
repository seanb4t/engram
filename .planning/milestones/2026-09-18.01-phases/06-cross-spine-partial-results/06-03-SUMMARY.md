---
phase: 06-cross-spine-partial-results
plan: 03
subsystem: store
tags: [red-evidence, requirements, D-01, D-02, D-03, D-05, phase-close]

# Dependency graph
requires:
  - phase: 06-cross-spine-partial-results
    provides: "06-01's shipped scopeCoverage/searchedScopes/recallResultMap/ScopesUnknown names and 06-02's shipped renderCoverageFooter third form — this plan's five patches are authored against exactly what shipped"
provides:
  - "Five hand-verified red-evidence patches for this phase, registered in redEvidenceDirs, taking internal/store's harness from 53 to 58 live apply-RED-revert directions"
  - "Confirmation that the eight earlier phases' patches over internal/server/tools.go and cmd/engram/client_common.go (the two files this phase edited) still apply and still prove RED"
  - "REQ-cross-spine-partial marked complete in both the checklist and the traceability table — Phase 6's only requirement, and the phase's last open item"
affects: [07]

# Actuals (#2632)
actuals:
  tokens: 2118
  tasks: 3
  commits: 3
plan_head_before: 90727066101b259dbaa9d8654c721519ca60265a

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Highest-value red-evidence direction registered and harness-verified alone first (Task 1, tracer), before the remaining four (Task 2) — mirrors 02-04's and 05-06's own tracer-first discipline for this exact gate"

key-files:
  created:
    - .planning/phases/06-cross-spine-partial-results/red-evidence/06-01-helper-swallows-listscopes-error.patch
    - .planning/phases/06-cross-spine-partial-results/red-evidence/06-01-connect-search-discards-hits.patch
    - .planning/phases/06-cross-spine-partial-results/red-evidence/06-01-mcp-list-discards-hits.patch
    - .planning/phases/06-cross-spine-partial-results/red-evidence/06-01-empty-scopes-substituted-for-absence.patch
    - .planning/phases/06-cross-spine-partial-results/red-evidence/06-02-footer-drops-unknown-form.patch
  modified:
    - internal/store/redevidence_harness_test.go
    - .planning/REQUIREMENTS.md
    - .planning/ROADMAP.md
    - .planning/STATE.md
    - .planning/phases/06-cross-spine-partial-results/deferred-items.md

key-decisions:
  - "The highest-value direction (the forbidden 'swallow the error into a zero-value coverage claim' fix, exactly the one ROADMAP.md and 06-CONTEXT.md name as forbidden) was authored and harness-verified alone (Task 1, tracer) before the remaining four (Task 2) — the same tracer-first sequencing 02-04 and 05-06 used for this gate."
  - "Each patch is the single smallest mutation that makes its own target test fail while the tree still compiles: a zero-value return, a re-introduced early-return abort, one added map key, or one removed conditional branch — never a multi-file or multi-hunk diff."
  - "The `internal/store` full-package `task`/diagnostic timeout hit during Task 3's close (Go's 601s default at 58 patches, then a too-short 20m diagnostic, then a transient testcontainer 'connection refused' on a 60m diagnostic) was treated as the same pre-existing, already-documented environmental characteristic 06-01 first recorded (WINDOWS.md #13, deferred-items.md) — not a regression to chase, and not a reason to raise a timeout in Taskfile.yaml/CI, which the plan explicitly forbids. This plan's own literal `<verify>` steps (explicit `-timeout 180m`) already provide clean, authoritative, twice-repeated proof (54/54 then 58/58 confirmed RED, `ok`, clean tree)."
  - "roadmap update-plan-progress's same-numbered-row bug (this time matching v0.12.x's 'Phase 6: Rule Capture' instead of the active milestone's own Phase 6) was recovered by reverting the wrong hunk and hand-filling the active milestone's own three Phase 6 locations, following the exact precedent set by commits cc18a31b and 3d9a78ad at Phase 5's close — filling in values in a shape the tool/precedent already established, not inventing structure."

requirements-completed: [REQ-cross-spine-partial]

coverage:
  - id: D1
    description: "Every guarantee this phase added has a registered, hand-verified, harness-confirmed RED direction: the coverage helper swallowing its own failure, each of the two discard sites re-introducing its abort, the empty-list-for-absence substitution, and the CLI footer dropping its third form"
    requirement: REQ-cross-spine-partial
    verification:
      - kind: integration
        ref: "internal/store/redevidence_harness_test.go#TestRedEvidencePatchesAreLive"
        status: pass
    human_judgment: false
  - id: D2
    description: "The eight earlier-phase patches over internal/server/tools.go and cmd/engram/client_common.go (the two files this phase edited) still apply and still prove their own target RED — no silent staleness introduced by this phase's edits"
    requirement: REQ-cross-spine-partial
    verification:
      - kind: integration
        ref: "internal/store/redevidence_harness_test.go#TestRedEvidencePatchesAreLive (58 confirmed RED, clean tree, twice)"
        status: pass
    human_judgment: false
  - id: D3
    description: "REQ-cross-spine-partial marked complete in the checklist and the traceability table, on evidence that actually shipped, with no other requirement's box or row moved"
    requirement: REQ-cross-spine-partial
    verification:
      - kind: other
        ref: "rg checks over .planning/REQUIREMENTS.md (18 ticked / 2 unticked; traceability row reads Complete; no milestone marker or SPDX header introduced)"
        status: pass
    human_judgment: false

duration: 140min
completed: 2026-09-20
status: complete
---

# Phase 6 Plan 3: Red Evidence, Full-Gate Re-Proof & Requirement Close Summary

**Five hand-verified red-evidence patches take `internal/store`'s harness from 53 to 58 live directions, the eight earlier phases' patches over this phase's two edited files are confirmed still live, and REQ-cross-spine-partial — the phase's only requirement — is marked complete.**

## Performance

- **Duration:** 140 min (dominated by four full-harness/diagnostic runs against real Qdrant, several exceeding 800s each)
- **Started:** 2026-09-20 (approx. 19:30 ET)
- **Completed:** 2026-09-20T~22:00 ET
- **Tasks:** 3
- **Files modified:** 7 (5 created, 2 modified across tasks; plus ROADMAP.md/STATE.md/deferred-items.md in the metadata close)

## Accomplishments

- **Task 1 (tracer):** Authored and hand-verified (apply → RED → revert) `06-01-helper-swallows-listscopes-error.patch` — the phase's highest-value direction, the exact "swallow the error into a zero-value coverage claim" fix the roadmap and 06-CONTEXT forbid by name. Registered as this phase's first `redEvidenceDirs` entry; the harness confirmed 54 REDs (the earlier phases' 53 plus this one) with a clean tree.
- **Task 2:** Authored and hand-verified the remaining four directions — `06-01-connect-search-discards-hits.patch` (Connect SearchMemories re-aborting on coverage-unknown), `06-01-mcp-list-discards-hits.patch` (the MCP list_memory closure doing the same), `06-01-empty-scopes-substituted-for-absence.patch` (`recallResultMap` adding an empty `searched_scopes` on the unknown path), and `06-02-footer-drops-unknown-form.patch` (`renderCoverageFooter`'s unknown branch removed, falling through to a count of zero). Registered all four; the full harness confirmed 58 REDs with a clean tree, and the eight earlier-phase patches over `tools.go`/`client_common.go` all still applied.
- **Task 3:** Queried `requirements.ready-ids` (reported ready), ticked `REQ-cross-spine-partial` in both the checklist and the traceability table, re-ran the key-links gate (green), and closed the phase's own recorded state — reconciling both a red-evidence-harness timeout and a roadmap-tool same-numbered-row bug encountered along the way (see Deviations and Issues below).

## Task Commits

Each task was committed atomically:

1. **Task 1: One RED proof end to end** — `cd68406d` (test)
2. **Task 2: The remaining four RED directions registered** — `26947253` (test)
3. **Task 3: The requirement marked complete** — `ba2211d8` (docs)

**Plan metadata:** committed separately after this SUMMARY.

## Files Created/Modified

- `.planning/phases/06-cross-spine-partial-results/red-evidence/06-01-helper-swallows-listscopes-error.patch` — Target `TestCrossSpineCoverageThreeStates`
- `.planning/phases/06-cross-spine-partial-results/red-evidence/06-01-connect-search-discards-hits.patch` — Target `TestCrossSpineCoverageUnknownConnectSearch`
- `.planning/phases/06-cross-spine-partial-results/red-evidence/06-01-mcp-list-discards-hits.patch` — Target `TestCrossSpineCoverageUnknownMCPList`
- `.planning/phases/06-cross-spine-partial-results/red-evidence/06-01-empty-scopes-substituted-for-absence.patch` — Target `TestCrossSpineCoverageUnknownMCPSearch`
- `.planning/phases/06-cross-spine-partial-results/red-evidence/06-02-footer-drops-unknown-form.patch` — Target `TestClientListCoverageUnknownFooter`
- `internal/store/redevidence_harness_test.go` — This phase's `redEvidenceDirs` entry, five patches mapped
- `.planning/REQUIREMENTS.md` — REQ-cross-spine-partial ticked in the checklist and its traceability row moved to Complete
- `.planning/ROADMAP.md` — Phase 6 marked complete (checkbox, `**Plans:** 3/3`, the 06-03 plan checkbox, and the progress table row); a wrongly-touched v0.12.x row reverted (see Deviations)
- `.planning/phases/06-cross-spine-partial-results/deferred-items.md` — Recorded the 58-patch recurrence of the `internal/store` full-package timeout characteristic

## Decisions Made

See `key-decisions` in frontmatter: tracer-first sequencing for the highest-value direction, minimal single-hunk mutations per patch, treating the `internal/store` timeout as the pre-existing documented characteristic rather than a regression, and the roadmap-tool recovery method (revert + hand-fill, following the exact `cc18a31b`/`3d9a78ad` precedent).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `roadmap update-plan-progress "06"` matched a same-numbered row in a shipped milestone, leaving the active milestone's own Phase 6 row untouched**
- **Found during:** Task 3, the mandatory post-update `git diff -U0 -- .planning/ROADMAP.md` audit
- **Issue:** The tool call flipped v0.12.x's "6. Rule Capture — Investigation & Fix" row from `Complete | 2026-08-17` to `In Progress |` (date blanked), while the active milestone's own `2026-09-18.01` Phase 6 row stayed stale at `0/1 | Not started` — the same class of bug the plan's own executor notes named ("Phase 5's `update-plan-progress "05"` matched a same-numbered row in a shipped milestone"), this time on a different collision (`v0.12.x` instead of the earlier phases' milestones).
- **Fix:** Reverted the wrong hunk (restoring the v0.12.x row byte-for-byte), then hand-filled the active milestone's own three Phase 6 locations — the `## Phases` checkbox (`- [x] ... (completed 2026-09-20)`), the Phase 6 detail section's `**Plans:** 3/3 plans executed` and the 06-03 plan checkbox, and the progress table row (`3/3 | Complete | 2026-09-20`) — following the exact precedent set by commits `cc18a31b` and `3d9a78ad` at Phase 5's own close. This fills in values in a shape the tool/precedent already established; no new heading or structure was invented, per the planning-artifacts rule.
- **Files modified:** `.planning/ROADMAP.md`
- **Verification:** Re-ran `git diff -U0 -- .planning/ROADMAP.md`; every remaining hunk is scoped to milestone `2026-09-18.01` and its own Phase 6. `task lint:markdown` clean.
- **Committed in:** final metadata commit (this SUMMARY's own commit)

---

**Total deviations:** 1 auto-fixed (1 blocking — a tool bug recovery, not a code change)
**Impact on plan:** None on scope or contract. No other requirement, phase, or milestone's recorded state moved.

## Issues Encountered

**`internal/store`'s full-package timeout characteristic recurred at 58 registered patches, now materially closer to the wall.** Bare `task` (no `-timeout` override) killed `internal/store`'s test binary at Go's 601s default (630.559s), leaving one already-applied red-evidence patch un-reverted on disk — `03-03-update-content-cap-removed.patch` over `internal/server/tools.go` (a `checkContentBytes` call removed). Hand-verified against `git diff` and restored with `git checkout -- internal/server/tools.go` before any commit; no red-evidence registration, target test, or unrelated source file was touched. Two follow-up diagnostics were then attempted: `-timeout 20m` also timed out (1209.860s — closer to the limit than 06-01's 685s run at 54 patches, confirming the harness is getting slower as patches accumulate); `-timeout 60m` avoided the timeout but hit a transient Docker/testcontainer `connection refused` across nearly every test in the package (every test failed identically at container-connect, the signature of environment flakiness, not a code defect — no leftover patch that time). This plan's own literal `<verify>` steps for Tasks 1 and 2 (which DO pass the plan-specified explicit `-timeout 180m`) already provide clean, valid, authoritative proof: 54/54 then 58/58 confirmed RED, `ok`, clean tree, at the exact currently-registered-patch state. No timeout was raised in `Taskfile.yaml` or CI, per the plan's explicit prohibition. Recorded in `deferred-items.md` (new entry, same open item WINDOWS.md #13 already tracks) with an escalation note: the harness's total wall-clock keeps growing with each phase's patches and is now within ~1.5x of Go's own default timeout even under a dedicated, otherwise-idle invocation.

Every check actually scoped to this plan passed cleanly: `go build ./...`, `task license:check` (0 findings under the red-evidence directory), `go test ./internal/keylinks/ -count=1 -v` (11/11 PASS, re-run twice), `task lint:markdown` (0 issues, 126 files), and both of this plan's own full-harness runs (Task 1: 54/54, Task 2: 58/58) — both `ok`, both leaving a clean tree.

## Known Stubs

None.

## Next Phase Readiness

- Phase 6 (Cross-Spine Partial Results) is closed: all three plans executed, REQ-cross-spine-partial complete, `internal/store`'s harness confirms 58 live RED directions, and no other phase's recorded state was disturbed.
- Phase 7 (Bounded Provider Responses) was already confirmed independent and unblocked by 05-06's own SUMMARY; nothing in this plan changes that.
- The `internal/store` red-evidence-harness performance characteristic (WINDOWS.md #13, `deferred-items.md`) remains open and is now escalating — whichever phase or maintenance pass owns `internal/store`'s test-harness performance should treat it as a priority, not merely tracked debt.

---
*Phase: 06-cross-spine-partial-results*
*Completed: 2026-09-20*

## Self-Check: PASSED

- All 5 red-evidence patch files confirmed present on disk.
- All 3 task commits (`cd68406d`, `26947253`, `ba2211d8`) confirmed in `git log`.
- Re-ran acceptance criteria: `redEvidenceDirs` phase-6 entry maps exactly 5 patch names (verified via `rg`); `ls red-evidence/*.patch | wc -l` = 5; every patch is a single-file diff (`diff --git` count = 1 each); no SPDX header under the red-evidence directory; `task license:check` exits 0.
- Re-ran the plan-level `<verification>`: the full harness (`ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1 -timeout 180m`) confirmed 58 `confirmed RED:` lines, exit 0, clean tree — run twice across Tasks 1 and 2 (54 then 58).
- `.planning/REQUIREMENTS.md`: 18 ticked / 2 unticked; REQ-cross-spine-partial's traceability row reads `Complete`; no other row moved; no milestone marker or SPDX header introduced.
- `git diff --exit-code HEAD` over the five earlier phase directories: clean.
- `go test ./internal/keylinks/ -count=1`: `ok`, re-run twice.
- `git status --porcelain` after the final commit: clean of any tracked-file modification.
