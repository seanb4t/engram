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
| 04-01-01 | 04-01 | 1 | REQ-list-limit-contract-decided | T-04-01-01, T-04-01-03 | the overflow envelope still carries no digit and no upstream transport text after the rename | unit + docs gate | `go test ./internal/server/ -run '^(TestErrorsDocHintCodesMatchArgErrorConstants\|TestResponseTooLargeEnvelopeShape)$' -count=1 -v` | ✅ existing gate | ⬜ pending |
| 04-01-02 | 04-01 | 1 | REQ-list-limit-contract-decided | T-04-01-02 | the new numeric-maximum hint is published and classified `classMalformed` → `invalid_argument` → exit 2 | unit + docs gate | `go test ./internal/server/ -run '^TestErrorsDocHintCodesMatchArgErrorConstants$' -count=1 -v`; `rg -o -e 'hint=too[_]large' -e '"too[_]large"' internal cmd docs-site/src proto CLAUDE.md skill \| wc -l` must print `0` | ✅ existing gate | ⬜ pending |
| 04-02-01 | 04-02 | 2 | REQ-list-bounded | T-04-02-01, T-04-02-02 | every assembled cursor RPC carries the caller's authz + recall-gate filter unchanged; a crafted cursor's seen set stays bounded | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestStoreListCursorBounded$' -count=1 -v` | ❌ W0 → `internal/store/listbounded_oversized_test.go` | ⬜ pending |
| 04-02-02 | 04-02 | 2 | REQ-list-bounded, REQ-list-limit-contract-decided | T-04-02-03 | a zero limit resolves to the numeric maximum and the offset arithmetic cannot wrap | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestStoreListOffsetBounded$' -count=1 -v` | ❌ W0 → `internal/store/listbounded_oversized_test.go` | ⬜ pending |
| 04-02-03 | 04-02 | 2 | REQ-list-contract-unchanged | T-04-02-04, T-04-02-05 | a byte-cut page is never reported as the last page; the retargeted overflow regressions still overflow | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ ./internal/server/ -run '^(TestStoreListContractInvariant\|TestStoreListOverflowIsResponseTooLarge\|TestConnectListMemoriesResponseTooLarge\|TestMCPListMemoryResponseTooLarge)$' -count=1 -v` | ❌ W0 → `internal/store/listcontract_oversized_test.go` / ✅ existing (retargeted) | ⬜ pending |
| 04-03-01 | 04-03 | 3 | REQ-list-bounded | T-04-03-01, T-04-03-05 | the keys-only prefix walk carries the caller's filter unchanged and differs only in the payload selector | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestStoreListDeepOffsetBounded$' -count=1 -v` | ❌ W0 → `internal/store/listbounded_oversized_test.go` | ⬜ pending |
| 04-03-02 | 04-03 | 3 | REQ-list-scheduled-bounded | T-04-03-02 | the owner-only and scheduled-state conditions ride every assembled RPC | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestListScheduledBounded$' -count=1 -v` | ❌ W0 → `internal/store/listscheduled_oversized_test.go` | ⬜ pending |
| 04-04-01 | 04-04 | 4 | REQ-search-k-bounded | T-04-04-01, T-04-04-02 | the payload fetch is an id set AND the identical phase-one filter; a record that leaves visibility drops out | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestSearchTwoPhaseBounded$' -count=1 -v` | ❌ W0 → `internal/store/searchtwophase_oversized_test.go` | ⬜ pending |
| 04-04-02 | 04-04 | 4 | REQ-search-k-bounded | T-04-04-04, T-04-04-05 | an empty id batch issues no Scroll, so the recall gate's exact search counts hold; rank order is the query's own | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^(TestSearchDiscoveryTwoPhaseBounded\|TestSearchFetchSkipsEmptyBatch\|TestSearchPreservesRankOrder)$' -count=1 -v` | ❌ W0 → `internal/store/searchtwophase_oversized_test.go` | ⬜ pending |
| 04-05-01 | 04-05 | 5 | REQ-list-bounded, REQ-list-contract-unchanged | T-04-05-01, T-04-05-02 | the no-summary backfill inherits the id-set fetch's narrow-only filter and changes nothing a caller sees | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^(TestRecallViewSelection\|TestNoSummaryContentBackfill)$' -count=1 -v` | ❌ W0 → `internal/store/recallview_oversized_test.go` | ⬜ pending |
| 04-05-02 | 04-05 | 5 | REQ-list-limit-contract-decided | T-04-05-03, T-04-05-04 | an over-maximum count is refused before any Qdrant call; nothing clamps | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestStoreRejectsOverMaximumCount$' -count=1 -v` | ❌ W0 → `internal/store/recallmax_oversized_test.go` | ⬜ pending |
| 04-06-01 | 04-06 | 6 | REQ-list-bounded | T-04-06-03, T-04-06-05 | the projection selects a payload view, never a filter; the rule listing does not regress | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^(TestFullSelectsFetchView\|TestListRulesFullThreaded)$' -count=1 -v` | ❌ W0 → `internal/server/recallfullthreading_test.go` | ⬜ pending |
| 04-06-02 | 04-06 | 6 | REQ-search-k-bounded, REQ-list-scheduled-bounded, REQ-list-limit-contract-decided | T-04-06-01, T-04-06-02, T-04-06-04 | `limit`/`k` above the maximum rejected with `field=<limit\|k> hint=out_of_range` before any scope resolution, embed or Qdrant call | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^(TestOutOfRangeRejectedOnEveryRecallSurface\|TestOutOfRangeRejectedBeforeAnyBackend)$' -count=1 -v`; `go test ./internal/server/ -run '^(TestValidationErrorAttributionMatrix\|TestHintNeverEchoesValue)$' -count=1 -v` | ❌ W0 → `internal/server/outofrange_test.go` / ✅ existing matrix (extended) | ⬜ pending |
| 04-06-03 | 04-06 | 6 | REQ-list-limit-contract-decided | — | N/A | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestZeroCountKeepsSurfaceDefaults$' -count=1 -v` | ❌ W0 → `internal/server/outofrange_test.go` | ⬜ pending |
| 04-07-01 | 04-07 | 6 | REQ-list-limit-contract-decided | T-04-07-02 | the wire schema and CLI help state the maximum; every generated tree is regenerated, never hand-edited | golden + drift | `go test ./cmd/engram/ -run '^(TestHelpGolden\|TestCatalogGolden)$' -count=1 -v`; `task proto:lint && task proto:gen && git diff --exit-code -- gen ui/src/lib/gen proto` | ✅ existing goldens (regenerated) | ⬜ pending |
| 04-07-03 | 04-07 | 6 | REQ-list-limit-contract-decided | T-04-07-01 | every documented recall surface states the maximum numerically; no retired wording survives | docs gate | `go test ./internal/server/ -run '^TestRecallMaximumIsStatedNumerically$' -count=1 -v` | ❌ W0 → `internal/server/recallmaxdocs_test.go` | ⬜ pending |
| 04-08-02 | 04-08 | 7 | all five | T-04-08-02 | every guarantee this phase adds is RED-provable, and the harness leaves the tree clean | harness | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1 -v -timeout 90m` (42 `confirmed RED:` lines) | ✅ existing harness | ⬜ pending |
| 04-08-03 | 04-08 | 7 | all five | T-04-08-03, T-04-08-04 | requirements ticked only on shipped evidence; no other phase's recorded state touched | doc gate | `rg -c -e '^- \[x\] \*\*REQ-(list-bounded\|list-scheduled-bounded\|search-k-bounded\|list-contract-unchanged\|list-limit-contract-decided)\*\*' .planning/REQUIREMENTS.md` must print `5`; `task` exits 0 | ✅ existing | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Task IDs are `{phase}-{plan}-{task}` and were assigned by `/gsd-plan-phase` on 2026-09-19. Every `-run` name above is the name the owning plan's task authors; re-resolve each against `go test -list` before relying on it (`bsbsvn4hbc`: prove execution with `-v` RUN/PASS pairs, never a bare package `ok`).*

---

## Wave 0 Requirements

- [ ] `internal/store/listbounded_oversized_test.go` (`package store_test`, plans 04-02/04-03) — REQ-list-bounded: `TestStoreListCursorBounded`, `TestStoreListOffsetBounded` (zero limit → the numeric maximum, the offset wrap guard), `TestStoreListDeepOffsetBounded`; both `storetest.SeedOversized` shapes at `storetest.RecvLimit`
- [ ] `internal/store/listcontract_oversized_test.go` (plan 04-02) — REQ-list-contract-unchanged: `TestStoreListContractInvariant` — `total` exact, `next_cursor` empty only on `Exhausted`, a `CutByBudget` page at a shrunken `pageByteBudget` (`store.SetByteBudgets`) returns a non-empty cursor, ordering and recall gating unchanged
- [ ] `internal/store/listscheduled_oversized_test.go` (plan 04-03) — REQ-list-scheduled-bounded: `TestListScheduledBounded` at a large explicit limit, both shapes, with the owner-only and state gates re-asserted
- [ ] `internal/store/searchtwophase_oversized_test.go` (plan 04-04) — REQ-search-k-bounded: `TestSearchTwoPhaseBounded`, `TestSearchDiscoveryTwoPhaseBounded`, `TestSearchFetchSkipsEmptyBatch`, `TestSearchPreservesRankOrder`; both shapes, the cross-owner id-batch subtest, and the drop-on-disappear subtest
- [ ] `internal/store/recallview_oversized_test.go` (plan 04-05) — `TestRecallViewSelection`, `TestNoSummaryContentBackfill`: the projection follows the caller and the truncation fallback still sees content
- [ ] `internal/store/recallmax_oversized_test.go` (plan 04-05) — `TestStoreRejectsOverMaximumCount`: the store's refuse-never-clamp backstop on all four entry points, with zero recorded Qdrant calls
- [ ] `internal/server/outofrange_test.go` (plan 04-06) — `TestOutOfRangeRejectedOnEveryRecallSurface` (all seven knobs), `TestOutOfRangeRejectedBeforeAnyBackend`, `TestZeroCountKeepsSurfaceDefaults`; plus new rows in `internal/server/argattribution_test.go`
- [ ] `internal/server/recallfullthreading_test.go` (plan 04-06) — `TestFullSelectsFetchView`, `TestListRulesFullThreaded`
- [ ] `internal/server/recallmaxdocs_test.go` (plan 04-07) — `TestRecallMaximumIsStatedNumerically`: every documented surface states the maximum and no retired wording survives
- [ ] Phase 2 retargets (D-12, plan 04-02): `internal/server/responsetoolarge_test.go`'s Connect and MCP overflow regressions and `internal/store/responsetoolarge_oversized_test.go` re-pointed at a single legacy record whose payload alone exceeds `storetest.RecvLimit`, reached through a recall entry point via the batch-of-1 fallback. `internal/store/storetest/storetest_test.go` is NOT edited — its `port_too_large` case names a TCP port bound, not the renamed hint. Phase 2's `02-03-errors-doc-drops-*.patch` is renamed and regenerated by plan 04-01.
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
