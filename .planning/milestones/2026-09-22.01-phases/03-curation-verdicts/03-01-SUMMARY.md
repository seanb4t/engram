---
phase: 03-curation-verdicts
plan: 01
subsystem: curation
tags: [verdicts, jev, decide, spine-review-consolidate, qdrant, tdd]

# Dependency graph
requires:
  - phase: 02-decision-interface-jev-backend
    provides: "internal/decide (Decider/DecideMany/Choice/Noul/Status), internal/decide/jev (real HTTP client against {base}/alpha/decisions), internal/server.deciderFromConfig"
provides:
  - "internal/verdict package: the D-06 question set, D-09 per-record state truncation, pair ordering, and decide.Result -> Verdict mapping, shared by consolidate and the future curation eval"
  - "Store.RecordStates: a budgeted, Subject-less batched-by-id state fetch (content/summary/created_at)"
  - "server.StoreAndDeciderFromEnv: the store+decider+VerdictSettings wiring seam for the verdict pass, nil decider when no provider configured"
  - "spine-review consolidate --output json carries a nested advisory verdict per candidate pair (relation, probabilities, same_subject, needs_review, model) when a decider is configured; byte-identical output when it isn't"
affects: ["03-02 (registers the verdict-threshold/state-chars knobs verdictSettings falls back to)", "03-05 (extends verdictSettings to read those knobs)", "03-06 (text-lane rendering of the nested verdict object)", "03-04/03-07/03-08 (curation eval, consumes internal/verdict directly)"]

# Actuals (#2632)
actuals:
  tokens: 13890
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Budgeted-read view pair: verdictStateView()/verdictStateRecordCeiling(), same shape as citationsView/summaryView (internal/store/boundedread.go)"
    - "StoreAndXFromEnv combined-constructor wiring seam (StoreAndDeciderFromEnv, mirroring StoreAndSummarizerFromEnv) but without its \"feature unset means error\" branch — D-04's off-by-default contract"
    - "Two-shape JSON union via custom MarshalJSON (consolidateVerdictDoc: success fields vs. a lone error key), rather than field-level omitempty"
    - "Advisory-only / never-mutates: spineConsolidateStore interface exposes only NearDuplicates + RecordStates, proven by TestConsolidateStoreSurfaceIsReadOnly (reflection over the interface's method set)"

key-files:
  created:
    - internal/verdict/verdict.go
    - internal/verdict/verdict_test.go
    - internal/store/verdictstate.go
    - internal/store/verdictstate_test.go
  modified:
    - internal/store/boundedread.go
    - internal/server/decider.go
    - internal/server/decider_test.go
    - cmd/engram/spine_review_consolidate.go
    - cmd/engram/spine_review_consolidate_test.go

key-decisions:
  - "FromResult's malformed-answer validation was deliberately deferred out of Task 1's tracer implementation and added in Task 3, per the plan's own TDD sequencing (RED first against the tracer's already-shipped code) — Task 1's FromResult mapped an answer verbatim with no validation; Task 3 rejects an unrecognized choice, an incomplete probability set, a missing answer, or a wrong answer type, all through decide.ErrDecisionMalformedResponse (no new error class)."
  - "verdictSettings(cfg) ignores cfg entirely today (returns verdict.DefaultThreshold/DefaultStateChars), with an explicit //nolint:unparam,revive — plan 03-05 wires the registered knobs into this same function; the signature is fixed now so StoreAndDeciderFromEnv never needs a second signature change."
  - "runVerdictPass fetches every id referenced by the pair set through exactly ONE Store.RecordStates call, then sends every buildable request through exactly ONE DecideMany call — no per-pair state fetch and no per-pair decide call, matching D-11's bounded-concurrency-not-cap contract."
  - "Deferred (out of scope, logged to deferred-items.md): a pre-existing, unrelated go vet finding in cmd/engram/operator_view_test.go (a deliberate nolint:govet duplicate-json-tag probe, invisible to golangci-lint but visible to raw go vet). Not touched — outside plan 03-01's files_modified, reproduces identically on a clean checkout."

requirements-completed: [CUR-01, CUR-02]
# Both requirements are ALSO declared by not-yet-executed sibling plans in this
# phase (03-02/03-05 for CUR-02, 03-06 for CUR-01) — the shared-ID gate
# (`requirements.ready-ids`) correctly reports 0/2 ready and REQUIREMENTS.md
# checkboxes are intentionally NOT flipped by this plan. Listed here per the
# template's "copy the plan's requirements frontmatter verbatim" contract.

coverage:
  - id: D1
    description: "With a decider configured, spine-review consolidate --all-scopes --output json fetches both records' state through Store.RecordStates, sends ONE Decisions request per candidate pair carrying the five-option relation Choice and the same_subject Noul, and each candidate row gains a nested verdict object with exactly the five D-05 keys"
    requirement: CUR-01
    verification:
      - kind: integration
        ref: "cmd/engram/spine_review_consolidate_test.go#TestSpineReviewConsolidateVerdictTracer"
        status: pass
    human_judgment: false
  - id: D2
    description: "With no decider configured, consolidate's stdout is byte-identical to json.Marshal(consolidateDoc(...)) plus a newline, contains no verdict key, and RecordStates is never called (D-04)"
    requirement: CUR-01
    verification:
      - kind: integration
        ref: "cmd/engram/spine_review_consolidate_test.go#TestSpineReviewConsolidateNoProviderByteIdentical"
        status: pass
    human_judgment: false
  - id: D3
    description: "spineConsolidateStore exposes only read methods (NearDuplicates, RecordStates) — no mutating store method is reachable from the verdict pass (T-03-02)"
    requirement: CUR-02
    verification:
      - kind: unit
        ref: "cmd/engram/spine_review_consolidate_test.go#TestConsolidateStoreSurfaceIsReadOnly"
        status: pass
    human_judgment: false
  - id: D4
    description: "A failed pair decision renders verdict: {\"error\": \"<class>\"} with exactly one key; the sweep still exits 0 (D-10)"
    requirement: CUR-01
    verification:
      - kind: unit
        ref: "internal/verdict/verdict_test.go#TestFromResultMalformed"
        status: pass
      - kind: unit
        ref: "internal/verdict/verdict_test.go#TestErrorClass"
        status: pass
    human_judgment: false
  - id: D5
    description: "needs_review is probabilities[relation] strictly below threshold; no probability is rounded, clamped or renormalized (D-05, D-08, CUR-02 boundary/precision)"
    requirement: CUR-02
    verification:
      - kind: unit
        ref: "internal/verdict/verdict_test.go#TestFromResultThresholdBoundary"
        status: pass
      - kind: unit
        ref: "internal/verdict/verdict_test.go#TestFromResultCarriesValuesVerbatim"
        status: pass
    human_judgment: false
  - id: D6
    description: "Per-record state is the stored summary (when non-empty) followed by the head of content, truncated to N Unicode code points; truncation never splits a multi-byte UTF-8 sequence, invalid UTF-8 becomes U+FFFD (D-09, CUR-01 encoding)"
    requirement: CUR-01
    verification:
      - kind: unit
        ref: "internal/verdict/verdict_test.go#TestStateTruncation"
        status: pass
    human_judgment: false
  - id: D7
    description: "The more recently created record of a pair is always sent as record_b (tie: lexically larger id), regardless of the pair's A/B order"
    requirement: CUR-01
    verification:
      - kind: unit
        ref: "internal/verdict/verdict_test.go#TestPairRequestOrdersNewerSecond"
        status: pass
    human_judgment: false
  - id: D8
    description: "Store.RecordStates is a budgeted, Subject-less batched-by-id fetch: 82432-byte ceiling / 25 records per RPC at default caps, zero calls for zero ids, spans scopes and archived/superseded records, issues no write RPC"
    requirement: CUR-01
    verification:
      - kind: integration
        ref: "internal/store/verdictstate_test.go#TestVerdictStateViewCeiling"
        status: pass
      - kind: integration
        ref: "internal/store/verdictstate_test.go#TestRecordStatesFetchesState"
        status: pass
      - kind: integration
        ref: "internal/store/verdictstate_test.go#TestRecordStatesEmptyIDs"
        status: pass
      - kind: integration
        ref: "internal/store/verdictstate_test.go#TestRecordStatesSpansScopesAndStates"
        status: pass
      - kind: integration
        ref: "internal/store/verdictstate_test.go#TestRecordStatesDoesNotMutate"
        status: pass
    human_judgment: false
  - id: D9
    description: "StoreAndDeciderFromEnv loads config exactly once and returns a nil decider (never an error) when ENGRAM_DECISIONS_PROVIDER is unset"
    requirement: CUR-01
    verification:
      - kind: integration
        ref: "internal/server/decider_test.go#TestStoreAndDeciderFromEnv"
        status: pass
    human_judgment: false

# Metrics
duration: 1h 11m
completed: 2026-09-24
status: complete
commits: 3
plan_head_before: 50cdd75207b3b05a9d0590324e2fa8339e2a6cdd
---

# Phase 3 Plan 1: End-to-End Curation Verdicts Summary

**Advisory relation verdicts (duplicate/contradicts/updates/related/unrelated + same-subject) now flow from `spine-review consolidate` through a real Jev client to a nested `verdict` JSON object per candidate pair, with a byte-identical no-provider fallback.**

## Performance

- **Duration:** 1h 11m
- **Started:** 2026-09-24T00:12:13Z
- **Completed:** 2026-09-24T01:23:32Z
- **Tasks:** 3
- **Files modified:** 9 (4 created, 5 modified)

## Accomplishments

- New `internal/verdict` package: the D-06 question set (relation Choice over five options, same_subject Noul), D-09 per-record state truncation (Unicode-code-point-safe), newer-record-is-record_b pair ordering, and `FromResult` mapping a `decide.Result` to a `Verdict` — including malformed-answer rejection (unrecognized choice, incomplete probability set, missing/wrong-typed answer) through the existing `decide.ErrDecisionMalformedResponse` class.
- `Store.RecordStates`: a budgeted (82432 bytes / 25 records per RPC at default caps), Subject-less batched-by-id fetch reusing `fetchPayloadsByID`, spanning every scope and archived/superseded record, issuing no write RPC.
- `server.StoreAndDeciderFromEnv`: the single-config-load wiring seam for the verdict pass — a nil decider (never an error) when `ENGRAM_DECISIONS_PROVIDER` is unset (D-04).
- `spine-review consolidate --output json` runs the verdict pass when a decider is configured: one `RecordStates` call for every id in the candidate set, one `DecideMany` call for every buildable pair, a nested `verdict` object per candidate row, and a top-level `verdict_threshold`. The no-provider path stays byte-identical to `json.Marshal(consolidateDoc(...))`.

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end verdict on one candidate pair** - `f3d51a42` (feat, tracer)
2. **Task 2: State fetch guarantees** - `83c9bca9` (test)
3. **Task 3: Verdict edges** - `3944d596` (test)

_Note: TDD tasks 1 and 3 each carry their own RED evidence documented below rather than a separate test-only commit — the plan's own instructions left commit granularity to the executor ("commit order is free")._

## Files Created/Modified

- `internal/verdict/verdict.go` - relation constants/criteria, question builders, `State`/`PairRequest`/`NewRequest`, `Probabilities`, `Verdict`, `FromResult`, `ErrorClass`/`Unavailable`
- `internal/verdict/verdict_test.go` - the edge suite (question set, truncation, pair ordering, threshold boundary, verbatim carry, malformed answers, error classes)
- `internal/store/verdictstate.go` - `RecordState`, `Store.RecordStates`
- `internal/store/verdictstate_test.go` - ceiling, empty-input, span, no-mutation and fetch tests
- `internal/store/boundedread.go` - `verdictStateRecordCeiling`, `(*Store).verdictStateView`
- `internal/server/decider.go` - `VerdictSettings`, `verdictSettings`, `StoreAndDeciderFromEnv`
- `internal/server/decider_test.go` - `TestStoreAndDeciderFromEnv`
- `cmd/engram/spine_review_consolidate.go` - `spineConsolidateStore.RecordStates`, `spineConsolidateStoreFromEnv` (new 4-return signature), `consolidateVerdictDoc`/`consolidateProbabilitiesDoc` (custom `MarshalJSON` two-shape union), `consolidatePairDoc.Verdict`, `consolidateReportDoc.VerdictThreshold`, `runVerdictPass`, `attachVerdicts`
- `cmd/engram/spine_review_consolidate_test.go` - `spineConsolidateFakeStore` states/statesErr/call-counter, `withFakeConsolidateStoreAndDecider`, the three new Task 1 tests

## Decisions Made

- FromResult's malformed-answer validation was deliberately deferred out of Task 1 (a verbatim, unvalidated mapping) and added in Task 3 per the plan's TDD sequencing — see RED evidence below.
- `verdictSettings(cfg)` ignores `cfg` today (defaults only), carrying an explicit `//nolint:unparam,revive` — plan 03-05 wires the registered knobs into this same function without a second signature change.
- `runVerdictPass` makes exactly one `RecordStates` call and one `DecideMany` call per consolidate invocation, regardless of pair count (D-11: bounded concurrency, never a per-pair call).
- CUR-01/CUR-02 are NOT checked off in REQUIREMENTS.md by this plan: both are also declared by not-yet-executed sibling plans (03-02/03-05, 03-06), and `requirements.ready-ids` correctly reports 0/2 ready under the shared-ID gate (#2388).

## Deviations from Plan

None - plan executed exactly as written. (See "Known Issues" below for one pre-existing, out-of-scope condition surfaced during verification — not a deviation from this plan's own work.)

## TDD Gate Compliance

**Task 1 (tracer, tdd="true"):** `TestSpineReviewConsolidateVerdictTracer` and its sibling tracer tests could not exist before this task's RunE wiring, `spineConsolidateStoreFromEnv`'s new 4-return signature, and `consolidatePairDoc.Verdict` — the compile-level RED is the tracer's own pre-existing-code absence: the fake store's `NearDuplicates`-only interface, the old 2-return `spineConsolidateStoreFromEnv`, and `consolidateDoc` with no `Verdict` field. Task 1 landed the wiring, the new tests, and the fake-store extension together (a tracer task, per the plan, is production-quality and committed as one `feat` — not split into separate RED/GREEN commits).

**Task 3 (`tdd="true"`, explicit RED-first):** `internal/verdict/verdict_test.go` was written and run FIRST against Task 1's already-committed `FromResult`. Confirmed RED — `TestFromResultMalformed` failed all 5 subtests:

```
verdict_test.go:286: FromResult(unknown choice) = {Relation:bogus ... NeedsReview:true ErrorClass:}, want Failed()
verdict_test.go:286: FromResult(missing probability) = {Relation:duplicate ... ErrorClass:}, want Failed()
verdict_test.go:286: FromResult(missing relation answer) = {Relation: ... NeedsReview:true ErrorClass:}, want Failed()
verdict_test.go:286: FromResult(missing same_subject answer) = {Relation:duplicate ... ErrorClass:}, want Failed()
verdict_test.go:286: FromResult(same_subject wrong type) = {Relation:duplicate ... ErrorClass:}, want Failed()
--- FAIL: TestFromResultMalformed (0.00s)
```

All other Task 3 tests (`TestRelationQuestionSet`, `TestStateTruncation`, `TestPairRequestOrdersNewerSecond`, `TestFromResultThresholdBoundary`, `TestFromResultCarriesValuesVerbatim`, `TestErrorClass`) passed immediately against Task 1's code — they pin behavior Task 1 already implemented correctly. `FromResult` was then extended with validation (unrecognized `Choice`, incomplete probability set, missing/wrong-typed answer → `decide.ErrDecisionMalformedResponse`, no new error class) and the full suite went GREEN, confirmed with `-race` and 7/7 named PASS lines.

## Issues Encountered

None.

## Known Issues (pre-existing, out of scope)

- `go vet ./...` reports one pre-existing finding unrelated to this plan: `cmd/engram/operator_view_test.go:441` (`struct field B repeats json tag "dup"`), a deliberate `//nolint:govet` adjacency-edge probe that raw `go vet` cannot see past but `golangci-lint` (this project's actual gate) correctly suppresses — `golangci-lint run ./...` reports 0 issues. Not touched (file is outside plan 03-01's `files_modified`); logged to `.planning/phases/03-curation-verdicts/deferred-items.md` per the executor's scope-boundary rule.

## User Setup Required

None - no external service configuration required. (`ENGRAM_DECISIONS_PROVIDER`/`ENGRAM_DECISIONS_BASE_URL` etc. were already registered and documented in Phase 2; this plan adds no new config surface.)

## Next Phase Readiness

- `internal/verdict`, `Store.RecordStates` and `server.StoreAndDeciderFromEnv` are the shared foundation plans 03-02 through 03-08 build on directly (registered knobs, `--no-verdicts`, text-lane rendering, docs, and the curation eval harness).
- No blockers. `go build ./...`, `go vet` (scoped to this plan's own packages), `golangci-lint run ./...`, and `task license:check` are all clean; the full `internal/verdict`, `internal/store`, `internal/server` and `cmd/engram` test suites pass with no regressions.

---
*Phase: 03-curation-verdicts*
*Completed: 2026-09-24*

## Self-Check: PASSED

All 9 created/modified source and doc files verified present on disk; all 3 task commit hashes (`f3d51a42`, `83c9bca9`, `3944d596`) verified present in `git log --oneline --all`.
