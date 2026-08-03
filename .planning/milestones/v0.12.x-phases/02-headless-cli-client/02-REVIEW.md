---
phase: 02-headless-cli-client
reviewed: 2026-08-01T00:38:57Z
depth: standard
files_reviewed: 14
files_reviewed_list:
  - cmd/engram/catalog.go
  - cmd/engram/catalog_test.go
  - cmd/engram/client_common.go
  - cmd/engram/client_common_test.go
  - cmd/engram/client_list.go
  - cmd/engram/client_list_test.go
  - cmd/engram/client_search.go
  - cmd/engram/client_search_test.go
  - cmd/engram/client_store.go
  - cmd/engram/client_store_test.go
  - cmd/engram/clienttest_test.go
  - cmd/engram/root.go
  - cmd/engram/root_test.go
  - docs-site/src/content/docs/guides/cli.md
findings:
  critical: 1
  warning: 3
  info: 0
  total: 4
status: issues_found
---

# Phase 02: Code Review Report

**Reviewed:** 2026-08-01T00:38:57Z
**Depth:** standard
**Files Reviewed:** 14
**Status:** issues_found

## Summary

The credential/TLS/exit-code/catalog contract this phase exists to enforce is well
implemented and, unusually, well *proven*: `resolveToken` never puts a credential on
argv, `--insecure` warns unconditionally on stderr with no env fallback (empirically
gated by `TestInsecureIsNotSetByEnvironment`), stdout/stderr separation holds on every
path I traced, the exit-code mapper is genuinely single-sourced and the catalog is
derived from the live tree rather than a parallel literal (both anti-drift-gated by
`TestCatalogExitCodesMatchMapper`/`TestCatalogEnumeratesEveryFlag`), and the import/
stdin boundary gates are real AST walks, not string greps. `go build`, `go vet`, and
the package's own tests are clean at `count=1`.

That said, actually running the test suite adversarially (`go test -count=2` and
`-shuffle=on`, not just the default single pass CI uses) surfaces a real, deterministic
defect in the test harness itself: `resetClientFlags` does not reset every
package-level flag variable it needs to, and two tests (`TestClientListPassesFiltersToRequest`,
`TestClientStorePassesFieldsToRequest`) fail reproducibly as a direct result. This is
detailed as CR-01 below, with the exact repro command. Three further warnings round
out an otherwise solid implementation: an inconsistently-classified error path, no
timeout anywhere in the client's request path (the same "hung, unattended process"
failure mode the phase's stdin ban exists to prevent, arriving via the network
instead), and flag help text that promises a mutual-exclusivity invariant the client
never checks.

## Critical Issues

### CR-01: `resetClientFlags` does not reset every flag var it needs to — StringSlice flags accumulate across repeated invocations, and the failure is real and reproducible today

**File:** `cmd/engram/clienttest_test.go:99-111`
**Issue:**

`resetClientFlags`'s own doc comment states its purpose plainly: "The whole package
shares one rootCmd and one set of flag vars across the test binary, and a leaked
`--insecure` or `--output` would silently contaminate a later test." But the
implementation only zeroes the four vars bound by `addClientFlags`
(`clientServerURL`, `clientTokenFile`, `clientInsecure`, `clientOutput`). It does not
touch the per-command `StringSliceVar`-backed package vars: `storeTags`, `listTags`,
`listCategories`, `searchTags`, `searchCategories`.

pflag's `stringSliceValue.Set()` (`github.com/spf13/pflag@v1.0.10/string_slice.go:42-54`)
*appends* rather than replaces once its private `changed` bool has latched true from
any prior `Set()` call on that same `Value` — and since `storeCmd`/`listCmd`/`searchCmd`
are package-level `var`s parsed once at `init()` and reused for the lifetime of the
test binary process, that `changed` bool never resets between test functions. The
result: the *second* time any test in the process passes `--tags a,b` (or
`--categories x,y`) to the same subcommand, the values from the *first* invocation are
still there, silently appended to the new ones.

This is not a hypothetical — it reproduces deterministically with the standard,
commonly-used `-count` flag, no `-shuffle` required:

```sh
$ go test ./cmd/engram/... -run 'TestClientListPassesFiltersToRequest' -count=2 -v
=== RUN   TestClientListPassesFiltersToRequest
--- PASS: TestClientListPassesFiltersToRequest (0.00s)
=== RUN   TestClientListPassesFiltersToRequest
    client_list_test.go:105: Categories = [decision gotcha decision gotcha], want [decision gotcha]
    client_list_test.go:111: Tags = [foo bar foo bar], want [foo bar]
--- FAIL: TestClientListPassesFiltersToRequest (0.00s)

$ go test ./cmd/engram/... -run 'TestClientStorePassesFieldsToRequest' -count=2 -v
=== RUN   TestClientStorePassesFieldsToRequest
--- PASS: TestClientStorePassesFieldsToRequest (0.00s)
=== RUN   TestClientStorePassesFieldsToRequest
    client_store_test.go:93: Tags = [foo bar foo bar], want [foo bar]
--- FAIL: TestClientStorePassesFieldsToRequest (0.00s)
```

`go test -count=2` is a standard mechanism (used, among other things, to catch exactly
this class of non-hermetic test). `task test` currently runs `go test ./... -count=1`
(`Taskfile.yaml:54`), which is why this is dormant in CI today — but it is a live trap
for the next contributor who adds `-count=2`/`-shuffle=on` to CI, reruns a flaky test
with `-count`, or simply adds one more test earlier in file-sort order that passes
`--tags`/`--categories` to `store`/`list`/`search`. When it fires, it will look exactly
like a real regression in `client_list.go`/`client_store.go` request-building and cost
real debugging time chasing a harness bug, not a product bug — the precise failure mode
this repo's own review priorities call out ("a test that ... passes vacuously[/fails
spuriously] is a finding of the same severity as the bug it hides").

This does not affect the shipped binary's correctness (a real `engram` invocation is
one process, one `Execute()` call, so the latched-`changed` state never gets a second
`Set()` to accumulate onto) — it is a test-harness defect, not a product defect. It is
filed as Critical because it is a proven, reproducible false-failure generator in the
gate this whole phase leans on for its safety claims, not a style nit.

**Fix:** Reset every package-level flag var `resetClientFlags` is responsible for, not
just the four shared ones — including every `StringSliceVar`-backed var on every
client subcommand:

```go
func resetClientFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		clientServerURL = ""
		clientTokenFile = ""
		clientInsecure = false
		clientOutput = ""

		// pflag's stringSliceValue.Set latches "changed" permanently once set,
		// so a leaked slice from a prior test would otherwise be appended to,
		// not replaced, by the next --tags/--categories on the same
		// long-lived package-level *cobra.Command. Zeroing the backing slice
		// to nil here is sufficient (append to nil == a fresh slice).
		searchTags = nil
		searchCategories = nil
		listTags = nil
		listCategories = nil
		storeTags = nil
	})
}
```

Longer-term, consider a table-driven or reflection-based reset (or reconstructing the
FlagSet) so a future `StringSliceVar` flag added to any client command doesn't
silently reintroduce this same gap.

## Warnings

### WR-01: `resolveToken`'s file-read failure bypasses the exit-code taxonomy — and is untested

**File:** `cmd/engram/client_common.go:71-83`
**Issue:** Every other client-side validation failure (missing `--server`/
`ENGRAM_SERVER_URL`, empty `--query`, empty `--content`/`--scope`, an invalid
`--output` value) is wrapped via `usageErrorf` and exits `2`, per D-17/D-09. But an
unreadable or missing `--token-file` returns a plain `fmt.Errorf("reading
--token-file: %w", err)` — not a `*cliError` — so `exitCodeFromError` in `root.go`
falls through to its `1` default. A typo'd `--token-file` path or a permissions
problem is exactly the kind of client-side, no-network-round-trip mistake `usageErrorf`
exists to classify, and it is currently indistinguishable from a truly generic/
unclassified failure. No test in the suite exercises a nonexistent or unreadable
`--token-file` at all — `TestTokenFileTrailingNewlineTrimmed` and
`TestClientSearchTokenFromFile` only cover the success path.
**Fix:**
```go
b, err := os.ReadFile(tokenFilePath)
if err != nil {
	return "", usageErrorf("reading --token-file: %v", err)
}
```
and add a table case (or dedicated test) asserting `resolveToken`/`clientFromFlags`
on a nonexistent `--token-file` path returns `exitUsage`.

### WR-02: No timeout anywhere in the client request path

**File:** `cmd/engram/client_common.go:107-113`, `cmd/engram/client_search.go:46`,
`cmd/engram/client_list.go:44`, `cmd/engram/client_store.go:58`
**Issue:** `newHTTPClient` builds a bare `&http.Client{Transport: &http.Transport{...}}`
with no `Timeout` set, and none of `search`/`list`/`store` wrap `cmd.Context()` in a
`context.WithTimeout`/`WithDeadline` before issuing the RPC. `exitCodeForConnectErr`'s
own doc comment cites "a CI step, or a cron loop" as the target audience for this
CLI — the exact justification the phase gives for banning stdin reads (a blocked
prompt is a silent hang an unattended process can't recover from). A server that
accepts the TCP connection but never responds (a firewall black-holing the
connection, a hung reverse proxy, a stalled TLS handshake) produces the identical
failure mode — an indefinitely blocked process with no operator-visible signal —
just via the network instead of the keyboard, and nothing in this client path
prevents it.
**Fix:** Either set `http.Client{Timeout: ...}` in `newHTTPClient`, or wrap the
context passed to each RPC call (`cmd.Context()`) with a bounded deadline, with a
sensible default and (optionally) a `--timeout` flag alongside the other four shared
client flags.

### WR-03: `--offset`/`--cursor-mode`/`--page-token` mutual exclusivity is documented but not enforced client-side

**File:** `cmd/engram/client_list.go:86, 93-94`
**Issue:** The flag usage strings assert an invariant the code never checks:
`--offset`'s help text says `"mutually exclusive with --cursor-mode"`, and
`--page-token`'s says `"ignores --offset"`. `listCmd`'s `RunE` passes `listOffset`,
`listCursorMode`, and `listPageToken` straight through to `ListMemoriesRequest`
regardless of the combination, relying entirely on the server to reject an invalid
mix. Every other engram-detectable input error (empty `--query`, empty `--content`/
`--scope`) is caught locally with `usageErrorf` before any network call; this one is
not, so a caller who violates the documented invariant pays for a round trip (and
gets whatever exit code the server's rejection happens to map to) instead of the
immediate, offline `exitUsage` the help text implies exists.
**Fix:** Add a client-side check mirroring the existing pattern, e.g.:
```go
if listOffset != 0 && listCursorMode {
	return usageErrorf("--offset and --cursor-mode are mutually exclusive")
}
```
(and similarly for `--page-token` set alongside a non-zero `--offset`, if that
combination is also meant to be rejected rather than silently ignored).

---

_Reviewed: 2026-08-01T00:38:57Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
