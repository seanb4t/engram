---
phase: 06-typed-operator-renderer
plan: 01
subsystem: cli
tags: [cobra, encoding-json, operator-renderer, cli-output]

requires: []
provides:
  - "cmd/engram/operator_view.go: the one-serialization-plus-a-view rendering mechanism (viewFields, viewRow, viewScalar, humanizeKey, sanitizeViewValue, renderOperatorView) — walks json.Marshal(doc)'s own bytes, never the Go struct"
  - "renderOperator's text branch rewired to renderOperatorView; signature's third parameter renamed text -> headline"
  - "prune-expired converted to the mechanism; --output json byte-unchanged"
  - "a non-vacuous identity gate (assertViewIdentity / TestOperatorViewIdentity) plus committed negative-case proof that each half of the gate can fail, plus a live mutation-probe transcript proving the humanizer test and the identity gate are decomposed as D-06 requires"
  - "the four edge probes (empty, ordering, adjacency, encoding) and the T-06-03 control-character injection threat closed with committed tests"
affects: [06-03-PLAN.md, 06-04-PLAN.md, 06-05-PLAN.md, 06-06-PLAN.md, 06-07-PLAN.md]

actuals:
  tokens: 9742
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "One serialization (encoding/json over a hand-declared doc struct), one rendered view over that same marshaled document — no second format, no template language, no coverage-enforcement machinery (06-CONTEXT.md D-01/D-02)"
    - "View walks marshaled JSON bytes via (*json.Decoder).Token + Decode, never Go reflection over the struct — inherits omitempty/json:\"-\"/embedded-promotion/custom-MarshalJSON behavior for free"
    - "Manual utf8.RuneCountInString column padding, not text/tabwriter (tabwriter's block-termination-on-cell-count-change would misalign nested D-07 row lines)"

key-files:
  created:
    - cmd/engram/operator_view.go
    - cmd/engram/operator_view_test.go
  modified:
    - cmd/engram/operator_output.go
    - cmd/engram/operator_output_test.go
    - cmd/engram/prune_test.go
    - cmd/engram/reindex_test.go

key-decisions:
  - "Walked json.Marshal(doc)'s bytes rather than reflecting over the Go struct, per 06-CONTEXT.md's Claude's-Discretion note — makes D-02's identity-by-construction property literal rather than nearly true"
  - "humanizeKey is underscore-to-space plus a capitalized first rune, nothing else — no initialism table, since no top-level operator key is an initialism and nested row keys render raw (D-05)"
  - "renderOperator is SHARED code across all 15 operator commands, so rewiring its text branch is a global change even though only prune-expired's doc/summary functions were converted this plan — TestReindexTextModeUnchanged (out of this plan's file scope but broken by the shared change) was updated to the same D-03/R4 structural-pinning pattern as prune's own test (Rule 1 deviation, documented below)"

patterns-established:
  - "R4 structural pinning for --output text tests: assert headline-first-line, exactly-one-trailing-newline, and one-field-line-per-json-key via countTopLevelFieldLines — never a literal byte-identical string, since D-03 declares text explicitly unstable"

requirements-completed: [REQ-operator-renderer-typed]

coverage:
  - id: D1
    description: "renderOperatorView renders one aligned label line per top-level JSON key doc marshals to, walking the marshaled bytes exactly once"
    requirement: "REQ-operator-renderer-typed"
    verification:
      - kind: unit
        ref: "cmd/engram/operator_view_test.go#TestOperatorViewIdentity"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_view_test.go#TestOperatorViewEmptyShapes"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_view_test.go#TestOperatorViewOrdering"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_view_test.go#TestOperatorViewDuplicateKeyAdjacency"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_view_test.go#TestOperatorViewEncoding"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_view_test.go#TestOperatorViewNonObjectDocument"
        status: pass
    human_judgment: false
  - id: D2
    description: "prune-expired's --output json document is byte-unchanged; the identity gate holds for both its preview and applied shapes"
    requirement: "REQ-operator-renderer-typed"
    verification:
      - kind: unit
        ref: "cmd/engram/prune_test.go#TestPruneOutputJSONHasBestEffortMarker"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_view_test.go#TestOperatorViewIdentity/prune-expired"
        status: pass
    human_judgment: false
  - id: D3
    description: "The identity gate's two halves (key correspondence, rendered-line count) are each provably able to go RED, and are decomposed from the humanizer per D-06 (verified by a live mutation probe)"
    requirement: "REQ-operator-renderer-typed"
    verification:
      - kind: unit
        ref: "cmd/engram/operator_view_test.go#TestOrderedKeyDiffDetectsDivergence"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_view_test.go#TestCountTopLevelFieldLines"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_view_test.go#TestHumanizeKey"
        status: pass
    human_judgment: false
  - id: D4
    description: "A stored value cannot forge an extra report line or emit a terminal escape sequence (T-06-03 mitigated)"
    requirement: "REQ-operator-renderer-typed"
    verification:
      - kind: unit
        ref: "cmd/engram/operator_view_test.go#TestOperatorViewSanitizesControlCharacters"
        status: pass
    human_judgment: false

duration: 47min
completed: 2026-08-16
status: complete
---

# Phase 6 Plan 1: Typed Operator Renderer — one-serialization-plus-a-view mechanism Summary

**One-serialization-plus-a-view CLI renderer (walks `json.Marshal`'s own bytes via `encoding/json`'s streaming decoder) landed and proven end-to-end on `prune-expired`, with a non-vacuous identity gate and all four edge probes closed.**

## Performance

- **Duration:** 47 min
- **Started:** 2026-08-16T21:20:00Z (approx.)
- **Completed:** 2026-08-16T21:38:08Z
- **Tasks:** 3
- **Files modified:** 6 (2 created, 4 modified)

## Accomplishments

- `cmd/engram/operator_view.go`: `viewFields`, `viewRow`, `viewScalar`, `humanizeKey`, `sanitizeViewValue`, `renderOperatorView` — the sole text-rendering path, marshaling `doc` exactly once and walking those bytes with `(*json.Decoder).Token`/`Decode` rather than reflecting over the Go struct.
- `renderOperator` (`cmd/engram/operator_output.go`) rewired: its text branch now delegates to `renderOperatorView`; its third parameter renamed `text` → `headline`; `doc any` stays safe per D-02.
- `prune-expired` converted (`cmd/engram/prune.go` required zero changes beyond confirming R1 was a no-op — `prunePreviewSummary`/`pruneSummary` were already single-line headlines); its `--output json` document is byte-identical to commit `74d24853`.
- A non-vacuous identity gate: `assertViewIdentity`/`TestOperatorViewIdentity` pass for both `prune-expired` fixtures, backed by committed negative-case tests (`TestOrderedKeyDiffDetectsDivergence`, `TestCountTopLevelFieldLines`) proving each half can fail, and a recorded live mutation probe proving the humanizer test (`TestHumanizeKey`) and the identity gate are decomposed exactly as D-06 requires.
- All four edge probes (empty, ordering, adjacency, encoding) and the T-06-03 control-character injection threat closed with committed tests, not left as assumptions.

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end "prune-expired renders one text line per JSON key"** — `c18d4b4b` (feat)
2. **Task 2: Make every new gate demonstrably able to go RED** — `7cbacfac` (test)
3. **Task 3: Close the four edge probes and the text-lane injection threat** — `381f022c` (test)

**Plan metadata:** committed alongside this SUMMARY (see final commit in this plan's range).

## Files Created/Modified

- `cmd/engram/operator_view.go` — the view renderer (new)
- `cmd/engram/operator_view_test.go` — identity gate, negative-case proofs, humanizer pin, four edge-probe tests (new)
- `cmd/engram/operator_output.go` — `renderOperator`'s text branch rewired; `fmt` import dropped (no longer used)
- `cmd/engram/operator_output_test.go` — `TestRenderOperatorTextAndJSON`'s text-mode subtest converted to R4 structural pinning
- `cmd/engram/prune_test.go` — `TestPruneTextModeUnchanged` converted to R4 structural pinning
- `cmd/engram/reindex_test.go` — `TestReindexTextModeUnchanged` converted to R4 structural pinning (Rule 1 deviation, see below)

## Decisions Made

- Walked the marshaled JSON bytes rather than the Go struct (06-CONTEXT.md's Claude's-Discretion note resolved this way) — makes D-02's identity-by-construction property literal, and inherits `omitempty`/`json:"-"`/embedded-promotion/custom-`MarshalJSON` behavior without reimplementing any of it.
- `humanizeKey` is underscore→space plus a capitalized first rune and nothing else — no initialism table, since no top-level operator key is an initialism today and nested row keys render raw per D-05.
- Manual `utf8.RuneCountInString`-based column padding, not `text/tabwriter` — tabwriter's block-termination-on-differing-cell-count would silently re-align a report's label column around D-07's nested row lines.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `TestReindexTextModeUnchanged` broken by the shared `renderOperator` change**
- **Found during:** Task 1 verification (`go test ./cmd/engram/...`)
- **Issue:** `renderOperator` is one function shared by all 15 operator commands' `--output text` path (19 call sites). Rewiring its text branch to `renderOperatorView` is necessarily a global change, even though this plan converts only `prune-expired`'s doc/summary functions. `reindex.go`'s test file (out of this plan's declared `<files>` list, but downstream of the shared function) asserted byte-identical text output and failed as soon as the renderer changed.
- **Fix:** Converted `TestReindexTextModeUnchanged` to the same D-03/R4 structural-pinning pattern used for `prune_test.go` and `operator_output_test.go`'s text-mode subtest — headline-first-line, exactly-one-trailing-newline, `countTopLevelFieldLines` equals the json key count — rather than a literal string comparison.
- **Files modified:** `cmd/engram/reindex_test.go`
- **Verification:** `go test ./cmd/engram/... ` exits 0; `task` (lint + test, full module) exits 0.
- **Committed in:** `c18d4b4b` (Task 1 commit)

**2. [Rule 1 - Bug] Doc comments tripped the plan's own non-vacuity grep**
- **Found during:** Task 1, self-verification of the plan's `<prohibitions>` checks
- **Issue:** `rg -o 'json\.Marshal\(' cmd/engram/operator_view.go | wc -l` must output `1`, and `rg -l 'json\.Marshal\(' cmd/engram/ --glob '!*_test.go'` must output exactly `cmd/engram/operator_view.go`. Doc-comment prose that used the literal text `json.Marshal(doc)` to explain the mechanism tripped both greps (3 matches in `operator_view.go`, plus a spurious match in `operator_output.go`'s doc comment).
- **Fix:** Reworded the affected doc comments to describe the marshal call in prose without reproducing the literal `json.Marshal(` token, preserving the explanatory content.
- **Files modified:** `cmd/engram/operator_view.go`, `cmd/engram/operator_output.go`
- **Verification:** Both grep commands now return exactly the expected output.
- **Committed in:** `c18d4b4b` (Task 1 commit)

**3. [Rule 1 - Bug] `revive`'s `redefines-builtin-id` on a local `max` variable**
- **Found during:** Task 2, `task lint`
- **Issue:** `orderedKeyDiff` shadowed Go's builtin `max` identifier with a local variable, flagged by `golangci-lint`'s `revive` linter.
- **Fix:** Replaced the local variable with a direct call to the Go 1.21+ builtin `max(len(want), len(got))`.
- **Files modified:** `cmd/engram/operator_view_test.go`
- **Verification:** `task lint` exits 0.
- **Committed in:** `7cbacfac` (Task 2 commit)

**4. [Rule 1 - Bug] `govet`'s `structtag` flagged the deliberate duplicate-tag adjacency fixture**
- **Found during:** Task 3, `task lint`
- **Issue:** `TestOperatorViewDuplicateKeyAdjacency`'s throwaway struct deliberately declares two fields with the same `json:"dup"` tag (the point of the test — proving the view inherits `encoding/json`'s own field-conflict rule). `golangci-lint`'s `govet` `structtag` check flags this as a likely mistake.
- **Fix:** Added a justified `//nolint:govet` directive on the second field naming the reason (deliberate duplicate tag exercising the adjacency edge probe), per `nolintlint`'s requirement that every `//nolint` be justified.
- **Files modified:** `cmd/engram/operator_view_test.go`
- **Verification:** `task lint` exits 0.
- **Committed in:** `381f022c` (Task 3 commit)

**5. [Rule 1 - Bug] `humanizeKey`'s doc comment mention tripped the "invariant to label text" acceptance check**
- **Found during:** Task 2 self-verification
- **Issue:** The acceptance criteria requires `rg -n 'humanizeKey' cmd/engram/operator_view_test.go` to show hits only inside `TestHumanizeKey`, never inside `assertViewIdentity` or `TestOperatorViewIdentity`. `assertViewIdentity`'s doc comment mentioned `humanizeKey` by name in prose explaining D-06, which is a literal hit outside `TestHumanizeKey`.
- **Fix:** Reworded the doc comment to describe the property ("invariant to label text by construction") without naming the function.
- **Files modified:** `cmd/engram/operator_view_test.go`
- **Verification:** `rg -n 'humanizeKey' cmd/engram/operator_view_test.go` now shows hits only inside `TestHumanizeKey`.
- **Committed in:** `7cbacfac` (Task 2 commit)

---

**Total deviations:** 5 auto-fixed (5 Rule 1 — all direct fallout of Task 1's shared-code change or self-verification against this plan's own grep-based acceptance criteria).
**Impact on plan:** All auto-fixes were necessary for `task lint`/`task` to pass and for the plan's own stated acceptance criteria to hold literally. No scope creep — `prune.go` itself required zero changes (R1 was confirmed as a no-op), and no other operator report's doc/summary functions were touched.

## Retired-Test Facts (D-09, durable record `b3wd4wwwda`)

Per this plan's `<output>` instruction, recording the two facts about the not-yet-retired `TestOperatorOutputParity` (`cmd/engram/operator_output_test.go`) that must survive its deletion in plan 06-07:

1. Its `facts` strings (the values a text sentence was asserted to state) were **hand-listed** per row in `operatorParityRows()` — exactly the "test over hand-built rows" pattern ROADMAP Success Criterion 1 rejects. `TestOperatorOutputParity` itself is untouched by this plan (retirement is 06-07's job); this plan's own `TestOperatorViewIdentity` avoids the same defect by deriving its correspondence check from two independently-authored walks over the marshaled bytes (`viewFields` vs. `jsonTopLevelKeys`) rather than a hand-listed fact table.
2. It was **one-directional**: it asserted every declared text `fact` appears somewhere in the json document, but never that the json document fails to widen past what the text states. `TestOperatorViewIdentity`'s correspondence check (`orderedKeyDiff` over `jsonTopLevelKeys` vs. `viewFields`' keys) is symmetric — a dropped key AND an extra key both fail it, proven by `TestOrderedKeyDiffDetectsDivergence`.

Its one genuinely good property — gating its row set against `operatorCommands()` in both directions — is carried forward as work for whichever later plan (06-07) builds the full-15-report enumeration gate; this plan's fixture map (`pruneViewFixtures`) is deliberately single-report and not yet gated against `operatorCommands()`.

## Issues Encountered

None beyond the auto-fixed deviations documented above.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- The one-serialization-plus-a-view mechanism is proven end-to-end on `prune-expired`: `renderOperatorView`, `viewFields`, `viewRow`, `viewScalar`, `humanizeKey`, `sanitizeViewValue` are all in place, tested, and lint-clean.
- `renderOperator`'s new `(cmd, format, headline, doc)` signature is live for ALL 15 operator commands (the shared-code change from Task 1) — every other report's `--output text` output already renders through the new mechanism, but only `prune-expired`'s doc/summary functions were converted per R1/R2 this plan. The other 14 reports' `xxxSummary` functions still emit their full pre-conversion prose (loops, notices, re-run lines) — plans 06-03 through 06-06 must trim each down to a headline-only first line (R1) and add any gap-closure JSON keys (R2) per this plan's §Conversion Rules.
- Plan 06-07 depends on this plan's mechanism (`renderOperatorView`) and its §Conversion Rules (R1–R5), cited verbatim by 06-03/06-04/06-05/06-06 as confirmed by the cross-plan consistency check (`rg -l -F '06-01-PLAN.md §Conversion Rules' .planning/phases/06-typed-operator-renderer/*-PLAN.md` → exactly `06-01`, `06-03`, `06-04`, `06-05`, `06-06-PLAN.md`).
- No blockers.

---
*Phase: 06-typed-operator-renderer*
*Completed: 2026-08-16*

## Self-Check: PASSED

- `cmd/engram/operator_view.go` exists: FOUND
- `cmd/engram/operator_view_test.go` exists: FOUND
- `cmd/engram/operator_output.go`, `cmd/engram/operator_output_test.go`, `cmd/engram/prune_test.go`, `cmd/engram/reindex_test.go` all modified and present: FOUND
- Commit `c18d4b4b` (Task 1): FOUND in `git log --oneline --all`
- Commit `7cbacfac` (Task 2): FOUND in `git log --oneline --all`
- Commit `381f022c` (Task 3): FOUND in `git log --oneline --all`
- `go test ./cmd/engram/...` exits 0: PASSED
- `go test ./...` (full module) exits 0: PASSED
- `task` (lint + test, full module) exits 0: PASSED
- `task license:check` exits 0: PASSED
- `git diff --exit-code go.mod go.sum` exits 0: PASSED (no new dependencies)
- All plan-level `<verification>` and `<success_criteria>` commands re-run and passing (see above)
