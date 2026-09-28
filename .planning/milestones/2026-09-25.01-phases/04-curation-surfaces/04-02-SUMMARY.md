---
phase: 04-curation-surfaces
plan: 02
subsystem: ui
tags: [resume-envelope, sessionStorage, svelte, routing, navigation]

# Dependency graph
requires:
  - phase: 03-curation-rpcs-mcp-tools
    provides: SupersedeMemory/ArchiveMemory/RestoreMemory/ListRules/ListScheduled wire contracts the new resume kinds anticipate
provides:
  - "v2 resume envelope (ui/src/lib/resume.ts): a five-kind discriminated union (memory, discovery, supersede, archive, delete) with per-kind structural validation, RESUME_VERSION=2, and ALLOWED_DESTINATIONS widened to /search, /discovery, /rules, /scheduled"
  - "the /observe route, ScopesSidebar, and the observe URL codec fully deleted with no redirect; internal links and Go fixtures retargeted to /search"
  - "AppShell nav, CommandMenu ⌘K, and HeaderSearch command rows list Rules and Scheduled in Observe's former slot"
affects: [04-06, 04-08, 04-09, 04-10]

# Actuals (#2632)
actuals:
  tokens: 21918
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Distributive Omit over a discriminated union (`ResumeEnvelope extends infer E ? E extends ResumeEnvelope ? Omit<E, 'v'|'ts'> : never : never`) to derive a per-kind caller draft type without a second hand-written union"
    - "Per-kind schema-version gating inside a single isValidShape switch, rather than one global version check, so a v1 form envelope keeps restoring while a v1-stamped curation envelope (impossible pre-phase, but defensive) is rejected"

key-files:
  created: []
  modified:
    - ui/src/lib/resume.ts
    - ui/src/lib/resume.test.ts
    - ui/src/lib/components/WriteSurfaces.svelte
    - ui/src/routes/+page.svelte
    - ui/src/routes/page.browser.test.ts
    - ui/src/routes/discovery/+page.svelte
    - ui/src/routes/discovery/discovery.browser.test.ts
    - ui/src/lib/queries.ts
    - ui/src/lib/queries.test.ts
    - ui/src/lib/components/AppShell.svelte
    - ui/src/lib/components/AppShell.browser.test.ts
    - ui/src/lib/components/CommandMenu.svelte
    - ui/src/lib/components/CommandMenu.browser.test.ts
    - ui/src/lib/components/HeaderSearch.svelte
    - ui/src/lib/components/HeaderSearch.browser.test.ts
    - internal/webauth/static_test.go
    - internal/e2e/console_browser_test.go
  deleted:
    - ui/src/routes/observe/+page.svelte
    - ui/src/routes/observe/observe.browser.test.ts
    - ui/src/routes/observe/DeleteBannerHarness.svelte
    - ui/src/lib/components/ScopesSidebar.svelte
    - ui/src/lib/components/ScopesSidebar.browser.test.ts

key-decisions:
  - "Task 2's acceptance-criteria rg command excluding DetailPane.svelte via a non-anchored glob (`!lib/components/DetailPane.svelte`) does not actually match when rg is invoked with `ui/src` as the search root from the repo root — verified the intended zero-occurrence property with an equivalent `!**/DetailPane.svelte` glob instead of editing the plan-authored command; documented as a deviation, not silently worked around"
  - "Pulled Task 3's nav-array Observe->Rules/Scheduled swap forward (as an uncommitted working-tree edit) while still working through Task 2, because Task 2's own acceptance criterion requires zero literal '/observe' occurrences across ui/src and AppShell/CommandMenu/HeaderSearch all held a literal `/observe` href; committed the edits under Task 3's own commit and file list once Task 3's verify/acceptance ran clean, so per-task attribution stayed faithful to the plan's files_modified split"
  - "Reworded three D-14 comments (resume.ts, +page.svelte, discovery/+page.svelte) that named the deleted route by its literal '/observe' substring, so the zero-occurrence acceptance check holds for genuine prose too, not just live hrefs"

requirements-completed: [CUR-05]

coverage:
  - id: D1
    description: "v2 resume envelope: five-kind discriminated union, per-kind validation, TTL/version edges pinned by CUR-05's must-haves"
    requirement: CUR-05
    verification:
      - kind: unit
        ref: "ui/src/lib/resume.test.ts (31 cases: five-kind persist/peek, v1/v2 version gating, TTL boundary at 600000/600001ms, empty-field rejection, ordering preservation, bigint-in-fields never-throws)"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/page.browser.test.ts (/ui/ landing peek-not-consume, base-prefix normalization, archive-envelope goto, deleted-route rejection)"
        status: pass
    human_judgment: false
  - id: D2
    description: "/observe route, ScopesSidebar, and the observe URL codec deleted with no redirect; internal links and Go fixtures retargeted to /search"
    verification:
      - kind: unit
        ref: "ui/src/lib/queries.test.ts (listMemoriesKey only, ObserveParams/parseObserveParams/observeSearch removed)"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/page.browser.test.ts (scope tile -> /search?q=scope:<x>, row activation -> /search?q=<uuid>), ui/src/routes/discovery/discovery.browser.test.ts"
        status: pass
      - kind: unit
        ref: "internal/webauth static_test.go::TestStaticHandlerSPAFallback, go vet ./internal/e2e/"
        status: pass
    human_judgment: false
  - id: D3
    description: "AppShell nav, ⌘K, and header-search command rows list Rules and Scheduled where Observe used to be"
    requirement: null
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/AppShell.browser.test.ts, CommandMenu.browser.test.ts, HeaderSearch.browser.test.ts (38 cases)"
        status: pass
      - kind: unit
        ref: "pnpm --dir ui build (vite build exit 0)"
        status: pass
    human_judgment: false

duration: 26min
completed: 2026-09-27
status: complete
---

# Phase 4 Plan 2: Resume envelope v2, /observe removal, Rules/Scheduled navigation Summary

**Widened the re-auth resume envelope to a five-kind discriminated union (CUR-05/D-16), deleted the redundant `/observe` route (D-14), and put Rules/Scheduled in its navigation slot (D-11).**

## Performance

- **Duration:** 26 min
- **Started:** 2026-09-27T13:31:09Z
- **Completed:** 2026-09-27T13:57:00Z
- **Tasks:** 3
- **Files modified:** 22 (17 modified, 5 deleted)

## Accomplishments

- `ui/src/lib/resume.ts` now exports `FormResumeEnvelope | SupersedeResumeEnvelope | ArchiveResumeEnvelope | DeleteResumeEnvelope` as `ResumeEnvelope`, `RESUME_VERSION = 2`, a per-kind `isValidShape` switch (a v1 form draft still restores; a curation-kind envelope requires v2), and `ALLOWED_DESTINATIONS = ['/search', '/discovery', '/rules', '/scheduled']`.
- `/observe`, `ScopesSidebar`, and the observe URL codec (`ObserveParams`/`parseObserveParams`/`observeSearch`) are gone with no redirect; the landing's scope tiles and recent-row activation, discovery's stale comment, and both Go fixtures (`internal/webauth/static_test.go`, `internal/e2e/console_browser_test.go`) now point at `/search`.
- `AppShell`, `CommandMenu`, and `HeaderSearch` all replace Observe with Rules (`scroll-text` icon) then Scheduled (`calendar-clock` icon) in the same nav slot; the now-unused `EyeIcon` imports are gone.

## Task Commits

1. **Task 1: v2 resume envelope — five kinds, per-kind validation, new destinations, proven through the /ui/ landing** - `a0fd798d` (feat, tracer)
2. **Task 2: Delete /observe, ScopesSidebar and the observe codec; retarget the landing, discovery and the Go fixtures** - `b6e3f91c` (refactor!)
3. **Task 3: Rules and Scheduled replace Observe in the app-shell nav, ⌘K and the header search** - `cff5098e` (feat)

**Plan metadata:** SUMMARY commit follows this file.

## Files Created/Modified

- `ui/src/lib/resume.ts` — v2 discriminated-union envelope, distributive `ResumeDraft`, per-kind `isValidShape`
- `ui/src/lib/resume.test.ts` — 31 cases covering all five kinds, TTL boundary, empty-field rejection, ordering, bigint-never-throws
- `ui/src/lib/components/WriteSurfaces.svelte` — `reopenFromResume` narrowed to `FormResumeEnvelope`
- `ui/src/routes/+page.svelte` — scope tiles and row activation retargeted to `/search` via `encodeSearchParams`
- `ui/src/routes/page.browser.test.ts` — retargeted redirect tests, new archive-envelope and deleted-route cases, new scope-tile/row-activation cases
- `ui/src/routes/discovery/+page.svelte`, `discovery.browser.test.ts` — stale `/observe` comment/returnPath removed
- `ui/src/lib/queries.ts`, `queries.test.ts` — `ObserveParams`/`parseObserveParams`/`observeSearch`/`Visibility`/`VISIBILITIES` removed (zero importers after route deletion); `PAGE_LIMIT`/`CATEGORIES`/`INCLUDE_STATES`/`listMemoriesKey` kept
- `ui/src/lib/components/AppShell.svelte`, `CommandMenu.svelte`, `HeaderSearch.svelte` (+ their browser tests) — Rules/Scheduled replace Observe
- `internal/webauth/static_test.go`, `internal/e2e/console_browser_test.go` — fixture routes moved to `/search`
- Deleted: `ui/src/routes/observe/{+page.svelte,observe.browser.test.ts,DeleteBannerHarness.svelte}`, `ui/src/lib/components/ScopesSidebar.{svelte,browser.test.ts}`

## Decisions Made

- Distributive `Omit` over the `ResumeEnvelope` union (`ResumeEnvelope extends infer E ? E extends ResumeEnvelope ? Omit<E,'v'|'ts'> : never : never`) gives `ResumeDraft` per-kind field checking without a second hand-written union — exactly as the plan's Artifacts table specified.
- Kept the TTL boundary check unchanged (`Date.now() - parsed.ts > RESUME_TTL_MS`, strict `>`) so an envelope aged exactly `RESUME_TTL_MS` still peeks; only removed the *global* version check, replacing it with per-kind version gates.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Task 2's acceptance-criteria `rg` command's DetailPane.svelte exclusion glob does not anchor from the invocation cwd**
- **Found during:** Task 2 acceptance-criteria verification loop
- **Issue:** The plan's literal check `rg -o -e '/observe' ui/src -g '!*.test.ts' -g '!**/gen/**' -g '!lib/components/DetailPane.svelte' | wc -l` is supposed to print `0`, excluding `DetailPane.svelte`'s header comment (explicitly out of scope — owned by plan 04-01). Run from the worktree root with `ui/src` as the search path, the non-anchored glob `!lib/components/DetailPane.svelte` does not match `ui/src/lib/components/DetailPane.svelte`, so the literal command as written cannot print `0` regardless of how clean the rest of `ui/src` is.
- **Fix:** Did not edit the plan (plans are read-only during execution) or silently declare the criterion unsatisfiable. Instead reworded every remaining prose comment naming the deleted route by its literal `/observe` substring (in `resume.ts`, `+page.svelte`, `discovery/+page.svelte`) so the *intended* zero-occurrence property holds, then independently verified intent with the anchored equivalent `-g '!**/DetailPane.svelte'`, which prints `0`. The literal plan command still reports `1` (DetailPane.svelte's own untouched comment), matching the plan's own footnote that this file is out of scope for this wave.
- **Files modified:** `ui/src/lib/resume.ts`, `ui/src/routes/+page.svelte`, `ui/src/routes/discovery/+page.svelte` (comment wording only, no behavior change)
- **Verification:** `rg -o -e '/observe' ui/src -g '!*.test.ts' -g '!**/gen/**' -g '!**/DetailPane.svelte' | wc -l` → `0`
- **Committed in:** `b6e3f91c` (Task 2 commit)

**2. [Rule 3 - Blocking] Task 2's zero-`/observe`-occurrence acceptance criterion required Task 3's nav-array edit**
- **Found during:** Task 2 acceptance-criteria verification loop
- **Issue:** `AppShell.svelte`, `CommandMenu.svelte`, and `HeaderSearch.svelte` (all declared as Task 3's files) each held a literal `${base}/observe` href for the Observe nav entry Task 3 is scoped to replace. Task 2's own acceptance criterion (`rg -o -e '/observe' ui/src ... | wc -l` → `0`, with no exclusion for these three files) cannot pass until they are also clean, and the HARD GATE protocol blocks starting the next task until a task's own acceptance criteria clear.
- **Fix:** Implemented Task 3's full nav-array swap (Rules/Scheduled replacing Observe, `EyeIcon` imports removed) as an uncommitted working-tree edit during Task 2's fix loop, re-ran Task 2's acceptance criteria (now `0`), then continued to Task 3 as planned — ran Task 3's own `<verify>`/`<acceptance_criteria>` against the same edits and committed them under Task 3's commit message and declared file list, so per-task file attribution in git history still matches the plan exactly.
- **Files modified:** `ui/src/lib/components/AppShell.svelte`, `AppShell.browser.test.ts`, `CommandMenu.svelte`, `CommandMenu.browser.test.ts`, `HeaderSearch.svelte`, `HeaderSearch.browser.test.ts`
- **Verification:** Task 2's `rg` acceptance check → `0`; Task 3's own verify (38/38 browser tests, `pnpm --dir ui build` exit 0) and acceptance criteria (`label: '(Rules|Scheduled)'` → 2, `label: 'Observe'` → 0, per file) both pass
- **Committed in:** `cff5098e` (Task 3 commit, not Task 2's)

---

**Total deviations:** 2 auto-fixed (both Rule 3 — blocking issues preventing the current task's acceptance gate from clearing). **Impact:** No scope creep beyond what Task 2's own literal acceptance criteria required; Task 3's substantive nav work and its own verify/acceptance/commit still happened exactly where the plan placed them — only the *timing* of the edit (made during Task 2's fix loop, committed at Task 3) shifted to satisfy Task 2's gate.

## Issues Encountered

- The first `vitest run --project browser` pass on `AppShell.browser.test.ts`/`CommandMenu.browser.test.ts`/`HeaderSearch.browser.test.ts` failed all 26 tests with `TypeError: Cannot read properties of null (reading 'nodes')` inside Svelte's hydration internals — a stale `ui/node_modules/.vite` pre-bundle cache from before the new `scroll-text`/`calendar-clock` icon imports were added. Clearing `ui/node_modules/.vite` and `ui/.vitest` resolved it; all 38 tests passed on the next run. Not a code defect — no fix committed, just a cache clear.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `ResumeEnvelope`'s `SupersedeResumeEnvelope`/`ArchiveResumeEnvelope`/`DeleteResumeEnvelope` kinds and the widened `ALLOWED_DESTINATIONS` are ready for plans 04-06/04-08/04-09/04-10 to wire their own dialogs' `persistResume` calls and each route's `peekResume`/`consumeResume` ownership against.
- `/rules` and `/scheduled` are live nav destinations (AppShell/⌘K/header search) but have no route yet — those land in later plans in this phase.
- No blockers.

---

## Self-Check: PASSED

- All 17 modified files found on disk; all 5 deleted files confirmed absent.
- All 3 task commits (`a0fd798d`, `b6e3f91c`, `cff5098e`) found in git log.

*Phase: 04-curation-surfaces*
*Completed: 2026-09-27*
