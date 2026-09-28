---
phase: 02-recall-first-search
plan: 10
subsystem: ui
tags: [svelte5, tanstack-query, connect-web, shadcn-svelte, design-system, docs]

requires:
  - phase: 02-recall-first-search
    provides: "recall_gate_hidden CLI/docs coverage (plan 02-02)"
  - phase: 02-recall-first-search
    provides: "CommandMenu.svelte, shouldFilter={false} Command.Dialog pattern (plan 02-05)"
  - phase: 02-recall-first-search
    provides: "RecallState.svelte, recall-header.ts, params.ts nextK/listMemoriesCursorKey/listMemoriesRequest, the honest-state pipeline on /search (plan 02-09)"
provides:
  - "/ renders its cross-spine recent feed through ResultsList (mode=unranked) instead of MemoryList"
  - "/observe renders its list and record through ResultsList + RecallSplit + DetailPane instead of MemoryList/MemoryDetail, keeping ScopesSidebar and offset pagination"
  - ".claude/skills/engram-console-conventions/SKILL.md: tokens, state-word order, scope-chip semantics, the classifier contract, the honest-feedback rule, the keyboard model — the skill later UI phases cite instead of re-deriving these facts"
  - ".claude/skills/engram-connect-client/SKILL.md: the engram/engramWrite split, query-key conventions, CSRF contract, resume envelope, and a per-RPC contract table for SearchMemories/ListMemories/GetMemory/ListScopes"
  - "internal/webauth/static rebuilt from ui/ via task ui:build, proven reproducible (a second build produces an identical tree)"
affects: [04-curation-surfaces, 05-related-memories-graph-tag-cloud]

actuals:
  tokens: 11433
  tasks: 3
  commits: 3
  plan_head_before: d9af3905da2299b2aa5d4ab16d5e184f3e29bc3a

tech-stack:
  added: []
  patterns:
    - "WriteSurfaces lives in a stable toolbar row OUTSIDE RecallSplit's list()/detail() snippets on /observe, mirroring /search (plan 02-09) — RecallSplit's narrow/wide layout switch tears down and recreates snippet content, which would destroy WriteSurfaces' bind:this reference and its one-shot onMount resume-restore the first time the measured container width crosses the narrow breakpoint"
    - "A route-level error string that would otherwise collide with the e2e's negative-assertion substring ('failed to load') is deliberately reworded ('Could not load recent memories') so that guard stays meaningful for a genuine regression rather than tripping on this route's own happy-path copy"

key-files:
  created:
    - .claude/skills/engram-console-conventions/SKILL.md
    - .claude/skills/engram-connect-client/SKILL.md
  modified:
    - ui/src/routes/+page.svelte
    - ui/src/routes/page.browser.test.ts
    - ui/src/routes/observe/+page.svelte
    - ui/src/routes/observe/observe.browser.test.ts
    - CLAUDE.md
    - internal/webauth/static/** (rebuilt, 43 files)

key-decisions:
  - "WriteSurfaces relocated from inside /observe's list pane to a stable toolbar row outside RecallSplit (deviation from the plan's literal PaneGroup-replacement wording, applied as a Rule 1 bug-prevention fix — plan 02-09 already documented this exact remount pitfall for /search, and /observe's original code placed WriteSurfaces inside the pane that RecallSplit now owns)."
  - "recentQ's failure text on / changed from a hypothetical 'failed to load recent memories' to 'Could not load recent memories', per the plan's own explicit instruction, so the e2e's `strings.Contains(body, \"failed to load\")` negative-assertion guard keeps a genuine regression signal."
  - "observe.browser.test.ts's edit-flow test now drives the 'e' row-action key (ResultsList's keyboard model) instead of clicking a 'row actions' kebab menu that no longer exists on ResultsList rows, per the plan's explicit instruction."
  - "The WR-04 delete-flow test now clicks DetailPane's own inline 'Delete' button (there is no 'record actions' menu on DetailPane) and scopes the confirm click to `getByRole('dialog').getByRole('button', { name: 'Delete' })`, since DetailPane's trigger button and DeleteConfirmDialog's confirm button share the exact accessible name 'Delete'."

patterns-established: []

requirements-completed: [DSYS-01, DSYS-02, ROW-01, ROW-07]

coverage:
  - id: D1
    description: "D-10: / keeps its heading, scope tiles and loading/error text, and renders its cross-spine recent feed through the shared ResultsList; choosing a recent row opens it on /observe as today"
    requirement: "ROW-01"
    verification:
      - kind: automated_ui
        ref: "ui/src/routes/page.browser.test.ts#/ui/ root — recent feed on the shared ResultsList (D-10) > shows the heading and loading scopes"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/page.browser.test.ts#/ui/ root — recent feed on the shared ResultsList (D-10) > renders a listbox named Recent memories with one option per returned memory"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/page.browser.test.ts#/ui/ root — recent feed on the shared ResultsList (D-10) > activating a row navigates to /ui/observe?sel=<id>"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/page.browser.test.ts#/ui/ root — Recent memories cross-spine feed (#500) (pre-existing listMemories request/cache-key tests, unchanged)"
        status: pass
    human_judgment: false
  - id: D2
    description: "D-12: /observe renders its list through ResultsList and its record through RecallSplit + DetailPane, keeps ScopesSidebar and offset pagination, and routes row keys and pane buttons through WriteSurfaces; the re-auth resume tests keep passing"
    requirement: "ROW-07"
    verification:
      - kind: automated_ui
        ref: "ui/src/routes/observe/observe.browser.test.ts#observe route — shared list and pane (D-12) (4 cases: select-a-scope guidance, listbox rendering, open, close)"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/observe/observe.browser.test.ts#observe route — onedit fetches the FULL record (Codex round-2 HIGH) > pressing 'e' triggers openEdit"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/observe/observe.browser.test.ts#observe route — re-auth landing recovery (Codex round-3 HIGH/MEDIUM) (3 cases, unchanged in intent)"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/observe/observe.browser.test.ts#observe route — deleting the selected record never flashes a NotFound banner (WR-04)"
        status: pass
    human_judgment: false
  - id: D3
    description: "DSYS-01: engram-console-conventions/SKILL.md exists with frontmatter, records tokens/state-word order/scope-chip semantics/classifier contract/honest-feedback rule/keyboard model, and every cited repository path and token matches the shipped code"
    requirement: "DSYS-01"
    verification:
      - kind: other
        ref: "task-level acceptance_criteria: name frontmatter, 'archived › superseded › expired › scheduled' string, every --cat-*/--surface-2/--text-faint/--warning token present in both the skill and ui/src/app.css, 0 SPDX occurrences"
        status: pass
      - kind: other
        ref: "plan verify: for-loop over cited ui|internal|proto|cmd paths in both skills — every path exists"
        status: pass
    human_judgment: false
  - id: D4
    description: "DSYS-02: engram-connect-client/SKILL.md exists with frontmatter and a per-RPC contract table for SearchMemories/ListMemories/GetMemory/ListScopes, the engram/engramWrite split, CSRF, query-key and resume-envelope contracts"
    requirement: "DSYS-02"
    verification:
      - kind: other
        ref: "task-level acceptance_criteria: name frontmatter, 4 distinct RPC names present, 5 distinct terms (engramWrite/CSRF/rawMessage/keepPreviousData/signal) present, 0 SPDX occurrences"
        status: pass
    human_judgment: false
  - id: D5
    description: "SC7: this phase's 02-UI-SPEC.md names both skills, and CLAUDE.md routes to both so later UI phases find them"
    verification:
      - kind: other
        ref: "plan verify: both skill names appear in 02-UI-SPEC.md and in CLAUDE.md"
        status: pass
    human_judgment: false
  - id: D6
    description: "The vendored SPA in internal/webauth/static is rebuilt from ui/ once for the whole phase and matches a fresh task ui:build, and the chromedp console e2e still renders the seeded record on / and /observe"
    verification:
      - kind: other
        ref: "task ui:build followed by git status --porcelain -- internal/webauth/static (clean after the commit)"
        status: pass
      - kind: e2e
        ref: "internal/e2e/console_browser_test.go#TestConsoleBundleRendersRecordInBrowser (ENGRAM_REQUIRE_BROWSER=1)"
        status: pass
    human_judgment: false
  - id: D7
    description: "Full phase gates green in one run: UI suite, Go suite (Qdrant required), chromedp e2e, lint, fmt, license, proto lint"
    verification:
      - kind: unit
        ref: "pnpm --dir ui test — 47 files / 505 tests"
        status: pass
      - kind: integration
        ref: "ENGRAM_REQUIRE_QDRANT=1 go test ./... -count=1 — all packages ok"
        status: pass
      - kind: other
        ref: "task lint && task fmt:check && task license:check && task proto:lint"
        status: pass
    human_judgment: false

duration: 23min
completed: 2026-09-26
status: complete
---

# Phase 2 Plan 10: Shared List/Pane on / and /observe, Two Design-System Skills, Phase-Close Gates Summary

**`/` and `/observe` now render through the same `ResultsList` + `RecallSplit` + `DetailPane` set as `/search`; two new project-local skills (`engram-console-conventions`, `engram-connect-client`) document the shipped tokens, state model, classifier, client contract and per-RPC shapes for every later UI phase; the vendored console SPA is rebuilt and every phase gate — UI suite, Go suite, chromedp e2e, lint/fmt/license/proto — is green.**

## Performance

- **Duration:** 23 min
- **Started:** 2026-09-26T23:26:00Z
- **Completed:** 2026-09-26T23:49:12Z
- **Tasks:** 3 (1 tracer, 2 auto)
- **Files modified:** 7 (2 created, 5 modified) plus a 43-file vendored SPA rebuild

## Accomplishments

- `/` drops `MemoryList`/`MemoryRow` for the shared `ResultsList` (mode `unranked`) rendering its
  cross-spine recent feed, keeping the exact heading, `loading scopes…` text, and scope tiles the
  e2e depends on; a `recentQ` failure now reads "Could not load recent memories" rather than
  colliding with the e2e's "failed to load" negative-assertion wording.
- `/observe` drops its `Resizable.PaneGroup` + `MemoryList`/`MemoryDetail` pair for
  `RecallSplit` wrapping `ResultsList` (mode `unranked`, labeled `Memories in <scope>`) and
  `DetailPane`, keeping `ScopesSidebar`, offset `Pagination`, and every write path through
  `WriteSurfaces` — relocated to a stable toolbar row outside `RecallSplit`'s snippets to avoid
  the narrow/wide remount pitfall plan 02-09 already documented for `/search`.
- `.claude/skills/engram-console-conventions/SKILL.md`: category tokens, the
  `archived › superseded › expired › scheduled` state-word order and dim-iff-past rule, scope-chip
  semantics (readable-record counts, `~` for approximate), the id/short_id/text classifier
  contract, the honest-feedback rule, and the full keyboard model — every fact and path checked
  against the shipped code.
- `.claude/skills/engram-connect-client/SKILL.md`: the `engram`/`engramWrite` client split, the
  CSRF double-submit contract, query-key conventions (RPC-name-first keys, index-3 visibility
  slot), the re-auth resume envelope and its allowed destinations, and a per-RPC contract table
  for `SearchMemories`/`ListMemories`/`GetMemory`/`ListScopes`.
- `CLAUDE.md`'s Conventions section gained two routing bullets beside the existing spike/sketch
  ones, so later phases (Phase 3 RPCs, Phase 4 curation surfaces, Phase 5 related-memories/tags)
  find both skills.
- `internal/webauth/static` rebuilt via `task ui:build` and proven reproducible — a second build
  after the commit produces a byte-identical tree (`git status --porcelain` empty).
- Full phase gate run: UI suite (47 files / 505 tests), Go suite with Qdrant required (all 27
  packages `ok`, including `internal/store` at 173s and `internal/server` at 61s), the chromedp
  `TestConsoleBundleRendersRecordInBrowser` e2e (explicit `ENGRAM_REQUIRE_BROWSER=1` run, PASS),
  `task lint`, `task fmt:check`, `task license:check`, `task proto:lint` — all green in one pass.

## Task Commits

Each task was committed atomically:

1. **Task 1 (tracer): / and /observe on the shared list and pane, tests updated, e2e hooks preserved** - `bd789d40` (feat)
2. **Task 2: engram-console-conventions and engram-connect-client skills, routed from CLAUDE.md** - `8c0ab49f` (docs)
3. **Task 3: Vendor the SPA once and run every phase gate, including the chromedp console e2e** - `feac3db6` (build)

**Plan metadata:** commit follows this SUMMARY.

## Files Created/Modified

- `ui/src/routes/+page.svelte` - recent feed on `ResultsList` (mode unranked), restyled scope tiles on the phase-2 foundation tokens
- `ui/src/routes/page.browser.test.ts` - new listbox/heading/navigation assertions for the D-10 recent feed
- `ui/src/routes/observe/+page.svelte` - `RecallSplit` + `ResultsList` + `DetailPane`, `WriteSurfaces` moved to a stable toolbar row, `select a scope` guidance restored
- `ui/src/routes/observe/observe.browser.test.ts` - edit-flow rewritten to the `e` row-action key, WR-04 delete flow rewritten to `DetailPane`'s own Delete button + dialog-scoped confirm, new D-12 listbox/toggle tests
- `CLAUDE.md` - two new skill-routing bullets
- `.claude/skills/engram-console-conventions/SKILL.md` - new
- `.claude/skills/engram-connect-client/SKILL.md` - new
- `internal/webauth/static/**` - rebuilt vendored SPA (43 files)

## Decisions Made

See `key-decisions` in the frontmatter above — WriteSurfaces relocation (Rule 1 bug-prevention,
consistent with plan 02-09's precedent), the recentQ error-copy change (explicit plan
instruction), and the two test-selector rewrites forced by ResultsList/DetailPane's markup no
longer carrying a "row actions"/"record actions" menu.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Moved WriteSurfaces outside RecallSplit's snippets on /observe**
- **Found during:** Task 1 (/observe restyle)
- **Issue:** The plan's action text described replacing "the current Resizable.PaneGroup
  list-and-detail block" with `RecallSplit`, and the pre-existing `/observe` code placed
  `WriteSurfaces` inside that same pane (a header row above the list). Leaving it there would put
  `WriteSurfaces` inside `RecallSplit`'s `list()` snippet, which plan 02-09 already documented as
  destroying and recreating a snippet's content (and therefore `WriteSurfaces`' `bind:this`
  reference and its one-shot `onMount` resume-restore) the first time `RecallSplit`'s
  `ResizeObserver`-driven narrow/wide layout switch fires.
- **Fix:** Relocated the `WriteSurfaces` host to a stable toolbar row directly above `RecallSplit`
  (outside its `list()`/`detail()` snippets), mirroring `/search`'s own layout exactly.
- **Files modified:** `ui/src/routes/observe/+page.svelte`
- **Verification:** All re-auth landing recovery tests (edit-mode, create-mode, kind-mismatch)
  and the WR-04 delete-flow test pass unchanged in intent.
- **Committed in:** `bd789d40` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug-prevention, matching a precedent already documented in
plan 02-09). **Impact:** No scope creep — the plan's own interfaces block cited plan 02-09's
identical pitfall for `/search`; applying the same fix to `/observe` is direct, in-scope
correctness work, not a new decision.

## Known Stubs

None introduced by this plan. Carried forward from `02-08-SUMMARY.md`/`02-09-SUMMARY.md` (still
open, files outside this plan's `files_modified`): on `/search`, when `?q=<uuid>` auto-opens the
detail pane on the resolved record (no explicit `sel` in the URL), clicking the pane's close
button does not fully collapse it, because the pane's open state falls back to the auto-resolved
id whenever the URL's own `sel` is empty. This plan did not touch `/search`'s `autoOpenId`/
`effectiveSel` derivations, so the rough edge is unchanged.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 2 (Recall-First Search) is complete: all 10 plans executed, every phase gate (UI suite,
  Go suite with Qdrant, chromedp e2e, lint/fmt/license/proto) green in this plan's single run.
- `.claude/skills/engram-console-conventions/SKILL.md` and `.claude/skills/engram-connect-client/SKILL.md`
  are ready for Phase 3 (Curation RPCs & MCP Tools), Phase 4 (Curation Surfaces), and Phase 5
  (Related-Memories Graph & Tag Cloud) to cite instead of re-deriving these facts — both are
  routed from `CLAUDE.md`.
- `requirements.mark-complete` continues to refuse this milestone's `Mapped` REQUIREMENTS.md rows
  (a pre-existing, milestone-wide condition — `WINDOWS.md` id 17, first recorded in
  `02-09-SUMMARY.md`, not caused by this plan). `DSYS-01`, `DSYS-02`, `ROW-01`, and `ROW-07` are
  recorded in this SUMMARY's `requirements-completed` per the standing workaround; REQUIREMENTS.md
  was not hand-edited.
- No blockers.

---
*Phase: 02-recall-first-search*
*Completed: 2026-09-26*

## Self-Check: PASSED

- Created files exist: `.claude/skills/engram-console-conventions/SKILL.md`,
  `.claude/skills/engram-connect-client/SKILL.md` — both found.
- Modified files exist: `ui/src/routes/+page.svelte`, `ui/src/routes/observe/+page.svelte`,
  `ui/src/routes/page.browser.test.ts`, `ui/src/routes/observe/observe.browser.test.ts`,
  `CLAUDE.md`, `internal/webauth/static/index.html` — all found.
- Commits exist in `git log --oneline --all`: `bd789d40` (Task 1), `8c0ab49f` (Task 2),
  `feac3db6` (Task 3).
- Task 1 acceptance criteria re-verified: `engram — operator console` count 1, `loading scopes`
  count 1 in `+page.svelte`; `MemoryList|MemoryDetail` count 0 across both routes;
  `ScopesSidebar` count 3 in `observe/+page.svelte`.
- Task 1 `<verify>` re-run: `pnpm --dir ui vitest run --project browser
  src/routes/page.browser.test.ts src/routes/observe/observe.browser.test.ts` — 2 files / 19
  tests pass, `Test Files  2 passed` present.
- Task 2 acceptance criteria re-verified: both `name:` frontmatter lines present exactly once;
  `archived › superseded › expired › scheduled` present; all 9 tokens
  (`--cat-convention/--cat-gotcha/--cat-decision/--cat-preference/--cat-discovery/--cat-rule/--surface-2/--text-faint/--warning`)
  present in both skills and `ui/src/app.css`; 4 distinct RPC names and 5 distinct terms present
  in the connect-client skill; 0 SPDX occurrences in either skill.
- Task 2 `<verify>` re-run: the cited-path loop (`skills-ok`) and the UI-SPEC/CLAUDE.md
  double-citation check (`cited-ok`) both pass.
- Task 3 acceptance criteria re-verified: `internal/webauth/static/index.html` exists; all five
  Task 3 `<verify>` commands (vendored-clean, UI suite, Go suite with Qdrant, chromedp e2e
  explicit run, lint/fmt/license/proto) pass in one run, recorded above.
- Plan-level `<verification>` re-run: Task 3's five commands pass together (this is the same run
  recorded in Task 3's own verify; no separate re-run needed since nothing changed after they
  last ran).
