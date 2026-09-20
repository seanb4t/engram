---
phase: "04"
slug: "list-listscheduled-search-bounded-reads"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-19"
validated: "2026-09-20"
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
| **Full suite command** | `task` (lint + whole-repo test, includes the real-Qdrant `internal/store` suite) |
| **Actual runtime** | ~60 s quick · ~5 min full (`internal/store` alone 190–300 s) |

---

## Sampling Rate

- **After every task commit:** `go test -short ./internal/store/... ./internal/server/... ./cmd/engram/...`
- **After every plan:** `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -count=1` (includes `TestSchemaVersionNeverGatesRecall`, `TestRecallEmissionSetIsCompleteAndClassified`, `TestRedEvidencePatchesAreLive`)
- **Phase gate:** full `task` green — run three times independently this phase (last plan commit, post-code-review-fix, post-gap-closure), exit 0 each time
- **Max feedback latency:** 90 seconds (quick run)

---

## Per-Requirement Verification Map

Every `-run` pattern below was re-resolved against `go test -list` on 2026-09-20 and confirmed to match the stated number of real test functions — never assumed from the plan. A pattern matching nothing exits 0 with `no tests to run`, which is the false-green this table exists to prevent (durable record `bsbsvn4hbc`).

| Requirement | Behavior proven | Test Type | Automated Command | Tests matched | Status |
|-------------|-----------------|-----------|-------------------|---------------|--------|
| REQ-list-bounded | `Store.List` stays under the receive limit in every mode — offset, `limit: 0`, deep offset, cursor — on both oversized fixture shapes | integration (real Qdrant) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^(TestStoreListOffsetBounded\|TestStoreListDeepOffsetBounded\|TestStoreListCursorBounded\|TestListCursorModeZeroLimitPagesCorrectly)$' -count=1 -v` | 4 | ✅ green |
| REQ-list-scheduled-bounded | `ListScheduled` with a large explicit limit stays bounded, both shapes | integration (real Qdrant) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestListScheduledBounded$' -count=1 -v` | 1 | ✅ green |
| REQ-search-k-bounded | Two-phase search stays bounded at large `k` and under `full=true`; the empty-id-batch short-circuit issues no Scroll; the no-summary backfill restores content on the search lane | integration (real Qdrant) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^(TestSearchTwoPhaseBounded\|TestSearchFetchSkipsEmptyBatch\|TestSearchNoSummaryContentBackfill)$' -count=1 -v` | 3 | ✅ green |
| REQ-list-contract-unchanged | `total`, ordering and recall gating are independent of internal batching; a budget-cut page is never reported as the last page (`next_cursor` empty only when genuinely exhausted) | integration (real Qdrant) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestStoreListContractInvariant$' -count=1 -v` | 1 | ✅ green |
| REQ-list-limit-contract-decided | An over-maximum `limit`/`k` is rejected — never clamped — on every recall surface, before any backend call; the maximum is stated numerically on every documented surface | integration + docs gate | `go test ./internal/server/ -run '^(TestOutOfRangeRejectedOnEveryRecallSurface\|TestOutOfRangeRejectedBeforeAnyBackend\|TestRecallMaximumIsStatedNumerically)$' -count=1 -v` | 3 | ✅ green |

**Structural gates that also guard this phase** (not requirement-specific, but they go red on a regression here):

| Gate | Command | Guards |
|------|---------|--------|
| Recall-gate AST | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^(TestRecallEmissionSetIsCompleteAndClassified\|TestSchemaVersionNeverGatesRecall)$' -count=1 -v` | every new Qdrant-emitting helper is classified; `schema_version` never enters a recall filter; per-entry-point RPC counts do not drift |
| Red evidence | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1 -v` | all 42 registered patches (Phase 1×4, 2×8, 3×13, 4×17) still apply and still go RED |
| Key links | `go test ./internal/keylinks/ -count=1` | every plan's `key_links` pattern still matches its target |

---

## Wave 0 Requirements

All satisfied — this phase wrote 11 new test files against the existing `storetest` harness (no new tooling was needed; Phases 1–3 built it):

- [x] `internal/store/listbounded_oversized_test.go` — REQ-list-bounded (offset, `limit: 0`, deep offset)
- [x] `internal/store/list_cursormode_zerolimit_test.go` — REQ-list-bounded / REQ-list-contract-unchanged (the CR-01 regression)
- [x] `internal/store/listcontract_oversized_test.go` — REQ-list-contract-unchanged
- [x] `internal/store/listscheduled_oversized_test.go` — REQ-list-scheduled-bounded
- [x] `internal/store/searchtwophase_oversized_test.go` — REQ-search-k-bounded (+ the Search backfill assertion)
- [x] `internal/store/searchfetch_batchfallback_test.go` — REQ-search-k-bounded (the WR-01 batch-of-1 fallback)
- [x] `internal/store/recallview_oversized_test.go` — projection + List's no-summary backfill
- [x] `internal/store/recallmax_oversized_test.go` — the store's refuse-never-clamp backstop
- [x] `internal/server/outofrange_test.go` — REQ-list-limit-contract-decided (seven-surface rejection)
- [x] `internal/server/recallfullthreading_test.go` — projection threaded through both transports and the rule listing
- [x] `internal/server/recallmaxdocs_test.go` — the maximum stated numerically on every documented surface
- [x] Framework install: none required

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Console `/ui/observe` pager still shows numbered pages of 50, and the root page lists 50 recent memories | REQ-list-contract-unchanged | The chromedp e2e asserts the root route renders; page arithmetic across a >50-record scope is visual | `ENGRAM_REQUIRE_QDRANT=1 ENGRAM_REQUIRE_BROWSER=1 go test ./internal/server/... -run 'TestConsole' -count=1 -v`, then open `/ui/observe` on a scope with >50 records and page forward |

The console needs no code change this phase (offset pages keep their size; the root page passes an explicit `limit: 50`), so this is a confirmation, not a gap.

---

## Validation Audit 2026-09-20

| Metric | Count |
|--------|-------|
| Requirements audited | 5 |
| COVERED | 5 |
| PARTIAL | 0 |
| MISSING | 0 |
| Gaps filled this audit | 1 (Search no-summary backfill assertion — WINDOWS #10, closed by `44b36d9d`) |
| Escalated to manual-only | 0 |

**`-run` name corrections.** The map seeded at plan time carried four illustrative test names. Three became real (`TestStoreListOffsetBounded`, `TestListScheduledBounded`, `TestSearchTwoPhaseBounded`); two never existed under the seeded spelling and were re-resolved against `go test -list`:

| Seeded (would have matched nothing) | Actual |
|---|---|
| `TestListContractInvariant` | `TestStoreListContractInvariant` |
| `TestOutOfRangeRejection` | `TestOutOfRangeRejectedOnEveryRecallSurface`, `TestOutOfRangeRejectedBeforeAnyBackend` |

Left uncorrected, both rows would have reported a permanent false green.
