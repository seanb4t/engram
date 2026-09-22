---
phase: 04-list-listscheduled-search-bounded-reads
plan: 08
subsystem: testing
tags: [red-evidence, ast-gate, ci, regression-proof, phase-close]

# Dependency graph
requires:
  - phase: 04-list-listscheduled-search-bounded-reads (plans 01-07)
    provides: "The renamed/expanded hint catalog and overflow envelope (04-01), Store.List's zero-limit/deep-offset/cursor bounding (04-02/04-03), the two-phase bounded search (04-04), the shared no-summary backfill and projection selection plus the store's over-maximum backstop (04-05), the server-side out_of_range rejection wired ahead of the embed call and threaded to rule listing (04-06), and the published numeric recall maximum across every doc/CLI surface (04-07) — every guarantee this plan's seventeen patches revert"
  - phase: 03-shared-bounded-read-mechanism-content-cap-decision (plan 06)
    provides: "the redEvidenceDirs harness and its one-mutation-per-patch authoring/hand-verification procedure this plan repeats for phase 4"
provides:
  - "redEvidenceDirs[\".planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence\"]: seventeen hand-verified RED patches, one per phase-4 guarantee"
  - "TestRedEvidencePatchesAreLive confirms 42 REDs (Phase 1's four, Phase 2's eight, Phase 3's thirteen, phase 4's seventeen) for the rest of the open milestone"
  - "A green task closes phase 4: REQ-list-bounded, REQ-list-scheduled-bounded, REQ-search-k-bounded, REQ-list-contract-unchanged and REQ-list-limit-contract-decided are all proven RED-able against real source and marked complete"
affects: [05]

# Actuals (#2632)
actuals:
  tokens: 8213
  tasks: 3
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "One-mutation-per-patch red-evidence authored by hand-verifying git apply --check / apply / go test -run '^Target$' (observing the target's own failure — an assertion failure, a genuine runtime panic from a real out-of-range slice allocation, or a real RPC rejection — never a build break) / apply -R before registration — Phase 1/2/3's own precedent (01-05, 02-04, 03-06 SUMMARYs), repeated for all seventeen of phase 4's lanes"
    - "When a plan's own described mutation ('restore the zero-limit resolution to the exact matched total') would be a no-op against the current oversized fixtures (both shapes seed at or under store.MaxRecallLimit, so any generous bound produces an identical item count), the historically-accurate revert — recovered from git history at the plan's own pre-fix commit — reproduces the real regression (an unbounded single Scroll overflowing the receive limit) and is the one actually registered"

key-files:
  created:
    - .planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence/04-01-hint-code-value-drift.patch
    - .planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence/04-01-overflow-envelope-hint-reverted.patch
    - .planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence/04-02-cursor-reports-last-page-on-budget-cut.patch
    - .planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence/04-02-zero-limit-fetches-whole-scope.patch
    - .planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence/04-02-offset-overflow-guard-removed.patch
    - .planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence/04-03-prefix-walk-uses-full-view.patch
    - .planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence/04-03-assembly-loop-stops-after-one-page.patch
    - .planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence/04-04-search-fetch-drops-caller-filter.patch
    - .planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence/04-04-search-fetch-always-issues-an-rpc.patch
    - .planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence/04-04-search-returns-reversed-rank-order.patch
    - .planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence/04-05-no-summary-backfill-removed.patch
    - .planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence/04-05-list-always-full-view.patch
    - .planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence/04-05-store-clamps-instead-of-refusing.patch
    - .planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence/04-06-surface-maximum-check-removed.patch
    - .planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence/04-06-maximum-check-runs-after-embed.patch
    - .planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence/04-06-full-not-threaded-to-rule-listing.patch
    - .planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence/04-07-cli-help-drops-the-maximum.patch
  modified:
    - internal/store/redevidence_harness_test.go
    - .planning/REQUIREMENTS.md
    - .planning/WINDOWS.md

key-decisions:
  - "Task 1 (type=tracer) proved one full lane end to end (author, hand-verify RED, register, re-run the harness alone at 26 confirmed REDs) before Task 2 authored the remaining sixteen — the tracer feedback gate (interactive, human_verify_mode=end-of-phase default, Task 1's <verify> carrying only <automated>) re-ran that same verify and passed, so execution proceeded to Task 2 without a checkpoint."
  - "04-02-zero-limit-fetches-whole-scope.patch's literal plan description ('restore ... to the exact matched total instead of the recall maximum') was hand-verified to be a NO-OP against the current fixtures: both storetest shapes (few-large=40, many-small=1000=MaxRecallLimit) seed at or under the maximum, so any generous resolution (total, MaxRecallLimit, or unbounded) yields an identical item count — empirically confirmed (mutation applied, target ran GREEN). The patch actually registered instead restores the pre-04-02 offset-mode zero-limit code (recovered from git history at commit 5144c449, the plan's own plan_head_before) — one unbounded s.client.Scroll requesting the full matched total with a full-payload selector — which reproduces the REAL historical regression (a Scroll response overflowing the receive limit) and genuinely trips TestStoreListOffsetBounded. Documented here per this phase's established literal-vs-intent resolution pattern."
  - "04-02-offset-overflow-guard-removed.patch's target test fails via a genuine runtime panic (`makeslice: cap out of range` inside walkOffsetPrefix/scrollOrderedPage, reached because the wrap guard no longer rejects an Offset near math.MaxUint64) rather than a plain assertion failure — hand-verified as a real, non-recovered process exit (`go test` exits 1, `*exec.ExitError`, not a build break) that the harness's own exec.Command.Run()/errors.As check accepts identically to an ordinary --- FAIL: line."
  - "Removing store.go's overflow guard also orphaned its sole `math` import; the patch additionally removes that import line so the mutated tree still compiles — the smallest coherent mutation, not a build break."
  - "The REQUIREMENTS.md Task 3 acceptance criterion asserting the total ticked-requirement count equals 12 was hand-verified to under-count by one: 8 requirements were already complete before this plan (not 7 as the criterion assumes), so the correct post-tick total is 13. Verified against the criterion's own stated INTENT ('no other phase's box moved') via `git diff` scoped to exactly this phase's five rows, rather than enforcing the literal miscounted number — the same resolution pattern every other plan in this phase applied to its own acceptance-criteria/intent gaps."
  - "REQUIREMENTS.md's trailing 'Last updated' line was left untouched: `git show` on every prior phase-close commit (01-05's a4da3b8d, 02-04's 5ca7dbaf, 03-06's 04267110) confirms none of them touched that line — the plan's instruction to 'update it the way the previous phase's close did' is followed by doing what the previous phase's close actually did (nothing), not by inventing an edit no precedent supports."
  - "WINDOWS.md entry 11 (the key_links pattern gap already repaired by commit 4afd5b75, immediately before this plan started) was closed via the ledger's own `gsd-tools windows fixed 11` verb, per the orchestrator's cross-plan note inviting this cleanup — a separate small commit, since it is not named in this plan's files_modified and the ledger is a tool-owned file whose structure is never hand-edited."
  - "The pre-authorized runtime contingency (narrowing the harness's nested `go test` to the declaring package) did NOT fire: the default-timeout `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -count=1` run completed in 191.96s, well under go test's 10-minute default at forty-two patches — no packageOfTest helper was added."

requirements-completed: [REQ-list-bounded, REQ-list-scheduled-bounded, REQ-search-k-bounded, REQ-list-contract-unchanged, REQ-list-limit-contract-decided]

coverage:
  - id: D1
    description: "redEvidenceDirs gains exactly one new entry for phase 4, mapping all seventeen patches to their target tests; the Phase 1, Phase 2 and Phase 3 entries are byte-unchanged"
    requirement: "REQ-list-contract-unchanged"
    verification:
      - kind: unit
        ref: "internal/store#TestRedEvidencePatchesAreLive/.planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every phase-4 guarantee has a live, harness-checked RED proof: the hint catalog's agreement with errors.md and the overflow envelope's hint (D-10/D-11); a budget-cut cursor page reported as the last page, a zero limit reverting to an unbounded fetch, and the offset+limit wrap guard (D-01/D-06); the deep-offset prefix walk's keys-only projection and the assembly loop following its resume position across multiple pages (D-05/D-07); the search fetch's empty-batch short-circuit and rank-order preservation, and the id-set filter's caller-filter wrap (D-09); the no-summary backfill, the per-caller projection, and the store's refusal-not-clamp (Phase 3 D-04, D-10); the surface rejection, its position ahead of the embedder, and the rule listing's projection threading (D-02/D-03/D-10); and the CLI help stating the maximum (D-01/D-04)"
    requirement: "REQ-list-bounded"
    verification:
      - kind: integration
        ref: "internal/store#TestRedEvidencePatchesAreLive (all 17 phase-4 subtests)"
        status: pass
    human_judgment: false
  - id: D3
    description: "task (lint + full test suite), task license:check, git diff --exit-code -- go.mod go.sum, and go test ./internal/keylinks/ -count=1 are all green at phase close; the default-timeout ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -count=1 run stays well under go test's 10-minute default at forty-two registered patches"
    requirement: "REQ-search-k-bounded"
    verification:
      - kind: integration
        ref: "task (full repo lint + test suite)"
        status: pass
      - kind: integration
        ref: "internal/keylinks (TestNoEscapedPatternsRepoWide, TestActiveMilestoneKeyLinksSatisfiable, and the full suite)"
        status: pass
    human_judgment: false
  - id: D4
    description: "The phase's five requirements (REQ-list-bounded, REQ-list-scheduled-bounded, REQ-search-k-bounded, REQ-list-contract-unchanged, REQ-list-limit-contract-decided) are ticked in both the checklist and the traceability table; no other phase's requirement, verification document, or context document is touched"
    requirement: "REQ-list-scheduled-bounded"
    verification:
      - kind: other
        ref: "rg -c -e '^- \\[x\\] \\*\\*REQ-(list-bounded|list-scheduled-bounded|search-k-bounded|list-contract-unchanged|list-limit-contract-decided)\\*\\*' .planning/REQUIREMENTS.md -> 5; rg -c -e 'Phase 4 \\| Complete' -> 5; rg -c -e 'Phase 4 \\| Pending' -> 0; git diff --exit-code HEAD -- .planning/phases/02-error-classification-resourceexhausted-mapping .planning/phases/03-shared-bounded-read-mechanism-content-cap-decision -> clean"
        status: pass
    human_judgment: false
  - id: D5
    description: "No red-evidence patch or other .planning/** file carries an SPDX header; no version-bearing (✅/📋/🚧) heading was introduced into REQUIREMENTS.md"
    requirement: "REQ-list-limit-contract-decided"
    verification:
      - kind: other
        ref: "rg -l 'SPDX-License-Identifier' .planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence/*.patch -> no matches; task license:check exits 0; rg -o -e '✅|📋|🚧' .planning/REQUIREMENTS.md | wc -l -> 0 (unchanged)"
        status: pass
    human_judgment: false

duration: 65min
completed: 2026-09-20
status: complete
plan_head_before: 4afd5b752289e6d5fc63132101fd36bcc6ae0162
commits: 4
---

# Phase 4 Plan 8: Seventeen Registered Red-Evidence Patches Close the Phase Summary

**Seventeen hand-verified `.patch` files — two for the hint vocabulary and overflow envelope, three for `Store.List`'s bounded offset/cursor/deep-offset modes, two for `ListScheduled`'s bounded assembly, three for the two-phase bounded search, three for the shared projection/backfill/over-maximum-guard mechanism, three for the server-side out-of-range rejection and its rule-listing threading, and one for the published CLI maximum — are registered in `redEvidenceDirs`, so `TestRedEvidencePatchesAreLive` now confirms 42 REDs (Phase 1's four, Phase 2's eight, Phase 3's thirteen, phase 4's seventeen), `task` is fully green, and the phase's five requirements are marked complete, closing phase 4.**

## Performance

- **Duration:** ~65 min
- **Completed:** 2026-09-20
- **Tasks:** 3 completed
- **Files:** 21 changed (17 created, 4 modified)

## Accomplishments

- `04-04-search-fetch-drops-caller-filter.patch` (Task 1, tracer) — deletes `includeIDs`' wrap of the caller's own filter as a nested condition, leaving only the id-set inclusion; turns `TestSearchTwoPhaseBounded`'s cross-owner subtest RED (another owner's private record leaks into a batched fetch). Registered first and the harness re-run alone (26 confirmed REDs) before any further patch was authored, proving the registration mechanism end to end on phase 4's highest-value lane (D-09).
- `04-01-hint-code-value-drift.patch` — drifts `HintOutOfRange`'s wire value from `out_of_range` to `out_of_bounds`, turning `TestErrorsDocHintCodesMatchArgErrorConstants` RED (D-10).
- `04-01-overflow-envelope-hint-reverted.patch` — makes the shared overflow envelope render `HintTooLong` instead of `HintResponseTooLarge`, turning `TestResponseTooLargeEnvelopeShape` RED (D-11).
- `04-02-cursor-reports-last-page-on-budget-cut.patch` — makes `listByCursor` treat `page.CutByBudget` the same as `page.Exhausted` (empty next cursor), turning `TestStoreListContractInvariant` RED (D-06: a budget-cut page must never read as the last page).
- `04-02-zero-limit-fetches-whole-scope.patch` — restores the pre-04-02 offset-mode zero-limit path (one unbounded `s.client.Scroll` requesting the full matched total with a full-payload selector, recovered from git history at the plan's own pre-fix commit), turning `TestStoreListOffsetBounded` RED with the exact historical failure (`qdrant response exceeded the client's receive limit`) — see key-decisions for why the plan's own literally-described mutation is a no-op against the current fixtures (D-01).
- `04-02-offset-overflow-guard-removed.patch` — deletes the `opts.Offset > math.MaxUint64-effectiveLimit` guard (and its now-orphaned `math` import), turning `TestStoreListOffsetBounded`'s `offset_plus_limit_overflows` subtest RED via a genuine runtime panic (`makeslice: cap out of range`) inside the now-unguarded prefix walk (D-01 precision).
- `04-03-prefix-walk-uses-full-view.patch` — makes `walkOffsetPrefix` request `s.fullView()` instead of `keysView()`, turning `TestStoreListDeepOffsetBounded` RED (no keys-only prefix RPC recorded) (D-07).
- `04-03-assembly-loop-stops-after-one-page.patch` — collapses `collectOrderedPages`' resume-following loop to a single `scrollOrderedPage` call, turning `TestListScheduledBounded` RED (many-small returns 193 of 1000 seeded records) (D-05).
- `04-04-search-fetch-always-issues-an-rpc.patch` — restructures `fetchPayloadsByID`'s batching loop into an always-run-at-least-once form (removing the empty-batch short-circuit while preserving normal-case batch boundaries), turning `TestSearchFetchSkipsEmptyBatch` RED: an empty id batch now issues a `Scroll` with `Limit: 0`, which Qdrant itself rejects (`limit: value 0 invalid, must be 1 or larger`) (D-09, RESEARCH Pitfall 1).
- `04-04-search-returns-reversed-rank-order.patch` — walks phase one's id order backwards when rebuilding `Store.Search`'s result, turning `TestSearchPreservesRankOrder` RED (D-09).
- `04-05-no-summary-backfill-removed.patch` — deletes `Store.List`'s call to `backfillNoSummaryContent` in offset mode (the helper stays referenced by `backfillNoSummaryContent`'s own caller and by `Search`, so the tree compiles), turning `TestNoSummaryContentBackfill` RED (Phase 3 D-04).
- `04-05-list-always-full-view.patch` — collapses `recallView` to always return `s.fullView()`, turning `TestRecallViewSelection` RED (the default list's Scroll selector no longer excludes content/citations) (Phase 3 D-04).
- `04-05-store-clamps-instead-of-refusing.patch` — collapses `rejectOverMaximum` to always return `nil`, turning `TestStoreRejectsOverMaximumCount` RED across List/ListScheduled/Search/SearchDiscovery's one-above-maximum subtests (D-10).
- `04-06-surface-maximum-check-removed.patch` — deletes `deps.listMemory`'s `rejectOverMaximumCount` call, turning `TestOutOfRangeRejectedOnEveryRecallSurface` RED: the over-maximum `ListMemories`/`list_memory` calls now fall through to the store's differently-shaped backstop error (`limit exceeds the maximum of 1000: invalid argument`, missing the `field=limit hint=out_of_range` envelope) (D-10).
- `04-06-maximum-check-runs-after-embed.patch` — moves `deps.searchMemory`'s over-maximum check to after the `EmbedQuery` call, turning `TestOutOfRangeRejectedBeforeAnyBackend` RED (the embedder is called before the over-maximum count is rejected) (D-10, the denial-of-service ordering hazard).
- `04-06-full-not-threaded-to-rule-listing.patch` — drops `Full: a.Full` from `listRules`' direct `store.ListOptions` literal, turning `TestListRulesFullThreaded` RED (a `full=true` rule read stays summary-shaped) (Pattern 6 step 4).
- `04-07-cli-help-drops-the-maximum.patch` — reverts the CLI `list --limit` flag's usage string to omit the number `1000`, turning `TestRecallMaximumIsStatedNumerically`'s `cli_list_limit_flag` subtest RED (D-01/D-04).
- `internal/store/redevidence_harness_test.go` — one new `redEvidenceDirs` entry mapping all seventeen patches to their target tests, added across two commits (Task 1's search-fetch mapping, then Task 2's remaining sixteen); Phase 1, Phase 2 and Phase 3's entries are byte-unchanged.
- `.planning/REQUIREMENTS.md` — this phase's five requirements ticked in both the checklist and the traceability table via `requirements mark-complete`, and nothing else touched.
- `.planning/WINDOWS.md` — entry 11 (the key_links pattern gap already repaired by commit `4afd5b75`) marked fixed via the ledger's own CLI verb.

## Task Commits

Each task was committed atomically:

1. **Task 1: One RED proof end to end — the search fetch's caller-filter wrap (tracer)** — `6160bac8` (test)
2. **Task 2: The remaining sixteen RED directions registered and the harness green at forty-two** — `3733a783` (test)
   - **Side commit: close WINDOWS.md entry 11** — `312d8cd4` (docs)
3. **Task 3 (LAST task of the phase): The five requirements marked complete and the phase gate closed** — `2dcfb0a3` (docs)

**Plan metadata:** commit pending (this SUMMARY + STATE/ROADMAP)

## Files Created/Modified

- `.planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence/*.patch` (17 files) — the hand-verified mutations, one per phase-4 lane.
- `internal/store/redevidence_harness_test.go` — the new phase-4 `redEvidenceDirs` entry.
- `.planning/REQUIREMENTS.md` — this phase's five requirements marked complete.
- `.planning/WINDOWS.md` — entry 11 marked fixed.

## Decisions Made

See `key-decisions` in frontmatter: the tracer-gate sequencing (prove one lane end to end before authoring the rest); the historically-accurate revert substituted for `04-02-zero-limit-fetches-whole-scope.patch`'s literal (but empirically no-op) plan description; the genuine-panic RED accepted for `04-02-offset-overflow-guard-removed.patch`; the orphaned `math` import removed alongside its guard; the REQUIREMENTS.md ticked-count criterion resolved against its own intent (13, not the miscounted 12) rather than hand-forced to match; the trailing "Last updated" line left untouched per actual (not claimed) precedent; and the WINDOWS.md entry 11 cleanup via its own ledger verb.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `04-02-zero-limit-fetches-whole-scope.patch`'s literal mutation description did not produce a RED against current fixtures**
- **Found during:** Task 2, authoring the sixteen remaining patches
- **Issue:** The plan's own text ("restore offset mode's zero-limit resolution to the exact matched total instead of the recall maximum") describes setting `effectiveLimit = total` instead of `MaxRecallLimit`. Hand-verified empirically: since both `storetest` oversized shapes (few-large=40, many-small=1000, pinned exactly to `store.MaxRecallLimit`) never seed a scope whose total exceeds the maximum, `effectiveLimit = total` and `effectiveLimit = MaxRecallLimit` produce an IDENTICAL item count in every case — the mutation ran the target test and it passed clean (no RED).
- **Fix:** Recovered the actual pre-04-02 offset-mode implementation from git history at commit `5144c449` (the plan's own recorded `plan_head_before` for 04-02) and restored its zero-limit path verbatim (adapted to current variable names): one unbounded `s.client.Scroll` requesting `total` records with a full-payload selector, bypassing `collectOrderedPages`/`walkOffsetPrefix` entirely. This reproduces the REAL historical D-01 regression and genuinely trips `TestStoreListOffsetBounded` with the exact `qdrant response exceeded the client's receive limit` failure 04-02-SUMMARY.md's own TDD RED evidence recorded.
- **Files modified:** `.planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence/04-02-zero-limit-fetches-whole-scope.patch` (patch content only; not a production-code change)
- **Verification:** Hand-verified apply → RED (`TestStoreListOffsetBounded` fails with the historical overflow message) → revert → clean tree; confirmed again by the full harness run (42/42 confirmed RED).
- **Committed in:** `3733a783` (Task 2 commit)

**2. [Rule 1 - Bug] `04-02-offset-overflow-guard-removed.patch` orphaned the `math` import**
- **Found during:** Task 2, authoring the same patch
- **Issue:** Deleting the `opts.Offset > math.MaxUint64-effectiveLimit` guard left `store.go`'s `"math"` import with no remaining reference, which would have broken the build (`"math" imported and not used`) — a build break is explicitly disallowed for a red-evidence patch.
- **Fix:** Removed the now-unused `"math"` import line in the same patch, so the mutated tree still compiles and the target test fails for the intended reason (a real, unrecovered panic from the now-unguarded near-`MaxUint64` offset) rather than a compile error.
- **Files modified:** `.planning/phases/04-list-listscheduled-search-bounded-reads/red-evidence/04-02-offset-overflow-guard-removed.patch` (patch content only)
- **Verification:** `go build ./...` clean with the patch applied; target test fails via a genuine runtime panic that `go test` reports as a non-zero exit (`*exec.ExitError`), which the harness's own `errors.As` check accepts identically to an ordinary `--- FAIL:` line.
- **Committed in:** `3733a783` (Task 2 commit)

**3. [Rule 1 - Bug] Task 3's own acceptance criterion for the total ticked-requirement count was miscounted**
- **Found during:** Task 3, before ticking the five requirements
- **Issue:** The plan's acceptance criterion asserts the total number of `[x]` REQ boxes equals 12 ("the seven that were already complete plus this phase's five"). Counting the file directly at plan start showed 8 requirements already complete (`REQ-oversized-fixture-helper`, `REQ-test-client-parity`, `REQ-exhausted-sentinel`, `REQ-exhausted-connect`, `REQ-exhausted-mcp`, `REQ-exhausted-cli-docs`, `REQ-byte-budget-pages`, `REQ-content-cap-decided`), not seven — so the correct post-tick total is 13.
- **Fix:** Verified the criterion's own stated INTENT instead of the miscounted literal number: confirmed via `git diff` that only this phase's five rows changed (no other phase's checkbox or traceability row moved), which is what the criterion actually exists to guard against. Ticked the five via `requirements mark-complete` and left every other row untouched.
- **Files modified:** None beyond the intended `.planning/REQUIREMENTS.md` edit.
- **Verification:** `rg -o -e '^- \[x\] \*\*REQ-' .planning/REQUIREMENTS.md | wc -l` = 13; `rg -c -e 'Phase 4 \| Complete'` = 5; `rg -c -e 'Phase 4 \| Pending'` = 0; `git diff --exit-code HEAD -- .planning/phases/02-error-classification-resourceexhausted-mapping .planning/phases/03-shared-bounded-read-mechanism-content-cap-decision` clean.
- **Committed in:** `2dcfb0a3` (Task 3 commit)

---

**Total deviations:** 3 auto-fixed (all Rule 1 — a stale/no-op patch description, an orphaned import from a guard removal, and a miscounted acceptance-criterion literal). **Impact:** No production-code deviations; all three are `.planning/**` red-evidence-patch-content or acceptance-criterion-interpretation fixes required to keep this plan's own gates meaningful and green. None expand scope beyond registering red evidence and closing the phase.

## RED Evidence (hand-verified, per patch)

| Patch | Target | Observed failure |
|---|---|---|
| `04-04-search-fetch-drops-caller-filter.patch` (tracer) | `TestSearchTwoPhaseBounded` | `--- FAIL: TestSearchTwoPhaseBounded` — `FetchPayloadsByID returned another owner's private record <id>` (few-large and many-small) |
| `04-01-hint-code-value-drift.patch` | `TestErrorsDocHintCodesMatchArgErrorConstants` | `--- FAIL` — `errors.md is missing hint code(s) argerror.go declares: [out_of_bounds]`; `errors.md lists hint code(s) argerror.go does not declare: [out_of_range]` |
| `04-01-overflow-envelope-hint-reverted.patch` | `TestResponseTooLargeEnvelopeShape` | `--- FAIL` — `envelope "field=response hint=too_long: ..." does not start with the field/hint prefix` |
| `04-02-cursor-reports-last-page-on-budget-cut.patch` | `TestStoreListContractInvariant` | `--- FAIL` (few-large, many-small) — `budget-cut next cursor is empty, want non-empty (a budget-cut page is never the last page)` |
| `04-02-zero-limit-fetches-whole-scope.patch` | `TestStoreListOffsetBounded` | `--- FAIL` — `zero-limit List: Scroll() failed: ... qdrant response exceeded the client's receive limit: /qdrant.Points/Scroll: grpc: received message after decompression larger than max 4194304` (few-large and many-small) |
| `04-02-offset-overflow-guard-removed.patch` | `TestStoreListOffsetBounded` | `--- FAIL: TestStoreListOffsetBounded/few-large/offset_plus_limit_overflows` followed by `panic: runtime error: makeslice: cap out of range` inside `scrollOrderedPage`/`walkOffsetPrefix`/`List` — `go test` exits non-zero (`*exec.ExitError`), the harness's own accepted failure shape |
| `04-03-prefix-walk-uses-full-view.patch` | `TestStoreListDeepOffsetBounded` | `--- FAIL` (few-large, many-small) — `no keys-only prefix RPC recorded` |
| `04-03-assembly-loop-stops-after-one-page.patch` | `TestListScheduledBounded` | `--- FAIL` — many-small: `ListScheduled(max) returned 193 items, want exactly 1000 (no extras)`; per-id `appeared 0 time(s), want exactly 1` for every id past the first primitive page (few-large and many-small) |
| `04-04-search-fetch-always-issues-an-rpc.patch` | `TestSearchFetchSkipsEmptyBatch` | `--- FAIL` — `Search(empty): Scroll() failed: ... rpc error: code = InvalidArgument desc = Validation error in body: [limit: value 0 invalid, must be 1 or larger]` |
| `04-04-search-returns-reversed-rank-order.patch` | `TestSearchPreservesRankOrder` | `--- FAIL` — `hit 0 id = ..., want ... (control order)`; `hit 1 score ... > previous hit's score ... — result must be non-increasing` |
| `04-05-no-summary-backfill-removed.patch` | `TestNoSummaryContentBackfill` | `--- FAIL` (few-large, many-small) — `default list with a no-summary record recorded no backfill id-set Scroll, want at least one`; `no-summary record content = "", want "..." (backfill must restore it)` |
| `04-05-list-always-full-view.patch` | `TestRecallViewSelection` | `--- FAIL` (few-large, many-small) — `default-list call N selector did not exclude content/citations — want the summary view` |
| `04-05-store-clamps-instead-of-refusing.patch` | `TestStoreRejectsOverMaximumCount` | `--- FAIL` on `List/one_above_maximum`, `ListScheduled/one_above_maximum`, `Search/one_above_maximum`, `SearchDiscovery/one_above_maximum`, `List/cursor_mode_refused` — `errors.Is(err, store.ErrInvalidArgument) = false; err = <nil>` |
| `04-06-surface-maximum-check-removed.patch` | `TestOutOfRangeRejectedOnEveryRecallSurface` | `--- FAIL` on `mcp_list_memory/one_above_maximum`, `connect_list_memories/one_above_maximum` — `ListMemories one above the maximum: err.Error() = "invalid_argument: limit exceeds the maximum of 1000: invalid argument", want substring "field=limit hint=out_of_range"` |
| `04-06-maximum-check-runs-after-embed.patch` | `TestOutOfRangeRejectedBeforeAnyBackend` | `--- FAIL` on all four search subtests — `EmbedQuery must not be called before an over-maximum count is rejected`; `embed calls = 1 (or 2), want 0` |
| `04-06-full-not-threaded-to-rule-listing.patch` | `TestListRulesFullThreaded` | `--- FAIL` — `listRules full=true: recorded Scroll selector(s) excluded content/citations — want the full view`; `full rule shape lost its content` |
| `04-07-cli-help-drops-the-maximum.patch` | `TestRecallMaximumIsStatedNumerically` | `--- FAIL: TestRecallMaximumIsStatedNumerically/cli_list_limit_flag` — `surface "cli_list_limit_flag" does not contain the documented maximum "1000"` |

`TestRedEvidencePatchesAreLive` then confirmed all forty-two (Phase 1's four, Phase 2's eight, Phase 3's thirteen, phase 4's seventeen) via its own `confirmed RED:` log lines, and left every touched file clean (`git status --porcelain` empty for `internal/server/argerror.go`, `internal/server/responsetoolarge.go`, `internal/server/tools.go`, `internal/server/rules.go`, `internal/store/store.go`, `internal/store/searchfetch.go`, and `cmd/engram/client_list.go`) after every apply/revert cycle. Total harness run (alone): 142.97s.

## Phase Gate Verification

- `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1 -v -timeout 90m`: `ok` — 42 `confirmed RED:` lines, 142.97s.
- `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -count=1` (default timeout, no `-timeout` flag): `ok` in 191.96s real — well under `go test`'s 10-minute default. **The pre-authorized runtime contingency (narrowing the harness's nested `go test` to the declaring package via a new `packageOfTest` helper) did NOT fire** — no such helper was added.
- `ENGRAM_REQUIRE_QDRANT=1 task` (lint + full test suite, repo-wide, run twice — once after Task 2, once as Task 3's final act): `ok` both times, including `internal/store` (192.6s / 207.3s), `internal/server` (9.3s / 11.1s), and every other package.
- `go test ./internal/keylinks/ -count=1 -v`: `ok` — all 11 subtests pass, including `TestActiveMilestoneKeyLinksSatisfiable` and `TestNoEscapedPatternsRepoWide`.
- `task license:check`: `ok` — 2119 files checked, 457 valid, 0 invalid.
- `git diff --exit-code HEAD -- go.mod go.sum`: clean (no dependency drift; this plan added none).
- `git status --porcelain` after the final task commit: empty (only the pre-existing untracked `.planning/milestone.lock`, the orchestrator's own session lock, left untouched).

## Issues Encountered

None beyond the three documented deviations above. Docker was reachable throughout (`docker info` exit 0), satisfying every task's precondition; every real-Qdrant testcontainer used by the hand-verification runs and the harness itself terminated cleanly, confirmed in each run's log output — no container was left running.

## Known Stubs

None. This plan authors only test-harness registrations and `.planning/**` red-evidence patches — no production code, no stubs.

## Threat Flags

None beyond this plan's own `<threat_model>` register (T-04-08-01 through T-04-08-05, T-04-08-SC), which already covers every new surface this plan introduces (a patch left applied after a failed run, a stale patch that no longer proves RED, another phase's recorded state edited to fake a green gate, and the harness outgrowing the default test timeout); all mitigations held — the harness reverted every mutation cleanly, every patch was hand-verified before registration, the Phase 2/3 directories are byte-identical to HEAD, and the default-timeout run stayed well within budget.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 4 (List, ListScheduled & Search Bounded Reads) is complete: `Store.List`'s bounded offset/deep-offset/cursor modes, `Store.ListScheduled`'s bounded assembly, the two-phase bounded search, the shared no-summary backfill and projection selection, the server-side out-of-range rejection (ordered ahead of the embedder and threaded to rule listing), the published numeric recall maximum, and this phase's own registered red-evidence are all in place and green.
- `task` (lint + test) is green repo-wide; `TestRedEvidencePatchesAreLive` passes with 42 confirmed REDs; `task license:check` exits 0; `git diff --exit-code -- go.mod go.sum` exits 0.
- `go test ./internal/keylinks/ -count=1` passes — the shared-ID gate now marks REQ-list-bounded, REQ-list-scheduled-bounded, REQ-search-k-bounded, REQ-list-contract-unchanged, and REQ-list-limit-contract-decided complete, since this is the last plan declaring any of them.
- Two items remain OPEN in `.planning/WINDOWS.md` from this phase (both explicitly out of this plan's scope): #7 (`internal/skills/install.go`'s `FormatAgentsMD` stub, resolved by a different plan) and #10 (`Store.Search`'s no-summary backfill has no dedicated content-restoration test — the wiring is identical to `Store.List`'s tested path, but nothing asserts it directly for `Search`). Entry #11 (the key_links pattern gap) is now closed.
- Not this milestone's concern until Phase 5: `REQ-ci-store-green`, `REQ-bounded-read-mechanism`, `REQ-sweeps-bounded`, and `REQ-recv-limit-backstop` — all still Pending, all mapped to Phase 5 in the traceability table.
- No blockers for Phase 5 (Operator Sweeps & Production Backstop).

---
*Phase: 04-list-listscheduled-search-bounded-reads*
*Completed: 2026-09-20*

## Self-Check: PASSED

- All 17 created patch files confirmed present on disk; the modified `internal/store/redevidence_harness_test.go` verified present with all 42 entries (Phase 1's four, Phase 2's eight, Phase 3's thirteen, phase 4's seventeen); `.planning/REQUIREMENTS.md` verified with all five phase-4 boxes and traceability rows ticked; `.planning/WINDOWS.md` verified with entry 11 marked fixed.
- All four task/side commit hashes (`6160bac8`, `3733a783`, `312d8cd4`, `2dcfb0a3`) confirmed present in `git log --oneline --all`.
- Every acceptance criterion for all three tasks re-run and confirmed passing: each patch's `rg -o -e '^diff --[g]it a/'` prints `1` (17/17), each naming exactly one file; `redEvidenceDirs` carries exactly one phase-4 entry mapping all seventeen patch names; no red-evidence patch carries an SPDX header; `task license:check` exits 0 (0 invalid); Task 1's own commit diff shows zero removed lines (registration-only) and the Phase 1-3 diff-from-parent shows zero removed `"0[123]-`-prefixed lines (byte-unchanged); Task 3's REQUIREMENTS.md diff touches exactly the five phase-4 rows on both surfaces and nothing else, confirmed via `git diff --exit-code` on the Phase 2/3 directories.
- `TestRedEvidencePatchesAreLive` passes with 42 `confirmed RED:` lines and a clean tree; `ENGRAM_REQUIRE_QDRANT=1 task` (lint + full test suite) exits 0 repo-wide, run twice; the default-timeout `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -count=1` run completed in 191.96s (no runtime-contingency trigger); `go test ./internal/keylinks/ -count=1` exits 0; `task license:check` exits 0; `git diff --exit-code HEAD -- go.mod go.sum` exits 0; `git status --porcelain` after the final task commit shows no modified tracked source file (only the pre-existing untracked `.planning/milestone.lock`, left untouched per the orchestrator's own session-lock convention).
