---
phase: 03-curation-verdicts
plan: 08
subsystem: testing
tags: [curationeval, verdict, jev, live-eval, docs, phase-gate]

# Dependency graph
requires:
  - phase: 03-curation-verdicts
    provides: "03-04's internal/curationeval.syntheticPairs (70-pair blind-agreed corpus); 03-05's server.DeciderFromEnv/VerdictSettings; 03-07's task eval:curation harness (evaluate, metrics, formatReport, thresholdGate)"
provides:
  - "03-EVAL-RESULTS.md: the live, aggregate-only measurement of the 70-pair committed corpus against a real Decisions provider (OpenRouter/jev, typesafe/jev-1.13-20260917), with provenance and the D-03 gate result recorded verbatim"
  - "CLAUDE.md Layout rows for internal/verdict/ and internal/curationeval/, and a cmd/engram/ note on consolidate's advisory-verdict attachment"
  - "A green phase gate on the final tree: task (lint + full suite), task license:check, task surfaces:gen (no-op), go test ./internal/keylinks/, clean git status"
affects: []

# Actuals (#2632)
actuals:
  tokens: 900
  tasks: 2
  commits: 3
  plan_head_before: ca28e8ca3cd1929ef344c134c2a36ba14628c8bb

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Tracer feedback gate skipped by design: Task 1 (tracer) produces no code, only a results doc, and its own <verify> automated blocks were the closing check — no expansion task follows it in this plan, so the gate's purpose (checking before layering more) does not apply"

key-files:
  created:
    - .planning/phases/03-curation-verdicts/03-EVAL-RESULTS.md
  modified:
    - CLAUDE.md
    - Taskfile.yaml

key-decisions:
  - "Task 2's phase-gate run surfaced a pre-existing task lint:yaml failure (Taskfile.yaml's eval:curation desc line at 198 chars, over yamlfmt's 120-char cap) introduced by 03-07 within this same phase. Per the plan's own instruction ('a failure caused by this phase gets fixed in the owning file with its own commit'), fixed it as an isolated deviation: ran yamlfmt Taskfile.yaml (reflows only that one line, verified via diff against an unmodified copy), committed separately from both of this plan's declared tasks."
  - "The live run used the single command the environment instructions specified (ENGRAM_DECISIONS_PROVIDER=jev ENGRAM_DECISIONS_BASE_URL=https://openrouter.ai/api task eval:curation), relying on the ambient ENGRAM_OPENAI_API_KEY for the D-03 API-key fallback (server.DeciderFromEnv: ENGRAM_DECISIONS_API_KEY or ENGRAM_OPENAI_API_KEY) — no ENGRAM_DECISIONS_MODEL override; the registry default (typesafe/jev-1.13) resolved to the live snapshot typesafe/jev-1.13-20260917."
  - "The run was captured once, exit 0, with a scored verdict for every one of the 70 pairs (no transport failure) — no re-run was needed or performed."

requirements-completed: [CUR-03]

coverage:
  - id: D1
    description: "task eval:curation ran against the real provider (OpenRouter/jev) and its aggregate report is recorded verbatim in 03-EVAL-RESULTS.md with provenance (date, commit SHA, endpoint host, model snapshot, threshold, corpus size)"
    requirement: CUR-03
    verification:
      - kind: other
        ref: "task eval:curation live run, 2026-09-24T05:05:29Z, exit 0 — 03-EVAL-RESULTS.md"
        status: pass
    human_judgment: false
  - id: D2
    description: "The D-03 gate row is recorded exactly as the harness printed it, unaltered, and it is PASS — a non-PASS would have been recorded as-is and escalated as a blocker without tuning threshold/corpus/criteria"
    requirement: CUR-03
    verification:
      - kind: other
        ref: "03-EVAL-RESULTS.md: 'gate threshold=0.900 n=40 correct=40 result=PASS'"
        status: pass
    human_judgment: false
  - id: D3
    description: "03-EVAL-RESULTS.md contains only CURATION-EVAL | aggregate lines plus provenance — no pair text, no key-shaped strings"
    requirement: CUR-03
    verification:
      - kind: other
        ref: "plan 03-08 Task 1 <verify> automated blocks (14 CURATION-EVAL lines >= 8; exactly 1 result= and 1 brier= line; 0 key-shaped matches; 0 of 70 recordA prefixes leaked)"
        status: pass
    human_judgment: false
  - id: D4
    description: "CLAUDE.md's Layout table names internal/verdict/ and internal/curationeval/, and its cmd/engram row notes that consolidate attaches advisory relation verdicts when a decisions provider is configured"
    verification:
      - kind: other
        ref: "git show --numstat --format= 1eda09d4 -- CLAUDE.md (3 insertions, 1 deletion — only the Layout-table lines)"
        status: pass
    human_judgment: false
  - id: D5
    description: "The phase gate is green on the final tree: task (lint + full suite), task license:check, task surfaces:gen as a no-op, the key-links satisfiability gate, and a clean git status"
    verification:
      - kind: other
        ref: "task (exit 0, all lint + go test ./... + python tests green); task license:check (2255 checked, 0 invalid); task surfaces:gen followed by git status --porcelain (clean, only this plan's own pending CLAUDE.md edit); go test ./internal/keylinks/ -count=1 (ok)"
        status: pass
    human_judgment: false

# Metrics
duration: ~35min
completed: 2026-09-24
status: complete
---

# Phase 3 Plan 8: Live Curation Eval & Phase Close-Out Summary

**Live `task eval:curation` run against OpenRouter's jev provider on the 70-pair committed corpus passes the D-03 hard gate (40/40 at or above threshold 0.900, `result=PASS`), recorded aggregate-only with provenance; CLAUDE.md documents the two new packages, and the full phase gate — lint, tests, license, surfaces:gen, key-links — is green on the final tree.**

## Performance

- **Duration:** ~35 min
- **Started:** ~2026-09-24T05:00:00Z
- **Completed:** 2026-09-24T05:40:00Z
- **Tasks:** 2/2 complete
- **Files touched:** 3 (1 created: `03-EVAL-RESULTS.md`; 2 modified: `CLAUDE.md`, `Taskfile.yaml`)

## Accomplishments

- Task 1 (tracer): ran `task eval:curation` once against a live Decisions provider (OpenRouter, jev, `typesafe/jev-1.13-20260917`) over the committed 70-pair corpus. Every pair scored (0 unavailable). D-03's threshold gate: `result=PASS` (40/40 verdicts at or above 0.900 threshold matched gold). Wrote `03-EVAL-RESULTS.md` — provenance (date, git SHA, endpoint host `openrouter.ai` only, model snapshot, threshold, corpus size) plus the verbatim `CURATION-EVAL | ` report (14 lines: corpus/scored/unavailable counts, per-bucket accuracy, Brier vs. the 0.800 uniform baseline, the gate row, the five-by-five confusion matrix, needs_review count) plus a one-paragraph reading comparing buckets against the spike's reference numbers and noting `updates`' role as the corpus's main confusability sink (matching 03-04's blind-labeling disagreement pattern).
- Deviation (auto-fixed, Rule 1): the phase-gate run's `task lint:yaml` failed on a pre-existing (within-phase) `Taskfile.yaml` line over yamlfmt's 120-char cap (03-07's `eval:curation` desc). Reflowed via `yamlfmt Taskfile.yaml`, verified isolated (no other lines changed), committed separately from both plan tasks.
- Task 2: added `internal/verdict/` and `internal/curationeval/` rows to CLAUDE.md's Layout table, plus a note on the `cmd/engram/` row that `spine-review consolidate` attaches advisory relation verdicts when `ENGRAM_DECISIONS_PROVIDER` is set. Ran the full phase gate on the final tree: `task` (lint + go test ./... + python tests, all green), `task license:check` (2255 files checked, 0 invalid), `task surfaces:gen` (regenerated goldens/proto stubs, resulted in a clean working tree — no drift), and `go test ./internal/keylinks/ -count=1` (ok).

## Task Commits

Each task was committed atomically:

1. **Task 1: Live curation eval recorded as an aggregate-only artifact** — `f62a6a2d` (docs)
2. **Deviation: reflow Taskfile.yaml's eval:curation desc under yamlfmt's line-length cap** — `44e6613f` (fix)
3. **Task 2: CLAUDE.md layout rows and the phase gate** — `1eda09d4` (docs)

**Plan metadata:** committed separately below (this SUMMARY + STATE/ROADMAP/REQUIREMENTS).

## Files Created/Modified

- `.planning/phases/03-curation-verdicts/03-EVAL-RESULTS.md` — provenance, verbatim aggregate report, D-03 gate result, reading paragraph
- `CLAUDE.md` — Layout table: `internal/verdict/`, `internal/curationeval/` rows; `cmd/engram/` row's `spine-review consolidate` clause
- `Taskfile.yaml` — `eval:curation` desc reflowed under yamlfmt's 120-char cap (deviation fix, not a plan-declared file)

## Decisions Made

See `key-decisions` in frontmatter: the within-phase yamlfmt fix (Rule 1, isolated commit), the live-run command and API-key fallback source, and the single-run-no-retry decision (no transport failure occurred).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `task lint:yaml` failed on Taskfile.yaml's `eval:curation` desc line**
- **Found during:** Task 2, phase-gate run (`task`)
- **Issue:** `yamlfmt -lint .` failed: the `eval:curation` task's `desc:` line (added by plan 03-07, earlier in this same phase) is 198 characters, exceeding `.yamlfmt`'s `max_line_length: 120`. `Taskfile.yaml` is not in this plan's `files_modified`, but the failure was introduced within this phase and blocks the phase gate this plan's own Task 2 must leave green.
- **Fix:** Ran `yamlfmt Taskfile.yaml` (single file, in place). Verified via a diff against an untouched copy that only the one desc line's wrapping changed — no other content in the file was reformatted.
- **Files modified:** `Taskfile.yaml`
- **Verification:** `task lint:yaml` — clean (no diff output). Re-ran the full `task` (lint + test) afterward — all green.
- **Commit:** `44e6613f`

**Total deviations:** 1 auto-fixed (yamlfmt line-length reflow, isolated to one line). **Impact:** none on any shipped behavior; purely a formatting fix required to leave the phase gate green, as the plan's own Task 2 instructs.

## TDD Gate Compliance

Not applicable — neither task in this plan carries `tdd="true"`.

## Issues Encountered

None beyond the auto-fixed deviation above.

## Authentication Gates

None — `ENGRAM_OPENAI_API_KEY` was already present in the executing environment (the orchestrator's credentialed shell), satisfying Task 1's precondition without needing a checkpoint.

## User Setup Required

None.

## Known Follow-ups (from Task 2's own instruction)

- The `curating-spine` skill does not yet read the new `verdict` object emitted by `spine-review consolidate` — its judgment workflow was out of scope for this phase.
- The private real-spine eval mode (`ENGRAM_CURATION_EVAL_PAIRS`, D-01) has not been run in this environment — no local pair file was configured. `task eval:curation` ran the committed-corpus subtest only.

## Next Phase Readiness

- CUR-03 marked complete in `REQUIREMENTS.md` (this plan was its last not-yet-executed declaring plan across 03-04/03-07/03-08, shared-ID gate #2388).
- Phase 3 (Curation Verdicts) is complete: all 3 requirements (CUR-01, CUR-02, CUR-03) satisfied, phase gate green on the final tree (`task`, `task license:check`, `task surfaces:gen` no-op, `go test ./internal/keylinks/`), clean `git status` apart from this plan's own pending metadata commit.
- No blockers. The D-03 gate result is `PASS` — no threshold, corpus, or criteria adjustment was needed or made.

---
*Phase: 03-curation-verdicts*
*Completed: 2026-09-24*

## Known Stubs

None.

## Threat Flags

None — this plan's threat register entries (T-03-08, T-03-21, T-03-22) are all satisfied: provenance recorded (T-03-08), the results file carries only aggregates and both verify checks (key-shaped strings, pair-text leak) passed with zero matches (T-03-21), and the first complete run was recorded verbatim with `result=PASS` — never tuned (T-03-22).

## Self-Check: PASSED

- `.planning/phases/03-curation-verdicts/03-EVAL-RESULTS.md`: FOUND
- Commit `f62a6a2d`: FOUND in `git log --oneline --all`
- Commit `44e6613f`: FOUND in `git log --oneline --all`
- Commit `1eda09d4`: FOUND in `git log --oneline --all`
- Plan-level `<verification>`: `03-EVAL-RESULTS.md` exists with the verbatim aggregate report and one gate row — confirmed. `task` green and `task surfaces:gen` a no-op on the final tree — re-confirmed at SUMMARY time (`git status --porcelain` clean apart from this plan's own uncommitted metadata).
- Gate result recorded verbatim: **`result=PASS`** (40/40 verdicts at or above threshold 0.900 matched gold).

Phase 3 (Curation Verdicts) complete. Ready for `/gsd-verify-work` and the next phase.
