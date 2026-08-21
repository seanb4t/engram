---
phase: 07-console-cli-state-surfacing
plan: 07
subsystem: ui
tags: [svelte, tanstack-query, connect-rpc, migration, error-handling]

# Dependency graph
requires:
  - phase: 07-console-cli-state-surfacing
    provides: "07-06's MigrateStatus Connect RPC and MigrateStatusResult.Pending() single-definition helper (pending / futureTotal on engram.migrateStatus)"
  - phase: 07-console-cli-state-surfacing
    provides: "07-04's WriteSurfaces.browser.test.ts QueryClientProvider-wrapper render idiom, reused for MigrationBanner and AppShell tests"
provides:
  - "handleQueryError — the whole QueryCache.onError routing (auth redirect, then meta.silent opt-out, then reportError), exported from ui/src/lib/errors.ts so +layout.svelte and every test lane share one implementation"
  - "logError — the log-without-banner half of reportError, letting a query fail silently to the user while still reaching the console"
  - "MigrationBanner.svelte — the two silent-at-zero migration advisory strips (D-07), mounted on AppShell so they render on every console route"
affects: []

# Actuals (#2632)
actuals:
  tokens: 5094
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Silent-by-default derivation: MigrationBanner reads pending/futureTotal off `query.data?.field ?? 0n`, so the undefined-data case (loading, or a rejected fetch that never populates data) collapses into the same zero-render branch as a genuine zero/zero response — no separate loading/error markup was needed."
    - "Single logging owner via meta.silent: a diagnostic query opts out of the global error banner with `meta: { silent: true }`, and the centralized QueryCache.onError (handleQueryError) is the only place that calls console.error for it — the component itself carries no error-logging $effect, closing off the double-log failure mode the plan called out."

key-files:
  created:
    - ui/src/lib/components/MigrationBanner.svelte
    - ui/src/lib/components/MigrationBanner.browser.test.ts
  modified:
    - ui/src/lib/errors.ts
    - ui/src/lib/errors.test.ts
    - ui/src/routes/+layout.svelte
    - ui/src/lib/components/AppShell.svelte
    - ui/src/lib/components/AppShell.browser.test.ts

key-decisions:
  - "handleQueryError's optional second parameter is typed structurally (`{ meta?: Record<string, unknown> }`) rather than importing TanStack's `Query` type, so errors.ts needs no `@tanstack/svelte-query` import and `onError: handleQueryError` still type-checks as a direct assignment against QueryCache's options (TanStack's QueryMeta is itself `Record<string, unknown>`)."
  - "MigrationBanner renders nothing purely from `query.data` being undefined (loading and failed-fetch both leave it unset) plus both counts defaulting to 0n — no explicit `query.isLoading`/`query.error` branches were needed to satisfy the silent-at-zero/loading/failure trio."
  - "Both test suites (errors.test.ts node-project cases and MigrationBanner.browser.test.ts's failed-fetch case) import the production `handleQueryError` from `$lib/errors` rather than re-declaring the routing closure, per the plan's explicit anti-vacuity instruction — a test-local copy could pass while production diverged from it."

requirements-completed: [REQ-migration-state-visible]

coverage:
  - id: D1
    description: "A query can be marked meta.silent and fail without raising the global error banner, while every existing query's error behaviour (auth redirect first, then log-and-banner) is unchanged; +layout.svelte delegates to the exported handleQueryError by direct reference."
    requirement: "REQ-migration-state-visible"
    verification:
      - kind: unit
        ref: "ui/src/lib/errors.test.ts — logError / reportError / handleQueryError describe block (10 tests total in the file)"
        status: pass
    human_judgment: false
  - id: D2
    description: "MigrationBanner is silent at zero, silent while loading, and silent on a failed MigrateStatus fetch (logging exactly once via the centralized handler, never setting the global error banner); renders one or two independently-gated strips, behind-version before ahead-version, singular/plural copy correct."
    requirement: "REQ-migration-state-visible"
    verification:
      - kind: unit
        ref: "ui/src/lib/components/MigrationBanner.browser.test.ts (7 tests)"
        status: pass
    human_judgment: false
  - id: D3
    description: "AppShell mounts MigrationBanner between the header and the route content row on every route, without disturbing the shell's existing layout or the two pre-existing nav/brand-mark assertions."
    requirement: "REQ-migration-state-visible"
    verification:
      - kind: unit
        ref: "ui/src/lib/components/AppShell.browser.test.ts (4 tests: 2 pre-existing + 2 new)"
        status: pass
    human_judgment: false

duration: ~25min
completed: 2026-08-20
status: complete
---

# Phase 07 Plan 07: The Console Migration Banner (D-07) Summary

**A slim, two-strip migration advisory now renders on every console route via `AppShell`, silent at zero/loading/failure alike, backed by a single exported `handleQueryError` that both production and every test lane share for the "diagnostic fails without becoming a user-facing error" property.**

## Performance

- **Duration:** ~25 min
- **Completed:** 2026-08-20T23:04:45-04:00
- **Tasks:** 3/3 completed
- **Files modified:** 7 (2 created, 5 modified)

## Accomplishments

- `ui/src/lib/errors.ts` gained `logError` (the console-only half of `reportError`, extracted so the file's single `console.error` call site is preserved) and `handleQueryError` — the whole `QueryCache.onError` routing (auth redirect first, then the `meta.silent` opt-out, then `reportError`) lifted out of `+layout.svelte`'s inline closure into an exported function both production and every test lane import by reference.
- `+layout.svelte` now delegates via a bare `onError: handleQueryError` reference — no layout-local branch logic left to drift from the tested function.
- `MigrationBanner.svelte` (new) fetches `engram.migrateStatus({})` once per root `QueryClient` lifetime (`staleTime: Infinity`, no mount/focus refetch, no retry, `meta: { silent: true }`) and renders zero, one, or two `px-3 py-2 text-[13px]` strips: a neutral behind-version strip (`pending > 0n`) first, a destructive ahead-version strip (`futureTotal > 0n`) second — both read straight off the server-computed fields, never re-derived client-side. Loading and a failed fetch both leave `query.data` undefined, which the same zero-default arithmetic (`?? 0n`) renders as nothing, so no separate loading/error branch was needed.
- `AppShell.svelte` mounts `<MigrationBanner />` between `</header>` and the `flex flex-1 min-h-0` content row — the one component every route shares, so the banner appears everywhere without per-route wiring.
- All three tasks' `<behavior>` and acceptance-criteria greps pass, including the call-shaped "exactly one `console.error`" gate for a rejected migration query and the "exactly one `onError`/`meta: { silent: true }`" line-count gates.

## Task Commits

Each task was committed atomically:

1. **Task 1: A query can log its failure without raising the global error banner** - `a1f622ea` (feat)
2. **Task 2: The MigrationBanner component** - `567e40bc` (feat)
3. **Task 3: Mount the banner on every route** - `00459371` (feat)

## Files Created/Modified

- `ui/src/lib/errors.ts` - `logError`, `handleQueryError` (exported), `reportError` now calls `logError`
- `ui/src/lib/errors.test.ts` - extended (not replaced) with 7 new cases covering `logError`/`reportError`/`handleQueryError`'s four branches, alongside the 3 pre-existing `describeError` cases
- `ui/src/routes/+layout.svelte` - `onError: handleQueryError` direct-reference delegation; dropped now-unused `mapAuthError`/`reportError` imports
- `ui/src/lib/components/MigrationBanner.svelte` (new) - the two silent-at-zero strips
- `ui/src/lib/components/MigrationBanner.browser.test.ts` (new) - 7 tests covering every `<behavior>` case
- `ui/src/lib/components/AppShell.svelte` - `<MigrationBanner />` mounted between header and content row
- `ui/src/lib/components/AppShell.browser.test.ts` - existing 2 tests wrapped in `QueryClientProvider` with a mocked zero/zero `migrateStatus`; 2 new tests for the zero-response silent case and the non-zero placement case

## Decisions Made

- **`handleQueryError`'s second parameter is structurally typed**, not TanStack's `Query`, so `errors.ts` imports nothing from `@tanstack/svelte-query` and the layout's `onError: handleQueryError` still type-checks as a direct assignment.
- **No explicit `isLoading`/`error` branches in `MigrationBanner`** — `query.data?.field ?? 0n` collapses loading and failure into the same silent-zero render path as a genuine zero/zero response, satisfying all three `<behavior>` silence requirements with one arithmetic expression rather than three conditionals.
- **Both test suites import the production `handleQueryError`** rather than re-declaring the routing closure, closing off the vacuity the plan explicitly warned about (a test-local copy could stay green while production diverged).

## Deviations from Plan

None — plan executed exactly as written. Icon choice (`ArrowUpCircleIcon` for behind-version, matching the UI-SPEC's suggested `ArrowUpCircle`; `TriangleAlertIcon` for ahead-version, matching the plan's explicit import path) was the one open choice the plan itself flagged as non-load-bearing.

## Issues Encountered

- **`ui/node_modules` was absent in this worktree** (gitignored, as expected for a fresh worktree) — ran `pnpm install` before any test could execute. Not a deviation from the plan; a one-time environment setup step.
- **`cd ui && npm run check` (svelte-check) crashes on startup** with `TypeError: Cannot read properties of undefined (reading 'useCaseSensitiveFileNames')` — the pre-existing `svelte-check@4.7.3` / `typescript@7.0.2` incompatibility flagged in this plan's own instructions. Confirmed not caused by this plan's changes: reproduced identically before writing this SUMMARY. Substituted `npx tsc --noEmit -p tsconfig.json` as a type-check signal; it reports 8 pre-existing errors in `ui/src/lib/components/ui/{badge,button,tabs}/index.ts` (a raw-`tsc`-vs-`.svelte`-module-resolution artifact unrelated to svelte-check), none touching any file this plan modified. Ran this substitute both before and after Task 3 — the error set was byte-identical both times, confirming no new TypeScript errors were introduced.
- **`npm run lint` does not exist** in `ui/package.json` — also flagged as pre-existing in this plan's instructions; not run.
- **`observe.browser.test.ts` alone emits ~104 pre-existing `[svelte] derived_inert` console warnings** (unrelated `$derived` reads after an unmounted effect, in code this plan never touches). Isolated by running `AppShell` and `MigrationBanner` test files alone — each produced zero `derived_inert` warnings — confirming this plan's new component introduces none of its own. `cd ui && npm run test` (both Vitest projects, all 32 files) passed 263/263 regardless.
- **Manual visual verification** (viewport-narrowing wrap check, and the live-collection "load a route, run `engram migrate`, reload" check) from the plan's `<verification>` section was **not performed** — this environment has no running server or Qdrant instance to drive it against. The banner strip uses no `whitespace-nowrap`/`truncate` classes and sits inside a `flex items-center gap-2` row with the sentence in a plain `<span>`, which should wrap the strip rather than clip per the `long-text / E4` backstop truth, but this was not confirmed by rendering at a narrow viewport.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `REQ-migration-state-visible` is now closed on the console: marked complete in `.planning/REQUIREMENTS.md` (both the checkbox and the traceability table row), since this plan was the last of the two declaring plans (07-06 closed the CLI side; this plan closes the console side).
- This is the last plan in Phase 07 (`console-cli-state-surfacing`). No downstream phase currently depends on this plan's output (`affects: []`).
- The two pre-existing environment issues (`svelte-check` crash, missing `lint` script) remain open and unowned by this plan, exactly as flagged in the plan's own instructions.

## Self-Check: PASSED

- FOUND: `ui/src/lib/components/MigrationBanner.svelte`
- FOUND: `ui/src/lib/components/MigrationBanner.browser.test.ts`
- FOUND: `ui/src/lib/errors.ts` (contains `export function handleQueryError`)
- FOUND: `ui/src/routes/+layout.svelte` (contains `onError: handleQueryError`)
- FOUND: `ui/src/lib/components/AppShell.svelte` (contains `<MigrationBanner />`)
- FOUND commit `a1f622ea` (Task 1)
- FOUND commit `567e40bc` (Task 2)
- FOUND commit `00459371` (Task 3)
- `cd ui && npm run test` (both Vitest projects, 32 files) passes 263/263
- `git diff --exit-code -- ui/package.json ui/pnpm-lock.yaml` exits 0
- `.planning/REQUIREMENTS.md` shows `REQ-migration-state-visible` as `[x]` / `Complete`

---
*Phase: 07-console-cli-state-surfacing*
*Completed: 2026-08-20*
