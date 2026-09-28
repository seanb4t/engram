---
phase: "2"
slug: "recall-first-search"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-26"
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Vitest 5 (projects `node` for `*.test.ts`, `browser` for `*.browser.test.ts` via vitest-browser-svelte + Playwright Chromium); `go test` for server |
| **Config file** | `ui/vite.config.ts` |
| **Quick run command** | `pnpm --dir ui vitest run --project node <file>` / `pnpm --dir ui vitest run --project browser <file>` / `go test ./internal/server/... -run <Test>` |
| **Full suite command** | `pnpm --dir ui test && task test:go` (phase gate adds `task test:e2e`, `task lint`, `task fmt:check`) |
| **Estimated runtime** | ~180 seconds (full suite, excluding e2e) |

---

## Sampling Rate

- **After every task commit:** Run the relevant quick command for the touched file
- **After every plan wave:** Run `pnpm --dir ui test && task test:go`
- **Before `/gsd-verify-work`:** Full suite plus `task test:e2e`, `task lint`, `task fmt:check` must be green
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

Filled in by the planner and executor; task IDs follow `2-{plan}-{task}`.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 2-04-01 | 04 | 2 | ENTRY-01 / ENTRY-02 | — | N/A | unit | `pnpm --dir ui vitest run --project node src/lib/search/classify.test.ts` | ✅ | ✅ green |
| 2-08-01 | 08 | 3 | ENTRY-03 | — | N/A | unit | `pnpm --dir ui vitest run --project node src/lib/search/recall-header.test.ts` | ✅ | ✅ green |
| 2-04-02 | 04 | 2 | ENTRY-05 | — | N/A | unit | `pnpm --dir ui vitest run --project node src/lib/errors/connect-error.test.ts` | ✅ | ✅ green |
| 2-01-02, 2-01-03 | 01 | 1 | D-01..D-03 | T-02-01 | Ungated comparison keeps owner/scope authz; only recall-gate Include* flags lifted | Go unit + parity | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run RecallHidden -count=1` (`hiddencount_test.go`; CLI footer: `go test ./cmd/engram/ -run RecallHiddenFooter -count=1`) | ✅ | ✅ green |
| 2-04-01 | 04 | 2 | ENTRY-04 | — | N/A | browser | `pnpm --dir ui vitest run --project browser src/lib/components/HeaderSearch.browser.test.ts` (SC2) + `CommandMenu.browser.test.ts` | ✅ | ✅ green |
| 2-08-01 | 08 | 3 | ENTRY-06 | — | N/A | unit + browser | `pnpm --dir ui vitest run --project browser src/routes/search/search.browser.test.ts` (SC4 race, keep-previous-data, typing is debounced) + `HeaderSearch.browser.test.ts -t "typing is debounced"` + `params.test.ts` | ✅ | ✅ green |
| 2-06-01, 2-06-03 | 06 | 2 | ROW-01..ROW-03 | — | N/A | browser | `pnpm --dir ui vitest run --project browser src/lib/components/ResultsList.browser.test.ts` | ✅ | ✅ green |
| 2-06-02 | 06 | 2 | ROW-04 | — | N/A | browser | `pnpm --dir ui vitest run --project browser src/lib/components/ResultRow.browser.test.ts` | ✅ | ✅ green |
| 2-04-03, 2-08-03 | 04, 08 | 2, 3 | ROW-05 | — | N/A | unit + browser | `pnpm --dir ui vitest run --project node src/lib/search/params.test.ts` + `FacetStrip.browser.test.ts` + `search.browser.test.ts -t "ROW-05"` | ✅ | ✅ green |
| 2-08-03 | 08 | 3 | ROW-06 | — | N/A | browser | `pnpm --dir ui vitest run --project browser src/lib/components/ScopeCombobox.browser.test.ts` | ✅ | ✅ green |
| 2-07-01 | 07 | 2 | ROW-07 | — | N/A | browser | `pnpm --dir ui vitest run --project browser src/lib/components/DetailPane.browser.test.ts` | ✅ | ✅ green |
| 2-10-02 | 10 | 5 | DSYS-01 / DSYS-02 | — | N/A | doc check | `test -f .claude/skills/engram-console-conventions/SKILL.md && test -f .claude/skills/engram-connect-client/SKILL.md` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `ui/src/lib/search/classify.ts` + `classify.test.ts` — shared id/short_id/text/operator classifier
- [x] `ui/src/lib/search/recall-header.ts` + `recall-header.test.ts` — honest header formatter
- [x] `ui/src/lib/errors/connect-error.ts` + `connect-error.test.ts` — `.rawMessage`/`.code` classifier
- [x] `internal/server/*_hiddencount_test.go` — hidden-count diff/bucket helper and lane wiring
- [x] Spike resolving `role="region"` (virtual list viewport) vs `role="listbox"` before `ResultsList.svelte` is locked
- [x] `.claude/skills/engram-console-conventions/SKILL.md`, `.claude/skills/engram-connect-client/SKILL.md`

---

## Manual-Only Verifications

All phase behaviors have automated verification. (Per Sean's standing preference `x0krpn67b0`, behavior a test can exercise gets an automated test, never a UAT checklist.)

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 60s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-09-26

---

## Validation Audit 2026-09-26

| Metric | Count |
|--------|-------|
| Gaps found | 1 |
| Resolved | 1 |
| Escalated | 0 |

- **Gap (PARTIAL):** ENTRY-06 "typing is debounced" had no test that failed when a debounce was removed. The SC4 race, keep-previous-data and WR-05 teardown tests did not cover coalescing.
- **Resolved by:** `search route — typing is debounced (ENTRY-06)` in `ui/src/routes/search/search.browser.test.ts` (160ms input debounce, single `replaceState` navigation for the final value) and `HeaderSearch — typing is debounced (ENTRY-06)` in `ui/src/lib/components/HeaderSearch.browser.test.ts` (70ms debounce, no query for an intermediate keystroke). Both were confirmed red with their debounce removed, then green once it was restored.
- **Suite evidence:** `pnpm --dir ui test` 47 files / 513 tests green; `go test ./internal/server/... ./cmd/... -count=1` ok.
