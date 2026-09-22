---
phase: 03-shared-bounded-read-mechanism-content-cap-decision
plan: 06
subsystem: testing
tags: [red-evidence, ast-gate, ci, regression-proof, phase-close]

requires:
  - phase: 03-shared-bounded-read-mechanism-content-cap-decision (plans 01-05)
    provides: "Every write cap (D-01/D-09/D-10), the byte-budget sweep primitive (D-02/D-06/D-07), the ordered-page primitive (D-02/D-03/D-07/D-08), the update-lane caps and production RecordCaps wiring (D-02/D-09), and the CLI/docs proof — every lane this plan's thirteen patches revert"
provides:
  - "redEvidenceDirs[\".planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/red-evidence\"]: thirteen hand-verified RED patches, one per Phase 3 lane"
  - "TestRedEvidencePatchesAreLive confirms 25 REDs (Phase 1's four, Phase 2's eight, Phase 3's thirteen) for the rest of the open milestone"
  - "A green task closes Phase 3: REQ-byte-budget-pages and REQ-content-cap-decided are both proven RED-able against real source"
affects: []

actuals:
  tokens: 4397
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "One-mutation-per-patch red-evidence authored by hand-verifying git apply --check / apply / go test -run '^Target$' (observing the target's own --- FAIL: line, never a build break) / apply -R before registration — Phase 1/2's own precedent (01-05-SUMMARY.md, 02-04-SUMMARY.md), repeated for all thirteen of Phase 3's lanes"

key-files:
  created:
    - .planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/red-evidence/03-01-content-cap-removed.patch
    - .planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/red-evidence/03-01-tags-cap-removed.patch
    - .planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/red-evidence/03-01-config-accepts-zero.patch
    - .planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/red-evidence/03-02-sweep-count-not-byte-derived.patch
    - .planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/red-evidence/03-02-ceiling-drops-citations.patch
    - .planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/red-evidence/03-02-sweep-fallback-removed.patch
    - .planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/red-evidence/03-02-sweep-swallows-single-overflow.patch
    - .planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/red-evidence/03-03-update-content-cap-removed.patch
    - .planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/red-evidence/03-03-update-gates-on-presence-not-change.patch
    - .planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/red-evidence/03-03-record-caps-not-wired.patch
    - .planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/red-evidence/03-04-ordered-page-ignores-page-budget.patch
    - .planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/red-evidence/03-04-budget-cut-reported-exhausted.patch
    - .planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/red-evidence/03-04-ordered-page-tie-exclusion-removed.patch
  modified:
    - internal/store/redevidence_harness_test.go

key-decisions:
  - "Task 1 (type=tracer) proved one full lane end to end (author, hand-verify RED, register, re-run the harness alone) before Task 2 authored the remaining twelve — the tracer feedback gate (interactive, human_verify_mode=end-of-phase default, Task 1's <verify> carrying only <automated>) re-ran that same verify and passed, so execution proceeded to Task 2 without a checkpoint."
  - "Each of the thirteen mutations is the smallest change that trips exactly its target test while the tree still compiles — deleting one call block, widening or narrowing one condition, dropping one arithmetic term, or collapsing a helper to its input — matching Phase 1/2's own established mutation-size discipline."
  - "The runtime contingency (narrowing the harness's nested go test to the declaring package) was pre-authorized but did NOT fire: the default-timeout ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -count=1 run completed in 111s, well under go test's 10-minute default — no packageOfTest helper was added."

requirements-completed: [REQ-byte-budget-pages, REQ-content-cap-decided]

coverage:
  - id: D1
    description: "redEvidenceDirs gains exactly one new entry for Phase 3, mapping all thirteen patches to their target tests; the Phase 1 and Phase 2 entries are byte-unchanged"
    requirement: "REQ-byte-budget-pages"
    verification:
      - kind: unit
        ref: "internal/store#TestRedEvidencePatchesAreLive/.planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/red-evidence"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every Phase 3 guarantee has a live, harness-checked RED proof: the create-path content/tags caps and the always-enforced config validation (D-01/D-09/D-10); the byte-budget sweep's count derivation, the full-record ceiling's citations term, the batch-of-1 fallback and its named-failure guarantee (D-02/D-07); the update-lane content cap and its presence-vs-change gate, and the production RecordCaps wiring (D-02/D-07/D-09); and the ordered page's byte budget, its Exhausted/CutByBudget distinction, and the tie-exclusion filter (D-02/D-03, REQ-list-contract-unchanged)"
    requirement: "REQ-content-cap-decided"
    verification:
      - kind: integration
        ref: "internal/store#TestRedEvidencePatchesAreLive (all 13 Phase 3 subtests)"
        status: pass
    human_judgment: false
  - id: D3
    description: "task (lint + test), task license:check, git diff --exit-code -- go.mod go.sum, and go test ./internal/keylinks/ -count=1 are all green at phase close; the default-timeout ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -count=1 run stays well under go test's 10-minute default"
    requirement: "REQ-byte-budget-pages"
    verification:
      - kind: integration
        ref: "task (full repo lint + test suite)"
        status: pass
      - kind: integration
        ref: "internal/keylinks (TestNoEscapedPatternsRepoWide, TestActiveMilestoneKeyLinksSatisfiable, and the full suite)"
        status: pass
    human_judgment: false
  - id: D4
    description: "No red-evidence patch or other .planning/** file carries an SPDX header; git status --porcelain after the final commit shows no modified tracked source file"
    requirement: "REQ-content-cap-decided"
    verification:
      - kind: other
        ref: "rg -l 'SPDX-License-Identifier' .planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/red-evidence/*.patch -> no matches; task license:check exits 0"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-09-19
status: complete
plan_head_before: 3f3db98e69e8e6c532c285df4588d158f23b986d
commits: 2
---

# Phase 3 Plan 6: Thirteen Registered Red-Evidence Patches Close the Phase Summary

**Thirteen hand-verified `.patch` files — three for the write caps (D-01/D-09/D-10), four for the byte-budget sweep primitive (D-02/D-07, plus the citations-ceiling pitfall), three for the update-lane caps and production RecordCaps wiring (D-02/D-07/D-09), and three for the ordered-page primitive (D-02/D-03, REQ-list-contract-unchanged) — are registered in `redEvidenceDirs`, so `TestRedEvidencePatchesAreLive` now confirms 25 REDs (Phase 1's four, Phase 2's eight, Phase 3's thirteen) and `task` is fully green, closing Phase 3.**

## Performance

- **Duration:** ~20 min
- **Completed:** 2026-09-19
- **Tasks:** 2 completed
- **Files:** 14 changed (13 created, 1 modified)

## Accomplishments

- `03-01-content-cap-removed.patch` (Task 1, tracer) — deletes `validateStoreArgs`' `checkContentBytes` call block, turning `TestMemoryWriteCapsRejectOnEveryCreateLane` RED on the content-too-long subtests across MCP and Connect's store/schedule/supersede lanes. Registered first and the harness re-run alone before any further patch was authored, proving the registration mechanism end to end on one lane (D-01).
- `03-01-tags-cap-removed.patch` — deletes `validateStoreArgs`' `checkTags` call, turning `TestMemoryWriteCapBoundaries`'s tags-over-count/tag-over-bytes/order/configured-caps-tags subtests RED (D-10).
- `03-01-config-accepts-zero.patch` — deletes `validatePositiveCap`'s zero-rejection arm, turning `TestMemoryCapsRejectZeroAndNonPositive` RED (D-09).
- `03-02-sweep-count-not-byte-derived.patch` — collapses `sweepLimit` to always return `spineScrollBatch`, turning `TestScrollAllPointsByteBudget`'s full-view subtests RED (D-02).
- `03-02-ceiling-drops-citations.patch` — drops the `Citations*(CitationExcerptBytes+citationEntryAllowance)` term from `fullRecordCeiling`, turning `TestRecordCeilingHoldsForMaxCapRecord` RED (RESEARCH.md's "content alone" pitfall).
- `03-02-sweep-fallback-removed.patch` — deletes `scrollAllPoints`' batch-of-1 fallback branch, turning `TestScrollAllPointsBatchOfOneFallback` RED (D-07).
- `03-02-sweep-swallows-single-overflow.patch` — makes a single-record overflow at the fallback's `Limit: 1` end the sweep with `return nil` instead of the named error, turning `TestScrollAllPointsSingleOversizedRecordFailsNamed` RED (D-07, no silent skip).
- `03-03-update-content-cap-removed.patch` — deletes the content-cap check inside `deps.updateMemory`, turning `TestUpdateMemoryContentCap`'s over-cap Connect/MCP subtests RED (D-09).
- `03-03-update-gates-on-presence-not-change.patch` — widens that check's `contentChanged` gate to `a.Content != nil`, turning `TestUpdateMemoryLegacyOversizedRecord`'s resend-same-content subtest RED (D-07/D-09: an unchanged legacy record must never be rejected).
- `03-03-record-caps-not-wired.patch` — drops `store.WithRecordCaps(recordCapsFromConfig(cfg))` from `storeFromConfig`, turning `TestStoreFromConfigCarriesRecordCaps` RED (D-02/D-09).
- `03-04-ordered-page-ignores-page-budget.patch` — removes the remaining-page-budget clamp from `scrollOrderedPage`'s per-RPC count, turning `TestScrollOrderedPageByteBudget`'s full-view subtests RED (D-02).
- `03-04-budget-cut-reported-exhausted.patch` — sets `exhausted = true` alongside `cutByBudget = true`, turning the same test's tie-visit-count assertions RED (REQ-list-contract-unchanged: a budget-cut page must never also report exhausted).
- `03-04-ordered-page-tie-exclusion-removed.patch` — collapses `excludeSeen` to return its input filter unchanged, turning `TestScrollOrderedPageTiesAcrossRPCBoundaries` RED (D-03: an id tied at the boundary is visited twice).
- `internal/store/redevidence_harness_test.go` — one new `redEvidenceDirs` entry mapping all thirteen patches to their target tests, added across two commits (Task 1's content-cap mapping, then Task 2's remaining twelve); Phase 1 and Phase 2's entries are byte-unchanged.

## Task Commits

Each task was committed atomically:

1. **Task 1: One lane's RED proof end to end (tracer)** — `21bee406` (test)
2. **Task 2 (LAST task of the phase): The remaining twelve RED directions registered, full gate green** — `e10c1331` (test)

**Plan metadata:** commit pending (this SUMMARY + STATE/ROADMAP/REQUIREMENTS)

## Files Created/Modified

- `.planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/red-evidence/*.patch` (13 files) — the hand-verified mutations, one per lane.
- `internal/store/redevidence_harness_test.go` — the new Phase 3 `redEvidenceDirs` entry.

## Decisions Made

See `key-decisions` in frontmatter — the tracer-gate sequencing (prove one lane end to end before authoring the rest), the smallest-mutation discipline applied uniformly across all thirteen patches, and the runtime-contingency non-trigger (111s default-timeout run, no per-package narrowing needed).

## Deviations from Plan

None — plan executed exactly as written. Every RED observation required by the plan (Task 1's content-cap proof, Task 2's twelve remaining proofs) is documented below as normal flow, not a deviation.

**Total deviations:** 0. **Impact:** None.

## RED Evidence (hand-verified, per patch)

| Patch | Target | Observed `--- FAIL:` |
|---|---|---|
| `03-01-content-cap-removed.patch` | `TestMemoryWriteCapsRejectOnEveryCreateLane` | `--- FAIL: TestMemoryWriteCapsRejectOnEveryCreateLane (0.01s)` (mcp/connect store/schedule/supersede content-too-long subtests) |
| `03-01-tags-cap-removed.patch` | `TestMemoryWriteCapBoundaries` | `--- FAIL: TestMemoryWriteCapBoundaries (0.00s)` (tags-over-count, tag-over-bytes, order-count-before-bytes, configured-caps-tags-count, configured-caps-tag-bytes subtests) |
| `03-01-config-accepts-zero.patch` | `TestMemoryCapsRejectZeroAndNonPositive` | `--- FAIL: TestMemoryCapsRejectZeroAndNonPositive (0.00s)` |
| `03-02-sweep-count-not-byte-derived.patch` | `TestScrollAllPointsByteBudget` | `--- FAIL: TestScrollAllPointsByteBudget (1.32s)` (few-large/full, many-small/full: `SweepLimit(FullView()) = 256, want 2`) |
| `03-02-ceiling-drops-citations.patch` | `TestRecordCeilingHoldsForMaxCapRecord` | `--- FAIL: TestRecordCeilingHoldsForMaxCapRecord (2.96s)` (`proto.Size(p) = 928143, exceeds ViewMaxRecordBytes 99840`) |
| `03-02-sweep-fallback-removed.patch` | `TestScrollAllPointsBatchOfOneFallback` | `--- FAIL: TestScrollAllPointsBatchOfOneFallback (0.45s)` (`qdrant response exceeded the client's receive limit`) |
| `03-02-sweep-swallows-single-overflow.patch` | `TestScrollAllPointsSingleOversizedRecordFailsNamed` | `--- FAIL: TestScrollAllPointsSingleOversizedRecordFailsNamed (0.26s)` (`got nil error, want a non-nil error wrapping store.ErrResponseTooLarge`) |
| `03-03-update-content-cap-removed.patch` | `TestUpdateMemoryContentCap` | `--- FAIL: TestUpdateMemoryContentCap (0.01s)` (connect_over, mcp_over subtests) |
| `03-03-update-gates-on-presence-not-change.patch` | `TestUpdateMemoryLegacyOversizedRecord` | `--- FAIL: TestUpdateMemoryLegacyOversizedRecord (0.02s)` (resend-same-content subtest) |
| `03-03-record-caps-not-wired.patch` | `TestStoreFromConfigCarriesRecordCaps` | `--- FAIL: TestStoreFromConfigCarriesRecordCaps (0.00s)` (`RecordCaps() = {...defaults...}, want {ContentBytes:1000 ...}`) |
| `03-04-ordered-page-ignores-page-budget.patch` | `TestScrollOrderedPageByteBudget` | `--- FAIL: TestScrollOrderedPageByteBudget (1.63s)` (`page 0: Bytes = 5266400, exceeds PageByteBudget 2097152`) |
| `03-04-budget-cut-reported-exhausted.patch` | `TestScrollOrderedPageByteBudget` | `--- FAIL: TestScrollOrderedPageByteBudget (1.33s)` (tie-visit-count assertions: `id ... visited 0 time(s), want exactly 1`) |
| `03-04-ordered-page-tie-exclusion-removed.patch` | `TestScrollOrderedPageTiesAcrossRPCBoundaries` | `--- FAIL: TestScrollOrderedPageTiesAcrossRPCBoundaries (0.21s)` (`id ... visited twice`) |

`TestRedEvidencePatchesAreLive` then confirmed all twenty-five (Phase 1's four, Phase 2's eight, Phase 3's thirteen) via its own `confirmed RED:` log lines, and left every touched file clean (`git status --porcelain` empty for `internal/server/tools.go`, `internal/config/validate.go`, `internal/store/boundedread.go`, `internal/store/spine.go`, and `internal/store/orderedpage.go`) after every apply/revert cycle. Total harness run: 102.60s.

## Phase Gate Verification

- `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1 -v -timeout 60m`: `ok` — 25 `confirmed RED:` lines.
- `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -count=1` (default timeout, no `-timeout` flag): `ok` in 111.25s real — well under `go test`'s 10-minute default. **The pre-authorized runtime contingency (narrowing the harness's nested `go test` to the declaring package via a new `packageOfTest` helper) did NOT fire** — no such helper was added, and the harness's doc comment describing the nested `go test` is unchanged.
- `ENGRAM_REQUIRE_QDRANT=1 task` (lint + test, full repo): `ok` across every package including `internal/store` (112.04s) and `internal/e2e` (8.81s).
- `go test ./internal/keylinks/ -count=1 -v`: `ok` — all 11 subtests pass, including `TestActiveMilestoneKeyLinksSatisfiable` and `TestNoEscapedPatternsRepoWide`.
- `task license:check`: `ok` — 2066 files checked, 447 valid, 0 invalid.
- `git diff --exit-code HEAD -- go.mod go.sum`: clean (no dependency drift; this plan added none).
- `git status --porcelain` after the final commit: empty (only the pre-existing untracked `.planning/milestone.lock`, the orchestrator's own session lock, left untouched).

## Issues Encountered

None. Docker was reachable throughout (`docker info` exit 0), satisfying both tasks' precondition; every real-Qdrant testcontainer used by the hand-verification runs and the harness itself terminated cleanly, confirmed in each run's log output — no container was left running.

## Known Stubs

None. This plan authors only test-harness registrations and `.planning/**` red-evidence patches — no production code, no stubs.

## Threat Flags

None beyond this plan's own `<threat_model>` register (T-03-06-01 through T-03-06-03, T-03-06-SC), which already covers every new surface this plan introduces (a patch left applied after a failed run, a stale patch that no longer proves RED, and the harness outgrowing the default test timeout); all three mitigations held — the harness reverted every mutation cleanly, every patch was hand-verified before registration, and the default-timeout run stayed well within budget.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 3 (Shared Bounded-Read Mechanism & Content Cap Decision) is complete: both shared bounded-read primitives (the byte-budget sweep extension and the ordered-page helper), the content/tags write caps and their always-enforced config validation, the update-lane caps, the production RecordCaps wiring, the D-08 call-site inventory, the CLI proof, the documentation, and this phase's own registered red-evidence are all in place and green.
- `task` (lint + test) is green repo-wide; `TestRedEvidencePatchesAreLive` passes with 25 confirmed REDs; `task license:check` exits 0; `git diff --exit-code -- go.mod go.sum` exits 0.
- `go test ./internal/keylinks/ -count=1` passes — the shared-ID gate now marks REQ-byte-budget-pages and REQ-content-cap-decided complete, since this is the last plan declaring either requirement.
- Both primitives (`scrollAllPoints`'s byte-budget extension, `scrollOrderedPage`) and the D-08 inventory are ready for Phase 4 (recall-path migrations: `Store.List`/`listByCursor`/`ListScheduled`/`Search`/`SearchDiscovery`) and Phase 5 (the five operator sweeps, plus `REQ-recv-limit-backstop` and `REQ-ci-store-green`).
- No blockers.

---
*Phase: 03-shared-bounded-read-mechanism-content-cap-decision*
*Completed: 2026-09-19*

## Self-Check: PASSED

- All 13 created patch files confirmed present on disk; the modified `internal/store/redevidence_harness_test.go` verified present with all 25 entries (Phase 1's four, Phase 2's eight, Phase 3's thirteen).
- Both task commit hashes (`21bee406`, `e10c1331`) confirmed present in `git log --oneline --all`.
- Every acceptance criterion for both tasks re-run and confirmed passing: each patch's `rg -o '^diff --git a/'` prints `1` (13/13); `redEvidenceDirs` carries exactly one Phase 3 entry mapping all thirteen patch names; no red-evidence patch carries an SPDX header; `task license:check` exits 0; the diff from Task 1's commit's parent to HEAD shows zero removed `"01-`/`"02-`-prefixed lines (Phase 1/2 entries byte-unchanged); Task 1's own commit diff shows zero removed lines (registration-only).
- `TestRedEvidencePatchesAreLive` passes with 25 `confirmed RED:` lines and a clean tree; `ENGRAM_REQUIRE_QDRANT=1 task` (lint + test) exits 0 repo-wide; the default-timeout `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -count=1` run completed in 111.25s (no runtime-contingency trigger); `go test ./internal/keylinks/ -count=1` exits 0; `task license:check` exits 0; `git diff --exit-code HEAD -- go.mod go.sum` exits 0; `git status --porcelain` after the final commit shows no modified tracked source file (only the pre-existing untracked `.planning/milestone.lock`, left untouched per the orchestrator's own session-lock convention).
