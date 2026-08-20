---
phase: 7
slug: console-cli-state-surfacing
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
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

Task IDs are assigned when PLAN.md files are written; this map is seeded at requirement
granularity and task IDs are filled in at `/gsd-validate-phase` time.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| pending | TBD | TBD | REQ-cli-record-state | — | `Search`/`List` with `IncludeSuperseded:true` surfaces a previously-hidden superseded record; default (all bools false) is byte-identical to today's gated behavior | integration | `go test ./internal/store/... -run TestSupersedeRecallGate` | ✅ exists (`store_test.go:3195`) — extend with sub-cases, do not add a file | ⬜ pending |
| pending | TBD | TBD | REQ-cli-record-state, REQ-console-record-state | — | Same relaxation for `IncludeArchived`; an archived record is reachable ONLY with the opt-in flag | integration | `go test ./internal/store/... -run TestArchiveRecallGate` (name TBD) | ❌ W0 | ⬜ pending |
| pending | TBD | TBD | REQ-cli-record-state, REQ-console-record-state | — | `IncludeScheduled` relaxes BOTH halves of `activeWindowConditions` together, so an `expired` record is reachable and the `expired` vocabulary word is derivable | integration | `go test ./internal/store/... -run TestSearchIncludeScheduled` (name TBD) | ❌ W0 | ⬜ pending |
| pending | TBD | TBD | REQ-console-record-state, REQ-cli-record-state | — | D-13 state-word derivation produces the identical vocabulary and canonical order (`archived, superseded, expired, scheduled`) on the Go surface and the TS surface | unit (Go) + component (Vitest browser) | `go test ./cmd/engram/... -run TestStateWord` (name TBD) AND `cd ui && npm run test:browser -- MemoryRow` | ❌ W0 — both new; D-13 explicitly requires a test per surface asserting the derivation | ⬜ pending |
| pending | TBD | TBD | REQ-migration-state-visible | — | New Connect read RPC returns the same histogram `Store.MigrateStatus` already computes, with nil→`[]` normalization per the `statusReportDoc` precedent; readable by any authenticated caller | integration | `go test ./internal/server/... -run TestMigrateStatus` (name TBD) | ❌ W0 | ⬜ pending |
| pending | TBD | TBD | REQ-cli-record-state | — | `engram get` text/json identity holds, mirroring Phase 6 `assertViewIdentity` fixture discipline; protojson bytes render through `renderOperatorView` including correct omission/presence of an `optional` field | unit (Go) | `go test ./cmd/engram/... -run TestGetCommand` (name TBD) | ❌ W0 | ⬜ pending |
| pending | TBD | TBD | REQ-cli-record-state, REQ-migration-state-visible | T-07-01 (issue #505) | The operator-view HEADLINE is sanitized: a state-word parenthetical with no control characters passes through unchanged, and a synthetic control-character input is neutralized — closing the `sanitizeViewValue` bypass this phase creates the exploit condition for | unit (Go) | `go test ./cmd/engram/... -run TestOperatorViewHeadlineSanitized` (name TBD) | ❌ W0 | ⬜ pending |
| pending | TBD | TBD | REQ-cli-record-state, REQ-migration-state-visible | — | Every new CLI command (`engram get`, the new migration verb) is classified in `internal/surfaces/toolclass.go`; `buildCatalog` panics at test time on any unclassified command | unit (Go, existing gate) | `go test ./cmd/engram/... -run TestCatalogBlastRadiusMatchesToolClasses` | ✅ exists — this IS the gate; new rows must be added to pass | ⬜ pending |
| pending | TBD | TBD | REQ-cli-record-state, REQ-console-record-state | — | Proto/codegen drift: `gen/go/` and `ui/src/lib/gen/` are regenerated and committed; `buf lint` and `buf breaking` pass against `main` | CI gate (existing) | `task proto:lint && task proto:gen && git diff --exit-code -- gen/ ui/src/lib/gen/` | ✅ exists — CI job `buf` (`.github/workflows/ci.yaml:244-264`) | ⬜ pending |
| pending | TBD | TBD | REQ-cli-record-state | — | CLI golden drift: `--help`/catalog goldens regenerate cleanly from the live cobra tree after the new verbs land | CI gate (existing) | `task surfaces:gen && git diff --exit-code` | ✅ exists — CI job `surfaces` (`.github/workflows/ci.yaml:273-`) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Archived-record recall-gate positive-relaxation test (`internal/store`) — covers REQ-cli-record-state, REQ-console-record-state
- [ ] Scheduled/expired recall-gate positive-relaxation test asserting BOTH window bounds relax together (`internal/store`) — covers REQ-cli-record-state, REQ-console-record-state; directly gates the `expired` vocabulary word
- [ ] Go-side D-13 state-word derivation unit test (`cmd/engram`) — covers REQ-cli-record-state
- [ ] TS-side Vitest browser D-13 state-word derivation test per touched component — covers REQ-console-record-state
- [ ] New-RPC integration test against a real histogram: multiple version buckets, absent bucket, future bucket (`internal/server`) — covers REQ-migration-state-visible
- [ ] `engram get` identity/adapter test (`cmd/engram`) — covers REQ-cli-record-state
- [ ] D-11 headline-sanitization regression test (`cmd/engram`) — covers the phase's own stated precondition fix (issue #505)
- [ ] Negative/default-path assertion on the two in-scope gate sites: with all three bools false, `Store.Search` and `Store.List` behavior is unchanged
- [ ] Re-run (do NOT assume) the `recallEntryPointSeeds` derivation in `schemaversion_recallgate_test.go` and the `backlogFilter` "never reachable from any recall entry point" claim — both rest on the gates being unconditional, and this phase makes them conditional

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Meta-line wrapping under narrow viewports reads correctly and the state badge is never the element pushed out of view | REQ-console-record-state | UI-SPEC D-13 mandates wrap-never-truncate so the load-bearing badge cannot be overflowed out; the visual outcome at real breakpoints is a judgment call a component test cannot fully assert | Run the console dev server, open a record carrying 3 compound state words in a narrow viewport, confirm the meta line wraps rather than eliding and that no state word is dropped |
| Dimmed rows remain WCAG AA legible at 9.5px and dimming reads as decorative only | REQ-console-record-state | UI-SPEC fixes dim-iff-PAST with the badge at full opacity inside a dimmed row; contrast at the specified size needs visual confirmation against the real theme | Render archived/superseded/expired rows in both light and dark themes; verify badge contrast and that no information is conveyed by dimming alone |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
