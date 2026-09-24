---
phase: 03-curation-verdicts
plan: 05
subsystem: cli
tags: [verdicts, jev, decide, config, spine-review-consolidate, tdd]

# Dependency graph
requires:
  - phase: 03-curation-verdicts
    provides: "03-01: internal/verdict, Store.RecordStates, server.StoreAndDeciderFromEnv, runVerdictPass/attachVerdicts; 03-02: DecisionsConfig.VerdictThreshold/VerdictStateChars + config.ParseProbability; 03-03: consolidate's RunE ordering (scope guard first)"
provides:
  - "server.VerdictSettings gains Provider/Model/EndpointHost; verdictSettings resolves the registered ENGRAM_DECISIONS_VERDICT_THRESHOLD/STATE_CHARS knobs via config.ParseProbability/ParsePositiveIntCap, warn-and-default on an unparseable value"
  - "server.DeciderFromEnv: store-free decider+settings wiring seam for the curation eval (plan 03-07)"
  - "consolidate's --verdict-threshold (per-run override) and --no-verdicts (opt-out) flags"
  - "the D-10 disclosure/summary stderr lines, the all-auth WARNING, and the advisory headline clause"
  - "--help, the command's doc comment, and the blast-radius row all state the advisory-verdict contract; goldens regenerated"
affects: ["03-06 (text-lane rendering touches the same spine_review_consolidate.go file)", "03-07 (curation eval consumes server.DeciderFromEnv directly)", "03-08 (docs pass reads consolidate's shipped --help/flags)"]

# Actuals (#2632)
actuals:
  tokens: 14177
  tasks: 3
  commits: 3
plan_head_before: d13da657e224eeb539d592752fa982a9ce5f44e7

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "verdictSettings resolves BOTH registered knobs through the SAME exported parsers Config.Validate uses (config.ParseProbability/config.ParsePositiveIntCap), and now also carries Provider/Model/EndpointHost unconditionally — a disclosure line needs them even though StateChars/Threshold are the only two knobs D-08/D-09 gate"
    - "String flag with empty-means-registered-value, mirroring parseMinScore's own doc-comment discipline: --verdict-threshold's DefValue is never a bogus numeric default"
    - "verdictOutcome tally struct (Requested/Answered/NeedsReview/FailuresByClass) shared by the headline clause, the stderr summary line and the all-auth warning — one reduction over []verdict.Verdict, three renderers"
    - "In-file fake decide.Decider whose DecideMany delegates to the real decide.DecideMany with a per-request function (blockingFakeDecider), so a --timeout deadline test exercises production ctx-check/concurrency behavior, not a hand-rolled substitute"

key-files:
  created: []
  modified:
    - internal/server/decider.go
    - internal/server/decider_test.go
    - cmd/engram/spine_review_consolidate.go
    - cmd/engram/spine_review_consolidate_test.go
    - internal/surfaces/toolclass.go
    - cmd/engram/testdata/help.golden
    - cmd/engram/testdata/catalog.golden

key-decisions:
  - "verdictSettings copies Provider/Model/EndpointHost from cfg.Decisions unconditionally (no provider gate) — Task 2's disclosure line needs them regardless of which knob path resolved Threshold/StateChars; this meant the PRE-EXISTING TestStoreAndDeciderFromEnv/provider_unset assertion (03-01) had to move from full-struct equality to per-field checks, since VerdictSettings now carries the registry's default model string even with no provider configured."
  - "Disclosure, summary line, headline clause and the all-auth warning are all gated together under `dec != nil && !spineConsolidateNoVerdicts`; the disclosure line is additionally gated on `len(pairs) > 0` per the plan's own wording — an empty candidate set never prints a phantom 'requesting verdicts for 0 pairs' line."
  - "Corrected a stale per-plan commit ledger (.git/gsd-plan-head-before-03-05) at SUMMARY time — same class of pre-existing artifact noted in 03-03-SUMMARY.md's Issues Encountered, left over from an earlier milestone's use of the same phase/plan numbering. Verified the true pre-plan HEAD via `ad4e29b1^` == `d13da657` (the prior plan's own metadata commit) before recording actuals, rather than trusting the stale value."
  - "CUR-02 marked complete in REQUIREMENTS.md via the shared-ID gate (`requirements.ready-ids`): this plan and 03-02 were its only two declaring plans, both now finished. CUR-01 stays unflipped — plan 03-06 also declares it and has not executed yet."

requirements-completed: [CUR-01, CUR-02]
# Per the template's "copy verbatim" contract this lists the plan's own
# `requirements:` frontmatter. Only CUR-02 was actually flipped in
# REQUIREMENTS.md this plan (see key-decisions above) — CUR-01 is shared
# with not-yet-executed sibling plan 03-06 and the shared-ID gate (#2388)
# correctly left it unflipped, reporting 1/2 ready.

coverage:
  - id: D1
    description: "The registered ENGRAM_DECISIONS_VERDICT_THRESHOLD/STATE_CHARS knobs resolve through server.verdictSettings via the shared config.ParseProbability/ParsePositiveIntCap parsers, warn-and-default on an unparseable value, and --verdict-threshold overrides the threshold for one run; needs_review and the top-level verdict_threshold reflect the resolved value end to end"
    requirement: CUR-02
    verification:
      - kind: unit
        ref: "internal/server/decider_test.go#TestVerdictSettingsResolvesKnobs"
        status: pass
      - kind: integration
        ref: "cmd/engram/spine_review_consolidate_test.go#TestSpineReviewConsolidateVerdictThresholdFlag"
        status: pass
    human_judgment: false
  - id: D2
    description: "With both vars empty, config.Load(nil)'s registered defaults resolve through verdictSettings to exactly verdict.DefaultThreshold/verdict.DefaultStateChars — the registry default and the package fallback cannot drift"
    requirement: CUR-02
    verification:
      - kind: unit
        ref: "internal/server/decider_test.go#TestVerdictSettingsDefaultsMatchVerdictPackage"
        status: pass
    human_judgment: false
  - id: D3
    description: "--verdict-threshold values -0.1, 1.5, NaN and abc exit exitUsage naming the flag, before any store or decider construction; --verdict-threshold with no provider configured changes nothing in stdout"
    requirement: CUR-02
    verification:
      - kind: integration
        ref: "cmd/engram/spine_review_consolidate_test.go#TestSpineReviewConsolidateVerdictThresholdFlag"
        status: pass
    human_judgment: false
  - id: D4
    description: "--no-verdicts with a provider configured makes zero RecordStates and zero decider calls, prints no verdict stderr lines, and stdout is byte-identical to the no-provider run"
    requirement: CUR-01
    verification:
      - kind: integration
        ref: "cmd/engram/spine_review_consolidate_test.go#TestSpineReviewConsolidateNoVerdictsSuppresses"
        status: pass
    human_judgment: false
  - id: D5
    description: "Before sending, consolidate prints one stderr disclosure line naming provider, model, endpoint host, the per-record character bound and --no-verdicts; after the pass it prints one stderr summary line with requested/answered/needs-review/unavailable counts plus per-class failure counts in sorted order"
    requirement: CUR-01
    verification:
      - kind: integration
        ref: "cmd/engram/spine_review_consolidate_test.go#TestSpineReviewConsolidateFailurePolicy"
        status: pass
    human_judgment: false
  - id: D6
    description: "Mixed failures (auth, missing state, timeout) alongside a success produce per-pair verdict:{error:<class>} objects, correct summary counts, and exit status 0; an all-auth failure set prints an additional loud WARNING naming both key env vars, still exiting 0"
    requirement: CUR-01
    verification:
      - kind: integration
        ref: "cmd/engram/spine_review_consolidate_test.go#TestSpineReviewConsolidateFailurePolicy"
        status: pass
      - kind: integration
        ref: "cmd/engram/spine_review_consolidate_test.go#TestSpineReviewConsolidateAllAuthFailureWarns"
        status: pass
    human_judgment: false
  - id: D7
    description: "A --timeout deadline reached during the verdict pass leaves every structural candidate present, each unanswered pair classed timeout, and exits 0; a state-fetch error degrades every pair to state_unavailable without calling the decider"
    requirement: CUR-01
    verification:
      - kind: integration
        ref: "cmd/engram/spine_review_consolidate_test.go#TestSpineReviewConsolidateDeadlineDuringVerdictPass"
        status: pass
      - kind: integration
        ref: "cmd/engram/spine_review_consolidate_test.go#TestSpineReviewConsolidateStateFetchErrorDegrades"
        status: pass
    human_judgment: false
  - id: D8
    description: "250 candidate pairs produce exactly 250 requests in one DecideMany call — no cap on pair count (D-11)"
    requirement: CUR-01
    verification:
      - kind: integration
        ref: "cmd/engram/spine_review_consolidate_test.go#TestSpineReviewConsolidateNoCapOnPairs"
        status: pass
    human_judgment: false
  - id: D9
    description: "When the pass ran, the text headline appends a clause stating verdicts are advisory and never merge or mutate, with answered/needs-review/unavailable counts; with no pass the headline carries none of that vocabulary"
    requirement: CUR-01
    verification:
      - kind: unit
        ref: "cmd/engram/spine_review_consolidate_test.go#TestVerdictHeadlineClause"
        status: pass
    human_judgment: false
  - id: D10
    description: "consolidate's --help (Long text and flag usages) states that verdicts run by default only when ENGRAM_DECISIONS_PROVIDER is set, what each request sends, --no-verdicts, --verdict-threshold and its registered default, needs_review's meaning, and that consolidate never merges or mutates; the blast-radius row's comment records the pass's outbound network effect while its Class values (ReadOnly/non-destructive/idempotent/OpenWorld-false) stay unchanged"
    requirement: CUR-01
    verification:
      - kind: integration
        ref: "cmd/engram/golden_test.go#TestHelpGolden"
        status: pass
      - kind: integration
        ref: "cmd/engram/golden_test.go#TestCatalogGolden"
        status: pass
      - kind: other
        ref: "rg phrase check over cmd/engram/testdata/help.golden (no-verdicts, verdict-threshold, ENGRAM_DECISIONS_PROVIDER, needs_review, never merges or mutates — all present) + rg over internal/surfaces/toolclass.go confirming Class{...} unchanged"
        status: pass
    human_judgment: false

# Metrics
duration: 49min
completed: 2026-09-24
status: complete
commits: 3
---

# Phase 3 Plan 5: Verdict Operator Contract Summary

**`spine-review consolidate` now exposes the full operator-facing verdict contract: registered threshold/state-length knobs plus a per-run `--verdict-threshold` override, a `--no-verdicts` opt-out, a pre-send disclosure line and a post-pass summary line with per-class failure counts, a loud all-auth WARNING, an advisory headline clause, and `--help`/blast-radius text describing all of it — with goldens regenerated.**

## Performance

- **Duration:** 49 min
- **Started:** 2026-09-24T02:58:00Z
- **Completed:** 2026-09-24T03:46:57Z
- **Tasks:** 3
- **Files modified:** 7

## Accomplishments

- `server.VerdictSettings` gains `Provider`, `Model` and `EndpointHost`; `verdictSettings` now resolves `ENGRAM_DECISIONS_VERDICT_THRESHOLD`/`ENGRAM_DECISIONS_VERDICT_STATE_CHARS` through the shared `config.ParseProbability`/`config.ParsePositiveIntCap` parsers (warn-and-default on an unparseable value), and `server.DeciderFromEnv` gives the curation eval (plan 03-07) a store-free decider+settings seam.
- `--verdict-threshold` (string, empty means the registered value) overrides the threshold for one run, parsed after the scope/output/min-score gates and before any store or decider construction; an invalid value exits `exitUsage` naming the flag, and the constructor is never reached.
- `--no-verdicts` skips the entire pass — zero `RecordStates` calls, zero decider calls, byte-identical stdout to the no-provider path, no verdict stderr lines.
- One stderr disclosure line before sending (provider, model, endpoint host, the per-record character bound, and that `--no-verdicts` skips it) and one summary line after (requested/answered/needs_review/unavailable plus sorted per-class failure counts); an extra `WARNING:` line fires only when every request failed authentication. None of this changes the sweep's exit status.
- The headline gains an advisory-framing clause (counts included) only when the pass ran; `--help`, the command's Go doc comment, and the blast-radius row in `internal/surfaces/toolclass.go` all state the contract; `help.golden`/`catalog.golden` regenerated via `task surfaces:gen`, diff confined to consolidate's `Long` and its two new flags.

## Task Commits

Each task was committed atomically:

1. **Task 1: Registered knobs and --verdict-threshold reach needs_review end to end; DeciderFromEnv for the eval** - `ad4e29b1` (feat, tracer/tdd)
2. **Task 2: --no-verdicts, disclosure and summary lines, failure degradation, no cap, headline clause** - `b2e08ad3` (feat, tdd)
3. **Task 3: Help text, doc comments and regenerated goldens** - `b70352a9` (docs)

**Plan metadata:** committed separately below.

_Note: this plan's own instructions left commit granularity to the executor ("commit as feat(...)"), and each task was implemented and committed as one production-quality commit — Task 1 is a tracer per its own type, Tasks 2/3 carry `tdd="true"`/prose respectively with RED evidence documented in TDD Gate Compliance below rather than a separate test-only commit._

## Files Created/Modified

- `internal/server/decider.go` - `VerdictSettings.Provider/Model/EndpointHost`; rewritten `verdictSettings`; new `DeciderFromEnv`
- `internal/server/decider_test.go` - `TestVerdictSettingsDefaultsMatchVerdictPackage`, `TestVerdictSettingsResolvesKnobs`, `TestVerdictSettingsEndpointHostOnly`, `TestDeciderFromEnv`; `TestStoreAndDeciderFromEnv`'s provider-unset assertion moved to per-field checks
- `cmd/engram/spine_review_consolidate.go` - `--verdict-threshold`/`parseVerdictThreshold`, `--no-verdicts`, `verdictOutcome`/`verdictStats`/`verdictHeadlineClause`/`verdictDisclosureLine`/`verdictSummaryLine`/`verdictAllAuthWarning`, the command's `Long` text and rewritten doc comment
- `cmd/engram/spine_review_consolidate_test.go` - `TestSpineReviewConsolidateVerdictThresholdFlag`, `TestSpineReviewConsolidateNoVerdictsSuppresses`, `TestSpineReviewConsolidateFailurePolicy`, `TestSpineReviewConsolidateAllAuthFailureWarns`, `TestSpineReviewConsolidateStateFetchErrorDegrades`, `TestSpineReviewConsolidateDeadlineDuringVerdictPass`, `TestSpineReviewConsolidateNoCapOnPairs`, `TestVerdictHeadlineClause`, and the scripted/counting/blocking fake deciders they share
- `internal/surfaces/toolclass.go` - consolidate row's comment extended to describe the advisory verdict pass's outbound network effect (`Class` values untouched)
- `cmd/engram/testdata/help.golden`, `cmd/engram/testdata/catalog.golden` - regenerated via `task surfaces:gen`

## Decisions Made

- `verdictSettings` copies `Provider`/`Model`/`EndpointHost` from `cfg.Decisions` unconditionally (no provider gate), since the disclosure line needs them regardless of which knob path resolved `Threshold`/`StateChars`. This required moving the pre-existing `TestStoreAndDeciderFromEnv`/`provider_unset` subtest (03-01) from full-struct equality to per-field checks, since `VerdictSettings` now carries the registry's default model string even with no provider configured — not a regression, a widened struct whose old exact-equality test was too strict for its own stated scope.
- Disclosure, summary line, headline clause and the all-auth warning are all gated together under `dec != nil && !spineConsolidateNoVerdicts`; the disclosure line is additionally gated on `len(pairs) > 0` so an empty candidate set never prints a phantom "requesting verdicts for 0 pairs" line.
- Corrected a stale per-plan commit ledger (`.git/gsd-plan-head-before-03-05`) at SUMMARY time — the same class of leftover artifact 03-03-SUMMARY.md's "Issues Encountered" already documented, from an earlier milestone reusing this phase/plan numbering. Verified the true pre-plan HEAD (`ad4e29b1^` == `d13da657`, the prior plan's own metadata commit) before recording `commits`/`plan_head_before` below.
- CUR-02 marked complete via the shared-ID gate; CUR-01 stays unflipped pending sibling plan 03-06.

## Deviations from Plan

None - plan executed exactly as written.

## TDD Gate Compliance

**Task 1 (tracer, `tdd="true"`):** the four `internal/server` tests and `TestSpineReviewConsolidateVerdictThresholdFlag` could not pass before this task's edits — `verdictSettings` was a no-op returning only package defaults (no `Provider`/`Model`/`EndpointHost` fields existed on `VerdictSettings`, so the endpoint-host and knob-resolution tests could not even compile), `DeciderFromEnv` did not exist, and `--verdict-threshold` was not a registered flag. Confirmed RED by running the target tests against the pre-edit tree (compile failure: undefined field/function). Implemented the resolver rewrite, `DeciderFromEnv`, the flag and its RunE wiring together as one tracer commit (production-quality, per the plan's own instruction), confirmed GREEN: 5/5 named PASS in `internal/server`, 3/3 named PASS in `cmd/engram` for the target set, plus the full `TestStoreAndDeciderFromEnv`/`TestSpineReviewConsolidateVerdictTracer`/`TestSpineReviewConsolidateNoProviderByteIdentical` re-run clean.

**Tracer feedback gate:** re-ran Task 1's full `<verify>` (both automated blocks) immediately after commit, before starting Task 2 — both passed. Per `workflow.human_verify_mode` defaulting to `end-of-phase` and the tracer's `<verify>` carrying only `<automated>` blocks (no `<human-check>`), expansion continued without a checkpoint.

**Task 2 (`tdd="true"`, not a tracer):** all seven target tests reference production symbols (`spineConsolidateNoVerdicts`, `verdictOutcome`, `verdictStats`, `verdictHeadlineClause`, `verdictDisclosureLine`, `verdictSummaryLine`, `verdictAllAuthWarning`) that did not exist before this task — confirmed RED via the pre-edit tree (undefined symbols, compile failure) before writing any of Task 2's production code. Implemented the opt-out flag, the six pure/stderr functions, and RunE's gating together, confirmed GREEN: 7/7 named PASS with `-race`, plus the full `go test ./cmd/engram/ -count=1` run showing only the two anticipated golden-drift failures (`TestHelpGolden`/`TestCatalogGolden` — expected until Task 3's `task surfaces:gen`, not a regression in any consolidate/sweep-scope/operator-output test).

## Issues Encountered

A stale per-plan commit ledger (`.git/gsd-plan-head-before-03-05`) was found at SUMMARY time, carrying a commit hash from an earlier milestone's use of the same phase/plan numbering (the identical class of issue 03-03-SUMMARY.md already documented for `03-03`'s own ledger). Corrected to the verified true pre-plan HEAD (`d13da657`, confirmed via `git rev-parse ad4e29b1^`) before computing `commits`/`plan_head_before` above.

Task 1's initial `Long` text draft for `spineReviewConsolidateCmd` split the required literal phrase "never merges or mutates" across a line-wrap boundary (`\n`), which the `help.golden` phrase-check acceptance criterion (`rg -o -F`) correctly flagged as absent (0 occurrences) on the first `task surfaces:gen` pass. Fixed by keeping the phrase on one unbroken line and regenerating; the second pass confirmed all five required phrases present.

## User Setup Required

None - no external service configuration required. Both new flags are opt-in per-run CLI behavior; the two underlying env knobs (`ENGRAM_DECISIONS_VERDICT_THRESHOLD`/`_STATE_CHARS`) were already registered and documented in plan 03-02.

## Next Phase Readiness

- `server.VerdictSettings`'s widened shape and `server.DeciderFromEnv` are the exact seam plan 03-07's curation eval needs (a store-free decider+settings constructor).
- Plan 03-06 (text-lane rendering of consolidate's verdict object) touches the same file (`spine_review_consolidate.go`) next — no conflict expected: this plan's changes are confined to the JSON-mode contract, the disclosure/summary stderr lines, and help/doc text; 03-06's rendering work is additive on top.
- No blockers. `go build ./...`, `go vet` (scoped, excluding the pre-existing `operator_view_test.go:441` finding tracked in `deferred-items.md`), `golangci-lint run ./internal/server/... ./cmd/engram/... ./internal/surfaces/...`, and `task surfaces:gen` (confirmed a no-op after the final commit) are all clean; `go test ./internal/server/ ./cmd/engram/ ./internal/surfaces/... -count=1` passes with no regressions.

---
*Phase: 03-curation-verdicts*
*Completed: 2026-09-24*

## Self-Check: PASSED

All 7 modified source/golden files and the SUMMARY.md itself verified present on disk with `[ -f ]`; all 3 task commit hashes (`ad4e29b1`, `b2e08ad3`, `b70352a9`) verified present in `git log --oneline --all`; the plan-level `<verification>` (`go test ./internal/server/ ./cmd/engram/ ./internal/surfaces/... -count=1` and `task surfaces:gen` leaving `git status --porcelain` empty) re-run and passing at SUMMARY time.
