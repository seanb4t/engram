---
phase: 02-error-classification-resourceexhausted-mapping
plan: 04
subsystem: testing
tags: [red-evidence, ast-gate, ci, regression-proof]

requires:
  - phase: 02-error-classification-resourceexhausted-mapping (plans 01-03)
    provides: "store.ErrResponseTooLarge classifier, Connect/MCP mappers, CLI exit code 10 (both tiers), errors.md hint-code table — every lane this plan's patches revert"
provides:
  - "redEvidenceDirs[\".planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence\"]: eight hand-verified RED patches, one per phase lane"
  - "TestRedEvidencePatchesAreLive confirms 12 REDs (Phase 1's four plus Phase 2's eight) for the rest of the open milestone"
affects: []

actuals:
  tokens: 3189
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "One-mutation-per-patch red-evidence authored by hand-verifying git apply --check / apply / go test -run '^Target$' (observing the target's own --- FAIL: line, never a build break) / apply -R before registration — Phase 1's own precedent (01-05-SUMMARY.md), repeated for every one of Phase 2's eight lanes"

key-files:
  created:
    - .planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence/02-02-connect-arm-removed.patch
    - .planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence/02-01-classifier-not-in-base-options.patch
    - .planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence/02-01-classifier-requires-two-number-shape.patch
    - .planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence/02-01-classifier-relabels-any-resource-exhausted.patch
    - .planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence/02-02-mcp-mapper-unregistered.patch
    - .planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence/02-03-exit-mapping-reverted.patch
    - .planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence/02-03-operator-arm-removed.patch
    - .planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence/02-03-errors-doc-drops-too-large.patch
  modified:
    - internal/store/redevidence_harness_test.go

key-decisions:
  - "Task 1 (type=tracer) proved one full lane end to end — author, hand-verify RED, register, re-run the harness alone — before Task 2 authored the remaining seven; the tracer feedback gate (interactive, human_verify_mode=end-of-phase default, Task 1's <verify> carrying only <automated>) re-ran that same verify and passed, so execution proceeded to Task 2 without a checkpoint."
  - "Each of the eight mutations is the smallest single-statement or single-line change that trips exactly its target test while the tree still compiles — deleting one appended dial option, tightening one boolean condition, deleting one switch case (and its log/return lines where present), or deleting one markdown table row — matching Phase 1's own established mutation-size discipline."

requirements-completed: [REQ-exhausted-sentinel, REQ-exhausted-connect, REQ-exhausted-mcp, REQ-exhausted-cli-docs]

coverage:
  - id: D1
    description: "redEvidenceDirs gains exactly one new entry for this phase, mapping all eight patches to their target tests; Phase 1's entry is unchanged"
    requirement: "REQ-exhausted-sentinel"
    verification:
      - kind: unit
        ref: "internal/store#TestRedEvidencePatchesAreLive/.planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every one of the phase's four requirements has a live, harness-checked RED proof: the store classifier (three lanes: base-options installation, real-traffic message shape, message-shape relabeling guard), the Connect arm, the MCP mapper, the CLI exit code on both the client and operator tiers, and the errors.md hint-code table gate"
    requirement: "REQ-exhausted-connect"
    verification:
      - kind: integration
        ref: "internal/store#TestRedEvidencePatchesAreLive (all 8 Phase 2 subtests)"
        status: pass
    human_judgment: false
  - id: D3
    description: "task (lint + test), task license:check, and git diff --exit-code go.mod/go.sum are all green at phase close, and go test ./internal/keylinks/ -count=1 passes"
    requirement: "REQ-exhausted-mcp"
    verification:
      - kind: integration
        ref: "task (full repo lint + test suite)"
        status: pass
      - kind: integration
        ref: "internal/keylinks (TestNoEscapedPatternsRepoWide, TestActiveMilestoneKeyLinksSatisfiable, and the full suite)"
        status: pass
    human_judgment: false
  - id: D4
    description: "No red-evidence patch or other .planning/** file carries an SPDX header; git status --porcelain after the final commits shows no modified tracked source file"
    requirement: "REQ-exhausted-cli-docs"
    verification:
      - kind: other
        ref: "rg -l 'SPDX-License-Identifier' .planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence/*.patch -> no matches; task license:check exits 0"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-09-19
status: complete
plan_head_before: 83297cdcd3d5dc6ac9176c8f8187388672d44b85
commits: 2
---

# Phase 2 Plan 4: Eight Registered Red-Evidence Patches Close the Phase Summary

**Eight hand-verified `.patch` files — one per Phase 2 lane (store classifier x3, Connect, MCP, CLI exit code x2, errors.md) — are registered in `redEvidenceDirs`, so `TestRedEvidencePatchesAreLive` now confirms 12 REDs (Phase 1's four plus Phase 2's eight) and `task` is fully green.**

## Performance

- **Duration:** ~25 min
- **Completed:** 2026-09-19
- **Tasks:** 2 completed
- **Files:** 9 changed (8 created, 1 modified)

## Accomplishments

- `02-02-connect-arm-removed.patch` (Task 1, tracer) — deletes `connectError`'s `ErrResponseTooLarge` case (case, log, and return lines), turning `TestConnectListMemoriesResponseTooLarge` RED (`internal` instead of `resource_exhausted`). Registered first and the harness re-run alone before any further patch was authored, proving the registration mechanism end to end on one lane (D-06).
- `02-01-classifier-not-in-base-options.patch` — deletes the `grpc.WithChainUnaryInterceptor(classifyResponseTooLarge)` line from `NewQdrantClient`'s base dial options, turning `TestResponseTooLargeClassifierSitsInsideCallerChain` RED (D-01).
- `02-01-classifier-requires-two-number-shape.patch` — adds a ` vs. ` requirement to `isRecvLimitMessage`, so the single-number receive shape this project's own traffic actually produces no longer classifies, turning both `TestStoreListOverflowIsResponseTooLarge` subtests RED (RESEARCH.md Pitfall 1).
- `02-01-classifier-relabels-any-resource-exhausted.patch` — drops the `isRecvLimitMessage` condition from `classifyResponseTooLarge`, relabeling every `ResourceExhausted`; five of `TestClassifyResponseTooLarge`'s subtests (the pass-through rows) go RED (D-02).
- `02-02-mcp-mapper-unregistered.patch` — `addToolMiddleware` registers only `instrumentTools`, turning `TestMCPListMemoryResponseTooLarge` RED (raw tool-result text leaks again) (D-08).
- `02-03-exit-mapping-reverted.patch` — deletes `exitCodeForConnectErr`'s `CodeResourceExhausted` case, turning both `TestExitCodeBaseline` response-too-large rows RED (exit 1 instead of 10) (D-07).
- `02-03-operator-arm-removed.patch` — deletes `classifyOperatorErr`'s `ErrResponseTooLarge` arm, turning `TestClassifyOperatorErrCodesAreDistinct` RED (the sentinel falls to the unclassified passthrough default) (D-07/D-10).
- `02-03-errors-doc-drops-too-large.patch` — deletes the `too_large` row from `errors.md`'s eleven-code hint table, turning `TestErrorsDocHintCodesMatchArgErrorConstants` RED (D-05).
- `internal/store/redevidence_harness_test.go` — one new `redEvidenceDirs` entry mapping all eight patches to their target tests, added across two commits (Task 1's Connect-arm mapping, then Task 2's remaining seven); Phase 1's entry is byte-unchanged.

## Task Commits

Each task was committed atomically:

1. **Task 1: One lane's RED proof end to end (tracer)** — `facb06bc` (test)
2. **Task 2 (LAST task of the phase): The remaining seven RED directions registered, full gate green** — `e8b6d968` (test)

**Plan metadata:** commit pending (this SUMMARY + STATE/ROADMAP/REQUIREMENTS)

## Files Created/Modified

- `.planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence/*.patch` (8 files) — the hand-verified mutations, one per lane.
- `internal/store/redevidence_harness_test.go` — the new Phase 2 `redEvidenceDirs` entry.

## Decisions Made

See `key-decisions` in frontmatter — the tracer-gate sequencing (prove one lane end to end before authoring the rest) and the smallest-mutation discipline applied uniformly across all eight patches.

## Deviations from Plan

None — plan executed exactly as written. Every RED observation required by the plan (Task 1's Connect-arm proof, Task 2's seven remaining proofs) is documented below as normal flow, not a deviation.

**Total deviations:** 0. **Impact:** None.

## RED Evidence (hand-verified, per patch)

| Patch | Target | Observed `--- FAIL:` |
|---|---|---|
| `02-02-connect-arm-removed.patch` | `TestConnectListMemoriesResponseTooLarge` | `--- FAIL: TestConnectListMemoriesResponseTooLarge (0.42s)` |
| `02-01-classifier-not-in-base-options.patch` | `TestResponseTooLargeClassifierSitsInsideCallerChain` | `--- FAIL: TestResponseTooLargeClassifierSitsInsideCallerChain (0.55s)` (subtest `receive_single-number_shape_becomes_the_sentinel`) |
| `02-01-classifier-requires-two-number-shape.patch` | `TestStoreListOverflowIsResponseTooLarge` | `--- FAIL: TestStoreListOverflowIsResponseTooLarge (1.89s)` (both `few-large`/`many-small` subtests) |
| `02-01-classifier-relabels-any-resource-exhausted.patch` | `TestClassifyResponseTooLarge` | `--- FAIL: TestClassifyResponseTooLarge (0.00s)` (5 pass-through subtests) |
| `02-02-mcp-mapper-unregistered.patch` | `TestMCPListMemoryResponseTooLarge` | `--- FAIL: TestMCPListMemoryResponseTooLarge (0.52s)` |
| `02-03-exit-mapping-reverted.patch` | `TestExitCodeBaseline` | `--- FAIL: TestExitCodeBaseline (0.62s)` (both `list/response-too-large`, `search/response-too-large` rows) |
| `02-03-operator-arm-removed.patch` | `TestClassifyOperatorErrCodesAreDistinct` | `--- FAIL: TestClassifyOperatorErrCodesAreDistinct (0.00s)` |
| `02-03-errors-doc-drops-too-large.patch` | `TestErrorsDocHintCodesMatchArgErrorConstants` | `--- FAIL: TestErrorsDocHintCodesMatchArgErrorConstants (0.01s)` |

`TestRedEvidencePatchesAreLive` then confirmed all twelve (Phase 1's four plus these eight) via its own `confirmed RED:` log lines, and left every touched file clean (`git status --porcelain` empty for `internal/store/store.go`, `internal/store/responsetoolarge.go`, `internal/server/connecterror.go`, `internal/server/instrument.go`, `cmd/engram/client_common.go`, `cmd/engram/operror.go`, and `docs-site/src/content/docs/reference/errors.md`) after every apply/revert cycle.

## Issues Encountered

None. Docker was reachable throughout (`docker info` exit 0), satisfying both tasks' precondition; every real-Qdrant testcontainer used by the hand-verification runs and the harness itself terminated cleanly, confirmed in each run's log output — no container was left running.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 2 (Error Classification & ResourceExhausted Mapping) is complete: the receive-limit classifier, the Connect and MCP mappings, the CLI exit code on both tiers, the errors.md hint-code table, and this phase's own registered red-evidence are all in place and green.
- `task` (lint + test) is green repo-wide; `TestRedEvidencePatchesAreLive` passes with 12 confirmed REDs; `task license:check` exits 0; `git diff --exit-code main...HEAD -- go.mod go.sum` exits 0.
- `go test ./internal/keylinks/ -count=1` passes (checked below, per this plan's phase-expectations gate).
- All four of the phase's requirements (REQ-exhausted-sentinel, REQ-exhausted-connect, REQ-exhausted-mcp, REQ-exhausted-cli-docs) are satisfied and ready to mark complete via the shared-ID gate now that this last declaring plan's SUMMARY exists.
- No blockers or concerns for the next phase.

---
*Phase: 02-error-classification-resourceexhausted-mapping*
*Completed: 2026-09-19*

## Self-Check: PASSED

All 8 created patch files verified present on disk; the modified `internal/store/redevidence_harness_test.go` verified present with both new entries. Both task commits (`facb06bc`, `e8b6d968`) verified present in `git log --oneline --all`. Every acceptance criterion for both tasks re-run and confirmed passing: each patch's `rg -c '^diff --git a/'` prints `1`; `redEvidenceDirs` carries exactly one Phase 2 entry with all eight patches mapped to their target tests (12 total names including Phase 1's four); no red-evidence patch carries an SPDX header; `task license:check` exits 0. `TestRedEvidencePatchesAreLive` passes with 12 `confirmed RED:` lines and a clean tree; `task` (lint + test) exits 0 repo-wide; `go test ./internal/keylinks/ -count=1` exits 0; `git diff --exit-code main...HEAD -- go.mod go.sum` exits 0; `git status --porcelain` after the final commit shows no modified tracked source file (only the pre-existing untracked `.planning/milestone.lock`, left untouched per the orchestrator's own session-lock convention).
