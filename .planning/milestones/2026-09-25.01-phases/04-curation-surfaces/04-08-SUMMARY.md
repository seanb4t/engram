---
phase: 04-curation-surfaces
plan: 08
subsystem: ui
tags: [svelte, tanstack-query, connect-rpc, curation, supersede, chain, command-menu]

# Dependency graph
requires:
  - phase: 04-curation-surfaces
    provides: "04-04 (ChainDialog/chain.ts headIdFrom+buildChain), 04-06 (SupersedeDialog, CurationSurfaces host with openSupersede/reopenFromResume/onlookup/onresolvehead, curation/host.svelte.ts registry), 04-07 (RowActions toolbar, ResultsList onsupersede/onchain wiring points, ResultsHeader bulk bar)"
provides:
  - "/search as the complete curation workbench: ⇧S/row-toolbar/bulk-bar/pane all route through CurationSurfaces.openSupersede"
  - "CurationSurfaces.openChain(id) hosting the Chain dialog, with 'Supersede head…' handing off into the supersede dialog"
  - "CurationSurfaces.onbusychange, fired around every archive/restore/supersede COMMIT (never the validate_only preview), driving the bulk bar's disabled state"
  - "/search-owned resume reopen for supersede and archive envelopes (CUR-05, D-15/D-16), alongside the pre-existing memory-kind reopen"
  - "SupersedeDialog's success footer (View superseded / Open) now closes the dialog before handing off to the route"
  - "⌘K record-group curation actions (Supersede/Archive/Restore/Show chain) sourced from the route's registered curation host (Phase 2 D-11)"
affects: [04-09, 04-10, 04-11]

# Actuals (#2632)
actuals:
  tokens: 9028
  tasks: 3
  commits: 3
  plan_head_before: 54200fa76723b1172292d084fceedf321b9d6b06
  plan_head_after: 52e882de50e5138a18c709cf50d4acd79ff95000

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "CurationSurfaces wraps SupersedeDialog's onviewsuperseded/onopenrecord to close its own supersedeOpen state FIRST, then forwards to the route -- the dialog itself never sets open=false (host-authoritative pattern, unchanged), so 'leaves supersede-review mode' has to be the host's own responsibility"
    - "onbusychange wraps only the COMMIT calls (archive/restore onsubmit, the archive/restore onundo inverse, and supersedeOnsubmit) in try/finally -- the supersede validate_only preview never touches it, matching E4's 'preview excluded' contract"
    - "/search registers itself as the curationHost singleton in onMount and returns the unregister function directly (Svelte's onMount cleanup-return convention) -- no separate onDestroy needed"
    - "CommandMenu's curation items are appended to recordItems from curationHost.current.actionsFor(selMemory) rather than a bespoke per-route list, so any future host consumer (04-09/04-10) gets the same ⌘K actions for free"

key-files:
  created: []
  modified:
    - ui/src/routes/search/+page.svelte
    - ui/src/routes/search/search.browser.test.ts
    - ui/src/lib/components/CurationSurfaces.svelte
    - ui/src/lib/components/CurationSurfaces.browser.test.ts
    - ui/src/lib/components/CommandMenu.svelte
    - ui/src/lib/components/CommandMenu.browser.test.ts

key-decisions:
  - "CurationSurfaces (not SupersedeDialog) owns closing the supersede dialog on 'View superseded'/'Open' -- SupersedeDialog.svelte was not in Task 2's declared file list, and the dialog's host-authoritative `open` contract (established in 04-06) means only the host ever flips it. Implemented as handleViewSuperseded/handleOpenRecord wrappers that close supersedeOpen then call the route's own onviewsuperseded/onopenrecord prop, satisfying the must_haves truth ('View superseded (N)' closes the dialog...') without touching the dialog component."
  - "onbusychange is a CurationSurfaces-level concern, not something ArchiveConfirmDialog's own bindable `pending` or SupersedeDialog's internal (non-bindable) `submitPending` could drive directly -- SupersedeDialog never exposes its pending state as a prop, so the host wraps its own mutateAsync calls in try/finally rather than watching dialog-internal state."
  - "The route's curation-host `run` callback dispatches by action name to the existing CurationSurfaces methods (openSupersede/openArchive/openRestore/openChain) rather than exposing a new unified 'run curation action' method on CurationSurfaces itself -- keeps the host registry's contract (CurationHostApi) decoupled from CurationSurfaces' own public API shape."

requirements-completed: [CUR-01, CUR-05]

coverage:
  - id: D1
    description: "CUR-01 tracer: ⇧S on the active row or a multi-selection opens the supersede dialog from /search (key, row toolbar via RowActions, bulk bar, and the pane's Supersede… button all route through CurationSurfaces.openSupersede); committing dims the predecessors in place with the superseded state word"
    requirement: "CUR-01"
    verification:
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — supersede with ⇧S (CUR-01 tracer) > selects m1 and m2, ⇧S opens the supersede dialog, commits once, and both rows dim in place as superseded"
        status: pass
    human_judgment: false
  - id: D2
    description: "D-06: CurationSurfaces.openChain hosts the Chain dialog end to end; the pane's View chain link and a row toolbar Chain button both open it, and 'Supersede head…' hands off into the supersede dialog for the resolved head"
    requirement: "CUR-01"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/CurationSurfaces.browser.test.ts#CurationSurfaces — openChain (D-06)"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — chain dialog entry points (D-06) (2 cases: row toolbar, pane)"
        status: pass
    human_judgment: false
  - id: D3
    description: "D-07 success footer: 'View superseded (N)' closes the dialog, turns on include-superseded and flashes the predecessors; 'Open {short_id}' closes the dialog and navigates sel to the new record"
    requirement: "CUR-01"
    verification:
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — supersede success footer (D-06, D-07) (2 cases)"
        status: pass
    human_judgment: false
  - id: D4
    description: "CUR-05/D-15/D-16: /search's onMount reopens a seeded supersede or archive resume envelope through CurationSurfaces.reopenFromResume (supersede runs exactly one validate_only preview, no commit; archive makes no ArchiveMemory call), consumed exactly once; the pre-existing memory-kind reopen through WriteSurfaces is unchanged"
    requirement: "CUR-05"
    verification:
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — curation resume reopen (CUR-05, D-06, D-15/D-16) (2 cases)"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — re-auth landing recovery (pre-existing memory-kind cases, unaffected)"
        status: pass
    human_judgment: false
  - id: D5
    description: "E4 loading: CurationSurfaces.onbusychange fires true/false around every archive/restore/supersede COMMIT (never the supersede validate_only preview), and /search wires it to the bulk bar's selection.busy so the verb buttons disable while a call is in flight"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/CurationSurfaces.browser.test.ts#CurationSurfaces — onbusychange (E4) (2 cases: archive true/false, preview excluded)"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — bulk-bar busy during a pending curation call (E4)"
        status: pass
    human_judgment: false
  - id: D6
    description: "Phase 2 D-11: with a record selected (?sel=) on a route that registered a curation host, ⌘K's record group offers Supersede/Archive-or-Restore/Show chain items sourced from the host's own actionsFor, and selecting one closes the menu and dispatches run(action, [id]); with no host registered none of these items appear"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/CommandMenu.browser.test.ts#CommandMenu — curation row actions from the registered host (Phase 2 D-11) (4 cases)"
        status: pass
    human_judgment: false

# Metrics
duration: ~50min
completed: 2026-09-27
status: complete
---

# Phase 4 Plan 8: /search Curation Workbench Summary

**Wires supersede, the Chain dialog, curation resume reopen, bulk-bar busy state, and ⌘K row actions into every /search entry point, completing CUR-01 on the primary surface and CUR-05's supersede/archive round trips at the route that owns the envelope.**

## Performance

- **Duration:** ~50 min
- **Started:** 2026-09-27T17:47:00Z (approx.)
- **Completed:** 2026-09-27T18:29:26Z (approx.)
- **Tasks:** 3 of 3 completed (Task 1 tracer, Task 2 TDD, Task 3 TDD)
- **Files modified:** 6 (all modified, none created)

## Accomplishments

- `ui/src/routes/search/+page.svelte`: `onsupersede`/`onchain` wired to `ResultsList` and `DetailPane`; `ResultsHeader`'s bulk bar gets `onsupersede`/`busy`; `CurationSurfaces` gets `onviewsuperseded` (navigate `includeSuperseded` + re-flash), `onopenrecord` (navigate `sel`), `onresumeapplied={consumeResume}`, `onbusychange` (drives `curationBusy`); `onMount` reopens a seeded supersede/archive envelope via `curation?.reopenFromResume(env)` alongside the pre-existing memory-kind branch, and registers `/search` as the active curation host for ⌘K (unregistered on teardown).
- `ui/src/lib/components/CurationSurfaces.svelte`: new `<ChainDialog>` host with `openChain(id)`; `onbusychange` prop wired around every archive/restore/supersede commit (`onsubmit`, `onundo`, `supersedeOnsubmit`) via try/finally, never around the supersede preview; `handleViewSuperseded`/`handleOpenRecord` close `supersedeOpen` before forwarding to the route's `onviewsuperseded`/`onopenrecord`.
- `ui/src/lib/components/CommandMenu.svelte`: `recordItems` appends one item per action `curationHost.current.actionsFor(selMemory)` returns (`Supersede {short}…`, `Archive {short}`, `Restore {short}`, `Show chain {short}`), each closing the menu and calling `curationHost.current.run(action, [sel])`; absent a registered host or a cached record, none of these appear — the existing copy-id items and hand-off row are unchanged.

## Task Commits

Each task was committed atomically (Tasks 2 and 3 ran under TDD, RED confirmed by direct observation):

1. **Task 1 (tracer): supersede from /search keys, bulk bar and pane** — `b20f5335` (feat)
2. **Task 2 (TDD): chain dialog, view-superseded and curation resume on /search** — `591d6325` (feat, GREEN — RED confirmed by reverting `CurationSurfaces.svelte`/`+page.svelte` to their Task 1 commit and re-running the 10 new tests: all failed for real reasons — missing `openChain`, missing `onbusychange`, resume reopen never firing, chain buttons absent, success-footer dialog not closing, bulk bar never disabling — then restored the implementation for GREEN)
3. **Task 3 (TDD): curation row actions in the ⌘K record group** — `52e882de` (feat, GREEN — RED confirmed directly: 3/4 new `CommandMenu.browser.test.ts` cases failed for real reasons — missing record-group items, `run()` never called — before the `CommandMenu.svelte` change)

**Plan metadata:** this commit (docs: complete plan)

_Note: as in 04-04/04-06/04-07's documented precedent, each TDD task produced a single GREEN commit; RED evidence was captured by direct observation (reverting the implementation file and re-running the new tests) rather than a separate `test(...)` commit, since there was no prior-commit baseline gap to exploit differently._

## Files Created/Modified

- `ui/src/routes/search/+page.svelte` — supersede/chain wiring on list/pane/bulk bar, curation resume reopen, bulk-bar busy, ⌘K host registration
- `ui/src/routes/search/search.browser.test.ts` — supersede tracer, resume reopen (supersede + archive), chain entry points, success footer, bulk-bar busy
- `ui/src/lib/components/CurationSurfaces.svelte` — `openChain`, `onbusychange`, success-footer close wrappers, `<ChainDialog>` host
- `ui/src/lib/components/CurationSurfaces.browser.test.ts` — `openChain`/`onbusychange` coverage
- `ui/src/lib/components/CommandMenu.svelte` — curation row actions sourced from `curationHost`
- `ui/src/lib/components/CommandMenu.browser.test.ts` — curation row action coverage

## Decisions Made

See `key-decisions` in the frontmatter — summarized: CurationSurfaces (not SupersedeDialog) closes the supersede dialog on the success-footer actions, preserving SupersedeDialog's host-authoritative `open` contract from 04-06; `onbusychange` wraps CurationSurfaces' own mutation calls in try/finally rather than watching dialog-internal pending state (SupersedeDialog never exposes one); the curation host's `run` callback dispatches to CurationSurfaces' existing per-action methods rather than adding a new unified entry point.

## Deviations from Plan

### Auto-fixed Issues

None — all three tasks' behavior matched their action text directly; no Rule 1/2/3 fixes were needed beyond the literal wiring specified.

### Acceptance-criterion discrepancy (not a code deviation)

**1. [Plan text] Task 3's acceptance criterion `registerCurationHost` count cannot be exactly 1 in a working implementation**
- **Found during:** Task 3 acceptance-criteria verification
- **Issue:** The criterion `rg -o -e 'registerCurationHost' ui/src/routes/search/+page.svelte | wc -l` prints `1`. A working implementation necessarily both imports the symbol (`import { registerCurationHost, ... }`) and calls it (`registerCurationHost({...})`), producing 2 literal matches of the bare identifier — the same pattern Task 1's `openSupersede` criterion sidesteps with "at least 3" and Task 2's `openChain` criterion sidesteps with the more specific `export function openChain`. This reads as an authoring oversight (the import statement's match wasn't accounted for), not an implementation gap.
- **Resolution:** Did not contort the import (e.g. aliasing) purely to force the literal count to 1 — that would trade a real code-quality signal for a miscounted check. Verified the criterion's actual intent instead: `registerCurationHost` is imported once and called exactly once in `+page.svelte` (confirmed by reading the file), the paired criterion (`curationHost[.]current` in `CommandMenu.svelte`, "at least 1") passes with 4 matches, and the plan's own `<verification>` (all three tasks' verify commands + `pnpm --dir ui build`) is the actual hard gate — both pass. No code changed as a result.
- **Verification:** `rg -o -e 'registerCurationHost' ui/src/routes/search/+page.svelte | wc -l` → `2` (1 import + 1 call, both genuine); `rg -o -e 'curationHost[.]current' ui/src/lib/components/CommandMenu.svelte | wc -l` → `4` (passes "at least 1").
- **Impact:** None on functionality — flagged for visibility only, per the executor's "log a criterion that cannot be satisfied after 2 fix attempts as a deviation" instruction rather than silently skipping it.

---

**Total deviations:** 0 auto-fixed. 1 documented acceptance-criterion discrepancy (plan-text miscount, not a functional gap — see above).
**Impact on plan:** None. All three tasks' actual `<verify>` commands and the plan-level `pnpm --dir ui build` gate pass; the flagged criterion's semantic intent (registerCurationHost genuinely wired into the route) is met.

## Issues Encountered

None beyond the acceptance-criterion discrepancy documented above. Three test failures were observed in one `pnpm --dir ui vitest run --project browser` full-suite pass (unrelated files: `ResultsList.browser.test.ts` and two others, all timing-sensitive hover/delay assertions) — a re-run with no code changes was 400/400 green, consistent with host-load flake rather than a regression from this plan's changes.

## Known Stubs

None new. `resolveRecordsKeepAll`'s not-found placeholder chip styling gap (logged in 04-06's `.planning/WINDOWS.md` entry 21) is unchanged by this plan.

## Threat Flags

None beyond the plan's own `<threat_model>` (T-04-18, T-04-19), both mitigated as specified: the resume reopen for supersede runs only the `validate_only` preview (never a commit) and for archive makes no call at all, with `consumeResume` invoked exactly once via `onresumeapplied`; the curation host registers on mount and unregisters on destroy (clearing only its own registration), and ⌘K shows actions only while a host is registered and the selected record is cached — every action still opens a confirm dialog.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `/search` is the complete curation workbench: every CUR-01 entry point (key, row toolbar, bulk bar, pane, ⌘K) and the CUR-05 resume round trip for supersede/archive are wired and tested.
- `curation/host.svelte.ts`'s registry now has its first real consumer (`/search`); plans 04-09/04-10 (Rules/Scheduled) can register the same way for their own ⌘K row actions and reuse `CurationSurfaces.openChain`/`onbusychange` without further host-level changes.
- No blockers.

---
*Phase: 04-curation-surfaces*
*Completed: 2026-09-27*

## Self-Check: PASSED

- All 6 modified key-files verified present on disk with correct content (re-read after final commit).
- All 3 commits (`b20f5335`, `591d6325`, `52e882de`) verified present in `git log --oneline`.
- Re-ran all three tasks' plan-level `<verify>` commands: Task 1 (34/34), Task 2 (53/53 combined), Task 3 (57/57 combined) — all green; `pnpm --dir ui build` exits 0 (confirmed after the final commit).
- Full regression: `pnpm --dir ui vitest run --project browser` (31 files / 400 tests, green on re-run after transient host-load flake) and `--project node` (23 files / 272 tests) both green.
- Re-ran all acceptance-criteria `rg` checks across the three tasks — Task 1 and Task 2's all match their required counts; Task 3's `curationHost[.]current` matches ("at least 1" → 4); Task 3's `registerCurationHost` count (2, not the literal `1` the criterion states) is documented above as a plan-text discrepancy, not a functional gap.
