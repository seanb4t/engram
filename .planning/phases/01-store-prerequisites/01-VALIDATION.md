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
| 01-xx | TBD | TBD | STORE-01 | T-01 access control | Non-owner archive/restore of a readable shared record returns ErrNotFound, record unchanged | integration | `go test ./internal/store/... -run TestArchiveAsOwnerGate -v` | ❌ W0 | ⬜ pending |
| 01-xx | TBD | TBD | STORE-01 | T-01 access control | ActionArchive denied for shared non-owned resource at Cedar corpus level | unit | `go test ./internal/authz/... -run TestPolicyCorpus_SharedReadOnly -v` | ✅ widen | ⬜ pending |
| 01-xx | TBD | TBD | STORE-01 | — | CLI spine-review archive/restore unchanged | compile + existing | `go build ./... && go test ./cmd/engram/... -count=1` | ✅ | ⬜ pending |
| 01-xx | TBD | TBD | STORE-02 | T-01 info disclosure | RelatedMemories never returns an unreadable candidate; read predicate composed into every sub-query | integration | `go test ./internal/store/... -run TestRelatedMemories -v` | ❌ W0 | ⬜ pending |
| 01-xx | TBD | TBD | STORE-03 | T-01 info disclosure | ListTags counts only caller-readable, recall-visible records | integration | `go test ./internal/store/... -run TestListTags -v` | ❌ W0 | ⬜ pending |
| 01-xx | TBD | TBD | STORE-03 | — | Recall-gate static scan + interceptor recognize filtered Facet | unit/AST + integration | `go test ./internal/store/... -run 'TestRecallEmissionSetIsCompleteAndClassified|TestSchemaVersionNeverGatesRecall' -v` | ✅ widen | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Planner fills Task IDs / Plan / Wave; executor updates Status.*

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
