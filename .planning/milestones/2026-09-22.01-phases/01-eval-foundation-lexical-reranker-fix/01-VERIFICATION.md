---
phase: 01-eval-foundation-lexical-reranker-fix
verified: 2026-09-24T17:37:02Z
status: passed
score: 4/4 must-haves verified
covered_files: [".planning/phases/01-eval-foundation-lexical-reranker-fix/01-01-PLAN.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-01-SUMMARY.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-02-PLAN.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-02-SUMMARY.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-03-PLAN.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-03-SUMMARY.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-04-PLAN.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-04-SUMMARY.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-05-PLAN.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-05-SUMMARY.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-06-PLAN.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-06-SUMMARY.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-CONTEXT.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-EVAL-BASELINE.log", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-EVAL-SHIPPED.log", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-ISSUE-605-EVIDENCE.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-RANKING-DECISION.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-REVIEW.md", "cmd/engram/reindex.go", "docs-site/src/content/docs/reference/tools.md", "internal/retrievaleval/comparison_rankers.go", "internal/retrievaleval/comparison_rankers_test.go", "internal/retrievaleval/doc.go", "internal/retrievaleval/fixtures.go", "internal/retrievaleval/gate.go", "internal/retrievaleval/gate_test.go", "internal/retrievaleval/paraphrase_fixture.go", "internal/retrievaleval/paraphrase_fixture_test.go", "internal/retrievaleval/rankers.go", "internal/retrievaleval/rankers_test.go", "internal/retrievaleval/retrieval_eval_test.go", "internal/retrievaleval/vector.go", "internal/retrievaleval/vector_test.go", "internal/server/connectapi_test.go", "internal/server/tools.go", "internal/server/tools_test.go", "internal/store/rerank.go", "internal/store/rerank_test.go", "internal/store/store.go"]
covered_digest: "v1:sha256:c888096b21154344d7705734710199d5c74cc317c0b02689f2ceb463f98260af"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: passed
  previous_score: 4/4
  gaps_closed: []
  gaps_remaining: []
  regressions: []
---

# Phase 1: Eval Foundation & Lexical Reranker Fix Verification Report

**Phase Goal:** The retrieval eval measures reranking correctly, including a paraphrase case, so later reranking work (Phase 4) has trustworthy numbers to gate on.
**Verified:** 2026-09-24T17:37:02Z (HEAD `f37c5f0e`)
**Status:** passed
**Re-verification:** Yes. The prior `passed` report (fingerprinted at commit `673ff3e5`) read `stale` because Phases 3–5 changed files in its `covered_files`. It had no `gaps:`. This run repeats goal-backward verification against HEAD.

## Re-Verification 2026-09-24: Later-Phase Changes to Covered Files

`git diff 673ff3e5..HEAD --stat -- <covered_files>`: 11 files changed, 987 insertions(+), 45 deletions(-). All 39 covered paths still exist, so none were pruned. The file set is unchanged: every `01-0N-PLAN.md`/`01-0N-SUMMARY.md` plus the delivered source, test, and docs files. `.planning/REQUIREMENTS.md`, `STATE.md`, `ROADMAP.md`, `PROJECT.md`, and this report remain excluded.

| File | Changed by | Effect on a Phase 1 truth |
|------|-----------|---------------------------|
| `internal/store/store.go` | `ca022a70`, `eb68f0e6` (Phase 4) | `SearchReranked` now returns `RankWithHook(ctx, query, hits, int(k), opts.RankHook)`. It also adds a transient `Memory.Relevance`, `SearchOptions.RankHook`, and the new `SearchDiscoveryReranked`. **No regression**: with a nil hook, `RankWithHook` returns `rankCandidates(query, hits, k)`, which is the call Phase 1 shipped. |
| `internal/store/rerank.go` | `ca022a70` (Phase 4) | Purely additive: `RankHook`, `applyRelevance`, `applyRankHook`, `RankWithHook`. `rankCandidates` still delegates to `RerankHits` (the D-05 lexical winner), and `CandidateK`/`VectorOrder` are untouched. |
| `internal/retrievaleval/rankers.go` | `4863f4d0` (Phase 4) | `evalRankers()` became `evalRankers(jevHook)`. A nil hook appends the same disabled `Jev: disabled` stub. A non-nil hook appends an `optIn` Jev row, which `decideRanking` and `formatVariantTable` exclude from D-05 (`row.optIn` → skip / `opt-in`). The D-05 rule for every other row is unchanged. |
| `internal/retrievaleval/retrieval_eval_test.go` | `f16ee36a` (Phase 4) | Builds the Jev hook through `server.SearchRankHookFromEnv()` and passes it to `evalRankers`. It logs `JEV-EVAL` provenance and fallback counts. The shipped row still calls `st.SearchReranked(..., store.SearchOptions{})`, which has no hook and so stays lexical. Both D-10 gates and the `TestMain` → `retrievalEvalEnabled()` gate are intact. This change fulfills SC3's "Jev when enabled" clause instead of weakening it. |
| `internal/server/tools.go` | `963df2c3`, `a94a39f4` (Phase 4) | Additive `deps.rankHook`, which is nil unless `ENGRAM_SEARCH_RANKER=jev`; the config default is `lexical`, validated in `internal/config/validate.go`. The hook is threaded into `searchMemory`'s `SearchOptions`. `search_discovery` gains an opt-in branch. `StoreAndEmbedderFromEnvNoEnsure` (`tools.go:290`) keeps its 6-value signature. |
| `internal/server/tools_test.go`, `connectapi_test.go` | Phase 4 | Additive tests only. `TestRerankParityMCPAndConnect` gained 5 Jev-hook subtests. |
| `internal/retrievaleval/rankers_test.go` | `4863f4d0` | Adds `TestEvalRankersJevEnabled` and updates call sites to `evalRankers(nil)`. |
| `docs-site/.../reference/tools.md` | `b673d0e3` | Documents the opt-in per-hit `relevance` field. The default lexical ranking description is unaffected. |
| `01-04-PLAN.md`, `01-06-PLAN.md` | `48924c3d`, `61f12212` | Key-link `pattern:` values were widened to follow the renamed call sites (`evalRankers[(]`, `rankCandidates[(]query, hits, (k\|len[(]hits[)])[)]`). The via/intent text did not change. `verify.key-links` passes for both (5/5, 2/2). |

Files not touched since `673ff3e5`: `gate.go`, `vector.go`, `paraphrase_fixture.go`, `comparison_rankers.go`, `fixtures.go`, `doc.go`, `rerank_test.go`, `cmd/engram/reindex.go`, all evidence logs, and `01-RANKING-DECISION.md`/`01-ISSUE-605-EVIDENCE.md`.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence (HEAD `f37c5f0e`) |
|---|-------|--------|----------|
| 1 | The embedding differ gate compares vectors with a cosine epsilon instead of `reflect.DeepEqual` on `[]float32` (ROADMAP SC1 / EVAL-01) | ✓ VERIFIED | `vector.go` is unchanged, and `retrieval_eval_test.go:547-552` still gates on `cosineDistance(...) > differMinCosineDistance`. The remaining `reflect.DeepEqual` uses in the package compare only id/int slices. `TestCosineDistance` PASS. |
| 2 | The retrieval-eval skip guard reads the resolved koanf config rather than raw `os.Getenv` (ROADMAP SC2 / EVAL-02) | ✓ VERIFIED | `gate.go` is unchanged. `retrieval_eval_test.go:67,589` call `retrievalEvalEnabled()`, and `os.Getenv` has zero matches in `internal/retrievaleval/`. `TestEvalGate` PASS. |
| 3 | `task eval:retrieval` reports recall@k and MRR for vector-only, lexical-reranked, and (when enabled) Jev-reranked ordering, including a paraphrase case written independently of #261's targets (ROADMAP SC3 / RANK-01) | ✓ VERIFIED | The roster still holds vector-only, lexical, the overlap-gate and cosine-blend grids, and a Jev row. The Jev row now actually ranks through `store.RankWithHook` when a decisions provider is configured. `TestEvalRankersRoster`, `TestEvalRankersJevEnabled`, `TestParaphraseCorpusIntegrity`, `TestDecideRanking`, and `TestFormatVariantTable` PASS. `01-EVAL-SHIPPED.log` is unchanged (`EXIT_STATUS:0`). |
| 4 | On that paraphrase case, the shipped ranking does not regress vs vector-only and keeps #261's target at rank 1 (ROADMAP SC4 / RANK-02) | ✓ VERIFIED | The default shipped path is `SearchReranked` → `RankWithHook(..., nil)` → `rankCandidates` → `RerankHits` (lexical, the D-05 winner). `TestRankWithHookNilIsRankCandidates` and `TestRankCandidatesIsTheD05Winner` PASS. The eval's D-10 gates (`retrieval_eval_test.go:388-449`) still measure that default path. Jev stays an operator opt-in (`ENGRAM_SEARCH_RANKER=jev`) that never enters D-05. |

**Score:** 4/4 truths verified (0 present, behavior-unverified)

### Required Artifacts

`gsd-tools verify.artifacts` over all six plans: 22/22 pass.

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/retrievaleval/gate.go` | `resolveEvalGate`/`retrievalEvalEnabled`, koanf-based | ✓ VERIFIED | Unchanged. Wired into `TestMain`. |
| `internal/retrievaleval/vector.go` | `cosineDistance`, `differMinCosineDistance` | ✓ VERIFIED | Unchanged. Wired into the differ gate. |
| `internal/server/tools.go` | `StoreAndEmbedderFromEnvNoEnsure` returns `*config.Config` as 5th value | ✓ VERIFIED | Signature unchanged (`tools.go:290`). All callers destructure 6 values: `reindex.go:55`, `retrieval_eval_test.go:164,506`, `tools_test.go:5157,5599`. |
| `internal/store/rerank.go` | `CandidateK`, `VectorOrder`, `RerankHits`, `rankCandidates` | ✓ VERIFIED | All four are unchanged. Phase 4 only appended hook helpers. |
| `internal/retrievaleval/comparison_rankers.go` | `lexicalRerank`, `cosineBlendRerank`, `overlapGateRerank` | ✓ VERIFIED | Unchanged. |
| `internal/retrievaleval/paraphrase_fixture.go` | `paraphraseSeeds`, `paraphraseTopics`, `paraphraseCase` | ✓ VERIFIED | Unchanged. |
| `internal/retrievaleval/rankers.go` | `evalRankers`, `decideRanking`, `formatVariantTable` | ✓ VERIFIED | Present. The D-05 rule is intact for non-opt-in rows; see the change table. |
| `01-EVAL-BASELINE.log` / `01-EVAL-SHIPPED.log` | verbatim live runs | ✓ VERIFIED | Unchanged. |
| `01-RANKING-DECISION.md` / `01-ISSUE-605-EVIDENCE.md` | D-05 decision, #605 evidence | ✓ VERIFIED | Unchanged. |

### Key Link Verification

`gsd-tools verify.key-links` passes for every plan that declares links: 01-01 5/5, 01-02 4/4, 01-03 2/2, 01-04 5/5, 01-06 2/2. 01-05 declares none. `go test ./internal/keylinks/` PASS, including `TestActiveMilestoneKeyLinksSatisfiable`.

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `store.go` (`SearchReranked`) | `rerank.go` (`rankCandidates`) | the only rank step | ✓ WIRED | `store.go:1360` returns `RankWithHook(..., opts.RankHook)`. A nil hook takes the `rankCandidates(query, hits, k)` branch. |
| `rerank.go` (`rankCandidates`) | `rerank.go` (`RerankHits`) | D-05 lexical winner | ✓ WIRED | The body is still `return RerankHits(query, hits, k)`. |
| `retrieval_eval_test.go` (`TestMain`) | `gate.go` (`retrievalEvalEnabled`) | package-wide gate | ✓ WIRED | Unchanged. |
| `retrieval_eval_test.go` | `rankers.go` (`evalRankers`, `decideRanking`) | roster + D-05 in code | ✓ WIRED | `roster := evalRankers(jevHook)` and `d := decideRanking(rows)` (`:423`). |

### Behavioral Spot-Checks

Every test ran live on HEAD with `env -u ENGRAM_RETRIEVAL_EVAL` and `-count=1`.

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Whole retrievaleval package (EVAL-01/02, RANK-01/02 unit layer) | `go test ./internal/retrievaleval/` | ok. `TestCosineDistance`, `TestEvalGate`, `TestParaphraseCorpusIntegrity`, `TestDecideRanking`, `TestEvalRankersRoster`, `TestEvalRankersJevEnabled`, and `TestFormatVariantTable` PASS. The live-gated `TestRetrievalEval*` tests SKIP as intended with the gate unset. | ✓ PASS |
| Key-link sweep | `go test ./internal/keylinks/` | ok | ✓ PASS |
| Shipped seam = D-05 winner; nil hook = Phase 1 behavior | `go test ./internal/store/ -run 'TestRankCandidatesIsTheD05Winner\|TestRankWithHook*\|TestCandidateK\|TestVectorOrder*\|TestSearchRerankedRejectsZeroK\|TestRerankHits*'` | 19/19 PASS, including `TestRankWithHookNilIsRankCandidates` and `TestRankWithHookFallback` | ✓ PASS |
| MCP/Connect rerank parity (default + Jev) | `go test ./internal/server/ -run 'TestRerankParityMCPAndConnect\|TestStoreAndEmbedderFromEnvNoEnsureLoadsConfigOnce'` | 10/10 parity subtests PASS, plus the single-load contract | ✓ PASS |
| Compile / vet / fmt | `go build ./...`; `go vet ./internal/retrievaleval/ ./internal/store/ ./internal/server/`; `gofmt -l` on the same packages + `reindex.go` | clean | ✓ PASS |

### Probe Execution

This phase declares no `scripts/*/tests/probe-*.sh` probes, so this step was skipped. The live evidence is the gated `task eval:retrieval` run captured in `01-EVAL-*.log`, and those logs are unchanged.

### Requirements Coverage

| Requirement | Source Plan | Status | Evidence |
|-------------|------------|--------|----------|
| EVAL-01 | 01-01 | ✓ SATISFIED | Truth 1 |
| EVAL-02 | 01-01 | ✓ SATISFIED | Truth 2 |
| RANK-01 | 01-02..01-05 | ✓ SATISFIED | Truth 3 |
| RANK-02 | 01-02, 01-04..01-06 | ✓ SATISFIED | Truth 4 |

No orphaned requirements.

### Anti-Patterns Found

None in this phase's changed files. I scanned every covered Go file and `tools.md` for `TBD|FIXME|XXX|TODO|HACK` and found zero matches. `gofmt -l` is clean.

Info, not a Phase 1 finding: `go vet ./cmd/engram/` reports a duplicate json tag in `cmd/engram/operator_view_test.go:441`. That duplicate is deliberate, carries `//nolint:govet`, and lives in an operator-view file outside Phase 1's scope.

### Prohibitions (must_haves.prohibitions)

| # | Statement | Tier | Status |
|---|-----------|------|--------|
| 1 | MUST NOT let the blind query author see any seed-record text (01-03) | judgment | Human-approved 2026-09-23 (carried forward; the covered artifacts did not change) |
| 2 | MUST NOT commit verbatim spine content/secrets/engram identifiers in the corpus (01-03) | test | ✓ VERIFIED: `TestParaphraseCorpusIntegrity/denylist` PASS on HEAD |
| 3 | MUST NOT offer/accept/record a shipped ranking other than `decideRanking`'s winner (01-05) | judgment | Human-approved 2026-09-23 (carried forward). Phase 4's opt-in Jev row is excluded from `decideRanking`, so it cannot become a D-05 winner. |
| 4 | MUST NOT post to GitHub unless `01-RANKING-DECISION.md` records authorization (01-06) | judgment | Human-approved 2026-09-23 (carried forward) |
| 5 | MUST NOT ship a ranking other than the approved winner / re-tune to pass gates (01-06) | test | ✓ VERIFIED: `TestRankCandidatesIsTheD05Winner` and `TestRankWithHookNilIsRankCandidates` PASS. The default rank path is still the lexical winner, and Jev ships only as an explicit operator opt-in. |

### Gaps Summary

No gaps. Phase 4 layered an opt-in Jev rank hook onto the Phase 1 seam. With no hook, the default behaves exactly as Phase 1 shipped (`RankWithHook(nil)` returns `rankCandidates`, which calls `RerankHits`). The eval's D-05 decision and D-10 gates still measure that default. The Jev row competes nowhere in D-05, and it gives SC3 its "when enabled" Jev measurement. All four ROADMAP success criteria hold on HEAD.

## Prior Verification History

- 2026-09-23T12:06:02Z: initial verification, `human_needed` (3 judgment-tier prohibitions).
- 2026-09-23T16:50:00Z: re-verified after Phase 2's additive `deps.decider` edit (`344ba149`) to `tools.go`/`tools_test.go`. That run confirmed no regression, dropped `.planning/REQUIREMENTS.md` from the fingerprint, and set status `passed` after the judgment-tier items were approved.
- 2026-09-24T17:37:02Z: this run, covering the Phase 4 changes above.

---

_Verified: 2026-09-24T17:37:02Z_
_Verifier: Claude (gsd-verifier)_

## Human Validation

**2026-09-23: approved by user ("All good — continue")** in the autonomous run. The user confirmed all three judgment-tier items: blind-query authorship integrity, D-05 checkpoint non-reselection, and consent for the #605 GitHub post.

_(This is carried forward verbatim as a historical record of a human review event. The later-phase changes checked here add no new judgment-tier prohibition, so no new sign-off is required.)_
