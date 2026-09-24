---
phase: 01-eval-foundation-lexical-reranker-fix
plan: 05
subsystem: retrieval-eval
tags: [retrieval-eval, ranking, lexical-reranker, D-05, D-09, D-10, D-12, embedder]

# Dependency graph
requires:
  - phase: 01-eval-foundation-lexical-reranker-fix (plans 01-04)
    provides: retrieval eval harness, ranking variants, paraphrase fixture, decideRanking rule, blind paraphrase corpus wiring
provides:
  - "Live task eval:retrieval run against real Qdrant and the production embedder (google/gemini-embedding-2, dim 3072), captured verbatim (D-09 before-evidence)"
  - "The pre-committed D-05 rule's own verdict over that measurement: winner=lexical"
  - "A blocking-decision checkpoint outcome: approved, with #605 evidence-comment posting authorized after plan 01-06's green re-run"
  - "01-RANKING-DECISION.md: the single source plan 01-06 reads to know its branch is `lexical` (no ranking code change required in internal/store)"
affects: [01-06]

# Actuals (#2632) — pairs with the plan's `estimate` to calibrate future estimates.
actuals:
  tokens: 5554
  tasks: 3
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Checkpoint confirms/halts a pre-committed decideRanking verdict; it never re-selects a variant (D-05 'decide before seeing numbers')"

key-files:
  created:
    - .planning/phases/01-eval-foundation-lexical-reranker-fix/01-EVAL-BASELINE.log
    - .planning/phases/01-eval-foundation-lexical-reranker-fix/01-RANKING-DECISION.md
  modified: []

key-decisions:
  - "D-05 rule verdict on the live 80-120-record multi-domain corpus with independently-authored blind paraphrase queries: winner=lexical (paraphrase MRR 0.817 vs vector-only 0.579), reversing spike 004's 16-record single-domain finding (lexical 0.656 vs vector-only 0.922)"
  - "Plan 01-06 branch: lexical — the rank step stays the lexical reranker; no ranking code change required in internal/store"
  - "Checkpoint approved: approve-and-post — plan 01-06 is authorized to post the #605 evidence comment after its green re-run, carrying three user-acknowledged caveats (spike-004 reversal, lexical's 0.950 vs vector-only's 1.000 paraphrase recall@8, and the live AsymmetryDiffer SKIP under a symmetric embed config)"

patterns-established:
  - "Decision-record shape (Measurement / Table / Rule verdict / Gates / Shipped matches / Plan branch / No-answer queries / Approval) separates the rule's mechanical output from the human's sign-off, so a later reader can verify byte-for-byte that Approved winner == Rule winner"

requirements-completed: [RANK-01]
# RANK-02 is also declared by plan 01-06 (shared-ID gate, #2388) — stays Pending in REQUIREMENTS.md
# until 01-06 produces its own SUMMARY.md.

coverage:
  - id: D1
    description: "Live task eval:retrieval run captured verbatim in 01-EVAL-BASELINE.log — the per-variant table (vector-only/lexical/jev rows), the D-05 decision line, and the shipped-matches line, with no harness failure, no eval SKIP, and no AsymmetryDiffer FAIL"
    requirement: RANK-01
    verification:
      - kind: other
        ref: "plan verify block 1 (Task 1) re-run against committed 01-EVAL-BASELINE.log: rg assertions for D-05 decision line, vector-only/lexical table rows, Jev: disabled, shipped-matches line, zero harness/SKIP/AsymmetryDiffer-FAIL lines"
        status: pass
    human_judgment: false
  - id: D2
    description: "01-RANKING-DECISION.md records the table, the rule's verdict (Rule winner: lexical), the eligible set, deciding clause, both D-10 gate statuses, embedder identity (model+dim only, no credential), and the plan 01-06 branch (lexical)"
    requirement: RANK-01
    verification:
      - kind: other
        ref: "plan verify block 2 (Task 1) re-run: single Rule winner line, all 7 required sections present, Rule winner token matches the log's D-05 decision line verbatim"
        status: pass
    human_judgment: false
  - id: D3
    description: "No credential leaked into any committed artifact (ENGRAM_OPENAI_API_KEY value absent from both files)"
    verification:
      - kind: other
        ref: "rg -cF against the live env var value over .planning/phases/01-eval-foundation-lexical-reranker-fix/ — zero matches"
        status: pass
    human_judgment: false
  - id: D4
    description: "Blocking checkpoint confirmed the measurement is sound (not a harness artifact) and authorized the #605 evidence-comment post for plan 01-06, without re-selecting the rule's winner"
    requirement: RANK-02
    verification:
      - kind: other
        ref: "plan verify block (Task 3) re-run: single Approved winner line, single Evidence comment authorized line, Approved winner == Rule winner byte-for-byte (lexical == lexical)"
        status: pass
    human_judgment: true
    rationale: "The checkpoint decision itself (accepting the measurement as sound, not a harness/fixture artifact) is a human judgment call the code cannot make — the automated check only proves the recorded outcome is byte-consistent with the rule's own output, not that the human's underlying judgment was correct."

# Metrics
duration: 13min
completed: 2026-09-23
status: complete
---

# Phase 1 Plan 5: Live Retrieval-Eval Baseline & D-05 Ranking Decision Summary

**Live `task eval:retrieval` run against real Qdrant and the production embedder shows lexical reranking beating vector-only on the blind multi-domain paraphrase corpus (MRR 0.817 vs 0.579) — the pre-committed D-05 rule picked `lexical` as the winner, reversing spike 004's earlier fixture-based finding, and the human checkpoint approved it plus authorized plan 01-06 to post the #605 evidence comment.**

## Performance

- **Duration:** 13 min (from Task 1's commit `4912d9b9` at 2026-09-22T23:44:04-04:00 to Task 3's commit `1f2ee5e0` at 2026-09-22T23:52:21-04:00, executed across two agent sessions separated by a checkpoint)
- **Started:** 2026-09-22T23:44:04-04:00
- **Completed:** 2026-09-22T23:52:21-04:00
- **Tasks:** 3/3 (Task 1: live baseline; Task 2: checkpoint:decision; Task 3: record approval)
- **Files modified:** 2 (both created)

## Accomplishments

- Ran `task eval:retrieval` end to end against real Qdrant and the production embedder (`google/gemini-embedding-2`, dim 3072) and captured the output verbatim in `01-EVAL-BASELINE.log` — no harness failure, no eval SKIP, no `TestRetrievalEval_AsymmetryDiffer` FAIL.
- `decideRanking` selected `lexical` as the winner by best-eligible paraphrase MRR (0.817), ahead of `vector-only` (0.579) and every cosine-blend/overlap-gate grid point measured, with no D-05 margin gate triggered (lexical already had the outright-best eligible MRR).
- Both D-10 gates PASS on today's shipped ranking: the `#261` target stays at rank 1 for both queries, and the shipped paraphrase MRR (0.817) is >= vector-only's (0.579).
- `shipped (SearchReranked) matches: lexical` — confirms the eval's lexical port matches what ships today, before any ranking change.
- Recorded the plan 01-06 branch as `lexical`: the rank step stays the lexical reranker, so no ranking code change is required in `internal/store` for 01-06.
- Human checkpoint (Task 2) approved `approve-and-post`: the rule's winner is accepted, and plan 01-06 is authorized to post the #605 evidence comment after its own green re-run, carrying three explicit caveats for that write-up (see Deviations below and `01-RANKING-DECISION.md`'s `## Approval` section).
- Task 3 recorded the approved outcome in `01-RANKING-DECISION.md`, with `Approved winner: lexical` verified byte-for-byte equal to `Rule winner: lexical`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Live baseline end to end** - `4912d9b9` (docs) — captured `01-EVAL-BASELINE.log` and the initial `01-RANKING-DECISION.md` (Measurement/Table/Rule verdict/Gates/Shipped matches/Plan branch/No-answer sections)
2. **Task 2: Confirm the measurement and authorize the #605 evidence post** - checkpoint:decision, no commit (blocking gate; resolved by user reply `approve-and-post`)
3. **Task 3: Record the approved outcome for plan 01-06** - `1f2ee5e0` (docs) — appended `## Approval` to `01-RANKING-DECISION.md`

**Plan metadata:** committed alongside this SUMMARY (see `git_commit_metadata` step)

## Files Created/Modified

- `.planning/phases/01-eval-foundation-lexical-reranker-fix/01-EVAL-BASELINE.log` - verbatim `task eval:retrieval` output against real Qdrant and the production embedder (D-09 before-evidence)
- `.planning/phases/01-eval-foundation-lexical-reranker-fix/01-RANKING-DECISION.md` - the table, the rule's verdict, both D-10 gate statuses, the plan 01-06 branch, the no-answer diagnostics, and the approved outcome with authorization and caveats

## Decisions Made

- **D-05 rule verdict:** `winner=lexical reason=best eligible paraphrase MRR` — the eligible set was `vector-only, lexical, overlap-gate-t0.90, overlap-gate-t0.75, overlap-gate-t0.60, cosine-blend-a0.05, cosine-blend-a0.10, cosine-blend-a0.20, cosine-blend-a0.30`. `lexical`'s paraphrase MRR (0.817) beat every other eligible variant, including the best cosine-blend grid point (`cosine-blend-a0.30` at 0.792).
- **Plan 01-06 branch: `lexical`** — no ranking code change required in `internal/store`; the rank step stays the lexical reranker.
- **Checkpoint reply: `approve-and-post`** — the measurement was accepted as sound (not a harness/fixture artifact), and plan 01-06 is authorized to post the #605 evidence comment after its own green re-run.
- **Three user-acknowledged caveats recorded for the evidence write-up** (in `01-RANKING-DECISION.md`'s `## Approval` section, to carry into plan 01-06's #605 comment):
  1. This result reverses spike 004's finding: live corpus lexical MRR 0.817 vs vector-only 0.579, versus the spike's 16-record single-domain fixture showing lexical 0.656 vs vector-only 0.922. The two are not comparable measurements of the same claim; the live result supersedes the spike finding for shipping purposes.
  2. Lexical's paraphrase recall@8 is 0.950 vs vector-only's 1.000 — one target falls out of the default k=8 window under lexical ranking that vector-only would have retrieved.
  3. `TestRetrievalEval_AsymmetryDiffer` SKIPPED on this live run because the operator's embed config (`google/gemini-embedding-2`) is symmetric — EVAL-01's cosine asymmetry gate is covered only by hermetic unit tests in this run, not a live asymmetric-embedder observation.

## Deviations from Plan

None - plan executed exactly as written. The checkpoint's decision was made by the user (not auto-approved, since this plan's checkpoint carries no `gate="blocking-human"` override in the workflow sense but is a genuine `checkpoint:decision` requiring explicit reply — the resume instructions confirmed the user's `approve-and-post` selection plus the three caveats, which were recorded verbatim into `01-RANKING-DECISION.md`'s `## Approval` section per Task 3's action spec).

**One executor-side correction (out of scope for the deviation-rule framework, logged for traceability):** the on-disk plan-commit ledger sentinel used to measure `actuals.commits` (`$(git rev-dir)/gsd-plan-head-before-01-05`) was found stale — it pointed at `da0b7bb8` (2026-09-18, a different milestone's phase-01/plan-05), left over from a prior milestone that reused the same phase/plan numbering under the same `.git` directory. Corrected it to the true base (`5bc6b9fa`, the parent of this plan's first commit) before measuring `actuals.commits: 2`. No production or planning content was affected; this only corrected the executor's own instrumentation.

## Authentication Gates

None encountered. The `<precondition>` on Task 1 (Docker/Qdrant reachable + `ENGRAM_OPENAI_*`/`ENGRAM_EMBED_*` env vars exported) was satisfied at run time in the prior session — the live run completed end to end with no auth error.

## Known Stubs

None.

## Threat Flags

None — this plan's changes stay within the threat model already registered in `01-05-PLAN.md` (`<threat_model>` T-01-12/T-01-13/T-01-14), all of which were mitigated per that model (credential-absence check before commit, `Approved winner == Rule winner` byte-equality gate, embedder identity recorded for the human to judge).

## Issues Encountered

None.

## Next Phase Readiness

Ready for plan 01-06. Its precondition (an `## Approval` section with a non-empty `Approved winner:` line in `01-RANKING-DECISION.md`) is satisfied: `Approved winner: lexical`, `Evidence comment authorized: yes`. 01-06's branch is `lexical` — no `internal/store` ranking code change is required; its scope is the #605 evidence comment (carrying the three caveats above) after a green re-run of the eval. `RANK-02` stays `Pending` in `REQUIREMENTS.md` until 01-06 completes (shared with this plan under the #2388 shared-ID gate); `RANK-01` is now `Complete`.

## Self-Check: PASSED

- `01-EVAL-BASELINE.log` exists on disk: FOUND
- `01-RANKING-DECISION.md` exists on disk: FOUND
- Commit `4912d9b9` exists in git history: FOUND
- Commit `1f2ee5e0` exists in git history: FOUND
- Task 1 plan-verify block 1 (log content) re-run against committed artifact: PASS
- Task 1 plan-verify block 2 (decision-record sections + winner match) re-run against committed artifact: PASS
- Task 3 plan-verify block (Approved winner == Rule winner byte-for-byte) re-run against committed artifact: PASS
- Credential-leak check (`ENGRAM_OPENAI_API_KEY` value absent from `.planning/phases/01-eval-foundation-lexical-reranker-fix/`): PASS, no leak found
- Stub-pattern scan (not available/coming soon/placeholder/TODO/FIXME) over both artifacts: none found
