---
phase: 04-diagnosability
plan: 05
subsystem: api
tags: [go, connect-rpc, mcp, error-handling, validation]

requires:
  - phase: 04-diagnosability
    provides: "04-01's argError envelope (argErrf/argErrFieldsf, HintCode vocabulary, argClass table) and its connectError *argError dispatch ordering; 04-04's fully-converted tools.go (effectiveSearchScope now returns a classified *argError, closing the reason connectapi.go's boundary calls to it needed a hand-wrap)"
provides:
  - "validateStoreRule/validateRuleSummary/listRules (rules.go) converted to the argError envelope — the third and last of D-11a's three unwrapped validators closed"
  - "connectapi.go's seven hand-wrapped connect.NewError(CodeInvalidArgument, ...) sites removed; every ListMemories/SearchMemories rejection now routes through connectError so the failure CLASS selects the Connect code (D-11 actually live on the Connect lane, not a no-op)"
  - "The cursor_mode/offset combination check reshaped as a relational rejection naming both fields, classPrecondition -> CodeFailedPrecondition"
  - "internal/server/connectargerror_test.go::TestStoreRuleValidationIsNotCodeInternal, TestConnectValidationCodeMapping, TestConnectCombinationAttribution — the D-11a closure pin, the full code-mapping table (12 subtests, 5 through real Connect handlers), and the relational-shape pin"
affects: [04-06, 04-07]

actuals:
  tokens: 6598
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "parseConnectWindowBound(field, raw string) (time.Time, error) — the connectapi.go equivalent of tools.go's inline MCP-closure window parses (04-04, rows 18-21): same argErrf(classMalformed, HintFormat, field, ...) shape, shared across the four created_after/created_before parse sites instead of writing the construction four times"
    - "Connect handler-driven table rows as the load-bearing proof of a mapper rewiring, not just a direct connectError(ctx, err) call: TestConnectValidationCodeMapping drives 5 of its 12 rows through the real ListMemories/SearchMemories/StoreDiscovery handlers specifically because a direct call proves the mapper's classification but NOT that the handler stopped overriding it"

key-files:
  created:
    - internal/server/connectargerror_test.go
  modified:
    - internal/server/rules.go
    - internal/server/connectapi.go
    - internal/server/connectapi_test.go
    - internal/server/argerror.go

key-decisions:
  - "Row 30 (listRules per-element scope rejection) keeps the plain field name 'scopes' — never 'scopes[i]' and never the offending value — per the plan's explicit instruction; the offending POSITION is stated in Detail text instead (D-12). This differs from validateCitations' 04-04 convention of a literal 'citations[i].kind' field name; the plan calls out this difference explicitly and this plan follows the plan's instruction for rules.go specifically."
  - "errStaleSummary (summary.go:34) intentionally left unconverted (RESEARCH sweep row 31): it is a state-precondition sentinel already mapped correctly to CodeFailedPrecondition by connecterror.go, not an argument rejection. Converting it would fold a state precondition into the argument-validation vocabulary."
  - "Deviation (Rule 1, connectapi_test.go): TestListMemoriesRejectsCursorModeWithOffset's expected code flipped from CodeInvalidArgument to CodeFailedPrecondition — the class change is the intended breaking effect of this plan (D-11/D-20), and the pre-existing test's assertion was correct for the old hand-wrapped code path, not the new classified one. Not declared in the plan's files_modified, but the plan's own D-20/Task-2 instruction makes this test's old assertion definitionally wrong once Task 2 lands."
  - "Deviation (Rule 3, blocking-gate): argerror.go's argErrFieldsf dropped its always-nil 'format string, a ...any' variadic in favor of a plain 'detail string' parameter, after golangci-lint's unparam check flagged it once this plan's fourth call site (connectapi.go's cursor_mode/offset rejection) joined the three pre-existing call sites (tools.go x2, argerror_test.go x1), none of which had ever populated it. Mechanical fix — no call site's syntax changed since none passed variadic args. Not declared in files_modified, but required for 'task' (a Task 3 verify gate) to pass. This is the ONE touch to argerror.go across 04-04/04-05, contrary to 04-01-SUMMARY's stated expectation that the sweep plans would not need to touch it again — the touch is lint-forced and mechanical, not a redesign of the envelope shape."

patterns-established:
  - "A Connect-mapper rewiring plan's proof-of-work table must drive at least one row per touched class through the REAL handler, not just through connectError(ctx, err) directly on the validator's output — the latter proves the mapper's classification logic but is blind to whether the handler is still hand-wrapping a code in front of it (exactly the T-04-14 failure mode this plan's Task 2 fixes)."

requirements-completed: [REQ-validation-error-attribution, REQ-error-hint-envelope]

coverage:
  - id: D1
    description: "validateStoreRule's four and validateRuleSummary's three rejections are field-attributed and hint-carrying, and do not map to Connect CodeInternal — the third and last of D-11a's three unwrapped validators closed"
    requirement: "REQ-validation-error-attribution"
    verification:
      - kind: unit
        ref: "internal/server/connectargerror_test.go#TestStoreRuleValidationIsNotCodeInternal (7 subtests)"
        status: pass
    human_judgment: false
  - id: D2
    description: "connectapi.go's seven hand-wrapped CodeInvalidArgument sites are removed; the failure CLASS selects the Connect code via connectError, proven by rows that travel through the real ListMemories/SearchMemories/StoreDiscovery handlers rather than by a direct constructor call"
    requirement: "REQ-error-hint-envelope"
    verification:
      - kind: unit
        ref: "internal/server/connectargerror_test.go#TestConnectValidationCodeMapping (12 subtests, 5 handler-driven, RED transcript recorded)"
        status: pass
      - kind: static
        ref: "region-scoped, comment-stripped zero-count of connect.NewError(connect.CodeInvalidArgument, ...) in connectapi.go (pre-task value 7)"
        status: pass
    human_judgment: false
  - id: D3
    description: "The cursor_mode/offset Connect-only combination check names BOTH fields (not an arbitrary single pick) and classifies as classPrecondition -> CodeFailedPrecondition, driven through the real ListMemories handler"
    requirement: "REQ-error-hint-envelope"
    verification:
      - kind: unit
        ref: "internal/server/connectargerror_test.go#TestConnectCombinationAttribution"
        status: pass
    human_judgment: false
  - id: D4
    description: "Every Connect code a validation failure can produce is a member of {InvalidArgument, OutOfRange, FailedPrecondition}, asserted as a SET, and the shipped CLI exit-code contract is proven intact against its own unmodified test"
    requirement: "REQ-error-hint-envelope"
    verification:
      - kind: unit
        ref: "internal/server/connectargerror_test.go#TestConnectValidationCodeMapping (trio-membership assertion on every row)"
        status: pass
      - kind: unit
        ref: "cmd/engram/client_common_test.go#TestExitCodeForConnectErrTable"
        status: pass
    human_judgment: false

duration: ~18min
completed: 2026-08-01
status: complete
---

# Phase 4 Plan 05: rules.go + connectapi.go — Finish the Sweep, Make Connect Honor the Class Summary

**`validateStoreRule`/`validateRuleSummary`/`listRules` converted to the field+hint envelope (closing D-11a's third and last unwrapped validator), and all seven of `connectapi.go`'s hand-wrapped `CodeInvalidArgument` sites removed so the failure CLASS — not a hand-wrap — selects the Connect code, proven by a 12-row table with five rows driven through the real `ListMemories`/`SearchMemories`/`StoreDiscovery` handlers and a recorded RED transcript.**

## Performance

- **Duration:** ~18 min
- **Started:** 2026-08-01T18:44:00Z (approx)
- **Completed:** 2026-08-01T19:02:00Z
- **Tasks:** 3
- **Files modified:** 5 (1 created, 4 modified)

## Accomplishments

- `rules.go`'s nine rejections (four in `validateStoreRule`, three in `validateRuleSummary`, two in `listRules`) converted from bare `fmt.Errorf`/manually-wrapped `store.ErrInvalidArgument` to `argErrf`. Both `got %q` value-echo tails dropped (D-12); the per-element `listRules` rejection keeps the plain field name `scopes` and states the offending position, never the value, per the plan's explicit instruction.
- **The load-bearing edit:** all seven of `connectapi.go`'s hand-wrapped `connect.NewError(connect.CodeInvalidArgument, ...)` sites in `ListMemories`/`SearchMemories` removed. The four `created_after`/`created_before` parse sites now share a new `parseConnectWindowBound` helper; the two `effectiveSearchScope` boundary calls simply hand their already-classified error to `connectError`; the `cursor_mode`/`offset` check becomes a relational rejection naming both fields (`argErrFieldsf`, `classPrecondition`, `HintMutuallyExclusive`) mapping to `CodeFailedPrecondition` — previously `CodeInvalidArgument`. Stale doc comments explaining the now-obsolete hand-wrap rationale rewritten to warn against reintroducing one.
- Three named tests in a new `internal/server/connectargerror_test.go`: the D-11a closure pin for `validateStoreRule` (7 subtests, direct `connectError` call since `store_rule` has no Connect RPC), the full code-mapping table (12 subtests, floor was ≥10, with 5 rows driven through real Connect handlers including one `classOutOfRange` and one `classPrecondition` row), and the relational-shape pin on `cursor_mode`/`offset`.

## The RED Transcript — TestConnectValidationCodeMapping

With Task 2 already committed, `ListMemories`' `cursor_mode`/`offset` check was temporarily reverted to its exact pre-Task-2 hand-wrap:

```go
if req.Msg.CursorMode && req.Msg.Offset > 0 {
    return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cursor_mode is mutually exclusive with offset"))
}
```

`go test ./internal/server/... -run 'TestConnectValidationCodeMapping$/connect_list_memories_cursor_mode_offset' -v -count=1` was re-run:

```
=== RUN   TestConnectValidationCodeMapping
=== RUN   TestConnectValidationCodeMapping/connect_list_memories_cursor_mode_offset
    connectargerror_test.go:258: connect.CodeOf(err) = invalid_argument, want failed_precondition (err: invalid_argument: cursor_mode is mutually exclusive with offset)
--- FAIL: TestConnectValidationCodeMapping (0.10s)
    --- FAIL: TestConnectValidationCodeMapping/connect_list_memories_cursor_mode_offset (0.10s)
FAIL
```

The revert was undone; `git diff --exit-code -- internal/server/connectapi.go` confirmed zero net change against the committed tree afterward. This is the observed proof — not an argument — that skipping or reverting Task 2 would have shipped D-11 as a no-op on the Connect lane, invisibly, because every pre-04-05 test (including `TestListMemoriesRejectsCursorModeWithOffset`, before this plan's fix) asserted `CodeInvalidArgument`.

## `errStaleSummary` — Deliberately Not Converted (sweep row 31)

`errStaleSummary` (`summary.go:34`) is a genuine state sentinel — a caller-authored summary went stale relative to a content change — not an argument rejection. `connecterror.go:60-61` already maps it correctly to `CodeFailedPrecondition`. Converting it to the `argError` envelope would fold a state precondition into the argument-validation vocabulary the envelope exists to serve. This is documented here so a later coverage audit does not read the exclusion as a missed row.

## Task Commits

Each task was committed atomically:

1. **Task 1: Convert rules.go and close the last of D-11a's three unwrapped validators** — `3bd08e97` (fix)
2. **Task 2: Stop connectapi.go overriding the class it is now being handed** — `db11dfcf` (feat!, `BREAKING CHANGE:` footer)
3. **Task 3: Pin the latent rule defect, the full code mapping, and the relational Connect shape** — `b2b83b04` (test)

## Files Created/Modified

- `internal/server/rules.go` — `validateStoreRule`, `validateRuleSummary`, `listRules` converted (Task 1)
- `internal/server/connectapi.go` — seven hand-wraps removed, `parseConnectWindowBound` helper added, cursor_mode/offset reshaped, doc comments rewritten (Task 2)
- `internal/server/connectapi_test.go` — `TestListMemoriesRejectsCursorModeWithOffset`'s expected code updated `CodeInvalidArgument` -> `CodeFailedPrecondition` (Task 2 deviation, Rule 1)
- `internal/server/connectargerror_test.go` — new: the three named pins (Task 3)
- `internal/server/argerror.go` — `argErrFieldsf`'s dead variadic parameter dropped (Task 3 deviation, Rule 3)

## Decisions Made

See `key-decisions` in the frontmatter. No checkpoint was hit — D-20 was pre-resolved by Sean before this plan's execution began, and this plan implemented it exactly as specified.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1] `TestListMemoriesRejectsCursorModeWithOffset`'s expected Connect code updated**
- **Found during:** Task 2, first `go test ./internal/server/...` run after removing the hand-wraps
- **Issue:** This pre-existing test (not declared in the plan's `files_modified`) asserted `CodeInvalidArgument` for the `cursor_mode`+`offset` rejection. D-20/Task 2 intentionally reclassifies this specific rejection to `CodeFailedPrecondition` — the test's assertion was correct for the old hand-wrapped code path and definitionally wrong for the new classified one.
- **Fix:** Updated the assertion and its doc comment to state the new expected code and why it changed.
- **Files modified:** `internal/server/connectapi_test.go`
- **Verification:** `go test ./internal/server/... -count=1` green afterward; `go test ./internal/server/... -count=1 -shuffle=on` green.
- **Committed in:** `db11dfcf` (Task 2 commit)

**2. [Rule 3 - Blocking] `argErrFieldsf`'s dead variadic parameter dropped**
- **Found during:** Task 3, first `task` run
- **Issue:** golangci-lint's `unparam` check flagged `argErrFieldsf`'s `a ...any` parameter as always receiving `nil` — true across all four call sites in the tree (the pre-existing two in `tools.go`, one in `argerror_test.go`, and this plan's new one in `connectapi.go`), and `task`'s lint gate is one of Task 3's own `<verify>` entries. `unparam` did not fire at 04-04 with three call sites; this plan's fourth call site apparently crossed its confidence threshold. `revive`'s `unused-parameter` also flagged three unused `t *testing.T` closure parameters in the new table-driven test.
- **Fix:** Changed `argErrFieldsf`'s signature from `(class argClass, hint HintCode, fields []string, format string, a ...any) error` to `(class argClass, hint HintCode, fields []string, detail string) error`, using `detail` directly instead of `fmt.Sprintf(format, a...)`. No call site's syntax changed since none ever passed variadic args. Renamed the seven unused closure `t` parameters to `_` in `connectargerror_test.go`.
- **Files modified:** `internal/server/argerror.go`, `internal/server/connectargerror_test.go`
- **Verification:** `task` (lint + full suite) green afterward; `go vet ./...` clean; all four named tests still pass.
- **Committed in:** `b2b83b04` (Task 3 commit)

---

**Total deviations:** 2 auto-fixed (1 Rule 1 — test assertion corrected for an intentional class change; 1 Rule 3 — lint-forced mechanical signature fix)
**Impact on plan:** No architectural change. Neither required a checkpoint. The Rule 3 fix is the one touch to `argerror.go` across 04-04/04-05, contrary to 04-01-SUMMARY's stated expectation that the sweep plans would not need to touch it again — the touch is mechanical (drops dead code) and does not alter the envelope shape (`Fields`/`Hint`/`Detail`/`Class`, `Error()`/`Unwrap()`/`ConnectCode()` all unchanged).

## Issues Encountered

None beyond the two deviations above. All `<verify>` gates across the three tasks passed after the deviation fixes; Task 1's gates passed on first attempt.

## Requirements Status

`REQ-validation-error-attribution` and `REQ-error-hint-envelope` are declared in this plan's frontmatter and this plan closes the `rules.go`/`connectapi.go` half of the D-06 sweep plus all of D-11's Connect-lane wiring — but **REQUIREMENTS.md was NOT updated to mark either complete** (`requirements.mark-complete` was not run), per this session's explicit instruction: 04-06 still owns D-06a's schema-level `omitempty` extension (issue #360's actual root cause) and D-19's `delete_all` relaxation, both required before either requirement is fully satisfied. 04-01 and 04-04 already established this same non-completion precedent for the same reason.

## Next Phase Readiness

- `internal/server/rules.go` and `internal/server/connectapi.go` carry zero bare argument-error constructors and zero hand-wrapped Connect codes; every rejection in both files routes through the single `argError`/`connectError` mechanism.
- All three of D-11a's named functions (`validateStoreDiscovery` 04-01, `validateCitations` 04-04, `validateStoreRule` 04-05) are converted and pinned.
- The Connect lane's code is now genuinely selected by the failure class — proven by a table containing `CodeOutOfRange` and `CodeFailedPrecondition` rows traveling through real handlers, not just `CodeInvalidArgument`.
- Every producible Connect code is inside the CLI-compatible trio, proven against the shipped, unmodified `TestExitCodeForConnectErrTable`.
- Plan 04-06 (D-06a's schema-level `omitempty` extension, D-19's `delete_all` relaxation, and the koanf-configurable summary bound D-18) can proceed independently — no arg struct tag was touched in this plan.
- Plan 04-07 (docs-site error reference) can proceed once 04-06 lands — the full class/code/hint vocabulary is now stable across both wire lanes.
- No blockers. `task` (lint + full suite) is green on the final tree; `go.mod`/`go.sum` show zero diff; `task license:check` reports 0 invalid.

---
*Phase: 04-diagnosability*
*Completed: 2026-08-01*

## Self-Check: PASSED

All modified/created files confirmed present on disk (`internal/server/rules.go`,
`internal/server/connectapi.go`, `internal/server/connectapi_test.go`,
`internal/server/connectargerror_test.go`, `internal/server/argerror.go`, this
SUMMARY.md). All three task commits (`3bd08e97`, `db11dfcf`, `b2b83b04`)
confirmed present in `git log --oneline --all`.
