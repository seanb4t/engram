---
phase: 19-console-write-ux
plan: 04
subsystem: ui
tags: [tanstack-query, svelte-query, connect-rpc, optimistic-cache, vitest]

# Dependency graph
requires:
  - phase: 19-02-console-write-ux
    provides: engramWrite client on the [retryOnce, attachCsrf] write transport
  - phase: 19-01-console-write-ux
    provides: re-vendored gen client with all 6 write RPCs, Citation, and Visibility
provides:
  - "ui/src/lib/mutations/memory.ts: useCreateMemory/useUpdateMemory/useDeleteMemory/useSetMemoryVisibility/useScheduleMemory createMutation hooks, plus named pure exports (buildStoreMemoryRequest, buildUpdateMemoryRequest, buildScheduleMemoryRequest, normalizeVisibility, createMemoryComposite, scheduleMemoryComposite, snapshotMemoryQueries, restoreMemoryQueries, applyUpdateOptimistic, applyDeleteOptimistic, applySetVisibilityOptimistic, CONSOLE_SOURCE, EngramWriteClient, WriteResult, CacheEntry types)"
  - "ui/src/lib/mutations/discovery.ts: useCreateDiscovery/useDeleteDiscovery/useSetDiscoveryVisibility createMutation hooks, plus named pure exports (buildStoreDiscoveryRequest, createDiscoveryComposite, snapshotDiscoveryQueries, restoreDiscoveryQueries, applyDeleteDiscoveryOptimistic, applySetDiscoveryVisibilityOptimistic) -- no discovery edit/update hook (D-04 fence)"
  - "Create/schedule-as-shared composite STATE MACHINE: WriteResult = {status:'created'|'created_shared'|'created_private', id} -- a secondary SetVisibility auth failure is caught and never rethrown, so the form's whole-create resubmit path (Plan 05) can consume the result directly without any duplicate-create risk"
affects: [19-05-resume-envelope, 19-06]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Composite write RPC as an explicit state machine (mutationFn -> discriminated {status,id} result), never a bare promise -- lets onSuccess be the sole toast site and lets the caller distinguish a full failure (propagate) from a partial one (keep record, no resubmit)"
    - "Pure request-builder and cache-transform functions exported as named symbols alongside the createMutation hooks, so vitest node-tier tests exercise cache logic against a real QueryClient without a Svelte QueryClientProvider"
    - "getQueriesData/setQueryData iterated per-key (not setQueriesData's key-less updater) whenever a transform needs to inspect the query key itself -- required for the listMemoriesKey visibility-filter membership check"

key-files:
  created:
    - ui/src/lib/mutations/memory.ts
    - ui/src/lib/mutations/memory.test.ts
    - ui/src/lib/mutations/discovery.ts
    - ui/src/lib/mutations/discovery.test.ts

key-decisions:
  - "source field on every console-originated write is the literal string 'console' (CONSOLE_SOURCE constant in memory.ts, re-exported for discovery.ts) -- Claude's discretion per the plan, documented in a source comment; distinguishes console writes from MCP-tool writes in the source field"
  - "createMemoryComposite/scheduleMemoryComposite/createDiscoveryComposite take the engramWrite client as an explicit parameter (not a closed-over import) so tests can pass a createRouterTransport-backed fake client and assert exact RPC call counts -- the exported hooks (useCreateMemory etc.) call these composites with the real engramWrite"
  - "Partial-success toast uses toast.warning (svelte-sonner has a warning variant), not toast.error or toast.success -- the record genuinely landed (not a full failure) but sharing did not happen (not a full success)"
  - "applySetVisibilityOptimistic/applyUpdateOptimistic/applyDeleteOptimistic share one applyToMemoryCaches iterator that walks getQueriesData per matching key and calls setQueryData per key (rather than setQueriesData's key-less updater), specifically so the listMemoriesKey visibility filter (key[3]) can be read to decide whether to drop a record from a now-mismatched filtered page"
  - "discovery.ts's createDiscoveryComposite duplicates (does not import) the try/catch SetVisibility shape from memory.ts's shareIfRequested, but reuses memory.ts's EngramWriteClient/WriteResult/CacheEntry types to avoid type drift between the two composites"

patterns-established:
  - "Every mutating hook's onError restores the pre-mutation cache snapshot AND re-throws nothing extra -- the mutation's own error still reaches a caller-supplied .mutate(vars, {onError}) so Plan 05/06's D-09/SC3 inline re-auth can observe a terminal Unauthenticated/PermissionDenied"
  - "onSuccess is the single toast site per mutation; mutationFn/composite functions never toast, preventing a partial-success case from firing two toasts"

requirements-completed: [REQ-console-write-ux]

coverage:
  - id: D1
    description: "Memory mutation hooks (create[+shared composite]/update/delete/set-visibility/schedule[+shared composite]) call the correct write RPC through engramWrite with correctly-shaped request messages, each an options-as-thunk createMutation"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/lib/mutations/memory.test.ts#buildStoreMemoryRequest, #buildUpdateMemoryRequest, #buildScheduleMemoryRequest (7 tests)"
        status: pass
      - kind: other
        ref: "rg \"createMutation\\(\\(\\) =>\" ui/src/lib/mutations/memory.ts ui/src/lib/mutations/discovery.ts (8 matches, thunk form confirmed)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Create-as-shared and scheduled-as-shared are two-call composites (Store*/Schedule then SetVisibility(SHARED)) returning a discriminated result; a secondary SetVisibility failure (including Unauthenticated/PermissionDenied) is caught into created_private and issues EXACTLY ONE Store/Schedule call (no duplicate-create)"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/lib/mutations/memory.test.ts#createMemoryComposite, #scheduleMemoryComposite (7 tests incl. Unauthenticated/PermissionDenied parameterized cases); ui/src/lib/mutations/discovery.test.ts#createDiscoveryComposite (4 tests)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Every Visibility enum reference uses Visibility.SHARED/Visibility.PRIVATE (never the VISIBILITY_ prefixed proto-source names)"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: other
        ref: "rg \"Visibility\\.VISIBILITY_\" ui/src/lib/mutations/ (no matches)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Edit/delete/set-visibility mutations optimistically update the cache and roll back to the exact prior multi-key state on failure via getQueriesData/setQueriesData; create/schedule are invalidate-only; set-visibility drops a record from a filtered list page whose visibility filter no longer matches"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/lib/mutations/memory.test.ts#applyUpdateOptimistic / snapshot / restore, #applyDeleteOptimistic, #applySetVisibilityOptimistic (3 tests); ui/src/lib/mutations/discovery.test.ts#applyDeleteDiscoveryOptimistic / snapshot / restore, #applySetDiscoveryVisibilityOptimistic (2 tests)"
        status: pass
    human_judgment: false
  - id: D5
    description: "Discovery mutation hooks (create[+shared composite]/delete/set-visibility) exist with correct RPC shapes (typed Citation[], discovery:-prefixed scope, non-empty content); no discovery edit/update hook is exported (D-04 fence)"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/lib/mutations/discovery.test.ts#buildStoreDiscoveryRequest, #discovery.ts exports (2 tests)"
        status: pass
    human_judgment: false

# Metrics
duration: 25min
completed: 2026-07-15
status: complete
---

# Phase 19 Plan 04: Write-RPC Mutation Hooks Summary

**Five memory mutation hooks and three discovery mutation hooks (`createMutation` wrappers over `engramWrite`), with a shared create/schedule-as-shared composite state machine that catches a secondary `SetVisibility` auth failure into a discriminated `created_private` result instead of ever re-issuing the primary create/schedule call.**

## Performance

- **Duration:** 25 min
- **Started:** 2026-07-15T10:00:00-04:00 (approx.)
- **Completed:** 2026-07-15T10:26:00-04:00
- **Tasks:** 2
- **Files modified:** 4 (all new)

## Accomplishments

- `ui/src/lib/mutations/memory.ts` exports `useCreateMemory`, `useUpdateMemory`, `useDeleteMemory`, `useSetMemoryVisibility`, `useScheduleMemory` as options-as-thunk `createMutation` wrappers, plus a full set of named pure exports (request builders, the composite state machine, and cache snapshot/restore/apply functions) so the write-RPC shapes and optimistic-rollback logic are node-testable without a Svelte `QueryClientProvider`.
- The create-as-shared and schedule-as-shared composites (`createMemoryComposite`/`scheduleMemoryComposite`) implement the round-3-hardened state machine: `Store*`/`Schedule` first, then (only if `shared:true`) a caught `SetVisibility(SHARED)` call. A secondary failure — including `Unauthenticated`/`PermissionDenied` — resolves to `{status:'created_private', id}` and is never rethrown, so the record can never be duplicated by a later resubmit.
- `ui/src/lib/mutations/discovery.ts` mirrors the same shape for `useCreateDiscovery`, `useDeleteDiscovery`, `useSetDiscoveryVisibility`, reusing `memory.ts`'s `EngramWriteClient`/`WriteResult`/`CacheEntry` types; no discovery edit/update hook exists (D-04 fence), verified by an explicit test asserting the symbols are `undefined`.
- Optimistic edit/delete/set-visibility snapshot and roll back across `listMemories`/`searchMemories`/`getMemory` (memory) and `searchDiscoveries`/`getMemory` (discovery) via `getQueriesData`/`setQueryData`, iterated per-key rather than through `setQueriesData`'s key-less updater — this is what lets `applySetVisibilityOptimistic` inspect `listMemoriesKey`'s `key[3]` visibility filter and drop a record from a now-mismatched filtered page instead of just patching the field in place.
- 26 vitest node-tier tests cover request shapes, the composite state machine (including a parameterized `Unauthenticated`/`PermissionDenied` case asserting exactly one primary call), and cache snapshot/apply/rollback round-trips against a real `QueryClient`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Memory mutation hooks (create/update/delete/set-visibility/schedule)** - `8c27b008` (feat)
2. **Task 2: Discovery mutation hooks (create/delete/set-visibility)** - `8de11482` (feat)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `ui/src/lib/mutations/memory.ts` - 5 mutation hooks + pure request builders + composite state machine + cache snapshot/apply/rollback functions
- `ui/src/lib/mutations/memory.test.ts` - 20 tests: request shapes, composite state machine (incl. no-duplicate-create auth-failure cases), cache transform round-trips
- `ui/src/lib/mutations/discovery.ts` - 3 mutation hooks (no edit/update) + pure request builder + composite + cache functions
- `ui/src/lib/mutations/discovery.test.ts` - 6 tests: export-surface (no edit hook), request shape, composite, cache round-trips

## Decisions Made

- `source` field on every console-originated write is the literal string `'console'` (`CONSOLE_SOURCE` in `memory.ts`) — no existing convention to match, documented in a source comment; distinguishes console-originated records from MCP-tool writes.
- The composite functions (`createMemoryComposite`, `scheduleMemoryComposite`, `createDiscoveryComposite`) take the write client as an explicit parameter rather than closing over the `engramWrite` import — this is what let the tests build a `createRouterTransport`-backed fake client and assert exact call counts (mirroring the existing `retryOnce.test.ts`/`client.test.ts` pattern from Plan 02) without any mocking library.
- The partial-success toast (`created (private) — sharing failed`) uses `toast.warning` (svelte-sonner exposes a warning variant) rather than `toast.error`/`toast.success` — the record genuinely landed (not a full failure) but sharing did not happen (not a full success), so neither existing convention fit exactly.
- `applyUpdateOptimistic`/`applyDeleteOptimistic`/`applySetVisibilityOptimistic` share one `applyToMemoryCaches` iterator built on `getQueriesData` + per-key `setQueryData` rather than `setQueriesData`'s key-less updater signature — required because the filtered-cache-membership rule needs to read `listMemoriesKey`'s `key[3]` visibility filter per page, which `setQueriesData`'s updater does not receive.
- `discovery.ts`'s `createDiscoveryComposite` duplicates (rather than imports) the small try/catch `SetVisibility` block from `memory.ts`'s internal `shareIfRequested` helper (which is not exported) — it reuses the exported `EngramWriteClient`/`WriteResult` types to avoid type drift between the two composites, at the cost of ~8 lines of duplication instead of adding a shared-composite-helper module for two call sites.

## Deviations from Plan

None - plan executed exactly as written. Two adjustments made purely to satisfy TypeScript's strict message-init typing (not scope changes):
- Test helper functions building fake `Memory` fixtures take a narrow `{ id?, content?, visibility? }` override type instead of `Partial<Memory>`, because `Partial<Memory>` carries an optional `$typeName` field that protobuf-es's `create()` rejects as not assignable to `MessageInit<Memory>`.
- `QueryClient` is imported from `@tanstack/svelte-query` (which re-exports it) rather than `@tanstack/query-core` directly in test files, since `@tanstack/query-core` is a nested transitive dependency not hoisted to the top-level `node_modules` under this repo's pnpm layout.

## Issues Encountered

None. `pnpm check` (svelte-check/tsc) and `npx vitest run --project node` (full node-tier suite, 65 tests across 12 files, not just this plan's 2 new files) both ran clean on the first full pass after the two typing fixes above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 05 (write forms) can import `useCreateMemory`/`useUpdateMemory`/`useDeleteMemory`/`useSetMemoryVisibility`/`useScheduleMemory` from `$lib/mutations/memory` and `useCreateDiscovery`/`useDeleteDiscovery`/`useSetDiscoveryVisibility` from `$lib/mutations/discovery` directly — each returns a `CreateMutationResult` whose `.mutate(vars, {onSuccess, onError})` callback receives the discriminated `WriteResult` (`created`/`created_shared`/`created_private`) for create/schedule, so the form's D-09 whole-create resubmit logic only needs to react to a rejected promise (primary failure), never to the `created_private` success path.
- The `update_mask` dirty-field contract is hook-side complete: `useUpdateMemory` builds the mask from exactly the fields present on the caller's vars object. Plan 05's form is responsible for diffing against the original record to produce that changed-field set and disabling Save when it's empty.
- `useSetMemoryVisibility`/`useSetDiscoveryVisibility`/`useDeleteMemory`/`useDeleteDiscovery` do not swallow a terminal `Unauthenticated`/`PermissionDenied` — Plan 06's inline delete/share host can attach its own `onError` to `.mutate()` and drive the SC3 re-auth CTA while retaining the delete/share target, since these operations are id-idempotent (no duplicate-create risk on a deliberate retry).
- No blockers.

---
*Phase: 19-console-write-ux*
*Completed: 2026-07-15*

## Self-Check: PASSED

All created files (ui/src/lib/mutations/memory.ts, memory.test.ts, discovery.ts, discovery.test.ts, this SUMMARY.md) verified present on disk. Both task commits (8c27b008, 8de11482) verified present in git log.
