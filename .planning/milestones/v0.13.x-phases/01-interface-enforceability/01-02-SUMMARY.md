---
phase: 01-interface-enforceability
plan: 02
subsystem: cli
tags: [cobra, pflag, exit-codes, flag-groups, self-describe-catalog, tracer]

# Dependency graph
requires: ["01-01"]
provides:
  - "The tracer spine: a declared cobra flag group on listCmd, validated centrally by
    rootCmd.PersistentPreRunE calling cmd.ValidateFlagGroups(), wrapped by usageErrorf into a
    *cliError{exitUsage} — proven end-to-end against a real accept-counting listener with zero
    dials. Every remaining flag-group site (plan 01-04), every operator command, and later
    timeout work build on exactly this mechanism."
  - "rootCmd.SetFlagErrorFunc: types every c.ParseFlags(a) failure (unknown flag, unparseable
    flag value) to exitUsage, inherited by every subcommand."
  - "config.CheckLegacy's error now wrapped with usageErrorf in PersistentPreRunE — the fourth
    bare-exit-1 site, closed for every command in the binary."
  - "The retracted D-17 catalog note replaced with the exitUsage/exitGeneric taxonomy actually
    shipped; TestCatalogClaimsNoFlagErrorExitsGeneric guards it with a positive-set assertion."
  - "A latent bug fix in resetCommandFlagState (from plan 01-01): it no longer calls
    f.Value.Set(f.DefValue) on stringSlice-typed flags, which was corrupting the underlying
    slice with a spurious literal element."
affects: [01-03, 01-04, 01-05, 01-06, 01-07, 01-08, 01-09]

# Actuals (#2632)
actuals:
  tokens: 7698
  tasks: 4
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Central flag-group interception: rootCmd.PersistentPreRunE receives the LEAF command
      (cobra's own behavior when EnableTraverseRunHooks is false) and calls
      cmd.ValidateFlagGroups() on it — one root-level call types every flag-group site in the
      binary, ahead of RunE and ahead of cobra's own redundant later call."
    - "Accept-counting listener for zero-dial proof: a net.Listener whose accept loop increments
      an atomic.Int64, used to prove a rejection path performs no I/O rather than merely
      asserting an error code."
    - "Positive-set assertion over prose: instead of a substring hunt for literal digits near a
      keyword (fragile — a reworded note can satisfy it by accident), extract the SET of numbers
      in the specific sentence naming the concept under test and assert set membership."

key-files:
  created:
    - cmd/engram/flaggroup_test.go
  modified:
    - cmd/engram/client_list.go
    - cmd/engram/root.go
    - cmd/engram/catalog.go
    - cmd/engram/catalog_test.go
    - cmd/engram/exitcode_baseline_test.go
    - cmd/engram/client_list_test.go
    - cmd/engram/clienttest_test.go

key-decisions:
  - "D-08 checkpoint pre-approved by the user before this execution: reject-both (any two of the
    paging trio, regardless of value) — including the widened blast radius where
    --offset 0 --page-token '' is also rejected, since cobra's flag groups count a supplied
    flag, not its value. Recorded here as an accepted checkpoint; execution proceeded straight
    through without stopping."
  - "TestFlagGroupRejectionPerformsNoIO (E-04) runs its table SEQUENTIALLY rather than from
    concurrent goroutines, per the plan's own documented escape hatch: runClient mutates
    rootCmd's shared SetOut/SetErr/SetArgs fields, which are not safe for concurrent access
    regardless of whether the supplied argv is identical across goroutines. The listener's own
    accept loop still runs on its own goroutine, so its counter stays an atomic.Int64 — the
    load-bearing claim (a shared accept total of 0) is preserved without a false claim of
    parallelism."
  - "The new D-17-retraction note embeds the actual exitUsage/exitGeneric constant values via
    fmt.Sprintf rather than literal digits, so the prose can never silently drift from the
    constants it describes."

requirements-completed: [REQ-flag-exclusivity-enforced, REQ-exit-code-unified, REQ-exit-code-migration-safe]

coverage:
  - id: D1
    description: "The tracer: one declared flag conflict travels declaration -> central
      interception -> typed error -> exit 2, provably without opening a socket"
    requirement: "REQ-flag-exclusivity-enforced"
    verification:
      - kind: unit
        ref: "cmd/engram/flaggroup_test.go#TestFlagGroupPagingTrioRejectedBeforeDial"
        status: pass
      - kind: unit
        ref: "cmd/engram/flaggroup_test.go#TestFlagGroupRejectionPerformsNoIO"
        status: pass
    human_judgment: false
  - id: D2
    description: "Unknown flags and unparseable flag values exit 2, not 1, via
      rootCmd.SetFlagErrorFunc"
    requirement: "REQ-exit-code-unified"
    verification:
      - kind: unit
        ref: "cmd/engram/flaggroup_test.go#TestFlagParseErrorsExitUsage"
        status: pass
    human_judgment: false
  - id: D3
    description: "A retired MEM_* env var exits 2, not 1, for any command"
    requirement: "REQ-exit-code-unified"
    verification:
      - kind: unit
        ref: "cmd/engram/flaggroup_test.go#TestLegacyEnvExitsUsage"
        status: pass
    human_judgment: false
  - id: D4
    description: "D-09 baseline rows flip from before to after as each behavior lands, staying
      green throughout"
    requirement: "REQ-exit-code-migration-safe"
    verification:
      - kind: unit
        ref: "cmd/engram/exitcode_baseline_test.go#TestExitCodeBaseline"
        status: pass
    human_judgment: false
  - id: D5
    description: "The published catalog no longer claims a flag-shaped failure exits 1; the two
      remaining exit-1 paths are named explicitly"
    requirement: "REQ-exit-code-unified"
    verification:
      - kind: unit
        ref: "cmd/engram/catalog_test.go#TestCatalogClaimsNoFlagErrorExitsGeneric"
        status: pass
      - kind: unit
        ref: "cmd/engram/catalog_test.go#TestRootUnknownSubcommandStillErrors"
        status: pass
    human_judgment: false

duration: ~13min
completed: 2026-08-03
status: complete
---

# Phase 01 Plan 02: Tracer — Flag Groups and Typed Framework Errors Summary

**Wires listCmd's paging trio through a declared cobra flag group, validated centrally by
`rootCmd.PersistentPreRunE`, proven end-to-end with a zero-dial accept-counting listener test;
closes the two remaining bare-exit-1 framework error classes (flag-parse errors, the legacy-env
guard); and retracts the falsified D-17 catalog note in favor of the taxonomy actually shipped.**

## Performance

- **Duration:** ~13 min
- **Tasks:** 4 (1 pre-approved checkpoint, 1 tracer, 2 auto)
- **Files modified:** 8 (1 new)

## Checkpoint: D-08 (pre-approved)

**Decision:** `reject-both` — any two of the paging trio (`--offset`/`--cursor-mode`/`--page-token`)
are rejected regardless of value, including the widened blast radius where
`--offset 0 --page-token ""` (both at zero values) is also rejected, since cobra's flag groups
count a *supplied* flag, not its value.

This checkpoint was shown to the user before this execution and pre-approved per the executor's
instructions; execution proceeded straight through without stopping. Recorded here as an accepted
checkpoint, not re-litigated.

## Accomplishments

**Task 1 (TRACER):** `listCmd.MarkFlagsMutuallyExclusive("offset", "cursor-mode", "page-token")`
in `client_list.go`, validated by `rootCmd.PersistentPreRunE` calling `cmd.ValidateFlagGroups()`
on the leaf command (cobra passes the running subcommand, not root, when
`EnableTraverseRunHooks` is unset — confirmed absent from the repo) and wrapping a non-nil result
with `usageErrorf`. `TestFlagGroupPagingTrioRejectedBeforeDial` proves the rejection against a
real `net.Listener` whose accept loop is counted with an `atomic.Int64`: the trio's conflicting
combination, the D-08 widened zero-value combination, and the legal empty case (no member
supplied — must NOT be rejected, and does dial) are all covered. `TestFlagGroupRejectionPerformsNoIO`
(edge item E-04) drives a table of five rejected combinations against ONE shared listener and
asserts a cumulative accept total of exactly 0, clean under `-race -count=2`. Flipped the
`list/offset+page-token` D-09 baseline row's `landed` to `true`.

The tracer's own `<verify>` (`go test ./cmd/engram/... -run 'TestFlagGroup|TestExitCodeBaseline' -v`)
was re-run after committing, per the tracer feedback gate, before proceeding to Task 2 — green.

**Task 2:** `rootCmd.SetFlagErrorFunc` types every `c.ParseFlags(a)` failure (unknown flag,
unparseable flag value) to `exitUsage`, inherited by every subcommand. `config.CheckLegacy`'s
error is now wrapped with `usageErrorf` in `PersistentPreRunE` — the fourth bare-exit-1 site.
`TestFlagParseErrorsExitUsage` and `TestLegacyEnvExitsUsage` cover both. Flipped
`search/unknown-flag`, `search/unparseable-flag-value`, and `root/legacy-env`'s baseline rows.

**Task 3:** Retracted the D-17 catalog note. The replacement states a framework flag error and a
violated flag group both exit `exitUsage`, and names the two remaining `exitGeneric` paths (a
mistyped verb, a genuinely unclassified internal error) explicitly, embedding the actual constant
values via `fmt.Sprintf` rather than literal digits. `TestCatalogDocumentsFlagParseExitCode`
(the old substring-hunt test, which could pass on a reworded note asserting the opposite) was
deleted and replaced by `TestCatalogClaimsNoFlagErrorExitsGeneric`, a positive-set assertion over
the numbers named in the specific sentence mentioning "flag". Extended
`TestRootUnknownSubcommandStillErrors` to assert the mistyped-verb exit code is `exitGeneric` via
`exitCodeFromError` (not `assertExitCode`, which would abort on the error's missing `ExitCode()`
method).

## Task Commits

Each task was committed atomically:

1. **Task 1 (TRACER)** — `f281d5a6` (feat): declared flag group + central interception +
   zero-dial proof, plus the two deviation fixes below.
2. **Task 2** — `f0fc38e3` (feat): `SetFlagErrorFunc` + wrapped `CheckLegacy` error.
3. **Task 3** — `c42b82d4` (docs): retracted D-17 note + replacement test.

**Plan metadata:** committed alongside this summary.

## Files Created/Modified

- `cmd/engram/flaggroup_test.go` — new file: `startAcceptCountingListener`,
  `TestFlagGroupPagingTrioRejectedBeforeDial`, `TestFlagGroupRejectionPerformsNoIO`,
  `TestFlagParseErrorsExitUsage`, `TestLegacyEnvExitsUsage`.
- `cmd/engram/client_list.go` — `MarkFlagsMutuallyExclusive` on the paging trio; corrected the
  three flags' `Usage` strings; removed the retracted "ignores --offset" promise.
- `cmd/engram/root.go` — `PersistentPreRunE` now receives the leaf command, calls
  `cmd.ValidateFlagGroups()`, and wraps `config.CheckLegacy`'s error with `usageErrorf`;
  `SetFlagErrorFunc` registered in `init()`.
- `cmd/engram/catalog.go` — retracted D-17 note; reworded the `exitGeneric` `Meaning` string.
- `cmd/engram/catalog_test.go` — replaced `TestCatalogDocumentsFlagParseExitCode` with
  `TestCatalogClaimsNoFlagErrorExitsGeneric`; extended `TestRootUnknownSubcommandStillErrors`.
- `cmd/engram/exitcode_baseline_test.go` — flipped four rows' `landed` to `true`.
- `cmd/engram/client_list_test.go` — deviation fix (see below).
- `cmd/engram/clienttest_test.go` — deviation fix (see below).

## RED Proof for TestCatalogClaimsNoFlagErrorExitsGeneric

Per the plan's Task 3 action, the old note text was temporarily restored and the new test run
against it to prove it goes RED (not merely that it's green on the new text), then the new note
was restored:

```
=== RUN   TestCatalogClaimsNoFlagErrorExitsGeneric
    catalog_test.go:387: the sentence naming a flag error also names exitGeneric (1): "A flag-parsing error raised by the command framework itself (an unknown flag, an unparseable flag value, or a mistyped verb) exits 1, not 2 (D-17)"
--- FAIL: TestCatalogClaimsNoFlagErrorExitsGeneric (0.00s)
FAIL
FAIL	github.com/seanb4t/engram/cmd/engram	0.254s
FAIL
```

Confirmed: the new test correctly rejects the retracted promise, not merely happens to pass on
the new wording. Restored the new note (`fmt.Sprintf(exitUsage, exitGeneric)`) and re-verified
green.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] pflag.Flag.Changed leak across `client_list_test.go`'s own tests, exposed by
Task 1's new `ValidateFlagGroups` check**
- **Found during:** Task 1, running `task test` (full-package suite) after Task 1's own new
  tests passed in isolation.
- **Issue:** Every pre-existing test in `client_list_test.go` called `resetClientFlags(t)` only,
  never `resetCommandFlagState(t, listCmd)`. Before this plan, nothing branched on pflag's
  `Changed` latch, so the omission was invisible. Once `PersistentPreRunE` started calling
  `cmd.ValidateFlagGroups()`, a flag `Changed` by an EARLIER test in the same package-level
  `rootCmd`/`listCmd` (e.g. `--offset` from one test, `--page-token` from another) stayed
  `Changed` for a LATER test that never itself passed that flag, causing spurious flag-group
  rejections (`TestClientListCursorModeReachesRequest`, `TestClientListEmptyResultIsEmptyArray`,
  `TestClientListNoDeprecatedApproximateFlag`, `TestClientListTextOutput`,
  `TestClientListCrossSpineEndToEnd`, `TestClientListFooterUnchangedWithoutCrossSpine`) and
  wrong exit codes on `TestClientListExitCodes` (the flag group short-circuited before the RPC
  ever fired, so the connect-code-to-exit-code mapping under test never ran).
- **Fix:** Added `resetCommandFlagState(t, listCmd)` immediately after `resetClientFlags(t)` in
  every test in the file. Also split `TestClientListPassesFiltersToRequest`, which combined
  `--offset` and `--page-token` in one invocation — now illegal under D-08 — into that test
  (using `--offset` alone) plus a new `TestClientListPageTokenReachesRequest` (using
  `--page-token` alone).
- **Files modified:** `cmd/engram/client_list_test.go`
- **Verification:** `go test ./cmd/engram/... -run 'TestClientList' -v` green; full package green
  under `-shuffle=on -count=2`.
- **Committed in:** `f281d5a6` (Task 1 commit)

**2. [Rule 1 - Bug] Latent `resetCommandFlagState` bug: `f.Value.Set(f.DefValue)` corrupts
stringSlice-typed flags**
- **Found during:** Task 1, immediately after applying deviation #1's fix — `--categories` and
  `--tags` assertions in `TestClientListPassesFiltersToRequest` started failing with a spurious
  leading empty-string element (`[[] decision gotcha]` instead of `[decision gotcha]`).
- **Issue:** `resetCommandFlagState` (added in plan 01-01) unconditionally called
  `f.Value.Set(f.DefValue)` for every flag. For a `StringSliceVar`-backed flag, `DefValue` is the
  bracketed DISPLAY string (`"[]"` for a nil default), not a value `Set` is designed to receive
  back — `pflag`'s `stringSliceValue.Set` runs the raw string straight through `readAsCSV`, and
  once its own unexported `changed` bit has latched (true after ANY prior `Set` call, including
  a call the same reset helper just made in an earlier test), `Set` APPENDS rather than replaces.
  Confirmed directly: a standalone reproduction (`go run` against a `pflag.FlagSet`) showed
  `s=[]string{"a","b"}` becoming `s=[]string{"a","b","[]"}` after `f.Value.Set(f.DefValue)`. This
  bug was latent since plan 01-01 — no test had called `resetCommandFlagState` against a command
  with a `stringSlice` flag until deviation #1's fix above added the calls that exposed it.
- **Fix:** `resetCommandFlagState` now skips the `Value.Set` call entirely for
  `stringSlice`-typed flags (`f.Value.Type() == "stringSlice"`), clearing only
  `pflag.Flag.Changed` for them — matching the helper's own pre-existing doc comment, which
  already claimed this was the behavior but the code didn't actually implement it.
  `resetClientFlags` already nils the underlying Go var directly, so the zero-value reset for
  these flags was never actually dependent on the `Set` call succeeding.
- **Files modified:** `cmd/engram/clienttest_test.go`
- **Verification:** `go test ./cmd/engram/... -run 'TestClientList' -v` green; full package green
  under `-shuffle=on -count=2`; `task test` and `task lint` clean.
- **Committed in:** `f281d5a6` (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (2 bugs, both required by Task 1's own change and both
confined to test infrastructure — no production `.go` file outside the plan's declared scope was
touched).
**Impact on plan:** Necessary for the tracer to be provably correct across the whole package, not
just in isolation — exactly the class of defect a tracer slice exists to surface once, before
plans 01-04 through 01-08 extend the same mechanism to every remaining flag-group site.

## Issues Encountered

None beyond the deviations above.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- The tracer spine (`MarkFlagsMutuallyExclusive` + `ValidateFlagGroups` in
  `PersistentPreRunE` + `usageErrorf`) is proven end-to-end and ready for plan 01-04 to extend to
  `client_common.go`'s `--scope`/`--cross-spine` guard and `migrate.go`'s `--from` tri-state.
- `SetFlagErrorFunc` and the wrapped `CheckLegacy` call are binary-wide; no later plan needs to
  touch them again.
- The D-09 baseline table now has 4 of its rows `landed: true`
  (`list/offset+page-token`, `search/unknown-flag`, `search/unparseable-flag-value`,
  `root/legacy-env`); the remaining rows are plans 01-03 through 01-08's to flip.
- `resetCommandFlagState`'s stringSlice fix is available to every later test in this package —
  no future plan should reintroduce a blind `Value.Set(DefValue)` call.
- No blockers.

---
*Phase: 01-interface-enforceability*
*Completed: 2026-08-03*

## Self-Check: PASSED

All created/modified files and all three task commit hashes (f281d5a6, f0fc38e3, c42b82d4)
verified present.
