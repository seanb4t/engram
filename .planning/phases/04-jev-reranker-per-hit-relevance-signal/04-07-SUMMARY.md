---
phase: 04-jev-reranker-per-hit-relevance-signal
plan: 07
subsystem: search
tags: [jev, reranker, retrieval-eval, d-02, claude-md, docs, phase-close]

# Dependency graph
requires:
  - phase: 04-jev-reranker-per-hit-relevance-signal
    plan: 02
    provides: hardened relevance.FromResponse and store.RankHook guarantee tests underlying the composition this plan measures live
  - phase: 04-jev-reranker-per-hit-relevance-signal
    plan: 04
    provides: MCP/Connect rerank parity proof and CLI/MCP relevance surfacing
  - phase: 04-jev-reranker-per-hit-relevance-signal
    plan: 05
    provides: store.SearchDiscoveryReranked and search_discovery relevance documentation
  - phase: 04-jev-reranker-per-hit-relevance-signal
    plan: 06
    provides: evalRankers(jevHook) opt-in Jev row (never a D-05 candidate), JEV-EVAL provenance/fallback/no-answer-relevance log lines, server.SearchRankHookFromEnv() eval wiring
  - phase: 04-jev-reranker-per-hit-relevance-signal
    plan: 08
    provides: charts/engram memory.search Helm values and docs-site deploy.md rows, chart:validate gate
provides:
  - "04-EVAL-JEV.log / 04-EVAL-JEV.md — the live D-02 Jev retrieval-eval measurement with provenance, recorded verbatim, one run only (zero fallbacks at the production 2s timeout)"
  - "CLAUDE.md internal/relevance Layout row and a Memory contract sentence describing ENGRAM_SEARCH_RANKER=jev's per-hit relevance and fail-open-on-decision-error contract"
  - "A green full phase gate (task, license:check, proto:lint, chart:validate, buf breaking, surfaces:gen/ui:build no-op, keylinks, Qdrant-required suite) on the final tree, closing Phase 4"
affects: [milestone close-out (ROADMAP Phase 4 success criterion 1 reconciliation), any future phase reading CLAUDE.md's internal/relevance row]

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
# Same estimateTokens scale (chars/4 over the realized diff), never a harness token count.
actuals:
  tokens: 5699
  tasks: 2
  commits: 2
plan_head_before: 48924c3dc63929e9e9fee918b85d8e23fb0ece9b

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Live-eval provenance files (04-EVAL-JEV.log/.md) follow the same shape as 03-EVAL-RESULTS.md/01-EVAL-SHIPPED.log — verbatim capture, credential-scan verify gate, no post-hoc tuning (D-02)."

key-files:
  created:
    - .planning/phases/04-jev-reranker-per-hit-relevance-signal/04-EVAL-JEV.log
    - .planning/phases/04-jev-reranker-per-hit-relevance-signal/04-EVAL-JEV.md
  modified:
    - CLAUDE.md

key-decisions:
  - "Single run recorded: the default-2s-timeout run's JEV-EVAL | fallbacks=0/26 line showed zero fallbacks, so the plan's long-timeout re-run condition (N>0) was not triggered — no 04-EVAL-JEV-LONG-TIMEOUT.log was created, per the plan's own instruction to only produce it on a fallback."
  - "Per D-02, the numbers are recorded and reported (Jev outscored lexical and vector-only on paraphrase recall@8/MRR here) without changing ship posture: lexical stays the D-05 winner and the default ranker; Jev ships opt-in-only regardless."

requirements-completed: [RANK-03, RANK-04, RANK-05]

coverage:
  - id: D1
    description: "Live task eval:retrieval run with ENGRAM_DECISIONS_PROVIDER=jev recorded verbatim in 04-EVAL-JEV.log: JEV-EVAL enabled=true, both D-10 gate PASS lines, D-05 winner not jev, fallbacks=0/26, four no-answer jev relevance lines, package ok line, no credential-shaped strings"
    requirement: RANK-03
    verification:
      - kind: other
        ref: "live task eval:retrieval (ENGRAM_DECISIONS_PROVIDER=jev ENGRAM_DECISIONS_BASE_URL=https://openrouter.ai/api ENGRAM_SEARCH_RANKER=jev) — exit 0, ok github.com/seanb4t/engram/internal/retrievaleval 74.133s"
        status: pass
      - kind: other
        ref: "plan 04-07 Task 1 automated <verify> greps (JEV-EVAL enabled=true, 2x D-10 gate PASS, fallbacks=N/M, >=1 jev relevance=, no D-05 winner=jev, no 'Jev: disabled', package ok line, zero credential-shaped strings, D-02/task eval:retrieval present in .md)"
        status: pass
    human_judgment: false
  - id: D2
    description: "04-EVAL-JEV.md records provenance (UTC date, git SHA, endpoint host, model, timeout, embed model, exact command), the verbatim variant table/D-05/D-10/JEV-EVAL lines, every no-answer relevance line, and a reading comparing Jev to lexical/vector-only plus the D-02 supersession statement"
    requirement: RANK-03
    verification:
      - kind: other
        ref: "rg -o -e '^## (Provenance|Variant table [(]verbatim[)]|No-answer relevance [(]verbatim[)]|Reading)$' 04-EVAL-JEV.md | wc -l == 4"
        status: pass
    human_judgment: false
  - id: D3
    description: "CLAUDE.md's Layout table names internal/relevance/ and the Memory contract documents ENGRAM_SEARCH_RANKER=jev's per-hit relevance reordering with a decision failure never failing the search"
    requirement: RANK-04
    verification:
      - kind: other
        ref: "rg -o -e '^[|] .internal/relevance/. [|]' CLAUDE.md | wc -l == 1; rg -o -F 'ENGRAM_SEARCH_RANKER=jev' CLAUDE.md | wc -l >= 1"
        status: pass
    human_judgment: false
  - id: D4
    description: "The phase gate is green on the final tree: task, task license:check, task proto:lint, task chart:validate, buf breaking against main, task surfaces:gen and task ui:build as no-ops, go test ./internal/keylinks/, a fail-closed (ENGRAM_REQUIRE_QDRANT=1) run of the touched packages"
    requirement: RANK-05
    verification:
      - kind: other
        ref: "task && task license:check && task proto:lint && task chart:validate && go tool buf breaking --against '.git#branch=main' && go test ./internal/keylinks/ -count=1"
        status: pass
      - kind: other
        ref: "ENGRAM_REQUIRE_QDRANT=1 go test ./internal/relevance/ ./internal/store/ ./internal/server/ ./internal/retrievaleval/ ./cmd/engram/ -count=1"
        status: pass
      - kind: other
        ref: "task surfaces:gen && task ui:build diff-stability check (git diff before == git diff after over proto/docs-site/skill/gen/ui-gen/testdata/webauth-static) plus git status --porcelain --untracked-files=all -- internal/webauth/static empty"
        status: pass
    human_judgment: false

# Metrics
duration: 25min
completed: 2026-09-24
status: complete
---

# Phase 4 Plan 7: Live Jev Retrieval Eval Recorded, Phase Gate Green Summary

**Live `task eval:retrieval` with the Jev slot enabled measured Jev at recall@8 1.000 / MRR 0.883 (ahead of lexical's D-05-winning 0.950 / 0.817) with zero fallbacks at the production 2s timeout, recorded verbatim per D-02 without changing what ships — CLAUDE.md now documents `internal/relevance/` and the opt-in relevance contract, and the full phase gate is green on a clean tree.**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-09-24T15:35:00Z (approximate)
- **Completed:** 2026-09-24T16:00:00Z (approximate)
- **Tasks:** 2
- **Files modified:** 3 (2 created, 1 modified)

## Accomplishments

- Ran `ENGRAM_DECISIONS_PROVIDER=jev ENGRAM_DECISIONS_BASE_URL=https://openrouter.ai/api ENGRAM_SEARCH_RANKER=jev task eval:retrieval` live against Docker Qdrant, the production embedder (`google/gemini-embedding-2`), and the live Jev decisions provider over OpenRouter. Exit 0 in 74.1s. Captured verbatim (no ANSI, nothing stripped except the file:line prefix in the derived `.md` excerpts) into `04-EVAL-JEV.log`.
- One run only: `JEV-EVAL | fallbacks=0/26` — the default-2s-timeout run had zero fallbacks, so the plan's `ENGRAM_SEARCH_RERANK_TIMEOUT=10s` long-timeout re-run condition was never triggered; no `04-EVAL-JEV-LONG-TIMEOUT.log` exists.
- Jev's measured numbers: `#261 ranks [1 1]`, `recall@8 1.000`, `MRR 0.883`, `D-05 eligible: opt-in` — ahead of lexical (0.950 / 0.817, the actual D-05 winner and shipped ranking) and vector-only (1.000 / 0.579) on this corpus. Four no-answer paraphrase queries (T21-T24) each logged a `jev relevance=` line; all eight per-candidate values across all four queries sit in 0.010-0.030, near the decision-provider spike's ~0.02 no-answer reference.
- Wrote `04-EVAL-JEV.md` in `03-EVAL-RESULTS.md`'s shape: `## Provenance`, `## Variant table (verbatim)`, `## No-answer relevance (verbatim)`, `## Reading` — the reading states, per locked D-02, that these numbers are recorded and not acted on: lexical stays the default and Jev ships opt-in regardless, and that ROADMAP Phase 4 success criterion 1's "shipped only once the eval numbers justify it" is superseded by D-02.
- Added a `CLAUDE.md` Layout row for `internal/relevance/` (after `internal/verdict/`) and one sentence in the Memory contract, directly after the always-on `score` sentence, describing `ENGRAM_SEARCH_RANKER=jev`'s per-hit relevance reordering and its fail-open-on-decision-error/timeout contract.
- Ran the full phase gate on the resulting tree: `task` (lint + full Go/Python test suite), `task license:check`, `task proto:lint`, `task chart:validate`, `go tool buf breaking --against '.git#branch=main'`, `go test ./internal/keylinks/ -count=1`, `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/relevance/ ./internal/store/ ./internal/server/ ./internal/retrievaleval/ ./cmd/engram/ -count=1`, and a `task surfaces:gen`/`task ui:build` no-op stability check (diff over `proto/`, `docs-site/`, `skill/`, `gen/`, `ui/src/lib/gen/`, `cmd/engram/testdata/`, `internal/webauth/static` unchanged before/after regeneration). All green.

## Task Commits

Each task was committed atomically:

1. **Task 1: Live retrieval eval with the Jev slot enabled, recorded verbatim with provenance** — `d98860fd` (docs)
2. **Task 2: CLAUDE.md rows and the full phase gate** — `b7d3f664` (docs)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `.planning/phases/04-jev-reranker-per-hit-relevance-signal/04-EVAL-JEV.log` — verbatim `task eval:retrieval` (Jev-enabled) output
- `.planning/phases/04-jev-reranker-per-hit-relevance-signal/04-EVAL-JEV.md` — provenance, verbatim excerpts, reading
- `CLAUDE.md` — `internal/relevance/` Layout row, Memory contract relevance-reranking sentence

## Decisions Made

See `key-decisions` in frontmatter. No architectural deviations from the plan.

## Deviations from Plan

None - plan executed exactly as written. No fixes were needed to any auto-mitigated area; the tracer's own `<verify>` (both `<automated>` blocks) passed on first run.

---

**Total deviations:** 0
**Impact on plan:** None.

## Known Stubs

None.

## Threat Flags

None beyond the plan's own `<threat_model>` register (T-04-14, T-04-11), which Task 1's verify directly exercises: the credential-shaped-string scan over both `04-EVAL-JEV.log` and `04-EVAL-JEV.md` found zero matches (T-04-14), and the run was captured verbatim in a single pass with no post-hoc edits before commit (T-04-11).

## Self-Check: PASSED

- `.planning/phases/04-jev-reranker-per-hit-relevance-signal/04-EVAL-JEV.log` — FOUND
- `.planning/phases/04-jev-reranker-per-hit-relevance-signal/04-EVAL-JEV.md` — FOUND
- `CLAUDE.md` — FOUND, contains `internal/relevance/` row and `ENGRAM_SEARCH_RANKER=jev`
- Commit `d98860fd` — FOUND (`git log --oneline --all | grep d98860fd`)
- Commit `b7d3f664` — FOUND (`git log --oneline --all | grep b7d3f664`)
- `rg -o -e '^## (Provenance|Variant table [(]verbatim[)]|No-answer relevance [(]verbatim[)]|Reading)$' 04-EVAL-JEV.md | wc -l` — 4
- `git status --porcelain -- internal/ cmd/ proto/ gen/` after Task 1 — empty
- Full phase gate (`task`, `license:check`, `proto:lint`, `chart:validate`, `buf breaking`, `keylinks`, `ENGRAM_REQUIRE_QDRANT=1` suite, `surfaces:gen`/`ui:build` no-op) — all exit 0

## Issues Encountered

**Note on the literal Task 2 acceptance criterion "`git status --porcelain` prints nothing after the commit":** it does not, byte-for-byte — one pre-existing untracked entry remains, `.planning/milestone.lock`, a GSD session-tracking lock file (`{"phase":"4","session":"claude-code-session-id-...","pid":...,"updated_at":...}`) that predates this plan's execution (present in the orchestrator's own git-status snapshot at dispatch time) and is not part of this plan's `files_modified`. It is orchestrator/session bookkeeping, not a build artifact this plan's gate produced, and per the executor's scope-boundary rule it is out of scope to commit or otherwise touch here. `git status --porcelain -- CLAUDE.md .planning/phases/04-jev-reranker-per-hit-relevance-signal` is empty; everything this plan touched is committed.

## User Setup Required

None - no external service configuration required. The live provider credentials were supplied via the orchestrator's exported shell environment for this run only, never written to a file or committed.

## Next Phase Readiness

Phase 4 (Jev Reranker & Per-Hit Relevance Signal) is complete: all 8 plans have summaries, RANK-03/RANK-04/RANK-05 are shipped and gated green end to end, and the live D-02 Jev numbers are on record. Ready for `/gsd-verify-work 04` and milestone close-out. ROADMAP Phase 4 success criterion 1 ("shipped only once the eval numbers justify it") needs the orchestrator's roadmap-edit verb to record that it is superseded by locked decision D-02, per this plan's `04-EVAL-JEV.md` reading — that reconciliation is explicitly out of this plan's `files_modified` and left to the orchestrator.

---
*Phase: 04-jev-reranker-per-hit-relevance-signal*
*Completed: 2026-09-24*
