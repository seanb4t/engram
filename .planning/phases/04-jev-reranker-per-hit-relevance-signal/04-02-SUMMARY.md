---
phase: 04-jev-reranker-per-hit-relevance-signal
plan: 02
subsystem: search
tags: [jev, reranker, relevance, decide, rank-05, rank-03]

# Dependency graph
requires:
  - phase: 04-jev-reranker-per-hit-relevance-signal
    plan: 01
    provides: internal/relevance package (NewRequest, FromResponse, Hook), store.RankHook seam
provides:
  - Explicit-truth test coverage for RANK-05's D-08 state budget (boundary, floor, multi-byte shrink, query cap, integer precision, empty/single/duplicate inputs, ordering) at the recall-maximum pool size
  - Hardened FromResponse (finite, [0,1]-ranged noul answers only) closing D-03's malformed-answer gap
  - Hook guarantee tests (single Decide call, nil-decider no-op, empty-input no-call, content-free failure log)
affects: [04-03 (ENGRAM_SEARCH_RANKER config-driven construction), 04-04 (MCP compact + CLI relevance surfacing), 04-05 (search_discovery reranker), 04-06/04-07/04-08 (docs, security review, Helm values)]

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
# Same estimateTokens scale (chars/4 over the realized diff), never a harness token count.
actuals:
  tokens: 6488
  tasks: 2
  commits: 2
plan_head_before: 61f12212b84dcf6dd66ac703908dda06160ea24a

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Budget-boundary/floor tests compute their expected threshold from the implementation's own output (build directly at the target per-candidate length, take EstimateTokens of that) rather than hand-derived arithmetic — exact regardless of the shrink loop's internal rounding"
    - "Fake decide.Decider test doubles (countingDecider) live in the test file, mirroring cmd/engram/spine_review_consolidate_test.go's existing fake-decider shape"

key-files:
  created: []
  modified:
    - internal/relevance/relevance.go
    - internal/relevance/relevance_test.go

key-decisions:
  - "No relevance.go change was needed for Task 1 — all ten RANK-05 budget/shape tests passed against plan 04-01's NewRequest/EstimateTokens as written on the first run; the plan's own D-08 shrink loop and constants required no adjustment."
  - "Task 2's RED evidence covers exactly the NaN/+Inf/-Inf/below-zero/above-one rows (the truths the plan flagged as absent from the tracer); the missing-answer, wrong-type and 0/1-boundary rows already passed against 04-01's FromResponse and stayed green throughout."

requirements-completed: [RANK-05, RANK-03]

coverage:
  - id: D1
    description: "RANK-05's D-08 state budget is proven at the recall-maximum pool (100 candidates): boundary (TokenBudget=E vs E-1), floor (MinCandidateChars vs one token less), uniform multi-byte shrink, the 2000-code-point query cap, integer ceiling precision, and the empty/single/duplicate/ordering edges — every truth is a named passing test, no relevance.go change needed"
    requirement: RANK-05
    verification:
      - kind: unit
        ref: "internal/relevance/relevance_test.go#TestNewRequestShape"
        status: pass
      - kind: unit
        ref: "internal/relevance/relevance_test.go#TestNewRequestBudgetASCII"
        status: pass
      - kind: unit
        ref: "internal/relevance/relevance_test.go#TestNewRequestBudgetShrinksMultibyte"
        status: pass
      - kind: unit
        ref: "internal/relevance/relevance_test.go#TestNewRequestQueryCap"
        status: pass
      - kind: unit
        ref: "internal/relevance/relevance_test.go#TestNewRequestBudgetBoundary"
        status: pass
      - kind: unit
        ref: "internal/relevance/relevance_test.go#TestNewRequestBudgetFloor"
        status: pass
      - kind: unit
        ref: "internal/relevance/relevance_test.go#TestNewRequestEmptyAndSingle"
        status: pass
      - kind: unit
        ref: "internal/relevance/relevance_test.go#TestNewRequestDuplicateContentStaysSeparate"
        status: pass
      - kind: unit
        ref: "internal/relevance/relevance_test.go#TestEstimateTokensCeil"
        status: pass
      - kind: unit
        ref: "internal/relevance/relevance_test.go#TestNewRequestAtRecallMaximum"
        status: pass
    human_judgment: false
  - id: D2
    description: "D-03's malformed-answer gap is closed: FromResponse now rejects NaN/+Inf/-Inf/out-of-[0,1] probabilities (RED observed first against 04-01's implementation), and Hook's single-call, nil-decider, empty-input and content-free-failure-log guarantees are proven"
    requirement: RANK-03
    verification:
      - kind: unit
        ref: "internal/relevance/relevance_test.go#TestFromResponse"
        status: pass
      - kind: unit
        ref: "internal/relevance/relevance_test.go#TestHookCallsDecideOnce"
        status: pass
      - kind: unit
        ref: "internal/relevance/relevance_test.go#TestHookNilDecider"
        status: pass
      - kind: unit
        ref: "internal/relevance/relevance_test.go#TestHookEmptyHitsNoCall"
        status: pass
      - kind: unit
        ref: "internal/relevance/relevance_test.go#TestHookFailureLogIsContentFree"
        status: pass
      - kind: integration
        ref: "internal/server/rerank_jev_test.go#TestSearchRerankJevTracer (re-verified against the stricter FromResponse)"
        status: pass
    human_judgment: false

# Metrics
duration: ~10min
completed: 2026-09-24
status: complete
---

# Phase 4 Plan 2: RANK-05 Budget Suite and D-03 Malformed-Answer Validation Summary

**Ten explicit-truth tests pin the D-08 state budget's boundary/floor/shrink/cap/precision/edge behavior at the recall-maximum pool (all green against 04-01's implementation unchanged), and a RED-first fix closes FromResponse's NaN/Inf/out-of-range validation gap while pinning Hook's single-call, nil, empty-input and content-free-log guarantees.**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-09-24T14:23:00Z (approximate — first read tool call)
- **Completed:** 2026-09-24T14:33:09Z
- **Tasks:** 2
- **Files modified:** 2 (1 hand-authored new test file grown across both tasks, 1 production file)

## Accomplishments

- `internal/relevance/relevance_test.go` (new): 15 top-level tests. Task 1 contributes 10 (`TestNewRequestShape`, `TestNewRequestBudgetASCII`, `TestNewRequestBudgetShrinksMultibyte`, `TestNewRequestQueryCap`, `TestNewRequestBudgetBoundary`, `TestNewRequestBudgetFloor`, `TestNewRequestEmptyAndSingle`, `TestNewRequestDuplicateContentStaysSeparate`, `TestEstimateTokensCeil`, `TestNewRequestAtRecallMaximum`), all passing against plan 04-01's `NewRequest`/`EstimateTokens` on the first run — no fix needed.
- `TestNewRequestBudgetBoundary` and `TestNewRequestBudgetFloor` compute their expected token thresholds from the implementation's own output (building directly at the target per-candidate length and reading `EstimateTokens` of that) rather than hand-derived math — exact regardless of the shrink loop's internal integer-division rounding.
- `TestNewRequestBudgetShrinksMultibyte` proves the D-08 guard shrinks uniformly for both 3-byte (CJK) and 4-byte (emoji) content at the recall maximum (100 candidates of 64 KiB each), landing between `MinCandidateChars` and `DefaultCandidateChars`, staying valid UTF-8.
- Task 2: `TestFromResponse`'s NaN/+Inf/-Inf/below-zero/above-one subtests were run FIRST and observed RED against plan 04-01's `FromResponse` (it mapped `Probability` verbatim with no finiteness/range check) — the missing-answer, wrong-type and 0/1-boundary subtests already passed. `FromResponse` was then extended with `math.IsNaN`/`math.IsInf` plus a `[0,1]` range check, mapping any violation to the existing `decide.ErrDecisionMalformedResponse` Kind (no new error class); all nine subtests are now green.
- `TestHookCallsDecideOnce`, `TestHookNilDecider` and `TestHookEmptyHitsNoCall` pin `Hook`'s single-call/nil/empty-input contracts — all passed against plan 04-01's `Hook` unchanged.
- `TestHookFailureLogIsContentFree` proves T-04-05 across every D-12 provider failure class (`unavailable`, `timeout`, `context_too_large`, `rate_limited`, `auth`) plus the new malformed-answer path: exactly one JSON Warn record carrying only the `class` attribute, with a planted content sentinel absent from both the captured log output and `err.Error()`.
- `internal/server`'s `TestSearchRerankJevTracer` re-verified green against the stricter `FromResponse`.

## Task Commits

Each task was committed atomically:

1. **Task 1: RANK-05 budget suite** — `dabb12a2` (test)
2. **Task 2: Provider-answer validation (RED first) and Hook guarantees** — `04a632ea` (fix, tdd — RED observed via `TestFromResponse`'s NaN/Inf/range subtests failing against plan 04-01's `FromResponse` before the fix)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `internal/relevance/relevance_test.go` (new) — the 15-test RANK-05 budget suite plus the D-03 response/hook edge suite
- `internal/relevance/relevance.go` — `FromResponse` now rejects NaN/±Inf/out-of-[0,1] probabilities (`math.IsNaN`/`math.IsInf`)

## Decisions Made

See `key-decisions` in frontmatter. No architectural deviations from the plan.

## Deviations from Plan

None — plan executed exactly as written. Task 1 required no implementation change (all ten tests passed against 04-01's code as-is); Task 2's fix was exactly the plan's own flagged gap (NaN/Inf/range validation), with no new error class or algorithm change beyond what the plan specified.

## Known Stubs

None.

## Threat Flags

None beyond the plan's own `<threat_model>` register (T-04-03, T-04-04, T-04-05), which this plan's tests directly exercise (`TestFromResponse`'s malformed-answer matrix for T-04-03, the recall-maximum budget suite for T-04-04, `TestHookFailureLogIsContentFree`'s sentinel scan for T-04-05).

## Self-Check: PASSED

- `internal/relevance/relevance_test.go` — FOUND
- `internal/relevance/relevance.go` (modified) — FOUND
- Commit `dabb12a2` — FOUND (`git log --oneline --all | grep dabb12a2`)
- Commit `04a632ea` — FOUND (`git log --oneline --all | grep 04a632ea`)
- `go test ./internal/relevance/ -count=1 -race` — PASS (15/15 top-level tests)
- `go test ./internal/server/ -run '^TestSearchRerankJevTracer$' -count=1` — PASS
- `go build ./...` — PASS
- `go vet ./internal/relevance/...` — PASS
- `golangci-lint run ./internal/relevance/...` — 0 issues
- `task license:check` — PASS
- `rg -o -e 'func Test(NewRequest[A-Za-z]+|EstimateTokensCeil)[(]' internal/relevance/relevance_test.go \| wc -l` — 10
- `rg -o -e 'math[.]IsNaN|math[.]IsInf' internal/relevance/relevance.go \| wc -l` — 2

## Issues Encountered

None. Two unrelated commits (`7ecc158b`, `61f12212`) landed on the shared branch from a concurrent process (the Phase 5 planner named in the dispatch's concurrency note) between plan 04-01's completion and this plan's first commit — neither touched `internal/relevance/`, confirmed by a clean `git diff --stat` isolating exactly this plan's two files. `plan_head_before` above is recorded as the commit immediately preceding this plan's own first commit (`dabb12a2`'s parent), which correctly excludes those unrelated commits from this plan's measured diff/commit count.

## User Setup Required

None.

## Next Phase Readiness

Ready for 04-03 (`ENGRAM_SEARCH_RANKER` config-driven construction, dedicated no-retry client). The RANK-05 budget guard and RANK-03's malformed-answer fallback are now both explicit-truth-tested at the recall maximum; no further changes to `internal/relevance` are anticipated before 04-05 (search_discovery reranker reuses this package unchanged).

---
*Phase: 04-jev-reranker-per-hit-relevance-signal*
*Completed: 2026-09-24*
