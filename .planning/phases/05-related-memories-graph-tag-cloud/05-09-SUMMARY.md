---
phase: 05-related-memories-graph-tag-cloud
plan: 09
subsystem: ui
tags: [chromedp, e2e, wcag, axe-core, svelte5, related-memories, tags, bits-ui]

requires:
  - phase: 05-related-memories-graph-tag-cloud
    provides: "plans 05-01..05-08's related view route, graph, lanes, evidence section, tag bars, tag combobox and rail navigation"
provides:
  - "TestConsoleRelatedView: a live chromedp round trip proving GRAPH-01/D-03 end to end -- DetailPane's Related button navigates to /ui/related/<id>, the rail graph renders the anchor plus its vector neighbour, and a direct deep link to /ui/related/<neighbour short_id> renders that record's own view through the Go static handler's SPA fallback and short_id resolution"
  - "WCAG 2.2 AA audits (zero violations, both themes) for every new Phase 5 surface: /related populated+selected+evidence, the arrow-key focus card, the Tags tab with an active filter, the not-found state, the /search docked Tags panel with an active tag, the TagCombobox picker with matches and an unknown-tag row, and the header search's Tags autocomplete group"
  - "Two genuine WCAG 2.2 AA fixes found by the audit: GraphLegend's edge-type toggle checkboxes floored to a 24px hit target (SC 2.5.8), and an explicit Command.List id/Command.Input aria-controls pair closing an aria-required-attr gap (SC 4.1.2) shared by TagCombobox and HeaderSearch's bits-ui Command usage"
  - "engram-console-conventions and engram-connect-client skills updated with the shipped Phase 5 keyboard model, code locations, URL codec and query keys"
  - "The vendored console SPA (internal/webauth/static) rebuilt and matching a fresh build"
affects: []

actuals:
  tokens: 9300
  tasks: 3
  commits: 4

commits: 4
plan_head_before: 1ed8570dd23788f776b8b934b775d7202c918e91
plan_head_after: 3027e7b6962de0a0d8c53a35571140312103bb44

tech-stack:
  added: []
  patterns:
    - "GraphLegend's small (14px) checkbox toggles floored to a 24px WCAG 2.2 SC 2.5.8 hit target via min-width/min-height: max(calc(24 * var(--u)), 24px) on a scoped class, the same technique RowActions.svelte's .ra-btn already established -- CSS min-* always wins over a smaller explicit size, so the fix composes with bits-ui's own h-3.5/w-3.5 classes rather than fighting them"
    - "bits-ui's Command.Input only wires aria-controls from an (unrendered) Command.Viewport; passing an explicit id on Command.List plus a matching aria-controls on Command.Input closes the gap because svelte-toolbelt's mergeProps keeps a caller-supplied value whenever the library's own merged value is undefined (b !== undefined ? b : a)"
    - "The chromedp related-view e2e relies on the vector edge having no score floor (internal/store/relatedmemories.go) plus a fresh per-test Qdrant collection (storetest's testCollection(port)) -- two records seeded under one scope are always each other's sole vector neighbour, with no fixture tuning needed"

key-files:
  created:
    - .planning/phases/05-related-memories-graph-tag-cloud/deferred-items.md
  modified:
    - internal/e2e/console_browser_test.go
    - internal/webauth/static/** (rebuilt)
    - ui/src/lib/a11y/surfaces.browser.test.ts
    - ui/src/lib/components/GraphLegend.svelte
    - ui/src/lib/components/TagCombobox.svelte
    - ui/src/lib/components/HeaderSearch.svelte
    - .claude/skills/engram-console-conventions/SKILL.md
    - .claude/skills/engram-connect-client/SKILL.md

key-decisions:
  - "HeaderSearch.svelte was fixed alongside TagCombobox.svelte even though it is not in the plan's files_modified list -- it shares the identical bits-ui Command.Input aria-controls gap, and the plan's own behavior list requires the header Tags group surface to pass the AA audit with zero violations (Rule 2: missing critical accessibility functionality)"
  - "ScopeCombobox.svelte, which shares the same Command.Input pattern, was left untouched -- it carries no new-in-phase surface this plan's must-haves require auditing, and fixing it would be scope creep beyond this task's own deliverable"
  - "requirements mark-complete could not flip GRAPH-01/02/03 or TAGS-01/02's checkbox/traceability surfaces -- a pre-existing, project-wide REQUIREMENTS.md convention (traceability Status = 'Mapped', present since the original GSD bootstrap commit) does not match gsd-tools' hardcoded Pending/Gaps-Found acceptance vocabulary. Logged to deferred-items.md rather than hand-edited; REQUIREMENTS.md itself is unchanged"

requirements-completed: [GRAPH-01, GRAPH-02, GRAPH-03, TAGS-01, TAGS-02]

coverage:
  - id: D1
    description: "Live browser round trip: DetailPane Related button -> /ui/related/<id> rail graph render -> direct short_id deep link, against a real engram binary and Qdrant (GRAPH-01, D-03)"
    requirement: "GRAPH-01"
    verification:
      - kind: e2e
        ref: "internal/e2e/console_browser_test.go#TestConsoleRelatedView"
        status: pass
    human_judgment: false
  - id: D2
    description: "WCAG 2.2 AA audits (axe-core, both themes, zero violations) for /related (populated+selected+evidence, focus card, Tags tab filter, not-found), the /search Tags panel, the TagCombobox picker, and the header Tags group"
    requirement: "GRAPH-03"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/a11y/surfaces.browser.test.ts (25 tests, describes: '/related — AA audit', '/search Tags panel — AA audit', 'tag picker — AA audit', 'header Tags group — AA audit')"
        status: pass
    human_judgment: false
  - id: D3
    description: "engram-console-conventions and engram-connect-client skills record the shipped Phase 5 keyboard model, code locations, URL codec and query keys"
    verification:
      - kind: other
        ref: "rg acceptance-criteria checks: engram.console.relatedRailTab, '__all_readable__', related-params.ts all present"
        status: pass
    human_judgment: false
  - id: D4
    description: "Vendored console SPA matches a fresh build (ui-drift gate) and every phase gate (pnpm --dir ui test, go test ./... with Qdrant, chromedp e2e with browser+Qdrant, lint/fmt:check/license:check/proto:lint) is green in one session"
    verification:
      - kind: other
        ref: "task ui:build && git status --porcelain -- internal/webauth/static (clean); pnpm --dir ui test (993 passed); ENGRAM_REQUIRE_QDRANT=1 go test ./... (28 packages ok); ENGRAM_REQUIRE_BROWSER=1 ENGRAM_REQUIRE_QDRANT=1 go test ./internal/e2e/ -run '^TestConsole' (5 PASS); task lint/fmt:check/license:check/proto:lint"
        status: pass
    human_judgment: false

duration: 50min
completed: 2026-09-28
status: complete
---

# Phase 5 Plan 9: Live Related-View Proof, AA Audits and Phase Gate Summary

**A live chromedp round trip through the vendored console proves `/related`'s pane→graph entry and short_id deep link, WCAG 2.2 AA audits close two genuine violations (checkbox target-size, missing `aria-controls`), and every phase gate is green.**

## Performance

- **Duration:** ~50 min
- **Started:** 2026-09-28T04:50:00Z (approx.)
- **Completed:** 2026-09-28T05:26:00Z
- **Tasks:** 3
- **Files modified:** 8 (plus the vendored `internal/webauth/static` tree, rebuilt twice)

## Accomplishments

- `TestConsoleRelatedView` drives a real headless Chrome against the real `engram` binary and a
  fresh per-test Qdrant collection: seeds an anchor and its vector neighbour, opens the anchor's
  detail pane via `/ui/search?sel=<id>`, clicks the pane's **Related** button, confirms the
  client-side navigation lands on `/ui/related/`, and proves the rail graph renders both the
  anchor and neighbour (`svg[role="listbox"] [role="option"]` count ≥ 2) with the anchor's
  short_id in the mono call line and the neighbour's short_id in the vector lane. A second phase
  navigates directly to `/ui/related/<neighbour short_id>` and re-proves the same graph render and
  call line — the Go static handler's SPA fallback plus server-side short_id resolution
  (`ResolvePointID`), with no prior SPA navigation.
- WCAG 2.2 AA audits (axe-core, light + dark, zero violations, one screenshot per theme) now cover
  every new Phase 5 surface: the `/related` route in four states (populated with a selected node
  and its evidence, the floating focus card shown after an arrow key, the Tags tab with an active
  `#qdrant` filter, and the not-found state), the `/search` docked Tags panel with an active tag,
  `TagCombobox` open with ranked matches and the honest unknown-tag row, and the header search's
  Tags autocomplete group (`#qd` typed).
- The audit found two genuine AA violations and fixed both in-phase, per the plan's policy: (1)
  `GraphLegend`'s four edge-type toggle checkboxes measured 13.1×13.1px (WCAG SC 2.5.8's 24px
  minimum), floored via `min-width`/`min-height: max(calc(24 * var(--u)), 24px)` — the same
  technique `RowActions.svelte`'s `.ra-btn` already uses; (2) `TagCombobox` and `HeaderSearch`'s
  shared bits-ui `Command.Input` carried `role="combobox" aria-expanded="true"` with no
  `aria-controls` (WCAG SC 4.1.2), because bits-ui only wires `aria-controls` from an unused
  `Command.Viewport` — closed by giving `Command.List` an explicit `id` and passing a matching
  `aria-controls` on `Command.Input`, which `mergeProps` preserves since bits-ui's own value is
  `undefined` without a `Viewport`.
- Both skills brought current: `engram-console-conventions` gained the `r` row-action key, a full
  `/related` keyboard sub-table (`g`, `[`, the graph's single Tab stop, lane-order arrows,
  Home/End, Space, Enter, the tiered Escape, the wheel gate/hint, corner controls, refit/zoom
  rules, the focus card, the D-11 live summary, and the D-14 tag-filter dim rule), 15 new "Where
  the code lives" rows, and a Tokens note on the undefined `--text-2xs`/`--radius-sm`.
  `engram-connect-client` gained `related-params.ts`'s codec (`from`/`trail`, canonical order) and
  recorded `/related`'s `['relatedMemories', id, 64, false]` query key (plus `ChainDialog`'s `k=1`
  peek key) and `['listTags', scope || '__all_readable__', 1000]` with `staleTime: Infinity`,
  shared by `TagBars`, `TagCombobox` and the header Tags group.
- The vendored console SPA was rebuilt twice (once after Task 1's e2e addition, once after Task
  2's component fixes) and both times matched a fresh `task ui:build` byte-for-byte before commit.
- Every phase gate ran green in one session: `pnpm --dir ui test` (993 tests), `ENGRAM_REQUIRE_QDRANT=1
  go test ./...` (28 packages, all `ok`), `ENGRAM_REQUIRE_BROWSER=1 ENGRAM_REQUIRE_QDRANT=1 go test
  ./internal/e2e/ -run '^TestConsole'` (5 `--- PASS` lines: bundle render, archive undo, supersede,
  entry-point resolution, related view), and `task lint`/`fmt:check`/`license:check`/`proto:lint`.
  A `read-only-ok` grep confirms no `engramWrite` reference exists in any of the nine new/touched
  related-view or tag-surface files.

## Task Commits

Each task was committed atomically:

1. **Task 1: Vendor the SPA and prove pane → Related → graph, plus a deep link, against a live server** - `db8ffd25` (test)
2. **Task 2: WCAG 2.2 AA audits of every new surface in both themes; fix AA failures in-phase, file the rest** - `cf1e41e1` (test, includes the GraphLegend/TagCombobox/HeaderSearch fixes and a Go lint fix in Task 1's file)
3. **Task 3a: Re-vendor the SPA after Task 2's component changes** - `5cdcedcf` (build)
3. **Task 3b: Bring the two skills current** - `3027e7b6` (docs)

_Note: Task 3's action text splits into a re-vendor commit and a docs commit, per its own instruction ("a separate build(ui): ... commit")._

## Files Created/Modified

- `internal/e2e/console_browser_test.go` — `TestConsoleRelatedView`, `relatedScope`,
  `locationPathPrefixPollExpr`, `graphOptionCountPollExpr`
- `internal/webauth/static/**` — rebuilt console SPA (two rebuilds, both clean against source)
- `ui/src/lib/a11y/surfaces.browser.test.ts` — `listTags` mock wiring, `renderRelated` helper, four
  new AA-audit describes (25 tests total in the file)
- `ui/src/lib/components/GraphLegend.svelte` — `.legend-cb` 24px target-size floor
- `ui/src/lib/components/TagCombobox.svelte` — explicit `Command.List` id + `Command.Input
  aria-controls`
- `ui/src/lib/components/HeaderSearch.svelte` — same `aria-controls` fix for the header search
  combobox
- `.claude/skills/engram-console-conventions/SKILL.md` — `/related` keyboard model, code
  locations, tokens note
- `.claude/skills/engram-connect-client/SKILL.md` — `related-params.ts` codec, `RelatedMemories`/
  `ListTags` query keys
- `.planning/phases/05-related-memories-graph-tag-cloud/deferred-items.md` — the
  `requirements mark-complete` finding (see Deviations)

## Decisions Made

- `HeaderSearch.svelte` was fixed even though it is outside the plan's declared `files_modified`
  list, because it shares the exact `Command.Input`/`aria-controls` gap `TagCombobox.svelte` has,
  and the plan's own behavior list requires the header Tags group surface to audit clean (Rule 2).
- `ScopeCombobox.svelte`, which shares the same pattern, was deliberately left untouched — no
  must-have in this plan requires auditing it, and touching it would be scope creep.
- The chromedp test's anchor/neighbour records share a scope but rely on the vector edge's
  documented no-score-floor behavior (not the shared scope) to guarantee mutual neighbour status,
  matching the plan's own flagged assumption.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] GraphLegend checkbox target-size (WCAG SC 2.5.8)**
- **Found during:** Task 2, running the `/related — AA audit` populated-view describe
- **Issue:** The four edge-type toggle `Checkbox`es measured 13.1×13.1px with no adjacent text
  label to widen the click target — axe-core reported `target-size (serious)` on all four
- **Fix:** Added a scoped `.legend-cb` class with `min-width`/`min-height:
  max(calc(24 * var(--u)), 24px)`, matching `RowActions.svelte`'s existing `.ra-btn` precedent
- **Files modified:** `ui/src/lib/components/GraphLegend.svelte`
- **Verification:** `auditAA` reports zero violations for the populated-view describe in both themes
- **Committed in:** `cf1e41e1`

**2. [Rule 2 - Missing Critical] TagCombobox/HeaderSearch missing `aria-controls` (WCAG SC 4.1.2)**
- **Found during:** Task 2, running the `tag picker` and `header Tags group` AA-audit describes
- **Issue:** bits-ui's `Command.Input` sets `role="combobox" aria-expanded="true"` but only
  populates `aria-controls` from an (unrendered) `Command.Viewport`, so both comboboxes carried a
  required-but-absent ARIA attribute — `aria-required-attr (critical)`
- **Fix:** Gave `Command.List` an explicit `id` (`` `${$props.id()}-list` ``) and passed a matching
  `aria-controls` on `Command.Input`; `svelte-toolbelt`'s `mergeProps` keeps the caller's value
  since bits-ui's own is `undefined` without a `Viewport`
- **Files modified:** `ui/src/lib/components/TagCombobox.svelte`,
  `ui/src/lib/components/HeaderSearch.svelte` (not in the plan's `files_modified` list — added
  because it shares the identical gap and the header Tags group surface must audit clean)
- **Verification:** `auditAA` reports zero violations for both describes in both themes
- **Committed in:** `cf1e41e1`

**3. [Rule 3 - Blocking] golangci-lint `redefines-builtin-id` in Task 1's `graphOptionCountPollExpr`**
- **Found during:** Task 2, running `task lint` ahead of the Task 2 commit
- **Issue:** The helper's parameter was named `min`, shadowing Go's builtin `min` function
- **Fix:** Renamed the parameter to `minCount`
- **Files modified:** `internal/e2e/console_browser_test.go`
- **Verification:** `task lint:go` reports 0 issues
- **Committed in:** `cf1e41e1` (folded into Task 2's commit since Task 1 was already committed)

---

**Total deviations:** 3 auto-fixed (2 missing-critical accessibility, 1 blocking lint fix)
**Impact on plan:** All three were necessary for correctness (WCAG conformance) or to keep the
gate green. No scope creep beyond the two extra accessibility-critical files.

## Known Stubs

None.

## Issues Encountered

`requirements mark-complete GRAPH-01 GRAPH-02 GRAPH-03 TAGS-01 TAGS-02` returned all five IDs as
`not_found` despite `requirements.ready-ids` confirming `5/5 requirement(s) ready to mark complete`
(the shared-ID gate cleared correctly — every sibling `05-0N-SUMMARY.md` exists). Root cause traced
into `gsd-tools`' `cmdRequirementsMarkComplete`: it only flips a traceability row whose Status reads
`Pending` or `Gaps Found`; this project's `.planning/REQUIREMENTS.md` traceability table has used
`Mapped` as its Status value since the original GSD bootstrap commit (`d2120f09`), so the row never
qualifies, and the tool's own checkbox-rollback safeguard then reverts the `- [ ]` checkbox flip
too. `rg -c '^\- \[x\]' .planning/REQUIREMENTS.md` finds zero completed checkboxes anywhere in the
file — this is a pre-existing, project-wide condition, not something this plan introduced.
REQUIREMENTS.md was left unmodified (no local invention of tool-owned structure); the finding is
recorded in `.planning/phases/05-related-memories-graph-tag-cloud/deferred-items.md` with a
recommendation to reconcile the vocabulary mismatch upstream or via a dedicated docs-only change.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Phase 5 (Related-Memories Graph & Tag Cloud) is functionally and gate-complete: GRAPH-01..03 and
TAGS-01..02 are proven live (chromedp), audited (WCAG 2.2 AA, zero violations), and documented
(both project skills current). The only open item is the `REQUIREMENTS.md` mark-complete gap
above, which is a documentation/tooling reconciliation, not a functional blocker — the milestone's
`/gsd-verify-work` or phase-close step should account for it when checking requirement completion
state, since the checkbox/traceability surfaces will read `Pending`/`Mapped` even though the
underlying capability is shipped and verified.

---
*Phase: 05-related-memories-graph-tag-cloud*
*Completed: 2026-09-28*
