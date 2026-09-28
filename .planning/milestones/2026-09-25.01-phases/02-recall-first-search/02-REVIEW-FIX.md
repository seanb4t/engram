---
phase: 02-recall-first-search
fixed_at: 2026-09-27T00:19:02Z
review_path: .planning/phases/02-recall-first-search/02-REVIEW.md
iteration: 1
findings_in_scope: 5
fixed: 5
skipped: 0
status: all_fixed
---

# Phase 2: Recall-First Search — Code Review Fix Report

**Fixed at:** 2026-09-27T00:19:02Z
**Source review:** .planning/phases/02-recall-first-search/02-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope (critical + warning): 5
- Fixed: 5
- Skipped: 0

Ran on the current branch `feat/2026-09-25.01` in the main checkout (no worktree, per
explicit dispatcher instruction — `workflow.use_worktrees` normally applies but was
overridden for this run). Verification (tests, build) ran in the same checkout, so
these results are directly reproducible there.

Each fix has a `vitest-browser` test that was confirmed to fail without the fix
(verified by temporarily reverting each source change in isolation and re-running its
new test, then restoring) and pass with it. `pnpm --dir ui test` is green (511/511,
47 files) on the final state; two unrelated flakes were observed and confirmed
transient by re-running (`DiscoveryFormSheet.browser.test.ts`'s re-auth "Create"
button click once, timing-sensitive under parallel workers) — no ScopesSidebar flake
was observed. `pnpm --dir ui build` succeeds. The vendored SPA in
`internal/webauth/static` was rebuilt via `task ui:build` after the fixes and a
second run reproduces an identical tree (clean `git status`).

## Fixed Issues

### WR-01: Role-rewrite to `listbox` silently drops the vendored library's keyboard focus-visible ring

**Files modified:** `ui/src/lib/components/ResultsList.svelte`, `ui/src/lib/components/ResultsList.browser.test.ts`
**Commit:** `ec80f303`
**Applied fix:** Added a compensating `:focus-visible` outline rule scoped to
`.results-listbox-wrapper :global([role='listbox']:focus-visible)`, since
`applyRole()` rewrites the vendored viewport's `role` from `region` to `listbox`,
un-matching the library's own `[role='region']:focus-visible` rule. New test
asserts `outlineStyle`/`outlineWidth` on the focused, rewritten listbox element.

### WR-02: `/search`'s detail pane cannot be closed for an id/short_id-classified query

**Files modified:** `ui/src/routes/search/+page.svelte`, `ui/src/routes/search/search.browser.test.ts`
**Commit:** `4cfdf8dc`
**Applied fix:** Added a `dismissedAutoOpen` `$state` flag (reset via `$effect` when
`idToFetch` changes, so a fresh resolution still auto-opens once) that `autoOpenId`
now respects. Every close affordance (`RecallSplit`'s close button, `DetailPane`'s
close button, `ResultsList`'s `onescape`) now routes through a new `closeSel()`
helper that sets the flag before clearing `sel`. `onopen` is replaced by a
`toggleOpen(id)` helper that compares against `effectiveSel` (not the stale
`params.sel`) so a second Enter/click on an auto-opened row is recognized as
"close", not a redundant "open". Two new tests cover the close-button path and the
second-click-toggle path.

### WR-03: `ScopeCombobox`'s "Retry" button is dead — `FacetStrip` never wires `onretry`

**Files modified:** `ui/src/lib/components/FacetStrip.svelte`, `ui/src/lib/components/FacetStrip.browser.test.ts`, `ui/src/routes/search/+page.svelte`
**Commit:** `f4315f55`
**Applied fix:** `FacetStrip` now accepts an optional `onretry` prop and forwards it
to `ScopeCombobox`; `+page.svelte` wires `onretry={() => scopesQ.refetch()}`. New
test opens the combobox with a scopes-load error and confirms clicking "Retry"
invokes the passed-through callback.

### WR-04: `HeaderSearch`'s "fix" rows for `lower-k`/`without-full`/`clear-created` are no-ops

**Files modified:** `ui/src/lib/components/HeaderSearch.svelte`, `ui/src/lib/components/HeaderSearch.browser.test.ts`
**Commit:** `f59ba848`
**Applied fix:** Since `k`/`full` are hardcoded (`50n`/`false`) for every
`searchQuery` call in this component with no local state for those rows to mutate,
added a `visibleFixRows` derived value that filters `lower-k`/`without-full`/
`clear-created` out of `fixRowsFor`'s output before rendering (the always-appended
generic `retry` row survives the filter). New test triggers a `response_too_large`
rejection and asserts only "Retry the request" renders, never the two no-op labels.

### WR-05: `/search`'s query-box debounce timer is never cleared on component teardown

**Files modified:** `ui/src/routes/search/+page.svelte`, `ui/src/routes/search/search.browser.test.ts`
**Commit:** `d5c64efc`
**Applied fix:** Added `onDestroy(() => { if (debounceTimer) clearTimeout(debounceTimer); })`,
matching every other timer/listener cleanup pattern already used in this file. New
test types into the search box, unmounts the route before the 160ms debounce fires,
and asserts `goto` is never called afterward.

## Skipped Issues

None — all 5 in-scope findings (Critical: 0, Warning: 5) were fixed. IN-01 was out
of scope for this run (`fix_scope: critical_warning` excludes Info-severity findings).

---

_Fixed: 2026-09-27T00:19:02Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
