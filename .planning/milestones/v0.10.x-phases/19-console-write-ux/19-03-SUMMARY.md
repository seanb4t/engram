---
phase: 19-console-write-ux
plan: 03
subsystem: ui
tags: [svelte, dialog, dropdown-menu, bits-ui, destructive-ux]

# Dependency graph
requires:
  - phase: 19-console-write-ux (Plan 01)
    provides: "--destructive design tokens and a real destructive Button variant, and the re-vendored gen client's Visibility enum/Memory shape"
provides:
  - "DeleteConfirmDialog.svelte — host-authoritative delete confirm dialog (kind-parameterized memory|discovery), awaitable onconfirm, pending-state double-fire guard, optional authFailure/onreauth re-auth CTA"
  - "ShareWarningInline.svelte — host-controlled inline share-warning banner with accurate (unshare-is-possible) copy, same awaitable-onconfirm/pending/authFailure pattern"
  - "MemoryRow.svelte restructured to a non-button shell + sibling hover kebab DropdownMenu (Edit/Delete/Share), MemoryList.svelte onedit/ondelete/onshare passthrough"
  - "MemoryDetail.svelte header kebab DropdownMenu (Edit/Delete/Share) next to the existing copy button"
affects: [19-04-write-forms, 19-06-route-wiring]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Host-authoritative surface closure: DeleteConfirmDialog's open is $bindable and driven only by bits-ui's internal onOpenChange (Cancel/Escape/overlay) → oncancel(); a host-driven open=false assignment never routes through that callback, so success-close and cancel-close are distinguishable without extra state"
    - "Awaitable onconfirm + local pending $state disables both action buttons and (for the dialog) suppresses Escape/interactOutside dismissal — one shared shape used identically by DeleteConfirmDialog and ShareWarningInline"
    - "bits-ui asChild via {#snippet child({ props })} to make a shadcn Button itself the DropdownMenu/Dialog trigger/close element — avoids nested <button> (matches the existing dialog-content.svelte X-close precedent)"
    - "Per-item DropdownMenu.Item gating on its own optional callback prop ({#if onedit}/{#if ondelete}/{#if onshare && visibility!=='shared'}) plus one mechanical category==='rule' guard around the whole trigger — reused identically in MemoryRow and MemoryDetail"

key-files:
  created:
    - ui/src/lib/components/DeleteConfirmDialog.svelte
    - ui/src/lib/components/DeleteConfirmDialog.browser.test.ts
    - ui/src/lib/components/ShareWarningInline.svelte
    - ui/src/lib/components/ShareWarningInline.browser.test.ts
  modified:
    - ui/src/lib/components/MemoryRow.svelte
    - ui/src/lib/components/MemoryRow.browser.test.ts
    - ui/src/lib/components/MemoryList.svelte
    - ui/src/lib/components/MemoryDetail.svelte
    - ui/src/lib/components/MemoryDetail.browser.test.ts

key-decisions:
  - "Cancel is wrapped in bits-ui's Dialog.Close (not a plain onclick), so Cancel/Escape/overlay all funnel through the SAME onOpenChange(false) → oncancel() path, while Delete stays a plain Button (never touches bits-ui's internal open state) — this is what makes 'self-close on confirm' structurally impossible rather than merely avoided by convention"
  - "MemoryDetail's Edit/Delete/Share collapse into a single icon-sm ghost kebab DropdownMenu (mirroring MemoryRow) rather than three separate outline/sm buttons, per UI-SPEC's explicit overflow-note alternative (360px pane + existing copy button leaves little room for 3 more full buttons); the copy button itself is untouched"
  - "Spread order in the {#snippet child({ props })} pattern matters: {...props} must precede an explicit disabled={pending} override, or DialogClose's own (usually-undefined) disabled prop silently wins — caught by a failing pending-state test, fixed before commit"

patterns-established:
  - "Awaitable-callback + local pending $state is the reusable shape for any future destructive/host-controlled inline action (WriteSurfaces in Plan 06 consumes it as-is, no new contract)"

requirements-completed: [REQ-console-write-ux]

coverage:
  - id: D1
    description: "DeleteConfirmDialog renders exact per-kind copy, uses the real destructive Button variant, never self-closes on Delete, and stays open with a re-auth CTA on authFailure"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/lib/components/DeleteConfirmDialog.browser.test.ts (8 tests: memory/discovery copy, confirm-keeps-open, pending disables Delete/Cancel, Cancel→oncancel, host open=false closes without oncancel, authFailure CTA + onreauth, CTA absent when false)"
        status: pass
    human_judgment: false
  - id: D2
    description: "ShareWarningInline renders the accurate (unshare-is-possible) disclosure copy, is a role=alert banner, and supports the same awaitable/pending/authFailure contract"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/lib/components/ShareWarningInline.browser.test.ts (6 tests: copy+alert role, onconfirm, oncancel, pending disables both buttons, authFailure CTA + onreauth, CTA absent when false)"
        status: pass
    human_judgment: false
  - id: D3
    description: "MemoryRow root is not a <button> (no nested-interactive-DOM); hover kebab exposes per-callback-gated Edit/Delete/Share, suppressed for rule records, Share suppressed when already shared; MemoryList threads the three new callbacks"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/lib/components/MemoryRow.browser.test.ts (8 tests, 5 new: root-not-button/no-kebab-without-callbacks, kebab fires Edit/Delete/Share without onselect + sibling-not-nested assertion, rule suppression, shared-hides-Share, private/empty-string-shows-Share, discovery-shape per-item gating)"
        status: pass
    human_judgment: false
  - id: D4
    description: "MemoryDetail header exposes the same gated Edit/Delete/Share via a kebab next to the unchanged copy button"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/lib/components/MemoryDetail.browser.test.ts (5 new tests: no-kebab-without-callbacks, fires Edit/Delete/Share + copy-button-untouched, rule suppression, shared-hides-Share, discovery-shape per-item gating; 8 pre-existing tests still pass)"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-07-15
status: complete
---

# Phase 19 Plan 03: Destructive/Write-Action Presentational Components Summary

**Host-authoritative DeleteConfirmDialog + ShareWarningInline, and hover/header dropdown-menu row/detail actions (Edit/Delete/Share), all pure presentational — no mutation wiring yet.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-07-15T13:52:00Z
- **Completed:** 2026-07-15T14:10:25Z
- **Tasks:** 3
- **Files modified:** 9 (4 created, 5 modified)

## Accomplishments

- `DeleteConfirmDialog.svelte` and `ShareWarningInline.svelte` ship with the exact UI-SPEC copy (kind-parameterized memory/discovery for the dialog; the corrected, accurate share-disclosure sentence for the banner) and a real `destructive` Button, and are structurally incapable of self-closing on confirm — Cancel/Escape/overlay funnel through bits-ui's `onOpenChange` into `oncancel()`, while Delete/Share-anyway are plain buttons that never touch that path, so the host's `open`/visibility remains the sole source of truth (Codex round-5 HIGH, symmetric across both components).
- Both components share one awaitable-`onconfirm` + local `pending` `$state` shape: Delete/Cancel (or Share anyway/Cancel) disable while in flight, and the dialog additionally suppresses Escape/overlay dismissal during that window — no double-fire (T-19-32).
- Both expose an optional `authFailure`/`onreauth` pair rendering the exact same inline re-auth alert the write forms will use (`write failed — session expired. re-authenticate to continue.`), ready for Plan 06's WriteSurfaces host to retain the target on a terminal auth failure.
- `MemoryRow.svelte`'s root changed from a `<button>` to a non-button `<div>` shell with an inner selection `<button>` and a sibling hover-revealed `icon-sm` ghost kebab `DropdownMenu` — resolving the nested-interactive-DOM defect (Codex+grok HIGH, T-19-34) as a structural fact, not a stopPropagation patch, since the trigger/menu are no longer inside the selection button at all.
- `MemoryRow`/`MemoryDetail` gate each menu item on its OWN callback (`{#if onedit}`/`{#if ondelete}`/`{#if onshare && visibility!=='shared'}`, normalizing `''` as private) and mechanically suppress the entire kebab for `category === 'rule'` regardless of which callbacks are supplied — verified against the discovery-route shape (`ondelete`+`onshare`, no `onedit`) that Plan 06 will pass.
- `MemoryDetail`'s three new actions collapse into a single kebab (per UI-SPEC's explicit overflow-note alternative) rather than crowding the 360px header with 4 full buttons; the existing `copy` button is untouched.

## Task Commits

Each task was committed atomically:

1. **Task 1: DeleteConfirmDialog + ShareWarningInline** - `0396d873` (feat)
2. **Task 2: MemoryRow hover dropdown-menu + MemoryList passthrough** - `1618f8fb` (feat)
3. **Task 3: MemoryDetail action buttons** - `5b46b3fa` (feat)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `ui/src/lib/components/DeleteConfirmDialog.svelte` - host-authoritative destructive confirm dialog
- `ui/src/lib/components/DeleteConfirmDialog.browser.test.ts` - 8 tests covering copy, host-authoritative closure, pending, authFailure
- `ui/src/lib/components/ShareWarningInline.svelte` - host-controlled inline share-warning banner
- `ui/src/lib/components/ShareWarningInline.browser.test.ts` - 6 tests covering copy/alert, confirm/cancel, pending, authFailure
- `ui/src/lib/components/MemoryRow.svelte` - non-button shell + sibling kebab DropdownMenu (Edit/Delete/Share)
- `ui/src/lib/components/MemoryRow.browser.test.ts` - 5 new tests (root/kebab structure, gating, rule/shared suppression)
- `ui/src/lib/components/MemoryList.svelte` - `onedit`/`ondelete`/`onshare` threaded to `MemoryRow` like `onselect`
- `ui/src/lib/components/MemoryDetail.svelte` - header kebab DropdownMenu next to the existing copy button
- `ui/src/lib/components/MemoryDetail.browser.test.ts` - 5 new tests mirroring the row's gating/suppression cases

## Decisions Made

- Cancel in `DeleteConfirmDialog` is wrapped in bits-ui's `Dialog.Close` (via the `{#snippet child({ props })}` asChild pattern) rather than a plain `onclick` — this makes Cancel/Escape/overlay all resolve through the exact same `onOpenChange(false) → oncancel()` path, while Delete stays a bare `<Button>` that never touches bits-ui's internal open-state setter. That's the structural mechanism behind "never self-closes on confirm," not a convention that could be violated by a future edit.
- `MemoryDetail`'s three new actions were collapsed into one `icon-sm` ghost kebab (mirroring `MemoryRow`) instead of three separate `outline`/`sm` buttons — UI-SPEC's Component Specs section explicitly sanctions this as the fallback for the 360px pane's overflow risk (copy + 3 more full buttons), and it lets the item-gating/rule-guard/share-visibility logic be identical code shape to the row's, reducing drift risk ahead of Plan 06's wiring.
- In the `{#snippet child({ props })}` asChild pattern, `{...props}` must be spread BEFORE an explicit prop override (e.g. `disabled={pending}`) — spreading it after silently lets the primitive's own (usually-`undefined`) value win. Caught by the pending-state browser test failing before the fix; corrected in the same task commit.

## Deviations from Plan

None — plan executed exactly as written, including the round-5 host-authoritative closure requirement, the corrected share-disclosure copy, per-item callback gating, and the mechanical rule fence in both surfaces.

## Issues Encountered

- A stale Vite dependency-optimization cache (`node_modules/.vite`) produced a transient `Cannot read properties of undefined (reading 'call')` crash the first time the newly-imported `@lucide/svelte/icons/ellipsis-vertical` icon rendered inside the `DropdownMenu.Trigger`'s floating-layer plumbing. Clearing `node_modules/.vite` resolved it; not a code defect, and not touched by this plan's file changes (out of scope per the deviation-rules scope boundary — no fix committed, no source changed).
- Two MemoryRow tests initially reused a single rendered instance across multiple dropdown open/close cycles via `rerender`, which left a lingering portal overlay intercepting subsequent clicks (Playwright "element intercepts pointer events" retries). Rewrote as independent `render()` calls per scenario instead of `rerender`-and-reopen — cleaner isolation, no flakiness. Test-only change, no source impact.

## User Setup Required

None — no external service configuration required. All shadcn primitives (`dialog`, `dropdown-menu`) were already vendored in `ui/src/lib/components/ui/`.

## Next Phase Readiness

- `DeleteConfirmDialog`/`ShareWarningInline`'s awaitable-`onconfirm`/`pending`/`authFailure`/`onreauth` contract is ready for Plan 06's WriteSurfaces host to wire against the real delete/set-visibility mutations, including the retain-on-terminal-auth-failure behavior this plan's tests already prove structurally possible (host owns `open`/visibility, component never self-closes).
- `MemoryRow`/`MemoryList`/`MemoryDetail`'s `onedit(id)`/`ondelete(id)`/`onshare(memory)` callback shape and per-item gating are ready for Plan 06 to pass real handlers from each route; the discovery route's `ondelete`+`onshare`-only (no `onedit`, D-04) shape is already verified to never render Edit or invoke `undefined`.
- No blockers.

---
*Phase: 19-console-write-ux*
*Completed: 2026-07-15*

## Self-Check: PASSED

All 9 created/modified source files plus this SUMMARY.md verified present on disk. All 3 task commits (0396d873, 1618f8fb, 5b46b3fa) verified present in `git log --oneline --all`.
