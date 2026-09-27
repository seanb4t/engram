---
phase: "04"
slug: "curation-surfaces"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-27"
---

# Phase 4 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | vitest 5.0.1 + vitest-browser-svelte 3.1.0 (Chromium via Playwright); Go `internal/e2e` chromedp harness |
| **Config file** | `ui/vitest.config.ts` (node + browser projects) |
| **Quick run command** | `cd ui && pnpm test:browser -- <file>` / `go test ./internal/e2e/ -run <Test> -v` |
| **Full suite command** | `cd ui && pnpm test` + `go test ./internal/e2e/ -count=1 -v` |
| **Estimated runtime** | ~120 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd ui && pnpm test:browser -- <touched component>` (or the single chromedp test for e2e additions)
- **After every plan wave:** Run `cd ui && pnpm test` and `go test ./internal/e2e/ -count=1 -v`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 04-01-01 | 01 | 1 | CUR-02 | T-04-01 | archive only via engramWrite (CSRF) and only after the confirm | browser | `pnpm --dir ui vitest run --project browser src/routes/search/search.browser.test.ts src/lib/components/DetailPane.browser.test.ts` | ✅ | ⬜ pending |
| 04-01-02 | 01 | 1 | CUR-02 | T-04-02 | not-owned presentation decided (checkpoint:decision) | checkpoint | — (decision recorded in 04-01-SUMMARY.md) | n/a | ⬜ pending |
| 04-01-03 | 01 | 1 | CUR-02 | T-04-02, T-04-03 | not_found renders requested input only, never the response id | node + browser | `pnpm --dir ui vitest run --project node src/lib/mutations/curation.test.ts` · `pnpm --dir ui vitest run --project browser src/lib/components/ArchiveConfirmDialog.browser.test.ts src/lib/components/CurationSurfaces.browser.test.ts src/lib/components/DetailPane.browser.test.ts src/routes/search/search.browser.test.ts` | ❌ W0 (created in task) | ⬜ pending |
| 04-02-01 | 02 | 1 | CUR-05 | T-04-04, T-04-05 | allowlisted destinations only; per-kind envelope validation | node + browser | `pnpm --dir ui vitest run --project node src/lib/resume.test.ts` · `pnpm --dir ui vitest run --project browser src/routes/page.browser.test.ts src/lib/components/WriteSurfaces.browser.test.ts` | ✅ | ⬜ pending |
| 04-02-02 | 02 | 1 | CUR-05 | T-04-06 | deleted route unreachable; stale envelope discarded | node + browser + go | `pnpm --dir ui vitest run --project node src/lib/queries.test.ts` · `pnpm --dir ui vitest run --project browser src/routes/page.browser.test.ts src/routes/discovery/discovery.browser.test.ts` · `go test ./internal/webauth/ -count=1 && go vet ./internal/e2e/` | ✅ | ⬜ pending |
| 04-02-03 | 02 | 1 | CUR-05 | — | N/A (navigation) | browser | `pnpm --dir ui vitest run --project browser src/lib/components/AppShell.browser.test.ts src/lib/components/CommandMenu.browser.test.ts src/lib/components/HeaderSearch.browser.test.ts` | ✅ | ⬜ pending |
| 04-03-01 | 03 | 1 | DSYS-03 | T-04-08 | axe-core legitimacy verified before install (blocking-human) | checkpoint | — (verdict recorded in 04-03-SUMMARY.md) | n/a | ⬜ pending |
| 04-03-02 | 03 | 1 | DSYS-03 | T-04-08 | audit helper proven red on a seeded violation; test-only import | browser | `pnpm --dir ui vitest run --project browser src/lib/a11y/axe.browser.test.ts` · `pnpm --dir ui install --frozen-lockfile && task fmt:check` | ❌ W0 (created in task) | ⬜ pending |
| 04-03-03 | 03 | 1 | DSYS-03 | T-04-07 | only skills that passed fable-security-review installed | shell | verdict-line + installed-path loop over `.planning/notes/console-overhaul-exploration.md` | ✅ | ⬜ pending |
| 04-04-01 | 04 | 1 | CUR-01 | T-04-10 | text-only rendering of summaries | node + browser | `pnpm --dir ui vitest run --project node src/lib/curation/chain.test.ts` · `pnpm --dir ui vitest run --project browser src/lib/components/ChainDialog.browser.test.ts` | ❌ W0 (created in task) | ⬜ pending |
| 04-04-02 | 04 | 1 | CUR-01 | T-04-09 | placeholder shows only already-received ids | node + browser | same two commands as 04-04-01 | ✅ | ⬜ pending |
| 04-05-01 | 05 | 2 | CUR-02 | T-04-11 | bulk archive only through the confirm | browser | `pnpm --dir ui vitest run --project browser src/routes/search/search.browser.test.ts src/lib/components/ResultsList.browser.test.ts` | ✅ | ⬜ pending |
| 04-05-02 | 05 | 2 | CUR-02 | — | N/A (selection UI) | browser | `pnpm --dir ui vitest run --project browser src/lib/components/ResultsList.browser.test.ts src/lib/components/ResultRow.browser.test.ts` | ✅ | ⬜ pending |
| 04-05-03 | 05 | 2 | CUR-02 | T-04-11, T-04-12 | selection clears on query/facet change | node + browser | `pnpm --dir ui vitest run --project node src/lib/mutations/memory.test.ts src/lib/mutations/curation.test.ts` · `pnpm --dir ui vitest run --project browser src/lib/components/ResultsHeader.browser.test.ts src/lib/components/ResultsList.browser.test.ts src/lib/components/ResultRow.browser.test.ts src/routes/search/search.browser.test.ts` | ✅ | ⬜ pending |
| 04-06-01 | 06 | 2 | CUR-01 | T-04-13, T-04-14 | preview and commit via engramWrite; commit only on click | node + browser | `pnpm --dir ui vitest run --project node src/lib/mutations/curation.test.ts` · `pnpm --dir ui vitest run --project browser src/lib/components/CurationSurfaces.browser.test.ts` | ✅ | ⬜ pending |
| 04-06-02 | 06 | 2 | CUR-01 | T-04-14 | primary enabled only on a validated current revision | node + browser | `pnpm --dir ui vitest run --project node src/lib/curation/supersede-rejection.test.ts` · `pnpm --dir ui vitest run --project browser src/lib/components/SupersedeDialog.browser.test.ts` | ❌ W0 (created in task) | ⬜ pending |
| 04-06-03 | 06 | 2 | CUR-05 | T-04-15, T-04-16 | resume reopens waiting; same idempotency key | browser | `pnpm --dir ui vitest run --project browser src/lib/components/CurationSurfaces.browser.test.ts src/lib/components/DetailPane.browser.test.ts src/lib/components/ArchiveConfirmDialog.browser.test.ts src/lib/components/SupersedeDialog.browser.test.ts` | ✅ | ⬜ pending |
| 04-07-01 | 07 | 3 | CUR-02 | T-04-17 | toolbar outside options; acts on its anchored row | browser | `pnpm --dir ui vitest run --project browser src/lib/components/ResultsList.browser.test.ts src/routes/search/search.browser.test.ts` | ✅ | ⬜ pending |
| 04-07-02 | 07 | 3 | CUR-03, CUR-04 | — | N/A (list capability) | browser | `pnpm --dir ui vitest run --project browser src/lib/components/ResultsList.browser.test.ts src/lib/components/ResultRow.browser.test.ts` | ✅ | ⬜ pending |
| 04-07-03 | 07 | 3 | CUR-03, CUR-04 | — | coverage rendered verbatim | node | `pnpm --dir ui vitest run --project node src/lib/search/recall-header.test.ts` | ✅ | ⬜ pending |
| 04-08-01 | 08 | 4 | CUR-01 | T-04-18 | commit only on click | browser | `pnpm --dir ui vitest run --project browser src/routes/search/search.browser.test.ts` | ✅ | ⬜ pending |
| 04-08-02 | 08 | 4 | CUR-05 | T-04-18 | route-owned resume; no automatic resend | browser | `pnpm --dir ui vitest run --project browser src/lib/components/CurationSurfaces.browser.test.ts src/routes/search/search.browser.test.ts` | ✅ | ⬜ pending |
| 04-08-03 | 08 | 4 | CUR-01 | T-04-19 | ⌘K actions only for a registered host | browser | `pnpm --dir ui vitest run --project browser src/lib/components/CommandMenu.browser.test.ts src/routes/search/search.browser.test.ts` | ✅ | ⬜ pending |
| 04-09-01 | 09 | 4 | CUR-03 | — | N/A (read) | browser | `pnpm --dir ui vitest run --project browser src/routes/rules/rules.browser.test.ts` | ❌ W0 (created in task) | ⬜ pending |
| 04-09-02 | 09 | 4 | CUR-03 | — | coverage and advisory rendered verbatim | node + browser | `pnpm --dir ui vitest run --project node src/lib/search/rules-params.test.ts` · `pnpm --dir ui vitest run --project browser src/routes/rules/rules.browser.test.ts` | ❌ W0 (created in task) | ⬜ pending |
| 04-09-03 | 09 | 4 | CUR-03, CUR-05 | T-04-20, T-04-21, T-04-22 | delete-only; delete resume waits for the click | browser | `pnpm --dir ui vitest run --project browser src/lib/components/DeleteConfirmDialog.browser.test.ts src/routes/rules/rules.browser.test.ts` | ✅ | ⬜ pending |
| 04-10-01 | 10 | 4 | CUR-04 | T-04-23 | server returns only the caller's records | browser | `pnpm --dir ui vitest run --project browser src/routes/scheduled/scheduled.browser.test.ts` | ❌ W0 (created in task) | ⬜ pending |
| 04-10-02 | 10 | 4 | CUR-04 | — | N/A (read) | node + browser | `pnpm --dir ui vitest run --project node src/lib/time.test.ts src/lib/search/scheduled-params.test.ts` · `pnpm --dir ui vitest run --project browser src/routes/scheduled/scheduled.browser.test.ts` | ❌ W0 (params test created in task) | ⬜ pending |
| 04-10-03 | 10 | 4 | CUR-04, CUR-05 | T-04-24, T-04-25 | archive on expired only; resume waits | browser | `pnpm --dir ui vitest run --project browser src/routes/scheduled/scheduled.browser.test.ts` | ✅ | ⬜ pending |
| 04-11-01 | 11 | 5 | DSYS-03 | — | AA contrast on dimmed rows | browser | `pnpm --dir ui vitest run --project browser src/lib/a11y/surfaces.browser.test.ts src/lib/components/ResultRow.browser.test.ts` | ❌ W0 (created in task) | ⬜ pending |
| 04-11-02 | 11 | 5 | DSYS-03 | — | AA + keyboard across every new surface | browser | `pnpm --dir ui vitest run --project browser src/lib/a11y/surfaces.browser.test.ts …` (six files) | ✅ | ⬜ pending |
| 04-11-03 | 11 | 5 | DSYS-03 | T-04-26, T-04-27 | issues carry no record content | shell | skills cited-path loop + docs-ok check | ✅ | ⬜ pending |
| 04-12-01 | 12 | 6 | DSYS-04, CUR-02 | T-04-28 | vendored bundle equals a fresh build | e2e (chromedp) | `ENGRAM_REQUIRE_BROWSER=1 ENGRAM_REQUIRE_QDRANT=1 go test ./internal/e2e/ -run '^(TestConsoleArchiveUndoRoundTrip\|TestConsoleBundleRendersRecordInBrowser)$' -count=1 -v` · `task ui:build && git status --porcelain -- internal/webauth/static` | ✅ | ⬜ pending |
| 04-12-02 | 12 | 6 | DSYS-04, CUR-01 | T-04-29 | test-only session minting | e2e (chromedp) | `ENGRAM_REQUIRE_BROWSER=1 ENGRAM_REQUIRE_QDRANT=1 go test ./internal/e2e/ -run '^(TestConsoleSupersedeRoundTrip\|TestConsoleEntryPointResolution)$' -count=1 -v` | ✅ | ⬜ pending |
| 04-12-03 | 12 | 6 | DSYS-04 | T-04-28 | all gates green in one run | full | `pnpm --dir ui test` · `ENGRAM_REQUIRE_QDRANT=1 go test ./... -count=1` · `ENGRAM_REQUIRE_BROWSER=1 ENGRAM_REQUIRE_QDRANT=1 go test ./internal/e2e/ -run '^TestConsole' -count=1 -v` · `task lint && task fmt:check && task license:check && task proto:lint` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure (vitest 5 node + browser projects, the chromedp harness in `internal/e2e`) covers the phase; no framework install is needed. New test files are written by the task that needs them, RED first (`tdd="true"` tasks):

- [ ] `ui/src/lib/mutations/curation.test.ts` — archive/supersede cache patch (04-01, 04-06)
- [ ] `ui/src/lib/components/ArchiveConfirmDialog.browser.test.ts`, `CurationSurfaces.browser.test.ts` — CUR-02 (04-01)
- [ ] `ui/src/lib/a11y/axe.browser.test.ts` — DSYS-03 negative/positive controls (04-03)
- [ ] `ui/src/lib/curation/chain.test.ts`, `ui/src/lib/components/ChainDialog.browser.test.ts` — CUR-01 chain (04-04)
- [ ] `ui/src/lib/curation/supersede-rejection.test.ts`, `ui/src/lib/components/SupersedeDialog.browser.test.ts` — CUR-01 (04-06)
- [ ] `ui/src/routes/rules/rules.browser.test.ts`, `ui/src/lib/search/rules-params.test.ts` — CUR-03 (04-09)
- [ ] `ui/src/routes/scheduled/scheduled.browser.test.ts`, `ui/src/lib/search/scheduled-params.test.ts` — CUR-04 (04-10)
- [ ] `ui/src/lib/a11y/surfaces.browser.test.ts` — DSYS-03 per-surface AA gate (04-11)
- [ ] new `TestConsole*` functions in `internal/e2e/console_browser_test.go` — DSYS-04 (04-12)
- [ ] `axe-core` devDependency — installed only after 04-03's blocking-human legitimacy checkpoint

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Not-owned presentation choice (D-08 gap: the SPA has no caller-identity signal) | CUR-02, CUR-01 | a human decision, not a verification | 04-01 Task 2 `checkpoint:decision`; the choice is recorded in 04-01-SUMMARY.md and implemented by 04-01 Task 3 and 04-06 Task 2 |
| axe-core package legitimacy | DSYS-03 | supply-chain trust gate (never auto-approved) | 04-03 Task 1 `checkpoint:human-verify` gate=blocking-human |

All phase behaviors have automated verification; the two rows above are gates on decisions and installs, not on behaviour.

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s (per-task commands; the chromedp e2e runs standalone)
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** {pending / approved YYYY-MM-DD}
