---
phase: 04-curation-surfaces
plan: 04
subsystem: ui
tags: [svelte, tanstack-query, connect-rpc, curation, supersession]

requires:
  - phase: 03-curation-rpcs-mcp-tools
    provides: RelatedMemories Connect RPC (oneof edge evidence, supersession direction/depth)
provides:
  - Chain dialog (D-06) rendering a record's supersession history, oldest-left to head-right
  - Pure chain model (chain.ts) consumed by 04-08's curation host wiring
affects: [04-08-curation-host-wiring]

actuals:
  tokens: 8618
  tasks: 2
  commits: 4

plan_head_before: 5b896d536dfd0a96201cc95c338ce3d1a42b167c
plan_head_after: 0e7dce38b5c51013f12494d93d618fc6da039b99

tech-stack:
  added: []
  patterns:
    - "Pure model / rendering split: buildChain/headIdFrom in chain.ts, ChainDialog.svelte renders only"
    - "Two-query dependent fetch: anchor RelatedMemories -> headIdFrom -> conditional second RelatedMemories for the head, reusing the first response when the anchor is already the head"
    - "Per-node click-to-peek via a single peekId $state and a getMemory createQuery keyed on it"

key-files:
  created:
    - ui/src/lib/curation/chain.ts
    - ui/src/lib/curation/chain.test.ts
    - ui/src/lib/components/ChainDialog.svelte
    - ui/src/lib/components/ChainDialog.browser.test.ts
  modified: []

key-decisions:
  - "beyondCap reads headResp.anchor.supersededBy (the returned head's own forward pointer), not a separate cap-detection query — the 8-hop server cap is already implied by the head still carrying a successor"
  - "Chain node's superseded_by peek line shows the raw predecessor/successor id (DetailPane's own precedent for supersededBy links), not a resolved short_id — no second lookup exists to resolve it"
  - "Placeholder ids are synthesized once per readable node's own supersedes list (deduplicated across nodes) rather than by a second RelatedMemories read, since the E3 partial truth only requires depth/order honesty, not fetching the missing record"

requirements-completed: [CUR-01]

coverage:
  - id: D1
    description: "ChainDialog renders a real supersession chain (oldest-left to head-right columns) from two RelatedMemories reads, with the anchor's own node highlighted"
    requirement: "CUR-01"
    verification:
      - kind: unit
        ref: "ui/src/lib/curation/chain.test.ts#buildChain places head at depth 0 and predecessors at their edge depth, oldest-first columns"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/ChainDialog.browser.test.ts#renders the header, three columns d2/d1/head·d0, and p1 highlighted"
        status: pass
    human_judgment: false
  - id: D2
    description: "CUR-01 ordering: nodes within a column sort by createdAt ascending, id ascending on a tie — the same input always renders the same order"
    requirement: "CUR-01"
    verification:
      - kind: unit
        ref: "ui/src/lib/curation/chain.test.ts#two depth-1 nodes are ordered by createdAt ascending, then id ascending on a tie"
        status: pass
    human_judgment: false
  - id: D3
    description: "Every chain node is a button; clicking it shows an inline fetch-by-id peek (get_memory ... · fetch-by-id ignores the recall gate · superseded_by ...), loading per-node, NotFound handled"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/ChainDialog.browser.test.ts#clicking a node shows a loading line on that node only, then the peek copy"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/ChainDialog.browser.test.ts#a peek NotFound renders the not-found copy inline"
        status: pass
    human_judgment: false
  - id: D4
    description: "E3 partial: a predecessor id in a readable node's supersedes but absent from the RelatedMemories result renders as a not-clickable placeholder carrying the not-found copy at its own depth"
    verification:
      - kind: unit
        ref: "ui/src/lib/curation/chain.test.ts#a readable depth-1 node whose supersedes lists an id absent from the result yields a placeholder node at depth 2"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/ChainDialog.browser.test.ts#a predecessor absent from the RelatedMemories result renders as a not-clickable placeholder carrying the not-found copy"
        status: pass
    human_judgment: false
  - id: D5
    description: "E3 empty/zero-one-many: a head-only response renders one column [[head]]; a depth-1 chain renders two columns, never a single-node special case"
    verification:
      - kind: unit
        ref: "ui/src/lib/curation/chain.test.ts#a head-only response (no predecessors) yields one column [[head]]"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/ChainDialog.browser.test.ts#a depth-1 chain (one predecessor, one head) still renders two columns"
        status: pass
    human_judgment: false
  - id: D6
    description: "Footer 'Supersede head…' renders only when the head is neither a rule nor a discovery record, and calls onsupersedehead(headId)"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/ChainDialog.browser.test.ts#appears for a convention head and calls onsupersedehead(headId)"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/ChainDialog.browser.test.ts#is absent when the head is a rule record"
        status: pass
    human_judgment: false
  - id: D7
    description: "E3 overflow/long-text: the column strip scrolls horizontally inside a scroll-area and never wraps; node summaries ellipsize and the subline wraps rather than widening the dialog"
    verification: []
    human_judgment: true
    rationale: "Layout/overflow behavior (CSS ellipsis, scroll-area horizontal scroll, no dialog-width growth) is visual and not asserted by the vitest-browser suite in this plan — needs a human/visual check, deferred to phase-level UAT or DSYS-03's audit pass."

duration: 20min
completed: 2026-09-27
status: complete
---

# Phase 4 Plan 4: Chain Dialog Summary

**The Chain dialog (D-06) renders a record's full supersession history — oldest-left to head-right columns, per-node fetch-by-id peek, placeholder nodes for unreadable predecessors, and a "Supersede head…" entry — built from a pure `chain.ts` model over two Phase 3 RelatedMemories reads.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-09-27T13:32:00Z (approx.)
- **Completed:** 2026-09-27T13:52:40Z
- **Tasks:** 2 completed
- **Files modified:** 4 (all created)

## Accomplishments

- `ui/src/lib/curation/chain.ts`: pure `buildChain`/`headIdFrom`/`compareChainNodes` model — groups a head's RelatedMemories predecessors by supersession depth, sorts each column (createdAt asc, id asc tiebreak), synthesizes placeholder nodes for unreadable predecessors, and reports `beyondCap` from the head's own `supersededBy`.
- `ui/src/lib/components/ChainDialog.svelte`: the Chain dialog — two dependent RelatedMemories queries (anchor then head, reused when the anchor is already the head), per-node click-to-peek via `GetMemory`, a subline summarizing the anchor's depth/predecessor/successor counts, a horizontally-scrolling column strip, and a gated `Supersede head…` footer.
- Every E3 UI consideration from the plan (empty, loading, error, populated, partial, overflow-scroll, zero-one-many, long-text-ellipsis) is either directly tested or, for pure-CSS overflow/ellipsis behavior, flagged for human/visual follow-up (coverage D7).

## Task Commits

Each task was committed atomically (Task 2 as a full RED → GREEN → REFACTOR cycle, tdd="true"):

1. **Task 1: Chain model from RelatedMemories and a dialog that renders a real three-node chain (tracer)** - `6d05f858` (feat)
2. **Task 2: Peek by id, unreadable-predecessor placeholders, subline, beyond-cap note, Supersede head…, every E3 state** - `93efae52` (test/RED), `92fae3de` (feat/GREEN), `0e7dce38` (refactor)

**Plan metadata:** (this commit)

## Files Created/Modified

- `ui/src/lib/curation/chain.ts` - pure chain model (buildChain, headIdFrom, compareChainNodes, ChainNode/ChainModel types)
- `ui/src/lib/curation/chain.test.ts` - 12 node-project unit tests over the pure model
- `ui/src/lib/components/ChainDialog.svelte` - the Chain dialog component
- `ui/src/lib/components/ChainDialog.browser.test.ts` - 8 vitest-browser tests (including 3 `page.screenshot()` captures for populated/peek/placeholder states)

## Decisions Made

- `beyondCap` is read straight off `headResp.anchor.supersededBy` — if the record the forward SUCCESSOR walk landed on still has its own `supersededBy` set, the walk stopped at the server's 8-hop cap rather than genuinely finding no successor. No extra query needed.
- The peek line's `superseded_by {...}` slot shows the raw predecessor/successor id, matching `DetailPane.svelte`'s existing precedent of rendering `supersededBy`/`supersedes` as raw ids (there is no secondary lookup in this dialog to resolve a linked id to its `short_id`).
- Placeholder synthesis walks each *readable* node's own `supersedes` list once (a `Set` dedupes across nodes) rather than issuing a further RelatedMemories read — the E3 partial truth only requires that depth and column order stay honest, not that the unreadable record be fetched.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `@vitest/browser/context` throws at import time in this project's Vitest 5 setup**
- **Found during:** Task 2 (writing the `page.screenshot()` calls)
- **Issue:** `import { page } from '@vitest/browser/context'` threw `"vitest/browser can be imported only inside the Browser Mode... Your test is running in browser pool"` — that package's `context.js` is a static-analysis stub in this Vitest/Vite version; the real browser-mode module is the `vitest/browser` virtual specifier.
- **Fix:** Changed the import to `import { page } from 'vitest/browser'`, confirmed against `vitest`'s own `package.json` exports map (`"./browser"` → `browser/context.js`) and Context7's fetched Vitest docs.
- **Files modified:** `ui/src/lib/components/ChainDialog.browser.test.ts`
- **Verification:** All 8 browser tests pass, including the 3 screenshot captures.
- **Committed in:** `93efae52` (the RED test commit already carries the corrected import)

**2. [Rule 3 - Blocking] Generated `page.screenshot()` PNGs would have landed as untracked files beside the test**
- **Found during:** Task 2 verification (post-run `git status`)
- **Issue:** A bare `path: 'chain-dialog-populated.png'` resolves relative to the test file's own directory (per Vitest's `resolveScreenshotPath`), landing three PNGs directly in `ui/src/lib/components/` — untracked, non-gitignored files.
- **Fix:** Moved all three screenshot paths under `__screenshots__/`, which `.gitignore`'s existing `ui/**/__screenshots__/` rule already covers (the project's established screenshot-artifact convention, per `CONTEXT.md`'s "vitest-browser tests with screenshots beside each component (`__screenshots__/`)").
- **Files modified:** `ui/src/lib/components/ChainDialog.browser.test.ts`
- **Verification:** `git status --short` shows no untracked PNGs after a test run.
- **Committed in:** `93efae52`

### Notes (non-blocking, documented rather than fixed)

**3. TDD gate: `gsd_run check tdd-red-evidence` is not runnable against this stack.** Task 2 carries `tdd="true"` and RED/GREEN/REFACTOR were performed genuinely (tests written first, run, confirmed to fail on the correct target assertion — not a crash or zero-test-discovery — then implemented to GREEN). The tool that machine-verifies RED evidence (`gsd_run check tdd-red-evidence`) parses Node's `node --test` TAP summary lines (`# tests N` / `# pass N` / `# fail N`); Vitest's own `--reporter=tap` output (confirmed live) does not emit those summary lines, so every invocation would misclassify as `zero_tests_discovered` regardless of the real RED/GREEN state. RED was instead evidenced by direct observation of two full `vitest run` executions, captured in the `test(04-04)` commit message: the two new `chain.ts` cases (placeholder synthesis, `beyondCap: true`) failed with a real `AssertionError` on the correct target test; five new `ChainDialog.browser.test.ts` cases timed out locating markup absent from Task 1's dialog. This mirrors the project's own documented precedent (`svelte-check`/TypeScript environment gaps in `ui/`, per `STATE.md`'s Deferred Items) of substituting a working direct check when a gate tool doesn't cover the frontend stack. See `## TDD Gate Compliance` below.

**4. [Style] `test(04-04):`/`refactor(04-04):` commit scope diverges from this repo's `ui/`-scope convention.** The plan's own action text specifies the feat commit literally as `feat(ui): ...` (matching every prior `ui/`-touching commit in this repo's history — `feat(ui):`, `fix(ui):`, `test(ui):`). The mandatory RED and REFACTOR commits the TDD cycle requires for Task 2 weren't given a literal message by the plan, so they were authored as `test(04-04): ...` / `refactor(04-04): ...` (GSD's default phase-plan scope) rather than `test(ui): ...` / `refactor(ui): ...`. Cosmetic only — both are syntactically valid Conventional Commits and CI validates only the PR title, not per-commit scope — left as-is per the "never amend a commit" rule rather than rewritten.

---

**Total deviations:** 2 auto-fixed (both Rule 3 — blocking tooling/environment issues), 2 documented notes (one tooling-coverage gap, one cosmetic scope-naming inconsistency).
**Impact on plan:** No scope creep; both auto-fixes were required to get `page.screenshot()` working and keep the working tree clean. Neither note blocks the dialog's correctness — TDD discipline was followed in substance (real RED, real GREEN, real REFACTOR), verified by direct test execution instead of the incompatible red-evidence tool.

## TDD Gate Compliance

Task 2 (`tdd="true"`) RED → GREEN → REFACTOR sequence, verified by commit + direct test-run observation (not `gsd_run check tdd-red-evidence` — see Deviations note 3 above, tool/stack incompatibility, not a process skip):

| Gate | Commit | Evidence |
|------|--------|----------|
| RED | `93efae52` | `pnpm --dir ui vitest run --project node src/lib/curation/chain.test.ts` → 2 of 12 tests failed with `AssertionError` on the target assertions (`toHaveLength(3)` got `2`; `beyondCap` expected `true` got `false`). `pnpm --dir ui vitest run --project browser src/lib/components/ChainDialog.browser.test.ts` → 5 of 8 tests failed on `Matcher did not succeed in time` (markup genuinely absent), 3 passed (pre-existing behavior, not the target of this RED). |
| GREEN | `92fae3de` | Same two commands → 12/12 and 8/8 pass. |
| REFACTOR | `0e7dce38` | `{@const peeked = ...}` cleanup; re-ran both commands → 12/12 and 8/8 still pass, no behavior change. |

## Issues Encountered

None beyond the two auto-fixed items above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `ui/src/lib/curation/chain.ts` and `ChainDialog.svelte` are ready for plan 04-08 to wire into the curation host and `/search` (row Chain action, pane's history links).
- `ChainDialog`'s `open`/`anchorId`/`onsupersedehead`/`oncancel` props match `DeleteConfirmDialog`'s host-authoritative pattern that 04-08's other dialogs also follow.
- Coverage D7 (overflow/ellipsis CSS behavior) is flagged `human_judgment: true` for phase-level UAT or the DSYS-03 audit pass — no automated coverage exists for that specific visual property in this plan.

---
*Phase: 04-curation-surfaces*
*Completed: 2026-09-27*

## Self-Check: PASSED

- Files: `ui/src/lib/curation/chain.ts`, `ui/src/lib/curation/chain.test.ts`, `ui/src/lib/components/ChainDialog.svelte`, `ui/src/lib/components/ChainDialog.browser.test.ts`, this SUMMARY.md — all present on disk.
- Commits: `6d05f858`, `93efae52`, `92fae3de`, `0e7dce38`, `311709a1` — all present in `git log`.
- Acceptance criteria re-run: Task 1's three `rg` checks and Task 2's three `rg` checks all print the expected counts (verified above, in-flow).
- Plan-level `<verification>` (both tasks' `<verify>` commands) re-run clean: `chain.test.ts` 12/12 pass, `ChainDialog.browser.test.ts` 8/8 pass.
