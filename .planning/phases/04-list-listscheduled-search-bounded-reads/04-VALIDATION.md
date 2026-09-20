---
phase: "04"
slug: "list-listscheduled-search-bounded-reads"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-19"
---

# Phase 04 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (stdlib); real Qdrant via `internal/store/storetest` (testcontainers, or `ENGRAM_QDRANT_TEST_ADDR`) |
| **Config file** | none — `Taskfile.yaml` `test:*` tasks are the entry points |
| **Quick run command** | `go test -short ./internal/store/... ./internal/server/... ./cmd/engram/...` |
| **Full suite command** | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -count=1` then `task` (lint + whole-repo test) |
| **Estimated runtime** | ~60 s quick · ~4–6 min full (real-Qdrant oversized fixtures) |

---

## Sampling Rate

- **After every task commit:** Run `go test -short ./internal/store/... ./internal/server/... ./cmd/engram/...`
- **After every plan wave:** Run `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -count=1` (must include `TestSchemaVersionNeverGatesRecall` and `TestRecallEmissionSetIsCompleteAndClassified` — the search rows of `recallInvocationRows` must keep their exact RPC counts)
- **Before `/gsd-verify-work`:** Full suite must be green (`task`)
- **Max feedback latency:** 90 seconds (quick run)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 04-W0-list | TBD | 0 | REQ-list-bounded | T-04-V4 | offset/cursor pages re-apply the caller's authz filter; never widened | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -run 'TestStoreListOffsetBounded' -count=1 -v` (name illustrative — the plan's Wave 0 authors it) | ❌ W0 | ⬜ pending |
| 04-W0-sched | TBD | 0 | REQ-list-scheduled-bounded | T-04-V4 | `ListScheduled` keeps `ownerOnlyCondition` + `scheduledStateCondition` on every RPC | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -run 'TestListScheduledBounded' -count=1 -v` | ❌ W0 | ⬜ pending |
| 04-W0-search | TBD | 0 | REQ-search-k-bounded | T-04-V4 | phase-2 fetch is `has_id` AND the identical phase-1 filter; empty id batch issues no Scroll | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -run 'TestSearchTwoPhaseBounded' -count=1 -v` | ❌ W0 | ⬜ pending |
| 04-W0-contract | TBD | 0 | REQ-list-contract-unchanged | — | N/A | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -run 'TestListContractInvariant|TestSchemaVersionNeverGatesRecall|TestRecallEmissionSetIsCompleteAndClassified' -count=1 -v` | ❌ W0 / ✅ existing gate | ⬜ pending |
| 04-W0-reject | TBD | 0 | REQ-list-limit-contract-decided | T-04-V5 | `limit`/`k` > 1000 rejected before any Qdrant/embed call with `field=<limit|k> hint=out_of_range` | unit/integration | `go test ./internal/server/... -run 'TestOutOfRangeRejection' -count=1 -v`; `go test ./cmd/engram/... -run 'TestExitCodeBaseline' -count=1 -v` | ❌ W0 / ✅ existing baseline (extended) | ⬜ pending |
| 04-rename | TBD | — | REQ-list-limit-contract-decided | — | N/A | unit + docs gate | `go test ./internal/server/... -run 'TestHintCodeDocs|TestArgAttribution' -count=1 -v`; `rg -o 'hint=too_large|"too_large"' internal cmd docs-site/src proto CLAUDE.md skill \| wc -l` must print `0` | ✅ existing gate | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Task IDs are placeholders until the planner assigns `{phase}-{plan}-{task}`; the planner replaces this table's rows with the real task ids and exact `-run` names (re-resolve every `-run` against `go test -list` — `bsbsvn4hbc`: prove execution with `-v` RUN/PASS pairs, never a bare package `ok`).*

---

## Wave 0 Requirements

- [ ] `internal/store/list_bounded_test.go` (or equivalent, `package store_test`) — REQ-list-bounded: offset `limit: 0` → 1000, deep offset, cursor pages; both `storetest.SeedOversized` shapes at `storetest.RecvLimit`; RED against pre-fix `Store.List`
- [ ] `internal/store/listscheduled_bounded_test.go` — REQ-list-scheduled-bounded: large explicit limit, both shapes; RED against pre-fix `ListScheduled`
- [ ] `internal/store/search_twophase_test.go` — REQ-search-k-bounded: `Search` `full=true` with `k` well above 2, `SearchDiscovery`, both shapes; the empty-batch-skips-fetch property; score order preserved after re-attach
- [ ] `internal/store/list_contract_test.go` — REQ-list-contract-unchanged: `total` exact, `next_cursor` empty only on `Exhausted`, a `CutByBudget` page at a shrunken `pageByteBudget` (`SetByteBudgets`) returns a non-empty cursor, ordering unchanged, recall gate unchanged
- [ ] `internal/server/outofrange_test.go` (or rows in `argattribution_test.go` / `connectargerror_test.go`) — `out_of_range` on MCP `list_memory`/`list_scheduled`/`search_memory`/`search_discovery` and Connect `ListMemories`/`SearchMemories`/`SearchDiscoveries`; `cmd/engram/exitcode_baseline_test.go` row for exit 2
- [ ] Phase 2 retargets (D-12): `internal/server/responsetoolarge_test.go` and siblings, `internal/store/storetest/storetest_test.go`, and `.planning/phases/02-*/red-evidence/*.patch` moved to a post-fix overflow shape (D-07 single-record-over-limit)
- [ ] Framework install: none — `storetest`/`go test` infrastructure exists from Phases 1–3

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Console `/ui/observe` pager still shows numbered pages of 50 and the root page lists 50 recent memories after the store change | REQ-list-contract-unchanged | The chromedp e2e covers the root route only; page arithmetic is visual | `ENGRAM_REQUIRE_QDRANT=1 ENGRAM_REQUIRE_BROWSER=1 go test ./internal/server/... -run 'TestConsole' -count=1 -v`, then open `/ui/observe` against a scope with >50 records and click to page 2 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
