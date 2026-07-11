---
phase: 14-embedder-model-options-eval
plan: 03
subsystem: eval
tags: [gemini, qwen3, retrieval-eval, embeddings, evidence]

# Dependency graph
requires:
  - phase: 14-01
    provides: skip-gated TestRetrievalEval_AsymmetryDiffer differ-case
  - phase: 14-02
    provides: Gemini and qwen3-embedding-8b@4096 docs/Helm recipes with exact run env
provides:
  - Committed, auditable live-eval evidence closing success criteria #1 and #2
  - Confirmed Gemini compat model-id (gemini-embedding-2, unchanged from 14-02)
affects: [phase-14-verification, milestone-v0.10.x-closeout]

# Tech tracking
tech-stack:
  added: []
  patterns: []

key-files:
  created: [.planning/phases/14-embedder-model-options-eval/14-EVAL-EVIDENCE.md]
  modified: []

key-decisions:
  - "Gemini compat model-id confirmed unchanged (gemini-embedding-2, 3072-dim) — no edits to embedding-models.md or values.yaml needed"

patterns-established: []

requirements-completed: [REQ-embed-gemini-direct, REQ-embed-prod-parity-eval]

coverage:
  - id: D1
    description: "Live curl confirms gemini-embedding-2 is the correct GA model-id with 3072-dim embeddings, matching the shipped 14-02 recipe unchanged"
    requirement: REQ-embed-gemini-direct
    verification:
      - kind: manual_procedural
        ref: "Operator-run curl against https://generativelanguage.googleapis.com/v1beta/openai/embeddings, HTTP 200 + len(embedding)==3072"
        status: pass
    human_judgment: false
  - id: D2
    description: "Live TestRetrievalEval_AsymmetryDiffer PASS against the Gemini recipe env — instruction-prefix asymmetry took effect at 3072 dim"
    requirement: REQ-embed-gemini-direct
    verification:
      - kind: integration
        ref: "internal/retrievaleval/retrieval_eval_test.go#TestRetrievalEval_AsymmetryDiffer — operator-run, exit status 0"
        status: pass
    human_judgment: false
  - id: D3
    description: "Live gh261-sticky-neighbor-crowding recall@8=1.00 PASS against qwen3-embedding-8b@4096 recipe env — closes #261"
    requirement: REQ-embed-prod-parity-eval
    verification:
      - kind: integration
        ref: "internal/retrievaleval/retrieval_eval_test.go#TestRetrievalEval/gh261-sticky-neighbor-crowding — operator-run, exit status 0"
        status: pass
    human_judgment: false
  - id: D4
    description: "Committed evidence artifact records both PASSes + confirmed model-id, keys redacted, fail-closed grep verified"
    verification:
      - kind: other
        ref: "fail-closed grep against 14-EVAL-EVIDENCE.md (recall@8=1.00, 2x hard rank bar PASS, differ PASS token, 2x exit status 0, no failing tokens) — GREP_OK"
        status: pass
    human_judgment: false

duration: 8min
completed: 2026-07-11
status: complete
---

# Phase 14 Plan 03: Live Eval Evidence Summary

**Committed a redacted, fail-closed evidence artifact proving the Gemini differ-assertion and qwen3-embedding-8b@4096 recall@8 PASS live, and confirmed the Gemini compat model-id is unchanged.**

## Performance

- **Duration:** 8 min (Task 2 only — Task 1 was a human-verify checkpoint executed live by the operator in a prior session)
- **Tasks:** 2 (1 checkpoint, 1 auto)
- **Files modified:** 1 created

## Accomplishments

- Confirmed the exact Gemini OpenAI-compat model-id (`gemini-embedding-2`, HTTP 200, 3072-dim embedding) via a live curl — matches the shipped 14-02 recipe, no reconciliation edits needed
- Captured a live `TestRetrievalEval_AsymmetryDiffer` PASS against the Gemini recipe env (query vector != document vector at 3072 dim), closing success criterion #1
- Captured a live `gh261-sticky-neighbor-crowding` `recall@8=1.00` PASS against the qwen3-embedding-8b@4096 recipe env (both `hard rank bar: PASS` lines), closing success criterion #2 and #261
- Wrote `14-EVAL-EVIDENCE.md` as a narrow, redacted structured template (no keys, no raw terminal dump) that passes the plan's fail-closed verification grep

## Task Commits

Task 1 (checkpoint:human-verify) produced no commit — it was a pause-and-report gate; the operator's confirmed results were the resume signal consumed by Task 2.

1. **Task 2: Write the eval-evidence artifact and reconcile the model-id** - `0a1372f0` (docs)

**Plan metadata:** (this commit)

## Files Created/Modified

- `.planning/phases/14-embedder-model-options-eval/14-EVAL-EVIDENCE.md` - Redacted live-eval evidence: confirmed model-id + 3072 dim, sanitized differ command + PASS lines, sanitized qwen3@4096 command + PASS lines, issue-closure handoff

## Decisions Made

- Gemini compat model-id confirmed unchanged (`gemini-embedding-2`, 3072-dim) — `docs-site/src/content/docs/guides/embedding-models.md` and `charts/engram/values.yaml` left untouched per the plan's "confirmed unchanged" branch.

## Deviations from Plan

None - plan executed exactly as written. Task 1's checkpoint was satisfied by an operator-run live evaluation (per D-06, local/manual execution — this environment holds no Gemini/OpenRouter keys); the sanitized, redacted results were supplied as the resume signal and transcribed verbatim into the evidence template by Task 2.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required for this plan's automated task. (Task 1's live runs required operator-held `ENGRAM_OPENAI_API_KEY` values for Gemini and OpenRouter, already exercised and reported before this continuation began.)

## Next Phase Readiness

- Success criteria #1 and #2 for Phase 14 are closed with committed, auditable, fail-closed-verified evidence.
- REQ-embed-gemini-direct (#331) and REQ-embed-prod-parity-eval (#334) are satisfied.
- The committing PR for this branch must use `Closes #261`, `Closes #334`, `Closes #331` (already included in the Task 2 commit message; carry through to the PR body per review B10 handoff).
- Phase 14 (embedder-model-options-eval) has no further plans — this closes the phase's execution.

---
*Phase: 14-embedder-model-options-eval*
*Completed: 2026-07-11*
