---
phase: 05-related-memories-graph-tag-cloud
plan: 04
subsystem: ui
tags: [svelte5, related-memories, edge-lanes, evidence-section, supersession-chain, shared-selection]

requires:
  - phase: 05-related-memories-graph-tag-cloud
    provides: "05-01's /related route, graph.ts model and RelatedGraph.svelte; 05-02's related-params.ts codec"
provides:
  - "ui/src/lib/related/lanes.ts: pure lane, evidence and empty-state copy (evidenceLines, evidenceHeader, laneCaption, laneCountLabel, emptyLaneReason, supersessionColumns, noNeighboursLines, collapsedRowCopy, TRUNCATION_BANNER, LANE_GLYPH, formatCosine, formatWeight)"
  - "EdgeLane.svelte / SupersessionLane.svelte / EvidenceSection.svelte / GraphLegend.svelte: the lanes body, the D-05 evidence section under the graph, and the legend/lane hide-show switches"
  - "/related/[id] route wired with one shared selection driving lanes, chain cards, graph and evidence, re-centre via relatedPath, and the vector-lane/graph collapse mirror"
affects: [05-05, 05-08, 05-09]

actuals:
  tokens: 17335
  tasks: 3
  commits: 5

commits: 5
plan_head_before: 289c0bc2e90d22755de10aff1c57534891672a9c
plan_head_after: 2278b31950af56007a4144918cb23fa65b22b0c1

tech-stack:
  added: []
  patterns:
    - "lanes.ts mirrors graph.ts's split: a testable, DOM-free module (evidence/caption/empty-state copy plus the supersession-column projection) that EdgeLane/SupersessionLane/EvidenceSection/GraphLegend only render"
    - "One shared selection state ({ id, lane } | null) lifted to the route, read by every lane component, the graph and the evidence section -- selecting/clearing it is centralized in one selectFromLane function so the anchor-clears-selection rule lives in exactly one place"
    - "TDD RED evidence for TS/Svelte plans in this repo cannot run through `gsd_run check tdd-red-evidence` (vitest's tap/tap-flat reporters never emit node:test's own summary lines the checker's parser requires) -- RED verified by hand each time (real AssertionErrors / element-not-found timeouts against the correct target test), matching the documented gap from plans 04-04 and 05-03"

key-files:
  created:
    - ui/src/lib/related/lanes.ts
    - ui/src/lib/related/lanes.test.ts
    - ui/src/lib/components/EdgeLane.svelte
    - ui/src/lib/components/SupersessionLane.svelte
    - ui/src/lib/components/EvidenceSection.svelte
    - ui/src/lib/components/GraphLegend.svelte
  modified:
    - ui/src/routes/related/[id]/+page.svelte
    - ui/src/routes/related/related.browser.test.ts

key-decisions:
  - "hiddenTypes (SvelteSet<LaneType>) and vectorExpanded ($state) were introduced already in Task 1, not deferred to Task 3 as the plan's per-task action text literally sequences them -- EdgeLane's hide/show button needed a real state to toggle from the moment it was built, and the route's single visibleMembership(model, { hiddenTypes, vectorExpanded }) call site is edited in place across all three tasks (never duplicated), so the Task 3 acceptance grep for exactly one call site still holds"
  - "selectFromLane(candidateId, lane) is the one selection-setter every lane (EdgeLane and SupersessionLane) and the legend-adjacent hide/show toggles route through -- clicking the anchor's own supersession card clears the selection instead of opening evidence for it, mirroring the graph's existing anchor-node click behavior"
  - "EdgeLane's inline per-row evidence span was renamed from .ev to .row-ev after a real collision surfaced: EvidenceSection's own root class is also .ev, and querySelectorAll('.ev') in a test matched both, producing a false '3 elements' failure during Task 1's own verification -- caught and fixed before the RED->GREEN split even started (Task 1 is type=tracer, not TDD)"
  - "EvidenceSection's <section> dropped its explicit role=\"region\" (kept only aria-label) after Svelte's a11y linter flagged it as redundant -- a <section> with an accessible name already carries the implicit ARIA region role, so getByRole('region', { name }) in tests is unaffected and the compiler warning is gone"
  - "The no-neighbours card's 'Open {short_id} in search ↗' link reuses openHrefFor(model.anchor.id) (the same /search?sel={id} builder the evidence section's 'Open record ↗' uses) rather than a second ad hoc URL, per the plan's own flagged assumption that the way-forward link is /search?sel={id}, not /search?q="

patterns-established:
  - "Container-query drop-out on lane rows (@container lane (max-width: 520px) swapping --cols and hiding the evidence cell) follows the codebase's existing gotcha xx98my50ng convention from ResultRow.svelte"
  - "A lane's hide/show affordance is one boolean (hiddenTypes.has(type)) read by three independent renderers (the lane's own header/body, the graph's visibleMembership filter, and the legend checkbox) -- never three separate toggle states that could drift"

requirements-completed: [GRAPH-01, GRAPH-03]

coverage:
  - id: D1
    description: "Citation/tag/vector lanes render from membership.lanes with inline per-type evidence; one shared selection lights a candidate's row in every lane, its chain-card appearance, and its graph node; selecting a candidate opens the D-05 evidence section directly under the graph (never over it, never for the anchor); Re-centre walks the trail via relatedPath"
    requirement: "GRAPH-03"
    verification:
      - kind: automated_ui
        ref: "ui/src/routes/related/related.browser.test.ts, describe 'lanes, shared selection and evidence under the graph (Task 1)' (5 cases)"
        status: pass
    human_judgment: false
  - id: D2
    description: "The supersession lane renders as a horizontal −2/−1/anchor/+1 timeline (never ChainDialog's d2/d1/head·d0 vocabulary) with hidden-state dashing; the truncation banner appears when truncated=true; each lane is independently empty with an honest reason; a zero-candidate response renders the no-neighbours card with a way-forward link instead of lane cards"
    requirement: "GRAPH-03"
    verification:
      - kind: unit
        ref: "ui/src/lib/related/lanes.test.ts (19/19: evidenceLines, evidenceHeader, laneCaption, emptyLaneReason, supersessionColumns, TRUNCATION_BANNER, collapsedRowCopy, noNeighboursLines, formatCosine/formatWeight)"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/related/related.browser.test.ts, describe 'supersession timeline, truncation and empty states (Task 2)' (7 cases including two screenshots)"
        status: pass
    human_judgment: false
  - id: D3
    description: "The vector lane collapses to 8 rows behind a 'show all N' ghost row mirrored 1:1 in the graph's drawn node set, expanding needs no second RelatedMemories call, the ceiling-cut clause appears only when truncated, and the legend checkbox is the SAME switch as each lane's hide/show button in both directions; re-centring resets the vector lane to collapsed"
    requirement: "GRAPH-01"
    verification:
      - kind: automated_ui
        ref: "ui/src/routes/related/related.browser.test.ts, describe 'vector show all mirrored in the graph, and legend/lane switches (Task 3)' (6 cases); ui/src/lib/components/RelatedGraph.browser.test.ts (11/11 regression)"
        status: pass
    human_judgment: false
  - id: D4
    description: "The lane-row container-query drop-out at 520px (--cols swap, evidence cell hidden) and the supersession lane's horizontal scroll-area at real narrow widths render correctly"
    verification: []
    human_judgment: true
    rationale: "The CSS rules and their string/count content are asserted structurally (greps, both-theme screenshots at default width), but no test resizes a live viewport to below 520px -- a human/visual pass at a real narrow width is the stronger proof, matching 05-01-SUMMARY's identical D5 precedent."

duration: 62min
completed: 2026-09-28
status: complete
---

# Phase 5 Plan 4: Related-Memories Lanes, Shared Selection and Evidence Summary

**The lanes are now the body of `/related`: citation/tag/vector rows, a supersession timeline, an evidence section under the graph, and a vector-lane/graph collapse mirrored through one shared selection and one `hiddenTypes` toggle set.**

## Performance

- **Duration:** 62 min
- **Started:** 2026-09-27T23:52:00Z
- **Completed:** 2026-09-28T00:54:00Z
- **Tasks:** 3
- **Files modified:** 8 (6 created, 2 modified)

## Accomplishments

- `lanes.ts`: the pure evidence/caption/empty-state/supersession-column module every lane component renders, with `formatCosine`/`formatWeight`/`LANE_GLYPH` as the one shared formatting surface
- `EdgeLane.svelte`: citation, tag and vector one-line rows with inline per-type evidence (citation pills, tag pills + Σ, vector score + bar), the shared-selection highlight, a container-query drop-out, and (Task 3) the vector lane's collapsed "show all" ghost row
- `SupersessionLane.svelte`: the horizontal chain-card timeline with its own column vocabulary, distinct from Phase 4's `ChainDialog`
- `EvidenceSection.svelte`: the D-05 evidence section directly under the graph -- candidate header, per-type "why it is related" lines, Re-centre and Open record, the closing note
- `GraphLegend.svelte`: one row per edge type sharing the exact same hide/show switch as each lane's own button
- The route: one shared `selection` state and one `hiddenTypes` set drive every lane, the graph, the legend and the evidence section; `recenter()` now walks the trail via `relatedPath` instead of a bare id; the truncation banner and no-neighbours card complete the honest-feedback states

## Task Commits

Each task was committed atomically (Tasks 2 and 3 as TDD RED → GREEN pairs; Task 1 is `type="tracer"`, committed as a single production-quality commit per protocol):

1. **Task 1: Citation, tag and vector lanes; one shared selection; evidence under the graph; re-centre (tracer)** - `68143d27` (feat)
2. **Task 2 RED: failing tests for supersession columns, truncation banner and no-neighbours copy** - `5692e2a4` (test)
2. **Task 2 GREEN: supersession timeline lane and honest empty/truncated states** - `6e584fe8` (feat)
3. **Task 3 RED: failing tests for vector show-all mirrored in the graph and legend/lane switches** - `05debd35` (test)
3. **Task 3 GREEN: vector collapse mirrored in the graph and shared lane/legend switches** - `2278b319` (feat)

**Plan metadata:** committed separately after this SUMMARY.

## Files Created/Modified

- `ui/src/lib/related/lanes.ts` - Pure evidence/caption/empty-state/supersession-column module
- `ui/src/lib/related/lanes.test.ts` - 19 node-project tests covering every exported function
- `ui/src/lib/components/EdgeLane.svelte` - Citation/tag/vector lane rows, shared selection, collapsed-row row
- `ui/src/lib/components/SupersessionLane.svelte` - The supersession timeline lane
- `ui/src/lib/components/EvidenceSection.svelte` - The D-05 evidence section under the graph
- `ui/src/lib/components/GraphLegend.svelte` - Legend rows sharing the lane hide/show switch
- `ui/src/routes/related/[id]/+page.svelte` - Route wiring: shared selection, hiddenTypes, vectorExpanded, recenter via relatedPath, truncation banner, no-neighbours card
- `ui/src/routes/related/related.browser.test.ts` - 25 new browser tests across the three tasks (fixtures: multi-type candidates, a two-predecessor/one-successor chain, 20 vector-only hits)

## Decisions Made

See `key-decisions` in frontmatter above.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Renamed EdgeLane's inline evidence class from `.ev` to `.row-ev`**
- **Found during:** Task 1 (own test verification, before commit)
- **Issue:** `EvidenceSection.svelte`'s root class is also `.ev`; `querySelectorAll('.ev')` in the esc-clears-selection test matched both the evidence section and every populated lane row's inline evidence span, producing `3` instead of the expected `0`/`1`
- **Fix:** Renamed EdgeLane's row-level evidence span class to `.row-ev` (and its container-query drop-out rule) -- `.ev` now uniquely identifies `EvidenceSection`
- **Files modified:** ui/src/lib/components/EdgeLane.svelte
- **Verification:** related.browser.test.ts's esc/clear-selection test passes; `.ev` matches exactly one element when the evidence section is open
- **Committed in:** `68143d27` (Task 1 commit)

**2. [Rule 1 - Bug] Dropped EvidenceSection's redundant `role="region"`**
- **Found during:** Task 1 verification (Svelte compiler a11y warning during test run)
- **Issue:** `<section aria-label="...">` already carries the implicit ARIA `region` role; the explicit `role="region"` triggered Svelte's `a11y_no_redundant_roles` warning
- **Fix:** Removed the explicit `role="region"` attribute, keeping `aria-label` -- `getByRole('region', { name })` in tests is unaffected since the implicit role still applies
- **Files modified:** ui/src/lib/components/EvidenceSection.svelte
- **Verification:** `pnpm --dir ui vitest run --project browser src/routes/related/related.browser.test.ts` runs with zero Svelte compiler warnings; all region-role assertions still pass
- **Committed in:** `68143d27` (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (both Rule 1 — bugs found and fixed during the plan's own verification, before any task commit landed)
**Impact on plan:** Both fixes are internal correctness/cleanliness corrections with zero behavioral change to the shipped feature. No scope creep.

## Issues Encountered

**Task 3 acceptance criterion `rg -o -e 'visibleMembership' 'ui/src/routes/related/[id]/+page.svelte' | wc -l` prints `1` is not satisfiable with an idiomatic named import.** A named import (`import { visibleMembership } from '$lib/related/graph'`) contributes one occurrence of the literal string on its own, and the single real call site (`const membership = $derived(visibleMembership(model, { hiddenTypes, vectorExpanded }))`) contributes a second — the minimum achievable count is `2`, confirmed after removing every prose mention of the function name from comments. The only way to reach a literal count of `1` would be a namespace import (`import * as graph from '...'; graph.visibleMembership(...)`), which would be a one-off, unidiomatic deviation from every other file in this codebase (`graph.test.ts`, `RelatedGraph.svelte`, `ChainDialog.svelte`) that always uses named imports for these exact symbols — and would only move the same word to a different position in the file, not remove it. The plan's own prose states the real intent precisely: `visibleMembership(...)` must be "the ONE result the lanes and RelatedGraph both read" — i.e., exactly one *call site*, never a duplicated/diverging computation. That property is verified structurally: `rg -n 'visibleMembership'` shows exactly one line containing `visibleMembership(` (the `$derived` declaration), unchanged in place across Tasks 1, 2 and 3. Treated as a miscalibrated literal grep threshold rather than an unmet requirement; not fixed by code changes.

## Known Stubs

None — every lane, the evidence section, the supersession timeline, the truncation banner, the no-neighbours card and the legend render from real `RelatedMemories` response data with no hardcoded/placeholder values.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `lanes.ts`'s exported contracts (`EvidenceLine`, `LaneCard`, `ChainColumn`, every formatter/projection function) and the four new components (`EdgeLane`, `SupersessionLane`, `EvidenceSection`, `GraphLegend`) are stable for plan 05-05 (keyboard, pan/zoom, drag) to build against -- none of their public props changed shape across Tasks 1-3, only `EdgeLane`'s `collapsed` prop was added incrementally.
- The graph's own keyboard model (roving `aria-activedescendant`, Enter-to-recentre, arrow-key lane-order traversal) is still plan 05-05's scope, per 05-CONTEXT D-10 and 05-01-SUMMARY's own Known Stubs -- this plan only wired the click/dblclick path already shipped in `RelatedGraph.svelte`.
- No blockers. `pnpm --dir ui build` succeeds; the full `ui/` suite (67 files, 908 tests) passes with no regressions.

## Self-Check: PASSED

- All 6 created files confirmed present on disk (`[ -f ]`): `ui/src/lib/related/lanes.ts`, `ui/src/lib/related/lanes.test.ts`, `ui/src/lib/components/EdgeLane.svelte`, `ui/src/lib/components/SupersessionLane.svelte`, `ui/src/lib/components/EvidenceSection.svelte`, `ui/src/lib/components/GraphLegend.svelte`.
- All 5 commits (`68143d27`, `5692e2a4`, `6e584fe8`, `05debd35`, `2278b319`) confirmed in `git log --oneline`.
- All 3 tasks' `<verify>` commands re-run and passing: `pnpm --dir ui vitest run --project browser src/routes/related/related.browser.test.ts` (25/25), `pnpm --dir ui vitest run --project node src/lib/related/lanes.test.ts` (19/19), `pnpm --dir ui vitest run --project browser src/routes/related/related.browser.test.ts src/lib/components/RelatedGraph.browser.test.ts` (42/42), `pnpm --dir ui build` (exit 0).
- All acceptance-criteria greps for all 3 tasks re-run: Task 1 (4/4 matching required counts), Task 2 (4/4), Task 3 (2/3 exact, 1 documented as unsatisfiable-as-literally-written under Issues Encountered — the semantic property it names is independently verified).
- Full `ui/` regression: `pnpm --dir ui vitest run` — 67 files, 908 tests, all passing.
- `task fmt:check` clean (gofmt, dprint).

## TDD Gate Compliance

Both TDD tasks (2 and 3) followed genuine RED → GREEN discipline: tests were written first, run, and confirmed to fail on the correct target assertion (a real `AssertionError` for Task 2's node-project tests, real element-not-found `TimeoutError`s for Task 3's browser tests) — never a fixture crash, import error, or zero-test discovery — before any implementation code made them pass.

`gsd_run check tdd-red-evidence` is not runnable against this stack for either task, reproducing the exact gap already disclosed in plans 04-04 and 05-03: the checker's TAP parser requires `node --test`'s own `# tests N`/`# pass N`/`# fail N` summary lines, which neither of vitest's TAP reporters emit (confirmed live for Task 2 by running `vitest run --reporter=tap` and feeding the output to the checker — verdict `INVALID_RED`/`zero_tests_discovered`, even though the checker's own `failing_tests` array correctly named every real failure). Task 3's tests run through `vitest-browser-svelte`, which has no TAP-adjacent output at all. Both RED phases were instead verified by direct inspection of the `vitest run` output, documented in each `test(05-04): ...` commit message with the specific target test and its real failure mode.

| Task | RED commit | GREEN commit | Target test | Verdict |
|------|-----------|---------------|--------------|---------|
| 2 | `5692e2a4` | `6e584fe8` | `supersessionColumns > orders columns by signed depth ascending, anchor at 0, labels '−2', '−1', 'anchor', '+1' (U+2212 minus)` | Manually verified RED (real AssertionError, 7/19 cases failing); `check tdd-red-evidence` returns `INVALID_RED`/`zero_tests_discovered` (documented gap, not an invalid RED) |
| 3 | `05debd35` | `2278b319` | `collapses the vector lane to 8 rows with a "show all" row, and the graph draws only those 8 vector nodes` | Manually verified RED (real element-not-found timeout, all 6/6 new cases failing); checker not applicable to vitest-browser output |

No REFACTOR commit was needed for either task — both GREEN implementations were already minimal and correct on the first pass.

---

*Phase: 05-related-memories-graph-tag-cloud*
*Completed: 2026-09-28*
