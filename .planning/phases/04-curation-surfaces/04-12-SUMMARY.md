---
phase: 04-curation-surfaces
plan: 12
subsystem: testing
tags: [chromedp, e2e, headless-chrome, archive, supersede, entry-point-resolution, ui-build]

# Dependency graph
requires:
  - phase: 04-curation-surfaces
    provides: "04-08/04-09/04-10 (every curation surface this phase ships); 04-11 (WCAG 2.2 AA audit + fixes, D-17 keyboard model)"
provides:
  - "internal/e2e/console_browser_test.go: TestConsoleArchiveUndoRoundTrip, TestConsoleSupersedeRoundTrip, TestConsoleEntryPointResolution -- three chromedp round trips against a live engram binary + Qdrant + real headless Chrome"
  - "internal/webauth/static/**: the console SPA rebuilt once from ui/ for the final state of Phase 4 (matches a fresh task ui:build byte-for-byte)"
  - "Live proof that the full phase gate (ui suite, Go suite, every chromedp e2e, lint/fmt/license/proto) is green in one run"
affects: []

# Actuals (#2632)
actuals:
  tokens: 6900
  tasks: 3
  commits: 3
  plan_head_before: 90e4779a9ec526f1de9285cd88f84b8ebe016b7f
  plan_head_after: 7dbb3fad609d861388ecdac6f8551255bc7bff59

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "chromedp's default headless window (observed ~756x413) is short enough to put a wide dialog's footer buttons below the fold; a coordinate-based chromedp.Click on such a button silently lands outside the viewport and does nothing (no error, no RPC, the dialog just never advances). Fix: pass chromedp.WindowSize(w, h) in the ExecAllocatorOption list for any test that opens a tall dialog."
    - "A component that opens a popover from an `onfocus` prop (bits-ui's CommandPrimitive.Input, used by HeaderSearch.svelte) does not reliably see that focus via chromedp.Focus's raw CDP DOM.focus() command -- the Svelte listener never fires and the popover never opens. chromedp.Click (a real dispatched mouse click: mousedown -> focus -> click) does trigger it. When a focus-driven UI transition fails to happen after chromedp.Focus, try chromedp.Click on the same selector before assuming the app is broken."
    - "Typing into a bits-ui Command input character-by-character via chromedp.SendKeys can lose keystrokes: the Command root's own CommandInputState re-focuses the input via `afterSleep(10, () => node.focus())` on every list-affecting update (command.svelte.js), and that steal can land between two of chromedp's per-character CDP round trips, redirecting subsequent keydowns away from the field entirely (ending with an empty .value). Fix: write the field's value in one atomic step -- the native HTMLInputElement.prototype.value setter plus one dispatched bubbling 'input' event -- rather than character-by-character SendKeys, whenever the target is a bits-ui Command/cmdk-style input."
    - "getMemoryAsFixture(ctx, t, fixture, id) generalizes the e2e package's cookie-authenticated Connect read pattern (GetMemory needs the session cookie only, no CSRF header, unlike the write lane's StoreMemory)."

key-files:
  created: []
  modified:
    - internal/e2e/console_browser_test.go
    - internal/webauth/static/** (vendored SPA rebuild; index.html + hashed asset filenames only, no ui/ source changed)

key-decisions:
  - "seedFixtureRecord generalized to (ctx, t, fixture, scope, marker) (id, shortID string) exactly as the plan's interfaces block specified, with the sole existing caller (TestConsoleBundleRendersRecordInBrowser) updated to pass the pre-existing fixtureScope constant -- no behavior change for that test."
  - "Each of the three new tests seeds its own record under its own dedicated scope (repo:e2e-console-archive, repo:e2e-console-supersede, repo:e2e-console-entry) with a fresh crypto/rand marker, per DSYS-04 ordering -- none depends on another test's state or execution order, and all four chromedp tests in the package (including the pre-existing one) now pass together in one run or individually."
  - "Click targets inside a dialog are scoped with an XPath rooted at //div[@role=\"dialog\"] (e.g. //div[@role=\"dialog\"]//button[normalize-space()=\"Archive\"]) rather than a bare //button[...] -- RowActions' per-row hover toolbar and ResultsHeader's bulk-selection bar both render buttons with overlapping text (\"Archive\", a Supersede action) OUTSIDE the dialog's DOM subtree, so an unscoped XPath risked ambiguity even though neither was actually visible during either test run."
  - "uiTextPollExpr generalizes the existing markerPollExpr pattern (document.body.innerText.includes(text)) for ordinary UI copy (dialog headings, result lines) rather than reusing markerPollExpr itself, since markerPollExpr's own doc comment specifically asserts the text is NOT bundle copy -- that invariant would have been false for a dialog heading."
  - "detailPaneMarkerPollExpr and submitButtonEnabledPollExpr were added as small, purpose-built poll expressions (scoped to aside[aria-label=\"Memory detail\"] and a dialog-scoped disabled-button check respectively) instead of overloading uiTextPollExpr for those two more specific waits."
  - "TestConsoleSupersedeRoundTrip waits for SupersedeDialog's primary button to become enabled (submitButtonEnabledPollExpr) BEFORE clicking it, rather than clicking immediately after the dialog opens -- canSubmit gates on the debounced (250ms) validate_only preview actually resolving, not merely on the dialog being open, and clicking a still-disabled button is silently a no-op in a real browser (no error, no click landing)."
  - "TestConsoleEntryPointResolution's third leg (header search + Enter) polls for the header's own \"-> 1 memory\" resolution text before pressing Enter, confirming HeaderSearch's getMemory-by-short_id query has actually resolved and rendered its single \"Memories\" command item -- pressing Enter before that would select nothing (or nothing deterministic)."
  - "Neither of the two chromedp gotchas above (window size, focus-vs-click) was routed around by loosening an assertion -- both are fixed at the automation layer (allocator options, event dispatch mechanics), and the plan's own e2e expectations (dialog result text, ArchivedAt/SupersededBy state, resolution line copy) are unchanged from what the plan specified."

requirements-completed: [DSYS-04, CUR-01, CUR-02]

coverage:
  - id: D1
    description: "DSYS-04 tracer: a real headless Chrome archives a seeded record through the console (listbox 'a' shortcut + ArchiveConfirmDialog), the server agrees (ArchivedAt set), then the same dialog's 'Undo — restore N' result-body action restores it and the server agrees again (ArchivedAt cleared) — TestConsoleArchiveUndoRoundTrip"
    requirement: DSYS-04
    verification:
      - kind: e2e
        ref: "internal/e2e/console_browser_test.go#TestConsoleArchiveUndoRoundTrip"
        status: pass
    human_judgment: false
  - id: D2
    description: "DSYS-04/CUR-01: a real headless Chrome supersedes a seeded record through the listbox's Shift+S shortcut and SupersedeDialog's debounced validate_only preview + commit button; the predecessor's SupersededBy resolves to a successor whose content carries the edited draft — TestConsoleSupersedeRoundTrip"
    requirement: CUR-01
    verification:
      - kind: e2e
        ref: "internal/e2e/console_browser_test.go#TestConsoleSupersedeRoundTrip"
        status: pass
    human_judgment: false
  - id: D3
    description: "DSYS-04/CUR-01: a real headless Chrome resolves a seeded record by full UUID and by short_id through /ui/search?q=, and by short_id typed into the header search and entered from the root route — each rendering the record in the detail pane with the matching resolution line — TestConsoleEntryPointResolution"
    requirement: CUR-01
    verification:
      - kind: e2e
        ref: "internal/e2e/console_browser_test.go#TestConsoleEntryPointResolution"
        status: pass
    human_judgment: false
  - id: D4
    description: "The vendored SPA in internal/webauth/static is rebuilt once from ui/ with task ui:build and matches a fresh build (the ui-drift gate), for the final state of Phase 4"
    requirement: DSYS-04
    verification:
      - kind: other
        ref: "task ui:build && git status --porcelain -- internal/webauth/static (empty -> vendored-clean)"
        status: pass
    human_judgment: false
  - id: D5
    description: "Phase gate: the full ui suite (pnpm --dir ui test), the Go suite with Qdrant, every chromedp e2e with ENGRAM_REQUIRE_BROWSER=1, and task lint / fmt:check / license:check / proto:lint all pass in one run"
    requirement: DSYS-04
    verification:
      - kind: other
        ref: "pnpm --dir ui test (Test Files 59 passed, Tests 754 passed); ENGRAM_REQUIRE_QDRANT=1 go test ./... -count=1 (all ok, no FAIL); ENGRAM_REQUIRE_BROWSER=1 ENGRAM_REQUIRE_QDRANT=1 go test ./internal/e2e/ -run '^TestConsole' -v (4/4 PASS); task lint && task fmt:check && task license:check && task proto:lint (all exit 0)"
        status: pass
    human_judgment: false
  - id: D6
    description: "D-14 (phase level): no non-test file under ui/src references the deleted /observe route; every new-surface browser test file captures at least one screenshot (from 04-11's inventory, re-verified here)"
    requirement: DSYS-04
    verification:
      - kind: other
        ref: "rg -o -e '/observe' ui/src -g '!*.test.ts' -g '!**/gen/**' | wc -l -> 0; page.screenshot counts >=1 across ArchiveConfirmDialog/SupersedeDialog/ChainDialog/ResultsList/ResultsHeader/rules/scheduled/surfaces browser test files"
        status: pass
    human_judgment: false

# Metrics
duration: ~56min
completed: 2026-09-27
status: complete
---

# Phase 4 Plan 12: DSYS-04 live proof — archive/undo, supersede, and entry-point resolution against a real browser and server, full phase gate green Summary

**Three chromedp tests drive a real headless Chrome against the real engram binary and Qdrant — archive→undo, supersede, and UUID/short_id/header-search entry-point resolution — and the entire Phase 4 gate (UI suite, Go suite, all four console e2e tests, lint/fmt/license/proto) passes in one run.**

## Performance

- **Duration:** ~56 min
- **Started:** 2026-09-27T19:05:00Z (approx.)
- **Completed:** 2026-09-27T20:01:00Z
- **Tasks:** 3 of 3 completed (Task 1 tracer, Task 2 expansion, Task 3 phase-gate verification)
- **Files modified:** 2 (1 test file, 1 vendored build tree)

## Accomplishments

- `TestConsoleArchiveUndoRoundTrip` (Task 1, tracer): seeds a record under its own scope
  (`repo:e2e-console-archive`), navigates the scoped `/ui/search`, focuses the listbox, sends
  `a`, clicks the resulting `ArchiveConfirmDialog`'s Archive button, confirms via Connect that
  `ArchivedAt` is set for the same id, clicks the dialog's own "Undo — restore 1" result-body
  action, and confirms `ArchivedAt` is cleared again — the D-09 result-body undo surface proven
  end to end against a live server, not a mocked one.
- `TestConsoleSupersedeRoundTrip` (Task 2): seeds a record under `repo:e2e-console-supersede`,
  supersedes it through the listbox's Shift+S shortcut and `SupersedeDialog`'s debounced
  `validate_only` preview + "Supersede 1 → 1" commit, then confirms via Connect that the
  predecessor's `SupersededBy` resolves to a successor whose content carries the edited draft.
- `TestConsoleEntryPointResolution` (Task 2): seeds a record under `repo:e2e-console-entry` and
  resolves it three ways — full UUID via `/ui/search?q=<uuid>`, short_id via
  `/ui/search?q=<short_id>`, and short_id typed into the header search + Enter from the root
  route — asserting the record renders in `aside[aria-label="Memory detail"]` with the matching
  `resolutionLine` text each time.
- `seedFixtureRecord` generalized to `(ctx, t, fixture, scope, marker) -> (id, shortID)` exactly
  per the plan's interfaces block; `getMemoryAsFixture` added as the cookie-authenticated
  (no-CSRF) `GetMemory` read helper the three new tests' server-side assertions need.
- **Two real headless-Chrome automation bugs found and fixed, not routed around:**
  1. The default headless window (~756×413) put `SupersedeDialog`'s footer buttons below the
     fold; a coordinate-based click landed outside the viewport and silently did nothing.
     Fixed with explicit `chromedp.WindowSize(1280, 1000)` on the three new tests' allocator
     options.
  2. `HeaderSearch.svelte` opens its popover from an `onfocus` prop on bits-ui's
     `CommandPrimitive.Input`; `chromedp.Focus`'s raw CDP `DOM.focus()` command never triggered
     that Svelte listener (confirmed live — focus succeeded, `open` never became true).
     `chromedp.Click` (a real dispatched mouse click) does. Typing character-by-character via
     `chromedp.SendKeys` also lost keystrokes to bits-ui Command's own
     `afterSleep(10, () => node.focus())` re-focus racing CDP's per-character round trips; fixed
     by writing the field's value in one atomic native-setter-plus-`input`-event step.
- `task ui:build` rebuilds `internal/webauth/static` once more for the final state of Phase 4
  (no `ui/` source changed this plan; the vendored tree already matched a fresh build after
  Task 1's own rebuild — Task 3's `ui:build` re-run confirmed `vendored-clean` with zero diff).
- **Full phase gate, one run:** `pnpm --dir ui test` (Test Files 59 passed, Tests 754 passed);
  `ENGRAM_REQUIRE_QDRANT=1 go test ./... -count=1` (every package `ok`, zero `FAIL`, includes
  `internal/keylinks` satisfiability); `ENGRAM_REQUIRE_BROWSER=1 ENGRAM_REQUIRE_QDRANT=1 go test
  ./internal/e2e/ -run '^TestConsole'` (4/4 `--- PASS`); `task lint && task fmt:check && task
  license:check && task proto:lint` (all exit 0).

## Task Commits

Each task was committed atomically:

1. **Task 1 (tracer): archive/undo round trip + vendored SPA rebuild** — `44d216ac` (test),
   `f576cb4a` (build)
2. **Task 2: supersede round trip + entry-point resolution** — `7dbb3fad` (test)
3. **Task 3: phase-gate verification** — no commit (verification-only; `ui:build` re-run
   confirmed zero diff, so no `build(ui):` commit was needed)

**Plan metadata:** this commit (docs: complete plan)

## Files Created/Modified

- `internal/e2e/console_browser_test.go` — `TestConsoleArchiveUndoRoundTrip`,
  `TestConsoleSupersedeRoundTrip`, `TestConsoleEntryPointResolution`; generalized
  `seedFixtureRecord`; new `getMemoryAsFixture`, `uiTextPollExpr`, `detailPaneMarkerPollExpr`,
  `submitButtonEnabledPollExpr` helpers
- `internal/webauth/static/**` — rebuilt once from `ui/`'s current source (through plan 04-11's
  a11y fixes) via `task ui:build`

## Decisions Made

See `key-decisions` in the frontmatter for full detail. Summary: each new test seeds its own
scope with a fresh marker (DSYS-04 ordering); dialog click targets are XPath-scoped to
`//div[@role="dialog"]` to avoid ambiguity with RowActions'/ResultsHeader's own same-text
buttons outside the dialog; the two chromedp gotchas (window size, focus-vs-click) were fixed at
the automation layer rather than by loosening any assertion the plan specified.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] chromedp's default headless window put SupersedeDialog's footer buttons below the fold**
- **Found during:** Task 2, `TestConsoleSupersedeRoundTrip` — the click on "Supersede 1 → 1"
  returned no error but the dialog never advanced to its result view; debug instrumentation
  showed the button's computed rect (`top: 493`, `bottom: 523`) exceeded the actual
  `window.innerHeight` (413) chromedp's headless allocator was using by default.
- **Issue:** A coordinate-based CDP click at a node's computed box-model center silently lands
  outside the actual rendered viewport when that box sits below the fold, with no error surfaced
  anywhere (not a chromedp bug, not an app bug — a test-environment sizing gap).
- **Fix:** Added `chromedp.WindowSize(1280, 1000)` to the three new tests' `ExecAllocatorOption`
  list.
- **Files modified:** `internal/e2e/console_browser_test.go`
- **Verification:** `TestConsoleSupersedeRoundTrip` and `TestConsoleArchiveUndoRoundTrip` both
  pass reliably (verified across 3 consecutive runs) with the fix; reproduced the failure without
  it via live debug instrumentation (button rect vs. `window.innerHeight`) before fixing.
- **Committed in:** `7dbb3fad` (Task 2 commit)

**2. [Rule 3 - Blocking] chromedp.Focus's raw DOM.focus() did not trigger HeaderSearch's onfocus-driven popover open, and character-by-character SendKeys lost keystrokes to bits-ui's own re-focus**
- **Found during:** Task 2, `TestConsoleEntryPointResolution` leg 3 (header search + Enter) — the
  input's `.value` was empty and `document.activeElement` was a `popover-content` `<div>` after
  typing, confirmed via live debug instrumentation across three iterations (checking
  active-element identity, the popover's presence/`data-state`, and bits-ui's own
  `command.svelte.js` source for the `afterSleep(10, () => node.focus())` re-focus behavior).
- **Issue:** `HeaderSearch.svelte` opens its dropdown from an `onfocus={() => (open = true)}` prop
  on `CommandPrimitive.Input`; a bare `chromedp.Focus` call (CDP's `DOM.focus()`) never fired
  that listener (confirmed: `document.activeElement` correctly became the input, but the popover
  never mounted). Separately, typing via `chromedp.SendKeys` (one CDP round trip per keystroke)
  raced bits-ui Command's own periodic re-focus of the input, scattering/losing characters.
- **Fix:** Switched to `chromedp.Click` on the header search input (a real dispatched mouse click
  reliably triggers the Svelte listener) and set its value in one atomic step — the native
  `HTMLInputElement.prototype.value` setter plus a single dispatched bubbling `'input'` event —
  instead of per-character `SendKeys`.
- **Files modified:** `internal/e2e/console_browser_test.go`
- **Verification:** `TestConsoleEntryPointResolution` passes reliably (verified across 3
  consecutive runs); the specific failure mode (empty input value, focus stolen to the popover
  content div) was reproduced and root-caused via live debug instrumentation before the fix, then
  re-verified absent after it.
- **Committed in:** `7dbb3fad` (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 Rule 3 blocking issues, both chromedp/headless-Chrome
automation-layer gotchas, neither an application bug). **Impact on plan:** Both fixes were
necessary to make the plan's own specified assertions (dialog result text, `ArchivedAt`/
`SupersededBy` state, resolution-line copy) actually observable through real browser automation;
neither assertion itself was changed or loosened.

## Issues Encountered

`gsd-tools requirements mark-complete DSYS-04 CUR-01 CUR-02` (the `update_requirements` step)
reported all three `not_found`, despite each appearing exactly once in REQUIREMENTS.md.
Root-caused to a pre-existing, milestone-wide gap: REQUIREMENTS.md's traceability table Status
column reads `Mapped` for all 40 v1 requirements (every phase, not just this one), not the
`Pending`/`Complete` vocabulary the tool's template and `mark-complete` verb expect — confirmed
neither `- [x]` checkboxes nor `| Complete |` rows exist anywhere in the file, even though Phases
1-3 already shipped. Not fixed here: REQUIREMENTS.md is a tool-owned generated file, and this
repo's `planning-artifacts.md` rule forbids hand-editing structure a tool doesn't recognize into
it. Filed as `.planning/phases/04-curation-surfaces/deferred-items.md` and a `WINDOWS.md` entry
(kind `deviation`) rather than worked around. DSYS-04/CUR-01/CUR-02 are functionally satisfied
per this plan's own `coverage:` block regardless of the traceability table's stale display state.

## Known Stubs

None. All three new tests exercise real code paths against a live server, browser, and Qdrant;
no hardcoded or placeholder values were introduced.

## Threat Flags

None beyond the plan's own `<threat_model>` (T-04-28, T-04-29), both as specified: `internal/webauth/static` is proven to match a fresh `task ui:build` (T-04-28's stated mitigation, re-verified in Task 3); the e2e session/CSRF minting remains test-only with a per-run random cookie key and stub OIDC discovery document (T-04-29, unchanged from prior plans).

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- DSYS-04 is satisfied: the phase's curation surfaces are proven end to end against a real
  browser, real server, and real Qdrant — archive/undo, supersede, and entry-point resolution by
  UUID, short_id (URL), and short_id (header search + Enter).
- CUR-01 and CUR-02 are both satisfied by this plan's live proof (no mocked test could give this;
  see the plan's own `<objective>`).
- `internal/webauth/static` matches a fresh `task ui:build` — the ui-drift CI gate is green.
- The full Phase 4 gate (UI suite, Go suite with Qdrant, every console e2e, lint/fmt/license/proto)
  passed together in one run at plan close, per Task 3.
- Phase 04-curation-surfaces is complete: all 12 plans have summaries. No blockers for the
  milestone's next phase.

---
*Phase: 04-curation-surfaces*
*Completed: 2026-09-27*

## Self-Check: PASSED

- `internal/e2e/console_browser_test.go` verified present on disk and containing all three new
  test functions (`rg -o -e 'func TestConsole(ArchiveUndoRoundTrip|SupersedeRoundTrip|EntryPointResolution)' internal/e2e/console_browser_test.go | sort -u | wc -l` → 3).
- `internal/webauth/static` verified present on disk; `task ui:build` re-run confirms
  `git status --porcelain -- internal/webauth/static` is empty (vendored-clean).
- All 3 commits (`44d216ac`, `f576cb4a`, `7dbb3fad`) verified present via `git log --oneline`.
- Re-ran all three tasks' plan-level `<verify>` commands: Task 1's two automated checks (2/2 PASS
  lines for `TestConsoleArchiveUndoRoundTrip`/`TestConsoleBundleRendersRecordInBrowser`, plus
  vendored-clean), Task 2's automated check (2/2 PASS lines for
  `TestConsoleSupersedeRoundTrip`/`TestConsoleEntryPointResolution`), and Task 3's six commands
  (vendored-clean; `pnpm --dir ui test` 59/59 Test Files, 754/754 Tests; `go test ./...` all `ok`;
  4/4 `--- PASS` for `^TestConsole`; `task lint && task fmt:check && task license:check && task
  proto:lint` → `gates-ok`; screenshot inventory + zero `/observe` → `inventory-ok`) — all pass.
- Re-ran the acceptance-criteria `rg` checks for both Task 1 and Task 2 — all matched their
  required counts (`func TestConsoleArchiveUndoRoundTrip` → 1; `repo:e2e-console-archive` → ≥1;
  `func (TestConsoleSupersedeRoundTrip|TestConsoleEntryPointResolution)` → 2 unique;
  `input[.]ModifierShift` → ≥1).
- `go vet ./internal/e2e/...` and `gofmt -l internal/e2e/console_browser_test.go` both clean.
- Full package run confirmed stable: `ENGRAM_REQUIRE_BROWSER=1 ENGRAM_REQUIRE_QDRANT=1 go test
  ./internal/e2e/ -run '^TestConsole' -count=1 -v` passed 4/4 across two separate full runs in
  this session (no flakes observed after the two fixes landed).
