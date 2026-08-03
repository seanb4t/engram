---
phase: 07-cli-cross-spine-wiring
reviewed: 2026-08-02T15:57:23Z
depth: standard
files_reviewed: 9
files_reviewed_list:
  - cmd/engram/client_common.go
  - cmd/engram/client_search.go
  - cmd/engram/client_list.go
  - cmd/engram/client_common_test.go
  - cmd/engram/client_search_test.go
  - cmd/engram/client_list_test.go
  - cmd/engram/clienttest_test.go
  - cmd/engram/catalog_test.go
  - internal/server/tools.go
findings:
  critical: 0
  warning: 1
  info: 1
  total: 2
status: issues_found
---

# Phase 07: Code Review Report

**Reviewed:** 2026-08-02T15:57:23Z
**Depth:** standard
**Files Reviewed:** 9
**Status:** issues_found

## Summary

Reviewed the CLI cross-spine wiring: `validateScopeCrossSpine` (client_common.go),
its wiring into `searchCmd.RunE`/`listCmd.RunE`, `renderCoverageFooter`, and the
exported `internal/server.EffectiveSearchScope` pinning function, plus the full
test suite added/touched for this phase.

The core correctness properties the phase context calls out all hold under direct
trace:

- **Guard-before-dial ordering.** Both `searchCmd.RunE` and `listCmd.RunE` call
  `validateScopeCrossSpine` before `resolveOutputFormat` and before
  `clientFromFlags`/the RPC call. `clientFromFlags` itself does not dial (it only
  constructs a `connect.Client`); the actual network round trip happens only at
  `client.SearchMemories`/`client.ListMemories`, which is unreachable from any
  error return above it. No early-return or error branch reaches a dial with an
  invalid scope/cross-spine combination. (search.go additionally checks
  `--query` before the scope guard; this doesn't create a window to dial with a
  bad scope — it's still just another early-return before the guard runs.)
- **Client/server parity.** Traced `effectiveSearchScope`/`EffectiveSearchScope`
  (internal/server/tools.go:1374-1390) against `validateScopeCrossSpine`
  (client_common.go:234-242) across the full 2x2 matrix by hand: the two agree on
  three of four rows and diverge on exactly the one documented row (scope set +
  cross-spine on — server silently discards scope and returns `""`; client
  rejects). `TestValidateScopeCrossSpineParity` pins this correctly and
  non-vacuously (asserts the one-directional "client never accepts what server
  rejects" invariant, not blanket equality). No second, undocumented divergence
  found.
- **Vacuous-test check.** The D-06 no-footer baseline tests
  (`TestClientSearchNoFooterWithoutCrossSpine`,
  `TestClientListFooterUnchangedWithoutCrossSpine`) genuinely populate
  `SearchedScopes`/`ScopesTruncated` on the stub response before asserting no
  footer line appears — they are not vacuously passing because the footer had
  nothing to print. The `--scope`/`--cross-spine` naming test
  (`TestScopeCrossSpineFlagsNameEachOther`) reads the live flag `Usage` strings
  rather than duplicating literals, so it can't drift silently.
- **Error-path/output correctness.** `renderCoverageFooter`'s return value is
  checked at both call sites (search.go:73, list.go:85). It writes only to
  `cmd.OutOrStdout()`, only from the `format == formatText` branch, so it can
  never appear in a JSON response.

One real gap surfaced under the "package-level flag state" priority: `resetClientFlags`
does not zero every package-level flag var it claims to, for flags outside this
phase's own scope/cross-spine additions. See WR-01.

The two `known_linter_signals` are both pre-existing lines untouched by this
phase's diff (`git diff b4544d47..HEAD` confirms neither line was added/modified
here) and are purely stylistic (`max()` builtin, `strings.Cut`) — no action
needed.

## Warnings

### WR-01: `resetClientFlags` does not zero every package-level client flag var, despite its own doc comment claiming it does

**File:** `cmd/engram/clienttest_test.go:99-133`
**Issue:** The doc comment states "resetClientFlags restores every package-level
client flag var to its zero value" and the function body was extended this phase
to add `searchScope`/`searchCrossSpine`/`listScope`/`listCrossSpine` — but it still
omits: `searchQuery`, `searchK`, `searchFull`, `searchCreatedAfter`,
`searchCreatedBefore` (search.go) and `listLimit`, `listOffset`, `listVisibility`,
`listFull`, `listCreatedAfter`, `listCreatedBefore`, `listPageToken`,
`listCursorMode` (list.go). All of these are cobra `*Var`-bound package-level
globals shared across the whole `cmd/engram` test binary, and — per the file's
own R-01 comment about `StringSliceVar`'s append-once-latched behavior — pflag
does **not** reset an unpassed flag to its default between `Execute()` calls; it
only calls `Set()` for flags actually present in `args`. A flag not reset after
one test, and not re-passed by a later test, silently carries its stale value
into that later test's RPC request.

This is concretely demonstrated by `TestClientListPassesFiltersToRequest`
(client_list_test.go:66-126), which sets `listLimit=10`, `listOffset=5`,
`listVisibility="shared"`, `listFull=true`, `listCreatedAfter`/`CreatedBefore`,
and `listPageToken="opaque-token"` via CLI flags. None of these are cleaned up.
Every later test in the binary that invokes `list` without explicitly re-passing
these flags (e.g. `TestClientListCursorModeReachesRequest`,
`TestClientListNoDeprecatedApproximateFlag`, `TestClientListTextOutput`,
`TestClientListCrossSpineEndToEnd`) sends a request carrying these leaked values
to the stub. It is currently dormant only because none of those later tests
assert on the leaked fields — it is exactly the "dormant under `-count=1`, real
under `-shuffle=on`/a future added assertion" failure mode the file's own R-01
comment warns about for `StringSliceVar`, just not generalized past the two
vars this phase happened to add.

**Fix:**
```go
func resetClientFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		clientServerURL = ""
		clientTokenFile = ""
		clientInsecure = false
		clientOutput = ""

		storeTags = nil
		searchQuery = ""
		searchScope = ""
		searchCrossSpine = false
		searchK = 0
		searchTags = nil
		searchFull = false
		searchCreatedAfter = ""
		searchCreatedBefore = ""
		searchCategories = nil

		listScope = ""
		listCrossSpine = false
		listLimit = 0
		listOffset = 0
		listCategories = nil
		listVisibility = ""
		listTags = nil
		listFull = false
		listCreatedAfter = ""
		listCreatedBefore = ""
		listPageToken = ""
		listCursorMode = false
	})
}
```
Longer-term, consider deriving this list mechanically (e.g. `VisitAll` over each
command's `Flags()` restoring `f.DefValue` via `f.Value.Set`, then clearing
`f.Changed`) so a newly added flag can't reintroduce the same gap silently.

## Info

### IN-01: `renderCoverageFooter`'s and `renderMemoryTable`'s writer-failure branch is never exercised by a test

**File:** `cmd/engram/client_common.go:263-273`, `cmd/engram/client_common.go:334-362`
**Issue:** Both functions correctly propagate a write error from `fmt.Fprintf`/
`tw.Flush()`, and both call sites in `client_search.go`/`client_list.go` check
that error — good. But no test in the suite exercises the failing-writer branch
(e.g. via a writer that returns an error on `Write`), for either function. This
is a pre-existing gap that predates this phase (the same is true of `renderJSON`),
not something the phase introduced or worsened, so it's informational only.
**Fix:** Optional — a small `io.Writer` stub that returns an error on `Write`
could be table-tested against `renderCoverageFooter`/`renderMemoryTable`/
`renderJSON` to pin the propagation behavior directly rather than relying on it
being incidentally exercised.

---

_Reviewed: 2026-08-02T15:57:23Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
