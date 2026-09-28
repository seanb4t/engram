---
phase: 04-curation-surfaces
reviewed: 2026-09-27T00:00:00Z
depth: standard
files_reviewed: 71
files_reviewed_list:
  - .claude/skills/engram-connect-client/SKILL.md
  - .claude/skills/engram-console-conventions/SKILL.md
  - internal/e2e/console_browser_test.go
  - internal/webauth/static_test.go
  - ui/package.json
  - ui/src/app.css
  - ui/src/lib/a11y/axe.browser.test.ts
  - ui/src/lib/a11y/axe.ts
  - ui/src/lib/a11y/surfaces.browser.test.ts
  - ui/src/lib/components/AppShell.browser.test.ts
  - ui/src/lib/components/AppShell.svelte
  - ui/src/lib/components/ArchiveConfirmDialog.browser.test.ts
  - ui/src/lib/components/ArchiveConfirmDialog.svelte
  - ui/src/lib/components/ChainDialog.browser.test.ts
  - ui/src/lib/components/ChainDialog.svelte
  - ui/src/lib/components/CommandMenu.browser.test.ts
  - ui/src/lib/components/CommandMenu.svelte
  - ui/src/lib/components/CurationSurfaces.browser.test.ts
  - ui/src/lib/components/CurationSurfaces.svelte
  - ui/src/lib/components/DeleteConfirmDialog.browser.test.ts
  - ui/src/lib/components/DeleteConfirmDialog.svelte
  - ui/src/lib/components/DetailPane.browser.test.ts
  - ui/src/lib/components/DetailPane.svelte
  - ui/src/lib/components/FacetStrip.svelte
  - ui/src/lib/components/HeaderSearch.browser.test.ts
  - ui/src/lib/components/HeaderSearch.svelte
  - ui/src/lib/components/ResultHoverCard.svelte
  - ui/src/lib/components/ResultRow.browser.test.ts
  - ui/src/lib/components/ResultRow.svelte
  - ui/src/lib/components/ResultsHeader.browser.test.ts
  - ui/src/lib/components/ResultsHeader.svelte
  - ui/src/lib/components/ResultsList.browser.test.ts
  - ui/src/lib/components/ResultsList.svelte
  - ui/src/lib/components/RowActions.svelte
  - ui/src/lib/components/ScopeChip.svelte
  - ui/src/lib/components/SupersedeDialog.browser.test.ts
  - ui/src/lib/components/SupersedeDialog.svelte
  - ui/src/lib/components/WriteSurfaces.svelte
  - ui/src/lib/components/ui/tabs/tabs-trigger.svelte
  - ui/src/lib/curation/chain.test.ts
  - ui/src/lib/curation/chain.ts
  - ui/src/lib/curation/flash.svelte.ts
  - ui/src/lib/curation/host.svelte.ts
  - ui/src/lib/curation/supersede-rejection.test.ts
  - ui/src/lib/curation/supersede-rejection.ts
  - ui/src/lib/mutations/curation.test.ts
  - ui/src/lib/mutations/curation.ts
  - ui/src/lib/mutations/memory.test.ts
  - ui/src/lib/mutations/memory.ts
  - ui/src/lib/queries.test.ts
  - ui/src/lib/queries.ts
  - ui/src/lib/resume.test.ts
  - ui/src/lib/resume.ts
  - ui/src/lib/search/recall-header.test.ts
  - ui/src/lib/search/recall-header.ts
  - ui/src/lib/search/rules-params.test.ts
  - ui/src/lib/search/rules-params.ts
  - ui/src/lib/search/scheduled-params.test.ts
  - ui/src/lib/search/scheduled-params.ts
  - ui/src/lib/time.test.ts
  - ui/src/lib/time.ts
  - ui/src/routes/+page.svelte
  - ui/src/routes/discovery/+page.svelte
  - ui/src/routes/discovery/discovery.browser.test.ts
  - ui/src/routes/page.browser.test.ts
  - ui/src/routes/rules/+page.svelte
  - ui/src/routes/rules/rules.browser.test.ts
  - ui/src/routes/scheduled/+page.svelte
  - ui/src/routes/scheduled/scheduled.browser.test.ts
  - ui/src/routes/search/+page.svelte
  - ui/src/routes/search/search.browser.test.ts
findings:
  critical: 0
  warning: 6
  info: 2
  total: 8
status: issues_found
---

# Phase 04: Code Review Report

**Reviewed:** 2026-09-27T00:00:00Z
**Depth:** standard
**Files Reviewed:** 71
**Status:** issues_found

## Summary

Reviewed the Phase 4 curation-surfaces work (supersede/archive/restore/chain dialogs, the
keyboard/selection model, the two new routes `/rules` and `/scheduled`, resume-envelope
widening, and the accompanying Go e2e/unit tests) against the two loaded project skills
(`engram-console-conventions`, `engram-connect-client`) and the codebase's own extensive
inline design-decision comments.

The implementation is unusually well-documented and internally consistent: the keyboard
model, dim/state-word rules, resume-envelope open-redirect guard, CSRF/client-split
discipline, and cache-invalidation strategy all match what the two skills document. No
security vulnerabilities, hardcoded secrets, injection vectors, or crash-level defects were
found. No debug artifacts, empty catch blocks, or dangerous-function usage were found by
pattern scan either.

What I did find is a cluster of real-but-narrow client-side race conditions and stale-state
bugs, mostly sharing one root cause: several `$effect`s that fire an async callback do not
apply the same "stale response" revision-guard that `SupersedeDialog`'s own `validate_only`
preview effect uses — so a handful of async UI updates can occasionally land against
superseded state. None of these corrupt server data (the server's own preflight is
authoritative either way), but they can show the operator inconsistent or stale UI, and in
one case (the add-target-by-id/short_id lookup) a slow/stale response could let an operator
add the *wrong* record to a destructive supersede operation. I also found two one-way/never-
resets pieces of UI state (state-chip overflow collapse, `ChainDialog`'s `peekId`) and one
UX asymmetry between the two documented "undo" surfaces for archive/restore.

## Warnings

### WR-01: Add-target-by-id/short_id lookup has no stale-response guard

**File:** `ui/src/lib/components/SupersedeDialog.svelte:205-228`
**Issue:** The debounced add-by-id/short_id lookup effect has no revision counter or
`AbortController`, unlike the `validate_only` preview effect a few lines below it (lines
267-304), which explicitly guards against exactly this class of race
(`if (rev !== previewRevision) return; // superseded by a later draft`). If the 200ms
debounce fires and the `onlookup(raw)` promise is still in flight when the operator edits
`addValue` again (a plausible sequence on a slow network — type a short_id, pause, start
typing a different one before the first lookup resolves), the effect re-runs and resets
`addResult = undefined`, but the **earlier** promise's `.then` still completes afterward and
unconditionally overwrites `addResult`/`addLookupPending` with the stale lookup's result.
Because `selectAddResult` adds whatever `addResult` currently holds to `currentTargets`
(the predecessor set for a destructive, no-undo `SupersedeMemory` commit), an operator who
trusts what's on screen could unknowingly add the record from their *first* typed value
rather than their second.
**Fix:** Add the same revision-guard pattern used by the preview effect — increment a
counter per effect run and ignore the resolved promise if a newer run has already started:
```ts
let lookupRevision = 0;
$effect(() => {
  const raw = addValue.trim();
  addResult = undefined;
  const rev = ++lookupRevision;
  if (!raw || !onlookup) { addLookupPending = false; return; }
  const classified = classifyInput(raw);
  if (classified.kind !== 'id' && classified.kind !== 'short_id') { addLookupPending = false; return; }
  const timer = setTimeout(async () => {
    addLookupPending = true;
    try {
      const found = await onlookup(raw);
      if (rev !== lookupRevision) return; // superseded by a newer query
      addLookupPending = false;
      addResult = found;
    } catch {
      if (rev === lookupRevision) addLookupPending = false;
    }
  }, ADD_LOOKUP_DEBOUNCE_MS);
  return () => clearTimeout(timer);
});
```

### WR-02: `onresolvehead` never retries a failed head resolution

**File:** `ui/src/lib/components/SupersedeDialog.svelte:124-135`
**Issue:** `headRequestsInFlight` (a plain `Set`, not reset) permanently marks a target id as
"already requested" the moment `onresolvehead(t.id)` is called, regardless of whether the
call succeeds. If the head-resolution RPC fails or returns `undefined` (transient network
blip, server hiccup), the `if (t.supersededBy && !headRequestsInFlight.has(t.id) && ...)`
guard means this id is never retried for the lifetime of the dialog — the "use head …" chip
stays stuck on the `…` placeholder forever. The component's own doc comment in
`CurationSurfaces.svelte:311-316` explicitly documents the intended behavior as "the dialog
just keeps showing the '…' placeholder until a later read succeeds," which the current
one-shot `Set` cannot deliver — nothing ever triggers a "later read."
**Fix:** Remove the id from `headRequestsInFlight` in the failure/no-result branch (or key on
a retry-eligible boolean rather than presence alone) so a later re-render (e.g. after the
target list changes, or on a manual retry affordance) can re-attempt the resolution.

### WR-03: State-chip overflow collapse never un-collapses

**File:** `ui/src/lib/components/ResultRow.svelte:95-126`
**Issue:** `measureStatesOverflow()` only transitions `statesCollapsed` from `false` to
`true` (`if (!statesCollapsed && statesEl.scrollWidth > statesEl.clientWidth + 1)`); it never
checks whether a now-collapsed row would fit again after the box grows (window resize wider,
text-size decreased, a sidebar/detail-pane closing that widens the list column). The
`ResizeObserver` re-invokes `measureStatesOverflow` on every resize, but the one-directional
guard means a row that collapsed to `first +N` at a narrow width stays collapsed even once
there is clearly enough room, until its `stateWords` array itself changes identity (a
different memory record, or an actual state change). This contradicts the resize-driven,
"re-measured on resize of that bounded box" design the component's own comment (lines 87-91)
describes.
**Fix:** Let `measureStatesOverflow` also un-collapse when there's room:
```ts
function measureStatesOverflow() {
  if (!statesEl) return;
  if (stateWords.length <= 1) { statesCollapsed = false; return; }
  const overflowing = statesEl.scrollWidth > statesEl.clientWidth + 1;
  if (overflowing !== statesCollapsed) statesCollapsed = overflowing;
}
```
(Watch for layout thrash: measuring `scrollWidth` after un-collapsing needs a second pass in
the un-collapsed layout to detect it should re-collapse; a `requestAnimationFrame` double-
measure may be needed instead of a single synchronous flip.)

### WR-04: `ChainDialog`'s `peekId` is not reset when the dialog reopens for a different anchor

**File:** `ui/src/lib/components/ChainDialog.svelte:91-102`
**Issue:** `peekId` is plain component state with no `$effect` resetting it when `anchorId`
changes (i.e., when the host calls `openChain(id)` for a *different* record after the dialog
was previously closed with a node peeked open). Two consequences: (1) `peekQuery` stays
`enabled: !!peekId` and will re-fire a `getMemory` fetch for the stale id the instant the
dialog reopens, even though nothing in the newly-rendered chain references it; (2) if the
same id happens to also appear in the new chain (plausible — chains for related records
often share nodes, e.g. reopening the chain from the same head/predecessor set), the peek
panel for that node auto-expands without the operator clicking it this time, which reads as
the dialog "remembering" an expansion state it shouldn't.
**Fix:** Reset `peekId` on anchor change:
```ts
let lastAnchorId: string | undefined;
$effect(() => {
  if (anchorId !== lastAnchorId) {
    lastAnchorId = anchorId;
    peekId = undefined;
  }
});
```

### WR-05: Shift+click range-select does not move keyboard focus (`activeId`)

**File:** `ui/src/lib/components/ResultsList.svelte:481-491, 594-615`
**Issue:** The keyboard path for Shift+X (`handleListboxKey`'s `case 'X'`) extends the range
to the currently **active** row and, because `m = memories[current]` *is* the active row,
keyboard focus is already there. The pointer path (`handleOptionClick`'s `if (event.shiftKey)
{ selectRange(id); return; }`) extends the range to the **clicked** row but never assigns
`activeId = id` the way a plain click does via `selectRow(id)` (line 594-597). Per the
`engram-console-conventions` skill, `⇧click` is documented as equivalent to `⇧X` ("Select an
inclusive range from the anchor row to the active/clicked row"), implying the clicked row
becomes the new reference point for subsequent keyboard navigation — but after a shift+click,
pressing `j`/`k` continues to move relative to whatever row was active *before* the
shift+click, not the just-clicked row. This is a real, reproducible UX inconsistency between
the two ways of doing the same operation.
**Fix:** Set `activeId = id` in the shift+click branch of `handleOptionClick`, mirroring
`selectRow`:
```ts
if (event.shiftKey) {
  selectRange(id);
  activeId = id;
  return;
}
```

### WR-06: Toast-triggered restore-undo (D-09 surface 2) skips the row flash and `onchanged` notification that the in-dialog undo (surface 1) performs

**File:** `ui/src/lib/components/CurationSurfaces.svelte:205-222` (compare `onundo`,
lines 185-203)
**Issue:** `onundo` (the "Undo — restore N" button inside the still-open result dialog) calls
`flashRows(changed)` and `onchanged?.({ kind: inverseMode, ids: changed })` after the inverse
mutation succeeds — this is what drives the row highlight and what lets the hosting route
clear its bulk-selection state (`onchanged={() => (selectedIds = [])}` in both
`search/+page.svelte` and `scheduled/+page.svelte`). `ondone`'s 8-second toast "Undo" action
(the *other* documented D-09 undo surface, fired after the dialog has already closed) instead
calls `restoreMutation.mutate({ ids })` directly with no `onSuccess` callback at all —
`applyArchiveResultsOptimistic`/cache invalidation still happen (inside the mutation hook's
own `onSuccess`), but the restored rows never join `flashing` and the route's `selectedIds`
is never cleared. `CurationSurfaces.browser.test.ts`'s own test for this path
(`"Done after an archive result toasts '1 archived · Undo'…"`) only asserts that
`restoreMemorySpy` was called with the right ids — it does not (and, given the current
implementation, cannot) assert a flash or an `onchanged` call, confirming this is an
untested/unintended asymmetry between the two undo affordances rather than a deliberate
simplification.
**Fix:** Route the toast's `onClick` through the same `restoreMutation.mutateAsync` +
`flashRows` + `onchanged` sequence `onundo` already uses, e.g. extract a shared
`runRestore(ids)` helper both call.

## Info

### IN-01: Client-side "not owned" guess for a rejected supersede/archive target can be wrong for a since-deleted record

**File:** `ui/src/lib/components/SupersedeDialog.svelte:177-183`,
`ui/src/lib/components/ArchiveConfirmDialog.svelte:91-100`
**Issue:** Both dialogs disambiguate a server-side `not found`/`NOT_FOUND` rejection (which
the memory contract deliberately makes indistinguishable between "doesn't exist," "not
owned," and "ambiguous short_id") by checking whether the *client's own stale cached copy* of
the target has `visibility === 'shared'`. If a record the client resolved as `shared` is then
genuinely deleted between resolution and submit, the UI will render "shared by {owner} —
readable, but only the owner can supersede/archive" for a target that no longer exists at
all — a plausible-but-incorrect explanation. This is a low-frequency, non-destructive
cosmetic inaccuracy (the underlying `NOT_FOUND_NOTE`/generic copy already tells the operator
"nothing was changed" either way), not a functional bug.
**Fix:** Consider softening the copy to acknowledge the ambiguity (e.g. "shared by {owner}
when last read — not found on the server: not owned, deleted, or an ambiguous short id"), or
leave as-is if the UX tradeoff (more specific in the common case) is accepted.

### IN-02: `resolveRecords`/`resolveRecordsKeepAll` re-fetch is not deduplicated against concurrent opens

**File:** `ui/src/lib/components/CurationSurfaces.svelte:68-128`
**Issue:** `openArchive`/`openRestore`/`openSupersede` are exported async functions with no
in-flight guard; a double-invocation (e.g. a fast double-click on a row action button before
the dialog's `archiveOpen`/`supersedeOpen` flips to `true` and starts intercepting further
input) would run two overlapping `resolveRecords` calls and could set `archiveRecords`/
`supersedeTargets` from whichever resolves last, non-deterministically. Given `RowActions`
buttons aren't disabled while a resolve is pending and there is no debounce, this is a
plausible (if narrow) real-world sequence. No data-corruption path — worst case is a dialog
opening with the wrong initial target set, correctable via the visible chip list before
confirming — but worth a guard for robustness.
**Fix:** Track an in-flight generation/request id the way `SupersedeDialog`'s preview effect
does, or disable the row toolbar buttons for the duration of `resolveRecords`.

---

_Reviewed: 2026-09-27T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
