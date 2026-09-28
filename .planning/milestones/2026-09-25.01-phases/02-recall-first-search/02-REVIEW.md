---
phase: 02-recall-first-search
reviewed: 2026-09-27T00:01:05Z
depth: standard
files_reviewed: 40
files_reviewed_list:
  - internal/server/hiddencount.go
  - internal/server/hiddencount_test.go
  - internal/server/connectapi.go
  - internal/server/connectapi_test.go
  - internal/server/connectapi_crossspine_test.go
  - internal/server/connectdescriptor_test.go
  - internal/server/fakestore_test.go
  - internal/server/store_iface.go
  - internal/server/tools.go
  - internal/server/tools_test.go
  - proto/engram/v1/engram.proto
  - cmd/engram/client_common.go
  - cmd/engram/client_list.go
  - cmd/engram/client_list_test.go
  - cmd/engram/client_search.go
  - cmd/engram/client_search_test.go
  - docs-site/src/content/docs/guides/cli.md
  - docs-site/src/content/docs/guides/upgrade.md
  - docs-site/src/content/docs/reference/tools.md
  - ui/src/routes/search/+page.svelte
  - ui/src/routes/+page.svelte
  - ui/src/routes/observe/+page.svelte
  - ui/src/routes/+layout.svelte
  - ui/src/lib/components/ResultsList.svelte
  - ui/src/lib/components/ResultsList.browser.test.ts
  - ui/src/lib/components/ResultRow.svelte
  - ui/src/lib/components/ResultHoverCard.svelte
  - ui/src/lib/components/ResultsHeader.svelte
  - ui/src/lib/components/DetailPane.svelte
  - ui/src/lib/components/RecallSplit.svelte
  - ui/src/lib/components/FacetStrip.svelte
  - ui/src/lib/components/HeaderSearch.svelte
  - ui/src/lib/components/CommandMenu.svelte
  - ui/src/lib/components/ScopeCombobox.svelte
  - ui/src/lib/components/WriteSurfaces.svelte
  - ui/src/lib/components/DisplayPopover.svelte
  - ui/src/lib/components/AppShell.svelte
  - ui/src/lib/search/classify.ts
  - ui/src/lib/search/params.ts
  - ui/src/lib/search/recall-header.ts
  - ui/src/lib/search/header-search.svelte.ts
  - ui/src/lib/errors/connect-error.ts
  - ui/src/lib/display.svelte.ts
findings:
  critical: 0
  warning: 5
  info: 1
  total: 6
status: issues_found
---

# Phase 2: Recall-First Search — Code Review Report

**Reviewed:** 2026-09-27T00:01:05Z
**Depth:** standard
**Files Reviewed:** 40 (of 83 files in scope; remaining files are mechanical
`px` → `calc(N * var(--u))` token-conversion diffs, generated/vendored code,
or test-only files, sampled and found low-risk — see Summary)
**Status:** issues_found

## Summary

Focus areas requested by the dispatcher were traced end-to-end:

1. **Recall-gate hidden-count authz** (`internal/server/hiddencount.go`,
   `tools.go`, `connectapi.go`): **clean**. Both `listRecallHidden` and
   `searchRecallHidden` build their comparison call with the caller's own
   `c.Subj` and the same resolved `scope`, call plain `Store.Search`/
   `Store.List` (never `SearchReranked`), copy every filterable field
   (`Tags`/`Categories`/`CreatedAfter`/`CreatedBefore` — verified against
   `store.SearchOptions`'s full field list), and degrade to `nil` (never a
   fabricated zero) on a comparison failure, proven by
   `TestRecallHiddenDegradesOnComparisonFailure`. MCP's `searchArgs`/
   `listArgs` structs carry no `Include*` fields at all, so the MCP recall
   gate genuinely never relaxes (D-03). No information-disclosure path found.
2. **Stale-response / `AbortSignal` handling** (`ui/src/routes/search/+page.svelte`):
   **clean**. Every query passes `{ signal }` through to `engram.*`, keys are
   built from the full normalized `effective` params, and TanStack Query's
   own key-change cancellation is relied on rather than a hand-rolled
   latest-response guard — the pattern PITFALLS.md's "search race" warns
   against is avoided.
3. **`ResultsList.svelte`'s capture-phase role-rewrite action**: functionally
   sound (keyboard model, focus tracking, hover-card decoupling all verified
   against the vendored library source), but see **WR-01** — the rewrite
   silently defeats the vendored library's own focus-visible ring.
4. **The id-mode pane-close rough edge** flagged in 02-08/02-09 SUMMARYs is
   still present and reproducible in the final code — see **WR-02**.

Three further issues were found by tracing components adjacent to the
requested focus areas (`FacetStrip`/`ScopeCombobox`, `HeaderSearch`'s fix
rows). No Critical-severity findings: no injection, no auth bypass, no data
loss, no crash paths were found in the reviewed files.

## Warnings

### WR-01: Role-rewrite to `listbox` silently drops the vendored library's keyboard focus-visible ring

**File:** `ui/src/lib/components/ResultsList.svelte:352-412` (the `listboxViewport` action, specifically `applyRole()`)
**Issue:** `@humanspeak/svelte-virtual-list`'s own viewport ships a scoped
CSS rule keyed on the attributes it renders itself with:

```css
/* ui/node_modules/.../SvelteVirtualList.svelte */
:where(div[role='region'][tabindex='0']):focus-visible {
  outline: 2px solid currentColor;
  outline-offset: -2px;
}
```

`applyRole()` rewrites that element's `role` attribute from `region` to
`listbox` (correctly, per D-07/Pitfall A), but never adds a matching
`:focus-visible` rule for the new `role="listbox"` state, and
`ResultsList.svelte`'s own `<style>` block defines no compensating focus
ring for `.results-listbox-wrapper` or the viewport it wraps. Once the
role-rewrite runs (immediately after mount, confirmed by the test at line 63
of `ResultsList.browser.test.ts` focusing the listbox), the attribute
selector above no longer matches, so a keyboard user who tabs to or focuses
the results list gets **no visible focus indicator at all** — the exact
failure mode the library's own comment ("the viewport fills a container with
`overflow: hidden`, so the default outside outline is clipped away
entirely") warns about.
**Fix:** add an explicit focus-visible rule scoped to the rewritten role, e.g.:
```css
.results-listbox-wrapper :global([role='listbox']:focus-visible) {
  outline: 2px solid currentColor;
  outline-offset: -2px;
}
```

### WR-02: `/search`'s detail pane cannot be closed for an id/short_id-classified query (known, unresolved)

**File:** `ui/src/routes/search/+page.svelte:178-183`
**Issue:**
```ts
const autoOpenId = $derived(
  (classified.kind === 'id' || classified.kind === 'short_id') && idQ.data?.memory && !shortIdMissText
    ? idQ.data.memory.id
    : ''
);
const effectiveSel = $derived(params.sel || autoOpenId);
```
Every close path (`DetailPane`'s close button, `ResultsList`'s `onescape`,
a second `Enter` on the open row) calls `navigate({ sel: '' })`. Because
`params.sel` becomes `''` (falsy), `effectiveSel` immediately falls back to
`autoOpenId`, which is still non-empty as long as the query stays classified
as `id`/`short_id` and `idQ.data` is loaded — so the pane silently reopens
on the same record and the close affordance appears to do nothing. This was
flagged as a known, untested "rough edge" in `02-08-SUMMARY.md` and
`02-09-SUMMARY.md` ("clicking the pane's close button does not fully
collapse it") and remains unresolved in the code delivered for this phase.
**Fix:** track an explicit "user dismissed the auto-open" flag (e.g.
`let dismissedAutoOpen = $state(false)`, set on close, reset whenever
`classified.kind`/`idToFetch` changes), or drop `sel=''` as the close signal
for this branch and instead navigate to a distinct sentinel/empty query.

### WR-03: `ScopeCombobox`'s "Retry" button is dead — `FacetStrip` never wires `onretry`

**File:** `ui/src/lib/components/FacetStrip.svelte:79-86`, `ui/src/lib/components/ScopeCombobox.svelte:70-74`
**Issue:** `ScopeCombobox` renders a `Retry` button on a scopes-load error
that calls `onretry?.()`:
```svelte
<button type="button" class="scope-combobox-retry" onclick={() => onretry?.()}>Retry</button>
```
`FacetStrip` is the only caller in this phase's diff, and it never passes
`onretry`:
```svelte
<ScopeCombobox
  value={params.scope} crossSpine={params.crossSpine} {scopes}
  loading={scopesLoading} error={scopesError}
  onselect={(s) => onchange({ scope: s, crossSpine: s ? false : true })}
/>
```
Because `onretry` is optional (`onretry?: () => void`), no error is thrown —
the button simply renders and does nothing when clicked. `+page.svelte`'s
`scopesQ` already exposes a working `.refetch()` (used elsewhere, e.g.
`onRecallFix`'s `activeResult?.refetch()`), so a real retry path exists and
was simply not threaded through.
**Fix:** pass `onretry={() => scopesQ.refetch()}` (or equivalent) from
`+page.svelte` through `FacetStrip` to `ScopeCombobox`.

### WR-04: `HeaderSearch`'s "fix" rows for `lower-k`/`without-full`/`clear-created` are no-ops that just retry the identical failing request

**File:** `ui/src/lib/components/HeaderSearch.svelte:317-332`
**Issue:**
```ts
function applyFixRow(row: FixRow) {
  switch (row.id) {
    case 'enable-cross-spine':
      crossSpineOff = false;
      break;
    case 'pick-scope':
      headerSearch.text = headerSearch.text.trim() ? `${headerSearch.text.trim()} scope:` : 'scope:';
      break;
    case 'lower-k':
    case 'without-full':
    case 'clear-created':
    case 'retry':
      searchQuery.refetch();
      break;
  }
}
```
`fixRowsFor` (shared with `/search`, `ui/src/lib/errors/connect-error.ts`)
labels these rows "Show fewer results" (`lower-k`) and "Retry without full
content" (`without-full`). In `HeaderSearch`, `k` is hardcoded to `50n` and
`full` is hardcoded to `false` in every `searchQuery` call
(`HeaderSearch.svelte:115-117`) — there is no local state these rows could
mutate, so all three collapse to a plain `retry`, which will fail again with
the identical `out_of_range`/`response_too_large` error. The user sees an
action-labelled button that silently does nothing different from "Retry".
**Fix:** either omit the `lower-k`/`without-full`/`clear-created` rows in the
header search's rendering of `fixRowsFor` output (falling back to a single
generic retry) or thread the equivalent state (a smaller hardcoded `k`
fallback, `full: false` is already the case) so the label matches the
behavior.

### WR-05 *(Info-adjacent, listed as Warning for consistency with debounce-timer leak conventions elsewhere in this file)*: `/search`'s query-box debounce timer is never cleared on component teardown

**File:** `ui/src/routes/search/+page.svelte:79-91`
**Issue:**
```ts
let debounceTimer: ReturnType<typeof setTimeout> | undefined;
function onInput(e: Event) {
  ...
  if (debounceTimer) clearTimeout(debounceTimer);
  debounceTimer = setTimeout(() => {
    goto(`${base}/search?${encodeSearchParams({ ...params, q: value, sel: '' })}`, { ... });
  }, 160);
}
```
There is no `onDestroy`/effect cleanup cancelling `debounceTimer` when the
route unmounts (e.g. the user types then immediately navigates away within
160ms). The pending `goto()` still fires against a torn-down route's stale
`params` closure. In practice SvelteKit's `goto` is safe to call from an
unmounted context, so this is unlikely to crash, but it is a real,
easily-fixed leak/race that every other `$effect`-based listener in this
same file (`onScroll`, `onTextSize`, etc.) is careful to clean up — this one
callback is the exception.
**Fix:** wrap the timer in an `$effect` with a cleanup function, or clear it
in `onDestroy`.

## Info

### IN-01: Duplicated "Scopes"/"Categories" `Command.Group` markup in `HeaderSearch.svelte`

**File:** `ui/src/lib/components/HeaderSearch.svelte:480-537`
**Issue:** The `Scopes` and `Categories` command groups are each written out
twice (once gated on `scopePrefixInProgress !== null`, once on
`=== null`) with byte-identical `{#each}` bodies, purely to relocate the
group above/below the results depending on token-completion state. This is
intentional (per the inline comment) but doubles the maintenance surface for
any future change to how a scope/category row renders.
**Fix:** extract each block into a `{#snippet scopesGroup()}` /
`{#snippet categoriesGroup()}` and `{@render}` it from both positions.

---

_Reviewed: 2026-09-27T00:01:05Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
