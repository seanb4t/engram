---
phase: 09-retrieval-eval-harness-ranking-precision
verified: 2026-07-10T00:44:59Z
status: human_needed
score: 3/3 must-haves verified
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "Run `ENGRAM_RETRIEVAL_EVAL=1 task eval:retrieval` with prod-parity env (`ENGRAM_EMBED_MODEL=qwen3-embedding-8b`, `ENGRAM_EMBED_DIM=4096`, `ENGRAM_EMBED_QUERY_INSTRUCTION` set per PR #262) once the qwen3-via-OpenRouter brownout resolves."
    expected: "Record T ranks within default k=8 (ideally rank 1) for both Query A and Query B on the actual production embedder; recall@8/MRR at or above the numbers already captured with the gemini-embedding-2 substitute (recall@8=1.00, MRR=1.000)."
    why_human: "Requires a live OpenAI-compatible embedder gateway configured with the exact production model/dim/instruction and Docker — not available in this automated verification session. The disclosed live run in 09-03-SUMMARY.md substituted `gemini-embedding-2` @3072 (and later cross-checked with `gemini-embedding-001`) because qwen3-via-OpenRouter was browning out (30s+/call vs. the client's hardcoded 30s timeout), and `ENGRAM_EMBED_QUERY_INSTRUCTION` was left unset. The D-06 rerank MECHANISM itself is proven embedder-independently by `internal/server/connectapi_test.go#TestRerankParityMCPAndConnect` (a tied-raw-score corpus where only the lexical rerank can promote the target — I re-ran this test and it passes), so the ordering logic is not in doubt; what remains unconfirmed is whether the full pipeline reaches the SC3 bar on the actual prod embedder config, per this phase's own D-05a baseline definition."
---

# Phase 9: Retrieval Eval Harness & Ranking Precision Verification Report

**Phase Goal:** A near-verbatim restatement of a record reliably surfaces that record within default `k`, and recall changes are measurable — so `search-before-store` stops producing duplicates (the #261 failure).
**Verified:** 2026-07-10T00:44:59Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth (ROADMAP Success Criterion) | Status | Evidence |
|---|---|---|---|
| 1 | A reproducible retrieval eval (`task eval:retrieval`) runs a labeled query→expected-record dataset — including the #261 miss as a regression fixture — and reports recall@k/MRR; CI or a make target can detect regressions. | ✓ VERIFIED | `internal/retrievaleval/` package exists (doc.go, retrieval_eval_test.go, fixtures.go); `Taskfile.yaml:56-59` `eval:retrieval` target runs `ENGRAM_RETRIEVAL_EVAL=1 go test ./internal/retrievaleval/ -run TestRetrievalEval -v`. `fixtures.go` encodes the permanent #261 regression fixture: Record T + 15 sticky topical-neighbor distractors (≥ the `defaultK+1`=9 floor) + Query A/B near-verbatim restatements. `TestRetrievalEval` computes and `t.Logf`s recall@k/MRR. Gate-unset run confirmed clean (`go test ./internal/retrievaleval/...` → SKIP, exit 0, cached). Source-order guard confirmed: `ENGRAM_RETRIEVAL_EVAL` gate (line 235) precedes `tcqdrant.Run` (line 245) in `TestMain`. |
| 2 | `search_memory` returns a per-result similarity score and the eval asserts score separation between the target record and its sticky topical neighbors (NOTE: score is always-on/documented per D-01/D-04; score-separation is a REPORTED diagnostic per D-03-as-narrowed, rank-within-k is the machine-checkable proof). | ✓ VERIFIED | Score is documented in all three client-visible surfaces: `internal/server/tools.go:937` search_memory `Description` ("Each result carries a `score`: the raw Qdrant cosine similarity..."); `CLAUDE.md:60-61`; `docs-site/src/content/docs/reference/tools.md:100-104`. `internal/retrievaleval/retrieval_eval_test.go:160-176` computes and `t.Logf`s `score(T)` vs. best-distractor score as a diagnostic (never `t.Errorf`), consistent with the D-03-narrowing this phase records. The rank-within-k hard assertion (line 170-174) is the machine-checkable proof, matching the phase's own reframing. |
| 3 | Phrasing-sensitive misses are eliminated: Query A/B from #261 surface Record T within default `k`, via the approach chosen by the eval numbers (D-06 heuristic rerank accepted; D-07/D-08 evaluated and not needed). | ✓ VERIFIED (mechanism) / see human item | `internal/store/rerank.go` implements a pure, stdlib-only lexical-overlap reranker (`RerankHits`); `internal/store/store.go:607-614` `SearchReranked` (over-fetch `candidateK=min(max(k*4,32),100)` → `RerankHits` → truncate; rejects `k<=0`). Both `deps.searchMemory` (`tools.go:724`) and `engramAPI.SearchMemories` (`connectapi.go:165`) call the SAME shared helper — verified by grep and by re-running `TestRerankParityMCPAndConnect`, which PASSED (identical reranked order, k/tags/created-window parity, no cross-owner leak). Hermetic unit tests (`TestCandidateK`, `TestSearchRerankedRejectsZeroK`, `TestRerankHitsPromotesLexicalOverlap`, `TestRerankHitsDeterministic`, `TestRerankHitsTruncatesToK`) all PASS. The #261 fixture's hard rank bar is coded as `t.Errorf` (not `t.Logf`) in the eval. A live run recorded in 09-03-SUMMARY.md shows recall@8=1.00/MRR=1.000/T-rank=1 for both queries — but that run used `gemini-embedding-2`/`gemini-embedding-001` @3072, NOT prod-parity `qwen3-embedding-8b` @4096 (D-05a's own baseline embedder), because qwen3-via-OpenRouter was browning out. This substitution is disclosed honestly in the SUMMARY and is not hidden. The rerank ordering MECHANISM is proven embedder-independently (parity test, tied-score corpus). What is NOT independently reconfirmed in this verification session is the SC3 outcome on the actual prod embedder — routed to human verification below. |

**Score:** 3/3 truths verified (0 present, behavior-unverified) — 1 truth carries a disclosed follow-up requiring a live prod-embedder run (human verification, not a code gap).

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `internal/retrievaleval/doc.go` | Package doc + SPDX header | ✓ VERIFIED | Exists, 17 lines, Apache-2.0 header present, `task license:check` passes. |
| `internal/retrievaleval/retrieval_eval_test.go` | TestMain gate + TestRetrievalEval + helpers | ✓ VERIFIED | 269 lines; gate precedes `tcqdrant.Run`; measured path calls `SearchReranked` (not raw `Search`) for the ranked queries; ceiling check uses raw `Search` only to prove harness soundness. |
| `internal/retrievaleval/fixtures.go` | `retrievalCase`/`seedRecord` types + #261 fixture | ✓ VERIFIED | 107 lines; Record T + 15 named distractors + Query A/B; `recallAtK`/`reciprocalRank` helpers present. |
| `Taskfile.yaml` `eval:retrieval` target | Runs the gated eval | ✓ VERIFIED | Present at line 56-59, correct command shape mirroring `eval:summary`. |
| `internal/store/rerank.go` | Pure heuristic reranker (D-06) | ✓ VERIFIED | 100 lines, stdlib-only (`sort`, `strings`); no `internal/embed`/`internal/server` import; deterministic tie-break (overlap → Score → ID). |
| `internal/store/rerank_test.go` | Hermetic unit tests | ✓ VERIFIED | 95 lines; 5 test functions, all PASS (re-run in this session). |
| `internal/store/store.go` `SearchReranked` | Shared over-fetch+rerank+truncate helper | ✓ VERIFIED | Lines 589-614; `candidateK` bounds (32 floor / 100 cap) confirmed by `TestCandidateK` subtests; `k==0` → `ErrInvalidArgument` confirmed by `TestSearchRerankedRejectsZeroK`. |
| `internal/server/tools.go` `deps.searchMemory` | Calls shared helper | ✓ VERIFIED | Line 724: `d.st.SearchReranked(...)`; default `k=8` applied first (line 705-707). |
| `internal/server/connectapi.go` `engramAPI.SearchMemories` | Calls SAME shared helper | ✓ VERIFIED | Line 165: `a.d.st.SearchReranked(...)`; default `k=20` applied first (line 152-154). |
| `internal/server/connectapi_test.go` `TestRerankParityMCPAndConnect` | Real two-surface behavioral test | ✓ VERIFIED | Lines 356-497; 5 subtests (identical order, k, tags+filter, created-window, no cross-owner leak); re-run in this session, all PASS. |
| `docs-site/.../reference/tools.md` | score field + rerank-order caveat | ✓ VERIFIED | Lines 95, 100-104: score field documented, rerank-order caveat present ("Final order may include reranking; `score` remains first-stage dense similarity and may be non-monotonic after rerank"), 09-02's score-field docs preserved (not overwritten). |
| `CLAUDE.md` Memory-contract section | score field documented | ✓ VERIFIED | Lines 60-61. |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `deps.searchMemory` (tools.go) | `store.SearchReranked` | direct call | ✓ WIRED | Line 724, after default-k applied. |
| `engramAPI.SearchMemories` (connectapi.go) | `store.SearchReranked` | direct call | ✓ WIRED | Line 165, after default-k applied. |
| `internal/retrievaleval` eval | `store.SearchReranked` | direct call | ✓ WIRED | `retrieval_eval_test.go:122` — the measured path uses the SAME shared helper as both handlers, no drift. |
| `internal/store/rerank.go` | `internal/embed` / `internal/server` | (prohibited import) | ✓ CONFIRMED ABSENT | grep confirms neither import path appears in `rerank.go` or the `SearchReranked` region (only in a code comment describing the constraint). |

### Behavioral Spot-Checks / Re-run tests

| Behavior | Command | Result | Status |
|---|---|---|---|
| Gate-unset eval self-skips, zero Docker cost | `go test ./internal/retrievaleval/...` | `SKIP: TestRetrievalEval`, exit 0 | ✓ PASS |
| Source-order guard (gate precedes tcqdrant.Run) | `awk` line-order check | gate=235 < run=245 | ✓ PASS |
| Rerank hermetic unit tests | `go test ./internal/store/ -run 'TestCandidateK\|TestSearchRerankedRejectsZeroK\|TestRerankHits'` | all 4 groups PASS (incl. 6 `TestCandidateK` subtests) | ✓ PASS |
| Two-surface behavioral parity (MCP == Connect reranked order, k/tags/window honored, no cross-owner leak) | `go test ./internal/server/ -run TestRerankParityMCPAndConnect -v` | all 5 subtests PASS | ✓ PASS |
| Full build | `go build ./...` | clean | ✓ PASS |
| Full vet | `go vet ./...` (scoped to touched packages) | clean | ✓ PASS |
| Full test suite (once) | `go test ./...` | all packages `ok` | ✓ PASS |
| Go lint | `task lint:go` (golangci-lint) | `0 issues` | ✓ PASS |
| License headers | `task license:check` | 619 checked, 0 invalid | ✓ PASS |
| Doc grep: score + cosine/similarity wording | `rg -i "cosine\|similarity" tools.go CLAUDE.md reference/tools.md` | present in all three | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|---|---|---|---|---|
| REQ-retrieval-eval | 09-01 | Reproducible retrieval-quality eval harness w/ #261 regression fixture, recall@k/MRR, `task eval:retrieval` | ✓ SATISFIED | See Truth 1 above. |
| REQ-search-similarity-scores | 09-01, 09-02 | `search_memory` returns/documents a per-result similarity score | ✓ SATISFIED | See Truth 2 above; documented in 3 surfaces, populated end-to-end (`store.Memory.Score`→`recallView.Score`/Connect). |
| REQ-ranking-precision | 09-03 | Eliminate phrasing-sensitive ranking misses, approach selected by eval numbers | ✓ SATISFIED (mechanism); prod-parity confirmation outstanding | See Truth 3 above and the human verification item. |

No orphaned requirements: `.planning/REQUIREMENTS.md` maps exactly these 3 IDs to Phase 9, and all 3 appear across the three plans' `requirements:` frontmatter.

### Anti-Patterns Found

None blocking. Scanned all files modified by this phase (`rerank.go`, `rerank_test.go`, `store.go`, `tools.go`, `connectapi.go`, `connectapi_test.go`, `tools_test.go`, `retrievaleval/*.go`, `Taskfile.yaml`, `CLAUDE.md`, `docs-site/.../tools.md`) for `TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER` — zero matches in phase-touched files (one unrelated pre-existing `TODO` reference in `CLAUDE.md`'s Issue Tracking section, not touched by this phase).

`task lint` (the full aggregate) reports ~140 pre-existing markdown-lint issues, but all are in `.planning/phases/09-.../09-CONTEXT.md`, `09-PATTERNS.md`, `09-03-PLAN.md`, and `09-03-SUMMARY.md` — planning artifacts, not phase deliverables. `09-02-SUMMARY.md` already disclosed an equivalent pre-existing count (132) and confirmed it reproduces identically with the phase diff stashed out; `task lint:go`, `task license:check`, and the docs-site/CLAUDE.md-scoped rumdl checks all pass clean. Not treated as a phase blocker.

### Human Verification Required

1. **Prod-parity retrieval eval confirmation**

   **Test:** Run `ENGRAM_RETRIEVAL_EVAL=1 task eval:retrieval` with `ENGRAM_EMBED_MODEL=qwen3-embedding-8b`, `ENGRAM_EMBED_DIM=4096`, and `ENGRAM_EMBED_QUERY_INSTRUCTION` set per PR #262 (the D-05a prod-parity baseline config), once the qwen3-via-OpenRouter brownout resolves.
   **Expected:** Record T ranks within default `k`=8 for both Query A and Query B on the actual production embedder; recall@8/MRR at or above the gemini-substitute numbers already captured (recall@8=1.00, MRR=1.000).
   **Why human:** Requires a live gateway configured with the exact prod model/dim/instruction plus Docker — unavailable in this automated verification session. The disclosed live run in `09-03-SUMMARY.md` used `gemini-embedding-2`/`gemini-embedding-001` @3072 as a substitute because qwen3-via-OpenRouter exceeded the embed client's 30s timeout, and `ENGRAM_EMBED_QUERY_INSTRUCTION` was unset for that run. This is not a hidden gap — it is honestly disclosed in the SUMMARY as a "Follow-Ups Filed" item — but it means SC3 (Query A/B surface Record T within default k) has not yet been reconfirmed on the exact D-05a baseline embedder. The rerank ORDERING mechanism itself is independently proven (embedder-agnostic) by `TestRerankParityMCPAndConnect`, which this verification re-ran and confirmed passing.

### Gaps Summary

No code-level gaps. All three ROADMAP success criteria have corresponding, working, tested implementations: the eval harness (with the #261 regression fixture, 15 distractors, gate-unset CI-safety verified), the documented always-on score (3 surfaces), and the shared D-06 lexical-overlap reranker wired into both recall surfaces with real behavioral parity tests (re-run and passing) plus hermetic unit tests (re-run and passing) covering `candidateK` bounds and the `k<=0` contract. `go build`, `go vet`, the full `go test ./...`, `task lint:go`, and `task license:check` all pass clean.

The single open item is a disclosed, not-yet-executed operational follow-up: confirming the #261 rank bar on the actual prod-parity embedder (`qwen3-embedding-8b` @4096 with the PR #262 query instruction) rather than the gemini substitute used in the one live run captured so far. This requires a live gateway not available to this verifier and is recorded as a human-verification item rather than a gap, since the shipped code, its unit tests, and its cross-surface behavioral tests are all present, wired, and passing today.

---

_Verified: 2026-07-10T00:44:59Z_
_Verifier: Claude (gsd-verifier)_
