---
phase: 04-curation-surfaces
plan: 01
subsystem: ui
tags: [svelte, tanstack-query, connect-rpc, archive, curation, tdd]

# Dependency graph
requires:
  - phase: 03-connect-write-lane
    provides: engramWrite CSRF-gated write client, ArchiveMemory/RestoreMemory RPCs
provides:
  - Task 1 (tracer): the full /search detail-pane archive path -- Archive button
    -> CurationSurfaces.openArchive -> ArchiveConfirmDialog -> engramWrite.archiveMemory
    -> in-place cache patch, proven end to end and verified
  - Task 2 (checkpoint:decision, resolved): not-owned presentation is
    option-a (server-answer only) -- no client-side authz inference; plan
    04-06 reads this decision
  - Task 3 (full contract): restore mode, per-id outcome rendering
    (ALREADY_ARCHIVED/NOT_ARCHIVED as information, NOT_FOUND with the
    one-rejection copy and the option-a not-owned variant), rejected-envelope
    and re-auth status blocks, both undos (result-body + toast), the D-10
    row-flash set
  - curation.ts mutation hooks, ArchiveConfirmDialog.svelte, CurationSurfaces.svelte
    -- the host architecture every later curation plan in this phase builds on
  - DetailPane callback-gated action buttons (D-05), replacing the Phase 2
    disabled curation stubs
affects: [04-02, 04-03, 04-04, 04-05, 04-06]

# Actuals (#2632) -- whole plan (Tasks 1-3; Task 2 was a decision, no code).
actuals:
  tokens: 16566
  tasks: 3
  commits: 4
  plan_head_before: 5b896d536dfd0a96201cc95c338ce3d1a42b167c
  plan_head_after: 388e1ab022f566286e759099966441cd0ed0a184

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
    - "ArchiveConfirmDialog's mode/pending/outcome are bindable (not just
      open): the D-09 result-body 'Undo — restore N' runs the inverse
      mutation THROUGH THE SAME DIALOG by having the host flip the bound
      mode and push a new outcome, without a second confirm click"
    - "flash.svelte.ts is a single shared SvelteSet singleton (FLASH_MS,
      flashing, flashRows) -- the same 'one shared reactive module' shape as
      display.svelte.ts's text-size preference"

key-files:
  created:
    - ui/src/lib/mutations/curation.ts
    - ui/src/lib/mutations/curation.test.ts
    - ui/src/lib/components/ArchiveConfirmDialog.svelte
    - ui/src/lib/components/ArchiveConfirmDialog.browser.test.ts
    - ui/src/lib/components/CurationSurfaces.svelte
    - ui/src/lib/components/CurationSurfaces.browser.test.ts
    - ui/src/lib/curation/flash.svelte.ts
  modified:
    - ui/src/lib/mutations/memory.ts
    - ui/src/lib/components/DetailPane.svelte
    - ui/src/lib/components/DetailPane.browser.test.ts
    - ui/src/routes/search/+page.svelte
    - ui/src/routes/search/search.browser.test.ts

key-decisions:
  - "Task 2 (checkpoint:decision) resolved by the user: option-a
    (server-answer only, no client-side owner-claim inference). Verbatim
    user response: \"option-a\". Archive/restore do not pre-block a
    not-owned record before submit; after the call, a NOT_FOUND result whose
    `requested` id matches a record this dialog was opened with that is
    `visibility === 'shared'` renders 'shared by {owner} — archive is
    owner-only'; every other NOT_FOUND renders the generic one-rejection
    copy. Plan 04-06 reads this decision for the supersede dialog's
    equivalent surface."
  - "Task 1 (tracer) executed and verified per plan; the tracer feedback gate
    re-ran Task 1's <verify> (both <automated> blocks) -- both passed, so
    execution proceeded to Task 2 with no additional checkpoint synthesized
    (interactive mode, human_verify_mode=end-of-phase, verify carries only
    <automated> entries)."
  - "This SUMMARY was written by a continuation executor resuming at Task 2
    after the checkpoint halt recorded in the prior partial SUMMARY (commit
    6b9c5125). Task 3 ran under full TDD discipline: curation.test.ts is a
    node-tier regression test over Task 1's already-shipped
    applyArchiveResultsOptimistic/changedIds (passes immediately -- see TDD
    Gate Compliance below); ArchiveConfirmDialog.browser.test.ts and
    CurationSurfaces.browser.test.ts are genuine RED -> GREEN (7/9 and 4/4
    cases failed against the Task-1-only implementation, confirmed before
    any Task 3 implementation code was written)."

requirements-completed: [CUR-02]

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
    description: "Task 2's not-owned presentation decision (D-08): resolved as option-a (server-answer only); NOT_FOUND rendering follows the chosen option"
    verification:
      - kind: manual_procedural
        ref: "User reply 'option-a' recorded in key-decisions above"
        status: pass
    human_judgment: false
  - id: D4
    description: "The full archive/restore confirm contract (D-08/D-09/D-10): restore mode, per-id outcome rendering, rejected-envelope and re-auth status blocks, result-body undo and toast undo, D-10 row-flash set and onchanged"
    verification:
      - kind: unit
        ref: "src/lib/mutations/curation.test.ts (applyArchiveResultsOptimistic, changedIds)"
        status: pass
      - kind: automated_ui
        ref: "src/lib/components/ArchiveConfirmDialog.browser.test.ts (9 cases: mode swap, chip removal, loading, result view, undo button, rejected envelope, re-auth, screenshots)"
        status: pass
      - kind: automated_ui
        ref: "src/lib/components/CurationSurfaces.browser.test.ts (4 cases: toast undo, result-body undo, flash + onchanged, re-auth redirect)"
        status: pass
    human_judgment: false

# Metrics
duration: 28min (this continuation session, Task 2 decision + Task 3 TDD cycle; Task 1's prior session duration was not separately tracked)
completed: 2026-09-27
status: complete
---

# Phase 4 Plan 1: Archive/restore confirm surfaces Summary

**CUR-02 shipped end to end: archive/restore on /search through a CurationSurfaces/ArchiveConfirmDialog host, with restore mode, per-id outcome rendering, a server-answer-only not-owned presentation (Task 2: option-a), rejected/re-auth status blocks, and both the result-body and toast undos (D-09/D-10).**

## Performance

- **Tasks:** 3 of 3 completed (Task 1 tracer, Task 2 decision, Task 3 full contract via TDD)
- **Files modified:** 12 (7 created, 5 modified)
- **Commits:** 4 (`3af488b4` feat, `6b9c5125` docs/halt, `30104f94` test/RED, `388e1ab0` feat/GREEN)

## Accomplishments

- New `ui/src/lib/mutations/curation.ts`: `useArchiveMemory()` / `useRestoreMemory()` mutation hooks built like `useDeleteMemory`, `applyArchiveResultsOptimistic` patching `archivedAt` in place via the shared `applyToMemoryCaches` walker (rows never removed — D-10), `invalidateAfterCuration` invalidating recall surfaces with `refetchType: 'none'` on the list queries so the visible list never jumps.
- New `ArchiveConfirmDialog.svelte`: host-authoritative confirm dialog (same shape as `DeleteConfirmDialog`) with a `mode: 'archive' | 'restore'` prop (now bindable, along with `pending` and `outcome`), removable chips (`×`, E2 empty), a loading status block (`archive_memory · N records…`), a result view rendering `✓ N archived`/`✓ N restored` plus per-id outcome lines (`ALREADY_ARCHIVED`/`NOT_ARCHIVED` as information text, `NOT_FOUND` as the one-rejection copy or, for a `shared` record this dialog was opened with, the not-owned copy), a rejected-envelope status block (field=/hint= pills + raw detail), and a re-auth status block with a `Re-authenticate` button.
- New `CurationSurfaces.svelte`: route-level curation write host mirroring `WriteSurfaces`, exposing `openArchive(ids)` / `openRestore(ids)` that resolve ids to `Memory` records from the query cache (falling back to `fetchQuery`), running the matching mutation, mapping mutation failures to `ArchiveSubmitOutcome` (`reauth` for Unauthenticated/PermissionDenied, `rejected` via `parseConnectError` otherwise), flashing the changed rows and firing `onchanged` immediately on success (not deferred to Done), running the D-09 result-body undo through the same dialog by flipping the bound `mode`/`pending`/`outcome`, and toasting `"N archived · Undo"` (8s) on Done for an archive result.
- New `ui/src/lib/curation/flash.svelte.ts`: `FLASH_MS = 1600`, the shared `flashing` `SvelteSet`, and `flashRows(ids, ms)` — a per-id timer that restarts on re-flash. Plan 04-05 wires `ResultRow` to read `flashing`.
- `DetailPane.svelte`: removed the two Phase-2 disabled Tooltip-wrapped curation buttons and the `CURATION_TOOLTIP` constant; every action button (Edit, Archive, Restore, Share/Make private, Delete) is now callback-gated — a route that omits a callback never renders a dead button, which `/rules` and `/scheduled` (plans 04-03/04-04) rely on.
- `search/+page.svelte`: `CurationSurfaces` mounted inside `.search-toolbar` (stable location outside `RecallSplit`, same reasoning as `WriteSurfaces`); `DetailPane`'s `onarchive`/`onrestore` call `curation?.openArchive([id])` / `openRestore([id])`.
- `memory.ts`: `applyToMemoryCaches` exported for reuse by `curation.ts` (no behavior change).

## Task Commits

Each task was committed atomically:

1. **Task 1: Archive one record from the /search detail pane, end to end** - `3af488b4` (feat)
2. **Task 2: DECISION — not-owned presentation** - resolved by user reply ("option-a"), no code commit (decision-only task); recorded in `6b9c5125`'s halt summary and finalized in this SUMMARY's key-decisions
3. **Task 3: The full archive/restore confirm** - `30104f94` (test, RED) then `388e1ab0` (feat, GREEN)

**Plan metadata:** this commit (docs: complete plan)

_Note: TDD tasks may have multiple commits (test → feat → refactor); Task 3 needed no REFACTOR commit — the GREEN implementation required no follow-up cleanup._

## Files Created/Modified

- `ui/src/lib/mutations/curation.ts` - archive/restore mutation hooks, cache-patch, invalidation
- `ui/src/lib/mutations/curation.test.ts` - node-tier regression coverage for the cache-patch/changedIds pure functions
- `ui/src/lib/components/ArchiveConfirmDialog.svelte` - shared archive/restore confirm dialog, now with restore mode, per-id outcomes, status blocks and both undos
- `ui/src/lib/components/ArchiveConfirmDialog.browser.test.ts` - 9 cases covering mode swap, chip removal, loading, result view, undo, rejected envelope, re-auth, screenshots
- `ui/src/lib/components/CurationSurfaces.svelte` - route-level curation write host, now with error mapping, flash, both undos and the toast
- `ui/src/lib/components/CurationSurfaces.browser.test.ts` - 4 cases covering toast undo, result-body undo, flash + onchanged, re-auth redirect
- `ui/src/lib/curation/flash.svelte.ts` - the D-10 row-flash set
- `ui/src/lib/mutations/memory.ts` - exported `applyToMemoryCaches`
- `ui/src/lib/components/DetailPane.svelte` - callback-gated action buttons, curation tooltip removed
- `ui/src/lib/components/DetailPane.browser.test.ts` - rewrote the two Phase-2-pinned tests for the new contract, added two more (no-callback-supplied cases)
- `ui/src/routes/search/+page.svelte` - CurationSurfaces wired in, DetailPane onarchive/onrestore
- `ui/src/routes/search/search.browser.test.ts` - archiveMemory/restoreMemory spies added to the `$lib/client` mock, new CUR-02 tracer describe block

## Decisions Made

- **Task 2 (D-08 not-owned presentation): option-a — server-answer only.** No client-side owner-claim inference; a `NOT_FOUND` result renders the not-owned copy only when it matches a `visibility: 'shared'` record already loaded into this dialog, otherwise the generic one-rejection copy. Rationale (from the plan's option table): zero new client-side authz surface, consistent with the research finding that the console never re-derives authz. Plan 04-06's supersede dialog reads this same decision.
- **ArchiveConfirmDialog's `mode`/`pending`/`outcome` made bindable** (beyond just `open`, which Task 1 already had) so `CurationSurfaces.onundo` can drive the D-09 result-body undo "through the same dialog" — flip the bound mode, run the inverse mutation, push the new outcome — without a second confirm click or a parallel state-push prop.
- **`curation.test.ts`'s assertions pass without any RED phase** because they test Task 1's already-shipped `applyArchiveResultsOptimistic`/`changedIds` pure functions as regression coverage for Task 3's contract — see TDD Gate Compliance below. The two browser test files (genuinely new dialog/host behavior) followed real RED → GREEN.

## Deviations from Plan

None — Task 1 executed exactly as written (see its own halt-time notes); Task 3 implements exactly D-08/D-09/D-10 as specified, including the option-a not-owned rendering Task 2 selected.

## TDD Gate Compliance

Task 3 (`tdd="true"`):

| Gate | Commit | Notes |
|------|--------|-------|
| RED | `test(04-01): add failing tests for archive/restore confirm full contract` (`30104f94`) | `ArchiveConfirmDialog.browser.test.ts`: 7 of 9 cases failed against the Task-1-only implementation (`$bindable` compile error initially, then genuine assertion failures once fixed — see below). `CurationSurfaces.browser.test.ts`: all 4 cases failed (missing toast/undo/flash wiring, error mapping). `curation.test.ts`: all cases passed immediately — it verifies Task 1's already-shipped `applyArchiveResultsOptimistic`/`changedIds`, not new Task 3 behavior; this is the tdd.md-sanctioned "feature may already exist" case, not a gate violation. |
| GREEN | `feat(04-01): archive/restore confirm with per-id outcomes and double undo (CUR-02, D-08, D-09, D-10)` (`388e1ab0`) | All 9 ArchiveConfirmDialog cases, all 4 CurationSurfaces cases, and Task 1's DetailPane/search suites (54 tests total in the combined `<verify>` run) pass. `pnpm --dir ui build` succeeds. |
| REFACTOR | — | Not needed; no follow-up cleanup required after GREEN. |

One RED-phase iteration was needed before the first genuine failing run: `$bindable()` cannot be declared as a standalone `let` outside `$props()` destructuring (a Svelte compile-time rule, not a runtime assertion) — moving `pending`/`outcome` into the props destructure fixed it, then the suite ran and showed real RED (7/9 failing) before any GREEN implementation.

## Issues Encountered

None during Task 2/3. Task 1's browser-test-flake fix (forcing `RecallSplit`'s wide layout via `screen.container.style.width`) is documented in the original halt-time summary and unchanged here.

**Test-authoring fix in Task 3 (not a deviation from PLAN.md):** one `ArchiveConfirmDialog.browser.test.ts` assertion initially checked for the literal substring `'m1'` in the rejected-envelope test to prove chips stay in place — that text is never rendered (chips show `short_id`, summary and a category dot, never the raw id). Fixed to assert the chip's `short_id` text instead; this was a weak test assertion, not an implementation gap (T-04-02's id-never-rendered guarantee is exactly what the fix now correctly proves).

## Known Stubs

- `CurationSurfaces.onreauth`'s `ids` parameter is accepted but unused — it calls `redirectToLogin()` only. Plan 04-06 adds the v2 resume-envelope persist (`persistResume(...)`) before this redirect, which is where `ids` will be consumed. This is explicitly out of this plan's scope per the interfaces block ("this plan's re-auth button calls `redirectToLogin()` only") and does not block CUR-02's own goal.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Both the tracer architecture (Task 1) and the full D-08/D-09/D-10 contract (Task 3) are in place: `curation.ts`'s mutation hooks, `applyArchiveResultsOptimistic`'s cache-patch pattern, `ArchiveConfirmDialog`'s bindable-mode/pending/outcome shape, `CurationSurfaces`'s error-mapping and undo/toast wiring, and `flash.svelte.ts`'s shared row-flash set are all available for plans 04-02 through 04-06 to build on directly.
- Task 2's resolved decision (option-a, server-answer only) is the not-owned-presentation contract plan 04-06's supersede dialog should follow for consistency.
- Plan 04-05 is expected to wire `ResultRow` to read `flashing` from `flash.svelte.ts` (not yet consumed by any renderer in this plan).

---
*Phase: 04-curation-surfaces*
*Completed: 2026-09-27*

## Self-Check: PASSED

- All 7 key-files (created) verified present on disk with `[ -f ]`.
- All 4 commits (`3af488b4`, `6b9c5125`, `30104f94`, `388e1ab0`) verified present in `git log --oneline --all`.
- Re-ran Task 3's plan-level `<verify>` commands: node project (`curation.test.ts`, 1 file/4 tests passed) and browser project (4 files/54 tests passed) both green; `pnpm --dir ui build` exits 0.
- Re-ran all 5 acceptance-criteria `rg` checks from Task 3 (Undo — restore, duration: 8000, NOT_FOUND_NOTE literal count = 1, page.screenshot count >= 3, FLASH_MS = 1600) — all matched their required counts.
