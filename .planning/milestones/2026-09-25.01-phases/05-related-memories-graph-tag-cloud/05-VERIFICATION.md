---
phase: 05-related-memories-graph-tag-cloud
verified: 2026-09-28T13:35:00Z
status: passed
score: 5/5 roadmap+requirement truths verified (plus representative plan-level must-haves sampled below)
covered_files: [".planning/phases/05-related-memories-graph-tag-cloud/05-01-PLAN.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-01-SUMMARY.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-02-PLAN.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-02-SUMMARY.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-03-PLAN.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-03-SUMMARY.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-04-PLAN.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-04-SUMMARY.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-05-PLAN.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-05-SUMMARY.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-06-PLAN.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-06-SUMMARY.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-07-PLAN.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-07-SUMMARY.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-08-PLAN.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-08-SUMMARY.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-09-PLAN.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-09-SUMMARY.md", "internal/e2e/console_browser_test.go", "internal/webauth/static/index.html", "ui/src/lib/components/CommandMenu.svelte", "ui/src/lib/components/DetailPane.svelte", "ui/src/lib/components/EvidenceSection.svelte", "ui/src/lib/components/FacetStrip.svelte", "ui/src/lib/components/GraphLegend.svelte", "ui/src/lib/components/HeaderSearch.svelte", "ui/src/lib/components/RecallSplit.svelte", "ui/src/lib/components/RelatedGraph.svelte", "ui/src/lib/components/ResultsList.svelte", "ui/src/lib/components/SupersessionLane.svelte", "ui/src/lib/components/TagBars.svelte", "ui/src/lib/components/TagCombobox.svelte", "ui/src/lib/components/TagMatchRow.svelte", "ui/src/lib/related/graph.ts", "ui/src/lib/related/lanes.ts", "ui/src/lib/related/zoom.ts", "ui/src/lib/search/related-params.ts", "ui/src/lib/tags/query.ts", "ui/src/lib/tags/tags.ts", "ui/src/routes/related/[id]/+page.svelte", "ui/src/routes/search/+page.svelte"]
covered_digest: "v2:sha256:2b647f86c299bffdc9d03cbc9066010cb09427f041bdcbae97ead12409936c2b"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: passed
  previous_score: "9/9 roadmap+requirement truths verified"
  reason: "Previous 05-VERIFICATION.md (verified 2026-09-28T01:50:00Z, commit a328c8a6) was stale: Phase 6 (Query Understanding) subsequently edited shared console files (ui/src/routes/search/+page.svelte, internal/e2e/console_browser_test.go, the vendored SPA rebuild) that overlap Phase 5's docked Tags panel and e2e harness. Re-ran the full must-have set against current HEAD."
  gaps_closed: []
  gaps_remaining: []
  regressions: []
---

# Phase 5: Related-Memories Graph & Tag Cloud Verification Report

**Phase Goal:** A user can browse a record's local related-memories neighbourhood and the scope's
tag cloud, both fully keyboard/ARIA-equivalent. (Criterion 4 amended 2026-09-27 from a
quantile-sized cloud to a linear-bar tag popularity list, Phase 5 D-12.)
**Verified:** 2026-09-28T13:35:00Z
**Status:** passed
**Re-verification:** Yes — Phase 6 edited shared console files overlapping Phase 5's scope; regenerated against current HEAD (branch `feat/2026-09-25.01`, `internal/e2e/console_browser_test.go`/`internal/webauth/static/index.html`/`ui/src/routes/search/+page.svelte` changed since the prior verification's commit `a328c8a6`)

## What Changed Since the Prior Verification (Regression Analysis)

`git diff a328c8a661afaa1ed570eba976ec47344e046829..HEAD` touches exactly three files that
overlap Phase 5's `covered_files`:

| File | Nature of Phase 6 change | Effect on Phase 5 must-haves |
|---|---|---|
| `ui/src/routes/search/+page.svelte` | Additive: imports `understand.ts`, adds the `SuggestedRow` component, a `searchInputEl` binding, an `UnderstandQuery` TanStack query, per-q dismissal state, and an `aria-live` announcement region for suggested chips. Existing `FacetStrip` props (`tagsPanelOpen`, `ontagspanel`) and the docked Tags panel markup are untouched — the diff is a pure insertion block plus one new `{#if visibleSuggested.length > 0}` block after `FacetStrip`. | None — Tags panel wiring is byte-for-byte where it was; verified below. |
| `internal/e2e/console_browser_test.go` | Additive: introduces `startConsoleServerWithEnv` (generalizes `startConsoleServer`, which every existing caller — including `TestConsoleRelatedView` — still calls unchanged via `startConsoleServer(t)` → `startConsoleServerWithEnv(t, nil)`), plus a new `TestConsoleQueryUnderstanding` test and its fake Decisions-API server, appended after `TestConsoleRelatedView`. | None — `TestConsoleRelatedView`'s own body is untouched; re-ran it live, see below. |
| `internal/webauth/static/index.html` | Vendored SPA rebuild reflecting the `+page.svelte` change above. | None — confirmed zero `task ui:build` drift against current source. |

No other file in Phase 5's `covered_files` list changed. `ui/src/routes/related/[id]/+page.svelte`, `RelatedGraph.svelte`, `TagBars.svelte`, `TagCombobox.svelte`, `HeaderSearch.svelte`, `FacetStrip.svelte`, `related/graph.ts`, `tags/tags.ts`, `tags/query.ts` etc. are all byte-identical to the prior verification's commit.

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria + Requirement IDs)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Opening the graph from a record renders that record plus its neighbourhood only (never global), as inline SVG with d3-force, pan/zoom/drag, fixed settle budget, capped edges/node, legend + per-type toggles (GRAPH-01) | ✓ VERIFIED | `ui/src/lib/related/graph.ts` unchanged since prior verification (`settleLayout`/`SETTLE_TICKS=300`/`lcg(42)`); `RelatedGraph.svelte` unchanged. Re-ran live: `ENGRAM_REQUIRE_BROWSER=1 go test ./internal/e2e/... -run TestConsoleRelatedView -v` → `--- PASS: TestConsoleRelatedView (6.91s)` against a self-booted testcontainers Qdrant. |
| 2 | Every graph node reachable by keyboard (Tab/arrows, Enter to focus+re-centre, Escape to return), accessible name, `aria-live` textual neighbourhood list (GRAPH-02) | ✓ VERIFIED | `RelatedGraph.svelte` unchanged; `aria-activedescendant` mirrors `activeId`, `neighbourhoodSummary()` feeds `aria-live="polite"`. Re-ran `RelatedGraph.browser.test.ts` this session — passing (part of the 112 route+component tests below). |
| 3 | Graph renders correctly in light/dark using category colour tokens; clicking a node selects it (GRAPH-03) | ✓ VERIFIED | `fill="var(--cat-{n.category})"` unchanged; click → `onselect(id)` unchanged. Re-ran the AA audit suite this session (26/26 passing, zero axe-core violations, both themes). |
| 4 | Tag popularity list as linear bars from zero with printed counts, DOM order = reading order, built from `ListTags`, click adds filter chip, same counts via chip autocomplete (TAGS-01/TAGS-02, amended D-12) | ✓ VERIFIED | `TagBars.svelte`/`TagCombobox.svelte`/`tags.ts`/`tags/query.ts` byte-identical to prior verification. The one shared file that DID change, `ui/src/routes/search/+page.svelte`, keeps its `FacetStrip` props (`{tagsPanelOpen}`, `ontagspanel`) and docked-panel markup untouched — confirmed by diff inspection and by re-running `search.browser.test.ts`'s "docked Tags panel toggle (TAGS-01, D-13)" and "shares the slot with the detail pane" describe blocks this session (69/69 passing in the file, including the `#tag` URL-filter toggle test and the chip-vs-suggested-chip parity test). |
| 5 | `internal/store` STORE-02/STORE-03 dependency intact — RelatedMemories/ListTags RPCs wired end to end from UI to server | ✓ VERIFIED | Live e2e round trip (`TestConsoleRelatedView`, re-run this session) proves the full stack unchanged: SPA → Connect RPC → `internal/store` → Qdrant → rendered graph + vector lane. |

**Score:** 5/5 roadmap-level truths verified (0 present-but-behavior-unverified, 0 overrides, 0 regressions from Phase 6).

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|---|---|---|---|---|
| GRAPH-01 | 05-01, 05-09 | Local related-memories graph, inline SVG, d3-force, pan/zoom/drag, settle budget, capped edges, legend+toggles | ✓ SATISFIED | See Truth 1; live e2e re-run passes |
| GRAPH-02 | 05-01, 05-05 | Full keyboard reachability, accessible names, `aria-live` equivalent list | ✓ SATISFIED | See Truth 2; `RelatedGraph.browser.test.ts` re-run passing |
| GRAPH-03 | 05-01, 05-04, 05-09 | Light/dark category tokens; click selects in detail equivalent | ✓ SATISFIED | See Truth 3; AA audit re-run passing |
| TAGS-01 | 05-03, 05-07, 05-08 | Tag popularity linear-bar list from `ListTags`, DOM order = reading order, click adds filter | ✓ SATISFIED | See Truth 4; `TagBars.browser.test.ts` + `search.browser.test.ts` docked-panel tests re-run passing |
| TAGS-02 | 05-03, 05-06, 05-07 | Tag filter autocomplete over `ListTags` with counts | ✓ SATISFIED | `TagCombobox.svelte`, `HeaderSearch.svelte` Tags group unchanged; `TagCombobox.browser.test.ts` (31/31 `HeaderSearch.browser.test.ts`, part of the graph+bars+combobox 112-test run) re-run passing |

REQUIREMENTS.md still maps all five IDs (GRAPH-01/02/03, TAGS-01/02) to Phase 5 with status `Mapped` — unchanged since prior verification, still a pre-existing `gsd-tools` vocabulary display quirk (`deferred-items.md`), not evidence of missing implementation. No ORPHANED requirements.

### Required Artifacts (unchanged files re-confirmed present; changed file re-verified below)

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `ui/src/lib/related/graph.ts` | pure neighbourhood model, membership, edges, layout | ✓ VERIFIED | unchanged since prior verification; 90 pure-model tests re-run passing |
| `ui/src/lib/components/RelatedGraph.svelte` | Svelte-owned inline-SVG graph | ✓ VERIFIED | unchanged; browser tests re-run passing |
| `ui/src/routes/related/[id]/+page.svelte` | `/related/<id>` route | ✓ VERIFIED | unchanged; route browser tests re-run passing (51 tests) |
| `ui/src/lib/search/related-params.ts` | `/related` URL codec | ✓ VERIFIED | unchanged; unit tests re-run passing |
| `ui/src/lib/tags/query.ts` | one `ListTags` query per scope key | ✓ VERIFIED | unchanged |
| `ui/src/lib/tags/tags.ts` | pure tag slicing/ranking/copy | ✓ VERIFIED | unchanged; unit tests re-run passing |
| `ui/src/lib/components/TagBars.svelte` | shared tag popularity list | ✓ VERIFIED | unchanged; browser tests re-run passing |
| `ui/src/lib/components/TagCombobox.svelte` | `+ tag` picker | ✓ VERIFIED | unchanged; browser tests re-run passing |
| `ui/src/lib/related/lanes.ts` | pure lane/evidence/empty-state copy | ✓ VERIFIED | unchanged |
| `ui/src/lib/components/EvidenceSection.svelte` | evidence under the graph | ✓ VERIFIED | unchanged |
| `ui/src/lib/components/SupersessionLane.svelte` | supersession timeline lane | ✓ VERIFIED | unchanged |
| `ui/src/lib/components/GraphLegend.svelte` | legend toggles shared with lane hide buttons | ✓ VERIFIED | unchanged |
| `ui/src/lib/related/zoom.ts` | pure zoom gate/fit/readout math | ✓ VERIFIED | unchanged |
| `ui/src/lib/components/HeaderSearch.svelte` | header search Tags group | ✓ VERIFIED | unchanged (zero-line diff against prior commit); 31/31 tests re-run passing |
| `ui/src/lib/components/FacetStrip.svelte` | `+ tag` picker + Tags panel toggle | ✓ VERIFIED | unchanged; 15/15 tests re-run passing |
| `ui/src/lib/components/RecallSplit.svelte` | narrow-breakpoint flag | ✓ VERIFIED | unchanged |
| `internal/e2e/console_browser_test.go` | live `/related` round trip | ✓ VERIFIED | `TestConsoleRelatedView` body byte-identical; new Phase 6 test (`TestConsoleQueryUnderstanding`) appended after it, `startConsoleServer` back-compat preserved via `startConsoleServerWithEnv(t, nil)`; ran `TestConsoleRelatedView` myself, PASS |
| `internal/webauth/static/index.html` | vendored console SPA incl. `/related` | ✓ VERIFIED | rebuilt via `task ui:build` this session — zero `git status --porcelain` drift against current source |
| `ui/src/routes/search/+page.svelte` | docked `/search` Tags panel host | ✓ VERIFIED | changed by Phase 6 (additive Suggested-row feature) but Tags-panel-relevant code (`FacetStrip` props, docked panel markup) untouched; re-run `search.browser.test.ts` in isolation: 69/69 passing including all Tags-panel-specific tests |

### Key Link Verification (unchanged links re-confirmed; new link inspected)

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `+page.svelte` (`/related`) | `client.ts` | one `RelatedMemories` read | ✓ WIRED | unchanged |
| `+page.svelte` | `related/graph.ts` | `visibleMembership` | ✓ WIRED | unchanged |
| `RelatedGraph.svelte` | `related/graph.ts` | `settleLayout` | ✓ WIRED | unchanged |
| `ResultsList.svelte` | route host | `onrelated` (row key `r`) | ✓ WIRED | unchanged |
| `CommandMenu.svelte` | route | `Related to {short_id}` ⌘K item | ✓ WIRED | unchanged |
| `tags/query.ts` | `client.ts` | `ListTags`, limit 1000 | ✓ WIRED | unchanged |
| `TagBars.svelte` | `tags/query.ts` | shared cached query | ✓ WIRED | unchanged |
| `TagCombobox.svelte` | `tags/tags.ts` | `rankTagMatches` | ✓ WIRED | unchanged |
| `/search +page.svelte` | `TagBars.svelte` | docked panel, `mode="panel"` | ✓ WIRED | re-confirmed: `{tagsPanelOpen}`/`ontagspanel` props to `FacetStrip` and the panel-mode `TagBars` mount are present, byte-identical region of the file, below Phase 6's new insertion |
| `/search +page.svelte` | `SuggestedRow.svelte` (Phase 6, new) | new suggested-chip row rendered after `FacetStrip` | ✓ WIRED (Phase 6 scope) | new, additive, does not replace or gate the Tags panel |
| `FacetStrip.svelte` | `TagCombobox.svelte` | `+ tag` picker | ✓ WIRED | unchanged |
| `HeaderSearch.svelte` | `tags/query.ts` + `tags/tags.ts` | Tags autocomplete group | ✓ WIRED | unchanged |
| `console_browser_test.go` | `internal/webauth/static/index.html` | e2e loads vendored SPA | ✓ WIRED | re-confirmed live |

No orphaned artifacts; no write-client (`engramWrite`) reference in the `/related` route, `RelatedGraph.svelte`, `TagBars.svelte`, or `TagCombobox.svelte` (re-checked this session, zero matches) — the safety prohibition still holds.

### Behavioral Spot-Checks / Live Verification (ran myself this session against current HEAD)

| Behavior | Command | Result | Status |
|---|---|---|---|
| Live chromedp round trip through a real binary + self-booted Qdrant | `ENGRAM_REQUIRE_BROWSER=1 go test ./internal/e2e/... -run TestConsoleRelatedView -v` | `--- PASS: TestConsoleRelatedView (6.91s)` | ✓ PASS |
| Pure related-model unit tests (graph/lanes/zoom/tags/related-params) | `pnpm exec vitest run src/lib/related/*.test.ts src/lib/tags/tags.test.ts src/lib/search/related-params.test.ts` | 5 files / 90 tests passed | ✓ PASS |
| Component browser tests: related route, graph, tag bars, tag combobox | `pnpm exec vitest run --project browser src/routes/related/related.browser.test.ts src/lib/components/RelatedGraph.browser.test.ts src/lib/components/TagBars.browser.test.ts src/lib/components/TagCombobox.browser.test.ts` | 4 files / 112 tests passed | ✓ PASS |
| `HeaderSearch.browser.test.ts` (Tags group, unchanged file) | `pnpm exec vitest run --project browser src/lib/components/HeaderSearch.browser.test.ts` | 31/31 passed | ✓ PASS |
| `FacetStrip.browser.test.ts` (`+ tag` picker host, unchanged file) | `pnpm exec vitest run --project browser src/lib/components/FacetStrip.browser.test.ts` | 15/15 passed | ✓ PASS |
| `/search` route full file, incl. docked Tags panel + `#tag` operator toggle (the file Phase 6 changed) | `pnpm exec vitest run --project browser src/routes/search/search.browser.test.ts` | 69/69 passed | ✓ PASS |
| WCAG 2.2 AA audit suite (axe-core, both themes) for all Phase 5 surfaces | `pnpm exec vitest run --project browser src/lib/a11y/surfaces.browser.test.ts` | 26/26 passed | ✓ PASS |
| Known pre-existing load flake, re-confirmed isolated-pass | `vitest run --project browser src/routes/search/search.browser.test.ts -t "Chain button opens the chain dialog"` | 1/1 passed | ✓ PASS (confirms this is a load-only flake under the full suite, not a regression from Phase 6's additions to this same file) |
| `go build ./...` | — | clean | ✓ PASS |
| `task ui:build` reproduces vendored SPA | `git status --porcelain internal/webauth/static` before and after rebuild | 0 lines both times (no drift) | ✓ PASS |
| Debt-marker scan (`TBD`/`FIXME`/`XXX`) across all `covered_files` | `rg` per file | none found | ✓ PASS |

### Anti-Patterns Found (carried forward from `05-REVIEW.md`; re-checked for Phase 6 impact)

| File | Line | Pattern | Severity | Impact | Phase 6 impact |
|---|---|---|---|---|---|
| `RelatedGraph.svelte` | 77 | Hand-copied `TYPE_LETTER` glyph map duplicating `related/lanes.ts`'s exported `LANE_GLYPH` (WR-01) | ℹ️ Info | No functional break; a future edge-type addition could desync focus-card glyph from legend/evidence | File unchanged; unaffected |
| `RelatedGraph.svelte` | 248-259 | Auto-pan effect can re-trigger from an unrelated node's drag-induced simulation reheat (WR-02) | ⚠️ Warning | Narrow interaction edge case; does not block core keyboard/ARIA equivalence | File unchanged; unaffected |
| `TagBars.svelte` | 148-174 | Roving `aria-activedescendant` listbox gives no visible focus ring for sighted keyboard users (WR-03) | ⚠️ Warning | Screen-reader equivalence intact; sighted-keyboard visual affordance gap | File unchanged; unaffected |
| `+page.svelte` (`/related`) | 204-210 | `goBack()` calls `history.back()` even on a bookmarked/direct URL with no real history (WR-04) | ⚠️ Warning | Edge case: bookmarked-trail "back" can strand user outside the app | File unchanged; unaffected |
| `RelatedGraph.svelte` | 349-359 | `cardPlacement` hardcodes `VIEW`'s half-extents (IN-01) | ℹ️ Info | Cosmetic maintainability nit | File unchanged; unaffected |
| `+page.svelte` | 288 | Skeleton call-line hardcodes `k=64` instead of interpolating `RELATED_K` (IN-02) | ℹ️ Info | Cosmetic maintainability nit | File unchanged; unaffected |

`05-REVIEW-DISPOSITION.md` (re-checked this session, `recorded: 2026-09-28T05:38:41.959Z`) still lists all six findings as `open`/6 total; none fixed, none filed as a GitHub issue (`gh issue list` search for these ids/titles found no match this session — the phase-15 issues #357/#358 reuse the WR-01..WR-03 ids for unrelated findings). None of these are BLOCKER-tier and none fail a must-have; carried forward unchanged.

### Human Verification Required

None. Re-ran the full automated evidence set (AA audit suite, live e2e round trip, all targeted component/unit test files) against current HEAD rather than trusting either the original SUMMARY or the prior VERIFICATION's claims.

### Gaps Summary

No gaps. No regression from Phase 6's edits to the three overlapping files: `ui/src/routes/search/+page.svelte`'s change is a pure addition after the existing Tags-panel markup, `internal/e2e/console_browser_test.go`'s change is a pure addition after `TestConsoleRelatedView` with `startConsoleServer` back-compat preserved, and `internal/webauth/static/index.html` is a clean, drift-free rebuild. The six open code-review findings (WR-01 through WR-04, IN-01, IN-02) carried forward from the initial verification remain open, unfixed, and unfiled as GitHub issues — same recommendation as before: file a follow-up issue for WR-02/WR-03/WR-04 before the next accessibility-focused phase.

---

_Verified: 2026-09-28T13:35:00Z_
_Verifier: Claude (gsd-verifier)_
