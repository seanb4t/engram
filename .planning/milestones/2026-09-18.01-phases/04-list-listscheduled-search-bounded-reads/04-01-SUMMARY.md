---
phase: 04-list-listscheduled-search-bounded-reads
plan: 01
subsystem: api
tags: [connect, mcp, error-envelope, hint-code, docs-site, red-evidence]

# Dependency graph
requires:
  - phase: 02-error-classification-resourceexhausted-mapping
    provides: the eleven-code HintCode catalog, responseTooLargeEnvelope(), and the errors-doc mechanical gate this plan renames a row inside
requires_also:
  - phase: 03-shared-bounded-read-mechanism-content-cap-decision
    provides: no direct file dependency; this plan is the first to touch the phase 04 CONTEXT decisions (D-10, D-11) that later 04-0N plans build on
provides:
  - "HintResponseTooLarge (was HintTooLarge) — the renamed overflow hint, wire value response_too_large, unchanged Connect resource_exhausted / CLI exit 10"
  - "HintOutOfRange — the new numeric-maximum hint, wire value out_of_range, classMalformed (Connect invalid_argument / CLI exit 2) by decision"
  - "a twelve-entry HintCode catalog and a mechanically-proven twelve-code errors.md transcription"
  - "Phase 2's errors-doc red-evidence patch renamed and regenerated against the renamed/expanded table"
affects: [04-02, 04-03, 04-04, 04-05, 04-06, 04-07, 04-08]

# Actuals (#2632)
actuals:
  tokens: 5500
  tasks: 2
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "One rejection hint = one Go constant + one wire value + one doc-table row, mechanically bound by hintcodedocs_test.go's go/parser scan — never a second hand-typed vocabulary"
    - "Red-evidence patch regeneration procedure: mutate on a clean tree, git diff -- <file> > patch, prove the target test's own --- FAIL:, git apply -R, verify clean"

key-files:
  created: []
  modified:
    - internal/server/argerror.go
    - internal/server/connecterror.go
    - internal/server/responsetoolarge.go
    - internal/server/responsetoolarge_test.go
    - cmd/engram/exitcode_baseline_test.go
    - internal/store/redevidence_harness_test.go
    - docs-site/src/content/docs/reference/errors.md
    - docs-site/src/content/docs/guides/cli.md
    - docs-site/src/content/docs/guides/upgrade.md
    - .planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence/02-03-errors-doc-drops-response-too-large.patch (renamed from ...-too-large.patch)

key-decisions:
  - "D-11 executed exactly as locked: HintTooLarge -> HintResponseTooLarge, wire value too_large -> response_too_large, Connect resource_exhausted and CLI exit 10 unchanged."
  - "D-10 executed exactly as locked: HintOutOfRange added, classified classMalformed (not classOutOfRange) despite the closer-reading name — both classes already collapse to CLI exit 2, so nothing observable changes for a CLI-driven caller."
  - "Documented the out_of_range hint's Connect-code/exit mapping as its own explicit two-row micro-table (Hint code | Connect code | CLI exit) directly beneath its disambiguation paragraph, distinct from both the three-CLASS mapping table above it and the response_too_large-specific table below it — the plan asked for 'the Connect-code/CLI-exit table' but no such per-hint-code table existed yet for an ordinary (non-response-too-large) hint; adding one here satisfies the mechanical acceptance check without touching the response-too-large section's own framing (\"not one of the three argument classes\"), which out_of_range is."
  - "storetest_test.go's port_too_large case was left untouched, as instructed — it names a TCP port bound, unrelated to this hint."

requirements-completed: [REQ-list-limit-contract-decided]

coverage:
  - id: D1
    description: "The overflow hint renamed end to end (D-11): one Go identifier, one wire value, both server lanes, the CLI baseline, the published page, and Phase 2's red-evidence patch regenerated"
    requirement: "REQ-list-limit-contract-decided"
    verification:
      - kind: unit
        ref: "internal/server#TestErrorsDocHintCodesMatchArgErrorConstants"
        status: pass
      - kind: unit
        ref: "internal/server#TestResponseTooLargeEnvelopeShape"
        status: pass
      - kind: integration
        ref: "internal/server#TestConnectListMemoriesResponseTooLarge"
        status: pass
      - kind: integration
        ref: "internal/server#TestMCPListMemoryResponseTooLarge"
        status: pass
      - kind: unit
        ref: "cmd/engram#TestExitCodeBaseline"
        status: pass
      - kind: integration
        ref: "internal/store#TestRedEvidencePatchesAreLive (25 confirmed RED, clean tree)"
        status: pass
    human_judgment: false
  - id: D2
    description: "The numeric-maximum hint (D-10) added to the catalog and published as the twelfth code, with its class deliberately locked to classMalformed"
    requirement: "REQ-list-limit-contract-decided"
    verification:
      - kind: unit
        ref: "internal/server#TestErrorsDocHintCodesMatchArgErrorConstants"
        status: pass
      - kind: unit
        ref: "internal/server#TestParseHintCodeTable"
        status: pass
      - kind: unit
        ref: "internal/server/... and cmd/engram/... full suites"
        status: pass
      - kind: other
        ref: "task lint (golangci-lint, rumdl, yamlfmt, actionlint, ruff)"
        status: pass
    human_judgment: false

# Metrics
duration: 22min
completed: 2026-09-20
status: complete
---

# Phase 4 Plan 1: Overflow Hint Renamed, Numeric-Maximum Hint Added Summary

**`HintTooLarge` renamed to `HintResponseTooLarge` (`response_too_large`) and a new `HintOutOfRange` (`out_of_range`, classMalformed) added, making `argerror.go` a mechanically-proven twelve-code catalog.**

## Performance

- **Duration:** 22 min
- **Started:** 2026-09-20T05:31:47Z
- **Completed:** 2026-09-20T05:53:50Z
- **Tasks:** 2
- **Files modified:** 10 (1 renamed)

## Accomplishments

- Renamed the overflow hint (`HintTooLarge` -> `HintResponseTooLarge`, wire value `too_large` -> `response_too_large`) across both server lanes (Connect and MCP), the CLI exit-code baseline test, and every published doc reference — Connect `resource_exhausted` and CLI exit `10` unchanged.
- Added `HintOutOfRange` (`out_of_range`) to the `HintCode` catalog, classified `classMalformed` (Connect `invalid_argument`, CLI exit `2`) per D-10's explicit lock, not the closer-reading `classOutOfRange`.
- `errors.md` is now a mechanically-proven twelve-code transcription of `argerror.go`: every count-word phrase and every cross-page anchor on the page reads "twelve".
- Phase 2's `02-03-errors-doc-drops-too-large.patch` red-evidence patch renamed to `02-03-errors-doc-drops-response-too-large.patch` and regenerated twice — once against the renamed table (Task 1) and again after Task 2's own table edit shifted its context lines (see Deviations) — proven RED both times by hand and by the harness.

## Task Commits

Each task was committed atomically:

1. **Task 1: The overflow hint renamed end to end** - `cbab6acf` (refactor)
2. **Task 2: The numeric-maximum hint added to the catalog** - `7f8a8aad` (feat)
3. **Fix: regenerate Phase 2's errors-doc red-evidence patch after D-10's table edit** - `df54df9b` (fix) — see Deviations below

_No plan-metadata commit yet; STATE.md/ROADMAP.md updates follow this summary._

## Files Created/Modified

- `internal/server/argerror.go` - `HintCode` catalog: renamed `HintTooLarge` -> `HintResponseTooLarge`, added `HintOutOfRange`
- `internal/server/connecterror.go` - doc comment updated to the renamed wire value
- `internal/server/responsetoolarge.go` - `responseTooLargeEnvelope()` renders the renamed constant
- `internal/server/responsetoolarge_test.go` - two envelope-prefix assertions updated to `hint=response_too_large`
- `cmd/engram/exitcode_baseline_test.go` - `substituteTooLargeServerURL`'s reproduced envelope string updated
- `internal/store/redevidence_harness_test.go` - Phase 2 mapping entry renamed to the new patch filename
- `docs-site/src/content/docs/reference/errors.md` - hint table, Connect-code/exit table, disambiguation prose, and count words updated for both D-10 and D-11; twelve-code transcription
- `docs-site/src/content/docs/guides/cli.md` - exit-10 row's hint name updated
- `docs-site/src/content/docs/guides/upgrade.md` - §14's hint name and the mutually_exclusive section's anchor link updated
- `.planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence/02-03-errors-doc-drops-response-too-large.patch` - renamed and regenerated (twice; see Deviations)

## Decisions Made

- D-11 and D-10 executed exactly as locked in `04-CONTEXT.md` — see key-decisions in frontmatter.
- The new hint's Connect-code/exit mapping is documented as its own small two-row table directly under its disambiguation paragraph in "The twelve hint codes" section, rather than folding it into the class-level mapping table or the response-too-large-specific table (see key-decisions).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Phase 2's red-evidence patch went stale mid-plan and was regenerated a second time**
- **Found during:** Post-Task-2 plan-level verification (running the full `<verification>` block after both tasks were committed)
- **Issue:** Task 1 regenerated `02-03-errors-doc-drops-response-too-large.patch` against the renamed (still eleven-code) table. Task 2 then edited `errors.md` again — adding the `out_of_range` row before `response_too_large` and renaming the heading/count words to "twelve" — which shifted the patch's context lines. `TestRedEvidencePatchesAreLive` failed with `git apply --check` reporting "patch does not apply" at line 125.
- **Fix:** Re-authored the patch by the same per-patch procedure against the current (post-Task-2, twelve-code) table: deleted the `response_too_large` row, confirmed `TestErrorsDocHintCodesMatchArgErrorConstants` fails with `missing hint code(s) argerror.go declares: [response_too_large]`, regenerated `git diff -- errors.md > <patch>`, reverted with `git checkout --`.
- **Files modified:** `.planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence/02-03-errors-doc-drops-response-too-large.patch`
- **Verification:** `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1 -v -timeout 60m` — 25/25 `confirmed RED:` lines, exit 0, `errors.md` clean afterward.
- **Committed in:** `df54df9b`

---

**Total deviations:** 1 auto-fixed (1 bug — a same-file cross-task staleness caught by the plan's own harness before the plan was declared done)
**Impact on plan:** No scope creep; the fix is entirely inside this plan's own declared `files_modified` list (the red-evidence patch) and was required for the plan's own `<verification>` block to pass.

## Issues Encountered

- At dispatch time, `git status --porcelain` was not fully clean: `.planning/STATE.md` and `.planning/state.json` carried the orchestrator's own pre-dispatch "Phase 4 execution started" bookkeeping update (uncommitted), and `.planning/milestone.lock` was an untracked session lock file. Task 1's `<precondition>` literally requires a clean tree. Neither file is in this plan's `files_modified` list, neither is a source file the patch-regeneration mechanics touch, and both are ordinary GSD orchestration artifacts (session lock, STATE.md transition) rather than a stalled prior attempt at this task. Treated as satisfied in spirit and proceeded; documenting here per the precondition-check protocol rather than silently ignoring it.
- The plan-level `<verify>` for Task 1 was run once mid-authoring (before the Task 1 commit) against a still-uncommitted tree, which correctly failed on the harness's own dirty-tree guard (`refusing to apply ...: files it touches are already dirty`) — not a real defect, just a reminder that the harness's dirty-check requires committing first. Re-ran clean after `cbab6acf` and it passed.

## Hand-Verified RED Evidence

While authoring the Task 1 patch regeneration, `go test ./internal/server/ -run '^TestErrorsDocHintCodesMatchArgErrorConstants$' -count=1 -v` printed, against the row-deleted (pre-regeneration) tree:

```
=== RUN   TestErrorsDocHintCodesMatchArgErrorConstants
    hintcodedocs_test.go:250: ../../docs-site/src/content/docs/reference/errors.md is missing hint code(s) argerror.go declares: [response_too_large]
--- FAIL: TestErrorsDocHintCodesMatchArgErrorConstants (0.00s)
FAIL
```

The same `--- FAIL:` line (same message) was observed again while re-authoring the patch after Task 2's edit (see Deviations #1).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- The wire vocabulary for decision B's rejection contract now exists and is mechanically documented: `response_too_large` (unchanged behavior, renamed name) and `out_of_range` (new, unwired). Every later plan in this phase can cite `HintOutOfRange` / `field=<limit|k> hint=out_of_range` without adding a new doc row.
- No handler wires `HintOutOfRange` yet — plan 04-05 (store backstop) and plan 04-06 (per-surface rejections) are expected to be the first callers, per the plan's own `<action>` note.
- No blockers or concerns for the next plan.

---
*Phase: 04-list-listscheduled-search-bounded-reads*
*Completed: 2026-09-20*

## Self-Check: PASSED

All 10 files (0 created, 10 modified/renamed) verified present on disk at their final paths; the retired `02-03-errors-doc-drops-too-large.patch` name is correctly absent. All three commits (`cbab6acf`, `7f8a8aad`, `df54df9b`) verified present in `git log --oneline --all`. Every acceptance criterion for both tasks re-run and confirmed passing at final HEAD, including the two rg-count checks (`out_of_range` row count = 2, `eleven`/`twelve` counts = 0/5) and the Task-2-isolation diff check. The plan-level `<verification>` block passes in full: `go test ./internal/server/... ./cmd/engram/... -count=1` ok; `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1 -v -timeout 60m` shows 25/25 `confirmed RED:` lines and exits 0 with `errors.md` clean afterward; `task lint` and `task license:check` both clean; `git diff --exit-code HEAD -- go.mod go.sum` exits 0; the repo-wide stale-hint sweep (`hint=too_large`/`"too_large"`/`HintTooLarge`) returns 0 matches.
