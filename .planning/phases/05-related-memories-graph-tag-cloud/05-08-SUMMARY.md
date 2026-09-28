---
phase: 05-related-memories-graph-tag-cloud
plan: 08
subsystem: ui
tags: [svelte5, related-memories, tag-filter, rail-tabs, keyboard, trail, localstorage]

requires:
  - phase: 05-related-memories-graph-tag-cloud
    provides: "plan 05-03's TagBars.svelte; plan 05-04's EdgeLane/SupersessionLane/EvidenceSection and the route's lanes+selection state; plan 05-05's RelatedGraph.svelte dimmedIds/summary/onleave props"
provides:
  - "The /related rail's Graph | Tags tabs (D-13), hosting the shared TagBars in mode=\"rail\" with the anchor's tags marked"
  - "D-14's in-view tag filter: a shared dimmedIds set applied identically to EdgeLane rows, SupersessionLane chain cards and RelatedGraph nodes, plus the #{tag} {N} of {M} carry it chip"
  - "D-15 rarity (ln(n/df), read from the candidates' own tag-edge evidence) shown apart from ListTags' popularity count"
  - "D-11's graph live-summary filter clause"
  - "D-01: the g key toggling and localStorage-remembering the rail tab"
  - "D-04: the trail crumbs row, back/[ walking the trail or exiting to origin, and the route-level Escape tier"
  - "RelatedGraph's onleave wired to focus the lanes column"
affects: []

actuals:
  tokens: 10106
  tasks: 2
  commits: 3

commits: 3
plan_head_before: a957af89361544c7b045a94e801b361129bb540f
plan_head_after: 53c3fa3c7bfb397a6e8f7450ba08ec457ddfd5c4

tech-stack:
  added: []
  patterns:
    - "One shared dimmedIds: ReadonlySet<string> prop threads through EdgeLane, SupersessionLane and RelatedGraph -- D-14's filter never computes membership differently per surface, it only dims the same drawn set the existing shared-selection state already lights"
    - "AA-safe dim rule (from the conventions skill's D-17 precedent) applied to .fdim exactly as to the existing past-state dim: opacity 0.3 on non-text marks (category dot, evidence pills, ×N badge), var(--muted-foreground) on the summary text"
    - "Rail tab persistence mirrors display.svelte.ts's localStorage convention: read at $state initializer time, written back in a bare $effect, both wrapped in try/catch (never blocks on missing/denied storage)"
    - "Route-level Escape/g/[ keys via svelte:window, guarded by the same isTypingTarget + Meta/Ctrl/Alt pattern ResultsList.svelte already uses for its own row-action keys -- RelatedGraph's own Escape tiers stopPropagation() so the window listener only ever sees an Escape pressed outside the graph"

key-files:
  created: []
  modified:
    - ui/src/routes/related/[id]/+page.svelte
    - ui/src/lib/components/EdgeLane.svelte
    - ui/src/lib/components/SupersessionLane.svelte
    - ui/src/routes/related/related.browser.test.ts

key-decisions:
  - "filterCounts/filterDimmed are computed over model.candidates (the full server-returned set), never membership.nodes (the drawn/visible subset) -- so the chip's \"N of M\" and the dimmed set stay correct even if a future candidate is hidden by a lane toggle or the vector collapse, matching D-14's 'never changes membership' contract by construction rather than by coincidence"
  - "tagRarity is read directly from each candidate's own tag-edge WeightedTag.weight, never from a second lookup against ListTags -- a tag with no tag-edge evidence in this response correctly has no rarity entry, so TagBars' tooltip falls back to 'count N' alone for it"
  - "The tag-filter chip renders above the lanes (not inside the rail), so it stays visible and clearable regardless of which rail tab is currently showing -- filtering and viewing the graph/tags are independent, matching the sketch's own placement"
  - "goBack()/exitToOrigin() are defined outside the {#if model} block (route-level) so the back button and the [/Escape keys work identically whether the rail is on Graph or Tags, and exitToOrigin's from-path branch does not require model to be loaded (only its anchor-id fallback does)"

patterns-established:
  - "A route hosting two mutually-exclusive rail panels (Graph | Tags) keeps its filter/selection state at the route level, orthogonal to which panel is showing -- selecting a candidate switches the panel to Graph (D-05 continuity) but never the reverse"

requirements-completed: [TAGS-01, GRAPH-02]

coverage:
  - id: D1
    description: "The rail's Graph | Tags tabs (D-13) host the shared TagBars in mode=\"rail\" with all-readable-scope counts and the anchor's tags marked; clicking a tag filters lane rows, chain cards and graph nodes in place (D-14), never changing membership, with a clearable #{tag} {N} of {M} carry it chip; rarity is shown apart from popularity (D-15) and the graph's live summary carries the same filter clause (D-11)"
    requirement: "TAGS-01"
    verification:
      - kind: automated_ui
        ref: "ui/src/routes/related/related.browser.test.ts, describe '/related/[id] — rail Tags tab and the in-view tag filter (Task 1, D-13/D-14/D-15/D-11)' (9 cases)"
        status: pass
    human_judgment: false
  - id: D2
    description: "g toggles and localStorage-remembers the rail tab (ignored while typing or with a modifier); the anchor bar's trail crumbs navigate with the trail cut at the clicked entry; back/[ walk the trail via history.back() or exit to the validated `from` path (or /search?sel={anchor id}); a route-level Escape clears the selection then exits to the origin, and the graph's onleave moves focus to the lanes region"
    requirement: "GRAPH-02"
    verification:
      - kind: automated_ui
        ref: "ui/src/routes/related/related.browser.test.ts, describe '/related/[id] — g, remembered tab, trail crumbs, back and Escape tiers (Task 2, D-01/D-04)' (11 cases)"
        status: pass
    human_judgment: false

duration: 50min
completed: 2026-09-28
status: complete
---

# Phase 5 Plan 8: Related View Rail Navigation and Tag Filter Summary

**The `/related` rail gains Graph | Tags tabs with a shared, in-place tag filter (TAGS-01), and the view's navigation is complete: the `g` key, a remembered tab, trail crumbs, back, and tiered Escape (GRAPH-02).**

## Performance

- **Duration:** ~50 min
- **Started:** 2026-09-28T00:27:00Z (approx.)
- **Completed:** 2026-09-28T00:48:57Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- D-13: the rail's `Graph | Tags` tabs (shadcn-svelte `Tabs`), the Tags tab hosting the shared `TagBars` in `mode="rail" scope=""` with the anchor's tags marked `●`
- D-14: an in-view tag filter — clicking a tag dims every lane row, chain card and graph node lacking it (AA-safe: opacity on non-text marks, muted-foreground on text), via one shared `dimmedIds` prop threaded through `EdgeLane`, `SupersessionLane` and `RelatedGraph`; a `#{tag} {N} of {M} carry it ×` chip above the lanes clears the filter
- D-15: rarity (`ln(n/df)`, read from the candidates' own tag-edge evidence) shown as a separate tooltip line, never merged with `ListTags`' popularity count
- D-11: the graph's polite live summary appends the same filter clause when a tag filter is active
- D-05 continuity: selecting a lane row or chain card while the Tags tab is showing switches the rail back to Graph so the evidence section is visible
- D-01: the `g` key toggles the rail tab (ignored while typing or with a modifier), remembered per viewer in `localStorage` (safe to lose, try/catch-wrapped)
- D-04: the anchor bar's trail crumbs (one per `?trail=` entry, each navigating with the trail cut at that point), `← back [` / the `[` key walking `history.back()` when the trail is non-empty or exiting to the validated `from` path (else `/search?sel={anchor id}`) otherwise, and a route-level Escape that clears the selection first, then exits to the origin on a second press
- `RelatedGraph`'s `onleave` now moves focus to the lanes column, completing the graph's own second Escape tier

## Task Commits

Each task was committed atomically (Task 1 is `type="tracer"`, a single production-quality commit; Task 2 is TDD `tdd="true"`, RED → GREEN, with no REFACTOR commit needed):

1. **Task 1: Rail Tags tab and the in-view tag filter (tracer)** - `d3f37596` (feat)
2. **Task 2 RED: failing tests for g/tab persistence, trail crumbs, back and Escape tiers** - `ba8eeaa6` (test)
2. **Task 2 GREEN: g, remembered tab, trail crumbs, back and Escape tiers** - `53c3fa3c` (feat)

**Plan metadata:** committed separately after this SUMMARY.

## Files Created/Modified

- `ui/src/routes/related/[id]/+page.svelte` - Rail tabs, tag-filter state/chip, tag rarity, graph live-summary suffix, D-05 continuity, tab persistence, crumbs/back/Escape route keys, `lanesEl`/`onleave` wiring
- `ui/src/lib/components/EdgeLane.svelte` - `dimmedIds` prop, `.fdim` AA-safe dim rule
- `ui/src/lib/components/SupersessionLane.svelte` - `dimmedIds` prop, `.fdim` AA-safe dim rule on chain cards
- `ui/src/routes/related/related.browser.test.ts` - 20 new browser tests across the two tasks (fixtures: a 5-candidate tag-filter scenario with a chain card, and the existing multi-type fixture reused for crumbs/back/Escape)

## Decisions Made

See `key-decisions` in frontmatter above.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Hardened two Task 1 tests against cross-test localStorage bleed**
- **Found during:** Task 2 GREEN verification (the pre-existing "the tag filter never changes membership" test started failing once Task 2 wired real `localStorage` persistence for the rail tab)
- **Issue:** `localStorage` persists across tests sharing the same browser page in this test harness; once Task 2's tests began writing `engram.console.relatedRailTab`, a later-running Task 1 test that measured the graph's `role=option` count immediately after render (assuming the default Graph tab) could instead see the Tags panel if a prior test had left `'tags'` stored, making its "before" measurement `0` instead of the real count
- **Fix:** The affected test now explicitly clicks the Graph tab before measuring; the whole Task 1 `describe` block's `beforeEach` now also clears `RAIL_TAB_KEY`, so every test in it starts from the true default regardless of run order
- **Files modified:** ui/src/routes/related/related.browser.test.ts
- **Verification:** both test files (`related.browser.test.ts`, `RelatedGraph.browser.test.ts`) pass together (86/86), and the full `ui/` suite passes (986/986)
- **Committed in:** `53c3fa3c` (Task 2 GREEN commit)

---

**Total deviations:** 1 auto-fixed (a test-harness ordering fix, no production-code behavior change). **Impact:** none on shipped behavior — the fix only makes the test suite robust to execution order.

## Issues Encountered

None beyond the deviation documented above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

This is the last plan in phase 05's wave 3 (depends on 05-03, 05-04, 05-05, all merged). The `/related` route's rail, filter, trail and keyboard model are complete per `05-CONTEXT.md` D-01, D-04, D-05, D-11, D-13, D-14, D-15. No blockers. `pnpm --dir ui build` succeeds; the full `ui/` suite (68 files, 986 tests) passes with no regressions.

## Self-Check: PASSED

- All 4 modified files confirmed present and changed on disk (`[ -f ]` + `git diff --stat`): `ui/src/routes/related/[id]/+page.svelte`, `ui/src/lib/components/EdgeLane.svelte`, `ui/src/lib/components/SupersessionLane.svelte`, `ui/src/routes/related/related.browser.test.ts`.
- All 3 commits (`d3f37596`, `ba8eeaa6`, `53c3fa3c`) confirmed in `git log --oneline`.
- Both tasks' `<verify>` commands re-run and passing: `pnpm --dir ui vitest run --project browser src/routes/related/related.browser.test.ts` (40/40 after Task 1); `pnpm --dir ui vitest run --project browser src/routes/related/related.browser.test.ts src/lib/components/RelatedGraph.browser.test.ts` (86/86 after Task 2); `pnpm --dir ui build` (exit 0, both tasks).
- All acceptance-criteria greps for both tasks re-run and matching exactly: Task 1 (`mode="rail"` = 1, `carry it` ≥ 1 [= 3], `dimmedIds` ≥ 2 [= 6]); Task 2 (`engram.console.relatedRailTab` = 1, `history.back` = 1, `onleave=` = 1).
- Full `ui/` regression: `pnpm --dir ui vitest run` — 68 files, 986 tests, all passing.
- `task fmt:check` clean (gofmt, dprint).

## TDD Gate Compliance

Task 2 followed genuine RED → GREEN discipline: 11 new tests were written first, run, and 9/11 failed for the right reason (element not found / timed-out locator / wrong observed value — never a fixture crash, import error, or zero-test discovery) before any implementation code made them pass; the other 2 assert a negative that already held on the unmodified component (an unrecognized stored value still defaulting to Graph; typing "g" in a text field still doing nothing) and were expected to stay green through GREEN by construction.

`gsd_run check tdd-red-evidence` is not runnable against this stack, reproducing the exact documented gap from plans 04-04, 05-03, 05-04 and 05-05: vitest's TAP reporters never emit `node:test`'s own summary lines the checker's parser requires. RED was instead verified by direct inspection of the `vitest run` output.

| Task | RED commit | GREEN commit | Target test | Verdict |
|------|-----------|---------------|--------------|---------|
| 2 | `ba8eeaa6` | `53c3fa3c` | `the anchor bar shows "trail", two crumbs and the current short_id when ?trail carries two entries` | Manually verified RED (real "Matcher did not succeed in time" / element-not-found timeout); `check tdd-red-evidence` not applicable to vitest-browser output |

No REFACTOR commit was needed — the GREEN implementation was already minimal and correct on the first pass.

---

*Phase: 05-related-memories-graph-tag-cloud*
*Completed: 2026-09-28*
