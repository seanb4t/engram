---
phase: 01-eval-foundation-lexical-reranker-fix
plan: 04
subsystem: eval
tags: [retrieval-eval, ranking, decision-rule, go, tdd]

# Dependency graph
requires:
  - phase: 01-01
    provides: "Trustworthy eval gates (cosine-epsilon differ, koanf-resolved ENGRAM_RETRIEVAL_EVAL) the restructured TestRetrievalEval still gates on first"
  - phase: 01-02
    provides: "store.CandidateK, store.VectorOrder, and the eval-local comparison rankers (lexicalRerank, cosineBlendRerank, overlapGateRerank) the named-ranker roster wraps"
  - phase: 01-03
    provides: "paraphraseSeeds/paraphraseTopics (the 96-record corpus and 24 topic labels) and 01-BLIND-QUERIES.md (the blind author's verbatim 24-query reply) this plan wires into paraphraseCase"
provides:
  - "internal/retrievaleval/rankers.go: the D-11 pluggable named-ranker roster (vector-only, lexical, the D-07 overlap-gate/cosine-blend grid, a disabled Jev stub), variantMetrics, variantSummary/buildSummaries/formatVariantTable, and decideRanking — D-05's mechanically-applied decision rule"
  - "internal/retrievaleval/retrieval_eval_test.go: TestRetrievalEval restructured to measure every roster entry over SearchReranked's own candidate pool, apply D-10's two hard gates, and log the variant table plus the D-05 decision"
  - "internal/retrievaleval/paraphrase_fixture.go: paraphraseCase — the independent paraphrase retrievalCase wired from the blind queries, wantKey-mapped to paraphraseTopics by topic ID"
  - "internal/retrievaleval/fixtures.go: retrievalQuery.wantKey (query-level target, D-12's no-answer marker) replacing the deleted retrievalCase.wantKey; caseRole (roleRegressionGuard/roleParaphrase)"
affects: ["01-05 (runs this eval live, gets a winner by decideRanking, not by eye)", "01-06 (wires the D-05 winner into SearchReranked)"]

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
actuals:
  tokens: 14014
  tasks: 3
  commits: 3
plan_head_before: 3fd8fa3b6f636cbe9e35f14b529b66f3699ac3e2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Pluggable named-ranker roster (namedRanker{name, family, simplicity, tuned, rank, disabledReason}): a disabled entry (Jev) carries a nil rank function and participates in no ranking/gating — Phase 4 enables it with a pure append, never a refactor"
    - "A pre-committed decision rule (decideRanking) implemented as a pure function over []variantSummary and unit-tested (14 subtests) BEFORE any live number exists, so a human or agent cannot pick the shipped ranking by eye"
    - "variantMetrics wraps the package's existing recallAtK/reciprocalRank helpers rather than reimplementing the id-list scan, and derives recall/mrr/allRank1 from its own accumulated per-query ranks so an empty value is never NaN"

key-files:
  created:
    - internal/retrievaleval/rankers.go
    - internal/retrievaleval/rankers_test.go
  modified:
    - internal/retrievaleval/fixtures.go
    - internal/retrievaleval/retrieval_eval_test.go
    - internal/retrievaleval/paraphrase_fixture.go
    - internal/retrievaleval/paraphrase_fixture_test.go

key-decisions:
  - "D-04 implemented exactly as planned: gh261Case's content/text strings stay byte-identical to main (verified by diff against `git show main:...`); only its Go struct shape changed to carry the new query-level wantKey and caseRole fields (Pitfall 3 avoided by deleting retrievalCase.wantKey entirely, forcing every literal onto the new shape)"
  - "D-11/D-06/D-09/D-10/D-12 implemented exactly as planned: the pluggable roster, the shipped row measured through st.SearchReranked itself, the per-variant Markdown table, the two D-10 hard gates (#261 rank 1; shipped paraphrase MRR >= vector-only's), and no-answer queries logged only"
  - "D-05/D-07 implemented exactly as planned: decideRanking applies the pre-committed rule mechanically (eligibility, the 0.05 tuned-only margin, best-eligible-MRR-wins, simplicity-then-name tie-break), and the D-07 grid (theta 0.90/0.75/0.60, alpha 0.05/0.10/0.20/0.30) is fixed in code before any live number exists"
  - "D-01 implemented exactly as planned: paraphraseCase's 24 queries are transcribed verbatim from 01-BLIND-QUERIES.md (mechanically verified — every blind-query line's exact text is present in paraphrase_fixture.go), mapped to targets by topic ID in a separate, non-blind pass, with the D-01 procedure (labels-only prompt, fresh tool-less subagent author, date, prompt commit 2606c358) recorded in the case's doc comment"
  - "Corrected a stale per-plan commit ledger (.git/gsd-plan-head-before-01-04), left over from an EARLIER milestone's own phase 01-04 (dated 2026-09-18, predating this plan by four days) — same class of issue plan 01-03 documented for its own ledger. Reset to this plan's actual start commit (3fd8fa3b) so actuals.commits above measures only this plan's 3 commits"

patterns-established:
  - "A rankingDecision{winner, eligible, reason} value flows from decideRanking into formatVariantTable (the eligible column) and into a separate t.Logf line (winner/reason) — the renderer and the decision-logging line consume the same value independently, so the table and the decision line can never disagree"

requirements-completed: [RANK-01, RANK-02]

coverage:
  - id: D1
    description: "wantKey moved from retrievalCase to retrievalQuery (Pitfall 3: the case-level field is deleted, not deprecated); gh261Case's content/query text stays byte-identical to main, only its struct shape changed (D-04)"
    requirement: RANK-01
    verification:
      - kind: unit
        ref: "acceptance check: rg -o 'wantKey +string' fixtures.go count == 1 (query-level only), case-level wantKey count == 0"
        status: pass
      - kind: other
        ref: "acceptance check: diff against `git show main:internal/retrievaleval/fixtures.go` content:/text: literals — IDENTICAL"
        status: pass
    human_judgment: false
  - id: D2
    description: "evalRankers() returns the D-11 pluggable roster in order: vector-only, lexical, the D-07 overlap-gate grid (t0.90/t0.75/t0.60), the D-07 cosine-blend grid (a0.05/a0.10/a0.20/a0.30), and a disabled Jev stub with a nil rank and 'Jev: disabled' reason (D-06, D-07, D-11)"
    requirement: RANK-01
    verification:
      - kind: unit
        ref: "internal/retrievaleval/rankers_test.go#TestEvalRankersRoster"
        status: pass
    human_judgment: false
  - id: D3
    description: "variantMetrics/variantSummary/buildSummaries/formatVariantTable aggregate recall@k/MRR per (ranker, case-role) and render a Markdown table with a leading '| variant |' header; the shipped row is measured through st.SearchReranked itself over the exact store.CandidateK(defaultK) pool every other variant ranks (D-09)"
    requirement: RANK-01
    verification:
      - kind: unit
        ref: "internal/retrievaleval/rankers_test.go#TestVariantMetrics"
        status: pass
      - kind: unit
        ref: "internal/retrievaleval/rankers_test.go#TestFormatVariantTable"
        status: pass
      - kind: other
        ref: "acceptance check: store.CandidateK(defaultK) count == 1, st.SearchReranked( count == 1 in retrieval_eval_test.go"
        status: pass
    human_judgment: false
  - id: D4
    description: "decideRanking applies D-05's rule mechanically over []variantSummary: eligibility (guard rank 1 AND paraphrase MRR >= vector-only's), the 0.05 margin gate applied only to tuned (grid) candidates when vector-only is itself eligible, best-eligible-MRR wins, ties go to lowest simplicity then smallest name, and a missing baseline or empty eligible set both report no winner with a named reason — pinned before any live number exists (D-05, D-07)"
    requirement: RANK-02
    verification:
      - kind: unit
        ref: "internal/retrievaleval/rankers_test.go#TestDecideRanking (14 subtests covering every clause and tie rule)"
        status: pass
    human_judgment: false
  - id: D5
    description: "D-10's two hard gates: gh261Case's shipped target at rank 1 for both queries (tightened from 'within default k'), and shipped paraphrase MRR at least vector-only's — both t.Errorf, with a harness failure if no paraphrase-role queries were measured. No absolute recall/MRR threshold is asserted anywhere"
    requirement: RANK-02
    verification:
      - kind: other
        ref: "acceptance check: 'D-10 gate FAILED:' count == 2, 'D-10 gate PASS:' count == 2, decideRanking( call-site count == 1, absolute-threshold regex count == 0, all in retrieval_eval_test.go"
        status: pass
    human_judgment: false
  - id: D6
    description: "No-answer queries (retrievalQuery.wantKey == \"\") are excluded from every recall@k/MRR aggregate and produce only 'no-answer <case>/<query>: <variant> top=<key> score=<score>' log lines, one per enabled roster entry plus shipped (D-12)"
    requirement: RANK-01
    verification:
      - kind: unit
        ref: "internal/retrievaleval/paraphrase_fixture_test.go#TestParaphraseCorpusIntegrity/queries (pins the 4 no-answer topics carry empty wantKey through paraphraseCase.queries)"
        status: pass
    human_judgment: true
    rationale: "The no-answer log lines themselves only execute inside the live, Qdrant-gated TestRetrievalEval (plan 01-05); this plan proves the fixture wiring hermetically but the log-line behavior itself needs a live run to observe."
  - id: D7
    description: "paraphraseCase wraps paraphraseSeeds with 24 queries transcribed verbatim from 01-BLIND-QUERIES.md, mapped to targets by topic ID; its doc comment records the D-01 procedure (labels-only prompt, fresh tool-less subagent author, 2026-09-22, prompt pinned at commit 2606c358, separate non-blind mapping pass) (D-01, D-03, RANK-01)"
    requirement: RANK-01
    verification:
      - kind: unit
        ref: "internal/retrievaleval/paraphrase_fixture_test.go#TestParaphraseCorpusIntegrity/queries"
        status: pass
      - kind: unit
        ref: "internal/retrievaleval/paraphrase_fixture_test.go#TestRetrievalCasesRoles"
        status: pass
      - kind: other
        ref: "acceptance check: every '- Tnn: <query>' line in 01-BLIND-QUERIES.md is present verbatim in paraphrase_fixture.go (0 MISSING lines)"
        status: pass
    human_judgment: false
  - id: D8
    description: "Harness-correctness failures (empty result, zero score, unseeded wantKey, target missing at the ceiling, embed error, no paraphrase queries measured) are all prefixed 'harness:', distinguishing a broken harness from a failed ranking gate"
    requirement: RANK-01
    verification:
      - kind: other
        ref: "acceptance check: rg -o '\"harness: ' retrieval_eval_test.go count == 10 (>= 5 required)"
        status: pass
    human_judgment: false

duration: ~35min
completed: 2026-09-23
status: complete
---

# Phase 1 Plan 4: Pluggable Named-Ranker Eval, D-05 Decision Rule, and the Blind Paraphrase Case Summary

**A pluggable named-ranker roster (vector-only, lexical, the D-07 overlap-gate/cosine-blend grid, a disabled Jev stub) measured over `SearchReranked`'s own candidate pool, a mechanically-applied D-05 decision rule (`decideRanking`, 14 pinned subtests), D-10's two hard gates, and the independent paraphrase case wired from plan 01-03's blind queries.**

## Performance

- **Duration:** ~35 min
- **Started:** ~2026-09-23T02:55Z (approximate)
- **Completed:** 2026-09-23T03:29:10Z
- **Tasks:** 3/3 completed
- **Files modified:** 6 (2 created, 4 modified)

## Accomplishments

- `retrievalQuery` now carries its own `wantKey` (an empty value marks a no-answer query, D-12); `retrievalCase.wantKey` is deleted entirely (Pitfall 3), forcing every case literal — including `gh261Case` — onto the new shape. `gh261Case`'s content and query text stay byte-identical to `main`; only its struct shape and the tightened D-10 rank-1 bar changed.
- New `internal/retrievaleval/rankers.go` holds the D-11 pluggable named-ranker roster (`evalRankers`), `variantMetrics` (wrapping `recallAtK`/`reciprocalRank`), `variantSummary`/`buildSummaries`/`formatVariantTable` (the per-variant Markdown table), and `decideRanking` — D-05's decision rule applied mechanically and pinned by 14 hermetic subtests (`TestDecideRanking`) before any live number exists.
- `TestRetrievalEval` is restructured: every enabled roster entry ranks the exact candidate pool `SearchReranked` would fetch (`store.CandidateK(defaultK)`), the shipped row is measured through `st.SearchReranked` itself, no-answer queries are logged only, and D-10's two hard gates (`#261` shipped target at rank 1; shipped paraphrase MRR >= vector-only's) are both `t.Errorf` — no absolute recall/MRR threshold is asserted anywhere.
- `paraphraseCase` wraps plan 01-03's 96-record corpus with 24 queries transcribed verbatim from `01-BLIND-QUERIES.md`, mapped to targets by topic ID in a separate, non-blind pass; its doc comment records the full D-01 provenance. `retrievalCases` now holds `gh261Case` plus `paraphraseCase`; `TestRetrievalCasesRoles` pins the dataset's two roles.

## Task Commits

Each task was committed atomically:

1. **Task 1: Named-ranker eval end to end on the #261 case** - `a4c1239d` (test, TDD RED->GREEN)
2. **Task 2: D-05 applied in code (decideRanking, D-07 grid, MRR gate)** - `edeea5dd` (test, TDD RED->GREEN)
3. **Task 3: Wire the paraphrase case from the blind queries** - `0604918e` (test)

**Plan metadata:** (this commit)

## RED Observations (TDD)

- **Task 1:** `rankers_test.go` (referencing `variantMetrics`, `variantSummary`, `formatVariantTable`, `evalRankers`, `rankingDecision`) was written before `rankers.go` existed. `go vet ./internal/retrievaleval/` failed at compile time: `vet: internal/retrievaleval/rankers_test.go:23:9: undefined: variantMetrics`. Restructuring `fixtures.go` to move `wantKey` to `retrievalQuery` then broke `retrieval_eval_test.go`'s case-level `tc.wantKey` reference (`vet: ... tc.wantKey undefined`), confirmed as the second RED signal before restructuring the test loop. Implemented `rankers.go` and the restructured `TestRetrievalEval`; all three targeted tests (`TestVariantMetrics`, `TestFormatVariantTable`, `TestEvalRankersRoster`) PASS, and the gated tests SKIP cleanly with the gate off.
- **Task 2:** `rankers_test.go` was extended with the T2-form `TestEvalRankersRoster` (10-entry roster) and `TestDecideRanking` before `decideRanking`/the D-07 grid existed. `go vet` failed: `vet: internal/retrievaleval/rankers_test.go:307:11: undefined: decideRanking`. Implemented the grid constants, extended `evalRankers()`, and `decideRanking`; all four targeted tests PASS, with `TestDecideRanking` covering all 14 behavior rows (>= 14 required).

## Files Created/Modified

- `internal/retrievaleval/fixtures.go` — `retrievalQuery.wantKey` added, `retrievalCase.wantKey` deleted, `caseRole`/`roleRegressionGuard`/`roleParaphrase` added, `gh261Case` updated to the new shape, `retrievalCases` now includes `paraphraseCase`
- `internal/retrievaleval/rankers.go` (new) — `rankerFunc`, `namedRanker`, `evalRankers`, `shippedRowName`, `jevDisabledReason`, `d05TunedMargin`, `mrrEpsilon`, `gateThetaGrid`, `blendAlphaGrid`, `variantMetrics`, `variantSummary`, `buildSummaries`, `rankingDecision`, `formatVariantTable`, `decideRanking`
- `internal/retrievaleval/rankers_test.go` (new) — `TestVariantMetrics`, `TestFormatVariantTable`, `TestEvalRankersRoster`, `TestDecideRanking`
- `internal/retrievaleval/retrieval_eval_test.go` — `TestRetrievalEval` restructured around the named-ranker roster, the D-10 gates, the D-05 decision log line, and the "shipped matches" diagnostic
- `internal/retrievaleval/paraphrase_fixture.go` — `paraphraseCase` added (24 blind-authored queries mapped by topic ID)
- `internal/retrievaleval/paraphrase_fixture_test.go` — `queries` subtest added to `TestParaphraseCorpusIntegrity`; `TestRetrievalCasesRoles` added

## Decisions Made

- Placed `decideRanking`'s "margin-veto" reason text (`"vector-only wins: no tuned variant cleared D-05's 0.05 margin over vector-only"`) behind a `marginDroppedAny` flag so it only fires when a tuned candidate was genuinely dropped by the margin filter — avoiding a misleading reason string on rows where vector-only simply had the best MRR outright with no tuned competition.
- Used the literal `"8"` in `formatVariantTable`'s header instead of referencing `retrieval_eval_test.go`'s `defaultK` constant: `rankers.go` is a non-test file and cannot reference a `_test.go`-only symbol without breaking `go build ./...`. [Rule 3 — blocking issue, resolved inline during Task 1]
- Corrected a stale per-plan commit ledger (`.git/gsd-plan-head-before-01-04`), discovered to be a leftover from an EARLIER milestone's own phase 01-04 (dated 2026-09-18, four days before this plan started) — the same class of issue plan 01-03 documented for its own ledger. Reset to this plan's actual start commit (`3fd8fa3b`). [Rule 3 — blocking issue, resolved before writing this SUMMARY's `actuals`]

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `formatVariantTable` could not reference `retrieval_eval_test.go`'s `defaultK`**
- **Found during:** Task 1, `go build ./internal/retrievaleval/...`
- **Issue:** `rankers.go` (a non-test file) referenced `defaultK`, which is defined in `retrieval_eval_test.go` (`_test.go`-only). `go build ./...` (excluding test files) failed: `undefined: defaultK`.
- **Fix:** Replaced the `%d`-formatted header with the literal `"8"`, with a comment explaining the constraint and noting both name the production MCP default k.
- **Files modified:** `internal/retrievaleval/rankers.go`
- **Verification:** `go build ./internal/retrievaleval/...` exits 0.
- **Commit:** `a4c1239d`

**2. [Rule 3 - Blocking] Transient `golangci-lint` unused-field finding on `rankingDecision.winner`/`.reason` mid-Task-1**
- **Found during:** Task 1, `golangci-lint run ./internal/retrievaleval/...`
- **Issue:** `rankingDecision{winner, eligible, reason}` was introduced in Task 1 per the plan's own artifact spec, but `winner`/`reason` have no reader until Task 2's `decideRanking` and its D-05 log line — golangci-lint's `unused` check flagged both fields.
- **Fix:** Added a narrowly-scoped `//nolint:unused` comment on each field, documenting that Task 2 (same plan, immediate next task) supplies the first real reader. Removed both comments in the same Task 2 commit that wired `decideRanking` and the eval's `D-05 decision` log line — the suppression never outlived the gap it covered.
- **Files modified:** `internal/retrievaleval/rankers.go`
- **Verification:** `golangci-lint run ./internal/retrievaleval/...` — 0 issues, both before the Task 2 fields became genuinely used (with the temporary nolint) and after (nolint removed, still 0 issues).
- **Commit:** `a4c1239d` (added), `edeea5dd` (removed)

**3. [Rule 3 - Blocking] Stale per-plan commit ledger sentinel from an earlier milestone's own phase 01-04**
- **Found during:** Task 3 (SUMMARY actuals computation)
- **Issue:** `.git/gsd-plan-head-before-01-04` already existed, dated 2026-09-18 (from the prior milestone `2026-09-18.01 — Bounded Reads`, which also had a phase 01, plan 04) — four days before this plan started. The ledger-setup guard (`if [ ! -f "$_GSD_LEDGER" ]`) correctly skipped writing over it, but that meant `git rev-list --count` against it measured 45 commits spanning two milestones, not this plan's 3.
- **Fix:** Reset the sentinel to this plan's actual pre-Task-1 HEAD (`3fd8fa3b`, the `01-03` plan's own close-out commit).
- **Files modified:** `.git/gsd-plan-head-before-01-04` (not a tracked repo file)
- **Verification:** `git rev-list --count 3fd8fa3b..HEAD` reports `3`, matching this plan's three task commits (`a4c1239d`, `edeea5dd`, `0604918e`).
- **Commit:** N/A (git-internal file, not tracked)

**Total deviations:** 3 auto-fixed (2 blocking-build/lint, 1 blocking-accounting). **Impact:** none on shipped behavior — all three are either a mechanical non-test/test-file boundary fix, a self-resolving temporary lint suppression, or a SUMMARY-accounting-only correction.

## Authentication Gates

None.

## Known Stubs

None — `evalRankers()`'s disabled Jev stub is an intentional, plan-specified D-11 artifact (not a data stub): it participates in no ranking or gating and renders `"Jev: disabled"` in the table. Phase 4 enables it with a pure append.

## Threat Flags

None — this plan implements exactly the mitigations recorded in the plan's own `<threat_model>` (T-01-09, T-01-10): `decideRanking` is unit-tested before any live number exists and the live eval logs its output (T-01-09); harness failures carry a distinct `harness:` prefix separate from `D-10 gate FAILED:`, and the shipped row is measured through `SearchReranked` itself with a "matches" diagnostic tying it to a roster row (T-01-10). No new trust boundary or surface was introduced.

## Self-Check: PASSED

- All 6 key-files (2 created, 4 modified) found on disk.
- `git log --oneline --all | grep -q a4c1239` → FOUND
- `git log --oneline --all | grep -q edeea5d` → FOUND
- `git log --oneline --all | grep -q 0604918` → FOUND
- Re-ran every task's `<acceptance_criteria>` command: all PASS (see per-task verify output above and the Deviations/Decisions sections).
- Re-ran the plan-level `<verification>`: `env -u ENGRAM_RETRIEVAL_EVAL go test ./internal/retrievaleval/ -count=1 -v` → all hermetic tests PASS, all three gated tests (`TestRetrievalEval`, `TestRetrievalEval_AsymmetryDiffer`, `TestSharedQdrantAddressHonored`) SKIP, exit 0; `task lint` → exit 0 (golangci-lint, actionlint, yamlfmt, rumdl, ruff all clean); `env -u ENGRAM_RETRIEVAL_EVAL go build ./...` → exit 0 (whole-repo build unaffected).

## Next Phase Readiness

- Plan 01-05 can run the live eval: the roster, the table, `decideRanking`, and both D-10 gates are all wired and ready to produce a real winner by rule, not by eye.
- Plan 01-06 will wire whichever variant `decideRanking` selects into `SearchReranked` itself.
- No blockers.

---
*Phase: 01-eval-foundation-lexical-reranker-fix*
*Completed: 2026-09-23*
