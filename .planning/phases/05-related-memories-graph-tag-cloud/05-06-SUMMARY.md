---
phase: 05-related-memories-graph-tag-cloud
plan: 06
subsystem: ui
tags: [svelte, tanstack-query, bits-ui, tags, header-search]

# Dependency graph
requires:
  - phase: 05-related-memories-graph-tag-cloud
    provides: "05-03: listTagsQuery/rankTagMatches/matchFooter/scopeLabel/tagsLoadingLine/tagsErrorCopy (ui/src/lib/tags/{query,tags}.ts), TagMatchRow.svelte"
provides:
  - "ui/src/lib/components/HeaderSearch.svelte: the header search Tags group (TAGS-02, D-16..D-19), closing Phase 2's deferred 'Tags section of the header dropdown'"
affects: []

# Actuals (#2632) -- pairs with the plan's `estimate` to calibrate future estimates.
actuals:
  tokens: 4128
  tasks: 2
  commits: 3
  plan_head_before: 289c0bc2e90d22755de10aff1c57534891672a9c
  plan_head_after: 588d1cb3f49da458b602ea55da99342d8e848cb5

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "tagPrefixInProgress is derived directly from the raw debounced text's last whitespace-separated token, not from classify.ts's chip kind -- a single-char tag value like '#qd' already forms a complete { kind: 'tag' } chip, but the group must still show while the user keeps typing it"
    - "The Tags group markup is factored into a Svelte 5 {#snippet tagsGroup()} and rendered from two call sites gated by classified.kind, so chip-only input (kind='operators') gets it first in the dropdown while free-text-plus-tag input (kind='text') gets it after the Results/search-all row -- preserving the pre-existing 'Enter hits the top Search-all row' behavior for mixed input"

key-files:
  created: []
  modified:
    - ui/src/lib/components/HeaderSearch.svelte
    - ui/src/lib/components/HeaderSearch.browser.test.ts

key-decisions:
  - "completeToken's replace-vs-append condition widened from `pendingChip` alone to `pendingChip || tagPrefixInProgress !== null` -- a tag token with a non-empty value ('#qd') is never a classify.ts pending chip, so without this fix selecting a suggested tag would append a duplicate token ('#qd #qdrant ') instead of replacing the partial one, contradicting the plan's own must-have truth."
  - "The Tags group renders before the Results/search-all/list-all section only when classified.kind !== 'text' (chip-only input); for text-kind input (free text plus a trailing partial tag, e.g. 'gofmt #ci') it renders AFTER that section. Discovered as a regression: once the unknown-tag row gave the Tags group a real item, bits-ui Command's default keyboard highlight moved to it, breaking the pre-existing 'Enter on the top row hands the classified query to /search' test. None of this plan's own authored tests exercise mixed free-text-plus-tag input, so this ordering split is invisible to them and only fixes the pre-existing regression."

requirements-completed: [TAGS-02]

coverage:
  - id: D1
    description: "Typing '#' or 'tag:' as the last, unfinished token in the header search shows a ranked 'Tags · counts in {scope}' group (with counts, prefix-first-then-count ranking, an honest unknown-tag 'Add #{tag}' row, and a true match-count footer) sourced from the one cached ListTags query the tag picker and bars use; selecting a row replaces the partial token with '#{tag} '."
    requirement: TAGS-02
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/HeaderSearch.browser.test.ts (9 new cases across both tasks: ranked group + Enter-completes-chip, no-group-on-free-text, scope-following label/request, unknown-tag row at more=false/true, 12-match cap+footer, loading line, rejected envelope, cache reuse)"
        status: pass
    human_judgment: false

# Metrics
duration: 32min
completed: 2026-09-28
status: complete
---

# Phase 5 Plan 6: Header Search Tags Group Summary

**The header search's `#`/`tag:` autocomplete now lists ranked, counted tags from the same cached `ListTags` query the tag picker and bars share, honest about unknown tags and scope-following, closing Phase 2's deferred "Tags section of the header dropdown" (TAGS-02).**

## Performance

- **Duration:** 32 min
- **Started:** 2026-09-27T23:52:00Z
- **Completed:** 2026-09-28T00:03:31Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- `tagPrefixInProgress` derived state detects an unfinished `#`/`tag:` last token directly from the raw text (not `classify.ts`'s chip kind), so the group stays open while the user keeps typing a tag whose partial value already forms a complete chip.
- A `Tags · counts in {scope}` `Command.Group` (built as a reusable `{#snippet}`) renders the same `rankTagMatches`/`TagMatchRow`/`matchFooter`/`tagsLoadingLine`/`tagsErrorCopy` helpers the "+ tag" picker and tag bars use, covering populated, loading, error, unknown-tag, and match-cap/footer states.
- The group's `ListTags` scope follows the last `scope:`/`in:` chip (`effectiveScope`), matching D-17; `tag:` and `#` tokens behave identically (D-19).
- Selecting a match or the "Add #{tag}" row replaces the in-progress token with `#{tag} ` via a widened `completeToken`.

## Task Commits

Each task was committed atomically; Task 2 (tdd) followed RED (`test(...)`) then GREEN (`feat(...)`) per `tdd.md`:

1. **Task 1: "#qd" shows a ranked Tags group from the cached ListTags query; selecting a row completes the chip** (tracer) - `0e4ffe77` (feat)
2. **Task 2: tag: prefix, scope-following counts, unknown-tag row, footer, loading and error states** (tdd) - `631949de` (test, RED) -> `588d1cb3` (feat, GREEN)

_Plan metadata commit follows this SUMMARY._

## Files Created/Modified

- `ui/src/lib/components/HeaderSearch.svelte` - Adds `tagPrefixInProgress`, the shared `tagsQuery`/`tagRows`/`tagMax`/`tagRank` derived state, the `tagsGroup` snippet (loading/error/populated/unknown-row/footer), two ordering-aware render call sites, and a widened `completeToken`.
- `ui/src/lib/components/HeaderSearch.browser.test.ts` - 9 new browser cases across both tasks (ranking/ordering/completion, scope-following, unknown-tag honesty at both `more` states, match cap + footer, loading, error, and cache-reuse).

## Decisions Made

See `key-decisions` in the frontmatter above (the `completeToken` replace-condition widening, and the classified-kind-gated render ordering that fixes a regression against pre-existing Phase 2 keyboard behavior).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `completeToken` did not replace an in-progress tag token with a non-empty value**
- **Found during:** Task 1, writing the "Enter on the top row makes the input read '#qdrant '" behavior
- **Issue:** `completeToken`'s replace-vs-append branch only fired when `pendingChip` was set (a `#`/`tag:`/`scope:`/`is:` token with an EMPTY value). `#qd` already forms a complete `{ kind: 'tag', value: 'qd' }` chip per `classify.ts` (non-empty value), so `pendingChip` was `undefined` for it, and selecting a suggested tag would have appended a duplicate token (`#qd #qdrant `) instead of replacing the partial one — directly contradicting the plan's own must-have truth ("selecting a row replaces the partial token").
- **Fix:** Widened the condition to `(pendingChip || tagPrefixInProgress !== null) && tokens.length > 0`.
- **Files modified:** `ui/src/lib/components/HeaderSearch.svelte`
- **Verification:** "Enter on the top row makes the input read '#qdrant '" and the `#zzz`-completion test both pass.
- **Committed in:** `0e4ffe77` (Task 1 commit)

**2. [Rule 1 - Bug] The Tags group's new unknown-tag row broke a pre-existing Phase 2 keyboard test**
- **Found during:** Task 2, GREEN verification run (`Enter on the top row hands the classified query and chips to /search through the codec, and clears the input` failed)
- **Issue:** Once the Tags group could render a real item (the "Add #{tag}" unknown-tag row, introduced by Task 2), bits-ui Command's default keyboard highlight moved to it whenever a mixed free-text-plus-partial-tag input (e.g. `"gofmt #ci"`, an existing Phase 2 test fixture) was typed — because the Tags group sat ahead of the "Results" group in DOM order. Pressing `Enter` then selected "Add #ci" instead of the pre-existing "Search all memories" top row, breaking that test's `gotoSpy` assertion.
- **Fix:** Factored the group into a `{#snippet tagsGroup()}` and render it from two call sites: before the Results/search-all section when `classified.kind !== 'text'` (chip-only input — matches every scenario this plan's own tests exercise), and after that section when `classified.kind === 'text'` (free text present) — preserving the pre-existing "Enter hits Search-all" behavior for mixed input while keeping the Tags group visually present and functional either way.
- **Files modified:** `ui/src/lib/components/HeaderSearch.svelte`
- **Verification:** Full suite (`HeaderSearch.browser.test.ts` + `TagCombobox.browser.test.ts`) — 39/39 pass, including the previously-broken pre-existing test.
- **Committed in:** `588d1cb3` (Task 2 GREEN commit)

**3. [Rule 1 - Minor, commit-message scope] Task 2's GREEN commit used the plan's literal `feat(ui): ...` scope rather than the `feat(05-06): ...` scope `tdd.md`'s canonical commit pattern specifies**
- **Found during:** Post-commit TDD gate self-check (this SUMMARY's own drafting)
- **Issue:** `tdd.md`'s Red-Green-Refactor Cycle section specifies `test({phase}-{plan})` → `feat({phase}-{plan})` as the canonical commit-scope contract ("the single source; do not improvise a variant"), matching sibling plan 05-03's actual TDD commits (`test(05-03): ...` / `feat(05-03): ...`). This plan's own `<action>` text quoted a literal `feat(ui): ...` commit message for both tasks, which I followed verbatim for the GREEN commit (`588d1cb3`); the RED commit (`631949de`) I authored independently already used the canonical `test(05-06): ...` scope.
- **Impact:** None functionally — `workflow.tdd_mode` is not enabled in this project's `.planning/config.json`, so the strict `git log --grep` gate-validation this scope mismatch would otherwise trip is inactive. Disclosed here rather than silently left inconsistent, since amending an already-created commit is prohibited by this session's git-safety rules.
- **Files modified:** none (commit-message-only; not fixed via amend per the no-amend rule)
- **Committed in:** N/A (disclosure only)

---

**Total deviations:** 3 auto-fixed (2 correctness bugs, 1 minor commit-scope inconsistency disclosed). **Impact on plan:** The two bug fixes were necessary for the plan's own must-have truths and for not regressing existing Phase 2 behavior; no scope creep. The commit-scope note has no functional impact given `workflow.tdd_mode` is off.

## Issues Encountered

None beyond the two auto-fixed bugs above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- The header search Tags group (TAGS-02, D-16) is complete and shares its cached query and ranking with `TagBars`/`TagCombobox` from plan 05-03 — no further wiring needed for this surface.
- No blockers for plans 05-07 (`/search` docked Tags panel) or 05-08 (`/related` rail Tags tab), which reuse the same `ui/src/lib/tags/*` helpers independently of this plan's `HeaderSearch.svelte` changes.

## Self-Check: PASSED

All modified files verified present on disk; all 3 commits (`0e4ffe77`, `631949de`, `588d1cb3`) verified present in `git log`.

---
*Phase: 05-related-memories-graph-tag-cloud*
*Completed: 2026-09-28*
