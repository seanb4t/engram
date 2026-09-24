---
phase: "1"
slug: "eval-foundation-lexical-reranker-fix"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-22"
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`) |
| **Config file** | none — `Taskfile.yaml` targets `eval:retrieval`, `test` |
| **Quick run command** | `go test ./internal/store/... ./internal/retrievaleval/... ./internal/server/...` |
| **Full suite command** | `task` (lint + test); live acceptance: `task eval:retrieval` (needs Qdrant + embedding gateway) |
| **Estimated runtime** | ~60–180 seconds hermetic; live eval several minutes |

---

## Sampling Rate

- **After every task commit:** Run the quick run command (hermetic; eval package skips cleanly with the gate off)
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** `task` green, plus a live `task eval:retrieval` run whose table is recorded (D-09)
- **Max feedback latency:** 180 seconds (hermetic)

---

## Per-Task Verification Map

Filled by the planner/executor from the PLAN.md task IDs. Requirement → test coverage (from RESEARCH.md § Validation Architecture):

| Requirement | Behavior | Test Type | Automated Command | File Exists | Status |
|-------------|----------|-----------|-------------------|-------------|--------|
| EVAL-01 | `cosineDistance` helper: identical ≈ 0, orthogonal ≈ 1, NaN/Inf rejected | unit | `go test ./internal/retrievaleval/ -run TestCosineDistance -v` | ✅ | ✅ green |
| EVAL-01 | Differ gate asserts distance > 1e-3 against the real embedder | integration (gated) | `ENGRAM_RETRIEVAL_EVAL=1 go test ./internal/retrievaleval/ -run TestRetrievalEval_AsymmetryDiffer -v` | ✅ | ✅ green |
| EVAL-02 | Gate + symmetric-config skip read resolved koanf config (test-local load, no registry entry, no `Validate()`) | unit (hermetic, `t.Setenv`) | `go test ./internal/retrievaleval/ -run TestEvalGate -v` | ✅ | ✅ green |
| EVAL-02 | `StoreAndEmbedderFromEnvNoEnsure` returns resolved config; single-load invariant holds | unit | `go test ./internal/server/ -run TestStoreAndEmbedderFromEnvNoEnsure -v` | ✅ | ✅ green |
| RANK-01 | Blend / overlap-gate rank functions are deterministic and correct | unit | `go test ./internal/retrievaleval/ -run 'TestCosineBlendRerank|TestOverlapGateRerank' -v` | ✅ | ✅ green |
| RANK-01 | Paraphrase case + named-ranker table (incl. Jev "disabled") reported | integration (gated) | `task eval:retrieval` | ✅ | ✅ green (live: 01-EVAL-SHIPPED.log) |
| RANK-02 | #261 at rank 1; shipped paraphrase MRR ≥ vector-only | integration (gated, hard `t.Errorf`) | `task eval:retrieval` | ✅ | ✅ green (live: 01-EVAL-SHIPPED.log) |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `cosineDistance` unit tests (identical, orthogonal, near-identical, NaN, Inf)
- [x] `cosineBlendRerank` / `overlapGateRerank` hermetic unit tests in `internal/retrievaleval/comparison_rankers_test.go` (eval-local per plan 01-02; `store.VectorOrder` tests in `internal/store/rerank_test.go`)
- [x] Resolved-config gate/skip unit tests in `internal/retrievaleval` (test-local koanf load; replaces the proposed `config_test.go` registry case — D-15 revised)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live eval numbers decide the shipped ranking (D-05) | RANK-02 | Needs Qdrant + live embedding gateway credentials | Run `task eval:retrieval` with gateway env set; record the per-variant table in the phase SUMMARY and on #605 |
| Blind-subagent authorship of paraphrase queries | RANK-01 | Process, not code | Confirm the fixture comment documents the procedure and that query authoring saw only topic descriptions |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 180s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-09-23

## Validation Audit 2026-09-23
| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

Hermetic tests re-run green: TestCosineDistance, TestEvalGate, TestSymmetricEmbedConfig, TestCosineBlendRerank, TestOverlapGateRerank, TestParaphraseCorpusIntegrity, TestDecideRanking, TestEvalRankersRoster, TestVariantMetrics, TestFormatVariantTable, TestStoreAndEmbedderFromEnvNoEnsure*, TestVectorOrder*, TestCandidateK, TestRankCandidatesIsTheD05Winner. Live evidence: 01-EVAL-SHIPPED.log (both D-10 gates PASS). Live AsymmetryDiffer run skipped under a symmetric embed config; hermetic coverage only (user-acknowledged caveat).
