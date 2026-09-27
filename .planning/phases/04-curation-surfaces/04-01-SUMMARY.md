---
phase: 04-curation-surfaces
plan: 01
subsystem: ui
tags: [svelte, tanstack-query, connect-rpc, archive, curation]

# Dependency graph
requires:
  - phase: 03-connect-write-lane
    provides: engramWrite CSRF-gated write client, ArchiveMemory/RestoreMemory RPCs
provides:
  - Task 1 (tracer): the full /search detail-pane archive path -- Archive button
    -> CurationSurfaces.openArchive -> ArchiveConfirmDialog -> engramWrite.archiveMemory
    -> in-place cache patch, proven end to end and verified
  - curation.ts mutation hooks, ArchiveConfirmDialog.svelte, CurationSurfaces.svelte
    -- the host architecture every later curation plan in this phase builds on
  - DetailPane callback-gated action buttons (D-05), replacing the Phase 2
    disabled curation stubs
affects: [04-02, 04-03, 04-04, 04-05, 04-06]

# Actuals (#2632) -- Task 1 only; Task 3's cost is not yet incurred.
actuals:
  tokens: 8284
  tasks: 1
  commits: 1
  plan_head_before: 5b896d536dfd0a96201cc95c338ce3d1a42b167c
  plan_head_after: 3af488b4fa7dbbf9225f61fc56ff35e232ef6017

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "CurationSurfaces mirrors WriteSurfaces: a route-level write host owning
      dialog open-state, exposing bind:this methods (openArchive/openRestore),
      host-authoritative dialog closure"
    - "applyArchiveResultsOptimistic patches archivedAt in place via the shared
      applyToMemoryCaches walker (never removes rows -- D-10)"
    - "invalidateAfterCuration uses refetchType: 'none' on recall-list queries
      so a background refetch never jumps the visible list"

key-files:
  created:
    - ui/src/lib/mutations/curation.ts
    - ui/src/lib/components/ArchiveConfirmDialog.svelte
    - ui/src/lib/components/CurationSurfaces.svelte
  modified:
    - ui/src/lib/mutations/memory.ts
    - ui/src/lib/components/DetailPane.svelte
    - ui/src/lib/components/DetailPane.browser.test.ts
    - ui/src/routes/search/+page.svelte
    - ui/src/routes/search/search.browser.test.ts

key-decisions:
  - "Task 1 only (tracer). Task 2 is a checkpoint:decision (gate=\"blocking\")
    that is NOT auto-approvable in this run (interactive mode,
    workflow._auto_chain_active=false, workflow.auto_advance unset/false).
    Execution halted at Task 2 per the executor's checkpoint protocol --
    awaiting Sean's choice of option-a / option-b / option-c for the
    not-owned-record presentation (D-08). Task 3 (the full confirm contract)
    has not started."

requirements-completed: []  # CUR-02 partially implemented (Task 1's pane path only); not marked complete until Task 3 lands the full D-08/D-09/D-10 contract this plan's own frontmatter requires.

coverage:
  - id: D1
    description: "Archive one record from the /search detail pane end to end (CUR-02 tracer): Archive button -> ArchiveConfirmDialog -> engramWrite.archiveMemory -> row patched in place with the archived state word"
    requirement: "CUR-02"
    verification:
      - kind: automated_ui
        ref: "src/routes/search/search.browser.test.ts#search route — archive from the detail pane (CUR-02 tracer) > archives the open record via CurationSurfaces/ArchiveConfirmDialog and patches the row in place"
        status: pass
    human_judgment: false
  - id: D2
    description: "DetailPane's every action button is callback-gated (D-05): Edit/Archive/Restore/Share/Delete each render only when their callback prop is supplied, and the Phase 2 disabled curation tooltip is gone"
    verification:
      - kind: automated_ui
        ref: "src/lib/components/DetailPane.browser.test.ts#DetailPane — inline actions (D-15, D-16)"
        status: pass
    human_judgment: false
  - id: D3
    description: "The not-owned presentation for archive/restore (D-08) -- awaits Task 2's checkpoint decision"
    verification: []
    human_judgment: true
    rationale: "Blocked on Task 2's checkpoint:decision; not yet implemented."

# Metrics
duration: partial (halted at Task 2)
completed: 2026-09-27
status: halted
---

# Phase 4 Plan 1: Archive/restore confirm surfaces (Task 1 of 3 — halted at checkpoint) Summary

**CUR-02 tracer shipped: archiving a record from the /search detail pane runs end to end through a new CurationSurfaces/ArchiveConfirmDialog host into engramWrite.archiveMemory, with the row patched in place — Task 2's not-owned-presentation decision is pending.**

## Performance

- **Tasks:** 1 of 3 completed (Task 2 is a `checkpoint:decision` awaiting a human choice; Task 3 not started)
- **Files modified:** 8 (3 created, 5 modified)

## Accomplishments

- New `ui/src/lib/mutations/curation.ts`: `useArchiveMemory()` / `useRestoreMemory()` mutation hooks built like `useDeleteMemory`, `applyArchiveResultsOptimistic` patching `archivedAt` in place via the shared `applyToMemoryCaches` walker (rows never removed — D-10), `invalidateAfterCuration` invalidating recall surfaces with `refetchType: 'none'` on the list queries so the visible list never jumps.
- New `ArchiveConfirmDialog.svelte`: host-authoritative confirm dialog (same shape as `DeleteConfirmDialog`) with a `mode: 'archive' | 'restore'` prop, one chip per record, a result view showing `✓ N archived`/`✓ N restored`, and a `Done` button.
- New `CurationSurfaces.svelte`: route-level curation write host mirroring `WriteSurfaces`, exposing `openArchive(ids)` / `openRestore(ids)` that resolve ids to `Memory` records from the query cache (falling back to `fetchQuery`), then run the matching mutation.
- `DetailPane.svelte`: removed the two Phase-2 disabled Tooltip-wrapped curation buttons and the `CURATION_TOOLTIP` constant; every action button (Edit, Archive, Restore, Share/Make private, Delete) is now callback-gated — a route that omits a callback never renders a dead button, which `/rules` and `/scheduled` (plans 04-03/04-04) will rely on.
- `search/+page.svelte`: `CurationSurfaces` mounted inside `.search-toolbar` (stable location outside `RecallSplit`, same reasoning as `WriteSurfaces`); `DetailPane`'s `onarchive`/`onrestore` call `curation?.openArchive([id])` / `openRestore([id])`.
- `memory.ts`: `applyToMemoryCaches` exported for reuse by `curation.ts` (no behavior change).

## Task Commits

Each task was committed atomically:

1. **Task 1: Archive one record from the /search detail pane, end to end** - `3af488b4` (feat)

Task 2 (checkpoint:decision) and Task 3 have not executed — no commits beyond Task 1.

## Files Created/Modified

- `ui/src/lib/mutations/curation.ts` - archive/restore mutation hooks, cache-patch, invalidation
- `ui/src/lib/components/ArchiveConfirmDialog.svelte` - shared archive/restore confirm dialog
- `ui/src/lib/components/CurationSurfaces.svelte` - route-level curation write host
- `ui/src/lib/mutations/memory.ts` - exported `applyToMemoryCaches`
- `ui/src/lib/components/DetailPane.svelte` - callback-gated action buttons, curation tooltip removed
- `ui/src/lib/components/DetailPane.browser.test.ts` - rewrote the two Phase-2-pinned tests for the new contract, added two more (no-callback-supplied cases)
- `ui/src/routes/search/+page.svelte` - CurationSurfaces wired in, DetailPane onarchive/onrestore
- `ui/src/routes/search/search.browser.test.ts` - archiveMemory/restoreMemory spies added to the `$lib/client` mock, new CUR-02 tracer describe block

## Decisions Made

- Task 1 (tracer) executed and verified per plan; the tracer feedback gate re-ran Task 1's `<verify>` (both `<automated>` blocks: the two-file browser test run and `pnpm --dir ui build`) — both passed, so execution proceeded directly to Task 2 with no additional checkpoint synthesized (interactive mode, `human_verify_mode=end-of-phase`, verify carries only `<automated>` entries).
- Task 2 is a `checkpoint:decision` with `gate="blocking"` (not `blocking-human`) but this run is NOT in auto mode (`workflow._auto_chain_active=false`, `workflow.auto_advance` unset). Per the executor's checkpoint protocol, a non-auto run STOPs at any `checkpoint:*` task regardless of gate value. Execution halted here to await Sean's answer to Task 2's decision (option-a / option-b / option-c for the not-owned-record presentation, D-08).

## Deviations from Plan

None - Task 1 executed exactly as written; the archive dialog's error/re-auth handling and full D-08/D-09/D-10 contract are explicitly Task 3 scope, not a deviation.

## Known Stubs

- `CurationSurfaces.onsubmit` currently only implements the success path (`{ kind: 'ok', results }`); a rejected or auth-expired mutation call propagates as an unhandled rejection rather than mapping to `ArchiveSubmitOutcome`'s `reauth`/`rejected` kinds. This is Task 3's explicit scope (D-08/D-09 error mapping), not silently dropped — documented inline in `CurationSurfaces.svelte` and `curation.ts`.
- `ArchiveConfirmDialog` has no status-block states for loading/rejected/re-auth yet (Task 3 adds them); Task 1's dialog only renders the confirm and result views.

## Issues Encountered

None during Task 1. A browser-test flake was diagnosed and fixed inline (see below) — not a deviation from the plan, a test-authoring fix within Task 1's own new test.

**Test fix (not a deviation from PLAN.md, but worth recording):** the new CUR-02 tracer test in `search.browser.test.ts` initially timed out clicking the pane's Archive button — `RecallSplit`'s narrow-mode absolute overlay layout (triggered because the vitest-browser test container has no explicit width) intercepted the pointer event. Fixed by setting `screen.container.style.width = '1200px'` before interacting, the same convention `RecallSplit.browser.test.ts` already uses to force the wide (non-overlay) layout for reliable clicks.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Task 1's tracer proves the curation-host architecture (mutation hooks, cache patch, dialog, route host, pane wiring) that Task 3 and every later curation plan (04-02 through 04-06) build on directly.
- **Blocked:** Task 2's checkpoint:decision must be answered before Task 3 can proceed. The decision: how the console presents a record the caller does not own (D-08), with three options (A: server-answer only [recommended], B: derive caller identity from a visible private record, C: add a WhoAmI RPC — stops for a replan). See `.planning/phases/04-curation-surfaces/04-01-PLAN.md` Task 2 for full context.
- Once Task 2 is answered, Task 3 implements the full confirm contract (restore mode, per-id outcomes, both undos, flash, the chosen not-owned behavior) via TDD, then this plan's SUMMARY will be regenerated with `status: complete`.

---
*Phase: 04-curation-surfaces*
*Completed: 2026-09-27 (Task 1 of 3; halted at Task 2 checkpoint)*
