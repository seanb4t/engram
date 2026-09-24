---
phase: 01-eval-foundation-lexical-reranker-fix
plan: 06
subsystem: retrieval-ranking
tags: [retrieval-eval, ranking, lexical-reranker, D-05, D-08, D-09, seam, rerank]

# Dependency graph
requires:
  - phase: 01-eval-foundation-lexical-reranker-fix (plan 01-05)
    provides: "01-RANKING-DECISION.md's Approved winner (lexical) and Evidence comment authorized (yes)"
provides:
  - "internal/store/rerank.go: rankCandidates, the single unexported rank-step seam SearchReranked's last line calls — the seam Phase 4's Jev reranker (RANK-03) plugs into (D-08)"
  - "Doc comments on RerankHits and SearchReranked citing D-05/01-RANKING-DECISION.md instead of hardcoding the lexical mechanism as permanent"
  - "internal/retrievaleval: the lexical roster row points at the exported store.RerankHits (no eval-local copy); every D-06 row still measured"
  - "internal/server/connectapi_test.go: TestRerankParityMCPAndConnect's header/messages attribute the fixed-vector fixture's order to the shipped rank step, naming D-05/lexical rather than a permanent mechanism"
  - "docs-site/src/content/docs/reference/tools.md: describes the shipped ranking as 'a lexical-overlap adjustment selected by the retrieval eval'"
  - "01-EVAL-SHIPPED.log (post-change live proof) and 01-ISSUE-605-EVIDENCE.md (posted to #605)"
affects: []

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
actuals:
  tokens: 12087
  tasks: 3
  commits: 3
plan_head_before: c2b07282e7692c05f8cf1ded9c3ff6fada1d773d

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "rankCandidates: a single unexported pass-through seam between SearchReranked and whichever function D-05 selected, so a future re-run's different winner changes one function body, never the seam's callers or signature"
    - "Eval roster rows point at the real shipped function (store.RerankHits) instead of maintaining a parallel eval-local copy, once that function is exported and stable — removes an entire class of copy/shipped drift for the winning variant"

key-files:
  created:
    - .planning/phases/01-eval-foundation-lexical-reranker-fix/01-EVAL-SHIPPED.log
    - .planning/phases/01-eval-foundation-lexical-reranker-fix/01-ISSUE-605-EVIDENCE.md
  modified:
    - internal/store/rerank.go
    - internal/store/rerank_test.go
    - internal/store/store.go
    - internal/retrievaleval/comparison_rankers.go
    - internal/retrievaleval/comparison_rankers_test.go
    - internal/retrievaleval/rankers.go
    - internal/retrievaleval/rankers_test.go
    - internal/server/connectapi_test.go
    - docs-site/src/content/docs/reference/tools.md

key-decisions:
  - "Plan 01-06 branch: lexical (per 01-RANKING-DECISION.md's Approved winner). rankCandidates delegates directly to the existing store.RerankHits — no lexical code deleted, no tuned constant added"
  - "Live re-run's D-05 decision (winner=lexical) and shipped-matches line are byte-identical to the pre-change baseline and to the approved winner — no halt, no re-decision needed"
  - "Evidence comment posted to #605 per the checkpoint's approve-and-post authorization: https://github.com/seanb4t/engram/issues/605#issuecomment-5788878938"

patterns-established:
  - "A D-05-style pre-committed decision rule's winner ships through one seam function whose body is the whole diff for a given branch — a future re-run that disagrees with the shipped winner changes exactly that function body and its pinning test, never any caller"

requirements-completed: [RANK-02]

coverage:
  - id: D1
    description: "SearchReranked's last line calls rankCandidates (the D-05 seam); its signature, k==0 guard, opts.Full=true and CandidateK(k) over-fetch are unchanged; rankCandidates delegates to RerankHits (the approved lexical winner); TestRankCandidatesIsTheD05Winner pins the seam over mixed-score, tied-score and #261-shaped hit sets at k below/equal/above len(hits)"
    requirement: RANK-02
    verification:
      - kind: unit
        ref: "internal/store/rerank_test.go#TestRankCandidatesIsTheD05Winner"
        status: pass
      - kind: unit
        ref: "internal/store/rerank_test.go#TestSearchRerankedRejectsZeroK, #TestCandidateK, #TestVectorOrderScoreThenIDOrder, #TestVectorOrderTruncatesAndCopies, #TestVectorOrderIgnoresAccessCount"
        status: pass
      - kind: integration
        ref: "internal/store real-Qdrant: TestSearchRerankedMatchesSearchMembership, TestSearchTwoPhaseBounded (ENGRAM_REQUIRE_QDRANT=1)"
        status: pass
      - kind: other
        ref: "acceptance check: seam/fetch-shape rg counts == 1 each in store.go; go build/vet/golangci-lint clean on internal/store"
        status: pass
    human_judgment: false
  - id: D2
    description: "Eval roster still reports all ten named rows (vector-only, lexical, overlap-gate grid, cosine-blend grid, disabled jev); lexical row points at the exported store.RerankHits with no eval-local copy; TestRerankParityMCPAndConnect's real-Qdrant subtests all pass with truthful comments naming the shipped rank step; docs-site tools.md describes the shipped ranking accurately"
    requirement: RANK-02
    verification:
      - kind: unit
        ref: "internal/retrievaleval/rankers_test.go#TestEvalRankersRoster"
        status: pass
      - kind: integration
        ref: "internal/server real-Qdrant: TestRerankParityMCPAndConnect (5/5 subtests, ENGRAM_REQUIRE_QDRANT=1)"
        status: pass
      - kind: other
        ref: "task lint (golangci-lint, rumdl on docs-site, license headers) exit 0"
        status: pass
    human_judgment: false
  - id: D3
    description: "Live task eval:retrieval re-run after the code change exits 0, both D-10 gates PASS, the re-run's D-05 decision (winner=lexical) matches 01-RANKING-DECISION.md's Approved winner byte-for-byte, and the shipped row matches lexical — proving RANK-02 (no paraphrase regression vs vector-only, #261 at rank 1). Evidence recorded in 01-ISSUE-605-EVIDENCE.md and posted to #605 per explicit authorization. Full task gate (lint+test) green."
    requirement: RANK-02
    verification:
      - kind: integration
        ref: "task eval:retrieval (live, real Qdrant + production embedder) captured verbatim in 01-EVAL-SHIPPED.log"
        status: pass
      - kind: other
        ref: "plan verify block (Task 3): winner match, D-10 PASS x2, zero FAILED/harness lines, D-05 decision line and shipped-matches line both equal 'lexical' — all re-run and confirmed against the committed log"
        status: pass
      - kind: other
        ref: "full `task` (lint + go test ./... + python tests) exit 0"
        status: pass
    human_judgment: true
    rationale: "Whether the checkpoint's approve-and-post authorization and its three recorded caveats were faithfully carried into the posted #605 comment (accuracy of the evidence write-up itself, not just its mechanical presence) is a human judgment call — the automated checks confirm the comment was posted with the required tables/gates/blind-provenance note present, not that a human reader would find it a faithful, non-misleading account."

# Metrics
duration: ~23min
completed: 2026-09-23
status: complete
---

# Phase 1 Plan 6: Ship the D-05 Winner and Close #605 Summary

**`SearchReranked` now routes through a single `rankCandidates` seam whose body is the D-05-approved winner — lexical reranking, unchanged in this branch — and a live post-change re-run proves RANK-02 (paraphrase MRR 0.817 ≥ vector-only's 0.579, #261 at rank 1) with evidence posted to #605.**

## Performance

- **Duration:** ~23 min (approximate; task commits span 2026-09-23T00:03:47-04:00 to 00:12:51-04:00, plus prior context-reading and post-commit `task` gate/write-up time not separately timestamped)
- **Started:** ~2026-09-23T03:50Z (approximate)
- **Completed:** 2026-09-23T04:13:02Z
- **Tasks:** 3/3 completed
- **Files modified:** 11 (2 created, 9 modified)

## Accomplishments

- `internal/store/rerank.go` gained `rankCandidates(query, hits, k)`, the single seam `SearchReranked`'s last line calls. Under the **lexical branch** (01-RANKING-DECISION.md: `Approved winner: lexical`), it delegates directly to the existing `RerankHits` — no lexical code was deleted, no tuned constant was added, and `SearchReranked`'s signature, k==0 guard, `opts.Full = true`, and `CandidateK(k)` over-fetch are all byte-identical to before.
- `TestRankCandidatesIsTheD05Winner` pins the seam over three hit-set shapes (mixed scores, tied scores, a #261-shaped set where a lower-score hit has full lexical overlap) at k below/equal/above `len(hits)`. No RED was observed or expected: the lexical branch is a direct pass-through to the already-shipped `RerankHits`, so there is no "wrong ranker" state for the test to have caught mid-implementation — this is recorded explicitly in the test's own doc comment, per the plan's lexical-branch carve-out.
- `internal/retrievaleval`: deleted the eval-local `lexicalRerank` copy and its dedicated test (`TestLexicalRerank`) — the "lexical" roster row now points directly at the exported `store.RerankHits`, so there is no copy left to drift from the shipped function. `tokenize`/`lexicalOverlap`/`normalizedOverlap` stay for the cosine-blend and overlap-gate variants. `evalRankers()` still returns all ten roster names in the same order (`TestEvalRankersRoster` PASS).
- `TestRerankParityMCPAndConnect`'s header comment and failure messages now attribute the fixed-vector fixture's deterministic order to `store.SearchReranked`'s shipped rank step (`rankCandidates`), naming lexical as today's D-05 winner rather than hardcoding it as a permanent mechanism. All 5 subtests pass against real Qdrant, including the no-cross-owner-leak assertion.
- `docs-site/src/content/docs/reference/tools.md` now describes final ordering as "a lexical-overlap adjustment selected by the retrieval eval" instead of a bare, undated "reranking" reference — keeping the non-monotonic-`score` caveat intact and accurate.
- A live `task eval:retrieval` re-run after the code change exits 0: both D-10 gates PASS (`#261 target at rank 1 for both queries`; `shipped paraphrase MRR 0.817 >= vector-only 0.579`), and the re-run's `D-05 decision: winner=lexical` / `shipped (SearchReranked) matches: lexical` lines are byte-identical to the pre-change baseline and to `01-RANKING-DECISION.md`'s `Approved winner: lexical` — no halt, no re-decision.
- `01-ISSUE-605-EVIDENCE.md` (before/after tables, both D-10 gate lines, no-answer diagnostics, D-01 blind-query provenance, the three user-acknowledged caveats) was posted to #605 per the checkpoint's `approve-and-post` authorization: **https://github.com/seanb4t/engram/issues/605#issuecomment-5788878938**
- Full `task` gate (lint + `go test ./...` + Python hook tests) is green.

## Task Commits

Each task was committed atomically:

1. **Task 1: The approved winner ships end to end through SearchReranked's single rank step** - `77d7c443` (feat, tracer/tdd — no RED possible under the lexical branch, documented in the test's doc comment)
2. **Task 2: Keep every eval row alive, the MCP/Connect parity test truthful, and the docs accurate for the shipped ranking** - `6d26c48a` (test)
3. **Task 3: Live proof after the change, the D-09 evidence, and the authorized #605 comment** - `d8ee2d5b` (docs)

**Plan metadata:** committed alongside this SUMMARY (see `git_commit_metadata` step)

## Files Created/Modified

- `internal/store/rerank.go` - added `rankCandidates` (the D-05 seam); rewrote `RerankHits`' doc comment to cite D-05/01-RANKING-DECISION.md
- `internal/store/rerank_test.go` - added `TestRankCandidatesIsTheD05Winner`
- `internal/store/store.go` - `SearchReranked`'s final line now calls `rankCandidates`; doc comments rewritten to describe the D-05-selected rank step generically
- `internal/retrievaleval/comparison_rankers.go` - deleted `lexicalRerank`; rewrote file doc comment and `cosineBlendRerank`/`overlapGateRerank` comments to reference `store.RerankHits` instead of the deleted copy
- `internal/retrievaleval/comparison_rankers_test.go` - deleted `TestLexicalRerank`; repointed every remaining `lexicalRerank(...)` call to `store.RerankHits(...)`
- `internal/retrievaleval/rankers.go` - the "lexical" roster entry's `rank` now points at `store.RerankHits` directly
- `internal/retrievaleval/rankers_test.go` - repointed `TestEvalRankersRoster`'s lexical-row assertion from `lexicalRerank` to `store.RerankHits`
- `internal/server/connectapi_test.go` - rewrote `TestRerankParityMCPAndConnect`'s header comment and `identical_reranked_order` failure message to name the shipped rank step / D-05
- `docs-site/src/content/docs/reference/tools.md` - reworded the ranking description at the two flagged locations
- `.planning/phases/01-eval-foundation-lexical-reranker-fix/01-EVAL-SHIPPED.log` (new) - verbatim post-change live eval output (D-09 after-evidence)
- `.planning/phases/01-eval-foundation-lexical-reranker-fix/01-ISSUE-605-EVIDENCE.md` (new) - the posted #605 comment body

## Decisions Made

- **Plan branch confirmed: lexical.** `01-RANKING-DECISION.md`'s `Approved winner: lexical` meant Task 1's action was the lexical-branch path: add the `rankCandidates` seam as a pass-through to the existing `RerankHits`, delete nothing, add no tuned constant.
- **No RED for Task 1's TDD gate.** The plan's own action spec anticipates this for the lexical branch ("no RED is possible because the behavior is unchanged") — `rankCandidates` is a direct delegate to already-shipped, already-tested code, so `TestRankCandidatesIsTheD05Winner` was true from its first commit. Documented in the test's own doc comment rather than silently omitted.
- **Eval-local `lexicalRerank` deleted, not kept as a parallel copy.** Per the plan's lexical-branch instruction: once the roster's lexical row points at the exported `store.RerankHits`, the eval-local copy has no purpose and is one less place to drift.
- **Evidence comment posted** immediately after the live re-run's D-05 decision matched the approved winner, per the checkpoint's `approve-and-post` authorization recorded in `01-RANKING-DECISION.md` and reiterated in this dispatch's own `<authorization>` block.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `internal/retrievaleval/rankers_test.go` also referenced the deleted `lexicalRerank`**
- **Found during:** Task 2, `go build ./internal/retrievaleval/...` after deleting `lexicalRerank` from `comparison_rankers.go`
- **Issue:** The plan's Task 2 files list did not include `rankers_test.go`, but `TestEvalRankersRoster` (in that file) called `lexicalRerank(query, sample, 3)` directly to cross-check the roster's lexical row. Deleting `lexicalRerank` broke this compile.
- **Fix:** Repointed the assertion to `store.RerankHits(query, sample, 3)`, which is what the roster row now calls — the test's intent (verify the roster's lexical entry calls the right function) is unchanged.
- **Files modified:** `internal/retrievaleval/rankers_test.go`
- **Verification:** `go build ./internal/retrievaleval/...` and `go test ./internal/retrievaleval/...` both pass; `TestEvalRankersRoster` PASS.
- **Commit:** `6d26c48a`

**Total deviations:** 1 auto-fixed (blocking build fix). **Impact:** none on shipped behavior — a necessary compile-fix consequence of deleting the eval-local `lexicalRerank` copy that the plan's Task 2 action already called for; the plan's files_modified list simply didn't enumerate every call site.

## Authentication Gates

None. The `<precondition>` on Task 3 (Docker/Qdrant reachable + `ENGRAM_OPENAI_*`/`ENGRAM_EMBED_*` env vars matching `01-RANKING-DECISION.md`'s `## Measurement`) was satisfied at run time — confirmed `ENGRAM_EMBED_MODEL=google/gemini-embedding-2` and `ENGRAM_EMBED_DIM=3072` before running, matching the recorded baseline exactly.

## Known Stubs

None.

## Threat Flags

None — this plan implements exactly the mitigations recorded in its own `<threat_model>`:
- T-01-15 (info disclosure via the ranking change): `TestRerankParityMCPAndConnect/no_cross_owner_leak_through_reranked_path` ran and passed against real Qdrant.
- T-01-16 (credential leak into committed evidence): the `ENGRAM_OPENAI_API_KEY` value was checked against both `01-EVAL-SHIPPED.log` and `01-ISSUE-605-EVIDENCE.md` before commit — no match.
- T-01-17 (posting without consent): the `gh issue comment 605` call ran only after confirming `01-RANKING-DECISION.md` contains the literal `Evidence comment authorized: yes` line.
- T-01-18 (tampering with decision integrity): the live re-run's `D-05 decision: winner=lexical` matched `Approved winner: lexical` exactly, so no re-decision or re-tuning occurred; `TestRankCandidatesIsTheD05Winner` pins the seam.

## Issues Encountered

None.

## Next Phase Readiness

Phase 1 (`01-eval-foundation-lexical-reranker-fix`) is complete: all 6 plans (01-01 through 01-06) have SUMMARY.md files. `RANK-01` and `RANK-02` are both `Complete`. `#605` is closed with evidence (comment posted, not the issue itself — the plan did not instruct closing the issue). No blockers for Phase 2.

## Self-Check: PASSED

- `internal/store/rerank.go` exists and contains `rankCandidates`: FOUND (`rg -o 'func rankCandidates' internal/store/rerank.go` → 1 match)
- `.planning/phases/01-eval-foundation-lexical-reranker-fix/01-EVAL-SHIPPED.log` exists on disk: FOUND
- `.planning/phases/01-eval-foundation-lexical-reranker-fix/01-ISSUE-605-EVIDENCE.md` exists on disk: FOUND
- Commit `77d7c443` exists in git history: FOUND
- Commit `6d26c48a` exists in git history: FOUND
- Commit `d8ee2d5b` exists in git history: FOUND
- Task 1 acceptance criteria (seam/fetch-shape rg counts, build/vet/golangci-lint): re-run, all PASS
- Task 2 acceptance criteria (`entirely RerankHits` count, `task lint`): re-run, all PASS
- Task 3 acceptance criteria (evidence file table headers + "blind", authorization-gated comment URL, no credential leak): re-run, all PASS
- Plan-level `<verification>`: `task` exits 0; `01-EVAL-SHIPPED.log` shows both D-10 gates PASS and the shipped row matching the approved winner; `git grep -n 'rankCandidates' internal/store` shows the definition, the `SearchReranked` call, and the test — all confirmed
- Credential-leak check (`ENGRAM_OPENAI_API_KEY` value absent from both new evidence files): PASS, no leak found
- Stub-pattern scan (not available/coming soon/placeholder/TODO/FIXME) over all modified files: none found
