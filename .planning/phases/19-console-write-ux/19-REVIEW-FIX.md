---
phase: 19-console-write-ux
fixed_at: 2026-07-15T00:00:00Z
review_path: .planning/phases/19-console-write-ux/19-REVIEW.md
iteration: 2
findings_in_scope: 1
fixed: 1
skipped: 0
status: all_fixed
---

# Phase 19: Code Review Fix Report — Iteration 2

**Fixed at:** 2026-07-15T00:00:00Z
**Source review:** .planning/phases/19-console-write-ux/19-REVIEW.md
**Iteration:** 2

**Summary (iteration 2):**
- Findings in scope: 1
- Fixed: 1
- Skipped: 0

## Fixed Issues (Iteration 2)

### WR-04: WR-02 fix is incomplete — the retained `getMemory` invalidation still refetches the tombstone before the async selection-clear takes effect

**Files modified:** `ui/src/lib/mutations/memory.ts`, `ui/src/lib/mutations/discovery.ts`, `ui/src/routes/observe/observe.browser.test.ts`, `ui/src/routes/observe/DeleteBannerHarness.svelte`
**Commit:** b0abc119
**Applied fix:** Applied exactly the WR-04 remediation. Dropped the
`['getMemory', vars.id]` invalidation from the delete mutation's `onSettled` in
both `useDeleteMemory` (`memory.ts`) and `useDeleteDiscovery` (`discovery.ts`),
keeping the `listMemories`/`searchMemories`/`searchDiscoveries`/`listScopes`
invalidations intact (these are the invalidations the WR-02 constraint refers
to). The record is deleted — a `getMemory(deletedId)` refetch can only resolve
`NotFound`, which was the sole trigger for the spurious top-level error banner;
the WR-02 `ondeleted`→`clearSelection` relay (already wired into all three route
hosts) empties the detail pane, so no reconciliation of the detail query is
needed on delete. Added a route-level browser test in `observe.browser.test.ts`
that selects a record, deletes it, and — because the mocked async `goto` leaves
`?sel` pointing at the deleted id (the detail observer stays enabled, replicating
the real race) — asserts (a) `getMemory` is never re-called for the deleted id
(no tombstone refetch) and (b) no `role="alert"` banner appears. The test drives
a new test-only harness `DeleteBannerHarness.svelte` that reproduces the layout's
`QueryCache.onError → (auth ? redirect : reportError)` wiring and renders the
`errorBanner`-driven alert, so a regression that re-adds the `getMemory`
invalidation would refetch → `NotFound` → banner → test failure. Verified with
`cd ui && pnpm check` (0 errors; 16 pre-existing unrelated `state_referenced_locally`
warnings) and `pnpm exec vitest run` on the touched suites: observe 5/5,
discovery route 31/31 (incl. mutations) pass.

## Skipped Issues (Iteration 2)

None.

---

## Iteration 1 (prior run — preserved for the record)

**Iteration-1 summary:** findings in scope 4, fixed 3 (WR-01 03d5b794, WR-02
e7350e94, IN-01 6f886e57), skipped 1 (WR-03 accepted/wontfix). Note: WR-04 above
is the iteration-2 completion of the incomplete WR-02 fix.

### WR-01: `applyUpdateOptimistic` leaves a private→shared record on visibility-filtered list pages

**Files modified:** `ui/src/lib/mutations/memory.ts`, `ui/src/lib/mutations/memory.test.ts`
**Commit:** 03d5b794
**Applied fix:** Made `applyUpdateOptimistic` visibility-filter-aware, mirroring
the sibling `applySetVisibilityOptimistic`. Reused the existing `ctx`
(`isListPage`/`visibilityFilter`) already passed by `applyToMemoryCaches` — no
signature change. On a private→shared edit, the record is now DROPPED (returns
`null`, decrementing `total`) from any `listMemories` page whose `key[3]`
visibility filter no longer matches the record's new visibility, instead of
being patched in place on a page it no longer belongs to. Added a test asserting
the filtered-membership drop (unfiltered page keeps + patches the record;
`private`-filtered page drops it and decrements total; `getMemory` still
reflects the new visibility; rollback restores). `pnpm check` clean (0 errors);
`memory.test.ts` 19/19 pass.

### WR-02: Deleting the selected record raises a spurious global error banner via the `getMemory` invalidation

**Files modified:** `ui/src/lib/components/WriteSurfaces.svelte`, `ui/src/lib/components/WriteSurfaces.browser.test.ts`, `ui/src/routes/observe/+page.svelte`, `ui/src/routes/search/+page.svelte`, `ui/src/routes/discovery/+page.svelte`
**Commit:** e7350e94
**Applied fix:** Applied the route-level remediation the review specifies (clear
the selection so the detail query disables and never refetches a tombstone) —
NOT dropping the invalidation or swapping to `removeQueries`. Added an
`ondeleted?: (id: string) => void` relay prop to `WriteSurfaces`, fired from
`confirmDelete`'s `onSuccess` with the deleted id (never on the terminal-auth
error path). Wired it in all three route hosts: `observe/+page.svelte` clears
`selectedId` via `navigate({ selectedId: '' })`; `search/+page.svelte` and
`discovery/+page.svelte` each gained a `clearSelection()` helper that rebuilds
`URLSearchParams` without `sel` and `goto`s, invoked when the deleted id equals
the current `sel`. Added two WriteSurfaces browser tests: `ondeleted` fires with
the deleted id on success, and does NOT fire on terminal-auth failure (retention
preserved). `pnpm check` clean; WriteSurfaces 17/17 and the three route suites
10/10 pass. **NOTE:** iteration-2 review found this fix incomplete — the delete
`onSettled` still invalidated `getMemory` and fired before the async `goto`
disabled the detail query. Completed in iteration 2 as WR-04.

### IN-01: `dirtyPaths` / `resumeDirtyPaths` is persisted and threaded but never consumed on restore

**Files modified:** `ui/src/lib/resume.ts`, `ui/src/lib/resume.test.ts`, `ui/src/lib/components/MemoryFormSheet.svelte`, `ui/src/lib/components/MemoryFormSheet.browser.test.ts`, `ui/src/lib/components/WriteSurfaces.svelte`, `ui/src/routes/observe/observe.browser.test.ts`
**Commit:** 6f886e57
**Applied fix:** Removed the dead data flow (review's preferred option: removal,
since edit-mode `values` is already exactly the dirty subset). Dropped
`dirtyPaths?` from the `ResumeEnvelope` interface and its `isValidShape`
validation branch in `resume.ts`; removed `dirtyPaths: Object.keys(dirty)` from
`MemoryFormSheet.handleReauthenticate`'s edit-mode `persistResume`; removed the
`resumeDirtyPaths` prop (destructured-and-ignored `_resumeDirtyPaths`) from
`MemoryFormSheet`; removed `formResumeDirtyPaths` state, its four
assignments, the `env.dirtyPaths` reads, and the `resumeDirtyPaths` prop
passthrough from `WriteSurfaces`. Updated stale comments and the now-obsolete
tests (removed the `non-string dirtyPaths entries` validation case in
`resume.test.ts`, dropped `dirtyPaths` from the observe test's seeded envelope,
renamed the MemoryFormSheet prop-restore test). `pnpm check` confirms no consumer
depends on it (0 errors); resume/MemoryFormSheet/WriteSurfaces/observe suites
52/52 pass.

### WR-03 (IN): Inline delete/share re-auth performs a bare `redirectToLogin()` with no navigation resume — SKIPPED (accepted/wontfix)

**File:** `ui/src/lib/components/WriteSurfaces.svelte:188-190,222-224`
**Reason:** skipped: accepted/wontfix per the review's own classification and the
round-5 scope decision. A strictly navigation-only envelope carrying ONLY a
`returnPath` cannot be expressed within the existing action-carrying
`ResumeEnvelope`/`isValidShape`/`reopenFromResume` machinery without either a
scope-expanding new envelope variant or reusing placeholder action fields (which
reintroduces the auto-replay path round-5 deliberately rejected). Left as
accepted/wontfix; not re-flagged in the iteration-2 review. No code changed.
**Original issue:** `handleDeleteReauth` / `handleShareReauth` call
`redirectToLogin()` without persisting any resume state, so after OIDC the
operator lands on `/ui/` home rather than the originating filtered route.

---

_Fixed: 2026-07-15T00:00:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 2_
