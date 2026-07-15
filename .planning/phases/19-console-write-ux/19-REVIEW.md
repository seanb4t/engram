---
phase: 19-console-write-ux
reviewed: 2026-07-15T00:00:00Z
depth: deep
files_reviewed: 42
files_reviewed_list:
  - .github/workflows/ci.yaml
  - ui/src/app.css
  - ui/src/app.css.test.ts
  - ui/src/lib/client.test.ts
  - ui/src/lib/client.ts
  - ui/src/lib/components/DeleteConfirmDialog.browser.test.ts
  - ui/src/lib/components/DeleteConfirmDialog.svelte
  - ui/src/lib/components/DiscoveryFormSheet.browser.test.ts
  - ui/src/lib/components/DiscoveryFormSheet.svelte
  - ui/src/lib/components/MemoryDetail.browser.test.ts
  - ui/src/lib/components/MemoryDetail.svelte
  - ui/src/lib/components/MemoryFormSheet.browser.test.ts
  - ui/src/lib/components/MemoryFormSheet.svelte
  - ui/src/lib/components/MemoryList.svelte
  - ui/src/lib/components/MemoryRow.browser.test.ts
  - ui/src/lib/components/MemoryRow.svelte
  - ui/src/lib/components/ShareWarningInline.browser.test.ts
  - ui/src/lib/components/ShareWarningInline.svelte
  - ui/src/lib/components/ui/button/button.svelte
  - ui/src/lib/components/ui/button/button.test.ts
  - ui/src/lib/components/WriteSurfaces.browser.test.ts
  - ui/src/lib/components/WriteSurfaces.svelte
  - ui/src/lib/interceptors/csrf.test.ts
  - ui/src/lib/interceptors/csrf.ts
  - ui/src/lib/interceptors/retryOnce.test.ts
  - ui/src/lib/interceptors/retryOnce.ts
  - ui/src/lib/mutations/discovery.test.ts
  - ui/src/lib/mutations/discovery.ts
  - ui/src/lib/mutations/memory.test.ts
  - ui/src/lib/mutations/memory.ts
  - ui/src/lib/resume.test.ts
  - ui/src/lib/resume.ts
  - ui/src/routes/+page.svelte
  - ui/src/routes/discovery/+page.svelte
  - ui/src/routes/discovery/discovery.browser.test.ts
  - ui/src/routes/observe/+page.svelte
  - ui/src/routes/observe/observe.browser.test.ts
  - ui/src/routes/page.browser.test.ts
  - ui/src/routes/search/+page.svelte
  - ui/src/routes/search/search.browser.test.ts
findings:
  critical: 0
  warning: 2
  info: 2
  total: 4
status: issues_found
---

# Phase 19: Code Review Report

**Reviewed:** 2026-07-15T00:00:00Z
**Depth:** deep
**Files Reviewed:** 42
**Status:** issues_found

## Summary

Deep pass over the console write-UX slice: the interceptor stack
(`retryOnce` → `attachCsrf`), the two write clients, the memory/discovery
mutation hooks (optimistic apply/rollback across filtered query caches), the
create-as-shared composites, the form sheets, the `WriteSurfaces` host, the
resume envelope, and the four route hosts.

The core round-1..5 hardened invariants hold as documented and were verified
against the actual call chains:

- Transport composes `[retryOnce, attachCsrf]` with `retryOnce` OUTER; the CSRF
  header is re-read fresh on the single retry. Server encodes the token with
  `base64.RawURLEncoding` (`internal/webauth/csrf.go:62`, no `=` padding), so the
  `split('=')[1]` cookie parse in `csrf.ts` cannot truncate the token.
- The create/schedule-as-shared composites catch a secondary `SetVisibility`
  failure and resolve to `created_private` — they never rethrow into the form's
  whole-create D-09 resubmit tier, so there is no duplicate-create path.
- `DeleteConfirmDialog` is host-authoritative (no self-close on Delete; stays
  open with the re-auth CTA on terminal auth); `ShareWarningInline` is
  host-rendered.
- Resume envelope has a single owner: routes peek→goto, forms persist + ack via
  `onresumeapplied`, the host relays consume; no two-owner deletion race.
- Edit-visibility is read-only for already-shared records; `shared` enters the
  update mask only as `true` on a currently-private record, never `false`.
- Root-page open-redirect guard (`isAllowedDestination`) rejects
  protocol-relative and off-route `returnPath`s before `goto`.

Two real defects surfaced that only the cross-file pass reveals: an
optimistic-cache asymmetry between the update and set-visibility paths, and a
spurious global error banner produced by the delete→`getMemory`-invalidation
interaction with the layout's `QueryCache.onError`. Plus one dead data-flow and
the one known/accepted WR-03 nav item.

## Narrative Findings (AI reviewer)

## Warnings

### WR-01: `applyUpdateOptimistic` leaves a private→shared record on visibility-filtered list pages

**File:** `ui/src/lib/mutations/memory.ts:233-241` (compare `applySetVisibilityOptimistic`, `252-261`)
**Issue:** In edit mode a currently-private record can be moved to `shared`
(one-way), so `useUpdateMemory` sends `shared:true` and `applyUpdateOptimistic`
patches `visibility: 'shared'` **in place** on every matching cache entry —
including a `listMemories` page whose `key[3]` visibility filter is `'private'`.
That page now optimistically shows a record whose visibility no longer matches
the page's own filter. `applySetVisibilityOptimistic` was written specifically
to fix exactly this (it drops the record from a filtered page whose filter no
longer matches — round-2 MED), but the update path re-introduces the same class
of bug for the private→shared transition. It is transient (the `onSettled`
`invalidateQueries(['listMemories'])` refetch corrects it), but it is a visible
optimistic inconsistency the sibling path already guards against.
**Fix:** Make `applyUpdateOptimistic` visibility-filter-aware for the `shared`
transition, mirroring `applySetVisibilityOptimistic`. `applyToMemoryCaches`
already passes `ctx` with `isListPage`/`visibilityFilter` — no signature change
is required:

```ts
export function applyUpdateOptimistic(queryClient: QueryClient, vars: UpdateMemoryVars): void {
  applyToMemoryCaches(queryClient, vars.id, (m, ctx) => {
    const nextVisibility =
      vars.shared !== undefined ? (vars.shared ? 'shared' : 'private') : m.visibility;
    // Drop from a filtered list page whose visibility filter no longer matches.
    if (ctx.isListPage && ctx.visibilityFilter && ctx.visibilityFilter !== nextVisibility) return null;
    return {
      ...m,
      ...(vars.content !== undefined ? { content: vars.content } : {}),
      ...(vars.tags !== undefined ? { tags: vars.tags } : {}),
      ...(vars.summary !== undefined ? { summary: vars.summary } : {}),
      ...(vars.shared !== undefined ? { visibility: nextVisibility } : {})
    };
  });
}
```

### WR-02: Deleting the selected record raises a spurious global error banner via the `getMemory` invalidation

**File:** `ui/src/lib/mutations/memory.ts:340-345` and `ui/src/lib/mutations/discovery.ts:194-198`; cross-file with `ui/src/routes/observe/+page.svelte:34-37`, `ui/src/routes/search/+page.svelte:20-23`, `ui/src/routes/discovery/+page.svelte:26-29`, and `ui/src/routes/+layout.svelte:18-24`
**Issue:** On a successful delete, `onSettled` runs
`invalidateQueries({ queryKey: ['getMemory', vars.id] })`. When the deleted
record is the one currently selected in the detail pane (the common
"open detail → Delete" flow), the detail `createQuery` is still enabled
(`enabled: !!sel`, `sel` still points at the deleted id), so the invalidation
triggers a background refetch of `getMemory(deletedId)`, which returns
`NotFound`. That error reaches the layout's `QueryCache.onError`; `mapAuthError`
returns `null` for `NotFound`, so `reportError` fires and a global `error: …`
banner appears — even though `MemoryDetail` already renders a graceful inline
"record not found". Net effect: every delete of the selected record flashes a
false top-level error banner. (`retry: false` guarantees the single refetch
error reaches `onError`.)
**Fix (route-level, safe):** Clear the selected id when the deleted record is the
selected one, so the detail query disables and never refetches a tombstone. Add
an `ondeleted` relay from `WriteSurfaces.confirmDelete`'s `onSuccess` up to each
route, and in the route clear `sel`. Concretely: in `observe/+page.svelte`, wire
`ondeleted={(id) => { if (id === params.selectedId) navigate({ selectedId: '' }); }}`
(threaded through `WriteSurfaces` alongside `requestDelete`); in
`search/+page.svelte` and `discovery/+page.svelte` clear the `sel` search param
the same way (rebuild `URLSearchParams` without `sel`, then `goto`). Do NOT
simply drop the `getMemory` invalidation or swap it for `removeQueries` — an
active detail observer with `enabled: !!sel` re-creates and refetches the
tombstone regardless, so the fix must disable the query by clearing `sel`.

## Info

### WR-03 (IN): Inline delete/share re-auth performs a bare `redirectToLogin()` with no navigation resume (KNOWN / accepted — round-5 scope decision)

**File:** `ui/src/lib/components/WriteSurfaces.svelte:188-190,222-224`
**Issue:** `handleDeleteReauth` / `handleShareReauth` call `redirectToLogin()`
without persisting any resume state, so after OIDC the operator lands on the
`/ui/` home rather than the originating filtered route. This is the deliberate
round-5 decision: delete/share are id-idempotent, input-free actions the
operator re-initiates, and an action-carrying envelope was explicitly rejected
to avoid auto-replaying a destructive action. Recorded here only for
traceability; classified info, not a defect.
**Fix (optional, navigation-only — never action-replaying):** If route
continuity is later desired, persist a **navigation-only** envelope carrying
ONLY a `returnPath` (no `kind`/`mode`/`recordId`/`action`), consumed purely to
restore the route on the `/ui/` landing so the operator returns to the same
filtered list. It MUST NOT carry or replay the delete/share action itself. If a
safe nav-only envelope cannot be expressed within the existing
`ResumeEnvelope`/`isAllowedDestination` machinery without risking auto-replay,
leave this as accepted/wontfix.

### IN-01: `dirtyPaths` / `resumeDirtyPaths` is persisted and threaded but never consumed on restore

**File:** `ui/src/lib/resume.ts:34` (`dirtyPaths`), `ui/src/lib/components/WriteSurfaces.svelte:55,141,145,236`, `ui/src/lib/components/MemoryFormSheet.svelte:35,43` (`_resumeDirtyPaths`)
**Issue:** `MemoryFormSheet.handleReauthenticate` persists
`dirtyPaths: Object.keys(dirty)` and the host threads `resumeDirtyPaths` into
the form, but the restore `$effect` applies **all** of `resumeValues`
unconditionally and never reads `dirtyPaths` (it is destructured as
`_resumeDirtyPaths` and ignored). For edit mode `values` is already exactly the
dirty subset, so `dirtyPaths` is redundant; this is dead data flow that implies
a selective-restore contract the form does not actually honor.
**Fix:** Either drop `dirtyPaths` from the persisted envelope and the
`WriteSurfaces`/`MemoryFormSheet` prop chain, or gate the restore on
`resumeDirtyPaths` (apply only keys present in it). Prefer removal unless
selective-merge semantics are intended.

---

_Reviewed: 2026-07-15T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
