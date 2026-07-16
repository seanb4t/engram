---
phase: 19-console-write-ux
reviewed: 2026-07-15T00:00:00Z
depth: deep
iteration: 3
files_reviewed: 19
files_reviewed_list:
  - ui/src/app.css
  - ui/src/lib/client.ts
  - ui/src/lib/components/DeleteConfirmDialog.svelte
  - ui/src/lib/components/DiscoveryFormSheet.svelte
  - ui/src/lib/components/MemoryDetail.svelte
  - ui/src/lib/components/MemoryFormSheet.svelte
  - ui/src/lib/components/MemoryList.svelte
  - ui/src/lib/components/MemoryRow.svelte
  - ui/src/lib/components/ShareWarningInline.svelte
  - ui/src/lib/components/WriteSurfaces.svelte
  - ui/src/lib/interceptors/csrf.ts
  - ui/src/lib/interceptors/retryOnce.ts
  - ui/src/lib/mutations/discovery.ts
  - ui/src/lib/mutations/memory.ts
  - ui/src/lib/resume.ts
  - ui/src/routes/+page.svelte
  - ui/src/routes/discovery/+page.svelte
  - ui/src/routes/observe/+page.svelte
  - ui/src/routes/search/+page.svelte
findings:
  critical: 0
  warning: 0
  info: 0
  total: 0
status: clean
---

# Phase 19: Code Review Report — Iteration 3 (final)

**Reviewed:** 2026-07-15T00:00:00Z
**Depth:** deep
**Files Reviewed:** 19
**Status:** clean

## Summary

Final iteration of the fix+review loop. Verified the iteration-2 WR-04 fix
against the actual call chains in both mutation modules, checked for regressions
from dropping the delete-triggered `getMemory` invalidation, and re-verified the
five round-hardened invariants. No actionable defects remain. WR-03 is the known,
accepted round-5 wontfix (noted once below, not counted).

### WR-04 fix — VERIFIED CLEAN (both modules)

- `useDeleteMemory.onSettled` (`memory.ts:348-355`) no longer invalidates
  `['getMemory', id]`. It invalidates only `['listMemories']`,
  `['searchMemories']`, and `['listScopes']` — exactly the set the WR-02
  constraint preserves. The tombstone-refetch trigger identified in iteration 2
  is gone; there is no remaining code path that re-issues `getMemory(deletedId)`
  after a delete.
- `useDeleteDiscovery.onSettled` (`discovery.ts:194-200`) received the identical
  change: invalidates only `['searchDiscoveries']` and `['listScopes']`, drops
  the `['getMemory', id]` invalidation.
- The detail-pane clear is handled entirely by the `ondeleted(id)` relay:
  `confirmDelete`'s `onSuccess` fires `ondeleted?.(id)` (`WriteSurfaces.svelte:167`),
  and each host clears its selection only when the deleted id matches the current
  selection — observe `navigate({ selectedId: '' })` (`observe/+page.svelte:71`),
  search/discovery `clearSelection()` (`search/+page.svelte:50`,
  `discovery/+page.svelte:56`). Dropping `?sel`/`selectedId` disables the detail
  `getMemory` query (`enabled: !!sel`), so no observer remains to refetch.

### No new regression from dropping the invalidation

Traced every consumer of the delete `onSettled`. The removed invalidation was
keyed to `['getMemory', vars.id]` — the *deleted* record's id only. It never
touched a different record's detail query, so a still-open detail pane for a
DIFFERENT, non-deleted record is unaffected: its `['getMemory', otherId]` entry
was never in the delete invalidation's key scope, and the host's `ondeleted`
guard (`id === selectedId`) leaves a non-matching selection untouched. The
still-live `getMemory` invalidations on `useUpdateMemory`
(`memory.ts:322`), `useSetMemoryVisibility` (`memory.ts:390`), and
`useSetDiscoveryVisibility` (`discovery.ts:235`) continue to reconcile records
that still exist — none depended on the delete path's invalidation. The
deleted record's `['getMemory', id]` cache entry is left stale-but-unread
(`applyDeleteOptimistic` returns `old` for that key, `memory.ts:226-230`); with
the observer disabled and no invalidation, it is inert and cannot surface a
tombstone. No data-loss or stale-read surface.

### Five invariants re-verified (all hold)

1. **Interceptor order `[retryOnce, attachCsrf]`, retryOnce OUTER** —
   `client.ts:23`. attachCsrf re-reads `document.cookie` fresh on each `next(req)`
   (`csrf.ts:14-20`), so the retry picks up a re-sealed cookie. Unchanged.
2. **No-duplicate-create composite → `created_private`** — `shareIfRequested`
   catches every secondary `SetVisibility` failure and returns `created_private`,
   never rethrows (`memory.ts:130-137`); `createDiscoveryComposite` mirrors it
   (`discovery.ts:80-87`). The form treats all three statuses as success and
   never routes `created_private` into the resubmit tier
   (`MemoryFormSheet.svelte:177-182`). Unchanged.
3. **Host-authoritative DeleteConfirmDialog** — the dialog never self-closes on
   confirm; `open` is host-driven and cleared only on delete success
   (`DeleteConfirmDialog.svelte:5-10,56-58`; `WriteSurfaces.svelte:160-179`). A
   terminal auth failure RETAINS the target and shows the re-auth CTA. Unchanged.
4. **Single-owner resume envelope** — forms only `persistResume`
   (`MemoryFormSheet.svelte:220-240`); the route is the sole `peek`/`consume`
   owner (`observe/+page.svelte:49-52`, `search`/`discovery` mirror), applying
   values as props and acking via `onresumeapplied`. No second consumer.
   Unchanged.
5. **Edit-visibility never emits `shared:false`** — `shared` enters the dirty
   mask only as `true` on a currently-private record
   (`MemoryFormSheet.svelte:96`); an already-shared record is read-only
   (`isEditSharedReadOnly`, line 77), so `useUpdateMemory` can never emit an
   unshare. Unchanged.

All reviewed files meet quality standards. No actionable issues found.

## Accepted / wontfix (not actionable)

**WR-03** (`WriteSurfaces.svelte:188-190,222-224`): inline delete/share re-auth
performs a bare `redirectToLogin()` and lands on `/ui/` rather than the
originating route. Deliberate round-5 scope decision — a nav-only envelope
cannot be expressed in the action-carrying `ResumeEnvelope` machinery without
reintroducing the rejected auto-replay risk. Recorded for traceability only; not
counted as a finding.

---

_Reviewed: 2026-07-15T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep — iteration 3 (final)_
