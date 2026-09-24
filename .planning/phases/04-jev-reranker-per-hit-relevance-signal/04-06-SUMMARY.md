---
phase: 04-jev-reranker-per-hit-relevance-signal
plan: 06
subsystem: search
tags: [jev, reranker, retrieval-eval, d-02, d-03, rank-03]

# Dependency graph
requires:
  - phase: 04-jev-reranker-per-hit-relevance-signal
    plan: 01
    provides: store.RankHook, store.RankWithHook(ctx, query, hits, k, hook), store.Memory.Relevance *float64
  - phase: 04-jev-reranker-per-hit-relevance-signal
    plan: 03
    provides: server.SearchRankHookFromEnv() (store.RankHook, server.SearchRerankInfo, error) — the provider-configured-regardless-of-ranker eval seam
provides:
  - evalRankers(jevHook store.RankHook) []namedRanker — an opt-in, non-D-05-competing Jev row when jevHook is non-nil
  - namedRanker.optIn / variantSummary.optIn threaded through buildSummaries, decideRanking (D-05 exclusion), formatVariantTable ("opt-in" eligible cell)
  - TestRetrievalEval JEV-EVAL provenance/fallback log lines and per-query no-answer Jev relevance lines
affects: [04-07 (the live retrieval-eval run records Jev's numbers next to lexical/vector-only), 04-08 (docs/security review/Helm values)]

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
# Same estimateTokens scale (chars/4 over the realized diff), never a harness token count.
actuals:
  tokens: 5076
  tasks: 2
  commits: 2
plan_head_before: e1d79b64105055f2dab6927d5c8331f3c74b944f

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Opt-in eval row: a roster entry can be measured and reported (buildSummaries/formatVariantTable) while being structurally excluded from the D-05 winner decision (decideRanking) via a single optIn bool — the same shape disabled/shipped rows already used, extended rather than special-cased per row type."
    - "Eval measures the shipped composition directly: the jev roster closure calls store.RankWithHook (the exact function SearchReranked composes), never a local reimplementation — so the eval's Jev numbers cannot drift from what search_memory would actually do if ranker=jev were set."

key-files:
  created: []
  modified:
    - internal/retrievaleval/rankers.go
    - internal/retrievaleval/rankers_test.go
    - internal/retrievaleval/retrieval_eval_test.go

key-decisions:
  - "jevCalls/jevFallbacks are counted from the ranked OUTPUT (non-empty result with a nil Relevance on the first hit), not by instrumenting the hook itself — the roster's rankerFunc type carries no side channel, and applyRelevance's full-coverage contract makes 'first hit has no Relevance' and 'the whole call fell back' the same fact, so no ambiguity is lost."
  - "The no-answer per-hit relevance line reads variantRanked[\"jev\"] (already computed by the ranking loop) rather than re-invoking the hook, so the eval never issues a second, redundant Decisions request per no-answer query."

requirements-completed: [RANK-03]

coverage:
  - id: D1
    description: "evalRankers(nil) returns the unchanged ten-row roster with the jev row disabled; evalRankers(hook) with a non-nil hook enables the jev row via store.RankWithHook composed with that hook — the exact composition SearchReranked ships"
    requirement: RANK-03
    verification:
      - kind: unit
        ref: "internal/retrievaleval/rankers_test.go#TestEvalRankersRoster"
        status: pass
      - kind: unit
        ref: "internal/retrievaleval/rankers_test.go#TestEvalRankersJevEnabled"
        status: pass
    human_judgment: false
  - id: D2
    description: "The enabled jev row is opt-in: it is structurally excluded from decideRanking's D-05 candidate set, never appears in eligible even with the best paraphrase MRR and a passing #261 guard, and renders 'opt-in' (not yes/no) in formatVariantTable's D-05 eligible column"
    requirement: RANK-03
    verification:
      - kind: unit
        ref: "internal/retrievaleval/rankers_test.go#TestDecideRanking/opt-in_row_never_wins"
        status: pass
      - kind: unit
        ref: "internal/retrievaleval/rankers_test.go#TestFormatVariantTable"
        status: pass
    human_judgment: false
  - id: D3
    description: "TestRetrievalEval sources the Jev hook from server.SearchRankHookFromEnv() (enabled whenever a decisions provider is configured, independent of ENGRAM_SEARCH_RANKER), logs one JEV-EVAL provenance line and a JEV-EVAL fallbacks line, and logs each no-answer query's per-hit Jev relevance values (or 'none' on a fallback) — while the gated test still skips without ENGRAM_RETRIEVAL_EVAL and the package's own unit tests need no network, decisions provider, or Qdrant"
    requirement: RANK-03
    verification:
      - kind: unit
        ref: "internal/retrievaleval/retrieval_eval_test.go#TestRetrievalEval (skip path, ENGRAM_RETRIEVAL_EVAL unset)"
        status: pass
      - kind: other
        ref: "go vet ./internal/retrievaleval/"
        status: pass
    human_judgment: true
    rationale: "The provenance/fallback/relevance log lines can only be observed end-to-end against a real decisions provider — that live run is plan 04-07's explicit scope. This plan proves the harness compiles, vets clean, preserves the existing ENGRAM_RETRIEVAL_EVAL skip gate, and (via the acceptance-criteria greps) that the exact log formats plan 04-07 will grep for are present in source."

# Metrics
duration: 9min
completed: 2026-09-24
status: complete
---

# Phase 4 Plan 6: Retrieval Eval Jev Slot Summary

**The retrieval eval's Jev row is now opt-in-enabled (never a D-05 candidate) through the exact `store.RankWithHook` composition `SearchReranked` ships, and `TestRetrievalEval` sources it from `server.SearchRankHookFromEnv()` with provenance, fallback-count, and per-hit no-answer relevance logging.**

## Performance

- **Duration:** ~9 min
- **Started:** 2026-09-24T15:17:00Z (approximate)
- **Completed:** 2026-09-24T15:26:27Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- `namedRanker`/`variantSummary` gained `optIn`. `evalRankers(jevHook store.RankHook)` (signature changed from `evalRankers()`): a nil hook appends today's disabled Jev stub unchanged; a non-nil hook appends an enabled `jev` row (`simplicity 99`, `optIn true`, empty `disabledReason`) whose rank closure calls `store.RankWithHook(context.Background(), query, pool, k, jevHook)` — never a local reimplementation.
- `buildSummaries` copies `optIn` verbatim. `decideRanking`'s step 2 now excludes `optIn` rows from the D-05 candidate set alongside disabled and shipped rows — `TestDecideRanking`'s new `opt-in row never wins` case proves a jev row with the highest paraphrase MRR and a passing #261 guard still produces the identical winner/reason/eligible list as the same rows with jev simply absent, and never appears in `eligible`.
- `formatVariantTable` renders `opt-in` (not `yes`/`no`) in the D-05 eligible column for an enabled opt-in row.
- `TestEvalRankersRoster` updated to `evalRankers(nil)` plus an `optIn == false` assertion on every row. New `TestEvalRankersJevEnabled`: with a scripted hook, the ten-name roster order is unchanged, every non-jev row is byte-identical to the nil-hook roster, and the jev row's rank output deep-equals `store.RankWithHook` called directly with the same hook.
- `TestRetrievalEval` now calls `server.SearchRankHookFromEnv()` once (fatal on error), builds `roster := evalRankers(jevHook)`, and logs `JEV-EVAL | enabled=<bool> model=<model> endpoint_host=<host> rerank_timeout=<duration>`. The existing per-query ranking loop counts one Jev call per query and, when the returned slice is non-empty but its first hit carries no `Relevance`, one fallback; a final `JEV-EVAL | fallbacks=<N>/<M>` line (only when enabled) is logged after the existing `shipped (SearchReranked) matches` line.
- The existing no-answer branch gained one more line when the Jev row is enabled: `no-answer <case>/<query>: jev relevance=[v1 v2 ...]` (three decimals, in returned order) or `relevance=none` on a fallback — the per-hit "nothing answers this" demonstration.
- No change to the D-05 decision code path, the D-10 gates, the shipped row, or any pre-existing log format — `go test ./internal/retrievaleval/ -count=1` (unit tests only; the live eval still skips without `ENGRAM_RETRIEVAL_EVAL`) and `go vet ./internal/retrievaleval/` both pass.

## Task Commits

Each task was committed atomically:

1. **Task 1: Opt-in Jev row in the roster, excluded from the D-05 decision** — `4863f4d0` (feat, tdd — RED observed: the new/updated tests failed to compile against the unmodified `rankers.go`, `unknown field optIn`/`too many arguments to evalRankers`, before the production change)
2. **Task 2: TestRetrievalEval runs the Jev slot when a provider is configured** — `f16ee36a` (feat)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `internal/retrievaleval/rankers.go` — `evalRankers(jevHook store.RankHook)`, `namedRanker.optIn`, `variantSummary.optIn`, `decideRanking`'s opt-in exclusion, `formatVariantTable`'s `opt-in` eligible cell
- `internal/retrievaleval/rankers_test.go` — `TestEvalRankersRoster` updated for `evalRankers(nil)`; new `TestEvalRankersJevEnabled`; `TestDecideRanking`'s `opt-in row never wins` case (plus a generalized `wantEligible` cross-check against the same rows with the opt-in row dropped); `TestFormatVariantTable`'s opt-in case
- `internal/retrievaleval/retrieval_eval_test.go` — `TestRetrievalEval` wired to `server.SearchRankHookFromEnv()`, `evalRankers(jevHook)`, `JEV-EVAL` provenance/fallback lines, no-answer Jev relevance line

## Decisions Made

See `key-decisions` in frontmatter. No architectural deviations from the plan.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] A doc comment accidentally satisfied its own acceptance-criteria grep pattern**

- **Found during:** Task 2, running the `JEV-EVAL | (enabled|fallbacks)=` count acceptance check
- **Issue:** A doc comment I wrote quoted the literal `"JEV-EVAL | fallbacks=N/M"` string, so the grep counted 3 occurrences instead of the required 2 (the two real `t.Logf` format strings) — the comment's own prose collided with the acceptance criterion's exact-match regex.
- **Fix:** Reworded the comment to describe the same behavior ("Logged once at the end via the provenance line below") without repeating the literal log-line text.
- **Files modified:** `internal/retrievaleval/retrieval_eval_test.go`
- **Verification:** The grep count now prints `2`; `go vet`, `go build ./...`, and the full package test/lint suite re-ran clean after the edit.
- **Committed in:** `f16ee36a` (part of Task 2's commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Cosmetic — no behavior change, only comment wording. No scope creep.

## Known Stubs

None.

## Threat Flags

None beyond the plan's own `<threat_model>` register (T-04-11, T-04-12, T-04-13), which this plan's tests directly exercise: `TestDecideRanking`'s opt-in case (T-04-11 — the opt-in row cannot win or gate D-05; fallbacks are counted and logged so degraded Jev numbers are labeled) and `SearchRankHookFromEnv`'s pre-existing host-only/no-key provenance contract, reused unchanged here (T-04-12).

## Self-Check: PASSED

- `internal/retrievaleval/rankers.go` — FOUND
- `internal/retrievaleval/rankers_test.go` — FOUND
- `internal/retrievaleval/retrieval_eval_test.go` — FOUND
- Commit `4863f4d0` — FOUND (`git log --oneline --all | grep 4863f4d0`)
- Commit `f16ee36a` — FOUND (`git log --oneline --all | grep f16ee36a`)
- `go build ./...` — PASS
- `go vet ./internal/retrievaleval/` — PASS
- `go test ./internal/retrievaleval/ -count=1` — PASS (unit tests only; the live eval skips without `ENGRAM_RETRIEVAL_EVAL`)
- `golangci-lint run ./internal/retrievaleval/...` — 0 issues
- `gofmt -l` on all 3 changed files — clean (no output)
- `rg -o -e 'store[.]RankWithHook[(]' internal/retrievaleval/rankers.go | wc -l` — 1
- `rg -o -e 'server[.]SearchRankHookFromEnv[(][)]' internal/retrievaleval/retrieval_eval_test.go | wc -l` — 1
- `rg -o -e 'JEV-EVAL [|] (enabled|fallbacks)=' internal/retrievaleval/retrieval_eval_test.go | wc -l` — 2
- `rg -o -F 'jev relevance=' internal/retrievaleval/retrieval_eval_test.go | wc -l` — 1
- License headers (SPDX) intact on all 3 changed files — verified

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required. This plan only wires the eval harness; the live run against a real decisions provider is plan 04-07's scope.

## Next Phase Readiness

Ready for 04-07: a live `task eval:retrieval` run with `ENGRAM_DECISIONS_PROVIDER` set will now report the Jev row's numbers, its `JEV-EVAL` provenance/fallback counts, and per-query no-answer relevance values, alongside lexical/vector-only — with no effect on which ranking ships (D-05 stays untouched) and no effect on any other roster row's output.

---
*Phase: 04-jev-reranker-per-hit-relevance-signal*
*Completed: 2026-09-24*
