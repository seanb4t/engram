---
phase: 04-drift-detection-read-only
plan: 01
subsystem: cli
tags: [mcp, drift-detection, json-decoding, redaction, codex-cli, go]

# Dependency graph
requires:
  - phase: 03-plugin-first-delivery
    provides: "PluginRuntime optional-interface precedent (plugin.go) this phase's DriftRuntime interface mirrors verbatim; Plan.Probe already authored and executed once in preview (execute()'s step 4)"
provides:
  - "OutcomePreserved: sixth Outcome value, a non-failed attempt in Classify (D-04), placed between OutcomeWrote and OutcomeAlreadyCorrect in AggregateOutcome's precedence"
  - "internal/setup/drift.go: Facet enum + facetOrder, AuthState, HeaderState, ObservedHeader, Observation, the optional DriftRuntime interface, Drift, Compare (the D-01 predicate), renderObservation, joinFacets, displayURL, notComparedNote — stdlib-only pure functions"
  - "codex.go's Observe implementing DriftRuntime via a json.Decoder.DisallowUnknownFields totality parse (D-11) of codex mcp get --json"
  - "apply.go's execute() !mutate branch rewritten to observe -> compare -> classify -> redact -> render; the mutate branch is byte-unchanged (pinned against ff5a6f94)"
  - "Result.Facets/Result.Drift flat string fields; Result.Registered rebuilt from the parsed-and-redacted observation (D-03), never raw probe bytes"
affects: [04-04 (Facets/Drift --output json + text rendering, results-table docs), 04-05 (Claude Code's own DriftRuntime scanner, gated on 04-OBSERVATIONS.md)]

# Actuals (#2632)
actuals:
  tokens: 23454
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Optional per-runtime capability interface, type-asserted exactly once (DriftRuntime mirrors plugin.go's PluginRuntime shape verbatim)"
    - "Redaction by construction: a raw observed value is compared in a local variable inside a runtime's own Observe frame and never assigned to any struct field that survives past the comparison"
    - "Single-declared-table facet ordering (drift.go's facetOrder), mirroring aggregate.go's precedenceOrder discipline"
    - "json.Decoder.DisallowUnknownFields totality parse for D-11: an unrecognized field becomes a preserved cause, never silently dropped"

key-files:
  created:
    - internal/setup/drift.go
    - internal/setup/drift_test.go
  modified:
    - internal/setup/plan.go
    - internal/setup/apply.go
    - internal/setup/codex.go
    - internal/setup/codex_test.go
    - internal/setup/exit.go
    - internal/setup/exit_test.go
    - internal/setup/aggregate.go
    - internal/setup/aggregate_test.go
    - internal/setup/apply_test.go

key-decisions:
  - "OutcomePreserved sits between OutcomeWrote and OutcomeAlreadyCorrect in precedenceOrder — this plan's own recorded decision resolving 04-RESEARCH.md Open Question 1, so a preserved registration is never hidden beneath an already-correct facet at the aggregate row, while a facet that actually wrote still outranks it"
  - "Compare/Observe touch no os/exec/filesystem import; the D-01 predicate is a pure function over a shared Observation type, with each runtime owning its own parser end-to-end (AUTHORED-HERE) — never a shared cross-runtime parser"
  - "Redaction is unconditional and by construction, never a shape heuristic: a raw header/credential value is compared for equality only, inside the observing runtime's own Observe call, and is never copied into any field of Observation, Drift, or Result"

patterns-established:
  - "DriftRuntime optional interface (drift.go): Observe(probeOutput string, opts Options) (Observation, bool), implemented per-runtime, asserted exactly once in apply.go's !mutate branch — the third optional-capability interface in this package after Runtime and PluginRuntime"

requirements-completed: [REQ-drift-observed-registration, REQ-drift-three-way, REQ-drift-preserved-outcome, REQ-drift-facet-naming, REQ-drift-redaction]

coverage:
  - id: D1
    description: "OutcomePreserved is a sixth, non-failed Outcome value: Classify places it beside already-correct/would-write/wrote (D-04), and AggregateOutcome's precedenceOrder places it between wrote and already-correct so it is never laundered into failure or hidden beneath convergence"
    requirement: "REQ-drift-preserved-outcome"
    verification:
      - kind: unit
        ref: "internal/setup/exit_test.go#TestClassifyExhaustiveOutcomeCombinations"
        status: pass
      - kind: unit
        ref: "internal/setup/aggregate_test.go#TestAggregateOutcomeExhaustive"
        status: pass
      - kind: unit
        ref: "internal/setup/aggregate_test.go#TestAggregatePrecedenceIsAuthoredNotDerived"
        status: pass
    human_judgment: false
  - id: D2
    description: "drift.go's Compare implements the D-01 three-way predicate (already-correct / would-write / preserved) as a pure function, naming every differing facet in the fixed D-12 order"
    requirement: "REQ-drift-three-way"
    verification:
      - kind: unit
        ref: "internal/setup/drift_test.go#TestCompareRegistrationThreeWay"
        status: pass
      - kind: unit
        ref: "internal/setup/drift_test.go#TestFacetOrderIsAuthored"
        status: pass
    human_judgment: false
  - id: D3
    description: "codex.go's Observe implements DriftRuntime: a totality parse of codex mcp get --json gated by DisallowUnknownFields (D-11), reaching all three Compare outcomes plus every unrecognized-content case"
    requirement: "REQ-drift-observed-registration"
    verification:
      - kind: unit
        ref: "internal/setup/codex_test.go#TestObserveCodexRegistration"
        status: pass
      - kind: unit
        ref: "internal/setup/drift_test.go#TestPreviewClassifiesRegistration"
        status: pass
    human_judgment: false
  - id: D4
    description: "apply.go's preview (!mutate) branch is rewritten to observe -> compare -> classify -> redact -> render; the mutate branch and every existing mutate-lane test stay byte-unchanged; ambiguity of every kind (seam error, unparseable output, no scanner) resolves to would-write, never already-correct or preserved (D-09/D-10)"
    requirement: "REQ-drift-three-way"
    verification:
      - kind: unit
        ref: "internal/setup/drift_test.go#TestPreviewAmbiguityResolvesToWouldWrite"
        status: pass
      - kind: unit
        ref: "internal/setup/drift_test.go#TestDriftRuntimeIsOptional"
        status: pass
      - kind: unit
        ref: "internal/setup/apply_test.go#TestApplyConvergesCodex"
        status: pass
      - kind: other
        ref: "diff <mutate-branch> against commit ff5a6f94 (pinned byte-identical)"
        status: pass
    human_judgment: false
  - id: D5
    description: "A header/credential value observed from any probe (including a foreign bearer variable's literal or reference-shaped value) never reaches Result.Registered, Reason, Drift, Facets, Notes, or a marshaled --output json Result — redaction is unconditional and by construction"
    requirement: "REQ-drift-redaction"
    verification:
      - kind: unit
        ref: "internal/setup/drift_test.go#TestRedactionUnconditional"
        status: pass
      - kind: unit
        ref: "internal/setup/drift_test.go#TestPreviewClassifiesRegistration/codex/preserved-unrecognized-field"
        status: pass
    human_judgment: false

# Metrics
duration: ~40min
completed: 2026-09-15
status: complete
---

# Phase 4 Plan 1: Codex Drift Detection Summary

**A single-read comparison against already-known intent classifies a Codex registration as already-correct/would-write/preserved, redacting every observed header value by construction so no probe byte ever reaches a rendered or marshaled field.**

## Performance

- **Duration:** ~40 min
- **Started:** ~2026-09-15T17:55:00Z (estimated — PLAN_START_TIME was not captured at the very first tool call this session)
- **Completed:** 2026-09-15T18:35:25Z
- **Tasks:** 3
- **Files modified:** 11 (2 created, 9 modified)

## Accomplishments

- `OutcomePreserved` is a real, documented sixth `Outcome` value: a non-failed attempt in `Classify` (D-04), positioned between `OutcomeWrote` and `OutcomeAlreadyCorrect` in `AggregateOutcome`'s precedence (this plan's own recorded decision, resolving 04-RESEARCH.md Open Question 1) — a declined write is never read as convergence at the aggregate row, while an actual write still outranks it.
- `internal/setup/drift.go` declares the D-01 predicate once: a closed typed `Facet` vocabulary (`url`, `auth-mode`, `header-name`, `header-value-ref`, `unrecognized-content`) rendered in a fixed stable order, and `Compare(obs Observation, opts Options) Drift` — a pure function classifying already-correct / would-write / preserved and naming every differing facet.
- `codex.go`'s `Observe` implements the optional `DriftRuntime` interface (the `PluginRuntime` precedent, asserted exactly once in `apply.go`): a totality parse of `codex mcp get engram --json` gated by `json.Decoder.DisallowUnknownFields`, so a codex release that adds a field makes setup cautious (`preserved`), never blind (a false `already-correct`).
- `apply.go`'s preview (`!mutate`) branch now observes, compares, classifies, redacts, and rebuilds `Result.Registered` from parsed-and-redacted fields — replacing the old unconditional `OutcomeWouldWrite` + raw `displayCapture(probe1...)` dump. The mutate branch is byte-unchanged, pinned against commit `ff5a6f94`.
- Redaction is proven unconditional: a sentinel planted in `bearer_token_env_var` (literal-shaped or reference-shaped — both produce byte-identical `Result`s, D-02) never appears in `json.Marshal(Result)`, `Registered`, `Reason`, `Drift`, `Notes`, or `Facets`.
- Ambiguity of every kind — an empty/unparseable probe body, a `name` field that is not `"engram"`, a probe seam error, or a scanner-less runtime (opencode, generic) — resolves to `would-write` with no facets and a `Drift` note that quotes no probe bytes (D-09/D-10).

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end preserved classification for Codex** — `805a16e` (feat)
2. **Task 2: Classify places preserved as a non-failed attempt; precedenceOrder gains preserved** — `51ce7b8` (feat)
3. **Task 3: Codex three-state coverage, the pure Compare table, facet-order gate, ambiguity, the optional interface, retargeted preview tests** — `02cae88` (test, includes a lint-defect fix to `drift.go`)

**Plan metadata:** committed alongside this SUMMARY.

_Note: this plan carried `tdd="true"` on every task; RED evidence for Task 1 was captured by temporarily reverting `plan.go`/`apply.go`/`codex.go`/`drift.go` to HEAD (`ff5a6f94`) while `drift_test.go`/`codex_test.go`'s new symbol references stood, confirming a compile failure, then restoring. Task 2's RED was captured by editing `exit_test.go`/`aggregate_test.go` against unmodified `exit.go`/`aggregate.go` and observing `--- FAIL`. Task 3's new subtests were GREEN on first run against Task 1/2's already-landed production code — correctly recorded as "GREEN on first run — pins existing behavior" per the plan's own `bsbsvn4hbc` guidance, not a faked RED._

## Files Created/Modified

- `internal/setup/drift.go` — NEW: `Facet` enum + `facetOrder`, `redactedValue`, `AuthState`, `HeaderState`, `ObservedHeader`, `rawHeader`/`plannedHeader`, `joinHeaders`, `sortedPlannedHeaders`, `sortObservedHeaders`, `Observation`, `DriftRuntime`, `Drift`, `Compare`, `orderedFacets`, `joinFacets`, `displayURL`, `notComparedNote`, `renderObservation`
- `internal/setup/drift_test.go` — NEW: `TestPreviewClassifiesRegistration`, `TestRedactionUnconditional`, `TestCompareRegistrationThreeWay`, `TestFacetOrderIsAuthored`, `TestPreviewAmbiguityResolvesToWouldWrite`, `TestDriftRuntimeIsOptional`
- `internal/setup/plan.go` — `OutcomePreserved` const + doc rewrite (six-value enum); `Result.Facets`/`Result.Drift` fields; `Registered`'s doc rewritten for D-03
- `internal/setup/codex.go` — `codexBearerForm`, `codexWholeEntryNote`, `codexRegistrationDoc`/`codexRegistrationTransport` mirror structs, `codexKnownTopKeys`/`codexKnownTransportKeys`, `isNullRaw`, `boundLabel`, `Observe` implementing `DriftRuntime`
- `internal/setup/codex_test.go` — `TestObserveCodexRegistration` (18 subtests: full three-state table)
- `internal/setup/apply.go` — `execute()`'s `!mutate` branch rewritten; doc comments for `Preview` and `execute` rewritten for the new comparison basis (04-RESEARCH.md Pitfall 1)
- `internal/setup/exit.go` — `Classify`'s switch gains `OutcomePreserved` in the non-failed-attempt case (D-04)
- `internal/setup/exit_test.go` — `allOutcomes` gains `OutcomePreserved`; new single-outcome and mixed-case rows
- `internal/setup/aggregate.go` — `precedenceOrder` gains `OutcomePreserved` between `OutcomeWrote` and `OutcomeAlreadyCorrect`
- `internal/setup/aggregate_test.go` — `allOutcomesPlusZero` (7 values), the 49-pair literal table, extended `TestAggregatePrecedenceIsAuthoredNotDerived`
- `internal/setup/apply_test.go` — `TestPreviewReportsRegisteredState`'s stale raw-capture subtest retargeted to `probe-zero-exit-reports-normalized-registration`; `Facets == ""` assertions added to the nonzero-exit and seam-error subtests

## Decisions Made

- **Precedence position (Open Question 1):** `OutcomePreserved` sits between `OutcomeWrote` and `OutcomeAlreadyCorrect` — the plan's own objective recorded this explicitly, rejecting 04-RESEARCH.md's earlier suggestion (between already-correct and would-write) because that would hide a preserved registration beneath an already-correct facet at the row's headline Outcome.
- **`Facets`/`Drift` as flat comma/`"; "`-joined strings** (Open Question 2): reused the existing joined-string idiom (`setupHeadersSummary`'s pattern) rather than introducing per-facet boolean scalar fields — no new renderer mechanism needed, and `sanitizeViewValue`'s scalar-only gate stays satisfied.
- **Expected side of the comparison is `Options` fields structurally** (Open Question 3): URL/auth-mode/header-NAME equality against `opts` directly, not a re-parse of `Plan.Display()`/rendered argv — the AUTHORED-HERE invariant already forbids re-deriving argv outside a runtime's own `Plan()`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed a golangci-lint `revive: package-comments` finding in `drift.go`**
- **Found during:** Task 3's plan gate (`task` lint)
- **Issue:** `drift.go`'s file-level doc comment was written directly above `package setup` (as drafted in Task 1), which `revive` flags as a second, malformed package doc comment ("package comment should be of the form 'Package setup ...'").
- **Fix:** Moved the file-doc comment to immediately after the `package setup` line — the exact convention `plugin.go` already establishes in this same package for a file-scoped (not package-scoped) doc comment.
- **Files modified:** `internal/setup/drift.go`
- **Verification:** `task` lint step (`golangci-lint run ./...`) reports "Success: No issues found in 126 files".
- **Committed in:** `02cae88` (Task 3 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking lint defect). **Impact:** cosmetic — a doc-comment placement fix with no behavioral change; no scope creep.

### Planning-artifact discrepancy (not a code deviation)

Task 2's own `<acceptance_criteria>` block states "`go test ./internal/setup/ -count=1` exits 0 (whole package)". After Task 2's commit, the whole-package run genuinely failed one pre-existing subtest: `TestPreviewReportsRegisteredState/probe-zero-exit-reports-registered`, which pinned the OLD raw-capture `Registered` assertion Task 1's `apply.go` rewrite made stale. This is not a Task 2 regression — the plan's own Task 3 `<behavior>` block explicitly assigns the retarget of that exact subtest (renamed to `probe-zero-exit-reports-normalized-registration`) to Task 3, which is what actually restores whole-package green (confirmed: `go test ./internal/setup/ -count=1 -shuffle=on` passes after Task 3). This is a plan-authored acceptance-criterion/task-sequencing mismatch (Task 2's whole-package gate depends on a fix Task 3 performs), not a defect in Task 2's own `exit.go`/`aggregate.go` changes — recorded here rather than silently worked around by jumping ahead into Task 3's assigned file.

A second, cosmetic planning-artifact note: Task 1's acceptance criteria literally grep for `'Facets string'` (single space) against `internal/setup/plan.go`, but `gofmt` aligns struct fields with multiple spaces (`Facets     string`), so the literal single-space grep does not match post-`gofmt` output — the same alignment issue the plan's own text already anticipated and worked around for the adjacent `Drift` field via a `+`-regex. Verified via the regex form instead (`rg -n -e 'Facets +string +`json:"facets,omitempty"`'`), which matches exactly once; the underlying property (a `Facets string` field with the correct json tag) is satisfied.

## Issues Encountered

- **`task test:go`'s one whole-repo failure is a pre-existing, environment-only gap, not a regression.** `TestDialTestClientFailsWhenRequiredAndUnavailable` (`internal/store`) fails in this sandbox because no Docker/Qdrant testcontainer provider is available ("qdrant testcontainer unavailable ... rootless Docker not found"). `internal/store` is unrelated to `internal/setup` (no import relationship), and `.planning/STATE.md`'s own Deferred Items table already records this exact test as an "acknowledged — Docker/testcontainer flake on one run; not a code defect, not reproduced since" gap from a prior milestone. `internal/keylinks`, `internal/setup` (including `-shuffle=on`), and every other package pass; `task license:check` passes (412 valid, 0 invalid).

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `internal/setup/drift.go`'s `Facet`/`Observation`/`DriftRuntime`/`Compare` contracts are stable and consumed verbatim by plans 04-04 (`--output json`/text rendering of `Facets`/`Drift`, `guides/agent-setup.md`'s results table) and 04-05 (Claude Code's own `DriftRuntime` scanner, gated on `04-OBSERVATIONS.md`).
- The `mutate == true` (apply) branch of `execute()` is untouched — Phase 5 is where the write path first consults this classification.
- No blockers. The five requirements this plan advances (`REQ-drift-observed-registration`, `REQ-drift-three-way`, `REQ-drift-preserved-outcome`, `REQ-drift-facet-naming`, `REQ-drift-redaction`) are shared with sibling plans 04-04/04-05 in the same phase (`requirements.ready-ids` reports 0/5 ready) — they are correctly left unmarked in `REQUIREMENTS.md` here and will flip to Complete once the last declaring sibling plan finishes (the shared-ID gate, #2388).

## Self-Check: PASSED

- All 11 `internal/setup/*.go` files and this SUMMARY.md confirmed present on disk (`[ -f ]`).
- Commits `805a16e`, `51ce7b8`, `02cae88` confirmed present via `git log --oneline --all`.
- Plan-level `<verification>` block re-run: the combined test-name set (`TestPreviewClassifiesRegistration`, `TestRedactionUnconditional`, `TestObserveCodexRegistration`, `TestCompareRegistrationThreeWay`, `TestFacetOrderIsAuthored`, `TestPreviewAmbiguityResolvesToWouldWrite`, `TestDriftRuntimeIsOptional`, `TestClassifyExhaustiveOutcomeCombinations`, `TestAggregateOutcomeExhaustive`) reports 107 `--- PASS` / 0 `--- FAIL`.
- `git diff --stat 805a16e^..02cae88` touches exactly the 11 files named in `files_modified`.
- `rt.(DriftRuntime)`/`Compare(obs, opts)`/`res.Registered = renderObservation(obs)` each appear exactly once in `apply.go`; `displayCapture(probe1` appears zero times.
- Mutate-branch byte-identity re-confirmed against `ff5a6f94`.

---
*Phase: 04-drift-detection-read-only*
*Completed: 2026-09-15*
