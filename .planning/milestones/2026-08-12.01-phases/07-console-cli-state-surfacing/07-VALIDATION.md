---
phase: 7
slug: console-cli-state-surfacing
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-20
---

# Phase 7 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go: stdlib `testing` + `testcontainers-go/modules/qdrant` (`internal/store/store_test.go:22-26`). UI: Vitest 4.1.10 + `vitest-browser-svelte` 2.2.1, Playwright-backed browser project (`ui/package.json:9-33`) |
| **Config file** | Go: none (stdlib). UI: `ui/vitest.config.ts` (browser + node projects) |
| **Quick run command** | Go: `go test ./internal/store/... -run <ScopedTestName>` · UI: `cd ui && npm run test:browser -- <ComponentName>` |
| **Full suite command** | `task` (lint + full Go suite) AND `cd ui && npm run test` |
| **Estimated runtime** | Scoped Go unit (`./cmd/engram/...`) ~5-15s; scoped Go store integration spins a Qdrant testcontainer, ~60-120s; scoped UI browser test ~15-30s. Measure and correct these during Wave 0 — they are estimates, not observations |

---

## Sampling Rate

- **After every task commit:** Run the scoped `-run` command from the Per-Task Verification Map row being implemented
- **After every plan wave:** Run `go test ./internal/store/... ./internal/server/... ./cmd/engram/...` and, if any `ui/` file was touched, `cd ui && npm run test`
- **Before `/gsd-verify-work`:** Full suite must be green — `task` AND `cd ui && npm run test`. Per STATE.md's standing note, `task chart:validate` and `task ui:build` run outside the normal phase-gate lifecycle and must be run locally if either surface is touched (neither is expected in this phase)
- **Max feedback latency:** 120 seconds

---

## Per-Task Verification Map

Re-derived 2026-08-22 by `/gsd-validate-phase` from the seven shipped SUMMARYs. Every test name
below was resolved by `go test -list` and then by execution — none is transcribed from this draft's
guesses, all of which were written before any PLAN.md existed and none of which matched a shipped
name exactly (record `1xe3ze1v9s`, and record `bsbsvn4hbc` on `-run` patterns that match nothing).

| Task | Plan | Requirement | Test Type | Automated Command | Status |
|------|------|-------------|-----------|-------------------|--------|
| 7-01 | 07-01 | REQ-cli-record-state | integration | `go test ./internal/store/ -run TestSupersedeRecallGate` | ✅ green |
| 7-02 | 07-01, 07-02 | REQ-cli-record-state, REQ-console-record-state | integration | `go test ./internal/store/ -run TestArchiveRecallGate` (4 tests: SearchAndList, IncludeArchived, SearchDiscovery, ListScheduled) | ✅ green |
| 7-03 | 07-03 | REQ-cli-record-state, REQ-console-record-state | integration | `go test ./internal/store/ -run 'TestRecallWindowGate\|TestListIncludeSupersededAndScheduled'` | ✅ green |
| 7-04 | 07-03, 07-04 | REQ-console-record-state, REQ-cli-record-state | unit (Go) + unit (Vitest) | `go test ./cmd/engram/ -run TestMemoryState` AND `cd ui && npx vitest run --project=node src/lib/memorystate.test.ts` | ✅ green (Go 2, TS 15) |
| 7-05 | 07-07 | REQ-migration-state-visible | integration | `go test ./internal/server/ -run TestMigrateStatus` | ✅ green |
| 7-06 | 07-05 | REQ-cli-record-state | unit (Go) | `go test ./cmd/engram/ -run 'TestGetHeadline\|TestGetTextOutputRendersHeadlineAndFieldTable'` | ✅ green |
| 7-07 | 07-06 | REQ-cli-record-state, REQ-migration-state-visible (T-07-01, issue #505) | unit (Go) | `go test ./cmd/engram/ -run TestRenderOperatorViewSanitizesHeadline` | ✅ green |
| 7-08 | all | REQ-cli-record-state, REQ-migration-state-visible | unit (existing gate) | `go test ./cmd/engram/ -run TestCatalogBlastRadiusMatchesToolClasses` | ✅ green |
| 7-09 | all | REQ-cli-record-state, REQ-console-record-state | CI gate | `task proto:lint && task proto:gen && git diff --exit-code -- gen/ ui/src/lib/gen/` | ✅ green (CI `buf`) |
| 7-10 | all | REQ-cli-record-state | CI gate | `task surfaces:gen && git diff --exit-code` | ✅ green (CI `surfaces`) |
| 7-RG | 07-01 | REQ-cli-record-state | regression (gates made conditional) | `go test ./internal/store/ -run 'TestSchemaVersionNeverGatesRecall\|TestRecallEmissionSetIsCompleteAndClassified'` | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] Archived-record recall-gate positive-relaxation test — `TestArchiveRecallGateSearchAndList` / `...IncludeArchived` (`internal/store`)
- [x] Scheduled/expired recall-gate test asserting BOTH window bounds relax together — `TestRecallWindowGate`
- [x] Go-side D-13 state-word derivation unit test — `TestMemoryStateWords`, `TestMemoryStateCell` (`cmd/engram`)
- [x] TS-side D-13 derivation test — `ui/src/lib/memorystate.test.ts` (15 tests) plus `MemoryRow.browser.test.ts`
- [x] New-RPC integration test against a real histogram — `TestMigrateStatusHistogram` and siblings (`internal/server`)
- [x] `engram get` identity/adapter test — `TestGetHeadline`, `TestGetTextOutputRendersHeadlineAndFieldTable`
- [x] D-11 headline-sanitization regression test — `TestRenderOperatorViewSanitizesHeadline` (issue #505)
- [x] Negative/default-path assertion: with all three bools false, `Store.Search`/`Store.List` behavior is unchanged — asserted at `internal/store/store_test.go:6905` and `:7096`
- [x] Re-derived (not assumed) `recallEntryPointSeeds` in `schemaversion_recallgate_test.go` — the list is declared once and a classification-coverage linkage subtest asserts it is set-equal to the distinct entry points `recallInvocationRows` actually invokes, so a recall-transmitted function never walked by any row cannot stay invisible. The `backlogFilter` unreachability claim rides the same gate. Both re-verified green after this phase made the gates conditional.

*All nine satisfied. Unlike Phase 6, Phase 7's Wave 0 work genuinely shipped — only the contract
went unreconciled, because every test landed under a different name than this draft guessed.*

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Meta-line wrapping under narrow viewports reads correctly and the state badge is never the element pushed out of view | REQ-console-record-state | UI-SPEC D-13 mandates wrap-never-truncate so the load-bearing badge cannot be overflowed out; the visual outcome at real breakpoints is a judgment call a component test cannot fully assert | Run the console dev server, open a record carrying 3 compound state words in a narrow viewport, confirm the meta line wraps rather than eliding and that no state word is dropped |
| Dimmed rows remain WCAG AA legible at 9.5px and dimming reads as decorative only | REQ-console-record-state | UI-SPEC fixes dim-iff-PAST with the badge at full opacity inside a dimmed row; contrast at the specified size needs visual confirmation against the real theme | Render archived/superseded/expired rows in both light and dark themes; verify badge contrast and that no information is conveyed by dimming alone |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 120s (measured: `cmd/engram` scoped 0.4s, `internal/store` scoped 1.6s with container reuse, UI node project 0.2s — the draft's 60-120s estimate was pessimistic)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-08-22 by `/gsd-validate-phase 07`

---

## Validation Audit 2026-08-22

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

No coverage gaps. All ten map rows and all nine Wave 0 items are covered by shipped, executing
tests: 18 Go subtests (7 `cmd/engram`, 9 `internal/store`, 2 `internal/server`) and 15 TS tests,
all green, **0 failures and 0 skips**. The `internal/store` run was checked for silent skips
specifically — testcontainers connected to Docker and reported no SKIP lines; its 1.6s wall time is
container reuse via `TestMain`, not an unrun suite.

This file was `status: draft` only because it was never reconciled after execution. Every one of
its `name TBD` guesses resolved to a real shipped test under a different name; the map above
records the resolved names.

**Observation, not a gap:** this phase converted three previously unconditional recall gates into
conditional ones, which is the change class where a red-proof earns its keep — but this contract
never asked for red-evidence (unlike Phase 6's), and Phase 7 has no `red-evidence/` directory.
Recorded here rather than silently expanding this audit's scope; closing it would be a deliberate
decision, not a gap-fill.
