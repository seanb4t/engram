---
phase: 01-eval-foundation-lexical-reranker-fix
verified: 2026-09-23T12:06:02Z
status: passed
score: 4/4 must-haves verified
covered_files: [".planning/REQUIREMENTS.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-01-PLAN.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-01-SUMMARY.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-02-PLAN.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-02-SUMMARY.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-03-PLAN.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-03-SUMMARY.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-04-PLAN.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-04-SUMMARY.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-05-PLAN.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-05-SUMMARY.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-06-PLAN.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-06-SUMMARY.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-CONTEXT.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-EVAL-BASELINE.log", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-EVAL-SHIPPED.log", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-ISSUE-605-EVIDENCE.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-RANKING-DECISION.md", ".planning/phases/01-eval-foundation-lexical-reranker-fix/01-REVIEW.md", "cmd/engram/reindex.go", "docs-site/src/content/docs/reference/tools.md", "internal/retrievaleval/comparison_rankers.go", "internal/retrievaleval/comparison_rankers_test.go", "internal/retrievaleval/doc.go", "internal/retrievaleval/fixtures.go", "internal/retrievaleval/gate.go", "internal/retrievaleval/gate_test.go", "internal/retrievaleval/paraphrase_fixture.go", "internal/retrievaleval/paraphrase_fixture_test.go", "internal/retrievaleval/rankers.go", "internal/retrievaleval/rankers_test.go", "internal/retrievaleval/retrieval_eval_test.go", "internal/retrievaleval/vector.go", "internal/retrievaleval/vector_test.go", "internal/server/connectapi_test.go", "internal/server/tools.go", "internal/server/tools_test.go", "internal/store/rerank.go", "internal/store/rerank_test.go", "internal/store/store.go"]
covered_digest: "v1:sha256:80138e5d08d1f80be96a957f8b060b116017b2efc6125f42726c6fcd43765828"
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "Confirm the blind-query authorship procedure for the paraphrase corpus (01-03 prohibition, verification: judgment) — that the query author (a fresh, tool-less subagent) never saw any paraphraseSeeds record text before writing the 24 queries in 01-BLIND-QUERIES.md."
    expected: "01-BLIND-QUERY-PROMPT.md contains only topic labels (no seed text, keys, or tags) below its `---8<---` marker, and 01-BLIND-QUERIES.md's provenance header records 0 tool calls and no repository context for the author. Both files were inspected and are consistent with this claim, but the verifier cannot observe the actual dispatched subagent's context window, only the artifacts it produced."
    why_human: "Integrity-of-evidence prohibition (category: integrity-of-evidence, verification: judgment, not test) — no wired mechanical check enforces this by design (spec-less fallback per the plan's own reason field). This determination rests on trusting the recorded procedure and provenance header, not on a runnable assertion."
  - test: "Confirm the D-05 decision checkpoint (01-05 prohibition, verification: judgment) never re-selected a ranking by eye — that 'Approved winner: lexical' in 01-RANKING-DECISION.md is the same token decideRanking mechanically emitted, and the human checkpoint only confirmed or halted the measurement."
    expected: "01-RANKING-DECISION.md's 'Rule verdict' section (winner=lexical, via decideRanking) and its 'Approval' section (Approved winner: lexical) agree, and the checkpoint reply was 'approve-and-post' with three caveats recorded, not a substitution of a different variant."
    why_human: "Integrity-of-evidence prohibition, verification: judgment — the verifier confirmed the two tokens match textually in the committed artifact, but cannot independently confirm the human checkpoint reply was not itself influenced by seeing the numbers before the rule ran (the rule's mechanical application is unit-tested via TestDecideRanking, but the checkpoint's own good-faith conduct is not machine-checkable)."
  - test: "Confirm the #605 evidence-comment posting (01-06 prohibition, verification: judgment) was authorized before it was posted."
    expected: "01-RANKING-DECISION.md records 'Evidence comment authorized: yes' dated 2026-09-23, and the GitHub comment (issue #605, comment id 5788878938) was created afterward (2026-09-23T04:10:10Z)."
    why_human: "Consent prohibition, verification: judgment — the verifier confirmed the comment exists live on GitHub (fetched via gh api) and that the authorization record predates a plausible posting time, but cannot verify ordering/causality beyond the recorded dates and the SUMMARY's own narrative."
---

# Phase 1: Eval Foundation & Lexical Reranker Fix Verification Report

**Phase Goal:** The retrieval eval measures reranking correctly, including a paraphrase case, so later reranking work (Phase 4) has trustworthy numbers to gate on.
**Verified:** 2026-09-23T12:06:02Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | The embedding differ gate compares vectors with a cosine epsilon instead of `reflect.DeepEqual` on `[]float32` (ROADMAP SC1 / EVAL-01) | ✓ VERIFIED | `internal/retrievaleval/vector.go` defines `differMinCosineDistance = 1e-3` and `cosineDistance(a, b []float32)`, erroring hard on NaN/Inf/zero-norm/length-mismatch/empty input. `TestCosineDistance` (12 subtests including degenerate-input errors and near-identical-jitter-fails-the-gate) run live and PASS. |
| 2 | The retrieval-eval skip guard reads the resolved koanf config rather than raw `os.Getenv` (ROADMAP SC2 / EVAL-02) | ✓ VERIFIED | `internal/retrievaleval/gate.go`'s `resolveEvalGate` builds a package-local koanf instance (`confmap` default layer + `env.Provider` with `config.Prefix`), and `retrievalEvalEnabled()` wraps it over `os.Environ`. `retrieval_eval_test.go`'s `TestMain` calls `retrievalEvalEnabled()` (not `os.Getenv`) to gate the whole package. `TestEvalGate` (12 subtests: unset/empty/true-spellings/malformed/look-alike-prefix/unrelated-malformed-vars) run live and PASS. `ENGRAM_RETRIEVAL_EVAL` is confirmed absent from `internal/config`'s registry (deliberate, per D-15 in 01-CONTEXT.md). |
| 3 | `task eval:retrieval` reports recall@k and MRR for vector-only, lexical-reranked, and (when enabled) Jev-reranked ordering, including a paraphrase case written independently of #261's targets (ROADMAP SC3 / RANK-01) | ✓ VERIFIED | `Taskfile.yaml` defines `eval:retrieval`. The committed `01-EVAL-SHIPPED.log` (a live run, `EXIT_STATUS:0`) contains a `| variant |` Markdown table with `vector-only`, `lexical`, four `overlap-gate-*`/`cosine-blend-*` rows, a `jev` row reading `Jev: disabled`, and a `shipped (SearchReranked)` row, each with `#261 ranks`, `recall@8` and `MRR` columns. `TestRetrievalEval/paraphrase-blind-multidomain` PASSED. The paraphrase corpus (`paraphrase_fixture.go`, 292 lines) and its 24 blind queries (`01-BLIND-QUERIES.md`) were authored via a documented blind-authorship procedure independent of `gh261Case`; `TestParaphraseCorpusIntegrity` (7 subtests incl. `denylist`, `label-leak`, `sticky-neighbours`) run live and PASS. |
| 4 | On that paraphrase case, the shipped ranking does not regress vs vector-only and keeps #261's target at rank 1 (ROADMAP SC4 / RANK-02) | ✓ VERIFIED | `01-EVAL-SHIPPED.log` (post-change live re-run) shows both hard gates: `D-10 gate PASS: #261 target at rank 1 for both queries under the shipped ranking` and `D-10 gate PASS: shipped paraphrase MRR 0.817 >= vector-only 0.579`. `internal/store/store.go`'s `SearchReranked` calls `rankCandidates(query, hits, int(k))` as its sole rank step (`internal/store/rerank.go`), and `TestRankCandidatesIsTheD05Winner` (3 subtests) run live and PASS, pinning the seam to the approved D-05 winner (lexical). |

**Score:** 4/4 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/retrievaleval/gate.go` | `resolveEvalGate`/`retrievalEvalEnabled`, koanf-based | ✓ VERIFIED | Exists, substantive (not a stub), wired into `TestMain` in `retrieval_eval_test.go`. |
| `internal/retrievaleval/vector.go` | `cosineDistance`, `differMinCosineDistance` | ✓ VERIFIED | Exists, substantive, wired into the differ gate (`TestRetrievalEval_AsymmetryDiffer` uses `cosineDistance` per grep in retrieval_eval_test.go). |
| `internal/server/tools.go` | `StoreAndEmbedderFromEnvNoEnsure` returns `*config.Config` as 5th value | ✓ VERIFIED | Confirmed by `cmd/engram/reindex.go` and `retrieval_eval_test.go` call-site adoption (per 01-REVIEW.md's cross-check and passing build/vet). |
| `internal/store/rerank.go` | `CandidateK`, `VectorOrder`, `RerankHits`, `rankCandidates` | ✓ VERIFIED | All four present; `rankCandidates` is the single seam `SearchReranked` calls (`store.go:1342`). |
| `internal/retrievaleval/comparison_rankers.go` | `lexicalRerank`, `cosineBlendRerank`, `overlapGateRerank` | ✓ VERIFIED | Exists; wired into `internal/retrievaleval/rankers.go`'s named-ranker roster (confirmed in the committed eval table's 4 overlap-gate/cosine-blend rows). |
| `internal/retrievaleval/paraphrase_fixture.go` | `paraphraseSeeds`, `paraphraseTopics`, `paraphraseCase` | ✓ VERIFIED | 292 lines (exceeds 150-line minimum); `TestParaphraseCorpusIntegrity` passes live. |
| `internal/retrievaleval/rankers.go` | `evalRankers`, `decideRanking`, `formatVariantTable` | ✓ VERIFIED | 391 lines; `TestDecideRanking` (14 subtests) passes live. |
| `.planning/phases/.../01-EVAL-BASELINE.log` | verbatim pre-change live run | ✓ VERIFIED | Contains `D-05 decision: winner=`, both D-10 gate lines, `EXIT_STATUS:0`, no `harness:` line, no `AsymmetryDiffer FAIL`. |
| `.planning/phases/.../01-EVAL-SHIPPED.log` | verbatim post-change live run | ✓ VERIFIED | Same shape, `EXIT_STATUS:0`, both `D-10 gate PASS:` lines. |
| `.planning/phases/.../01-RANKING-DECISION.md` | recorded D-05 decision + approval | ✓ VERIFIED | Contains table, rule verdict, both gate statuses, embedder identity (model+dim, no credential), `Approved winner: lexical`, `Evidence comment authorized: yes`. |
| `.planning/phases/.../01-ISSUE-605-EVIDENCE.md` | posted #605 evidence | ✓ VERIFIED | Content matches `01-RANKING-DECISION.md`'s tables; comment confirmed live on GitHub (see Key Link Verification). |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `internal/store/store.go` (`SearchReranked`) | `internal/store/rerank.go` (`rankCandidates`) | final rank step before truncation | ✓ WIRED | `store.go:1342`: `return rankCandidates(query, hits, int(k)), nil`. |
| `internal/store/rerank.go` (`rankCandidates`) | `internal/store/rerank.go` (`RerankHits`) | D-05 lexical winner delegation | ✓ WIRED | `rankCandidates` body is `return RerankHits(query, hits, k)`. |
| `internal/retrievaleval/retrieval_eval_test.go` (`TestMain`) | `internal/retrievaleval/gate.go` (`retrievalEvalEnabled`) | package-wide gate | ✓ WIRED | `TestMain` calls `retrievalEvalEnabled()` at line 539 per grep. |
| `.planning/phases/.../01-ISSUE-605-EVIDENCE.md` | GitHub issue #605 | `gh issue comment` per authorized checkpoint | ✓ WIRED | Comment id `5788878938` confirmed live via `gh api repos/seanb4t/engram/issues/comments/5788878938` (created `2026-09-23T04:10:10Z`, body length 9061 matching the evidence doc). |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Cosine-epsilon differ gate (EVAL-01) | `go test ./internal/retrievaleval/ -run TestCosineDistance -v` | 12/12 subtests PASS | ✓ PASS |
| koanf-resolved eval gate (EVAL-02) | `go test ./internal/retrievaleval/ -run TestEvalGate -v` | 12/12 subtests PASS | ✓ PASS |
| Paraphrase corpus integrity (RANK-01) | `go test ./internal/retrievaleval/ -run TestParaphraseCorpusIntegrity -v` | 7/7 subtests PASS | ✓ PASS |
| D-05 decision rule (RANK-01/02) | `go test ./internal/retrievaleval/ -run TestDecideRanking -v` | 14/14 subtests PASS | ✓ PASS |
| Shipped rank-step seam pinned to lexical winner (RANK-02) | `go test ./internal/store/ -run TestRankCandidatesIsTheD05Winner -v` | 3/3 subtests PASS | ✓ PASS |
| `CandidateK`/`VectorOrder` (RANK-01) | `go test ./internal/store/ -run 'TestCandidateK|TestVectorOrder|TestSearchRerankedRejectsZeroK' -v` | all PASS | ✓ PASS |
| `go vet` on touched packages | `go vet ./internal/retrievaleval/... ./internal/store/...` | clean, no output | ✓ PASS |
| `gofmt` on the file 01-REVIEW.md flagged | `gofmt -l internal/retrievaleval/rankers_test.go` | no output (clean) | ✓ PASS |

### Probe Execution

No `scripts/*/tests/probe-*.sh` probes declared or found for this phase — SKIPPED (no probe files apply; this phase's live evidence is the `task eval:retrieval` run itself, captured in `01-EVAL-BASELINE.log`/`01-EVAL-SHIPPED.log` and independently re-verified above via targeted `go test` runs).

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| EVAL-01 | 01-01 | Differ gate uses cosine epsilon, not `reflect.DeepEqual` on `[]float32` | ✓ SATISFIED | `internal/retrievaleval/vector.go`, `TestCosineDistance` passes live. |
| EVAL-02 | 01-01 | Retrieval-eval skip guard reads resolved koanf config, not raw `os.Getenv` | ✓ SATISFIED | `internal/retrievaleval/gate.go`, `TestMain` uses `retrievalEvalEnabled()`, `TestEvalGate` passes live. |
| RANK-01 | 01-02, 01-03, 01-04, 01-05 | Retrieval eval includes an independently written paraphrase case, reporting recall@k/MRR for vector-only/lexical/Jev | ✓ SATISFIED | `01-EVAL-SHIPPED.log` table; `paraphrase_fixture.go`; `TestParaphraseCorpusIntegrity` passes live. |
| RANK-02 | 01-02, 01-04, 01-05, 01-06 | Default ranking does not regress the paraphrase case vs vector-only and keeps #261's target at rank 1 | ✓ SATISFIED | `01-EVAL-SHIPPED.log`'s two `D-10 gate PASS:` lines; `rankCandidates` seam wired to `RerankHits`; `TestRankCandidatesIsTheD05Winner` passes live. |

No orphaned requirements: REQUIREMENTS.md's Traceability table maps only EVAL-01, EVAL-02, RANK-01, RANK-02 to Phase 1, and all four appear in at least one plan's `requirements:` frontmatter (01-01 through 01-06).

### Anti-Patterns Found

None. Scanned all 20 files listed in `01-REVIEW.md`'s `files_reviewed_list` (plus `fixtures.go`) for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER`/"not yet implemented"/"coming soon" — zero matches. `01-REVIEW.md`'s one WR-01 (gofmt non-compliance in `rankers_test.go`) was independently confirmed fixed (`gofmt -l` now clean, matching commit `d57ebd15`).

### Prohibitions (must_haves.prohibitions)

| # | Statement | Category | Verification tier | Status |
|---|-----------|----------|--------------------|--------|
| 1 | MUST NOT let the blind query author see any seed-record text (01-03) | integrity-of-evidence | judgment | Resolved per artifact evidence — flagged for human review (see Human Verification) |
| 2 | MUST NOT commit verbatim spine content/secrets/engram identifiers in the corpus (01-03) | privacy | test | ✓ VERIFIED — `TestParaphraseCorpusIntegrity/denylist` ran live and PASSED (wired enforcement) |
| 3 | MUST NOT offer/accept/record a shipped ranking other than `decideRanking`'s winner (01-05) | integrity-of-evidence | judgment | Resolved per artifact evidence — flagged for human review (see Human Verification) |
| 4 | MUST NOT post to GitHub unless `01-RANKING-DECISION.md` records authorization (01-06) | consent | judgment | Resolved per artifact evidence — flagged for human review (see Human Verification) |
| 5 | MUST NOT ship a ranking other than the approved winner / re-tune to pass gates (01-06) | integrity-of-evidence | test | ✓ VERIFIED — `TestRankCandidatesIsTheD05Winner` ran live and PASSED (wired enforcement) |

Per the fail-closed rule for test-tier prohibitions, items 2 and 5 have real, passing, wired tests and are not flagged. Per the mode-dependent soft-gate rule for judgment-tier prohibitions, items 1, 3, and 4 carry a non-authoritative LLM-judge verdict of "resolved" (backed by artifact inspection: blind-query provenance header, decision-record token match, and a live-confirmed GitHub comment) but are surfaced to a human for explicit sign-off rather than silently passed — see Human Verification below.

### Human Verification Required

3 items need human review — all are judgment-tier `must_haves.prohibitions` (integrity-of-evidence / consent) whose resolution rests on trusting recorded procedure and provenance rather than a machine-checkable assertion. The verifier inspected all referenced artifacts and found them internally consistent with each prohibition's claimed resolution; the items below exist because policy requires no judgment-tier prohibition to pass silently, not because contradicting evidence was found.

1. **Blind-query authorship integrity (01-03)**
   **Test:** Review `01-BLIND-QUERY-PROMPT.md` and `01-BLIND-QUERIES.md` provenance header; confirm the query author had no access to `paraphraseSeeds`.
   **Expected:** Labels-only prompt, 0 tool calls, no repository context, per the recorded provenance.
   **Why human:** No wired mechanical check (by design — the plan's own `reason` field calls this a "spec-less fallback, flagged-unverified"); the verifier can only inspect the artifact record, not the actual dispatched subagent's context.

2. **D-05 checkpoint never re-selected a ranking (01-05)**
   **Test:** Confirm `Approved winner: lexical` in `01-RANKING-DECISION.md` is the same token `decideRanking` mechanically emitted (`Rule winner: lexical`), and the checkpoint only confirmed/halted rather than choosing.
   **Expected:** Both tokens match; checkpoint reply is `approve-and-post`, not a substitution.
   **Why human:** Judgment-tier integrity-of-evidence prohibition; the mechanical rule itself is unit-tested (`TestDecideRanking`), but the human checkpoint's own conduct is not machine-checkable.

3. **#605 evidence comment posted only after authorization (01-06)**
   **Test:** Confirm `Evidence comment authorized: yes` was recorded before the GitHub comment was posted.
   **Expected:** Authorization dated 2026-09-23; comment `5788878938` created `2026-09-23T04:10:10Z`.
   **Why human:** Consent prohibition, judgment tier; the verifier confirmed the comment exists live and the dates are consistent, but cannot verify causal ordering beyond the recorded artifact dates.

### Gaps Summary

No gaps. All four ROADMAP success criteria (mapped 1:1 to EVAL-01, EVAL-02, RANK-01, RANK-02) are backed by code that exists, is substantive, is wired into the production call path (`SearchReranked` → `rankCandidates` → `RerankHits`), and is proven by live test runs re-executed independently by this verification (not just SUMMARY.md narration) — 12/12, 12/12, 7/7, 14/14, and 3/3 subtests across five targeted `go test` invocations, plus a full live `task eval:retrieval` run (`01-EVAL-SHIPPED.log`, `EXIT_STATUS:0`) showing both `D-10 gate PASS:` lines. The `#605` evidence comment was independently confirmed live on GitHub via `gh api`, not merely trusted from the SUMMARY. The single `01-REVIEW.md` warning (gofmt non-compliance) was independently confirmed fixed. The phase's status is `human_needed` solely because three `must_haves.prohibitions` are judgment-tier by design (no wired mechanical check was ever intended for blind-authorship integrity, checkpoint-non-reselection, or GitHub-posting consent) — policy requires these to surface for explicit human sign-off rather than pass silently, even though the verifier's own inspection of the recorded artifacts found no inconsistency.

---

_Verified: 2026-09-23T12:06:02Z_
_Verifier: Claude (gsd-verifier)_

## Human Validation

**2026-09-23 — approved by user ("All good — continue")** in the autonomous run. All three judgment-tier items were confirmed: blind-query authorship integrity, D-05 checkpoint non-reselection, and consent for the #605 GitHub post.
