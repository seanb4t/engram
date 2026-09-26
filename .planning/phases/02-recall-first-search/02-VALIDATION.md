---
phase: "2"
slug: "recall-first-search"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
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
| 2-04-01 | 04 | 2 | ENTRY-01 / ENTRY-02 | — | N/A | unit | `pnpm --dir ui vitest run --project node src/lib/search/classify.test.ts` | ❌ W0 | ⬜ pending |
| 2-08-01 | 08 | 3 | ENTRY-03 | — | N/A | unit | `pnpm --dir ui vitest run --project node src/lib/search/recall-header.test.ts` | ❌ W0 | ⬜ pending |
| 2-04-02 | 04 | 2 | ENTRY-05 | — | N/A | unit | `pnpm --dir ui vitest run --project node src/lib/errors/connect-error.test.ts` | ❌ W0 | ⬜ pending |
| 2-01-02, 2-01-03 | 01 | 1 | D-01..D-03 | T-02-01 | Ungated comparison keeps owner/scope authz; only recall-gate Include* flags lifted | Go unit + parity | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run RecallHidden -count=1` | ❌ W0 | ⬜ pending |
| 2-04-01 | 04 | 2 | ENTRY-04 | — | N/A | browser | `pnpm --dir ui vitest run --project browser src/lib/components/HeaderSearch.browser.test.ts` | ❌ W0 | ⬜ pending |
| 2-08-01 | 08 | 3 | ENTRY-06 | — | N/A | unit + browser | stale-response race test (two overlapping queries → final state = later query) | ❌ W0 | ⬜ pending |
| 2-06-01, 2-06-03 | 06 | 2 | ROW-01..ROW-03 | — | N/A | browser | `pnpm --dir ui vitest run --project browser src/lib/components/ResultsList.browser.test.ts` | ❌ W0 | ⬜ pending |
| 2-06-02 | 06 | 2 | ROW-04 | — | N/A | browser | `pnpm --dir ui vitest run --project browser src/lib/components/ResultRow.browser.test.ts` | ❌ W0 | ⬜ pending |
| 2-04-03, 2-08-03 | 04, 08 | 2, 3 | ROW-05 | — | N/A | unit | URL parse/encode round-trip tests for the new facet fields | ❌ W0 | ⬜ pending |
| 2-08-03 | 08 | 3 | ROW-06 | — | N/A | browser | `ScopeCombobox.browser.test.ts` | ❌ W0 | ⬜ pending |
| 2-07-01 | 07 | 2 | ROW-07 | — | N/A | browser | `DetailPane.browser.test.ts` | Partial (`MemoryDetail.browser.test.ts`) | ⬜ pending |
| 2-10-02 | 10 | 5 | DSYS-01 / DSYS-02 | — | N/A | doc check | `test -f .claude/skills/engram-console-conventions/SKILL.md && test -f .claude/skills/engram-connect-client/SKILL.md` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `ui/src/lib/search/classify.ts` + `classify.test.ts` — shared id/short_id/text/operator classifier
- [ ] `ui/src/lib/search/recall-header.ts` + `recall-header.test.ts` — honest header formatter
- [ ] `ui/src/lib/errors/connect-error.ts` + `connect-error.test.ts` — `.rawMessage`/`.code` classifier
- [ ] `internal/server/*_hiddencount_test.go` — hidden-count diff/bucket helper and lane wiring
- [ ] Spike resolving `role="region"` (virtual list viewport) vs `role="listbox"` before `ResultsList.svelte` is locked
- [ ] `.claude/skills/engram-console-conventions/SKILL.md`, `.claude/skills/engram-connect-client/SKILL.md`

---

## Manual-Only Verifications

All phase behaviors have automated verification. (Per Sean's standing preference `x0krpn67b0`, behavior a test can exercise gets an automated test, never a UAT checklist.)

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
