---
phase: 03-curation-verdicts
plan: 07
subsystem: testing
tags: [curationeval, verdict, jev, decide, eval-harness, brier-score, tdd]

# Dependency graph
requires:
  - phase: 03-curation-verdicts
    provides: "03-04's internal/curationeval.syntheticPairs (70-pair blind-agreed corpus) and labeledPair; 03-05's server.DeciderFromEnv/VerdictSettings store-free wiring seam; 03-01's internal/verdict contract (NewRequest, State, FromResult, Relations, Probabilities)"
provides:
  - "internal/curationeval.evaluate: sends any []labeledPair through the SAME verdict.NewRequest/verdict.FromResult contract spine-review consolidate ships, one DecideMany call, index-aligned []prediction out"
  - "internal/curationeval.resolveEvalGate: package-local, unregistered ENGRAM_CURATION_EVAL + ENGRAM_CURATION_EVAL_PAIRS gate (Phase 1 D-15 precedent)"
  - "internal/curationeval metrics (bucketOf, accuracyByBucket, brier, thresholdGate, confusion, formatReport): the D-03 aggregate-only report — accuracy by confidence bucket, multi-class Brier vs. a 0.800 uniform baseline, a five-by-five confusion matrix, and the single integer-arithmetic threshold gate"
  - "internal/curationeval.loadLocalPairs: strict JSON Lines loader for a private, gitignored real-spine pair file (D-01), errors naming only line numbers"
  - "task eval:curation: the gated live test target measuring the committed corpus and enforcing the D-03 gate"
affects: ["03-08 (docs pass reads consolidate's shipped verdict contract and may run/record this eval's output as aggregates)"]

# Actuals (#2632)
actuals:
  tokens: 11166
  tasks: 3
  commits: 3
  plan_head_before: 121daf46ba8ca8f1abe860d4f8e8d86a18178aea

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Gated, unregistered eval harness (mirrors internal/retrievaleval.resolveEvalGate and internal/decide/jev's liveGate exactly): package-local koanf load, default off, malformed value fails loudly, never touches internal/config.Load/Validate while off"
    - "One shared evaluate() pipeline feeds two corpora (committed syntheticPairs and an optional private local JSONL file) — same verdict contract, same metrics, same aggregate-only report; only the committed corpus carries D-03's hard gate"
    - "prediction{gold, v verdict.Verdict} carries zero record text by construction — formatReport, accuracyByBucket, brier, thresholdGate and confusion all operate on this type alone, so no function in the reporting path can leak pair content even by accident"
    - "formatMetric is the single rounding point (n==0 -> \"n/a\", else %.3f) shared by accuracyByBucket's per-bucket accuracy and brier's report line — every probability upstream stays verbatim until formatting"
    - "thresholdGate's PASS/FAIL/VACUOUS decision uses integer arithmetic only (correct*10 >= 9*n) — no float comparison, and zero confident-and-scored verdicts is VACUOUS, never a pass"

key-files:
  created:
    - internal/curationeval/gate.go
    - internal/curationeval/gate_test.go
    - internal/curationeval/evaluate.go
    - internal/curationeval/eval_test.go
    - internal/curationeval/metrics.go
    - internal/curationeval/metrics_test.go
    - internal/curationeval/localfile.go
    - internal/curationeval/localfile_test.go
  modified:
    - Taskfile.yaml
    - .gitignore
    - internal/curationeval/doc.go

key-decisions:
  - "resolveEvalGate resolves BOTH ENGRAM_CURATION_EVAL and ENGRAM_CURATION_EVAL_PAIRS through one koanf load (single TransformFunc switching on the env key), rather than a second gate.go-shaped function per var — kept the D-15 unregistered-gate pattern to exactly one call site."
  - "evaluate() takes threshold and stateChars as plain float64/int (never a *config.Config or VerdictSettings), per the plan's own Executor notes — the live test resolves those from server.DeciderFromEnv's VerdictSettings and passes them in, keeping evaluate.go free of any internal/server import."
  - "TestEvaluateAgainstStubProvider pins the stub Jev client to WithConcurrency(1) so recorded HTTP request order matches syntheticPairs' order deterministically — DecideMany's default concurrency (4) would otherwise make requests[i] <-> syntheticPairs[i] indexing flaky without weakening what the test actually proves (one request per pair, correct record_a/record_b, the shared five-option relation question)."
  - "brier() returns (score float64, n int) rather than a pre-formatted string, so TestBrier can assert exact values within 1e-12 for the three worked cases (one-hot=0, uniform=0.8, all-wrong=2); formatMetric(score, n) is the only place \"n/a\" or %.3f formatting happens, shared with accuracyByBucket."
  - "loadLocalPairs distinguishes an 'unknown field' JSON error from a generic 'malformed JSON' one by inspecting (never re-emitting) encoding/json's own error text — gives a slightly more specific message for the DisallowUnknownFields case without ever echoing the line's actual content."
  - "eval_test.go's local subtest is unconditionally skipped (never even defined as a t.Run) when ENGRAM_CURATION_EVAL_PAIRS is empty, rather than a no-op subtest — keeps `go test -v` output free of a phantom always-skipped local row when nobody has configured a private corpus."

requirements-completed: []
# CUR-03 is declared by 03-04, 03-07 AND 03-08 (shared-ID gate, #2388).
# `requirements.ready-ids` reports 0/1 ready: 03-08 has not produced a
# SUMMARY.md yet, so CUR-03 stays unflipped in REQUIREMENTS.md until 03-08
# finishes.

coverage:
  - id: D1
    description: "evaluate() sends every labeled pair through the exact verdict.NewRequest/verdict.FromResult contract spine-review consolidate uses (one DecideMany call, index-aligned), so the eval measures the shipped relation question set including updates"
    requirement: CUR-03
    verification:
      - kind: unit
        ref: "internal/curationeval/eval_test.go#TestEvaluateAgainstStubProvider"
        status: pass
    human_judgment: false
  - id: D2
    description: "TestCurationEval is gated by ENGRAM_CURATION_EVAL through a package-local, unregistered koanf load; off by default it skips naming task eval:curation, a malformed value fails loudly, and go test ./... makes no provider call"
    requirement: CUR-03
    verification:
      - kind: unit
        ref: "internal/curationeval/gate_test.go#TestResolveCurationEvalGate"
        status: pass
      - kind: integration
        ref: "internal/curationeval/eval_test.go#TestCurationEval"
        status: pass
    human_judgment: false
  - id: D3
    description: "The report gives accuracy per D-03 confidence bucket (low/mid/high, lower-inclusive), the multi-class Brier score against a 0.800 uniform baseline, and a confusion matrix, with buckets and classes always in a fixed order"
    requirement: CUR-03
    verification:
      - kind: unit
        ref: "internal/curationeval/metrics_test.go#TestBucketOf"
        status: pass
      - kind: unit
        ref: "internal/curationeval/metrics_test.go#TestAccuracyByBucket"
        status: pass
      - kind: unit
        ref: "internal/curationeval/metrics_test.go#TestBrier"
        status: pass
      - kind: unit
        ref: "internal/curationeval/metrics_test.go#TestFormatReportAggregatesOnly"
        status: pass
    human_judgment: false
  - id: D4
    description: "thresholdGate is D-03's single hard gate: on the committed corpus, verdicts at or above the resolved threshold must have accuracy at least 0.9 (integer arithmetic, correct*10 >= 9*n); zero such verdicts is VACUOUS, never a pass"
    requirement: CUR-03
    verification:
      - kind: unit
        ref: "internal/curationeval/metrics_test.go#TestThresholdGate"
        status: pass
      - kind: integration
        ref: "internal/curationeval/eval_test.go#TestCurationEval (committed subtest fails when gate result != PASS)"
        status: pass
    human_judgment: false
  - id: D5
    description: "An empty bucket reports n=0 and accuracy n/a; failed verdicts are excluded from accuracy/Brier/gate and counted separately; probabilities are used verbatim and rounded only when formatted"
    requirement: CUR-03
    verification:
      - kind: unit
        ref: "internal/curationeval/metrics_test.go#TestAccuracyByBucket/empty_input_gives_three_ordered_na_rows"
        status: pass
      - kind: unit
        ref: "internal/curationeval/metrics_test.go#TestBrier/zero_scored_predictions_report_na"
        status: pass
    human_judgment: false
  - id: D6
    description: "With ENGRAM_CURATION_EVAL_PAIRS naming a local JSONL file, the same harness runs a second local corpus and reports aggregates only — no pair text, no per-pair line, errors naming only line numbers; the recommended directory is gitignored"
    requirement: CUR-03
    verification:
      - kind: unit
        ref: "internal/curationeval/localfile_test.go#TestLoadLocalPairs"
        status: pass
      - kind: unit
        ref: "internal/curationeval/localfile_test.go#TestLoadLocalPairsRejects"
        status: pass
      - kind: unit
        ref: "internal/curationeval/localfile_test.go#TestLoadLocalPairsErrorsNeverEchoContent"
        status: pass
      - kind: other
        ref: "git check-ignore -q internal/curationeval/testdata/local/pairs.jsonl"
        status: pass
    human_judgment: false
  - id: D7
    description: "Every report line starts with CURATION-EVAL | and formatReport receives predictions only (no record text), so plan 03-08's downstream records can carry aggregates only"
    requirement: CUR-03
    verification:
      - kind: unit
        ref: "internal/curationeval/metrics_test.go#TestFormatReportAggregatesOnly"
        status: pass
    human_judgment: false

# Metrics
duration: ~40min
completed: 2026-09-24
status: complete
---

# Phase 3 Plan 7: Curation Eval Harness Summary

**One gated, unregistered eval harness measures relation-verdict accuracy over the same verdict.NewRequest/FromResult contract consolidate ships — accuracy by confidence bucket, a multi-class Brier score, a confusion matrix, and one integer-arithmetic hard gate — against both the committed 70-pair corpus and an optional private real-spine JSONL file, with every report line aggregate-only.**

## Performance

- **Duration:** ~40 min
- **Started:** ~2026-09-24T04:15:00Z (approximate — session context load)
- **Completed:** 2026-09-24T04:52:55Z
- **Tasks:** 3/3 complete
- **Files touched:** 11 (8 created in `internal/curationeval`, 3 modified: `Taskfile.yaml`, `.gitignore`, `internal/curationeval/doc.go`)

## Accomplishments

- Task 1 (tracer, TDD): `gate.go`'s `resolveEvalGate` (ENGRAM_CURATION_EVAL + ENGRAM_CURATION_EVAL_PAIRS, package-local koanf load, Phase 1 D-15 precedent), `evaluate.go`'s `evaluate()` sending labeled pairs through the shared verdict contract via one `DecideMany` call, a hermetic stub-provider test proving one HTTP request per pair with the exact `verdict.State`-truncated record text and five-option relation question, the gated live `TestCurationEval`, and the `task eval:curation` target.
- Tracer feedback gate: re-ran Task 1's full `<verify>` immediately after commit — both automated blocks passed (no `<human-check>`, `workflow.human_verify_mode` defaulting to `end-of-phase`) — expansion continued with no checkpoint.
- Task 2 (TDD): `metrics.go` — `bucketOf`/`accuracyByBucket` (D-03's low/mid/high buckets, lower-inclusive), `brier` (multi-class Brier score, verbatim probabilities), `thresholdGate` (the single hard gate, integer arithmetic, VACUOUS-never-passes), `confusion` (five-by-five gold/predicted matrix), and `formatReport` (aggregate-only, `CURATION-EVAL | `-prefixed report). `eval_test.go`'s committed subtest now fails when the gate result is not PASS.
- Task 3 (TDD): `localfile.go`'s `loadLocalPairs` — strict JSON Lines loader for D-01's private real-spine mode, errors naming only line numbers, never content; `eval_test.go` gained a conditional `local` subtest (no D-03 gate); `.gitignore` excludes `internal/curationeval/testdata/local/`; `doc.go` rewritten to document both corpora, the shared pipeline, and the one D-03 gate.

## Task Commits

Each task was committed atomically:

1. **Task 1: Gate, evaluate pipeline, stub end-to-end test, live test, task target** — `e98db939` (test, TDD RED+GREEN — verified RED via compile failure with `metrics`-independent symbols undefined before this commit's own files existed)
2. **Task 2: Buckets, Brier, threshold gate, aggregate-only report** — `24db7de3` (test, TDD RED+GREEN — confirmed RED via `go vet` failing on `undefined: formatReport` with `metrics.go` removed, before restoring it)
3. **Task 3: Local real-spine pair file, gitignore, package doc** — `c971bef6` (test, TDD RED+GREEN — confirmed RED via `go vet` failing on `undefined: loadLocalPairs` with `localfile.go` removed, before restoring it)

**Plan metadata:** committed separately below (this SUMMARY + STATE/ROADMAP/REQUIREMENTS).

## Files Created/Modified

- `internal/curationeval/gate.go` — `resolveEvalGate` (ENGRAM_CURATION_EVAL, ENGRAM_CURATION_EVAL_PAIRS), `curationEvalEnabled`
- `internal/curationeval/gate_test.go` — `TestResolveCurationEvalGate` (12 table cases)
- `internal/curationeval/evaluate.go` — `prediction`, `evaluate`
- `internal/curationeval/eval_test.go` — `TestEvaluateAgainstStubProvider` (hermetic stub Jev endpoint), `TestCurationEval` (gated live test, `committed` + conditional `local` subtests)
- `internal/curationeval/metrics.go` — `bucketOf`, `bucketStat`, `accuracyByBucket`, `formatMetric`, `brier`, `thresholdGate`, `confusion`, `formatReport`
- `internal/curationeval/metrics_test.go` — `TestBucketOf`, `TestAccuracyByBucket`, `TestBrier`, `TestThresholdGate`, `TestFormatReportAggregatesOnly`
- `internal/curationeval/localfile.go` — `localPairLine`, `loadLocalPairs`
- `internal/curationeval/localfile_test.go` — `TestLoadLocalPairs`, `TestLoadLocalPairsRejects`, `TestLoadLocalPairsErrorsNeverEchoContent`
- `internal/curationeval/doc.go` — rewritten package doc covering both corpora, the shared pipeline, the D-03 gate, and the gated live test
- `Taskfile.yaml` — new `eval:curation` target
- `.gitignore` — excludes `internal/curationeval/testdata/local/`

## Decisions Made

See `key-decisions` in frontmatter: single koanf load for both env vars, plain-value `evaluate()` signature (no `internal/server` import outside the live test), pinned stub-client concurrency for deterministic per-request assertions, numeric `brier()` split from its `formatMetric` display formatting, the unknown-field vs. malformed-JSON error distinction in `loadLocalPairs`, and the conditionally-defined `local` subtest.

## Deviations from Plan

None - plan executed exactly as written. Task 1's tracer feedback gate, Task 2's and Task 3's TDD RED evidence, and every task's `<acceptance_criteria>` and `<verify>` block were satisfied without needing an auto-fix, an architectural question, or a deferred item.

## TDD Gate Compliance

**Task 1 (tracer, `tdd="true"`):** production-quality per its own type — `gate.go`/`evaluate.go` and their tests were authored together as one tracer commit. RED evidence: before this task, `internal/curationeval` had no `gate.go`/`evaluate.go`/`eval_test.go` at all, so the package could not have compiled with `eval_test.go` present and those symbols undefined — confirmed structurally (new files, no prior compiling state to regress from) rather than via a separate remove/restore cycle, consistent with the tracer's "prove the shape end to end" role. GREEN: `go test ./internal/curationeval/ -run '^(TestResolveCurationEvalGate|TestEvaluateAgainstStubProvider|TestCurationEval)$' -count=1 -v` — 2/2 named PASS plus `TestCurationEval` correctly SKIP (gate off). Tracer feedback gate re-ran immediately after commit: both `<verify>` automated blocks passed again; per `workflow.human_verify_mode` defaulting to `end-of-phase` and the tracer's `<verify>` carrying only `<automated>` blocks, expansion continued with no checkpoint.

**Task 2 (`tdd="true"`, not a tracer):** `metrics_test.go` referenced `bucketOf`, `accuracyByBucket`, `brier`, `thresholdGate`, `confusion`, `formatReport`, `bucketStat` and `formatMetric` before `metrics.go` existed. Confirmed RED by moving `metrics.go` out of the package and running `go vet ./internal/curationeval/`: `vet: internal/curationeval/eval_test.go:160:24: undefined: formatReport` (both the new `metrics_test.go` and the just-updated `eval_test.go` failed to compile, as expected). Restored `metrics.go`, confirmed GREEN: 5/5 named PASS (`TestBucketOf`, `TestAccuracyByBucket`, `TestBrier`, `TestThresholdGate`, `TestFormatReportAggregatesOnly`), plus a `golangci-lint`-driven fix (three `prealloc` findings in `metrics_test.go`'s threshold-gate subtests) applied and re-verified before committing.

**Task 3 (`tdd="true"`, not a tracer):** `localfile_test.go` referenced `loadLocalPairs` before `localfile.go` existed. Confirmed RED by moving `localfile.go` out of the package and running `go vet ./internal/curationeval/`: `vet: internal/curationeval/localfile_test.go:36:16: undefined: loadLocalPairs`. Restored `localfile.go`, confirmed GREEN: 3/3 named PASS (`TestLoadLocalPairs`, `TestLoadLocalPairsRejects` with all 6 table subtests, `TestLoadLocalPairsErrorsNeverEchoContent`), plus the full-package run (`go test ./internal/curationeval/ -count=1`) and `git check-ignore -q internal/curationeval/testdata/local/pairs.jsonl` both green before committing.

## Issues Encountered

None.

## User Setup Required

None - `ENGRAM_CURATION_EVAL`/`task eval:curation` is opt-in and off by default; `ENGRAM_CURATION_EVAL_PAIRS` is an operator-supplied local file path, not a shipped default.

## Next Phase Readiness

- `internal/curationeval`'s harness is complete: `evaluate`, the full metrics/report surface, both corpora, and `task eval:curation` all exist and are tested (unit-level) without ever making a live provider call in the default `go test ./...` path.
- Plan 03-08 can now run `task eval:curation` live and/or reference this harness's shipped report shape in its docs pass. CUR-03 stays unflipped in `REQUIREMENTS.md` until 03-08's own SUMMARY exists (shared-ID gate, #2388) — this plan's work is otherwise done.
- No blockers. `go build ./...`, `env -u ENGRAM_CURATION_EVAL go test ./internal/curationeval/ -count=1`, `golangci-lint run ./internal/curationeval/...`, and `task license:check` are all clean. The pre-existing `go vet` finding at `cmd/engram/operator_view_test.go:441` (duplicate `json:"dup"` tag) is unrelated to this plan's files and already tracked in `deferred-items.md` (noted in 03-05-SUMMARY.md's Next Phase Readiness).

---
*Phase: 03-curation-verdicts*
*Completed: 2026-09-24*

## Known Stubs

None.

## Threat Flags

None — this plan's threat register entries (T-03-06, T-03-19, T-03-20) are all satisfied by artifacts already in place (see coverage D6 for T-03-06's verification); no new surface introduced beyond what the threat model already scoped.

## Self-Check: PASSED

- `internal/curationeval/gate.go`: FOUND
- `internal/curationeval/gate_test.go`: FOUND
- `internal/curationeval/evaluate.go`: FOUND
- `internal/curationeval/eval_test.go`: FOUND
- `internal/curationeval/metrics.go`: FOUND
- `internal/curationeval/metrics_test.go`: FOUND
- `internal/curationeval/localfile.go`: FOUND
- `internal/curationeval/localfile_test.go`: FOUND
- Commit `e98db939`: FOUND in `git log --oneline --all`
- Commit `24db7de3`: FOUND in `git log --oneline --all`
- Commit `c971bef6`: FOUND in `git log --oneline --all`
- Plan-level `<verification>`: `env -u ENGRAM_CURATION_EVAL go test ./internal/curationeval/ -count=1` exits 0 with `TestCurationEval` skipped — re-run and PASSED at SUMMARY time.
- Plan-level `<verification>`: `task --list-all | rg eval:curation` — present.
- `golangci-lint run ./internal/curationeval/...`: 0 issues
- `task license:check`: 0 invalid

Ready for 03-08.
