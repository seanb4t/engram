---
phase: 14-embedder-model-options-eval
plan: 01
subsystem: testing
tags: [embed, retrieval-eval, gemini, go-test, asymmetric-embedding]

# Dependency graph
requires:
  - phase: 09-retrieval-quality-baseline
    provides: internal/retrievaleval package (gh261Case fixture, TestRetrievalEval harness, TestMain testcontainer lifecycle)
provides:
  - "differProbe synthetic fixture (internal/retrievaleval/fixtures.go) — a single string embedded twice for the asymmetry differ-case"
  - "TestRetrievalEval_AsymmetryDiffer — permanent, skip-gated Pitfall-12 correctness gate proving asymmetric query/document embedding takes effect"
affects: [14-02-embedder-model-recipes, 14-03-gemini-eval-evidence]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Differ-case pattern: embed the same string via em.EmbedQuery and em.Embed through the prod-parity embedder and assert the vectors differ, with a dimension-contract check before the inequality assertion"
    - "Test naming as a routing mechanism: naming a new test with an existing -run regex's substring (TestRetrievalEval prefix) reaches it from a documented Taskfile command with zero Taskfile change"

key-files:
  created: []
  modified:
    - internal/retrievaleval/fixtures.go
    - internal/retrievaleval/retrieval_eval_test.go

key-decisions:
  - "Named the new test TestRetrievalEval_AsymmetryDiffer (not TestEmbedAsymmetryDiffer) so task eval:retrieval's `-run TestRetrievalEval` unanchored-regex substring-matches it without any Taskfile.yaml change (review finding A)."
  - "Gemini asymmetry rides ENGRAM_EMBED_QUERY_INSTRUCTION/ENGRAM_EMBED_DOCUMENT_INSTRUCTION (text-prefix mechanism), never ENGRAM_EMBED_QUERY_PARAMS/ENGRAM_EMBED_DOCUMENT_PARAMS/task_type — per research supersession (gemini-embedding-2 does not honor task_type over the OpenAI-compat endpoint)."
  - "Verification gate scoped to task lint:go + go test ./... + task license:check, not the whole-tree task/task lint target, because task lint runs rumdl over .planning/ and fails on pre-existing markdown noise scoped to a separate Phase-21 requirement (review B2)."

patterns-established:
  - "Symmetric-config skip guard: before asserting query != document, check whether the operator's config is legitimately symmetric (no instruction/params env vars set) and t.Skip() the inequality assertion in that case, so the suite stays green for symmetric embedders."
  - "Dimension-contract-before-inequality: never accept an empty or wrong-sized vector as evidence of correct asymmetry; assert len == configured dim on both sides first."

requirements-completed: [REQ-embed-gemini-direct]

coverage:
  - id: D1
    description: "Permanent skip-gated differ-case (TestRetrievalEval_AsymmetryDiffer) proves asymmetric query/document embedding takes effect through the prod-parity embedder path, reachable via task eval:retrieval, zero-cost when ENGRAM_RETRIEVAL_EVAL is unset"
    requirement: "REQ-embed-gemini-direct"
    verification:
      - kind: unit
        ref: "internal/retrievaleval/retrieval_eval_test.go#TestRetrievalEval_AsymmetryDiffer (skip path, ENGRAM_RETRIEVAL_EVAL unset)"
        status: pass
      - kind: other
        ref: "go build ./internal/retrievaleval/ && go vet ./internal/retrievaleval/"
        status: pass
      - kind: other
        ref: "task lint:go"
        status: pass
      - kind: other
        ref: "go test ./..."
        status: pass
      - kind: other
        ref: "task license:check"
        status: pass
    human_judgment: false

# Metrics
duration: 11min
completed: 2026-07-11
status: complete
---

# Phase 14 Plan 1: Gemini asymmetry differ-case Summary

**Permanent, skip-gated `TestRetrievalEval_AsymmetryDiffer` test that embeds one synthetic string through both `em.EmbedQuery` and `em.Embed` on the production embedder path and fails if the resulting vectors are identical — the Pitfall-12 correctness gate for asymmetric embedding configs.**

## Performance

- **Duration:** ~11 min
- **Started:** 2026-07-11T16:52:00Z (approx, first commit 12:52:42-04:00)
- **Completed:** 2026-07-11T17:03:00Z (approx)
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Added `differProbe`, a synthetic (secret-free) public-tooling sentence embedded once query-side and once document-side, as a permanent sibling fixture to `gh261Case` in `fixtures.go`.
- Added `TestRetrievalEval_AsymmetryDiffer`, gated on `ENGRAM_RETRIEVAL_EVAL=1` as its first statement, with a symmetric-config `t.Skip` guard, a dimension-contract assertion (non-empty and `len == dim`) before the inequality check, a hard `t.Fatal` on vector equality (Pitfall-12 failure), and a greppable `asymmetry differ PASS` log line on success.
- Confirmed the test is reachable via the documented `task eval:retrieval` command's `-run TestRetrievalEval` regex without any `Taskfile.yaml` change (the naming choice from review finding A).
- Verified the default gate stays zero-cost: with `ENGRAM_RETRIEVAL_EVAL` unset, the test skips immediately (no embedder call, no Docker/testcontainer cost beyond what `TestRetrievalEval` already gates).

## Task Commits

Each task was committed atomically:

1. **Task 1: Add the synthetic differ probe to fixtures.go** - `4f12c459` (feat)
2. **Task 2: Add the skip-gated differ assertion (TestRetrievalEval_AsymmetryDiffer)** - `f1832063` (feat)

**Plan metadata:** (this commit)

## Files Created/Modified
- `internal/retrievaleval/fixtures.go` - Added `differProbe` synthetic string constant + doc comment explaining the differ mechanism and the correct/incorrect env-var paths.
- `internal/retrievaleval/retrieval_eval_test.go` - Added `TestRetrievalEval_AsymmetryDiffer` (import: `reflect`) implementing the skip-gated Pitfall-12 correctness gate.

## Decisions Made
- Kept the differ probe out of `retrievalCases`/Qdrant entirely — it only compares two in-memory vectors, so it needs no `seedRecord`/testcontainer involvement, per D-04 discretion and the plan's Option B recommendation.
- Used `reflect.DeepEqual` for the `[]float32` vector-equality check (simplest correct comparison for a `nil`-safe, element-wise `[]float32` compare; matches the RESEARCH.md sketch).

## Deviations from Plan

None - plan executed exactly as written. Both tasks' acceptance criteria were met without needing any Rule 1/2/3 auto-fixes.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required. The differ-case is exercised against a live Gemini gateway in plan 14-03; this plan only lands the code and proves the default-gate skip path.

## Next Phase Readiness

`TestRetrievalEval_AsymmetryDiffer` is ready to be run live (`ENGRAM_RETRIEVAL_EVAL=1` plus the Gemini gateway/instruction env vars, with Docker available for the package `TestMain` testcontainer) as evidence for plan 14-03's live-eval evidence gate. No blockers for 14-02 (embedder model recipes) or 14-03 (Gemini eval evidence).

---
*Phase: 14-embedder-model-options-eval*
*Completed: 2026-07-11*

## Self-Check: PASSED

- FOUND: internal/retrievaleval/fixtures.go
- FOUND: internal/retrievaleval/retrieval_eval_test.go
- FOUND: .planning/phases/14-embedder-model-options-eval/14-01-SUMMARY.md
- FOUND: commit 4f12c459
- FOUND: commit f1832063
