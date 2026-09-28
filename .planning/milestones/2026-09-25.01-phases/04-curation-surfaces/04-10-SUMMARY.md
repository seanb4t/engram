---
phase: 04-curation-surfaces
plan: 10
subsystem: ui
tags: [svelte, tanstack-query, connect-rpc, scheduled, curation, tdd, resume-envelope, cursor-pagination]

# Dependency graph
requires:
  - phase: 04-curation-surfaces
    provides: "04-02 (ResumeEnvelope v2's ArchiveResumeEnvelope + /scheduled in ALLOWED_DESTINATIONS), 04-06 (CurationSurfaces host: openArchive/reopenFromResume, curation/host.svelte.ts registry), 04-07 (ResultsList rowTrailing/rowActions/RowActions toolbar, recall-header.ts's scheduledHeaderParts/scheduledEmptyHeading)"
provides:
  - "Task 1 (tracer): /scheduled end to end -- ListScheduled cross-spine 'scheduled' state -> rows with window range + relative reveal/expiry phrase"
  - "Task 2 (TDD): scheduled|expired|all tabs, cursor infinite scroll with a trailing loading-more/next-page-error+Retry row, the honest scheduledHeaderParts/scheduledEmptyHeading header/empty states, E6 overflow (phrase truncates before the timestamps, tooltip carries both)"
  - "Task 3 (TDD): archive restricted to expired rows only (toolbar/pane/keyboard/bulk-selection/⌘K), the Scheduled leg of the CUR-05 archive re-auth resume round trip"
  - "windowRange/windowPhrase (ui/src/lib/time.ts) and the scheduled-params.ts URL codec, both usable by any future windowed-record surface"
affects: []

# Actuals (#2632)
actuals:
  tokens: 12057
  tasks: 3
  commits: 5
  plan_head_before: 54200fa76723b1172292d084fceedf321b9d6b06
  plan_head_after: 32a77ee70356fc82ca89943a1a78308a9477120c

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "windowPhrase reuses relativeTime's own m/h/d/mo/y bucketing for BOTH directions by swapping which argument plays 'now' -- relativeTime(now, notBefore) yields the future delta through the identical thresholds relativeTime(notAfter, now) uses for the past, so there is only one bucketing implementation to keep in sync with time.test.ts's boundaries."
    - "The trailing cell's window phrase truncates independently of its (never-shrinking) timestamp range by splitting rowTrailing into two flex children: a flex:none range track and a min-width:0/overflow:hidden phrase track, inside the .trail cell ResultRow.svelte already gives a route-supplied trailing snippet."
    - "hasMore is gated off while isFetchNextPageError is true (hasMore={listQ.hasNextPage && !listQ.isFetchNextPageError}) -- the virtual list calls onloadmore eagerly whenever hasMore is true, independent of container size, so leaving hasMore on through an error would have the list retry the same failed page indefinitely instead of surfacing the Retry row."
    - "CurationSurfaces' resolveRecords only scans the searchMemories/listMemories query caches, not listScheduled -- every /scheduled archive-target id resolves through its GetMemory fallback instead of a cache hit. Functionally correct (documented below, not a stub); left unwidened since CurationSurfaces.svelte is shared infrastructure outside this plan's declared files."

key-files:
  created:
    - ui/src/lib/search/scheduled-params.ts
    - ui/src/lib/search/scheduled-params.test.ts
    - ui/src/routes/scheduled/+page.svelte
    - ui/src/routes/scheduled/scheduled.browser.test.ts
  modified:
    - ui/src/lib/time.ts
    - ui/src/lib/time.test.ts

key-decisions:
  - "windowPhrase's expired/scheduled precedence mirrors memorystate.ts exactly (expired evaluated first, suppresses scheduled for an inverted not_before/not_after pair) -- pinned by a dedicated time.test.ts case rather than left as an implicit consequence of the if/else order."
  - "Task 2's TDD tests for time.ts/scheduled-params.ts were written and run against Task 1's ALREADY-SHIPPED implementation (windowRange/windowPhrase/the codec are Task 1 deliverables per the plan's own Artifacts table) -- they passed immediately (GREEN from the first run), never RED. Task 2's genuine RED/GREEN cycle is scheduled.browser.test.ts's tabs/pagination/header/E6 coverage, confirmed failing against the pre-Task-2 route (empty header parts, no tabs, no trailing loading/error row) before implementing."
  - "Two real bugs found and fixed while confirming GREEN, both required for the plan's own specified behavior to actually work (not scope creep): (1) a stray code comment containing the literal word 'Pagination' tripped Task 2's own zero-occurrence acceptance grep -- reworded, no behavior change. (2) ResultsList's virtual list calls onloadmore eagerly whenever hasMore is true regardless of container size; without gating hasMore off during isFetchNextPageError, a next-page failure would have looped the same failed fetch forever instead of ever settling into the Retry-row state Task 2's own <behavior> text specifies."
  - "Archive-target chip resolution for /scheduled always takes CurationSurfaces' GetMemory fallback path, never a cache hit, because resolveRecords only scans searchMemories/listMemories (not listScheduled). Confirmed functionally correct (every archive test mocks GetMemory and passes); left CurationSurfaces.svelte unwidened since it is shared infrastructure outside this plan's own files_modified list -- flagged here for the verifier and any future plan that touches that resolver."

requirements-completed: [CUR-04]
# CUR-05 is shared with not-yet-executed sibling plans 04-08 and 04-09 (both
# declare it in their own PLAN.md frontmatter) -- per the shared-ID gate
# (#2388), a shared requirement does not read Complete until every plan
# declaring it has finished. The Scheduled leg of CUR-05 (archive re-auth
# resume round trip) is fully shipped and verified in this plan (see
# coverage D3 below); it will be marked complete once 04-08 and 04-09 also
# finish, mirroring 04-07-SUMMARY.md's identical documented precedent.

coverage:
  - id: D1
    description: "CUR-04 tracer: /scheduled opens on the scheduled tab, calls ListScheduled cross-spine with state 'scheduled', and renders each windowed record as one row showing its window range and relative reveal/expiry phrase"
    requirement: "CUR-04"
    verification:
      - kind: automated_ui
        ref: "ui/src/routes/scheduled/scheduled.browser.test.ts#scheduled route — lists windowed records across scopes (CUR-04 tracer)"
        status: pass
      - kind: unit
        ref: "ui/src/lib/time.test.ts (windowRange/windowPhrase, 10 cases incl. inverted-window precedence), ui/src/lib/search/scheduled-params.test.ts (7 cases)"
        status: pass
    human_judgment: false
  - id: D2
    description: "D-13: scheduled|expired|all state tabs persisted in the URL, cursor infinite scroll (never numbered pages) with a trailing loading-more row and a next-page-error+Retry row that never loses page 1, the honest scheduledHeaderParts/scheduledEmptyHeading header/empty states per tab, and E6 overflow (phrase truncates before the fixed timestamp range, tooltip carries both raw timestamps)"
    requirement: "CUR-04"
    verification:
      - kind: automated_ui
        ref: "ui/src/routes/scheduled/scheduled.browser.test.ts (describes 'tabs (D-13)', 'per-tab honest empty state (E6, D-13)', 'a single record renders one row (E6 zero-one-many)', 'cursor infinite scroll (E6 loading)', 'honest header (D-13)', 'overflow (E6, DSYS-04)' — 9 cases, 4 screenshots)"
        status: pass
    human_judgment: false
  - id: D3
    description: "D-13/CUR-05/D-16: archive is the only curation action /scheduled offers, gated to expired rows only across every entry point (toolbar, pane, keyboard 'a', bulk selection, ⌘K host); a mixed or all-scheduled selection is handled honestly (archives only the expired member, or toasts and opens nothing); the archive re-auth resume round trip persists returnPath '/scheduled?state=expired' and reopens without an auto-call"
    requirement: "CUR-05"
    verification:
      - kind: automated_ui
        ref: "ui/src/routes/scheduled/scheduled.browser.test.ts#scheduled route — archive for expired rows only (D-13) (9 cases: toolbar gating, pane gating + zero other curation actions, keyboard single/mixed/all-scheduled selection, dimmed-in-place after archive, PermissionDenied re-auth persist, seeded-envelope reopen with zero ArchiveMemory calls and one consumeResume, curationHost.actionsFor gating)"
        status: pass
    human_judgment: false

# Metrics
duration: ~48min
completed: 2026-09-27
status: complete
---

# Phase 4 Plan 10: Scheduled view — windows, tabs, cursor paging, archive-on-expired Summary

**Ships `/scheduled` (CUR-04): windowed records the recall gate is hiding, by state tab, with their window range and relative reveal/expiry phrase, cursor-paginated with honest loading/error/empty states — plus the Scheduled leg of the CUR-05 archive re-auth resume, with archive restricted to expired rows across every entry point.**

## Performance

- **Duration:** ~48 min
- **Started:** 2026-09-27T16:58:00Z
- **Completed:** 2026-09-27T17:46:00Z
- **Tasks:** 3 of 3 completed (Task 1 tracer, Task 2 TDD, Task 3 TDD)
- **Files modified:** 6 (4 created, 2 modified)

## Accomplishments

- `ui/src/lib/time.ts`: `windowRange(m)` (`{not_before} → {not_after}`, `—` for an absent bound) and `windowPhrase(m, now)` (`reveals in {unit}` / `expired {unit} ago`, `reveals in under a minute` / `expired just now` under a minute, `''` while active) — the latter reuses `relativeTime`'s own bucket thresholds for both the past and future direction by swapping which argument plays "now".
- New `ui/src/lib/search/scheduled-params.ts`: `ScheduledState`, `ScheduledParams { state, sel }`, `defaultScheduledParams`/`parseScheduledParams`/`encodeScheduledParams` — one parse function, one encode function, an unknown `state` value parses to `'scheduled'`, defaults never write back to the URL.
- New `ui/src/routes/scheduled/+page.svelte`: `ListScheduled` via `createInfiniteQuery` (cross-spine, cursor-paginated, `placeholderData: keepPreviousData`), `Tabs.Root`/`List`/`Trigger` for `scheduled`/`expired`/`all` navigating `state` and clearing `sel`, `scheduledHeaderParts`/`scheduledEmptyHeading` for the honest header/empty states, a trailing "Loading more…" row while `isFetchingNextPage` and a parsed-envelope+Retry row on `isFetchNextPageError` (with `hasMore` gated off during the error so the virtual list's own eager auto-load doesn't loop the failed fetch), a `rowTrailing` snippet splitting the window range (fixed) from the relative phrase (truncating) so overflow never eats the timestamps, `isExpired`/`archiveExpired` restricting archive to expired rows (toolbar, pane, keyboard `a`, bulk selection all wired; an all-scheduled request toasts "Archive applies to expired rows only" instead of opening anything), `CurationSurfaces` mounted in a stable spot outside `RecallSplit`, and an `onMount` that reopens a seeded archive resume envelope and registers the curation host for ⌘K.
- New `ui/src/routes/scheduled/scheduled.browser.test.ts` (21 cases) plus extensions to `ui/src/lib/time.test.ts` (7 new cases) and the new `ui/src/lib/search/scheduled-params.test.ts` (7 cases).

## Task Commits

Each task was committed atomically (Tasks 2 and 3 ran under TDD: test → feat):

1. **Task 1 (tracer): /scheduled end to end** — `7eb7a3e2` (feat)
2. **Task 2: tabs, cursor paging, header, E6 states (TDD)** — `062f7188` (test, RED — confirmed failing against the pre-Task-2 route before implementing) → `010cc5b6` (feat, GREEN)
3. **Task 3: archive for expired rows only, archive resume round trip (TDD)** — `91eea01b` (test, RED — all 9 cases confirmed failing against the pre-Task-3 route before implementing) → `32a77ee7` (feat, GREEN)

**Plan metadata:** this commit (docs: complete plan)

_Note: time.test.ts's and scheduled-params.test.ts's own new cases (written as part of Task 2) ran GREEN on their first execution — see Decisions Made for why that is expected, not a TDD-discipline gap._

## Files Created/Modified

- `ui/src/lib/time.ts` — `windowRange`/`windowPhrase`
- `ui/src/lib/time.test.ts` — 7 new cases covering both functions' edge cases and precedence
- `ui/src/lib/search/scheduled-params.ts` — the `/scheduled` URL/request codec
- `ui/src/lib/search/scheduled-params.test.ts` — 7 cases (default, parse/encode round-trip, unknown-state fallback)
- `ui/src/routes/scheduled/+page.svelte` — the `/scheduled` route
- `ui/src/routes/scheduled/scheduled.browser.test.ts` — 21 cases across tracer, tabs, empty/error/pagination states, header, overflow, and archive-on-expired

## Decisions Made

See `key-decisions` in the frontmatter for full detail. Summary: `windowPhrase`'s precedence deliberately mirrors `memorystate.ts`'s expired-suppresses-scheduled rule for an inverted window; Task 2's `time.ts`/`scheduled-params.ts` tests ran GREEN immediately because their implementation already shipped in Task 1 (only `scheduled.browser.test.ts`'s new tab/pagination/header/E6 coverage exercised a genuine RED→GREEN cycle); two real bugs (a stray "Pagination" comment word, and the virtual list's eager auto-load needing to be gated off during a next-page error) were found and fixed while confirming GREEN; `/scheduled`'s archive-chip resolution always takes `CurationSurfaces`' `GetMemory` fallback rather than a `listScheduled` cache hit (functionally correct, documented rather than silently left).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] A code comment's literal word "Pagination" tripped Task 2's own zero-occurrence acceptance check**
- **Found during:** Task 2 acceptance-criteria verification loop
- **Issue:** `rg -o -e 'Pagination' ui/src/routes/scheduled/+page.svelte | wc -l` is required to print `0` (cursor paging only, D-13's own rule), but the route's top comment explained the rule using the literal word "Pagination-free acceptance check."
- **Fix:** Reworded the comment to describe the same rule without using the word.
- **Files modified:** `ui/src/routes/scheduled/+page.svelte`
- **Verification:** `rg -o -e 'Pagination' ui/src/routes/scheduled/+page.svelte | wc -l` → `0`
- **Committed in:** `010cc5b6` (Task 2 commit)

**2. [Rule 1 - Bug] The virtual list's eager auto-load would have retried a failed next-page fetch forever instead of ever showing the Retry row**
- **Found during:** Task 2, writing the next-page-error browser test
- **Issue:** `ResultsList`'s underlying virtual list calls `onloadmore` as soon as `hasMore` is true, independent of container size or fetch state — a next-page rejection leaves `hasNextPage` (computed from the last SUCCESSFUL page) still true, so leaving `hasMore={listQ.hasNextPage}` unconditionally would have the list immediately re-invoke `fetchNextPage()` again on the same failed page, looping indefinitely rather than settling into the `isFetchNextPageError` state Task 2's own `<behavior>` text specifies (an envelope + Retry row).
- **Fix:** `hasMore={listQ.hasNextPage && !listQ.isFetchNextPageError}` — the virtual list stops auto-requesting once an error is showing; clicking Retry calls `fetchNextPage()` explicitly.
- **Files modified:** `ui/src/routes/scheduled/+page.svelte`
- **Verification:** `ui/src/routes/scheduled/scheduled.browser.test.ts`'s "a failed page-2 fetch keeps page 1 visible and shows the envelope with a Retry in place of the loading row" case passes without the duplicate-key/infinite-loop failure it reproduced before the fix.
- **Committed in:** `010cc5b6` (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (both Rule 1 — real bugs found while confirming the plan's own specified behavior, one cosmetic and one functional). **Impact:** No scope creep; both fixes were necessary for Task 2's own `<behavior>` text to actually hold.

## Issues Encountered

- **`requirements.mark-complete CUR-04` returned `not_found` despite `CUR-04` existing verbatim in `REQUIREMENTS.md` (line 62's checkbox, line 151's traceability row).** Investigated before concluding upstream: `CUR-01` — completed by 04-06 and listed in that plan's own `requirements-completed` frontmatter — shows the SAME unchecked `- [ ]` state in `REQUIREMENTS.md` today, proving this is a pre-existing gap in the mark-complete tooling/file-format match, not something this plan introduced or a worktree-isolation artifact. Per the planning-artifacts rule against hand-editing a tool-owned generated file, `REQUIREMENTS.md` was left untouched (the call was a genuine no-op, `updated: false`) rather than patched by hand. `CUR-04`'s `requirements-completed` frontmatter entry above is this plan's own accurate record regardless of whether `REQUIREMENTS.md`'s checkbox/table ever gets mechanically updated.
- **Test-mock realism, not app bugs (all fixed within the RED/GREEN cycle, no separate deviation):** several of my own test mocks needed correction after genuine failures exposed them — a mocked rejection needs to be a `ConnectError` (not a plain `Error`) for `parseConnectError`'s "rejected" branch to fire; a `notBefore` fixture needed a several-minute buffer past its exact day boundary since the route calls `windowPhrase(m)` with the real `Date.now()` (not an injected `now` — that's `time.test.ts`'s job), and a few seconds of render latency can floor the day bucket down by one; a mocked page-1 response with a non-empty `nextPageToken` combined with a repeat-forever `mockResolvedValue` (rather than `mockResolvedValueOnce`) causes the virtual list's eager auto-load to loop the same id across pages, tripping `SvelteVirtualList`'s own duplicate-itemKey guard — fixed by having every such mock's continuation either terminate with an empty `nextPageToken` or hang forever (`new Promise(() => {})`) rather than repeat.
- **Keyboard-selection tests needed the same conventions `search.browser.test.ts`'s own bulk-archive tracer already established:** `userEvent.keyboard(...)` (not a raw `element.dispatchEvent(new KeyboardEvent(...))`), a forced wide `RecallSplit` layout (`width: 1200px`) with a wait for `.rs-group` to mount before focusing, and waiting for `aria-activedescendant` to actually land on the next row between `j` and the following `x` (`moveActive` resolves asynchronously via the virtual list's own `scroll()`). Raw synchronous `dispatchEvent` calls without these waits produced flaky/incorrect selections in an earlier draft of the archive-selection tests.
- **`archived`/`superseded` state-word chips render with no CSS class of their own** (only `expired`/`scheduled` get one, per `engram-console-conventions`) — a test asserting the post-archive row's state word had to match on `.st` element text content rather than a `.st.archived` selector.

## Known Stubs

None. Every deliverable renders real data end to end; no hardcoded empty values or placeholder text. The `CurationSurfaces`-resolver gap noted in `key-decisions` (archive-target chips always resolve through a live `GetMemory` call rather than a `listScheduled` cache hit) is a documented efficiency characteristic, not a stub — every archive path is proven functionally correct by the Task 3 test suite.

## Threat Flags

None beyond the plan's own `<threat_model>` (T-04-23 through T-04-25), all handled as specified: T-04-23 (cross-spine `ListScheduled` information disclosure) is a server-side transfer, the route only renders what it receives; T-04-24 (archiving a not-yet-expired record) is an accepted client-side UX rule, backed by owner-only, reversible server-side archiving; T-04-25 (repudiation via an automatic archive after re-auth) is mitigated exactly as specified — the reopened confirm shows the notice and makes zero `ArchiveMemory` calls until the operator clicks Archive/Resend themselves, pinned by the "seeded archive envelope reopens..." test (`archiveMemorySpy` not called, `consumeResume` called exactly once).

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `/scheduled` is fully shipped: CUR-04 complete, the Scheduled leg of CUR-05 complete and verified.
- `requirements-completed` lists only `CUR-04` here — `CUR-05` stays unchecked in `REQUIREMENTS.md` until sibling plans 04-08 and 04-09 (both declare it) also finish, per the shared-ID gate. No action needed from a future plan beyond that; this plan's own CUR-05 obligation (the Scheduled leg) is done.
- `windowRange`/`windowPhrase` (`ui/src/lib/time.ts`) and the `scheduled-params.ts` codec pattern are available to any future windowed-record surface without re-deriving the reveal/expiry math.
- No blockers.

---

## Self-Check: PASSED

- All 6 key-files (4 created, 2 modified) verified present on disk with `[ -f ]`.
- All 5 commits (`7eb7a3e2`, `062f7188`, `010cc5b6`, `91eea01b`, `32a77ee7`) verified present in `git log --oneline --all`.
- Re-ran all three tasks' plan-level `<verify>` commands: browser project (`scheduled.browser.test.ts`, 21/21), node project (`time.test.ts` + `scheduled-params.test.ts`, 23/23), `pnpm --dir ui build` exits 0 — all green after the final commit.
- Full regression: `pnpm --dir ui vitest run --project node` (24 files / 291 tests) and `--project browser` (32 files / 406 tests) both green (one `resume.test.ts` TTL-boundary timing flake observed under host load during a full-suite run, confirmed passing in isolation and on a full-suite rerun — not a regression from this plan's changes, no file this plan touches).
- Re-ran all acceptance-criteria `rg` checks across all three tasks — all match their required counts (`engram.listScheduled` = 1, `crossSpine: true` ≥ 1, `windowRange|windowPhrase` = 2 exported; `scheduledHeaderParts|scheduledEmptyHeading` = 2, `Pagination` = 0; `onsupersede=|onrestore=|onedit=|onvisibility=|ondelete=|onchain=` = 0, `Archive applies to expired rows only` = 1, `env.kind === 'archive'` = 1).

*Phase: 04-curation-surfaces*
*Completed: 2026-09-27*
