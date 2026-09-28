---
phase: 04-curation-surfaces
verified: 2026-09-28T13:35:00Z
status: passed
score: 5/5 must-haves verified
covered_files: [".planning/phases/04-curation-surfaces/04-01-PLAN.md", ".planning/phases/04-curation-surfaces/04-01-SUMMARY.md", ".planning/phases/04-curation-surfaces/04-02-PLAN.md", ".planning/phases/04-curation-surfaces/04-02-SUMMARY.md", ".planning/phases/04-curation-surfaces/04-03-PLAN.md", ".planning/phases/04-curation-surfaces/04-03-SUMMARY.md", ".planning/phases/04-curation-surfaces/04-04-PLAN.md", ".planning/phases/04-curation-surfaces/04-04-SUMMARY.md", ".planning/phases/04-curation-surfaces/04-05-PLAN.md", ".planning/phases/04-curation-surfaces/04-05-SUMMARY.md", ".planning/phases/04-curation-surfaces/04-06-PLAN.md", ".planning/phases/04-curation-surfaces/04-06-SUMMARY.md", ".planning/phases/04-curation-surfaces/04-07-PLAN.md", ".planning/phases/04-curation-surfaces/04-07-SUMMARY.md", ".planning/phases/04-curation-surfaces/04-08-PLAN.md", ".planning/phases/04-curation-surfaces/04-08-SUMMARY.md", ".planning/phases/04-curation-surfaces/04-09-PLAN.md", ".planning/phases/04-curation-surfaces/04-09-SUMMARY.md", ".planning/phases/04-curation-surfaces/04-10-PLAN.md", ".planning/phases/04-curation-surfaces/04-10-SUMMARY.md", ".planning/phases/04-curation-surfaces/04-11-PLAN.md", ".planning/phases/04-curation-surfaces/04-11-SUMMARY.md", ".planning/phases/04-curation-surfaces/04-12-PLAN.md", ".planning/phases/04-curation-surfaces/04-12-SUMMARY.md", "internal/e2e/console_browser_test.go", "internal/webauth/static_test.go", "ui/package.json", "ui/pnpm-lock.yaml", "ui/src/app.css", "ui/src/lib/a11y/axe.browser.test.ts", "ui/src/lib/a11y/axe.ts", "ui/src/lib/a11y/surfaces.browser.test.ts", "ui/src/lib/components/AppShell.browser.test.ts", "ui/src/lib/components/AppShell.svelte", "ui/src/lib/components/ArchiveConfirmDialog.browser.test.ts", "ui/src/lib/components/ArchiveConfirmDialog.svelte", "ui/src/lib/components/ChainDialog.browser.test.ts", "ui/src/lib/components/ChainDialog.svelte", "ui/src/lib/components/CommandMenu.browser.test.ts", "ui/src/lib/components/CommandMenu.svelte", "ui/src/lib/components/CurationSurfaces.browser.test.ts", "ui/src/lib/components/CurationSurfaces.svelte", "ui/src/lib/components/DeleteConfirmDialog.browser.test.ts", "ui/src/lib/components/DeleteConfirmDialog.svelte", "ui/src/lib/components/DetailPane.browser.test.ts", "ui/src/lib/components/DetailPane.svelte", "ui/src/lib/components/FacetStrip.svelte", "ui/src/lib/components/HeaderSearch.browser.test.ts", "ui/src/lib/components/HeaderSearch.svelte", "ui/src/lib/components/ResultHoverCard.svelte", "ui/src/lib/components/ResultRow.browser.test.ts", "ui/src/lib/components/ResultRow.svelte", "ui/src/lib/components/ResultsHeader.browser.test.ts", "ui/src/lib/components/ResultsHeader.svelte", "ui/src/lib/components/ResultsList.browser.test.ts", "ui/src/lib/components/ResultsList.svelte", "ui/src/lib/components/RowActions.svelte", "ui/src/lib/components/ScopeChip.svelte", "ui/src/lib/components/SupersedeDialog.browser.test.ts", "ui/src/lib/components/SupersedeDialog.svelte", "ui/src/lib/components/WriteSurfaces.svelte", "ui/src/lib/components/ui/tabs/tabs-trigger.svelte", "ui/src/lib/curation/chain.test.ts", "ui/src/lib/curation/chain.ts", "ui/src/lib/curation/flash.svelte.ts", "ui/src/lib/curation/host.svelte.ts", "ui/src/lib/curation/supersede-rejection.test.ts", "ui/src/lib/curation/supersede-rejection.ts", "ui/src/lib/mutations/curation.test.ts", "ui/src/lib/mutations/curation.ts", "ui/src/lib/mutations/memory.test.ts", "ui/src/lib/mutations/memory.ts", "ui/src/lib/queries.test.ts", "ui/src/lib/queries.ts", "ui/src/lib/resume.test.ts", "ui/src/lib/resume.ts", "ui/src/lib/search/recall-header.test.ts", "ui/src/lib/search/recall-header.ts", "ui/src/lib/search/rules-params.test.ts", "ui/src/lib/search/rules-params.ts", "ui/src/lib/search/scheduled-params.test.ts", "ui/src/lib/search/scheduled-params.ts", "ui/src/lib/time.test.ts", "ui/src/lib/time.ts", "ui/src/routes/+page.svelte", "ui/src/routes/discovery/+page.svelte", "ui/src/routes/discovery/discovery.browser.test.ts", "ui/src/routes/page.browser.test.ts", "ui/src/routes/rules/+page.svelte", "ui/src/routes/rules/rules.browser.test.ts", "ui/src/routes/scheduled/+page.svelte", "ui/src/routes/scheduled/scheduled.browser.test.ts", "ui/src/routes/search/+page.svelte", "ui/src/routes/search/search.browser.test.ts"]
covered_digest: "v2:sha256:e3887c8eab655fcf16bf0a932f8009bc4a18761fb818d80bdc3601c997d70329"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: passed
  previous_score: 5/5
  gaps_closed: []
  gaps_remaining: []
  regressions: []
---

# Phase 4: Curation Surfaces Verification Report

**Phase Goal:** An operator can supersede, archive/restore, and browse rules/scheduled records
from the console, with every new write surface surviving a re-auth and the console passing an
accessibility and end-to-end check.
**Verified:** 2026-09-28T13:35:00Z
**Status:** passed
**Re-verification:** Yes — previous VERIFICATION.md was stale because later milestone phases 5-6
edited shared console files (`ui/src/routes/search/+page.svelte`, `search.browser.test.ts`, the
a11y surfaces test, the vendored SPA, `internal/e2e/console_browser_test.go`). This run
re-executes the goal-backward checks against current HEAD (branch `feat/2026-09-25.01`) rather
than trusting the prior report's timestamped evidence.

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | User can select records and supersede via preview-before-commit dialog, visible from successor and predecessor panes | ✓ VERIFIED | `SupersedeDialog.svelte` (821 lines, unchanged since prior verification) still wired from `/search`'s `curation?.openSupersede(...)` at 3 call sites (bulk bar, row toolbar, detail pane, ⌘K). Live e2e `TestConsoleSupersedeRoundTrip` re-run against real binary + testcontainer Qdrant + real headless Chrome on current HEAD: **PASS (5.39s)**. |
| 2 | User can archive/restore owned records from row menu, detail pane, or multi-select, with derived-state word updating in place and one-click undo toast | ✓ VERIFIED | `ArchiveConfirmDialog.svelte`, `RowActions.svelte`, `CurationSurfaces.svelte` unchanged (359/154/474 lines, identical to prior verification). Live e2e `TestConsoleArchiveUndoRoundTrip` re-run on current HEAD: **PASS (4.89s)**, confirming `archived_at` set then cleared via the real Connect API through a real dialog-driven undo. |
| 3 | User can open Rules (one-line index, full text on demand, no visibility toggle, delete only) and Scheduled (scheduled/expired/all, archive on expired) views | ✓ VERIFIED | `ui/src/routes/rules/+page.svelte` still calls `engram.listRules({scopes:[],tags:[],full:false})` and wires only `DeleteConfirmDialog` (no edit/visibility/archive/supersede import). `ui/src/routes/scheduled/+page.svelte` still gates `curation?.openArchive` behind `isExpired(m)`. Named vitest re-run: `rules.browser.test.ts -t "a seeded delete envelope reopens the confirm"` — 1/1 pass (flaked once across 5 runs with a DOM timing failure, then passed on 4 immediate re-runs; consistent with this suite's documented browser-test flakiness, not a code regression — see Anti-Patterns). `scheduled.browser.test.ts` matching test — 1/1 pass every run. |
| 4 | Resume round-trip test proves a draft on every new write surface (supersede, archive, rules, scheduled) survives an OIDC re-login | ✓ VERIFIED | `ui/src/lib/resume.ts` (250 lines, byte-identical to prior verification): `RESUME_VERSION=2`, `ALLOWED_DESTINATIONS` still `['/search','/discovery','/rules','/scheduled']`, `SupersedeResumeEnvelope`/`ArchiveResumeEnvelope`/`DeleteResumeEnvelope` all present. Named test re-run: `search.browser.test.ts -t "search route — curation resume reopen"` — 2/2 pass (supersede + archive). Rules/Scheduled resume tests (above) also pass. |
| 5 | WCAG 2.2 keyboard/contrast audit + WIG review pass (or fixed/recorded); chromedp e2e exercises entry-point resolution, supersede, archive/restore round trip against a live server | ✓ VERIFIED | `ui/src/lib/a11y/axe.ts` (33 lines, unchanged) still exports `auditAA`/`AA_TAGS` incl. `wcag22aa`. `surfaces.browser.test.ts` grew from 706→961 lines (phases 5-6 added `/related`, Tags panel, suggested-filter describe blocks) but every Phase-4 describe block is still present and independently re-run green: `"/search curation state"`, `"archive confirm dialog — AA audit"` (3/3 pass), `"supersede dialog — AA audit"` (3/3 pass), `"chain dialog — AA audit"`, `"rule delete confirm — AA audit"`, `"rules route — AA audit"` (1/1 pass), `"scheduled route — AA audit"` (1/1 pass), `"list selection and toolbar — AA audit"`, `"curation dialog keyboard model"`. `04-A11Y-AUDIT.md` (72 lines, unchanged) still records 9 AA findings fixed, 0 deferred. All three chromedp tests (`TestConsoleArchiveUndoRoundTrip`, `TestConsoleSupersedeRoundTrip`, `TestConsoleEntryPointResolution`) re-run live in this session against current HEAD: **all three PASS**. |

**Score:** 5/5 truths verified (0 present-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `ui/src/lib/components/ArchiveConfirmDialog.svelte` | shared archive/restore confirm dialog | ✓ VERIFIED | 359 lines — unchanged since prior verification |
| `ui/src/lib/components/CurationSurfaces.svelte` | route-hosted curation write host | ✓ VERIFIED | 474 lines — unchanged; `openArchive`/`openRestore`/`openSupersede`/`openChain` all present and wired from current `/search` route |
| `ui/src/lib/mutations/curation.ts` | archive/restore/supersede mutation hooks | ✓ VERIFIED | present, imported by `CurationSurfaces.svelte` |
| `ui/src/lib/curation/flash.svelte.ts` | 1600ms row flash set | ✓ VERIFIED | present, unchanged |
| `ui/src/lib/resume.ts` | v2 resume envelope union + destination allowlist | ✓ VERIFIED | 250 lines — unchanged; `SupersedeResumeEnvelope`, `ArchiveResumeEnvelope`, `DeleteResumeEnvelope` all present |
| `ui/src/lib/a11y/axe.ts` | WCAG 2.2 AA audit helper | ✓ VERIFIED | 33 lines — unchanged; exports `auditAA`, `AA_TAGS` includes `wcag22aa` |
| `ui/src/lib/curation/chain.ts` | pure supersession chain model | ✓ VERIFIED | present, exports `buildChain` |
| `ui/src/lib/components/ChainDialog.svelte` | Chain dialog | ✓ VERIFIED | present and wired from `SupersedeDialog`/`DetailPane`/row actions |
| `ui/src/lib/components/ResultsList.svelte` | multi-select listbox + curation keys | ✓ VERIFIED | `aria-multiselectable` still present |
| `ui/src/lib/components/SupersedeDialog.svelte` | preview-before-commit supersede dialog | ✓ VERIFIED | 821 lines — unchanged |
| `ui/src/lib/components/RowActions.svelte` | hover/focus row action toolbar | ✓ VERIFIED | `role="toolbar"` still present |
| `ui/src/routes/rules/+page.svelte` | Rules view | ✓ VERIFIED | 296 lines; calls `listRules`, only `DeleteConfirmDialog` wired |
| `ui/src/routes/scheduled/+page.svelte` | Scheduled view | ✓ VERIFIED | 327 lines; calls `listScheduled`, archive gated on `isExpired` |
| `ui/src/lib/a11y/surfaces.browser.test.ts` | per-surface WCAG 2.2 AA regression gate | ✓ VERIFIED | grew to 961 lines (phase 5/6 additions); all Phase-4 describe blocks intact and passing |
| `.planning/phases/04-curation-surfaces/04-A11Y-AUDIT.md` | DSYS-03 audit record | ✓ VERIFIED | 72 lines — unchanged |
| `internal/e2e/console_browser_test.go` | chromedp curation round trips | ✓ VERIFIED | grew to 1591 lines (phase 5/6 added more e2e tests); `TestConsoleArchiveUndoRoundTrip`, `TestConsoleSupersedeRoundTrip`, `TestConsoleEntryPointResolution` all still present and, re-run live against current HEAD, all PASS |
| `internal/webauth/static` (vendored SPA) | matches current `ui/` source | ✓ VERIFIED | `task ui:build` re-run in this session; `git status --porcelain internal/webauth/static` reports no diff — no drift |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `ui/src/routes/search/+page.svelte` | `CurationSurfaces.openSupersede/openArchive/openChain` | route wiring | ✓ WIRED | Re-confirmed via grep on current HEAD at lines 593-745: bulk bar, row toolbar, pane, ⌘K all still route through the host |
| `ui/src/routes/rules/+page.svelte` | `engram.listRules` | read client | ✓ WIRED | `engram.listRules({scopes:[],tags:[],full:false})` at line 38 |
| `ui/src/routes/rules/+page.svelte` | `DeleteConfirmDialog.svelte` | only write on /rules | ✓ WIRED | line 256; no edit/archive/supersede import present |
| `ui/src/routes/scheduled/+page.svelte` | `CurationSurfaces.openArchive` | expired-only archive | ✓ WIRED | line 139, gated by `isExpired` (line 125) |
| `ui/src/lib/a11y/surfaces.browser.test.ts` | `ui/src/lib/a11y/axe.ts` (`auditAA`) | per-surface AA gate | ✓ WIRED | used across every Phase-4 and Phase 5/6 describe block |
| `internal/webauth/static_test.go` | `/search` route | test fixture retarget | ✓ WIRED | `httptest.NewRequest(http.MethodGet, "/search", nil)` at line 24 — unchanged |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|---------------------|--------|
| `/rules` route | rule rows | `engram.listRules(...)` Connect RPC | Yes | ✓ FLOWING |
| `/scheduled` route | scheduled/expired rows | `engram.listScheduled(...)` Connect RPC, cursor paging | Yes | ✓ FLOWING |
| `SupersedeDialog` preview | per-target issues, chain preview | `SupersedeMemory{validateOnly:true}` via `engramWrite` | Yes — live e2e confirms server-driven commit changes `superseded_by` on current HEAD | ✓ FLOWING |
| `ArchiveConfirmDialog` | archived/restored state | `ArchiveMemory`/`RestoreMemory` via `engramWrite` | Yes — live e2e confirms `archived_at` set/cleared via Connect API on current HEAD | ✓ FLOWING |

### Behavioral Spot-Checks / Probe Execution

Re-executed independently in this verification session against current HEAD (branch
`feat/2026-09-25.01`), not trusted from the prior VERIFICATION.md's cited output:

| Test | Command | Result | Status |
|------|---------|--------|--------|
| `TestConsoleArchiveUndoRoundTrip` | `ENGRAM_REQUIRE_BROWSER=1 ENGRAM_REQUIRE_QDRANT=1 go test ./internal/e2e/... -run TestConsoleArchiveUndoRoundTrip -v` | `--- PASS: TestConsoleArchiveUndoRoundTrip (4.89s)` | ✓ PASS |
| `TestConsoleSupersedeRoundTrip` | same, `-run TestConsoleSupersedeRoundTrip` | `--- PASS: TestConsoleSupersedeRoundTrip (5.39s)` | ✓ PASS |
| `TestConsoleEntryPointResolution` | same, `-run TestConsoleEntryPointResolution` | `--- PASS: TestConsoleEntryPointResolution (5.59s)` | ✓ PASS |
| `go build ./...` | sanity build | exit 0, no output | ✓ PASS |
| `task ui:build` then `git status --porcelain internal/webauth/static` | ui-drift gate (vendored SPA matches fresh build on current HEAD) | no diff | ✓ PASS |
| `pnpm vitest run -t "search route — curation resume reopen"` (`search.browser.test.ts`) | targeted resume round-trip | 2 passed \| 67 skipped | ✓ PASS |
| `pnpm vitest run -t "a seeded delete envelope reopens the confirm"` (`rules.browser.test.ts`) | targeted resume round-trip | 1 passed \| 16 skipped (flaked once in 5 runs, see Anti-Patterns) | ✓ PASS |
| `pnpm vitest run -t "a seeded archive envelope reopens the confirm"` (`scheduled.browser.test.ts`) | targeted resume round-trip | 1 passed \| 20 skipped (5/5 runs) | ✓ PASS |
| `pnpm vitest run -t "rules route — AA audit"` / `"scheduled route — AA audit"` / `"archive confirm dialog — AA audit"` / `"supersede dialog — AA audit"` (`surfaces.browser.test.ts`) | targeted AA regression gates for Phase-4 surfaces | all pass (1/1, 1/1, 3/3, 3/3) | ✓ PASS |
| `pnpm vitest run -t "chain dialog entry points"` (`search.browser.test.ts`) | known pre-existing load flake, isolated | 2 passed \| 67 skipped | ✓ PASS (confirms it's a load-flake, not a regression) |
| No debt markers (`TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER`) in the 13 core Phase-4 impl files | `rg` scan | 0 hits | ✓ PASS |

Full `pnpm --dir ui test` and `task` (Go lint+test) were not re-run in full in this session — per
the "run full suite at most once" constraint, and because other concurrently-running verifiers
(phases 1, 01.1, 2, 3, 5) are exercising overlapping shared files. The targeted, independently
re-run checks above (three live chromedp e2e tests against a real binary/Qdrant/Chrome, the
ui-drift gate, and eight named vitest checks covering every Phase-4 truth) constitute this
session's own regression evidence.

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|-----------------|--------------|--------|----------|
| CUR-01 | 04-04, 04-06, 04-08, 04-12 | Supersede with preview-before-commit dialog, forward/backward links | ✓ SATISFIED | ChainDialog, SupersedeDialog, DetailPane links, live e2e round trip re-confirmed on current HEAD |
| CUR-02 | 04-01, 04-05, 04-07, 04-12 | Archive/restore from row menu, detail pane, multi-select, one-click undo | ✓ SATISFIED | ArchiveConfirmDialog, RowActions, multi-select, live e2e round trip re-confirmed on current HEAD |
| CUR-03 | 04-07, 04-09 | Rules view: one-line index, full text on demand, no visibility toggle, delete only | ✓ SATISFIED | `/rules` route, `rules-params.ts`, delete-only wiring re-confirmed |
| CUR-04 | 04-07, 04-10 | Scheduled view: scheduled/expired/all, archive on expired | ✓ SATISFIED | `/scheduled` route, tabs, `isExpired` gate re-confirmed |
| CUR-05 | 04-02, 04-06, 04-08, 04-09, 04-10 | Every new write surface in the re-auth resume envelope | ✓ SATISFIED | v2 resume envelope unchanged; 4 dedicated resume round-trip test suites re-run passing |
| DSYS-03 | 04-03, 04-11 | WCAG 2.2 keyboard/contrast + WIG review, findings fixed/recorded | ✓ SATISFIED | `auditAA`, `surfaces.browser.test.ts` Phase-4 blocks re-run passing, `04-A11Y-AUDIT.md` unchanged (0 AA findings deferred) |
| DSYS-04 | 04-12 | vitest-browser screenshot coverage + chromedp e2e round trips | ✓ SATISFIED | 3 chromedp tests re-confirmed PASS live against current HEAD |

No orphaned requirements — REQUIREMENTS.md maps CUR-01–05, DSYS-03, DSYS-04 to Phase 4, and all
seven appear across the 12 plans' `requirements:` frontmatter (re-confirmed by grep on current
HEAD).

### Anti-Patterns Found

No debt markers (`TBD`/`FIXME`/`XXX`), no `TODO`/`HACK`/`PLACEHOLDER` in the 13 core Phase-4
implementation files re-scanned on current HEAD.

One transient test flake observed: `pnpm vitest run -t "a seeded delete envelope reopens the
confirm" src/routes/rules/rules.browser.test.ts` failed once (a DOM query timing issue inside the
browser-test runner) out of 5 consecutive runs, passing the other 4 immediately. This matches the
project's documented pattern of vitest-browser load flakiness (the user-flagged
`search.browser.test.ts > chain dialog entry points (D-06)` flake, independently re-confirmed here
as passing in isolation) rather than a code regression — no change to `rules/+page.svelte` or
`resume.ts` accompanied it, and the same test passes reliably when run alone. Not treated as a
gap; noted for awareness.

Code review disposition (`04-REVIEW.md`/`04-REVIEW-DISPOSITION.md`, unchanged since prior
verification) still records 0 critical, 6 warning, 2 info findings, none blocking any of the 5
roadmap success criteria.

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `ui/src/routes/rules/rules.browser.test.ts` | n/a (runtime) | transient vitest-browser timing flake, 1/5 runs | ℹ️ Info | Pre-existing suite flakiness pattern, not a Phase-4 code regression — confirmed by 4 clean re-runs |
| (multiple, see 04-REVIEW-DISPOSITION.md) | — | WR-01..WR-06, IN-01, IN-02 | ⚠️ Warning / ℹ️ Info | Advisory robustness gaps, unchanged since prior verification, not goal-blocking |

### Human Verification Required

None. All five roadmap success criteria have direct code evidence, re-confirmed unchanged/intact
on current HEAD, plus live, independently re-run behavioral proof (three chromedp e2e tests
executed against a real binary/testcontainer-Qdrant/real headless Chrome in this verification
session).

### Gaps Summary

No gaps found, and no regressions from phases 5-6's edits to shared console files. The files
phases 5-6 touched (`ui/src/routes/search/+page.svelte`, `search.browser.test.ts`,
`ui/src/lib/a11y/surfaces.browser.test.ts`, the vendored SPA, `internal/e2e/console_browser_test.go`)
were additive from Phase 4's perspective — new describe blocks and new e2e tests were appended
alongside the existing Phase-4 ones, which remain present, wired, and green. All three of the
phase's hardest live claims (archive/restore round trip, supersede round trip, entry-point
resolution against a real server) were re-executed in this session on current HEAD and all three
passed. The `task ui:build` drift gate was also re-run and produced no diff, confirming the
vendored SPA still matches the current `ui/` source after phase 5/6 additions.

The known pre-existing browser-test load flake (`search.browser.test.ts > chain dialog entry
points (D-06)`) was re-confirmed to pass when run in isolation, consistent with it being a
full-suite load artifact rather than a regression.

An untracked `.planning/phases/04-curation-surfaces/04-EDGE-COVERAGE.probe.json` file remains from
an earlier session (per the task instructions, ignored — not a phase deliverable, not referenced
by any PLAN/SUMMARY).

---

_Verified: 2026-09-28T13:35:00Z_
_Verifier: Claude (gsd-verifier)_
