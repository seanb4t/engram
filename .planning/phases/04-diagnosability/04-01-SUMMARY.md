---
phase: 04-diagnosability
plan: 01
subsystem: api
tags: [go, connect-rpc, mcp, error-handling, validation]

requires:
  - phase: 04-diagnosability
    provides: "04-CONTEXT.md's checkpoint resolutions (D-17 through D-20), resolved by Sean 2026-08-01 prior to this plan's execution"
provides:
  - "argError: the one envelope carrying field attribution (D-05) and a remediation hint (D-09) together"
  - "connectError's *argError case, mapping failure CLASS to Connect code (D-11/D-20), positioned first to avoid the sentinel-collapse hazard (T-04-09)"
  - "validateStoreDiscovery converted end-to-end as the tracer's rejection site — the D-11a CodeInternal defect closed for it"
affects: [04-04, 04-05, 04-06]

actuals:
  tokens: 4421
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Structured argError type (Fields/Hint/Detail/Class) as the single MCP+Connect error envelope, replacing bare fmt.Errorf at validation rejection sites"
    - "argClass-driven Connect code selection via errors.As, never string matching"

key-files:
  created:
    - internal/server/argerror.go
    - internal/server/argerror_test.go
  modified:
    - internal/server/tools.go
    - internal/server/connecterror.go

key-decisions:
  - "D-17 (checkpoint, Sean 2026-08-01): envelope grammar is a flat prefix `field=<name> hint=<code>: <detail>`, not JSON — these errors are %w-wrapped by callers, and wrapping JSON produces unparseable nesting. On the MCP lane this string IS the wire format (go-sdk@v1.6.1/mcp/server.go:340-354 discards the built CallToolResult on error and returns only err.Error() as text)."
  - "D-18 (checkpoint, Sean 2026-08-01): the memory-summary bound is real but koanf-configurable, default 512, no Legacy key, 0 disables — deferred to plan 04-06, noted here so this plan's envelope makes no compile-time-constant assumption."
  - "D-19 (checkpoint, Sean 2026-08-01): delete_all's scopeArgs.Scope relaxation and its Go-level presence check ship as one indivisible task in 04-06 — accepted as proposed."
  - "D-20 (checkpoint, Sean 2026-08-01): Connect class-to-code mapping confined to {InvalidArgument, OutOfRange, FailedPrecondition} — exactly the trio exitCodeForConnectErr already groups under exitUsage, so the CLI exit-code contract is unchanged. classMalformed -> InvalidArgument, classOutOfRange -> OutOfRange, classPrecondition -> FailedPrecondition."
  - "Ten-code hint vocabulary implemented as typed HintCode constants: required, conditional_required, too_long, too_many, enum, format, prefix, ordering, mutually_exclusive, not_applicable."
  - "D-12 honored: dropped the pre-existing `got %q` value echo on the kind/scope rejections — Detail states the constraint and the bound, never the caller's rejected value. Byte counts (derived, bounded integers) are retained."

patterns-established:
  - "argError.Unwrap() -> store.ErrInvalidArgument keeps every existing errors.Is(err, store.ErrInvalidArgument) consumer working across the coming sweep (04-04, 04-05) with zero changes."
  - "connectError's *argError case is placed FIRST, before store.ErrNotFound and specifically before the errors.Is(err, store.ErrInvalidArgument) arm, with an explicit comment naming the collapse hazard (T-04-09) so a future reorder is caught by a reader, not just a test."

requirements-completed: [REQ-validation-error-attribution, REQ-error-hint-envelope]

coverage:
  - id: D1
    description: "A rejected store_discovery call returns an error leading with the field that failed and carrying a machine-stable hint code (D-05, D-08, D-09)"
    requirement: "REQ-validation-error-attribution"
    verification:
      - kind: unit
        ref: "internal/server/argerror_test.go#TestMCPErrorCarriesHintCode"
        status: pass
      - kind: unit
        ref: "internal/server/tools_test.go#TestValidateStoreDiscovery"
        status: pass
    human_judgment: false
  - id: D2
    description: "That rejection reaches the Connect lane as a caller-error code, never CodeInternal — D-11a closed for validateStoreDiscovery"
    requirement: "REQ-error-hint-envelope"
    verification:
      - kind: unit
        ref: "internal/server/argerror_test.go#TestStoreDiscoveryValidationIsNotCodeInternal"
        status: pass
    human_judgment: false
  - id: D3
    description: "The failure CLASS selects the Connect code and every class resolves inside {InvalidArgument, FailedPrecondition, OutOfRange} (D-11, D-20)"
    requirement: "REQ-error-hint-envelope"
    verification:
      - kind: unit
        ref: "internal/server/argerror_test.go#TestArgErrorConnectCodeTrio"
        status: pass
      - kind: unit
        ref: "cmd/engram/client_common_test.go#TestExitCodeForConnectErrTable"
        status: pass
    human_judgment: false
  - id: D4
    description: "The hint never echoes the caller's rejected value (D-12)"
    requirement: "REQ-error-hint-envelope"
    verification:
      - kind: unit
        ref: "internal/server/tools.go#validateStoreDiscovery (manual code review — got %q tail dropped from kind/scope rejections)"
        status: pass
    human_judgment: false

duration: 5min
completed: 2026-08-01
status: complete
---

# Phase 4 Plan 01: Field+Hint Error Envelope Tracer Summary

**One envelope (`argError`: Fields + Hint + Detail + Class) now carries `validateStoreDiscovery`'s five rejections end-to-end on both the MCP wire string and the Connect error code, closing the D-11a `CodeInternal` misclassification for that validator and proving the shape before the 04-04/04-05 sweep touches thirty more sites.**

## Performance

- **Duration:** 5 min
- **Started:** 2026-08-01T18:01:46Z
- **Completed:** 2026-08-01T18:06:40Z
- **Tasks:** 3 (checkpoint pre-resolved, not re-run)
- **Files modified:** 4 (2 created, 2 modified)

## Checkpoint Resolution (recorded, not re-asked)

Task 1 was `type="checkpoint:decision" gate="blocking"`. Sean resolved all four items on 2026-08-01,
**before** this execution began (per the orchestrator's `<checkpoint_already_resolved>` context):

1. **Envelope grammar (D-17):** `approve-as-proposed` — flat prefix `field=<name> hint=<code>: <human text>`.
2. **Summary bound (D-18):** approved on condition it is koanf-configurable (default 512, no `Legacy:` key,
   `0` disables). Lands in plan 04-06; this plan's envelope makes no compile-time-constant assumption that
   would conflict with that.
3. **Schema delta / `delete_all` (D-19):** mitigation accepted as proposed — the relaxation and its
   Go-level presence check are one indivisible task, deferred to 04-06.
4. **Class→code mapping (D-20):** confined to `{InvalidArgument, OutOfRange, FailedPrecondition}` —
   implemented exactly as specified in `argError.ConnectCode()`.

No re-ask occurred. Implementation proceeded directly per the resolved values above.

## Accomplishments

- `internal/server/argerror.go` — new file: `HintCode` (10 constants), `argClass` (3 constants),
  `argError{Fields, Hint, Detail, Class}`, `Error()`/`Unwrap()`/`ConnectCode()`, the two constructors
  (`argErrf`, `argErrFieldsf`), and the three `errors.As`-based accessors (`argFieldsOf`, `argHintOf`,
  `argClassOf`).
- `validateStoreDiscovery`'s five rejections (`content` empty, `content` too large, `kind` enum,
  `scope` empty, `scope` bad prefix) converted from bare `fmt.Errorf` to `argErrf`. `validateCitations`
  left byte-identical (confirmed via `git diff` scoped to lines outside the converted function).
- `connectError` gained an `*argError` case via `errors.As`, placed FIRST (before `store.ErrNotFound`
  and the `store.ErrInvalidArgument` sentinel arm), with a comment naming the T-04-09 collapse hazard
  explicitly so a future reorder is caught by a reader.
- Five named tests in `internal/server/argerror_test.go` pin: the grammar (single- and two-field),
  sentinel back-compat across all three classes, the Connect code trio as a SET (a fourth class fails),
  the MCP wire string itself (not struct fields), and the closed D-11a defect.

## Task Commits

Each task was committed atomically:

1. **Task 1 (tracer): field-attributed, hint-carrying argument errors** — `64b1e58d` (feat, `!` BREAKING CHANGE)
2. **Task 2: route argument-validation classes to distinct Connect codes** — `8550df20` (fix)
3. **Task 3: pin the argument-error envelope on both wire lanes** — `e7d74d5b` (test)

## RED Transcript — TestStoreDiscoveryValidationIsNotCodeInternal

Task 1 and Task 2 were already committed when Task 3 ran, so the RED reading was taken by temporarily
reverting the "empty content" rejection in `validateStoreDiscovery` to its pre-Task-1 bare `fmt.Errorf`
form, observing the failure, and restoring it (git diff confirmed zero net change to `tools.go`
afterward):

```
=== RUN   TestStoreDiscoveryValidationIsNotCodeInternal/empty_content
2026/08/01 14:05:00 ERROR connect handler: unexpected error error="content is required"
    argerror_test.go:168: connectError classified a validation rejection as CodeInternal (D-11a): content is required
--- FAIL: TestStoreDiscoveryValidationIsNotCodeInternal (0.00s)
    --- FAIL: TestStoreDiscoveryValidationIsNotCodeInternal/empty_content (0.00s)
    --- PASS: TestStoreDiscoveryValidationIsNotCodeInternal/content_too_large (0.00s)
    --- PASS: TestStoreDiscoveryValidationIsNotCodeInternal/bad_kind (0.00s)
    --- PASS: TestStoreDiscoveryValidationIsNotCodeInternal/empty_scope (0.00s)
    --- PASS: TestStoreDiscoveryValidationIsNotCodeInternal/non-discovery_scope (0.00s)
FAIL
```

This confirms the test would have caught the live, pre-Task-1 D-11a defect (a caller's invalid input
misclassified as a server fault). The other four subtests stayed green during the revert because only
the `content`-empty rejection was reverted — a genuine single-point RED, not a whole-function outage.

## Files Created/Modified

- `internal/server/argerror.go` — the envelope type, constructors, `Unwrap`/`ConnectCode`, three accessors
- `internal/server/argerror_test.go` — five named pinning tests plus accessor coverage
- `internal/server/tools.go` — `validateStoreDiscovery`'s five rejections converted; `validateCitations` untouched
- `internal/server/connecterror.go` — `*argError` case added first, doc comment extended with the class table

## Decisions Made

All four checkpoint items (D-17/D-18/D-19/D-20) were pre-resolved by Sean; see "Checkpoint Resolution"
above. One implementation-level decision made during execution: exercised the three `errors.As`
accessors (`argFieldsOf`/`argHintOf`/`argClassOf`) directly in `argerror_test.go` rather than leaving
them referenced only by their doc comments — `golangci-lint`'s `unused` check flagged them as dead code
since no production call site exists yet (their production consumers are 04-04/04-05's sweep sites and
the Connect mapper, which currently recovers `*argError` via a local `errors.As` rather than calling
`argClassOf`). This is a test-only usage addition, not a scope or shape change.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `golangci-lint`'s `errorlint` and `unused` checks failed on first `task` run**
- **Found during:** Task 3 (writing `argerror_test.go`)
- **Issue:** (a) `TestArgErrorConnectCodeTrio` used a bare `err.(*argError)` type assertion, which
  `errorlint` correctly flags as unsafe against wrapped errors; (b) the three `errors.As` accessor
  functions (`argFieldsOf`, `argHintOf`, `argClassOf`) had no call site anywhere, so `unused` failed
  the build. Neither issue is a task-scope deviation — both are corrections needed to make the
  already-planned artifacts compile clean under this repo's lint config.
- **Fix:** (a) replaced the type assertion with `errors.As(err, &ae)`; (b) added direct exercise of all
  three accessors inside `TestArgErrorGrammar`, plus a `non_argError_accessors_return_zero_values`
  subtest covering the zero-value path for a non-`*argError` input.
- **Files modified:** `internal/server/argerror_test.go`
- **Verification:** `task` (lint + full suite) green afterward.
- **Committed in:** `e7d74d5b` (Task 3 commit — the file was not yet committed when these fixes were made)

---

**Total deviations:** 1 auto-fixed (1 blocking — lint-clean compilation of already-planned artifacts)
**Impact on plan:** No scope creep. The accessor functions were already required artifacts per the
plan's frontmatter (`argFieldsOf(err) []string`, `argHintOf(err) HintCode`, `argClassOf(err) (argClass, bool)`);
exercising them in tests is the minimal fix that keeps them present without disabling the linter.

## Issues Encountered

None beyond the lint deviation above — all six `<verify>` gates across the three tasks passed on
first or second attempt (`go vet ./...`, five anchored `--- PASS:` greps, `task license:check`,
`git diff --exit-code -- go.mod go.sum`, and the full `task`).

## Next Phase Readiness

- The envelope shape (`argError`, the `HintCode`/`argClass` vocabularies, the `connectError` ordering
  discipline) is now locked and proven end-to-end on one real site. Plans 04-04 and 04-05 can sweep the
  remaining ~30 rejection sites using `argErrf`/`argErrFieldsf` without touching `argerror.go` or
  `connecterror.go` again, per the plan's stated boundary.
- Plan 04-06 owns the D-18 koanf-configurable summary bound and the D-19 `delete_all` schema-required
  relaxation — both checkpoint-approved but not implemented here, exactly as scoped.
- No blockers. `go.mod`/`go.sum` show zero diff (no new dependencies); `task` (lint + full suite) is
  green on the final tree.

---
*Phase: 04-diagnosability*
*Completed: 2026-08-01*

## Self-Check: PASSED

All created files confirmed present on disk (`internal/server/argerror.go`,
`internal/server/argerror_test.go`, this SUMMARY.md). All three task commits
(`64b1e58d`, `8550df20`, `e7d74d5b`) confirmed present in `git log --oneline --all`.
