---
phase: 01-eval-foundation-lexical-reranker-fix
verified: 2026-09-23T16:50:00Z
status: passed
score: 4/4 must-haves verified
covered_files: [".planning/phases/01-eval-foundation-lexical-reranker-fix/01-01-PLAN.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-01-SUMMARY.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-02-PLAN.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-02-SUMMARY.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-03-PLAN.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-03-SUMMARY.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-04-PLAN.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-04-SUMMARY.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-05-PLAN.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-05-SUMMARY.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-06-PLAN.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-06-SUMMARY.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-CONTEXT.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-EVAL-BASELINE.log", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-EVAL-SHIPPED.log", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-ISSUE-605-EVIDENCE.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-RANKING-DECISION.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-REVIEW.md", "cmd/engram/reindex.go", "docs-site/src/content/docs/reference/tools.md", "internal/retrievaleval/comparison_rankers.go", "internal/retrievaleval/comparison_rankers_test.go", "internal/retrievaleval/doc.go", "internal/retrievaleval/fixtures.go", "internal/retrievaleval/gate.go", "internal/retrievaleval/gate_test.go", "internal/retrievaleval/paraphrase_fixture.go", "internal/retrievaleval/paraphrase_fixture_test.go", "internal/retrievaleval/rankers.go", "internal/retrievaleval/rankers_test.go", "internal/retrievaleval/retrieval_eval_test.go", "internal/retrievaleval/vector.go", "internal/retrievaleval/vector_test.go", "internal/server/connectapi_test.go", "internal/server/tools.go", "internal/server/tools_test.go", "internal/store/rerank.go", "internal/store/rerank_test.go", "internal/store/store.go"]
covered_digest: "v1:sha256:b01750e792079ae99394ca2fb3867f80f040b504889a9b905a63725577a583d0"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 4/4
  gaps_closed: []
  gaps_remaining: []
  regressions: []
---

# Phase 1: Eval Foundation & Lexical Reranker Fix Verification Report

**Phase Goal:** The retrieval eval measures reranking correctly, including a paraphrase case, so later reranking work (Phase 4) has trustworthy numbers to gate on.
**Verified:** 2026-09-23T16:50:00Z
**Status:** passed
**Re-verification:** Yes — triggered by covered_digest staleness (Phase 2 edited `internal/server/tools.go`/`tools_test.go`; `phase.complete` rewrote `.planning/REQUIREMENTS.md`). No prior `gaps:` existed — this run re-executes full goal-backward verification against the current tree and carries the already-approved judgment-tier human validation forward per instruction.

## Why This Re-Verification Ran

The 2026-09-23T12:06:02Z VERIFICATION.md was stale: its `covered_digest` was computed over a file set that included `.planning/REQUIREMENTS.md` (subsequently rewritten by `phase.complete` bookkeeping, unrelated to Phase 1's actual deliverable) and predated Phase 2's additive edits to `internal/server/tools.go`/`tools_test.go` (commit `344ba149`, "build deps.decider at startup only when a provider is set", DEC-01/D-01). This run re-verifies the phase goal against the current tree and drops `.planning/REQUIREMENTS.md` from the fingerprint (per instruction: only include bookkeeping files the verification genuinely depends on for content — this one does not; requirement text/traceability was independently re-checked below instead).

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | The embedding differ gate compares vectors with a cosine epsilon instead of `reflect.DeepEqual` on `[]float32` (ROADMAP SC1 / EVAL-01) | ✓ VERIFIED | `internal/retrievaleval/vector.go` unchanged since Phase 1 (not touched by any later commit). `TestCosineDistance` (12 subtests) re-run live and PASS on the current tree. |
| 2 | The retrieval-eval skip guard reads the resolved koanf config rather than raw `os.Getenv` (ROADMAP SC2 / EVAL-02) | ✓ VERIFIED | `internal/retrievaleval/gate.go` unchanged since Phase 1. `TestEvalGate` (12 subtests) re-run live and PASS. |
| 3 | `task eval:retrieval` reports recall@k and MRR for vector-only, lexical-reranked, and (when enabled) Jev-reranked ordering, including a paraphrase case written independently of #261's targets (ROADMAP SC3 / RANK-01) | ✓ VERIFIED | `internal/retrievaleval/paraphrase_fixture.go`, `rankers.go`, `comparison_rankers.go` unchanged since Phase 1. `TestParaphraseCorpusIntegrity` (7 subtests) and `TestDecideRanking` (14 subtests) re-run live and PASS. `01-EVAL-SHIPPED.log` (committed, unchanged) still shows the full variant table with `EXIT_STATUS:0`. |
| 4 | On that paraphrase case, the shipped ranking does not regress vs vector-only and keeps #261's target at rank 1 (ROADMAP SC4 / RANK-02) | ✓ VERIFIED | `internal/store/store.go`'s `SearchReranked` still calls `rankCandidates(query, hits, int(k))` as its sole rank step (`store.go:1342`, unchanged text/line). `internal/store/rerank.go` unchanged. `TestRankCandidatesIsTheD05Winner` (3 subtests) re-run live and PASS, pinning the seam to the D-05 winner (lexical). |

**Score:** 4/4 truths verified (0 present, behavior-unverified)

### Regression Check: Phase 2 Edits to `internal/server/tools.go` / `tools_test.go`

Commit `344ba149` ("feat(server): build deps.decider at startup only when a provider is set (DEC-01, D-01)") is the only commit touching `internal/server/tools.go` or `internal/server/tools_test.go` since Phase 1 completed (commit `d1aef688`). `git diff d1aef688..HEAD --stat` for these two files shows 81 insertions, 0 deletions — purely additive:

- `tools.go`: adds a new `decider decide.Decider` field to the `deps` struct and a new `deciderFromConfig`/`logDeciderEnabled` call in `buildDepsFromEnv`. `StoreAndEmbedderFromEnvNoEnsure` (lines 283-294) is byte-for-byte unchanged — still returns `(*store.Store, uint64, *embed.Client, string, *config.Config, error)`, still calls `loadAndValidate` exactly once, still returns the same `cfg` pointer that produced the embedder.
- `tools_test.go`: adds three new tests (`TestBuildDepsFromEnvRejectsUnknownProvider`, `TestBuildDepsFromEnvConstructsDecider`) and one new `t.Setenv` isolation line inside the pre-existing `TestBuildDepsFromEnvLoadsConfigOnce`. No existing assertion was modified or removed.

All three call sites of `StoreAndEmbedderFromEnvNoEnsure()` still destructure the 5 non-error return values correctly:
- `cmd/engram/reindex.go:55` — `st, dim, em, identity, _, err := ...`
- `internal/retrievaleval/retrieval_eval_test.go:164` — `_, dim, em, _, _, err := ...`
- `internal/retrievaleval/retrieval_eval_test.go:456` — `_, dim, em, _, cfg, err := ...`

`TestRerankParityMCPAndConnect` (`internal/server/connectapi_test.go`, untouched by any commit since Phase 1) re-run live: 5/5 subtests PASS. `TestStoreAndEmbedderFromEnvNoEnsureLoadsConfigOnce` re-run live: PASS.

**Conclusion: no regression.** Phase 2's `deps.decider` addition is orthogonal to the Phase 1 rerank/eval seam — it adds a new unused-until-Phase-4 field and does not alter `StoreAndEmbedderFromEnvNoEnsure`'s signature, the `SearchReranked` → `rankCandidates` → `RerankHits` call chain, or any retrieval-eval code path.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/retrievaleval/gate.go` | `resolveEvalGate`/`retrievalEvalEnabled`, koanf-based | ✓ VERIFIED | Unchanged since Phase 1; wired into `TestMain` in `retrieval_eval_test.go`. |
| `internal/retrievaleval/vector.go` | `cosineDistance`, `differMinCosineDistance` | ✓ VERIFIED | Unchanged since Phase 1; wired into the differ gate. |
| `internal/server/tools.go` | `StoreAndEmbedderFromEnvNoEnsure` returns `*config.Config` as 5th value | ✓ VERIFIED | Confirmed unchanged; `go build ./...` and `go vet` clean on the current tree; all 3 call sites correct. |
| `internal/store/rerank.go` | `CandidateK`, `VectorOrder`, `RerankHits`, `rankCandidates` | ✓ VERIFIED | Unchanged since Phase 1; `rankCandidates` is still the single seam `SearchReranked` calls (`store.go:1342`). |
| `internal/retrievaleval/comparison_rankers.go` | `lexicalRerank`, `cosineBlendRerank`, `overlapGateRerank` | ✓ VERIFIED | Unchanged since Phase 1. |
| `internal/retrievaleval/paraphrase_fixture.go` | `paraphraseSeeds`, `paraphraseTopics`, `paraphraseCase` | ✓ VERIFIED | Unchanged since Phase 1; `TestParaphraseCorpusIntegrity` passes live. |
| `internal/retrievaleval/rankers.go` | `evalRankers`, `decideRanking`, `formatVariantTable` | ✓ VERIFIED | Unchanged since Phase 1; `TestDecideRanking` passes live. |
| `.planning/phases/.../01-EVAL-BASELINE.log` | verbatim pre-change live run | ✓ VERIFIED | Unchanged; still contains D-05/D-10 gate lines, `EXIT_STATUS:0`. |
| `.planning/phases/.../01-EVAL-SHIPPED.log` | verbatim post-change live run | ✓ VERIFIED | Unchanged; `EXIT_STATUS:0`, both `D-10 gate PASS:` lines. |
| `.planning/phases/.../01-RANKING-DECISION.md` | recorded D-05 decision + approval | ✓ VERIFIED | Unchanged; `Approved winner: lexical`, authorization recorded. |
| `.planning/phases/.../01-ISSUE-605-EVIDENCE.md` | posted #605 evidence | ✓ VERIFIED | Unchanged; comment `5788878938` still live on GitHub (not re-fetched this run — no code-relevant change since prior confirmation; see Human Validation below). |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `internal/store/store.go` (`SearchReranked`) | `internal/store/rerank.go` (`rankCandidates`) | final rank step before truncation | ✓ WIRED | `store.go:1342`: `return rankCandidates(query, hits, int(k)), nil` — line unchanged since Phase 1. |
| `internal/store/rerank.go` (`rankCandidates`) | `internal/store/rerank.go` (`RerankHits`) | D-05 lexical winner delegation | ✓ WIRED | `rankCandidates` body unchanged: `return RerankHits(query, hits, k)`. |
| `internal/retrievaleval/retrieval_eval_test.go` (`TestMain`) | `internal/retrievaleval/gate.go` (`retrievalEvalEnabled`) | package-wide gate | ✓ WIRED | Unchanged since Phase 1. |
| `internal/server/tools.go` (`buildDepsFromEnv`) | `internal/server/tools.go` (`StoreAndEmbedderFromEnvNoEnsure`) callers | 5-value return contract | ✓ WIRED | All 3 call sites (`reindex.go:55`, `retrieval_eval_test.go:164,456`) correctly destructure the unchanged 6-value signature; `go build ./...` confirms no compile break introduced by Phase 2's additive `deps.decider` field. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Cosine-epsilon differ gate (EVAL-01) | `go test ./internal/retrievaleval/ -run TestCosineDistance -v` | 12/12 subtests PASS | ✓ PASS |
| koanf-resolved eval gate (EVAL-02) | `go test ./internal/retrievaleval/ -run TestEvalGate -v` | 12/12 subtests PASS | ✓ PASS |
| Paraphrase corpus integrity (RANK-01) | `go test ./internal/retrievaleval/ -run TestParaphraseCorpusIntegrity -v` | 7/7 subtests PASS | ✓ PASS |
| D-05 decision rule (RANK-01/02) | `go test ./internal/retrievaleval/ -run TestDecideRanking -v` | 14/14 subtests PASS | ✓ PASS |
| Shipped rank-step seam pinned to lexical winner (RANK-02) | `go test ./internal/store/ -run TestRankCandidatesIsTheD05Winner -v` | 3/3 subtests PASS | ✓ PASS |
| `CandidateK`/`VectorOrder` (RANK-01) | `go test ./internal/store/ -run 'TestCandidateK|TestVectorOrder|TestSearchRerankedRejectsZeroK' -v` | all PASS | ✓ PASS |
| MCP/Connect rerank parity unaffected by Phase 2 (RANK-02 regression check) | `go test ./internal/server/ -run TestRerankParityMCPAndConnect -v` | 5/5 subtests PASS | ✓ PASS |
| `StoreAndEmbedderFromEnvNoEnsure` single-load contract unaffected by Phase 2 | `go test ./internal/server/ -run TestStoreAndEmbedderFromEnvNoEnsureLoadsConfigOnce -v` | PASS | ✓ PASS |
| `go build ./...` (whole-module compile) | `go build ./...` | clean | ✓ PASS |
| `go vet` on touched packages | `go vet ./internal/server/... ./internal/store/... ./internal/retrievaleval/...` | clean, no output | ✓ PASS |
| `gofmt` on Phase-1-relevant + Phase-2-touched files | `gofmt -l internal/server/tools.go internal/server/tools_test.go internal/store/rerank.go internal/store/store.go internal/retrievaleval/*.go cmd/engram/reindex.go` | no output (clean) | ✓ PASS |

### Probe Execution

No `scripts/*/tests/probe-*.sh` probes declared or found for this phase — SKIPPED (unchanged from initial verification; this phase's live evidence is the `task eval:retrieval` run itself, captured in `01-EVAL-BASELINE.log`/`01-EVAL-SHIPPED.log` and independently re-verified above via targeted `go test` runs).

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| EVAL-01 | 01-01 | Differ gate uses cosine epsilon, not `reflect.DeepEqual` on `[]float32` | ✓ SATISFIED | `internal/retrievaleval/vector.go`, `TestCosineDistance` passes live on current tree. |
| EVAL-02 | 01-01 | Retrieval-eval skip guard reads resolved koanf config, not raw `os.Getenv` | ✓ SATISFIED | `internal/retrievaleval/gate.go`, `TestEvalGate` passes live on current tree. |
| RANK-01 | 01-02, 01-03, 01-04, 01-05 | Retrieval eval includes an independently written paraphrase case, reporting recall@k/MRR for vector-only/lexical/Jev | ✓ SATISFIED | `01-EVAL-SHIPPED.log` table; `paraphrase_fixture.go`; `TestParaphraseCorpusIntegrity` passes live. |
| RANK-02 | 01-02, 01-04, 01-05, 01-06 | Default ranking does not regress the paraphrase case vs vector-only and keeps #261's target at rank 1 | ✓ SATISFIED | `01-EVAL-SHIPPED.log`'s two `D-10 gate PASS:` lines; `rankCandidates` seam wired to `RerankHits`, confirmed unregressed by Phase 2; `TestRankCandidatesIsTheD05Winner` and `TestRerankParityMCPAndConnect` pass live. |

`.planning/REQUIREMENTS.md`'s current Traceability table (re-checked this run) still maps only EVAL-01, EVAL-02, RANK-01, RANK-02 to Phase 1, all four marked `[x]` Complete. No orphaned requirements for Phase 1.

### Anti-Patterns Found

None. Re-scanned `internal/server/tools.go`, `internal/server/tools_test.go` (Phase 2's edits), plus `internal/store/rerank.go`, `internal/store/store.go` for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` — zero matches. `gofmt -l` clean on all Phase-1-relevant and Phase-2-touched files.

### Prohibitions (must_haves.prohibitions)

Unchanged from the initial verification — no code or process affecting these prohibitions changed since 2026-09-23T12:06:02Z:

| # | Statement | Category | Verification tier | Status |
|---|-----------|----------|--------------------|--------|
| 1 | MUST NOT let the blind query author see any seed-record text (01-03) | integrity-of-evidence | judgment | Resolved per artifact evidence — human-approved (carried forward, see Human Validation) |
| 2 | MUST NOT commit verbatim spine content/secrets/engram identifiers in the corpus (01-03) | privacy | test | ✓ VERIFIED — `TestParaphraseCorpusIntegrity/denylist` re-ran live and PASSED |
| 3 | MUST NOT offer/accept/record a shipped ranking other than `decideRanking`'s winner (01-05) | integrity-of-evidence | judgment | Resolved per artifact evidence — human-approved (carried forward, see Human Validation) |
| 4 | MUST NOT post to GitHub unless `01-RANKING-DECISION.md` records authorization (01-06) | consent | judgment | Resolved per artifact evidence — human-approved (carried forward, see Human Validation) |
| 5 | MUST NOT ship a ranking other than the approved winner / re-tune to pass gates (01-06) | integrity-of-evidence | test | ✓ VERIFIED — `TestRankCandidatesIsTheD05Winner` re-ran live and PASSED |

### Gaps Summary

No gaps. All four ROADMAP success criteria (EVAL-01, EVAL-02, RANK-01, RANK-02) remain backed by unmodified, live-tested code on the current tree. The only relevant change since the prior verification — Phase 2's `344ba149` addition of `deps.decider` to `internal/server/tools.go`/`tools_test.go` — is confirmed purely additive (81 insertions, 0 deletions), does not touch `StoreAndEmbedderFromEnvNoEnsure`'s signature or body, and does not affect the `SearchReranked` → `rankCandidates` → `RerankHits` seam. `TestRerankParityMCPAndConnect` and `TestStoreAndEmbedderFromEnvNoEnsureLoadsConfigOnce` were re-run live against the current tree and both PASS, confirming no regression. `go build ./...` and `go vet` are clean. The three judgment-tier prohibitions were explicitly approved by the user on 2026-09-23 (see Human Validation below, carried forward verbatim per instruction — those are historical facts about a human review event that do not change with subsequent code edits) — with no material regression found, this run is scored `status: passed`.

---

_Verified: 2026-09-23T16:50:00Z_
_Verifier: Claude (gsd-verifier)_

## Human Validation

**2026-09-23 — approved by user ("All good — continue")** in the autonomous run. All three judgment-tier items were confirmed: blind-query authorship integrity, D-05 checkpoint non-reselection, and consent for the #605 GitHub post.

_(Carried forward verbatim from the 2026-09-23T12:06:02Z verification per re-verification instruction — this is a historical record of a human review event, not a re-derived finding. No new judgment-tier prohibition or condition was introduced by the code changes checked in this re-verification, so no new human sign-off is required.)_
