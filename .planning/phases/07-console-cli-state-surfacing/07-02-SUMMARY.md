---
phase: 07-console-cli-state-surfacing
plan: 02
subsystem: ui
tags: [svelte, badge, state-derivation, accessibility, connect-rpc-types]

# Dependency graph
requires:
  - phase: 07-console-cli-state-surfacing
    provides: "07-03's Search-lane vendored TS types (ui/src/lib/gen/engram/v1/engram_pb.ts already carries supersededBy/supersedes/notBefore/notAfter/archivedAt/schemaVersion from Phase 5) — this plan is a tooling-ordering dependent only, not a semantic one"
provides:
  - "ui/src/lib/memorystate.ts — the console's sole D-13 state-word derivation (memoryStateWords, isPastState, STATE_WORD_ORDER), independently tested from the Go surface's cmd/engram/memory_state.go"
  - "MemoryRow state badges (achromatic, canonical order) plus the dim-iff-past row treatment with a hard accessibility carve-out keeping the badge at full opacity"
  - "MemoryDetail's unconditional schema chip and conditional State section, including full-UUID successor/predecessor links wired through a new optional onselect prop"
  - "observe/+page.svelte wiring MemoryDetail's onselect to the existing navigate({ selectedId }) helper"
affects: []

# Actuals (#2632)
actuals:
  tokens: 7335
  tasks: 3
  commits: 7

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Per-element opacity-60 dimming (never on a shared ancestor) so a badge sitting inside a dimmed row stays at full opacity by construction — CSS opacity multiplies through descendants, so this is the only expressible way to hold the accessibility carve-out"
    - "State section gate is words-present OR non-empty supersedes (not words-present alone) — a pure successor record carries no state word of its own but still needs the section to list its predecessor links"

key-files:
  created:
    - ui/src/lib/memorystate.ts
    - ui/src/lib/memorystate.test.ts
  modified:
    - ui/src/lib/components/MemoryRow.svelte
    - ui/src/lib/components/MemoryRow.browser.test.ts
    - ui/src/lib/components/MemoryDetail.svelte
    - ui/src/lib/components/MemoryDetail.browser.test.ts
    - ui/src/routes/observe/+page.svelte

key-decisions:
  - "MemoryDetail's State-section gate is `stateWords.length > 0 || memory.supersedes.length > 0`, not just the state-word count the plan's action text led with — a record that only supersedes others (a pure successor, no archived/superseded/expired/scheduled fields of its own) still needs the section to render its predecessor links. Caught by a genuine RED test (two-supersedes-links) that failed against the words-only gate."
  - "Test date fixtures for MemoryDetail's expired/scheduled cases must be relative to real wall-clock time (Date.now()), not a fixed fictional date — memoryStateWords has no injectable `now` on the MemoryDetail call site (default param `new Date()`), unlike the Go surface's test which passes an explicit now. An initial fixture using a fixed 2030 'now' broke the expired case because 2030 is still in the *real* future."
  - "The schema chip's value assertion uses substring matching (`getByText('v3')`, not `{exact:true}`), matching how the existing by/src/vis chip tests assert their values — the chip's value text shares a text run with its label sibling span, so no element's own text content is 'v3' alone (Playwright's exact getByText requires the full recursive textContent of one element to equal the target)."

requirements-completed: [REQ-console-record-state]

coverage:
  - id: D1
    description: "ui/src/lib/memorystate.ts — the console's sole D-13 state-word derivation, independently tested from the Go surface"
    requirement: "REQ-console-record-state"
    verification:
      - kind: unit
        ref: "ui/src/lib/memorystate.test.ts#memoryStateWords / isPastState"
        status: pass
    human_judgment: false
  - id: D2
    description: "MemoryRow renders achromatic state badges in canonical order plus dim-iff-past, with the badge staying full opacity inside a dimmed row"
    requirement: "REQ-console-record-state"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/MemoryRow.browser.test.ts (live/archived/archived+superseded/scheduled/scheduled+archived cases)"
        status: pass
    human_judgment: true
    rationale: "Visual verification (achromatic badge styling, meta-line wrap under a long summary) was deferred to a held-out visual check per the plan's own long-text/E1 backstop resolution — automated tests prove structure and class membership, not pixel-level appearance."
  - id: D3
    description: "MemoryDetail renders an unconditional schema chip and a conditional State section with full-UUID successor/predecessor links wired to onselect"
    requirement: "REQ-console-record-state"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/MemoryDetail.browser.test.ts (schema chip, archived/expired/scheduled(+close), superseded link identity, two-supersedes links, no-onselect-no-throw cases)"
        status: pass
    human_judgment: false

# Metrics
duration: 55min
completed: 2026-08-21
status: complete
---

# Phase 7 Plan 2: Console State Badges, Dim-iff-Past, and MemoryDetail State Section Summary

**Console rendering of the four wire-derived record states (archived/superseded/expired/scheduled) via achromatic badges, a dim-iff-past row treatment with a hard accessibility carve-out, an unconditional schema chip, and a State section with clickable full-UUID successor/predecessor links.**

## Performance

- **Duration:** ~55 min
- **Started:** 2026-08-20T21:56:00Z (approx.)
- **Completed:** 2026-08-21T22:11:00Z (approx., local clock)
- **Tasks:** 3/3 completed
- **Files modified:** 7 (2 created, 5 modified)

## Accomplishments

- `ui/src/lib/memorystate.ts` is the console's single D-13 state-word derivation (`memoryStateWords`, `isPastState`, `STATE_WORD_ORDER`), mirroring `cmd/engram/memory_state.go`'s boundary comparisons and inverted-window (expired-over-scheduled) precedence, with its own 15-case test suite covering every case in the plan's `<behavior>` including the inverted-window fixture.
- `MemoryRow` renders 0–3 achromatic state badges in canonical order in the meta line, plus per-element `opacity-60` dimming that is applied to every dimmable element (summary, category, time, scope, tags) but never to the badge itself or a shared ancestor — the badge stays full opacity inside a dimmed row by construction. The meta line gained `flex-wrap` so nothing is ever truncated or elided.
- `MemoryDetail` gained a fourth, unconditional `schema` chip in the Meta tab's chip row, and a conditional State section listing archived/superseded-by/supersedes/expired/scheduled(+close) lines. `superseded_by`/`supersedes` render as `<button>` links whose visible text is the exact 36-character UUID the wire supplies — never shortened — wired through a new optional `onselect` prop that `observe/+page.svelte` connects to the existing `navigate({ selectedId })` helper, so a successor link changes selection rather than navigating pages.

## Task Commits

Each task followed RED (failing test) → GREEN (implementation), each as its own commit:

1. **Task 1: The console's state-word vocabulary module**
   - `982653b2` test(07-02): add failing test for console state-word derivation
   - `25f00db2` feat(07-02): implement console state-word derivation
2. **Task 2: MemoryRow state badges and the dim-iff-past treatment**
   - `32be87c4` test(07-02): add failing test for MemoryRow state badges and dim-iff-past
   - `54c87610` feat(07-02): render state badges and dim-iff-past in MemoryRow
3. **Task 3: MemoryDetail schema chip and the State section**
   - `0d048877` test(07-02): add failing test for MemoryDetail schema chip and State section
   - `9d5389ea` test(07-02): fix MemoryDetail state fixtures to use real wall-clock offsets
   - `a832a91a` feat(07-02): add schema chip and State section to MemoryDetail

**Plan metadata commit:** (this SUMMARY.md commit, immediately following)

_Note: Task 3 carries an extra `test` commit — the initial RED fixtures for the expired/scheduled cases used a fixed fictional "now" (2030) rather than an offset from real wall-clock time, which happens to be a date still in the *real* future and so didn't actually exercise the expired branch. Caught and fixed before the GREEN commit; see Deviations._

## Files Created/Modified

- `ui/src/lib/memorystate.ts` — the console's sole D-13 state-word derivation
- `ui/src/lib/memorystate.test.ts` — 15-case test suite (every `<behavior>` case, boundary equalities, inverted window, order independence)
- `ui/src/lib/components/MemoryRow.svelte` — state badges + dim-iff-past
- `ui/src/lib/components/MemoryRow.browser.test.ts` — 7 new cases covering live/archived/compound/scheduled/badge-order/badge-never-dimmed/meta-line-wrap
- `ui/src/lib/components/MemoryDetail.svelte` — schema chip + State section + `onselect` prop
- `ui/src/lib/components/MemoryDetail.browser.test.ts` — 8 new cases covering schema chip, every state-section line shape, link identity, two-predecessor links, no-onselect-no-throw
- `ui/src/routes/observe/+page.svelte` — one-line `onselect` wiring to `navigate({ selectedId })`

## Decisions Made

- **State-section gate widened to include bare `supersedes`.** The plan's action text says "gate it on `memoryStateWords(memory).length > 0` plus a non-empty `supersedes` list" — read literally as AND this would hide a pure successor record's predecessor links (since `supersedes` alone contributes no state word). Implemented as OR (`stateWords.length > 0 || memory.supersedes.length > 0`), which is the only reading consistent with the plan's own behavior spec ("A record with two `supersedes` entries renders two links"). Caught by a genuine RED failure on the two-supersedes-links test before the fix.
- **Real-time-relative test fixtures for expired/scheduled.** `memoryStateWords` has no injectable `now` on the `MemoryDetail` call site (unlike `MemoryRow`, which also has none — both use the default `now = new Date()`). Test fixtures for the expired/scheduled State-section lines must therefore be offsets from `Date.now()`, not a fixed fictional date; a fixed "2030" fictional now used for the archived-timestamp test happened to still be in the real future, silently defeating the expired case until fixed.
- **Schema chip value assertion uses substring match.** The chip's rendered value text is a sibling of its label span, not its own element — matches how the pre-existing `by`/`src`/`vis` chip tests assert their values (e.g. `getByText('sean')`, no `exact: true`).

## Deviations from Plan

None — the two decisions above were RED-phase test corrections and a literal-vs-intent reading resolved in the implementation itself, not unplanned scope. No Rule 1-4 auto-fixes were required beyond normal TDD iteration.

## Issues Encountered

- **`cd ui && npm run check` (svelte-check) crashes on startup in this environment**, independent of any file this plan touches: `TypeError: Cannot read properties of undefined (reading 'useCaseSensitiveFileNames')` inside svelte-check's `ConfigLoader` construction, before any project file is loaded. Root cause: `svelte-check@4.7.3` (pinned in `ui/package.json`) is incompatible with `typescript@7.0.2` (the new native-compiler preview, also pinned) — `tsc --version` itself works fine (`Version 7.0.2`), but svelte-check's CJS bundle expects a `typescript` module shape TS 7.0.2's preview no longer exposes the same way. This is a pre-existing devDependency-version incompatibility in the repo's own pin, not something introduced by this plan's changes (the crash occurs during config loading, before any `.svelte`/`.ts` file is read), and is out of scope to fix here — bumping `svelte-check` or `typescript` would touch `ui/package.json`/`pnpm-lock.yaml`, which the plan's own verification step explicitly checks stays unchanged (`git diff --exit-code ui/package.json ui/package-lock.json` — no new/changed dependency this phase). Verified `git diff --exit-code ui/package.json` is clean. TypeScript correctness was instead verified by: matching every field access against the generated `ui/src/lib/gen/engram/v1/engram_pb.ts` types read in full, running the entire test suite (`pnpm exec vitest run` — 31 files, 234 tests, all passing, including every pre-existing test unrelated to this plan), and `rg`-based structural checks per the plan's acceptance criteria.
- **`cd ui && npm run lint` — no `lint` script exists in `ui/package.json`.** The package's `scripts` block only defines `dev`, `build`, `test`, `test:browser`, `test:node`, `check`. This is a pre-existing gap in the plan's own `<verification>` block (the script it names was never wired up on the `ui/` side — the repo's `task lint` covers Go/YAML/Markdown per `CLAUDE.md`, with no mention of an `ui/`-side eslint step), not something this plan's scope covers adding (that would be an unrelated tooling addition, arguably Rule 4 architectural territory). Flagging for the phase-level verifier/orchestrator rather than silently skipping.
- **`node_modules` was absent in this worktree** and had to be bootstrapped via `pnpm install --frozen-lockfile` before any test could run — expected for a freshly-created worktree, not a deviation.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `ui/src/lib/memorystate.ts` is available for any future console surface needing the state vocabulary (e.g. `ScopesSidebar`'s include-flag toggles in plan 07-06, if that plan wants to cross-check against the same derivation).
- The `onselect` prop pattern on `MemoryDetail` (wired to the existing `navigate({ selectedId })` helper) is now precedented for any future cross-record navigation inside the detail pane.
- Known follow-ups for the orchestrator/phase verifier: (1) `npm run check` is currently broken in this environment for reasons unrelated to any phase-7 plan (pre-existing devDependency pin incompatibility) — worth a repo-level issue independent of this milestone; (2) `ui/package.json` has no `lint` script, so the plan-authored `<verification>` line `npm run lint` cannot execute as written on any phase touching `ui/`.

---

## Self-Check: PASSED

- `ui/src/lib/memorystate.ts` — FOUND
- `ui/src/lib/memorystate.test.ts` — FOUND
- `ui/src/lib/components/MemoryRow.svelte` — FOUND (modified)
- `ui/src/lib/components/MemoryRow.browser.test.ts` — FOUND (modified)
- `ui/src/lib/components/MemoryDetail.svelte` — FOUND (modified)
- `ui/src/lib/components/MemoryDetail.browser.test.ts` — FOUND (modified)
- `ui/src/routes/observe/+page.svelte` — FOUND (modified)
- Commit `982653b2` — FOUND in `git log`
- Commit `25f00db2` — FOUND in `git log`
- Commit `32be87c4` — FOUND in `git log`
- Commit `54c87610` — FOUND in `git log`
- Commit `0d048877` — FOUND in `git log`
- Commit `9d5389ea` — FOUND in `git log`
- Commit `a832a91a` — FOUND in `git log`
- Full test suite: `pnpm exec vitest run` — 31 test files, 234 tests, all passing (including every pre-existing test, no regressions)
- `rg -o 'export function memoryStateWords' ui/src/lib/memorystate.ts | wc -l` → 1
- `rg -l 'function memoryStateWords' ui/src --glob '!*.test.ts'` → only `memorystate.ts`
- `rg -o 'text-\[10px\] uppercase' ui/src/lib/components/MemoryRow.svelte | wc -l` → 1 (≥1 required)
- `rg -o 'text-primary underline' ui/src/lib/components/MemoryDetail.svelte | wc -l` → 2 (≥1 required)
- `git diff --exit-code ui/package.json` → clean (no dependency change)

---
*Phase: 07-console-cli-state-surfacing*
*Completed: 2026-08-21*
