<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright 2026 Sean Brandt -->

---
phase: 07-cli-cross-spine-wiring
plan: 01
subsystem: cli
tags: [cross-spine, cli, connect, scope-guard]
status: complete
dependency-graph:
  requires: []
  provides:
    - validateScopeCrossSpine
    - renderCoverageFooter
    - EffectiveSearchScope
  affects:
    - cmd/engram/client_search.go
    - internal/server/tools.go
tech-stack:
  added: []
  patterns:
    - "shared client-side pre-flight guard (D-02), routed through usageErrorf (D-01/D-17)"
    - "conditional stdout footer gated on the caller's own request flag, not the response (D-06)"
    - "exported thin wrapper over an unexported server rule, for a compile-linked parity test (D-03)"
key-files:
  created: []
  modified:
    - cmd/engram/client_common.go
    - cmd/engram/client_search.go
    - cmd/engram/clienttest_test.go
    - cmd/engram/client_search_test.go
    - cmd/engram/client_common_test.go
    - internal/server/tools.go
decisions:
  - "Task 3's new tests (TestClientSearchMissingScopeIsUsageErrorBeforeDialing, TestClientSearchScopeWithCrossSpineIsUsageErrorBeforeDialing, TestClientSearchNoFooterWithoutCrossSpine) were written as part of Task 2's tracer commit instead of Task 3's commit — the tracer's own <action> text specified them directly, so they landed with the flag/guard/footer they exercise rather than being deferred. Task 3's own commit only added the parity test, the flag-naming test, and the EffectiveSearchScope export."
  - "resetClientFlags gained a searchScope reset (not just searchCrossSpine) — a pre-existing gap Task 2's own end-to-end test exposed: a leaked non-empty searchScope from an earlier test made the cross-spine-only invocation fail the new mutual-exclusion guard. Fixed under Rule 3 (blocking issue for the task at hand)."
actuals:
  tokens: 6141
  tasks: 3
  commits: 3
---

# Phase 07 Plan 01: CLI Cross-Spine Wiring (search tracer) Summary

One-liner: `engram search --cross-spine` now sends `cross_spine=true` and prints a count-based
coverage footer, with a client-side `--scope`/`--cross-spine` guard compile-linked against the
server's own `effectiveSearchScope` rule via a new exported `EffectiveSearchScope` wrapper — the
phase's sole authorized `internal/` edit.

## What Was Built

- **`validateScopeCrossSpine(scope string, crossSpine bool) error`** (`cmd/engram/client_common.go`)
  — the D-01/D-02/D-04 shared pre-flight guard. Rejects an empty scope with cross-spine off (D-01),
  rejects a non-empty scope together with cross-spine (D-04, deliberately stricter than the server),
  and accepts the two remaining rows. Routed through `usageErrorf` (exit 2).
- **`renderCoverageFooter(w io.Writer, crossSpine bool, searchedScopes []string, scopesTruncated bool) error`**
  (`cmd/engram/client_common.go`) — the D-05 stdout footer, a no-op when `crossSpine` is false (the
  D-06 byte-identical guarantee lives inside the helper, not at each call site). Prints
  `searched_scopes: <count>` and, only when true, `scopes_truncated: true` — a count, never scope
  names.
- **`--cross-spine` flag on `searchCmd`** (`cmd/engram/client_search.go`), wired into the
  `SearchMemoriesRequest` literal's new `CrossSpine` field, with the guard called before
  `resolveOutputFormat`/`clientFromFlags` (after the existing empty-`--query` check) so it fires
  before any dialing. The `--scope` flag's Usage was rewritten and `--cross-spine`'s Usage added,
  each naming the other by literal `--flag` spelling (D-00).
- **`EffectiveSearchScope(scope string, crossSpine bool) (string, error)`**
  (`internal/server/tools.go`) — the sole authorized `internal/` change this phase: an exported,
  behavior-preserving delegating wrapper around `effectiveSearchScope`, existing only so
  `TestValidateScopeCrossSpineParity` can compile against the real server rule.
- Test suite: `TestClientSearchCrossSpineEndToEnd` (the tracer's own end-to-end proof),
  `TestClientSearchMissingScopeIsUsageErrorBeforeDialing`,
  `TestClientSearchScopeWithCrossSpineIsUsageErrorBeforeDialing`,
  `TestClientSearchNoFooterWithoutCrossSpine` (all `client_search_test.go`),
  `TestValidateScopeCrossSpineParity` (the D-03 anti-drift gate against `EffectiveSearchScope`,
  4-row matrix with a count assertion, one-directional "client never accepts what server rejects"
  invariant) and `TestScopeCrossSpineFlagsNameEachOther` (the D-00 mechanical help-text check,
  table structured so plan 07-02 adds `listCmd` as a second row) in `client_common_test.go`.

## Task 1 — Migration Count

**Actual migration count: 18**, exactly matching RESEARCH.md's corrected count in the plan's
`<corrections_to_research>` block (RESEARCH.md's original claim of "no existing test breaks" was
already known-false going into this plan). All 18 `runClient(t, "search", ...)` invocations that
omitted `--scope` now pass `--scope repo:x` explicitly: 12 in `client_search_test.go` and 6 in
`client_common_test.go`. `TestClientSearchEndToEndJSON` was already explicit and untouched. No
invocation was found beyond the enumerated set.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - blocking issue] `resetClientFlags` did not reset `searchScope`**
- **Found during:** Task 2, running the full package suite after adding
  `TestClientSearchMissingScopeIsUsageErrorBeforeDialing` and `TestClientSearchCrossSpineEndToEnd`.
- **Issue:** `searchScope` was never included in `resetClientFlags`'s cleanup body. Every
  pre-existing test set `--scope` explicitly, and after Task 1's migration every remaining search
  invocation did too — so the gap was latent. The two new guard tests are the first callers to omit
  `--scope` on purpose, and pflag does not reset a flag var to its zero value when the flag is
  simply absent from a later invocation's args: a `searchScope = "repo:x"` left behind by an earlier
  test leaked into the new test, making `TestClientSearchCrossSpineEndToEnd` see a non-empty scope
  together with `--cross-spine` (tripping the new D-04 guard) and making
  `TestClientSearchMissingScopeIsUsageErrorBeforeDialing` see a non-empty scope (never observing the
  D-01 rejection it exists to prove).
- **Fix:** added `searchScope = ""` to `resetClientFlags`'s cleanup body, alongside the
  `searchCrossSpine = false` reset the plan's Task 2 action already specified.
- **Files modified:** `cmd/engram/clienttest_test.go`.
- **Commit:** `327fa9d6`.

### Structural note (not a deviation in outcome, only in commit boundary)

Task 3's `<action>` text assigns three new `client_search_test.go` tests
(`TestClientSearchMissingScopeIsUsageErrorBeforeDialing`,
`TestClientSearchScopeWithCrossSpineIsUsageErrorBeforeDialing`,
`TestClientSearchNoFooterWithoutCrossSpine`) to that task's commit, but Task 2's own `<action>` text
independently specifies the same tests as part of wiring the tracer (it names each one explicitly:
"the two before-dialing rejection tests and the D-06 baseline test" language appears in both tasks'
action bodies). They were written once, in Task 2's commit (`327fa9d6`), alongside the flag/guard/
footer they exercise. Task 3's commit (`119cb2f8`) therefore touches only `internal/server/tools.go`
and `cmd/engram/client_common_test.go` — the `EffectiveSearchScope` export, the parity test, and the
flag-naming test — with no `client_search_test.go` changes. All of Task 3's acceptance criteria
(the five named `--- PASS` lines, the `EffectiveSearchScope` grep, the containment gate, the import
boundary gate, `task lint`, and full `task`) are satisfied regardless of which commit the tests
physically landed in; verified directly rather than assumed.

## Auth Gates

None encountered.

## Known Stubs

None. Every symbol this plan promised (`validateScopeCrossSpine`, `renderCoverageFooter`,
`--cross-spine` on `searchCmd`, `EffectiveSearchScope`) is fully wired and covered by a passing test
proving it end to end.

## Threat Flags

None. All three STRIDE entries in the plan's `<threat_model>` (T-07-01 guard drift, T-07-02 scope-name
leakage, T-07-03 unhandled-row fallthrough) are mitigated exactly as the plan specified and pinned by
the tests it named — no new surface was introduced beyond what the plan enumerated.

## Verification

All commands below were run against the final tree (commit `119cb2f8`).

| Gate | Command | Result |
|------|---------|--------|
| Task 1 isolation | `go test ./cmd/engram/...` | `ok` |
| Task 1 named tests | `go test ./cmd/engram/... -run 'TestClientSearchExitCodeTransport\|TestClientSearchExitCodeAuth' -v \| rg -c '^--- PASS'` | `2` |
| Task 2 tracer | `go test ./cmd/engram/... -run TestClientSearchCrossSpineEndToEnd -v` | `--- PASS` |
| Task 2 completeness | `go test ./cmd/engram/... -shuffle=on -count=1` | `ok` |
| Task 2 import boundary | `go test ./cmd/engram/... -run TestClientFilesImportBoundary -v \| rg -c '^--- PASS'` | `1` |
| Task 2 build/vet | `go build ./...` / `go vet ./...` | both exit 0 |
| Task 3 named tests | `go test ./cmd/engram/... -run 'TestValidateScopeCrossSpineParity\|TestScopeCrossSpineFlagsNameEachOther\|TestClientSearchMissingScopeIsUsageErrorBeforeDialing\|TestClientSearchScopeWithCrossSpineIsUsageErrorBeforeDialing\|TestClientSearchNoFooterWithoutCrossSpine' -v` | 5x `--- PASS`, 0 `--- FAIL`/`--- SKIP` |
| `EffectiveSearchScope` export | `rg -c 'func EffectiveSearchScope\(' internal/server/tools.go` | `1` |
| Server behavior unchanged | `go test ./internal/server/...` | `ok` |
| Containment gate | `git diff --stat b4544d47 -- internal/ \| rg -c 'internal/server/tools.go'` and `git diff --name-only b4544d47 -- internal/ \| wc -l` | `1` and `1` |
| `task lint` | `task lint` | all linters pass |
| Full suite | `task` | `ok` on every package |
| `go vet ./...` | | exit 0 |
| `task license:check` | | 0 invalid |
| Zero new deps | `git diff --exit-code b4544d47 -- go.mod go.sum` | clean |
| Catalog absorption (D-07) | `go test ./cmd/engram/... -run 'TestCatalogEnumeratesEveryFlag\|TestCatalogExitCodesMatchMapper' -v` | both `--- PASS` |

## Commits

- `ab133d77` — `test(07-01): make search test invocations scope-explicit`
- `327fa9d6` — `feat(07-01): wire --cross-spine end to end on engram search`
- `119cb2f8` — `test(07-01): pin the scope guard against EffectiveSearchScope and the flag help contract`

## Self-Check: PASSED

- `cmd/engram/client_common.go` FOUND, contains `validateScopeCrossSpine` and `renderCoverageFooter`
- `cmd/engram/client_search.go` FOUND, contains `searchCrossSpine` var, flag, and request field
- `internal/server/tools.go` FOUND, contains `EffectiveSearchScope`
- `git log --oneline --all | grep ab133d77` FOUND
- `git log --oneline --all | grep 327fa9d6` FOUND
- `git log --oneline --all | grep 119cb2f8` FOUND

## Next

Plan 07-02 expands the same guard/flag/footer shape to `engram list` (`listCmd`), extending
`scopeCrossSpineFlagCommands` to a second row. Plan 07-03 closes D-00's correct-by-reading bar via
the self-describe catalog and docs-site updates.
