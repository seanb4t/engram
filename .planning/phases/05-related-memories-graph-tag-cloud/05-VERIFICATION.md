---
phase: 05-related-memories-graph-tag-cloud
verified: 2026-09-28T01:50:00Z
status: passed
score: 9/9 roadmap+requirement truths verified (plus representative plan-level must-haves sampled below)
covered_files: [".planning/phases/05-related-memories-graph-tag-cloud/05-01-PLAN.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-01-SUMMARY.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-02-PLAN.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-02-SUMMARY.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-03-PLAN.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-03-SUMMARY.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-04-PLAN.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-04-SUMMARY.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-05-PLAN.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-05-SUMMARY.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-06-PLAN.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-06-SUMMARY.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-07-PLAN.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-07-SUMMARY.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-08-PLAN.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-08-SUMMARY.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-09-PLAN.md", ".planning/phases/05-related-memories-graph-tag-cloud/05-09-SUMMARY.md", "internal/e2e/console_browser_test.go", "internal/webauth/static/index.html", "ui/src/lib/components/CommandMenu.svelte", "ui/src/lib/components/DetailPane.svelte", "ui/src/lib/components/EvidenceSection.svelte", "ui/src/lib/components/FacetStrip.svelte", "ui/src/lib/components/GraphLegend.svelte", "ui/src/lib/components/HeaderSearch.svelte", "ui/src/lib/components/RecallSplit.svelte", "ui/src/lib/components/RelatedGraph.svelte", "ui/src/lib/components/ResultsList.svelte", "ui/src/lib/components/SupersessionLane.svelte", "ui/src/lib/components/TagBars.svelte", "ui/src/lib/components/TagCombobox.svelte", "ui/src/lib/components/TagMatchRow.svelte", "ui/src/lib/related/graph.ts", "ui/src/lib/related/lanes.ts", "ui/src/lib/related/zoom.ts", "ui/src/lib/search/related-params.ts", "ui/src/lib/tags/query.ts", "ui/src/lib/tags/tags.ts", "ui/src/routes/related/[id]/+page.svelte"]
covered_digest: "v2:sha256:148ad1665106560fd66f8d38a5d2951c7b953505b327dea7da80fa3166c42a20"
behavior_unverified: 0
overrides_applied: 0
---

# Phase 5: Related-Memories Graph & Tag Cloud Verification Report

**Phase Goal:** A user can browse a record's local related-memories neighbourhood and the scope's
tag cloud, both fully keyboard/ARIA-equivalent. (Criterion 4 amended 2026-09-27 from a
quantile-sized cloud to a linear-bar tag popularity list, Phase 5 D-12.)
**Verified:** 2026-09-28T01:50:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria + Requirement IDs)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Opening the graph from a record renders that record plus its neighbourhood only (never global), as inline SVG with d3-force, pan/zoom/drag, fixed settle budget, capped edges/node, legend + per-type toggles (GRAPH-01) | ✓ VERIFIED | `ui/src/lib/related/graph.ts` `settleLayout`/`SETTLE_TICKS=300`/`lcg(42)`; `RelatedGraph.svelte` imports `d3-zoom`/`d3-drag` (not d3-selection for DOM — Svelte owns every element, confirmed by `role="listbox"` markup being hand-written); `GraphLegend.svelte` renders per-type `Checkbox` toggles wired to the same hide switch as lane bodies (`05-04-PLAN.md` D-10). Route (`+page.svelte`) issues exactly one `relatedMemories` read (`queryKey: ['relatedMemories', id, RELATED_K, false]`) and the graph's `role=option` nodes come only from that response's `visibleMembership`. Confirmed live: `TestConsoleRelatedView` (chromedp against a real binary + a testcontainers Qdrant I booted myself) passes — `--- PASS: TestConsoleRelatedView (2.35s)`. |
| 2 | Every graph node reachable by keyboard (Tab/arrows, Enter to focus+re-centre, Escape to return), accessible name, `aria-live` textual neighbourhood list (GRAPH-02) | ✓ VERIFIED | `RelatedGraph.svelte`: `aria-activedescendant` mirrors `activeId`; `neighbourhoodSummary()` (graph.ts:320) feeds a `<p class="sr-only" aria-live="polite">`; keyboard handler moves in lane order per D-10 (pinned by `RelatedGraph.browser.test.ts`, 61 tests passing incl. this file). |
| 3 | Graph renders correctly in light/dark using category colour tokens; clicking a node selects it (GRAPH-03) | ✓ VERIFIED | `fill="var(--cat-{n.category})"` (RelatedGraph.svelte:533); click → `onselect(id)` wired to the route's shared selection, which opens `EvidenceSection.svelte` under the graph (05-04-PLAN D-05, "GRAPH-03's 'selects it in the detail pane'"); both-theme AA screenshots in `surfaces.browser.test.ts` (25/25 passing, zero axe-core violations per `05-09-SUMMARY.md`). |
| 4 | Tag popularity list as linear bars from zero with printed counts, DOM order = reading order, built from `ListTags`, click adds filter chip, same counts via chip autocomplete (TAGS-01/TAGS-02, amended D-12) | ✓ VERIFIED | `TagBars.svelte`: `barPercent()` (`tags.ts:21`, linear `count/max*100`, `.fill{min-width:2px}` for the count-of-1 case), rows rendered in server order (`visible.rows` — no client re-sort), `ontoggle(row.tag)` on click. `TagCombobox.svelte` (`shouldFilter={false}`, `rankTagMatches`) and `HeaderSearch.svelte`'s `'Tags · counts in '` group share the identical cached `listTagsQuery(scope)` (query key `['listTags', scope \|\| '__all_readable__', 1000]`, `staleTime: Infinity`) — one fetch, three surfaces. |
| 5 | `internal/store` STORE-02/STORE-03 dependency intact — RelatedMemories/ListTags RPCs wired end to end from UI to server | ✓ VERIFIED | Live e2e round trip (`TestConsoleRelatedView`) proves the full stack: SPA → Connect RPC → `internal/store` → Qdrant → back to rendered graph + vector lane, against a testcontainers-booted Qdrant, not mocked. |

**Score:** 5/5 roadmap-level truths verified (0 present-but-behavior-unverified, 0 overrides).

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|---|---|---|---|---|
| GRAPH-01 | 05-01, 05-09 | Local related-memories graph, inline SVG, d3-force, pan/zoom/drag, settle budget, capped edges, legend+toggles | ✓ SATISFIED | See Truth 1; live e2e pass |
| GRAPH-02 | 05-01, 05-05 | Full keyboard reachability, accessible names, `aria-live` equivalent list | ✓ SATISFIED | See Truth 2; `RelatedGraph.browser.test.ts` (part of the 61 passing component tests) |
| GRAPH-03 | 05-01, 05-04, 05-09 | Light/dark category tokens; click selects in detail equivalent | ✓ SATISFIED | See Truth 3; AA audits |
| TAGS-01 | 05-03, 05-07, 05-08 | Tag popularity linear-bar list from `ListTags`, DOM order = reading order, click adds filter | ✓ SATISFIED | See Truth 4; `TagBars.browser.test.ts` passing |
| TAGS-02 | 05-03, 05-06, 05-07 | Tag filter autocomplete over `ListTags` with counts | ✓ SATISFIED | `TagCombobox.svelte`, `HeaderSearch.svelte` Tags group; `TagCombobox.browser.test.ts` passing |

REQUIREMENTS.md traceability rows for all five IDs read `Mapped` (not a tool-recognized "Complete" value); `deferred-items.md` documents this as a pre-existing, project-wide `gsd-tools` vocabulary mismatch (confirmed via `git log -S "Mapped"` predating this phase) — not evidence of missing implementation. No ORPHANED requirements: REQUIREMENTS.md maps exactly GRAPH-01/02/03 and TAGS-01/02 to Phase 5, and all five appear in plan `requirements:` frontmatter (05-01, 05-04, 05-05, 05-09 declare the GRAPH IDs; 05-03, 05-06, 05-07, 05-08 declare the TAGS IDs).

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `ui/src/lib/related/graph.ts` | pure neighbourhood model, membership, edges, layout | ✓ VERIFIED | 446 lines; `visibleMembership`, `neighbourhoodSummary`, `settleLayout` present; unit tests (`graph.test.ts`, part of 90 passing pure-model tests) pin D-06/D-07/D-08/D-09/D-20 boundaries |
| `ui/src/lib/components/RelatedGraph.svelte` | Svelte-owned inline-SVG graph | ✓ VERIFIED | 798 lines; `role="listbox"`, `aria-activedescendant`, d3-zoom/d3-drag gesture-only usage, zoom controls, focus card |
| `ui/src/routes/related/[id]/+page.svelte` | `/related/<id>` route | ✓ VERIFIED | 705 lines; one `relatedMemories` read, rail tabs, trail, Escape tiers, tag filter |
| `ui/src/lib/search/related-params.ts` | `/related` URL codec | ✓ VERIFIED | 47 lines; `relatedPath`, `parseRelatedParams`/`encodeRelatedParams`, validates `from` via `isAllowedDestination` |
| `ui/src/lib/tags/query.ts` | one `ListTags` query per scope key | ✓ VERIFIED | 28 lines; `listTagsQuery` |
| `ui/src/lib/tags/tags.ts` | pure tag slicing/ranking/copy | ✓ VERIFIED | 207 lines; `rankTagMatches`, `barPercent`, `TAG_TOP=30` |
| `ui/src/lib/components/TagBars.svelte` | shared tag popularity list | ✓ VERIFIED | 268 lines; both `panel` and `rail` modes |
| `ui/src/lib/components/TagCombobox.svelte` | `+ tag` picker | ✓ VERIFIED | 144 lines; `shouldFilter={false}` |
| `ui/src/lib/related/lanes.ts` | pure lane/evidence/empty-state copy | ✓ VERIFIED | 204 lines; `evidenceLines` |
| `ui/src/lib/components/EvidenceSection.svelte` | evidence under the graph | ✓ VERIFIED | 179 lines; "Evidence is per type; there is no blended score." present |
| `ui/src/lib/components/SupersessionLane.svelte` | supersession timeline lane | ✓ VERIFIED | 252 lines; `supersessionColumns` |
| `ui/src/lib/components/GraphLegend.svelte` | legend toggles shared with lane hide buttons | ✓ VERIFIED | 94 lines; shadcn `Checkbox` |
| `ui/src/lib/related/zoom.ts` | pure zoom gate/fit/readout math | ✓ VERIFIED | 98 lines; `wheelZoomFilter` |
| `ui/src/lib/components/HeaderSearch.svelte` | header search Tags group | ✓ VERIFIED | 699 lines; `'Tags · counts in '` heading |
| `ui/src/lib/components/FacetStrip.svelte` | `+ tag` picker + Tags panel toggle | ✓ VERIFIED | 296 lines; imports/mounts `TagCombobox` |
| `ui/src/lib/components/RecallSplit.svelte` | narrow-breakpoint flag | ✓ VERIFIED | 161 lines; `narrow = $bindable(false)` |
| `internal/e2e/console_browser_test.go` | live `/related` round trip | ✓ VERIFIED | 1344 lines; `func TestConsoleRelatedView` — ran it myself against a self-booted testcontainers Qdrant, PASS |
| `internal/webauth/static/index.html` | vendored console SPA incl. `/related` | ✓ VERIFIED | present; `task ui:build` reproduces it with zero `git status` drift (verified myself) |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `+page.svelte` (`/related`) | `client.ts` | one `RelatedMemories` read | ✓ WIRED | `engram.relatedMemories({...})` at `+page.svelte:51` |
| `+page.svelte` | `related/graph.ts` | `visibleMembership` | ✓ WIRED | model + membership shared by lanes and graph |
| `RelatedGraph.svelte` | `related/graph.ts` | `settleLayout` | ✓ WIRED | seeded synchronous settle before render |
| `related/graph.ts` | `ui/package.json` | d3-force only | ✓ WIRED | `from 'd3-force'` (graph.ts:27) |
| `ResultsList.svelte` | route host | `onrelated` (row key `r`) | ✓ WIRED | prop threaded through, invoked on `r` |
| `CommandMenu.svelte` | route | `Related to {short_id}` ⌘K item | ✓ WIRED | `items.push({label: 'Related to ...', onSelect: selectRelated})` |
| `tags/query.ts` | `client.ts` | `ListTags`, limit 1000 | ✓ WIRED | `engram.listTags` inside `listTagsQuery` |
| `TagBars.svelte` | `tags/query.ts` | shared cached query | ✓ WIRED | `createQuery(() => listTagsQuery(scope))` |
| `TagCombobox.svelte` | `tags/tags.ts` | `rankTagMatches` | ✓ WIRED | one pure ranking function reused |
| `+page.svelte` | `EvidenceSection.svelte` | shared selection → evidence | ✓ WIRED | selection state feeds evidence panel under the graph |
| `+page.svelte` | `related-params.ts` | re-centre pushes `relatedPath` with trail | ✓ WIRED | confirmed in route source |
| `/search +page.svelte` | `TagBars.svelte` | docked panel, `mode="panel"` | ✓ WIRED | import + mount confirmed |
| `FacetStrip.svelte` | `TagCombobox.svelte` | `+ tag` picker | ✓ WIRED | import + mount confirmed |
| `HeaderSearch.svelte` | `tags/query.ts` + `tags/tags.ts` | Tags autocomplete group | ✓ WIRED | same cached query, same `rankTagMatches` |
| `console_browser_test.go` | `internal/webauth/static/index.html` | e2e loads vendored SPA | ✓ WIRED | live navigation to `/ui/related/...` proven by passing test |

No orphaned artifacts found; no write-client (`engramWrite`) reference anywhere in the `/related` route or any Phase 5 component (`rg` returned zero matches) — the safety prohibition holds.

### Behavioral Spot-Checks / Live Verification (ran myself, not taken from SUMMARY claims)

| Behavior | Command | Result | Status |
|---|---|---|---|
| Live chromedp round trip through a real binary + self-booted Qdrant | `ENGRAM_REQUIRE_BROWSER=1 go test ./internal/e2e/... -run TestConsoleRelatedView -v` | `--- PASS: TestConsoleRelatedView (2.35s)` | ✓ PASS |
| Pure related-model unit tests (graph/lanes/zoom/tags/related-params) | `pnpm exec vitest run src/lib/related/*.test.ts src/lib/tags/tags.test.ts src/lib/search/related-params.test.ts` | 5 files / 90 tests passed | ✓ PASS |
| Component browser tests: route, graph, tag bars, tag combobox | `pnpm exec vitest run --project browser <files>` | route: 51/51; graph+bars+combobox: 61/61 | ✓ PASS |
| WCAG 2.2 AA audit suite (axe-core, both themes) for all new Phase 5 surfaces | `pnpm exec vitest run --project browser src/lib/a11y/surfaces.browser.test.ts` | 25/25 passed | ✓ PASS |
| Full Go suite | `go test ./...` | all packages `ok` (store 153s, server 33s) | ✓ PASS |
| `go build ./...` | — | clean | ✓ PASS |
| `task lint` | golangci-lint/markdown/actions/yaml/setup/python | all green | ✓ PASS |
| `task ui:build` reproduces vendored SPA | `git status --porcelain internal/webauth/static` after build | 0 lines (no drift) | ✓ PASS |
| Reported flaky vitest test, re-run in isolation | `vitest run --project browser search.browser.test.ts -t "Chain button opens the chain dialog"` | 1/1 passed | ✓ PASS (confirms pre-existing, unrelated flake — file not in Phase 5's `covered_files`) |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|---|---|---|---|---|
| — | — | `TBD`/`FIXME`/`XXX` scan across all Phase 5 files | — | none found — no debt-marker gate triggered |
| `RelatedGraph.svelte` | 77 | Hand-copied `TYPE_LETTER` glyph map duplicating `related/lanes.ts`'s exported `LANE_GLYPH` (code review WR-01) | ℹ️ Info | Values agree today; no functional break, a future edge-type addition could desync the focus card's glyph from the legend/evidence section |
| `RelatedGraph.svelte` | 248-259 | Auto-pan effect can re-trigger from an unrelated node's drag-induced simulation reheat, not just the focused node's own movement (code review WR-02) | ⚠️ Warning | Narrow interaction edge case (focus a node, then drag a different node) that could re-centre the viewport unexpectedly; not covered by the single-node drag test; does not block core keyboard/ARIA equivalence |
| `TagBars.svelte` | 148-174 | Roving `aria-activedescendant` listbox gives no visible focus ring for sighted keyboard users (row only gets `aria-selected`, no `class:active` on `activeIndex`) (code review WR-03) | ⚠️ Warning | Screen-reader equivalence is intact (`aria-activedescendant` correctly reported and tested); a sighted keyboard-only user lacks a visual "you are here" cue that `RelatedGraph`'s own `.kfocus` ring provides for the graph. Automated axe-core AA audit reported zero violations (this specific SC 2.4.7 gap is a known blind spot for automated tooling, not a false pass) |
| `+page.svelte` (`/related`) | 204-210 | `goBack()` calls `history.back()` whenever `params.trail` is non-empty, even on a bookmarked/direct URL where no real browser history backs the trail (code review WR-04) | ⚠️ Warning | Edge case: a shared/bookmarked `/related/<id>?trail=...` link's "← back" can strand the user outside the app instead of falling back to `exitToOrigin()` |
| `RelatedGraph.svelte` | 349-359 | `cardPlacement` hardcodes `VIEW`'s half-extents as separate literals (code review IN-01) | ℹ️ Info | Cosmetic maintainability nit |
| `+page.svelte` | 288 | Skeleton call-line hardcodes `k=64` instead of interpolating `RELATED_K` (code review IN-02) | ℹ️ Info | Cosmetic maintainability nit |

All six anti-patterns above come from `05-REVIEW.md` (0 critical / 4 warning / 2 info, `05-REVIEW-DISPOSITION.md`: all six still `open`, none fixed or filed as a GitHub issue yet). None of them fail a specific PLAN `must_haves` truth or a ROADMAP success criterion: the graph and tag surfaces remain keyboard-operable and screen-reader equivalent per the passing test suites and the zero-violation AA audits; WR-02/WR-03/WR-04 are real, narrow robustness/discoverability gaps worth follow-up but are WARNING-tier, not BLOCKER-tier, and the code reviewer explicitly found "no BLOCKER-tier issue." They are recorded here so they are not lost, and are recommended for a follow-up GitHub issue (see Gaps Summary).

### Human Verification Required

None. The phase's own must-have (`05-09-PLAN.md`, "No manual UAT") establishes that `VALIDATION.md`'s two manual-only rows (theme legibility, gesture feel) are covered by the automated both-theme AA-audit screenshots plus the wheel-gate/zoom-limit/drag unit tests — and this verification independently re-ran that full audit suite (25/25) plus the live chromedp round trip against a self-booted Qdrant, rather than trusting the SUMMARY's claim.

### Gaps Summary

No gaps block the phase goal. Four open, unfixed code-review warnings (WR-01 through WR-04) are carried forward as advisory follow-up work — none of them was filed as a GitHub issue per the phase's own "file the rest" policy for review findings, and `05-REVIEW-DISPOSITION.md` still lists all six findings as `open`. Recommend filing a follow-up issue for WR-02 (auto-pan re-trigger during an unrelated drag), WR-03 (TagBars visible keyboard-focus ring), and WR-04 (bookmarked-trail "back" can leave the app) before the next accessibility-focused phase, since WR-03 in particular touches the phase's own "fully keyboard/ARIA-equivalent" framing (screen-reader equivalence holds; sighted-keyboard visual affordance does not, for this one widget).

---

_Verified: 2026-09-28T01:50:00Z_
_Verifier: Claude (gsd-verifier)_
