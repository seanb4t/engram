---
phase: 07-console-cli-state-surfacing
plan: 04
subsystem: ui
tags: [svelte, tanstack-query, url-state, connect-rpc]

# Dependency graph
requires:
  - phase: 07-console-cli-state-surfacing
    provides: "07-01's ListMemoriesRequest.include_archived/include_superseded/include_scheduled wire fields (vendored into ui/src/lib/gen/engram/v1/engram_pb.ts as includeArchived/includeSuperseded/includeScheduled) and 07-02's memoryStateWords/MemoryRow/MemoryDetail state rendering, which this plan's toggles make reachable for the archive/superseded/scheduled tiers"
provides:
  - "ui/src/lib/queries.ts: ObserveParams.includeArchived/includeSuperseded/includeScheduled, INCLUDE_STATES (shared parse/encode source), and listMemoriesKey's three trailing booleans"
  - "ui/src/lib/components/ScopesSidebar.svelte: the three include-state checkboxes (D-16) under a new `state` section below `visibility`, with an `oninclude` callback"
  - "ui/src/routes/observe/+page.svelte: oninclude wired to navigate() with offset reset, and the three flags threaded into engram.listMemories()'s request object and query cache key"
affects: []

# Actuals (#2632)
actuals:
  tokens: 4224
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Repeated URL parameter with a shared recognised-value list (INCLUDE_STATES) driving both parse (filter-against-known-set) and encode (iterate-and-append) — mirrors the existing CATEGORIES/`cat` pattern exactly rather than inventing a boolean-parameter convention"

key-files:
  created: []
  modified:
    - ui/src/lib/queries.ts
    - ui/src/lib/queries.test.ts
    - ui/src/lib/components/ScopesSidebar.svelte
    - ui/src/lib/components/ScopesSidebar.browser.test.ts
    - ui/src/routes/observe/+page.svelte

key-decisions:
  - "Task 1 touches only the listMemoriesKey call site in +page.svelte per the plan's task-boundary split; the engram.listMemories() request object's three fields were left to Task 3, which also owns the ScopesSidebar wiring — kept the diff-per-task boundary the plan specifies even though both changes land in the same file."
  - "observeSearch's inc-flag encoding is driven by iterating INCLUDE_STATES against a small archived/superseded/scheduled lookup map, rather than three separate `if` statements, so the encode and parse literally cannot drift out of the same recognised-value list."

requirements-completed: [REQ-console-record-state]

coverage:
  - id: D1
    description: "The three include flags round-trip through ObserveParams, the URL (repeated `inc` parameter), and listMemoriesKey's cache key, with an all-false default byte-identical to pre-phase output"
    requirement: "REQ-console-record-state"
    verification:
      - kind: unit
        ref: "ui/src/lib/queries.test.ts (12 cases: default-false, no inc emitted at all-false, byte-identical all-false output, two-of-three inc emission, unrecognised-value drop, full idempotency sweep over all 8 flag combinations, listMemoriesKey divergence on includeSuperseded)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Three labelled, keyboard-reachable include-state checkboxes render in ScopesSidebar below visibility, reflect their incoming props, and call oninclude with the other two flags' incoming values preserved"
    requirement: "REQ-console-record-state"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/ScopesSidebar.browser.test.ts (13 cases: 8 pre-existing + 5 new — label text, checked-state reflection, check/uncheck preserving sibling flags, existing controls unchanged)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Flipping a sidebar toggle changes the URL, resets the page offset, invalidates the query cache, and widens the engram.listMemories() request so previously-unreachable archived/superseded/scheduled records surface wearing 07-02's state markers"
    requirement: "REQ-console-record-state"
    verification:
      - kind: unit
        ref: "cd ui && npm run test (31 files, 247 tests, all passing, no regressions)"
        status: pass
      - kind: other
        ref: "rg -n 'includeArchived|oninclude|parseObserveParams' ui/src/routes/observe/+page.svelte (confirms the flag reaches both listMemoriesKey and the request object, oninclude wired with offset:0, no second parseObserveParams call introduced)"
        status: pass
    human_judgment: true
    rationale: "The plan's own <verification> block includes a manual step (load /observe?scope=...&inc=archived against a running server with a seeded archived record, confirm the badge and dimmed treatment render, confirm reload reproduces the view). That requires a live server + Qdrant-backed data this worktree does not stand up; automated tests prove the wiring is structurally correct (flag reaches every layer, cache key changes, URL round-trips) but not the live rendered pixels."

# Metrics
duration: ~35min
completed: 2026-08-21
status: complete
---

# Phase 7 Plan 4: Console Sidebar Include-State Toggles Summary

**Three include-archived/include-superseded/include-scheduled checkboxes in `ScopesSidebar`, round-tripped through a repeated `inc` URL parameter and threaded into `listMemories`'s request and cache key, using the exact same `ObserveParams` machinery every other console filter already uses.**

## Performance

- **Duration:** ~35 min
- **Completed:** 2026-08-21
- **Tasks:** 3/3 completed
- **Files modified:** 5

## Accomplishments

- `ObserveParams` gains `includeArchived`/`includeSuperseded`/`includeScheduled`, encoded as a repeated `inc` URL parameter that mirrors the existing `cat` parameter exactly — including its filter-against-a-recognised-set parse (`INCLUDE_STATES`, exported so parse and encode share one source). All three false is the default and produces a URL byte-identical to the pre-phase output, proven by an explicit test.
- `listMemoriesKey` gained three trailing booleans, so flipping any one toggle produces a distinct `svelte-query` cache key and refetches instead of serving the previous filter's cached page.
- `ScopesSidebar` renders a new `state` section directly below `visibility`, with three `include archived` / `include superseded` / `include scheduled` checkboxes using the same `Checkbox` + label markup as the existing category filters (no `--cat-*` colour), each carrying its own `aria-label`. Every existing control — scope buttons, category checkboxes, visibility select — kept its exact position and behaviour.
- `observe/+page.svelte` wires `ScopesSidebar`'s new `oninclude` callback to `navigate({ includeArchived, includeSuperseded, includeScheduled, offset: 0 })`, resetting pagination exactly as the existing `onfilter` handler does, and threads the three flags into both `listMemoriesKey(...)` and the `engram.listMemories({...})` request object from the same `parseObserveParams(page.url.searchParams)` call the options function already made — no second source of filter-state truth introduced.

## Task Commits

Each task was committed atomically:

1. **Task 1: Round-trip the three include flags through ObserveParams** - `62554c65` (feat)
2. **Task 2: The three include toggles in ScopesSidebar** - `260c4a73` (feat)
3. **Task 3: Thread the flags from the URL into the listMemories request** - `7e79305a` (feat)

**Plan metadata:** (this SUMMARY.md commit, immediately following)

_Note: this plan carried `tdd="true"` on Tasks 1-2; tests and implementation landed together per task rather than as separate RED/GREEN commits, matching the same interpretation 07-01's SUMMARY documented for a wave-parallel worktree executor dispatch that does not support mid-plan checkpoint interruption/resumption._

## Files Created/Modified

- `ui/src/lib/queries.ts` - `ObserveParams`'s three new boolean fields, `INCLUDE_STATES`, `parseObserveParams`/`observeSearch` extended, `listMemoriesKey`'s three trailing booleans
- `ui/src/lib/queries.test.ts` - EXTENDED in place (was 4 cases, now 12): all four pre-existing cases preserved, one (`listMemoriesKey`) widened per the plan's explicit permitted edit, eight new cases covering the full `<behavior>` list
- `ui/src/lib/components/ScopesSidebar.svelte` - three new props, `oninclude` callback, new `state` section with three checkboxes
- `ui/src/lib/components/ScopesSidebar.browser.test.ts` - EXTENDED in place (was 8 cases, now 13): five new cases for label text, checked-state reflection, toggle-preserves-siblings, and existing-controls-unchanged
- `ui/src/routes/observe/+page.svelte` - `listMemoriesKey` call site widened (Task 1), `ScopesSidebar` props/`oninclude` wiring and `engram.listMemories()` request widened (Task 3)

## Decisions Made

- **Task 1/Task 3 diff-level split preserved in `+page.svelte`.** Task 1 touches only the `listMemoriesKey(...)` call site (required immediately since the signature gained three required trailing parameters and `pp` already carries them post-Task-1); Task 3 owns both the `ScopesSidebar` prop/`oninclude` wiring and the `engram.listMemories()` request object fields, per the plan's own task boundary — even though all three edits land in the same file.
- **`observeSearch`'s inc encoding shares `INCLUDE_STATES` with `parseObserveParams`** via a small lookup map (`{archived, superseded, scheduled} → boolean`) iterated in canonical order, rather than three independent `if` statements — structurally impossible for parse and encode to recognise different value sets, per the plan's explicit ask ("Export the recognised value list as `INCLUDE_STATES` so the parse and the encode share one source").

## Deviations from Plan

None - plan executed exactly as written. The `<verification>` block names `ui/package-lock.json`, but this repo uses pnpm (`ui/pnpm-lock.yaml`); verified `git diff --exit-code` clean against the actual lockfile instead of the plan's literal filename — not a deviation in substance (no dependency changed), just the verification command adapted to the repo's actual package manager.

## Issues Encountered

- **`node_modules` was absent in this worktree** and had to be bootstrapped via `pnpm install --frozen-lockfile` before any test could run — expected for a freshly-created worktree (same as 07-02's executor noted), not a deviation.
- **`cd ui && npm run check` (svelte-check) crashes on startup in this environment**, unrelated to this plan's changes: `TypeError: Cannot read properties of undefined (reading 'useCaseSensitiveFileNames')` inside svelte-check's `ConfigLoader` construction, before any project file loads — the same `svelte-check@4.7.3`/`typescript@7.0.2` incompatibility documented as pre-existing in 07-02-SUMMARY.md and in this plan's own dispatch instructions (`<known_preexisting_environment_issues>`). TypeScript correctness was instead verified by matching every field access against the generated `ui/src/lib/gen/engram/v1/engram_pb.ts` types read in full, and by the full green test suite below.
- **`cd ui && npm run lint` has no corresponding script** in `ui/package.json` (pre-existing gap, also documented in this plan's dispatch instructions and 07-02-SUMMARY.md) — Task 3's acceptance criteria names it but it cannot execute as written. Substituted `cd ui && npm run test` (full suite, both projects) as the closest available signal, which is green.
- **Manual verification step not run.** The plan's `<verification>` block's last item ("load `/observe?scope=…&inc=archived`, confirm archived records appear...") requires a running `engram serve` instance backed by a live Qdrant with a seeded archived record — outside this worktree's scope (no server/DB standing up here). Flagged as `human_judgment: true` in the coverage block above for the phase verifier's UAT pass. Every layer feeding that manual check (flag reaching the request, cache-key change, URL round-trip, badge rendering itself per 07-02) is proven by automated tests.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- The console's include-state filtering machinery is complete: `ObserveParams`/`INCLUDE_STATES`/`listMemoriesKey`/`ScopesSidebar`/`observe/+page.svelte` form one coherent, bookmarkable, cache-correct chain from URL to RPC request, reusing 07-02's already-shipped badge/dim rendering for the records this plan reveals.
- The live-server manual verification step (badge + dimmed treatment on an actual archived record) remains open for the phase-level UAT/verifier pass — see Issues Encountered.
- No known follow-ups specific to this plan beyond the two pre-existing, already-documented environment gaps (`npm run check` crash, missing `npm run lint` script) that 07-02 already surfaced to the orchestrator/phase verifier.

---

## Self-Check: PASSED

- `ui/src/lib/queries.ts` — FOUND (modified)
- `ui/src/lib/queries.test.ts` — FOUND (modified)
- `ui/src/lib/components/ScopesSidebar.svelte` — FOUND (modified)
- `ui/src/lib/components/ScopesSidebar.browser.test.ts` — FOUND (modified)
- `ui/src/routes/observe/+page.svelte` — FOUND (modified)
- Commit `62554c65` — FOUND in `git log`
- Commit `260c4a73` — FOUND in `git log`
- Commit `7e79305a` — FOUND in `git log`
- `cd ui && npx vitest run --project=node queries` — 12/12 passed
- `cd ui && npm run test:browser -- ScopesSidebar` — 13/13 passed
- `cd ui && npm run test` — 31 files, 247 tests, all passing, 0 failures
- `git diff --exit-code ui/package.json ui/pnpm-lock.yaml` — clean (no dependency change)
- `git diff --diff-filter=D --name-only` across all three task commits — no deletions

---
*Phase: 07-console-cli-state-surfacing*
*Completed: 2026-08-21*
