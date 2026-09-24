---
phase: 01-eval-foundation-lexical-reranker-fix
plan: 02
subsystem: eval
tags: [ranking, cosine-similarity, retrieval-eval, go]

# Dependency graph
requires:
  - phase: 01-01
    provides: "Trustworthy eval gates (cosine-epsilon differ, koanf-resolved ENGRAM_RETRIEVAL_EVAL) this plan's new tests run under"
provides:
  - "store.CandidateK(k uint64) uint64 — exported over-fetch bound, unchanged behavior, usable by an external package"
  - "store.VectorOrder(hits []Memory, k int) []Memory — the pure vector-only baseline ranking (D-06), and D-08's no-op rank-step candidate"
  - "internal/retrievaleval/comparison_rankers.go: lexicalRerank, cosineBlendRerank, overlapGateRerank, normalizedOverlap — the D-06 eval-local comparison rankers, parameterized for D-07's grid"
affects: ["01-04 (named-ranker roster wires these into the grid)", "01-05 (paraphrase eval measures over these rankers)", "01-06 (wires the D-05 winner into SearchReranked; may relocate lexicalRerank's logic if a tuned variant wins)"]

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
actuals:
  tokens: 6236
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Eval-local comparison rankers ported from a shipped package rather than depending on the shipped package's unexported ranking code, so the eval survives the shipped code being deleted (D-08)"

key-files:
  created:
    - internal/retrievaleval/comparison_rankers.go
    - internal/retrievaleval/comparison_rankers_test.go
  modified:
    - internal/store/rerank.go
    - internal/store/rerank_test.go
    - internal/store/store.go

key-decisions:
  - "Comparison rankers placed in internal/retrievaleval (eval package), not internal/store, per the plan's own placement decision: keeps the eval's lexical row alive after D-08 potentially deletes the shipped lexical code from internal/store"
  - "overlapGateRerank implemented as a stable partition (promoted hits sorted by raw overlap desc/Score desc/ID asc, non-promoted hits in store.VectorOrder), which the tests confirm collapses to lexicalRerank order at theta=0 and to store.VectorOrder at theta>1"

patterns-established:
  - "Comparison-ranker functions carry the pure (query, hits, k[, param]) contract established by RerankHits/VectorOrder — no I/O, no server concepts, hermetically unit-testable"

requirements-completed: [RANK-01, RANK-02]

coverage:
  - id: D1
    description: "store.CandidateK is exported (rename only, behavior unchanged) so an external package (the eval) can fetch SearchReranked's exact candidate pool"
    requirement: RANK-01
    verification:
      - kind: unit
        ref: "internal/store/rerank_test.go#TestCandidateK"
        status: pass
      - kind: unit
        ref: "internal/store/rerank_test.go#TestSearchRerankedRejectsZeroK"
        status: pass
    human_judgment: false
  - id: D2
    description: "store.VectorOrder is the pure vector-only baseline ranking (D-06): Score desc/ID asc tie-break, immutable input, AccessCount-invariant, correct truncation"
    requirement: RANK-01
    verification:
      - kind: unit
        ref: "internal/store/rerank_test.go#TestVectorOrderScoreThenIDOrder"
        status: pass
      - kind: unit
        ref: "internal/store/rerank_test.go#TestVectorOrderTruncatesAndCopies"
        status: pass
      - kind: unit
        ref: "internal/store/rerank_test.go#TestVectorOrderIgnoresAccessCount"
        status: pass
    human_judgment: false
  - id: D3
    description: "Shipped ranking (SearchReranked -> RerankHits) is unchanged by this plan; only the over-fetch helper's name changed"
    requirement: RANK-01
    verification:
      - kind: unit
        ref: "internal/store/rerank_test.go#TestRerankHitsPromotesLexicalOverlap"
        status: pass
    human_judgment: false
  - id: D4
    description: "internal/retrievaleval's lexicalRerank is a faithful port of the shipped lexical-overlap reranker, hermetically pinned to its own fixtures (D-06)"
    requirement: RANK-02
    verification:
      - kind: unit
        ref: "internal/retrievaleval/comparison_rankers_test.go#TestLexicalRerank"
        status: pass
    human_judgment: false
  - id: D5
    description: "cosineBlendRerank equals store.VectorOrder at alpha=0 and lexicalRerank at alpha=1000; reuses Memory.Score as the cosine term without recomputation (D-06, D-07, RESEARCH Pitfall 4)"
    requirement: RANK-02
    verification:
      - kind: unit
        ref: "internal/retrievaleval/comparison_rankers_test.go#TestCosineBlendRerank"
        status: pass
    human_judgment: false
  - id: D6
    description: "overlapGateRerank equals lexicalRerank at theta=0 and store.VectorOrder at theta>1; promotes only near-verbatim overlap at a mid-range theta (D-06, D-07)"
    requirement: RANK-02
    verification:
      - kind: unit
        ref: "internal/retrievaleval/comparison_rankers_test.go#TestOverlapGateRerank"
        status: pass
    human_judgment: false
  - id: D7
    description: "All three comparison rankers are deterministic, AccessCount-invariant, and truncate correctly (k<=0, k>=len, k in between)"
    requirement: RANK-02
    verification:
      - kind: unit
        ref: "internal/retrievaleval/comparison_rankers_test.go#TestComparisonRankersDeterministic"
        status: pass
      - kind: unit
        ref: "internal/retrievaleval/comparison_rankers_test.go#TestComparisonRankersIgnoreAccessCount"
        status: pass
      - kind: unit
        ref: "internal/retrievaleval/comparison_rankers_test.go#TestComparisonRankersTruncate"
        status: pass
    human_judgment: false

duration: ~25min
completed: 2026-09-23
status: complete
---

# Phase 1 Plan 2: Ranking Variants Summary

**Exported `store.CandidateK` and added `store.VectorOrder` (the vector-only D-06 baseline / D-08 no-op candidate), plus three eval-local comparison rankers in `internal/retrievaleval` — `lexicalRerank`, `cosineBlendRerank`, `overlapGateRerank` — pinned to vector order and lexical order at their parameter extremes.**

## Performance

- **Duration:** ~25 min
- **Started:** ~2026-09-23T02:16Z (approximate)
- **Completed:** 2026-09-23T02:41:31Z
- **Tasks:** 2/2 completed
- **Files modified:** 5 (3 edited, 2 created)

## Accomplishments

- `store.CandidateK` is exported (rename only) so an external package can fetch `SearchReranked`'s exact candidate pool; `store.VectorOrder` is the new pure vector-only baseline ranking, with a deterministic Score-desc/ID-asc tie-break and no lexical or usage signal read.
- `SearchReranked`'s shipped ranking behavior is byte-for-byte unchanged — only the over-fetch helper's name changed, confirmed by the acceptance grep for a stale lowercase `candidateK` (0 hits) and the unchanged `RerankHits(...)` call site.
- Three eval-local comparison rankers (`lexicalRerank`, `cosineBlendRerank`, `overlapGateRerank`) live in `internal/retrievaleval`, never in `internal/store` — the plan's own placement decision keeps the eval reporting a lexical row even if D-08 later deletes the shipped lexical code. Property tests pin `cosineBlendRerank` to `store.VectorOrder` at alpha=0 and to `lexicalRerank` at alpha=1000, and `overlapGateRerank` to `lexicalRerank` at theta=0 and `store.VectorOrder` at theta>1.

## Task Commits

Each task was committed atomically:

1. **Task 1: Vector-only end to end (CandidateK export + VectorOrder)** — `2cd255f4` (feat)
2. **Task 2: Eval-local comparison rankers (lexical port, cosine blend, overlap gate)** — `4eb8d35e` (test)

**Plan metadata:** (this commit)

## RED Observations (TDD)

- **Task 1:** With `internal/store/rerank.go` reverted to its pre-change state (`git apply -R` on the task's own diff), `go test ./internal/store/ -run '^(TestCandidateK|TestVectorOrderScoreThenIDOrder|TestVectorOrderTruncatesAndCopies|TestVectorOrderIgnoresAccessCount)$'` failed to build: `undefined: CandidateK` (store.go:1333, rerank_test.go:29) and `undefined: VectorOrder` (rerank_test.go:150,169,172,175,199,200). Reapplied the diff; all 6 targeted tests (including the two pre-existing ones in the plan's verify list) PASS, `go build ./...` and `go vet ./internal/store/` clean.
- **Task 2:** With `internal/retrievaleval/comparison_rankers.go` moved out of the package temporarily, `go test ./internal/retrievaleval/ -run '^(TestLexicalRerank|TestCosineBlendRerank|TestOverlapGateRerank|TestComparisonRankersDeterministic|TestComparisonRankersIgnoreAccessCount|TestComparisonRankersTruncate)$'` failed to build: `undefined: lexicalRerank`, `undefined: cosineBlendRerank`, `undefined: overlapGateRerank` across every new test. Restored the file; all 6 targeted tests PASS, `go vet ./internal/retrievaleval/` and `golangci-lint run ./internal/retrievaleval/... ./internal/store/...` both clean.

## Files Created/Modified

- `internal/store/rerank.go` — `candidateK` renamed to exported `CandidateK`; new `VectorOrder(hits []Memory, k int) []Memory`
- `internal/store/rerank_test.go` — `TestCandidateK` updated to the new name; added `TestVectorOrderScoreThenIDOrder`, `TestVectorOrderTruncatesAndCopies`, `TestVectorOrderIgnoresAccessCount`
- `internal/store/store.go` — every `candidateK` reference (call site + 4 comments) updated to `CandidateK`; `SearchReranked`'s ranking body unchanged
- `internal/retrievaleval/comparison_rankers.go` (new) — `tokenize`, `lexicalOverlap` (ports), `normalizedOverlap`, `lexicalRerank`, `cosineBlendRerank`, `overlapGateRerank`
- `internal/retrievaleval/comparison_rankers_test.go` (new) — `TestLexicalRerank`, `TestCosineBlendRerank`, `TestOverlapGateRerank`, `TestComparisonRankersDeterministic`, `TestComparisonRankersIgnoreAccessCount`, `TestComparisonRankersTruncate`

## Decisions Made

- Placed the comparison rankers in `internal/retrievaleval`, not `internal/store` as `01-RESEARCH.md`/`01-PATTERNS.md` originally suggested — this is the plan's own explicit "Claude's Discretion" resolution (CONTEXT.md), not a deviation introduced during execution.
- `overlapGateRerank` implemented as a stable partition (promoted subset sorted independently, non-promoted subset ordered via `store.VectorOrder`) rather than a single unified sort key, so the theta=0/theta>1 collapse-to-neighbor behavior the plan's `<behavior>` spec requires falls out mechanically rather than needing separate-cased logic.
- One `golangci-lint` `package-comments` (revive) finding surfaced on a file-level explanatory comment placed directly above `package retrievaleval` (Go treats any such adjacent comment as the package doc, which must start "Package retrievaleval ..."). Moved the comment below the package clause (a floating file comment, not a doc comment) — no behavior change, `task lint` now clean. [Rule 1 — bug, resolved inline during Task 2, folded into commit `4eb8d35e`]

## Deviations from Plan

None - plan executed exactly as written (including the CONTEXT.md-authorized placement decision documented in the plan itself, not a deviation).

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `store.CandidateK`/`store.VectorOrder` and the three eval-local comparison rankers are ready for plan 01-04's named-ranker roster and grid wiring.
- The shipped `SearchReranked` ranking is untouched, so plan 01-05's baseline measurement reflects today's production behavior.
- No blockers.

---
*Phase: 01-eval-foundation-lexical-reranker-fix*
*Completed: 2026-09-23*

## Self-Check: PASSED

- All 6 key-files (created + modified) found on disk.
- `git log --oneline --all | grep -q 2cd255f4` → FOUND
- `git log --oneline --all | grep -q 4eb8d35e` → FOUND
- Re-ran Task 1's `<verify>`: 6/6 `--- PASS:`, `go build ./...` + `go vet ./internal/store/` clean.
- Re-ran Task 2's `<verify>`: 6/6 `--- PASS:`, `go vet ./internal/retrievaleval/` + `golangci-lint run ./internal/retrievaleval/... ./internal/store/...` clean (0 issues).
- Re-ran the plan-level `<verification>`: `go test ./internal/store/ -run '^(TestCandidateK|TestVectorOrder)'` → ok; `env -u ENGRAM_RETRIEVAL_EVAL go test ./internal/retrievaleval/ -count=1` → ok; `task lint` → exit 0.
- Re-ran every task's `<acceptance_criteria>` command: all PASS (see per-task verify output above and Task 2's Accomplishments section).
