---
phase: 02-recall-first-search
verified: 2026-09-28T17:36:53Z
status: passed
score: 7/7 must-haves verified
covered_files:
  - ".claude/skills/engram-connect-client/SKILL.md"
  - ".claude/skills/engram-console-conventions/SKILL.md"
  - ".planning/REQUIREMENTS.md"
  - ".planning/phases/02-recall-first-search/02-01-PLAN.md"
  - ".planning/phases/02-recall-first-search/02-01-SUMMARY.md"
  - ".planning/phases/02-recall-first-search/02-02-PLAN.md"
  - ".planning/phases/02-recall-first-search/02-02-SUMMARY.md"
  - ".planning/phases/02-recall-first-search/02-03-PLAN.md"
  - ".planning/phases/02-recall-first-search/02-03-SUMMARY.md"
  - ".planning/phases/02-recall-first-search/02-04-PLAN.md"
  - ".planning/phases/02-recall-first-search/02-04-SUMMARY.md"
  - ".planning/phases/02-recall-first-search/02-05-PLAN.md"
  - ".planning/phases/02-recall-first-search/02-05-SUMMARY.md"
  - ".planning/phases/02-recall-first-search/02-06-PLAN.md"
  - ".planning/phases/02-recall-first-search/02-06-SUMMARY.md"
  - ".planning/phases/02-recall-first-search/02-07-PLAN.md"
  - ".planning/phases/02-recall-first-search/02-07-SUMMARY.md"
  - ".planning/phases/02-recall-first-search/02-08-PLAN.md"
  - ".planning/phases/02-recall-first-search/02-08-SUMMARY.md"
  - ".planning/phases/02-recall-first-search/02-09-PLAN.md"
  - ".planning/phases/02-recall-first-search/02-09-SUMMARY.md"
  - ".planning/phases/02-recall-first-search/02-10-PLAN.md"
  - ".planning/phases/02-recall-first-search/02-10-SUMMARY.md"
  - ".planning/phases/02-recall-first-search/02-REVIEW-FIX.md"
  - ".planning/phases/02-recall-first-search/02-REVIEW.md"
  - "CLAUDE.md"
  - "cmd/engram/client_common.go"
  - "cmd/engram/client_list.go"
  - "cmd/engram/client_list_test.go"
  - "cmd/engram/client_search.go"
  - "cmd/engram/client_search_test.go"
  - "docs-site/src/content/docs/guides/cli.md"
  - "docs-site/src/content/docs/guides/upgrade.md"
  - "docs-site/src/content/docs/reference/tools.md"
  - "internal/server/connectapi.go"
  - "internal/server/connectapi_crossspine_test.go"
  - "internal/server/connectapi_test.go"
  - "internal/server/connectdescriptor_test.go"
  - "internal/server/fakestore_test.go"
  - "internal/server/hiddencount.go"
  - "internal/server/hiddencount_test.go"
  - "internal/server/store_iface.go"
  - "internal/server/tools.go"
  - "internal/server/tools_test.go"
  - "internal/webauth/static/index.html"
  - "proto/engram/v1/engram.proto"
  - "ui/src/app.css"
  - "ui/src/app.css.test.ts"
  - "ui/src/app.html"
  - "ui/src/app.html.test.ts"
  - "ui/src/lib/components/AppShell.browser.test.ts"
  - "ui/src/lib/components/AppShell.svelte"
  - "ui/src/lib/components/CommandMenu.browser.test.ts"
  - "ui/src/lib/components/CommandMenu.svelte"
  - "ui/src/lib/components/DeleteConfirmDialog.svelte"
  - "ui/src/lib/components/DetailPane.browser.test.ts"
  - "ui/src/lib/components/DetailPane.svelte"
  - "ui/src/lib/components/DiscoveryFormSheet.svelte"
  - "ui/src/lib/components/DisplayPopover.browser.test.ts"
  - "ui/src/lib/components/DisplayPopover.svelte"
  - "ui/src/lib/components/FacetStrip.browser.test.ts"
  - "ui/src/lib/components/FacetStrip.svelte"
  - "ui/src/lib/components/HeaderSearch.browser.test.ts"
  - "ui/src/lib/components/HeaderSearch.svelte"
  - "ui/src/lib/components/MemoryDetail.svelte"
  - "ui/src/lib/components/MemoryFormSheet.svelte"
  - "ui/src/lib/components/MemoryList.svelte"
  - "ui/src/lib/components/MemoryRow.svelte"
  - "ui/src/lib/components/MigrationBanner.svelte"
  - "ui/src/lib/components/RecallSplit.browser.test.ts"
  - "ui/src/lib/components/RecallSplit.svelte"
  - "ui/src/lib/components/RecallState.svelte"
  - "ui/src/lib/components/ResultHoverCard.svelte"
  - "ui/src/lib/components/ResultRow.browser.test.ts"
  - "ui/src/lib/components/ResultRow.svelte"
  - "ui/src/lib/components/ResultsHeader.browser.test.ts"
  - "ui/src/lib/components/ResultsHeader.svelte"
  - "ui/src/lib/components/ResultsList.browser.test.ts"
  - "ui/src/lib/components/ResultsList.svelte"
  - "ui/src/lib/components/ScopeChip.svelte"
  - "ui/src/lib/components/ScopeCombobox.browser.test.ts"
  - "ui/src/lib/components/ScopeCombobox.svelte"
  - "ui/src/lib/components/ShareWarningInline.svelte"
  - "ui/src/lib/components/WriteSurfaces.browser.test.ts"
  - "ui/src/lib/components/WriteSurfaces.svelte"
  - "ui/src/lib/components/ui/popover/index.ts"
  - "ui/src/lib/components/ui/popover/popover-close.svelte"
  - "ui/src/lib/components/ui/popover/popover-content.svelte"
  - "ui/src/lib/components/ui/popover/popover-description.svelte"
  - "ui/src/lib/components/ui/popover/popover-header.svelte"
  - "ui/src/lib/components/ui/popover/popover-portal.svelte"
  - "ui/src/lib/components/ui/popover/popover-title.svelte"
  - "ui/src/lib/components/ui/popover/popover-trigger.svelte"
  - "ui/src/lib/components/ui/popover/popover.svelte"
  - "ui/src/lib/display.svelte.ts"
  - "ui/src/lib/display.test.ts"
  - "ui/src/lib/errors/connect-error.test.ts"
  - "ui/src/lib/errors/connect-error.ts"
  - "ui/src/lib/gen/engram/v1/engram_pb.ts"
  - "ui/src/lib/search/classify.test.ts"
  - "ui/src/lib/search/classify.ts"
  - "ui/src/lib/search/header-search.svelte.ts"
  - "ui/src/lib/search/params.test.ts"
  - "ui/src/lib/search/params.ts"
  - "ui/src/lib/search/recall-header.test.ts"
  - "ui/src/lib/search/recall-header.ts"
  - "ui/src/routes/+layout.svelte"
  - "ui/src/routes/+page.svelte"
  - "ui/src/routes/page.browser.test.ts"
  - "ui/src/routes/search/+page.svelte"
  - "ui/src/routes/search/search.browser.test.ts"
covered_digest: "v2:sha256:e99a8d5dfedf686be9fd6d4244b6d3cf7ec06c41915ae673365933c232e8c753"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: passed
  previous_score: 7/7
  gaps_closed: []
  gaps_remaining: []
  regressions: []
---

# Phase 2: Recall-First Search Verification Report

**Phase Goal:** A user typing a UUID, short_id, or free text into any console search surface gets
an honest, cross-spine result set, with the command palette and search box no longer lying about
what they searched.
**Verified:** 2026-09-28T17:36:53Z
**Status:** passed
**Re-verification:** Yes — the prior VERIFICATION.md (2026-09-27) was stale: `covered_digest`
inputs (`ui/src/routes/search/+page.svelte`, `FacetStrip.svelte`, `ui/src/lib/search/params.ts`,
`classify.ts`, `search.browser.test.ts`, and others) were substantially edited by later
milestone phases (3 Curation RPCs, 4 Curation Surfaces, 5 bulk/related/scoped actions, 6 Query
Understanding). This report re-derives the 7 ROADMAP.md success criteria against current HEAD
(`94c9552a`) rather than trusting the prior report's frozen evidence.

## Goal Achievement

### Observable Truths (ROADMAP.md Success Criteria 1–7)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | UUID/short_id in any search box fetches by id (never a semantic query); free text with no scope defaults to `cross_spine` | ✓ VERIFIED | `ui/src/lib/search/classify.ts` still holds `UUID_RE`/`SHORT_ID_RE` and checks them BEFORE any operator-token parsing added by Phase 6's NLQ work (lines 64–70); the single-token id/short_id branch is unchanged in shape, only extended with additional token-grammar (`scope:`/`tag:`/`is:`/`#`) for free text. Imported by exactly the same three surfaces as before: `HeaderSearch.svelte:18`, `routes/search/+page.svelte:31`, `CommandMenu.svelte:15` (confirmed by grep). `HeaderSearch.svelte:159` still gates `GetMemory` on `classified.kind === 'id' \|\| 'short_id'`. `params.ts:49` still defaults `crossSpine = scope ? false : sp.get('xs') !== '0'`. Targeted run: `HeaderSearch.browser.test.ts` (31/31 passing, includes the UUID hand-off test). |
| 2 | A test asserts the server-driven search surface (`Command.Root`/`CommandPrimitive.Root shouldFilter={false}`) issues a real `SearchMemories` call for a term absent from static labels and never reports "no matches" | ✓ VERIFIED | `HeaderSearch.svelte:397` still `<CommandPrimitive.Root shouldFilter={false} ...>`. Test named exactly `'SC2: a term absent from every static label calls SearchMemories and never shows "no matches"'` still present and passing (targeted run: `npx vitest run --project browser -t "SC2"` → 1 passed). |
| 3 | Every result set states what was searched (resolution path, hit/scope counts, `scopes_truncated`/`scopes_unknown`); empty state names query+coverage; rejection shows `field=<f> hint=<code>` | ✓ VERIFIED | `ui/src/lib/search/recall-header.ts` still exports the `scopesTruncated`/`scopesUnknown` clauses (now duplicated across the search-header and a second scheduled/related header shape added by later phases, but the original shape and copy are unchanged). `ui/src/lib/errors/connect-error.ts:23` still has `ENVELOPE_RE = /^field=([^ ]+) hint=([^:]+): ([\s\S]*)$/`. Targeted run: `recall-header.test.ts`, `params.test.ts`, `classify.test.ts`, `connect-error.test.ts` — 80/80 passing (node project). Go: `TestRecallHiddenListParity`/`TestRecallHiddenSearchParity` still pass against a real store fixture (see Go run below). |
| 4 | A test fires two overlapping queries and asserts the final rendered state matches the later query; prior results stay visible during a debounced fetch | ✓ VERIFIED | `search.browser.test.ts`'s `describe('search route — race safety (SC4)')` block still present (line 154) and passing (targeted run: `-t "race safety"` → 1 passed; full-file run below also green). `placeholderData: keepPreviousData` still wired in `+page.svelte`. |
| 5 | Keyboard traversal (`j`/`k`/arrows), Enter opens, `score` always renders and `relevance` only when reranked, hover never mutates the keyboard-active row | ✓ VERIFIED | `ResultsList.svelte` still tracks `aria-activedescendant` independently of `hoverRowId` (mousemove-driven, not mouseenter; lines 152–349). `ResultRow.svelte` still renders `scoreDisplay` unconditionally and `rel` only when `showRel && memory.relevance !== undefined`. Targeted runs: `ResultsList.browser.test.ts` (44/44), `ResultRow.browser.test.ts` (part of the 81/81 combined run below) — all passing despite `ResultsList.svelte` growing +468 lines from later-phase bulk-select/related-view work. |
| 6 | Removable, URL-persisted filter chips (category/tags/time/derived-state/scope); scope combobox with autocomplete + readable-record counts | ✓ VERIFIED | `params.ts` still round-trips `scope`/`xs`/`cat`/`tag`/`after`/`before`/`inc`/`k`/`sel` through `encodeSearchParams`/`decodeSearchParams` (`params.test.ts` passing). `ScopeCombobox.svelte` still renders `scopes[].count`; the WR-03 retry wiring (`onretry` threaded `+page.svelte:623` → `FacetStrip.svelte:129` → `ScopeCombobox.svelte:73`) is still present and unbroken by the later Tags-panel/facet work. Targeted run: `FacetStrip.browser.test.ts` + `ScopeCombobox.browser.test.ts` (part of the 81/81 combined run below). |
| 7 | The `engram-console-conventions` and `engram-connect-client` skills exist and this phase's UI-SPEC cites them | ✓ VERIFIED | Both `.claude/skills/engram-console-conventions/SKILL.md` and `.claude/skills/engram-connect-client/SKILL.md` still exist. Spot-checked cited path `ui/src/lib/memorystate.ts` (and its test) — exists. `02-UI-SPEC.md` and `CLAUDE.md` still cite both skills by name. |

**Score:** 7/7 truths verified (0 present-but-behavior-unverified)

### Regression Analysis (Phases 3–6 edits to Phase 2's covered files)

`git diff --stat` from the Phase 2 completion commit (`cfa4731f`) to `HEAD` (`94c9552a`) shows 22
files this phase owns were touched again, with the largest deltas in `ResultsList.svelte` (+468),
`search.browser.test.ts` (+1177, almost all new tests), `+page.svelte` (+291), `DetailPane.svelte`
(+85), `connectapi.go`/`tools.go` (+173/+185 — new curation RPCs), and `proto/engram.proto` (+392
— new RPC messages). None of these changes touched the id/short_id classifier's core regex
branch, the `shouldFilter={false}` command surface, the recall-header envelope/clause text, the
`aria-activedescendant`/hover decoupling, the `score`/`rel` rendering rule, the URL param codec, or
the `onretry` wiring chain — the load-bearing lines for all 7 success criteria are unchanged in
shape, only extended alongside them. One intentional, documented removal was found:

- **`/observe` route deleted** (commit `b6e3f91c`, "remove the /observe route; land scope links on
  /search (D-14)", Phase 5 or 6 work). `ui/src/routes/observe/+page.svelte`,
  `observe.browser.test.ts`, and `ui/src/lib/components/ScopesSidebar.svelte` no longer exist.
  This is **not a regression against Phase 2's success criteria**: none of the 7 ROADMAP truths
  name `/observe` as a required surface — they specify "any console search surface" and are
  satisfied by `/search`, the header search box, and the command palette, all three of which are
  unaffected and still carry the classifier/race-safety/keyboard/facet behavior. `covered_files`
  above drops the three deleted files and the phase's specific hashed vendored-SPA build
  filenames (which the `git status --porcelain` check below confirms are still checked in and
  drift-free, just under new content hashes from later rebuilds — tracking a build artifact by its
  exact hashed name is not a meaningful signal here, so only the stable `index.html` anchor is
  kept in `covered_files` going forward).

No other file in the prior `covered_files` list was deleted or renamed.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `proto/engram/v1/engram.proto` | `RecallGateHidden` message, additive fields 5/8 | ✓ VERIFIED | Still present; `go tool buf breaking --against '.git#branch=main'` returns clean at HEAD despite +392 lines of new Phase-3 RPC messages added since. |
| `internal/server/hiddencount.go` | D-01/D-02/D-03 hidden-count core | ✓ VERIFIED | Unchanged core logic; all 5 named tests still pass (see Go run below). |
| `cmd/engram/client_common.go` (`renderRecallHiddenFooter`) | CLI footer line | ✓ VERIFIED | Still wired into `client_search.go:85` and `client_list.go:97`; `TestClientSearchRecallHiddenFooter`/`TestClientListRecallHiddenFooter` pass. |
| `ui/src/lib/search/classify.ts` | Shared id/short_id/text classifier | ✓ VERIFIED | Grown from a 3-branch classifier to include operator-token parsing (Phase 6 NLQ), but the id/short_id branch and its 3 importers are unchanged. |
| `ui/src/lib/components/ResultsList.svelte` | WAI-ARIA listbox, virtualized, keyboard model | ✓ VERIFIED | Grew substantially (bulk-select, related-view keys) but keyboard/hover decoupling logic intact and tested at 44/44. |
| `ui/src/lib/components/DetailPane.svelte` | Stacked sections, all ROW-07 fields | ✓ VERIFIED | Supersession links, schedule window, archive stamp, schema version, citations, both id copy buttons all still present (now alongside newer curation actions). |
| `.claude/skills/engram-console-conventions/SKILL.md`, `.claude/skills/engram-connect-client/SKILL.md` | DSYS-01/02 | ✓ VERIFIED | Both exist; spot-checked cited paths resolve. |
| `internal/webauth/static/` (vendored SPA) | Reflects current source, no drift | ✓ VERIFIED | `git status --porcelain` on `ui/` and `internal/webauth/static/` is clean at HEAD — the checked-in vendored bundle matches the source that produced it (rebuilt and committed by later phases, not phase 2 itself). |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `HeaderSearch.svelte` / `routes/search/+page.svelte` / `CommandMenu.svelte` | `ui/src/lib/search/classify.ts` | shared `classifyInput()` import | ✓ WIRED | Confirmed via import grep on all three files at HEAD. |
| `internal/server/connectapi.go` (Connect) & `internal/server/tools.go` (MCP) | `internal/server/hiddencount.go` | `(*deps).listRecallHidden`/`searchRecallHidden` called once, shared `toProto`/`withRecallHidden` | ✓ WIRED | `TestRecallHiddenListParity`/`SearchParity` pass at HEAD despite `connectapi.go`/`tools.go` growing +173/+185 lines for Phase 3's new RPCs. |
| `cmd/engram/client_search.go` / `client_list.go` | `cmd/engram/client_common.go` | `renderRecallHiddenFooter(...)` call | ✓ WIRED | Call sites unchanged at `client_search.go:85`, `client_list.go:97`. |
| `ScopeCombobox.svelte` (Retry button) | `+page.svelte`'s `scopesQ.refetch()` | `onretry` prop threaded through `FacetStrip.svelte` | ✓ WIRED | Chain intact at HEAD (`+page.svelte:623` → `FacetStrip.svelte:129` → `ScopeCombobox.svelte:73`) despite `FacetStrip.svelte` growing +90 lines for the Tags-panel work. |
| `ResultsList.svelte` row action keys | `WriteSurfaces.svelte` handlers (edit/visibility/delete) | `onedit`/`onvisibility`/`ondelete` props | ✓ WIRED | Rule/discovery fences still present; unaffected by the later curation-action additions layered alongside. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|---------------------|--------|
| `ScopeCombobox.svelte` scope counts | `s.count` | `ListScopesResponse.scopes[]` via `scopesQ` in `+page.svelte` | Yes — real RPC, no static fallback | ✓ FLOWING |
| `ResultsHeader`/`RecallState` hidden-count clause | `recall_gate_hidden.total` | `SearchMemoriesResponse`/`ListMemoriesResponse.recallGateHidden`, computed server-side | Yes | ✓ FLOWING |
| `ResultRow.svelte` score/rel columns | `memory.score` / `memory.relevance` | Proto response fields from the actual `SearchMemories` call | Yes | ✓ FLOWING |
| CLI footer | `resp.Msg.GetRecallGateHidden()` | Same server-computed field over Connect | Yes | ✓ FLOWING |

No hardcoded/static fallbacks found in any of the re-inspected rendered dynamic surfaces.

### Behavioral Spot-Checks (re-run against HEAD `94c9552a`)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| SC1/ENTRY-01 UUID hand-off | `pnpm --dir ui vitest run --project browser src/lib/components/HeaderSearch.browser.test.ts` | 31/31 passed | ✓ PASS |
| SC2 (server-driven search, no false "no matches") | `pnpm --dir ui vitest run --project browser -t "SC2"` | 1 passed / 640 skipped | ✓ PASS |
| SC4 (stale-response race) | `pnpm --dir ui vitest run --project browser src/routes/search/search.browser.test.ts -t "race safety"` | 1 passed | ✓ PASS |
| Full `search.browser.test.ts` file (69 tests) | `pnpm --dir ui vitest run --project browser src/routes/search/search.browser.test.ts` | 68 passed, 1 failed on first run (D-06 Chain-button toolbar test — `TimeoutError` under concurrent host load) | ⚠️ see below |
| Same D-06 test in isolation | `pnpm --dir ui vitest run --project browser src/routes/search/search.browser.test.ts -t "a row toolbar Chain button opens the chain dialog for that anchor"` | 1 passed | ✓ PASS |
| `ResultsList.browser.test.ts` (keyboard/hover/virtualization) | `pnpm --dir ui vitest run --project browser src/lib/components/ResultsList.browser.test.ts` | 44/44 passed | ✓ PASS |
| `FacetStrip` + `ScopeCombobox` + `ResultRow` + `ResultsHeader` + `CommandMenu` browser tests | `pnpm --dir ui vitest run --project browser <5 files>` | 81/81 passed | ✓ PASS |
| `DetailPane.browser.test.ts` | `pnpm --dir ui vitest run --project browser src/lib/components/DetailPane.browser.test.ts` | 19/19 passed | ✓ PASS |
| `recall-header`/`params`/`classify`/`connect-error` unit tests | `pnpm --dir ui vitest run --project node <4 files>` | 80/80 passed | ✓ PASS |
| Go hidden-count core + parity (real Qdrant testcontainer) | `go test ./internal/server/... -run "TestRecallHidden\|TestCountRecallHidden" -v` | all subtests PASS, `ok` | ✓ PASS |
| CLI footer regression tests | `go test ./cmd/engram/... -run "TestSearch\|TestList\|Footer" -v` | all PASS, `ok` | ✓ PASS |
| `buf breaking` (proto compatibility) | `go tool buf breaking --against '.git#branch=main'` | No output (clean) at HEAD | ✓ PASS |
| `go build ./...` | full workspace build | clean, no errors | ✓ PASS |
| Vendored SPA drift | `git status --porcelain internal/webauth/static/ ui/` | clean | ✓ PASS |

**Note on the D-06 Chain-button test failure:** the task brief flagged this exact test
(`search.browser.test.ts > chain dialog entry points (D-06) > a row toolbar Chain button...`) as a
known, pre-existing load flake — it times out only when the full 69-test file runs under
concurrent host load (five other phase verifiers were running in parallel during this session) and
passes reliably alone. Reproduced exactly that signature here: full-file run failed only this one
test with a Playwright click-timeout on an element that "was detached from the DOM, retrying"
(classic resource-contention symptom, not a logic assertion failure), and the identical test passed
immediately in isolation. Treated as the documented flake, not a regression — no gap recorded.

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|---|---|---|---|---|
| ENTRY-01 | 02-04, 02-05, 02-08 | UUID/short_id fetched by id on every surface | ✓ SATISFIED | Classifier + `GetMemory` branch unchanged and passing at HEAD |
| ENTRY-02 | 02-04, 02-08 | Free text defaults `cross_spine`, scope as facet | ✓ SATISFIED | `params.ts` default rule unchanged, tested |
| ENTRY-03 | 02-01, 02-02, 02-08, 02-09 | Honest what-was-searched incl. hidden count | ✓ SATISFIED | `recall-header.ts`, `RecallState.svelte`, `hiddencount.go` all pass at HEAD |
| ENTRY-04 | 02-04, 02-05 | Server-driven palette, no false "no matches" | ✓ SATISFIED | SC2 named test passing |
| ENTRY-05 | 02-04, 02-09 | `field=<f> hint=<code>` rejection envelope | ✓ SATISFIED | `connect-error.ts` `ENVELOPE_RE` unchanged, tested |
| ENTRY-06 | 02-04, 02-08 | URL-persisted search state, debounce, abort, keepPreviousData | ✓ SATISFIED | `params.ts` codec unchanged; SC4 race test passing |
| ROW-01 | 02-03, 02-06, 02-09, 02-10 | Dense virtualized 1-line rows, smooth at 1000 | ✓ SATISFIED | `ResultsList.browser.test.ts` 44/44 passing at HEAD |
| ROW-02 | 02-06 | Hover card overlay, decoupled from keyboard-active row | ✓ SATISFIED | mousemove-driven hover state, decoupled from `aria-activedescendant`, unchanged |
| ROW-03 | 02-06 | Keyboard row actions with rule/discovery fence | ✓ SATISFIED | Fence logic unchanged, tested |
| ROW-04 | 02-06, 02-08 | score always, rel only when reranked | ✓ SATISFIED | `ResultRow.svelte` derived values unchanged |
| ROW-05 | 02-08 | Removable URL-persisted facet chips | ✓ SATISFIED | `params.ts`/`FacetStrip.svelte` round-trip intact |
| ROW-06 | 02-08 | Scope combobox with readable-record counts | ✓ SATISFIED | `ScopeCombobox.svelte`, `onretry` chain intact |
| ROW-07 | 02-07, 02-10 | Stacked detail pane, all required fields | ✓ SATISFIED | `DetailPane.svelte` fields present, 19/19 tests passing |
| DSYS-01 | 02-10 | `engram-console-conventions` skill | ✓ SATISFIED | Skill exists, spot-checked cited path resolves |
| DSYS-02 | 02-10 | `engram-connect-client` skill | ✓ SATISFIED | Skill exists |

All 15 requirement IDs mapped to Phase 2 in `.planning/REQUIREMENTS.md`'s traceability table are
still claimed by exactly one plan's `requirements` frontmatter within Phase 2 and no other phase's
`PLAN.md` claims any of them (checked by grepping every other phase's `requirements:` frontmatter
block for these 15 IDs — the one incidental text match in `04-02-PLAN.md` was prose, not a
frontmatter `requirements:` entry). No orphaned or duplicated requirements found. The
`requirements.mark-complete` limitation noted in the prior verification (milestone-wide,
`deferred-items.md`) is unchanged and still not caused by this phase.

### Anti-Patterns Found

None. Re-scanned all 111 files in the current `covered_files` list for `TBD`/`FIXME`/`XXX`
(0 hits), plus `TODO`/`HACK`/`PLACEHOLDER` and "coming soon"/"not yet implemented" copy (0 hits
beyond the legitimate HTML `placeholder` attribute and TanStack's `placeholderData` API name, as in
the prior verification).

### Known, Documented Deviations (Not Phase-2 Blockers)

- **WINDOWS.md #16:** Tab does not cycle header-search dropdown sections — unchanged, still
  non-blocking (no ROADMAP success criterion names Tab-cycling).
- **WINDOWS.md #17 / `deferred-items.md`:** `requirements.mark-complete` cannot flip this
  milestone's seeded `Mapped` traceability rows to `Complete` — pre-existing, milestone-wide,
  unchanged since the prior verification.
- **New since prior verification, not a Phase-2 blocker:** `/observe` route removed (D-14,
  Phase 5/6) — see Regression Analysis above. Does not touch any of the 7 success criteria.

### Human Verification Required

None. Every truth in ROADMAP.md's Success Criteria 1–7 is still backed by a passing automated test
re-run at current HEAD, exercising the actual behavior rather than presence/wiring inspection
alone.

### Gaps Summary

No gaps. Re-verification against HEAD (`94c9552a`) confirms all 7 ROADMAP success criteria still
hold after three subsequent milestone phases (3, 4, 5, 6 in progress) made substantial edits to
Phase 2's shared surfaces (`+page.svelte`, `FacetStrip.svelte`, `ResultsList.svelte`,
`connectapi.go`/`tools.go`, `proto/engram.proto`). The one structural change found — `/observe`'s
deletion in favor of `/search` (D-14) — is an intentional, documented consolidation that does not
touch any Phase 2 success criterion. All 15 mapped requirements remain satisfied, no new debt
markers were introduced, `buf breaking`/`go build`/vendored-SPA-drift all remain clean, and the one
test failure observed (`D-06` Chain-button toolbar test under full-suite concurrent-host load) is
the documented pre-existing flake, confirmed to pass in isolation, not a regression.

---

_Verified: 2026-09-28T17:36:53Z_
_Verifier: Claude (gsd-verifier)_
