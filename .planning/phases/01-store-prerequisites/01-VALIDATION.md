---
phase: "01"
slug: "store-prerequisites"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-25"
---

# Phase 01 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`), integration against real Qdrant via testcontainers-go (`internal/store/storetest`) |
| **Config file** | none — `go test` flags + `ENGRAM_QDRANT_TEST_ADDR` / `ENGRAM_REQUIRE_QDRANT` env vars |
| **Quick run command** | `go test ./internal/store/... ./internal/authz/... -run '<TestName>' -v` |
| **Full suite command** | `go test ./internal/store/... ./internal/authz/... ./cmd/engram/... -count=1` |
| **Estimated runtime** | ~180 seconds (Docker Qdrant) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/store/... ./internal/authz/... -run '<touched tests>' -v`
- **After every plan wave:** Run `go test ./internal/store/... ./internal/authz/... ./cmd/engram/... -count=1` and `gofmt -l .` (repo-wide)
- **Before `/gsd-verify-work`:** `task` (lint + full test suite) must be green
- **Max feedback latency:** 180 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 01-01-01 | 01 | 1 | STORE-01 | T-01 access control | Owner can archive; non-owner of a readable shared record gets ErrNotFound, record unchanged | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestArchiveAsOwnerGate$' -count=1 -v` | ❌ W0 (tracer creates) | ⬜ pending |
| 01-01-02 | 01 | 1 | STORE-01 | T-01 access control | RestoreAs gated identically; ActionArchive denied for shared non-owned at Cedar corpus level; CLI path unchanged | integration + unit | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^Test(ArchiveAs\|RestoreAs)' -count=1 -v` · `go test ./internal/authz/ -run '^TestPolicyCorpus' -count=1` · `go test ./cmd/engram/ -run '^Test(Archive\|SpineReviewArchive\|SpineReviewRestore)' -count=1` | ✅ widen / ❌ W0 | ⬜ pending |
| 01-02-01 | 02 | 2 | STORE-03 | — | tags keyword index created idempotently; ListTags counts owner's tags | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^(TestListTagsCountsOwnedTags\|TestEnsureIndexesCreatesTagsIndex)$' -count=1 -v` | ❌ W0 | ⬜ pending |
| 01-02-02 | 02 | 2 | STORE-03 | T-01 info disclosure | Counts only caller-readable, recall-visible records; limit + more | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestListTags' -count=1 -v` | ❌ W0 | ⬜ pending |
| 01-02-03 | 02 | 2 | STORE-03 | — | Recall-gate static scan + interceptor recognize filtered Facet (all four lists) | unit/AST + integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^(TestRecallEmissionSetIsCompleteAndClassified\|TestSchemaVersionNeverGatesRecall)$' -count=1 -v` | ✅ widen | ⬜ pending |
| 01-03-01 | 03 | 3 | STORE-02 | T-01 info disclosure | Vector edge query-by-id under caller read filter; seeded into recall gate | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^(TestRelatedMemoriesVectorEdge\|TestRelatedMemoriesAnchorAccess)$' -count=1 -v` | ❌ W0 | ⬜ pending |
| 01-03-02 | 03 | 3 | STORE-02 | T-01 info disclosure | Supersession walk; one entry per candidate; bounds; determinism | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestRelatedMemories' -count=1 -v` | ❌ W0 | ⬜ pending |
| 01-04-01 | 04 | 4 | STORE-02 | T-01 info disclosure | Rarity-weighted shared-tag edges from caller-visible facet | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestRelatedMemories' -count=1 -v` | ❌ W0 | ⬜ pending |
| 01-04-02 | 04 | 4 | STORE-02 | T-01 info disclosure | Shared-citation edges (kind+ref); gate walks every edge filter live | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ ./internal/authz/ ./cmd/engram/ -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Executor updates Status; exact PASS-count assertions live in each PLAN.md `<verify>`.*

---

## Wave 0 Requirements

- [ ] Authz-gated Archive/Restore integration test (mirrors `TestDeleteOwnerGate`)
- [ ] `RelatedMemories` integration test covering all four edge types + an unreadable near-neighbour
- [ ] `ListTags` integration test
- [ ] Widen `internal/authz/policy_corpus_test.go` action lists for `ActionArchive`
- [ ] Widen all 4 recall-gate lists in `internal/store/schemaversion_recallgate_test.go` (RESEARCH.md Pitfall 1)

*Existing fixture infra (`testStore`, `dialTestClient`, `storetest.Run`, `Authenticated`/`Anonymous`) covers every shape needed.*

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 180s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
