---
phase: 04-curation-surfaces
plan: 09
subsystem: ui
tags: [svelte, rules, delete, resume-envelope, honest-feedback, tdd]

# Dependency graph
requires:
  - phase: 04-curation-surfaces
    provides: "04-02's v2 resume envelope (delete kind, /rules ALLOWED_DESTINATIONS) and Rules/Scheduled nav slot; 04-07's ResultsList groupKey/groupHeader/rowTrailing props and recall-header.ts's rulesHeaderParts/rulesEmptyHeading formatters"
provides:
  - "The /rules route (CUR-03): a single ListRules(scopes:[]) cross-scope read, rows grouped by scope with a fixed 'shared' chip, full text on demand via GetMemory in the shared DetailPane"
  - "rules-params.ts: the /rules URL codec (RulesParams { sel }, defaultRulesParams, parseRulesParams, encodeRulesParams)"
  - "D-12's honest header/coverage/advisory contract on /rules: count/scope coverage, scopes_truncated/scopes_unknown clauses, the server's own advisory rendered verbatim, and the E5 empty/loading/error states reusing RecallState/rankedHeaderParts' pattern"
  - "DeleteConfirmDialog's third 'rule' kind and a new `notice?` prop (reused pattern from ArchiveConfirmDialog)"
  - "The Rules leg of CUR-05: a delete-only confirm whose Unauthenticated/PermissionDenied failure persists a v2 { kind: 'delete', id, returnPath } resume envelope before redirecting, and whose reopen on /rules never resends automatically"
affects: []

# Actuals (#2632)
actuals:
  tokens: 9608
  tasks: 3
  commits: 5
  plan_head_before: 54200fa76723b1172292d084fceedf321b9d6b06
  plan_head_after: c1e585a4380f0fd696d229a11695158b3da7c02d

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "The /rules route follows /search's exact composition shape (ResultsHeader + RecallSplit + ResultsList + DetailPane + a route-local DeleteConfirmDialog host) rather than inventing a new page layout — the only new pieces are the sel-only URL codec and the scope groupKey/rowTrailing wiring 04-07 already built generically"
    - "DeleteConfirmDialog's `notice?: string` prop mirrors ArchiveConfirmDialog's own `notice` (same faint block above the footer, same 'Signed in again — review and resend' copy) rather than inventing a second re-auth-reopen affordance"
    - "The route's delete flow is host-authoritative (deleteTarget/deleteOpen/deleteAuthFailure/deleteNotice state, confirmDelete/cancelDelete/onDeleteReauth functions) mirroring WriteSurfaces.svelte's confirmDelete pattern verbatim, rather than routing through CurationSurfaces (which owns archive/restore/supersede, not plain delete)"

key-files:
  created:
    - ui/src/lib/search/rules-params.ts
    - ui/src/lib/search/rules-params.test.ts
    - ui/src/routes/rules/+page.svelte
    - ui/src/routes/rules/rules.browser.test.ts
  modified:
    - ui/src/lib/components/DeleteConfirmDialog.svelte
    - ui/src/lib/components/DeleteConfirmDialog.browser.test.ts

key-decisions:
  - "Task 2's rules-params.test.ts round-trip cases pass immediately against Task 1's implementation (the codec shipped complete in the tracer) — the genuine RED for Task 2 comes entirely from rules.browser.test.ts's header/advisory/empty/loading/error/ordering cases (7 of 11 failed before implementation), which is disclosed in the RED commit message rather than treated as a violation of the TDD gate"
  - "DetailPane and ResultsList's `ondelete` callback is the only prop /rules ever passes to either component — no onedit/onvisibility/onarchive/onrestore/onsupersede/onchain/selectable, verified by a zero-occurrence rg acceptance check, so the legend and pane never advertise a control rules cannot use (rules are user-blessed ground truth: delete and re-bless, never edit/share/archive/supersede)"
  - "Delete-button click assertions in rules.browser.test.ts scope to `screen.getByRole('dialog').getByRole('button', { name: 'Delete' })` rather than a bare role query, once DetailPane's own inline Delete button (also gated on the same `ondelete` callback) created a second same-named button whenever the pane was open — disclosed as the reason those four assertions are dialog-scoped, not a workaround for a flaky test"

requirements-completed: [CUR-03, CUR-05]

coverage:
  - id: D1
    description: "CUR-03 tracer: /rules lists every readable rule via one ListRules(scopes:[]) read, grouped by scope with a fixed 'shared' chip, full text on demand via GetMemory"
    requirement: CUR-03
    verification:
      - kind: automated_ui
        ref: "ui/src/routes/rules/rules.browser.test.ts#rules route — lists every readable rule grouped by scope (CUR-03 tracer) > calls ListRules with empty scopes and full false, renders scope group headers with a shared chip, and opens the pane via GetMemory on click"
        status: pass
    human_judgment: false
  - id: D2
    description: "D-12 honest header/coverage/advisory and the E5 empty/loading/error/adjacency/long-text states"
    requirement: CUR-03
    verification:
      - kind: unit
        ref: "ui/src/lib/search/rules-params.test.ts (4 cases: parse/encode round-trip, default encodes to '')"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/rules/rules.browser.test.ts (describes 'honest header, coverage and advisory (D-12)', 'empty, loading and error states (E5)', 'adjacency and ordering (CUR-03)', 'long summaries stay one line (E5/CUR-03 backstop)' — 10 cases)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Delete-only rules and the CUR-05 delete resume round trip: no edit/visibility/archive/restore/supersede/selection anywhere on /rules; an Unauthenticated delete persists a v2 delete envelope and reopens the confirm with a review-and-resend notice, never auto-resending"
    requirement: CUR-05
    verification:
      - kind: unit
        ref: "ui/src/lib/components/DeleteConfirmDialog.browser.test.ts (3 new cases: 'rule' kind copy, notice rendered/omitted)"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/rules/rules.browser.test.ts (describe 'delete only (D-12, CUR-05)', 6 cases: delete-only affordances, '#' opens confirm, Delete succeeds + invalidates + clears sel, Unauthenticated persists envelope + redirects, seeded envelope reopens with notice and fires nothing until Delete is clicked, NotFound clears without crashing)"
        status: pass
      - kind: other
        ref: "pnpm --dir ui build (vite build exit 0)"
        status: pass
    human_judgment: false

# Metrics
duration: 40min
completed: 2026-09-27
status: complete
---

# Phase 4 Plan 9: Rules view — grouped index, honest coverage, delete-only with re-auth-safe resume Summary

**Ships `/rules` (CUR-03): one `ListRules` cross-scope read grouped by scope with a fixed 'shared' chip, honest header/advisory/empty/error states, and a delete-only confirm whose CUR-05 resume envelope reopens without ever auto-resending.**

## Performance

- **Duration:** 40 min
- **Started:** 2026-09-27T17:00:00Z (approx.)
- **Completed:** 2026-09-27T17:21:00Z
- **Tasks:** 3 (1 tracer, 2 TDD)
- **Files modified:** 6 (4 created, 2 modified)

## Accomplishments

- `ui/src/lib/search/rules-params.ts`: the one `/rules` URL codec (`RulesParams { sel }`, `defaultRulesParams`, `parseRulesParams`, `encodeRulesParams`), mirroring `params.ts`'s rule for `/search`.
- `ui/src/routes/rules/+page.svelte`: a single `engram.listRules({ scopes: [], tags: [], full: false })` read per load, rows grouped by scope via `ResultsList`'s `groupKey`/`groupHeader`, a `rowTrailing` snippet rendering the fixed `shared` badge on every row, and a pane that always reads the full record through `GetMemory` (rows are compact).
- D-12's honest feedback: `ResultsHeader` reads `rulesHeaderParts({ count, scopeCount, scopesTruncated, scopesUnknown })`; a non-empty server `advisory` renders verbatim under the header; `RecallState` handles the empty (`rulesEmptyHeading()`, no fix rows) and rejected-envelope cases; the skeleton renders on first load only and a re-fetch keeps the prior rows (`keepPreviousData`).
- `DeleteConfirmDialog.svelte` gained a third `kind: 'rule'` (`Delete this rule?` / `this can't be undone. the rule is removed permanently.`) and a `notice?: string` block, reused for the CUR-05 re-auth reopen.
- The Rules route is a host-authoritative delete confirm (mirrors `WriteSurfaces.confirmDelete`): `ondelete` is the *only* callback ever passed to `ResultsList`/`DetailPane` — no edit, visibility, archive, restore, supersede, or selection. An `Unauthenticated`/`PermissionDenied` failure retains the target and shows the re-auth CTA; clicking it persists `{ kind: 'delete', id, returnPath }` (v2 envelope) before redirecting. The route is the sole `peekResume`/`consumeResume` owner for the `delete` kind — a resumed envelope reopens the confirm with "Signed in again — review and resend" and fires nothing until the operator clicks Delete again, consuming the envelope exactly once.

## Task Commits

Each task was committed atomically (Tasks 2 and 3 ran under TDD: test → feat):

1. **Task 1 (tracer): /rules end to end** — `06654152` (feat)
2. **Task 2: honest header, coverage, advisory and states** — `1d5859b9` (test, RED) → `9fba4de8` (feat, GREEN)
3. **Task 3: delete only + the CUR-05 resume round trip** — `156a4712` (test, RED) → `c1e585a4` (feat, GREEN)

**Plan metadata:** this commit (docs: complete plan)

_Note: no REFACTOR commit was needed for either TDD task — each GREEN implementation required no follow-up cleanup._

## Files Created/Modified

- `ui/src/lib/search/rules-params.ts` — the `/rules` URL codec
- `ui/src/lib/search/rules-params.test.ts` — 4 cases (default/parse/encode round-trip)
- `ui/src/routes/rules/+page.svelte` — the Rules view
- `ui/src/routes/rules/rules.browser.test.ts` — 17 cases across the tracer, header/coverage/advisory/states, adjacency/ordering, the long-text backstop, and the delete-only/resume flow
- `ui/src/lib/components/DeleteConfirmDialog.svelte` — the `rule` kind and `notice` prop
- `ui/src/lib/components/DeleteConfirmDialog.browser.test.ts` — 3 new cases for the rule kind and notice rendering

## Decisions Made

See `key-decisions` in the frontmatter. Summary: Task 2's codec test cases pass immediately since Task 1 already shipped the complete codec (disclosed, not hidden); `/rules` never passes any curation callback besides `ondelete` to the shared list/pane components (verified by acceptance grep); Delete-button assertions in tests are scoped to the dialog role once DetailPane's own gated Delete button created a same-named duplicate whenever the pane was open.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- Attempted `gsd_run query requirements.mark-complete CUR-03` (ready-ids reported CUR-03 ready, CUR-05 blocked by a sibling plan). It reported `not_found` and made no write: `REQUIREMENTS.md`'s traceability table has every Phase 4 requirement (all of CUR-01..CUR-05, and every other phase's rows) parked at `Mapped`, never `Pending` — a milestone-wide pre-existing condition, not something this plan's changes touched. `mark-complete` only accepts `Pending`/`Gaps Found` as forward inputs and rolled back its own checkbox flip once it found a row it couldn't move, so the file is unmodified on disk (confirmed via `git status --short`). Out of this plan's scope per the deviation rules' scope boundary (pre-existing, milestone-wide, not caused by this plan's own changes) — logged here rather than hand-edited.

## Known Stubs

None.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `/rules` ships CUR-03 and the Rules leg of CUR-05 completely; `DeleteConfirmDialog`'s `rule` kind and `notice` prop are available for any future surface needing the same re-auth-safe delete pattern.
- `requirements-completed` lists `[CUR-03, CUR-05]` per this plan's own frontmatter. `requirements.ready-ids` reports CUR-03 ready (no sibling plan in this phase still lacks a SUMMARY declaring it) and CUR-05 blocked (a sibling still does) — but `mark-complete` itself cannot flip EITHER right now: REQUIREMENTS.md's traceability table has every Phase 4 row (and every other phase's) parked at `Mapped`, a status `mark-complete` does not accept as a forward input (only `Pending`/`Gaps Found`). This is milestone-wide and pre-existing, not introduced by this plan — see Issues Encountered. Reconciling REQUIREMENTS.md's Status vocabulary is the orchestrator's/a later pass's call, not this plan's to make unilaterally.
- No blockers.

---

## Self-Check: PASSED

- All 6 key-files (4 created, 2 modified) verified present on disk.
- All 5 commits (`06654152`, `1d5859b9`, `9fba4de8`, `156a4712`, `c1e585a4`) verified present in `git log --oneline`.
- Re-ran all three tasks' plan-level `<verify>` commands: `rules.browser.test.ts` alone (17/17), `rules-params.test.ts` (4/4), `DeleteConfirmDialog.browser.test.ts` + `rules.browser.test.ts` together (28/28), `pnpm --dir ui build` exits 0.
- Re-ran every acceptance-criteria `rg` check across all three tasks — all matched their required counts.

*Phase: 04-curation-surfaces*
*Completed: 2026-09-27*
