<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright 2026 Sean Brandt -->

# Phase 7: CLI Cross-Spine Wiring - Research

**Researched:** 2026-08-02
**Domain:** Go CLI (cobra) wiring a client-side flag/guard onto an already-shipped Connect RPC field; no server, proto, or store change.
**Confidence:** HIGH — every claim below is read from the live source this session, not inferred from CONTEXT.md's summaries.

## Summary

This phase adds one bool flag (`--cross-spine`) to two existing cobra commands (`searchCmd`,
`listCmd`), threads it into two existing request literals, adds one shared client-side guard
function, and prints one conditional stdout footer. There is no proto change, no server change, and
no new dependency. The single open question CONTEXT.md flagged — whether the D-15 self-describe
catalog picks up the new flag automatically — resolves cleanly to **yes, with zero further work**,
verified against the live catalog-derivation code and its own existing test.

The one genuine engineering wrinkle this research surfaces that CONTEXT.md's D-03 did not fully
resolve: `effectiveSearchScope`, the server rule the client guard must mirror, is **unexported**
(`func effectiveSearchScope` in `package server`), and `cmd/engram` is `package main`, which Go
**cannot import** at all (verified empirically this session — see Priority 2 below). CONTEXT.md's
claim that "the test compiles against both and goes RED the moment either moves" is not literally
achievable without a small change to `internal/server` (exporting the function or an equivalent
passthrough). This is a real decision the planner must make explicit, not a detail to paper over.

**Primary recommendation:** Implement D-01 through D-09 exactly as CONTEXT.md specifies, with one
addition: either (a) export `effectiveSearchScope` (or a thin passthrough) from `internal/server` so
the D-03 parity test can genuinely call both sides, accepting this as a narrow, behavior-preserving
exception to "CLI-only," or (b) accept a weaker parity test in `cmd/engram` that pins the *documented*
input/output matrix as literal expectations without calling the server function, and flag that this
does not structurally prevent drift the way D-03's own language promises. Recommend (a) — it is a
one-line, additive, non-behavior-changing export and is what "goes RED the moment either moves"
actually requires.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| `--cross-spine` flag definition + request-literal wiring | CLI (`cmd/engram`) | — | Pure client-side plumbing onto an already-additive proto field; no other tier is touched. |
| Client-side pre-flight scope/cross-spine guard (D-01/D-02/D-04) | CLI (`cmd/engram`) | API/Backend (mirrors `effectiveSearchScope`) | The rule is duplicated, not shared, by design (D-03) — CLI owns its own copy but must be pinned against the backend's copy. |
| Coverage footer rendering (D-05/D-06) | CLI (`cmd/engram`, text-mode renderer) | — | Presentation-only; the data (`searched_scopes`/`scopes_truncated`) already exists on the wire. |
| JSON-lane field emission | CLI (`cmd/engram`, `renderJSON`) | Database/Storage (protobuf `EmitDefaultValues`) | Already generic over any `proto.Message`; no new code needed (Priority 3, confirmed below). |
| Self-describe catalog entry for the new flag | CLI (`cmd/engram/catalog.go`) | — | Derived live from the cobra tree; requires no phase-specific code (Priority 1, confirmed below). |
| Authorization / scope enforcement | API/Backend (`internal/server`) | — | Already shipped in Phase 3; this phase adds no server-side behavior. |

## Phase Requirements

No `REQ-*` IDs are mapped to this phase (ROADMAP.md lists "TBD (none mapped)"). This phase closes a
cross-phase integration gap the milestone audit found (`.planning/v0.12.x-MILESTONE-AUDIT.md`,
"Gap — the CLI never reaches cross-spine"), not a new functional requirement. The audit is explicit
that no requirement text was violated — `REQ-cross-spine-search` and `REQ-cli-client-commands` are
each independently satisfied; the milestone *goal* ("make engram usable by agents that are not a
top-level MCP client") is what this phase closes.

## Priority 1 — D-07 verdict: the catalog picks up the flag automatically. YES.

**Verdict: YES — zero further work on the catalog itself.** Cite `cmd/engram/catalog.go:106-128`
(`collectFlags`) and `cmd/engram/catalog.go:52-76` (`buildCatalog`).

`collectFlags` walks the **live** `cobra.Command.Flags()` via `pflag.FlagSet.VisitAll`:

```go
// cmd/engram/catalog.go:109-128
func collectFlags(root, cmd *cobra.Command) []catalogFlag {
	seen := make(map[string]bool)
	var flags []catalogFlag
	add := func(f *pflag.Flag) {
		if f.Hidden || seen[f.Name] {
			return
		}
		seen[f.Name] = true
		flags = append(flags, catalogFlag{
			Name:    f.Name,
			Type:    f.Value.Type(),
			Default: f.DefValue,
			Usage:   f.Usage,
		})
	}
	cmd.Flags().VisitAll(add)
	root.PersistentFlags().VisitAll(add)
	sort.Slice(flags, func(i, j int) bool { return flags[i].Name < flags[j].Name })
	return flags
}
```

`f.Usage` (line 121, the exact `pflag.Flag.Usage` string set by `cmd.Flags().BoolVar(&x, "cross-spine",
false, "<usage string>")`) is emitted **verbatim** — this is the same string a human sees in
`engram search --help`. There is no hardcoded/curated flag list anywhere in `catalog.go`; `buildCatalog`
(`catalog.go:52-76`) iterates `root.Commands()` and calls `collectFlags` per command, with only
`help`/`completion`/`Hidden` excluded by name (`catalog.go:68-70`) — `search`/`list`/`store` are never
special-cased.

**Concretely:** after adding `searchCmd.Flags().BoolVar(&searchCrossSpine, "cross-spine", false, "<usage>")`
and the equivalent on `listCmd`, the next `engram | jq '.commands[] | select(.name=="search")'` will show
the new flag with zero catalog-code changes.

**This is independently self-verifying, not just my reading.** `cmd/engram/catalog_test.go:172-213`
(`TestCatalogEnumeratesEveryFlag`) already asserts, for `search`/`list`/`store`, that the catalog's flag
set is **exactly** (`reflect.DeepEqual`, not subset) the set `cmd.Flags().VisitAll` produces on the live
tree, and additionally fails if any flag's `Usage` string is empty (`catalog_test.go:205-207`). This
means: the moment `--cross-spine` is added with any Usage string, this pre-existing test starts
asserting it is present in the catalog and non-empty — with **no test-code change required** for this
phase. This is the single strongest piece of evidence that D-07's "may be nearly free" is correct: it
is not nearly free, it is **exactly** free for the catalog surface itself. The only real work under D-00
is writing a *good* Usage string (the `--help` text), because whatever string is written there is what
the catalog also carries, verbatim.

**Conclusion for the planner:** D-07 costs zero catalog tasks. It does NOT reduce the flag-help-text
work (D-00's actual deliverable) — the catalog is a mirror of whatever Usage string the flag
declaration carries, so writing that string well is still a first-class task, just not a *second*
task for the catalog on top of it.

## Priority 2 — the server rule the client guard must mirror (D-01/D-03)

### `effectiveSearchScope`'s exact logic

`internal/server/tools.go:1374-1382`:

```go
func effectiveSearchScope(scope string, crossSpine bool) (string, error) {
	if crossSpine {
		return "", nil
	}
	if scope == "" {
		return "", argErrf(classMalformed, HintConditionalRequired, "scope", "scope is required unless cross_spine is true")
	}
	return scope, nil
}
```

Its own doc comment (`tools.go:1361-1366`) states the rule plainly: *"cross_spine==true ignores any
supplied scope and returns "" (span every scope the caller may read); otherwise a non-empty scope is
mandatory."*

### Full input matrix

| scope | cross_spine | Server behavior today |
|---|---|---|
| `""` (empty) | `true` | Returns `("", nil)` — no error. Cross-spine wins; scope is irrelevant (never inspected). |
| non-empty | `true` | Returns `("", nil)` — **the supplied scope is silently ignored**, not an error. (This is Phase 3's D-02 asymmetry, documented at `internal/server/connectapi.go` per CONTEXT.md's "D-04 asymmetry comment block.") |
| `""` (empty) | `false` | Returns error: `argErrf(classMalformed, HintConditionalRequired, "scope", "scope is required unless cross_spine is true")`. This is the "most natural invocation fails" defect CONTEXT.md's `<domain>` section describes. |
| non-empty | `false` | Returns `(scope, nil)` — normal scope-confined path, unchanged. |

The client guard (D-01/D-04) must reject **client-side** exactly the third row (empty scope, no
`--cross-spine`) AND additionally reject a **new, stricter** case the server does not reject at all:
scope non-empty **and** `--cross-spine` set (row 2) — D-04 makes this a client-side hard error (exit 2)
specifically *because* the server's behavior for that row (silently discard the scope, log at Info) is
invisible to the caller. So the client guard's rule is **not identical** to `effectiveSearchScope`'s
rule — it is stricter on row 2. The D-03 parity test therefore is not "assert client guard ==
`effectiveSearchScope`" cell-by-cell; it is "assert the client never rejects a call the server would
accept EXCEPT row 2, which it rejects deliberately-more-strictly, and assert it always rejects what the
server would reject (row 3)." State this asymmetry explicitly in the parity test's doc comment, or a
future reader will "fix" the client guard to match the server's row-2 leniency and silently defeat D-04.

### Compile-reachability fact (the thing the planner needs, not an opinion)

`effectiveSearchScope` is **unexported**, in `package server` (`internal/server/tools.go:5`,
confirmed: `package server`). `cmd/engram` is `package main` (`cmd/engram/client_common.go:4`,
confirmed: `package main`).

**Verified empirically this session** (built a throwaway two-package Go module in the scratchpad and
attempted `import _ "mod/cmd/foo"` from a sibling package, where `cmd/foo` is `package main`):

```
consumer/consumer.go:3:8: import "mainimporttest/cmd/foo" is a program, not an importable package
```

This means, unconditionally:

1. **No file in `internal/server` can import `cmd/engram`** — package `main` cannot be imported by
   any other package, full stop. This rules out "put the parity test in `internal/server` and have it
   call the client guard."
2. **`cmd/engram` CAN import `internal/server`** as an ordinary package (it is not `package main`), but
   it can only reach identifiers `internal/server` **exports**. `effectiveSearchScope` (lowercase) is
   not one of them, so a test file inside `cmd/engram` — even a `_test.go` file, even in `package main`
   — cannot call it today.
3. Go's `export_test.go` idiom (a `_test.go` file inside `package server` that assigns an exported var
   to the unexported function, purely for test access) does **not** help here either: that idiom only
   exposes the identifier to *external test packages of `internal/server` itself* (i.e.
   `package server_test` compiled in the same `go test ./internal/server` run) — it does not propagate
   to a completely different package like `cmd/engram` importing `internal/server` as a normal
   dependency, because `_test.go` files are excluded from any build of `internal/server` performed for
   another package's sake.

**Conclusion: there is no location where a single test file can call both the client guard and
`effectiveSearchScope` as they exist today.** To satisfy D-03's literal claim ("compiles against both
... goes RED the moment either moves"), `internal/server` needs one additive, non-behavior-changing
change: export `effectiveSearchScope` (rename, or add a one-line exported passthrough e.g.
`EffectiveSearchScope(scope string, crossSpine bool) (string, error) { return effectiveSearchScope(scope, crossSpine) }`).
This is a touch to `internal/server`, which nominally sits outside "this phase edits only `cmd/engram/*`
files" (CONTEXT.md's `<canonical_refs>` "Files this phase edits" list does not include any
`internal/server` file). The planner must make an explicit call here — this is not "Claude's
Discretion" in CONTEXT.md's sense (which only asked *where* the test lives, not whether
`internal/server` needs a one-line export to make that location possible at all). Two honest options,
either acceptable, but the plan must pick one and say so:

- **(a) Export it.** Small, additive, testable, and it is what makes D-03's own stated guarantee true.
  Slightly stretches "CLI-only, no handler behavior" — but it changes zero behavior, only visibility.
- **(b) Duplicate the matrix as literal expected values in a `cmd/engram`-only test**, never calling
  `effectiveSearchScope` directly. This satisfies "the guard matches today's documented behavior" but
  explicitly does **not** deliver D-03's "goes RED the moment either moves" property — if
  `effectiveSearchScope`'s rule changes later, this test keeps passing on stale expectations. If the
  planner chooses this, RESEARCH strongly recommends the plan say so explicitly rather than let D-03's
  language stand unqualified.

## Priority 3 — the "no JSON-lane work needed" claim: TRUE for both response types

`renderJSON` (`cmd/engram/client_common.go:263-274`) is generic over `proto.Message` and unconditionally
sets `EmitDefaultValues: true` (`client_common.go:266`) — this is not response-type-specific code; it
applies identically to any message passed to it.

Both response types carry the two provenance fields as real proto3 fields (verified by reading the
generated code):

```
gen/go/engram/v1/engram.pb.go:587: SearchedScopes []string `protobuf:"bytes,5,...,json=searchedScopes,proto3" json:"searched_scopes,omitempty"` (on ListMemoriesResponse)
gen/go/engram/v1/engram.pb.go:591: ScopesTruncated bool `protobuf:"varint,6,...,json=scopesTruncated,proto3" json:"scopes_truncated,omitempty"` (on ListMemoriesResponse)
gen/go/engram/v1/engram.pb.go:787: SearchedScopes []string `protobuf:"bytes,2,...,json=searchedScopes,proto3" json:"searched_scopes,omitempty"` (on SearchMemoriesResponse)
gen/go/engram/v1/engram.pb.go:791: ScopesTruncated bool `protobuf:"varint,3,...,json=scopesTruncated,proto3" json:"scopes_truncated,omitempty"` (on SearchMemoriesResponse)
```

Since `renderJSON` is called on `resp.Msg` for both `client_search.go:66` and (would be, on the JSON
branch) `client_list.go:78` with `EmitDefaultValues: true` + `UseProtoNames: true`, both fields will
render as `"searched_scopes":[...]` / `"scopes_truncated":false` (or `true`) on **every** JSON-format
response from both commands, whether or not `cross_spine` was set. **Confirmed: no JSON-lane task is
needed.** (The `false`/`[]` default values appearing even on non-cross-spine calls is a byte-for-byte
compatible superset of today's output — today those keys are simply never populated at all because
the field doesn't exist client-side yet, so this is purely additive from the JSON consumer's point of
view; nothing currently reads and depends on their *absence*.)

## Priority 4 — the existing footer pattern (D-05/D-06)

`cmd/engram/client_list.go:64-79` (the whole conditional block is lines 68-76 in the file as read this
session — CONTEXT.md's `:70-75` is off by roughly the header-comment lines but points at the same
block):

```go
if format == formatText {
	if err := renderMemoryTable(cmd.OutOrStdout(), resp.Msg.GetMemories(), false); err != nil {
		return err
	}
	// The footer is data (total, next page token), not a
	// diagnostic, so it goes to stdout alongside the table.
	if resp.Msg.GetNextPageToken() != "" {
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "total: %d  next_page_token: %s\n",
			resp.Msg.GetTotal(), resp.Msg.GetNextPageToken())
	} else {
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "total: %d\n", resp.Msg.GetTotal())
	}
	return err
}
```

Exact shape:
- Renders the table first via `renderMemoryTable`, then conditionally appends one more `fmt.Fprintf`
  line to the **same writer** (`cmd.OutOrStdout()`), i.e. stdout, after the table — never interleaved.
- The footer's *content* is conditional-on-presence: `next_page_token` only appears in the line when
  non-empty; otherwise a shorter one-field line. `total` always prints.
- Format is a plain `key: value` line, tab/space-separated (two spaces between fields here), not
  JSON, not a proto text encoding — a hand-formatted human line, matching the table renderer's
  `text/tabwriter`-adjacent (but not tabwriter-itself, this line is a plain `Fprintf`) style.

### `client_search.go` baseline today: **no footer at all**

Read in full this session (`cmd/engram/client_search.go:63-66`):

```go
if format == formatText {
	return renderMemoryTable(cmd.OutOrStdout(), resp.Msg.GetMemories(), true)
}
return renderJSON(cmd.OutOrStdout(), resp.Msg)
```

`searchCmd`'s text-mode `RunE` branch calls `renderMemoryTable` and returns immediately — no `total`,
no `next_page_token` line, no footer of any kind exists on `engram search` today. This is the correct
baseline for D-06's "byte-identical for every invocation that does not pass `--cross-spine`" claim: for
`search`, that baseline is simply "table, nothing after it"; for `list`, the baseline is "table, then
the existing `total:`/`total: ... next_page_token: ...` line, unchanged." A regression test for D-06
should snapshot **both** baselines (search: no footer line at all when `--cross-spine` is absent; list:
the existing footer line, verbatim, when `--cross-spine` is absent) before adding the new coverage
footer logic, and assert both stay byte-identical post-change.

## Priority 5 — exit-code and usage-error plumbing

`usageErrorf` (`cmd/engram/client_common.go:214-218`):

```go
func usageErrorf(format string, a ...any) error {
	return &cliError{code: exitUsage, err: fmt.Errorf(format, a...)}
}
```

It wraps a formatted error in `*cliError{code: exitUsage}` — `exitUsage` is the constant `2`
(`client_common.go:197`). `searchCmd`'s existing `--query` check (`client_search.go:35-37`) is the
idiom to copy verbatim for the new guard:

```go
if searchQuery == "" {
	return usageErrorf("--query is required")
}
```

The D-01/D-04 guard should follow the exact same shape: a function (D-02's shared helper, e.g.
`func validateScopeCrossSpine(scope string, crossSpine bool) error` in `client_common.go`) that returns
`usageErrorf(...)` on the two client-rejected rows and `nil` otherwise, called from both `RunE` bodies
before `clientFromFlags`/dialing.

`client_common_test.go:29-53` is the anti-drift table gate to imitate for D-03's own test:

```go
func TestExitCodeForConnectErrTable(t *testing.T) {
	cases := []struct {
		code connect.Code
		want int
	}{ /* one row per connect.Code, 16 total */ }
	if len(cases) != 16 {
		t.Fatalf("test table has %d entries, want 16 (one per connect.Code)", len(cases))
	}
	for _, c := range cases {
		t.Run(c.code.String(), func(t *testing.T) { /* assert exitCodeForConnectErr(...) == c.want */ })
	}
	t.Run("not a connect.Error", func(t *testing.T) { /* assert default-case behavior */ })
}
```

The idiom: an explicit **count assertion** (`len(cases) != 16`) guards against someone silently
deleting a row and the test staying green on a shrunk table — the D-03 test should use the same
`len(cases) != N` guard over its own matrix (4 rows: scope empty/non-empty × cross_spine true/false, or
5 if the client's stricter row-2 rejection is itself asserted as a distinct case).

### Precedent caveat: `--offset`/`--cursor-mode` mutual exclusivity is enforced **server-side**, not client-side

CONTEXT.md cites `client_list.go:86` ("mutually exclusive with a non-zero --offset") as "precedent for a
mutually-exclusive pair in this file set." Verified this session: that line is only the flag's **Usage
string** (documentation). The actual enforcement lives **server-side**, in
`internal/server/connectapi.go:174-186`:

```go
// Enforce the cursor_mode/offset mutual exclusion (documented on the proto ...
if req.Msg.CursorMode && req.Msg.Offset > 0 {
	return nil, connectError(argErrFieldsf(..., []string{"cursor_mode", "offset"}, "cursor_mode is mutually exclusive with offset"))
}
```

There is **no existing client-side pre-flight mutual-exclusion check anywhere in `cmd/engram` today**
(confirmed by reading `client_list.go`/`client_search.go` in full — the only client-side pre-flight
check that exists is `searchCmd`'s empty-`--query` check). D-01/D-04's client-side guard is therefore
the **first** instance of this pattern in `cmd/engram`, not a second instance of an established one.
This doesn't change what to build — `usageErrorf` is still the right primitive — but the planner should
not describe this as "following an existing client-side precedent"; the precedent is the *documentation
convention* (each flag's help naming the other) and the *exit-code idiom* (`usageErrorf`/D-17), not a
prior client-side mutual-exclusion check to literally copy structurally.

## Existing tests affected by the behavior change

**No existing test pins `engram search --query x` with no `--scope`.** Read `client_search_test.go` in
full this session: every existing test that reaches `SearchMemories` passes `--scope repo:x` explicitly
(e.g. `client_search_test.go:35`). There is no test today asserting what happens when `--scope` is
omitted and `--cross-spine` is not passed — so the behavior change (today: dials, server rejects with
`argErrf(...)` → `CodeInvalidArgument` → exit 2 client-side after a real network round trip; after this
phase: rejected client-side before dialing, still exit 2, zero round trips) breaks **no** existing test.
This must still be its own **new** test, using the same `svc.searchCalls`/`svc.listCalls == 0` idiom
already established for other client-side pre-flight rejections
(`client_search_test.go:188-189`, `client_search_test.go:213-214`, `client_list_test.go:221-222` —
`TestClientSearchMissingServerURLIsUsageError`, `TestClientSearchMissingQueryIsUsageError`,
`TestClientListMissingServerURLIsUsageError`). The stub-server harness
(`cmd/engram/clienttest_test.go`) counts RPC invocations per method (`searchCalls`/`listCalls` fields,
incremented in the stub's `SearchMemories`/`ListMemories` methods), so "the guard fired before dialing"
is asserted the same way "the missing --server check fired before dialing" already is.

## `ENGRAM_REQUIRE_QDRANT` false-green trap: does NOT apply to `cmd/engram` tests

Engram memory `478rhhmhb0` warns `ENGRAM_REQUIRE_QDRANT` is not honored by `internal/store`'s test
dialer, and store tests can silently `t.Skip` while the package still reports `ok`. **Confirmed this
session: `cmd/engram` tests are pure in-process unit tests with no dependency on Qdrant, real or
faked, at all.** `startStubServer` (`cmd/engram/clienttest_test.go:84-97`) mounts the **real generated
Connect handler** (`engramv1connect.NewEngramServiceHandler`) over an `httptest.Server`, standing in for
the whole engram server via a hand-built `stubEngramService` (embeds
`engramv1connect.UnimplementedEngramServiceHandler` and overrides only `SearchMemories`/
`ListMemories`/`StoreMemory` with test-supplied closures). There is no `t.Skip` path anywhere in this
harness and no environment-variable gate at all — every test in this package always runs, and a `PASS`
here is a genuine pass, not a silently-skipped one. Any new test proposed for this phase (the D-01/D-04
guard test, the D-03 parity test, the D-05/D-06 footer tests) sits entirely inside this same harness and
carries the same guarantee.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Catalog entry for a new flag | A second, hand-maintained flag-list literal | Nothing — `collectFlags`/`buildCatalog` already derive it live (Priority 1) | Any hand-maintained duplicate is the exact class of drift D-15 was built to prevent. |
| Client-side rejection wrapping | A bespoke error type or `os.Exit` call in the `RunE` body | `usageErrorf(...)` returning `*cliError{code: exitUsage}` | `Execute()` (`root.go:59-64`) already consults `ExitCode()` via `errors.As`; a bespoke path would bypass the taxonomy and the catalog's own exit-code table (`TestCatalogExitCodesMatchMapper`). |
| Footer formatting | A second table-rendering helper | Plain `fmt.Fprintf` to `cmd.OutOrStdout()`, matching `client_list.go`'s existing footer line shape | Consistency with the one existing footer; no new rendering abstraction needed for one more conditional line. |

**Key insight:** every piece of this phase already has a direct structural analog somewhere in
`cmd/engram` (an existing flag, an existing guard idiom, an existing footer, an existing catalog
test) — the work is applying each analog to a new field, not inventing new mechanism.

## Common Pitfalls

### Pitfall 1: Treating the D-03 parity test as "free" because both sides "live in this repo"

**What goes wrong:** Assuming a single `_test.go` file can import both `cmd/engram`'s guard and
`internal/server`'s `effectiveSearchScope` because they're in the same module.

**Why it happens:** "Same repo" does not mean "same importability." `package main` cannot be imported
by anything (verified empirically, Priority 2); an unexported identifier in `package server` is
invisible outside that package regardless of module boundaries.

**How to avoid:** Decide explicitly (per Priority 2's two options) before writing the plan's task list,
and write the decision into the plan rather than discovering the compile error mid-execution.

**Warning signs:** A plan task that says "write a test in `cmd/engram` that calls
`server.effectiveSearchScope`" without first exporting it will fail to compile the moment it's
attempted.

### Pitfall 2: Describing D-04's rejection as matching an existing client-side precedent

**What goes wrong:** Citing `--offset`/`--cursor-mode` as a prior client-side mutual-exclusion check to
copy structurally.

**Why it happens:** The flag help text documents the relationship; the enforcement itself is
server-side (`connectapi.go:174-186`), not in `cmd/engram` at all.

**How to avoid:** Cite the `usageErrorf`/D-17 exit-code idiom and the empty-`--query` check as the
actual structural precedent (both genuinely live in `cmd/engram`); treat `--offset`/`--cursor-mode` only
as precedent for **documentation phrasing** ("each flag's help names the other"), not for enforcement
code.

### Pitfall 3: Assuming the JSON lane needs a task because "the fields are new"

**What goes wrong:** Writing a task to "add `searched_scopes`/`scopes_truncated` to the JSON renderer."

**Why it happens:** The fields are new to the *CLI's request/response vocabulary* but not new to the
generated Go types — they've existed on both response messages since Phase 3, and `renderJSON` is
generic over `proto.Message` with `EmitDefaultValues: true` already set. No renderer code change is
needed (Priority 3, confirmed against both message types' generated struct tags).

**How to avoid:** Do not write a JSON-lane task at all for this phase; the only JSON-lane verification
needed is a regression test asserting the two fields already appear (which incidentally also proves the
claim, since no code changed to make them appear).

## Code Examples

### Adding the flag (mirrors existing `BoolVar` pattern in both files)

```go
// Source: cmd/engram/client_search.go:76 (existing --full flag, same shape)
searchCmd.Flags().BoolVar(&searchFull, "full", false, "return full content instead of summaries")
// New flag follows the identical BoolVar shape:
searchCmd.Flags().BoolVar(&searchCrossSpine, "cross-spine", false,
	"span every scope you can read; mutually exclusive with --scope")
```

### The shared guard call site (D-01/D-02), mirroring the existing empty-query check

```go
// Source: cmd/engram/client_search.go:35-37 (existing pattern to extend)
if searchQuery == "" {
	return usageErrorf("--query is required")
}
// New (D-02's shared helper, called identically from listCmd's RunE):
if err := validateScopeCrossSpine(searchScope, searchCrossSpine); err != nil {
	return err
}
```

### The count-assertion test idiom for the D-03 matrix (mirrors `client_common_test.go:29-53`)

```go
// Source: cmd/engram/client_common_test.go:29-53 (structural pattern, adapt row count)
func TestValidateScopeCrossSpineMatrix(t *testing.T) {
	cases := []struct {
		name       string
		scope      string
		crossSpine bool
		wantErr    bool
	}{
		{"empty scope, cross-spine off", "", false, true},   // matches effectiveSearchScope's reject
		{"empty scope, cross-spine on", "", true, false},    // matches effectiveSearchScope's accept
		{"scope set, cross-spine off", "repo:x", false, false}, // matches effectiveSearchScope's accept
		{"scope set, cross-spine on", "repo:x", true, true},    // D-04: client is STRICTER than the server here
	}
	if len(cases) != 4 {
		t.Fatalf("test table has %d entries, want 4 (2x2 matrix)", len(cases))
	}
	// ... run each case, asserting error-or-nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `engram search --query x` with no `--scope` dials the server, gets a Connect `InvalidArgument`, exits 2 | Same exit code, rejected client-side before dialing | This phase | Zero round trips on the common misuse case; identical exit code so no script-visible contract change beyond "no network call happened." |

**Deprecated/outdated:** none — this phase adds capability, it does not remove or replace anything.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Exact wording for the new `--cross-spine`/`--scope` help strings and the coverage footer's text (beyond the sketched phrasing CONTEXT.md's `<specifics>` section already endorses in principle) is left to the planner/implementer. | Code Examples, Priority 4 | Low — CONTEXT.md explicitly delegates exact wording to the planner; this is not a verified-vs-assumed gap, just an open stylistic choice. |
| A2 | Recommendation (a) in the Summary (exporting `effectiveSearchScope`) is a judgment call, not something CONTEXT.md locked. | Priority 2 | Medium — if the user prefers option (b) instead, the plan's D-03 test must be written and worded differently (explicitly non-drift-proof), and this should be surfaced back to the user before planning proceeds, since it changes what "CLI-only" scope means for this phase. |

**If empty:** N/A — see table above; both entries are explicitly flagged, not silently assumed.

## Open Questions

1. **Does the user want `internal/server` touched at all to make D-03's parity test genuinely
   compile-linked, or is a weaker same-repo-but-not-literally-linked test acceptable?**
   - What we know: the two options and their tradeoffs (Priority 2).
   - What's unclear: which the user prefers — CONTEXT.md's own language ("goes RED the moment either
     moves") implies option (a)'s guarantee was intended, but the "Files this phase edits" list
     (canonical_refs) does not include any `internal/server` file, implying option (b) was assumed
     compatible without the export being checked.
   - Recommendation: surface this explicitly at plan-review or discuss a one-line addendum to
     CONTEXT.md before planning locks task boundaries — this is a scope-boundary question, not an
     implementation detail.

## Environment Availability

Skipped — this phase has no external tool/service/runtime dependencies beyond the repo's already-verified
Go toolchain (`go version go1.26.5 darwin/arm64`, confirmed available) and `task` (`3.52.0`, confirmed
available). No new packages, no new services, no database/network dependency beyond what already exists
in `cmd/engram`'s test harness (in-process `httptest.Server`, no live Qdrant).

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go's standard `testing` package, `go test` |
| Config file | none — driven by `Taskfile.yaml`'s `test` task |
| Quick run command | `go test ./cmd/engram/... -run 'CrossSpine|Guard|Catalog|Footer'` (adjust `-run` to the new test names once named) |
| Full suite command | `task` (lint + full repo test suite, per `CLAUDE.md`'s "Task runner" convention) |

### Phase Requirements → Test Map

No `REQ-*` IDs are mapped to this phase (see `<phase_requirements>`). The table below maps this phase's
own acceptance criteria (from CONTEXT.md's `<decisions>`) instead, since that is the contract this
phase must satisfy.

| Criterion | Behavior | Test Type | Automated Command | File Exists? |
|---|---|---|---|---|
| D-01/D-04 empty-scope and both-flags rejections | Client rejects (scope="", cross-spine=false) and (scope set, cross-spine=true) before dialing, exit 2 | unit | `go test ./cmd/engram/... -run TestValidateScopeCrossSpine` (name TBD by planner) | ❌ Wave 0 |
| D-02 single shared helper | Both `searchCmd` and `listCmd` call the same guard function | unit / static | grep-based or reflection-based check that both `RunE` bodies invoke the one helper — OR simply covered transitively by both commands' own guard tests passing against one function | ❌ Wave 0 (helper doesn't exist yet) |
| D-03 parity with `effectiveSearchScope` | Guard's accept/reject matrix matches (or, for the one deliberate divergence, documents why it diverges from) the server rule | unit | `go test ./cmd/engram/...` or `./internal/server/...`, depending on the Priority-2 location decision | ❌ Wave 0 — blocked on the Priority 2 decision |
| D-05/D-06 footer only on cross-spine calls, byte-identical output otherwise | Non-cross-spine invocations of `search`/`list` produce identical stdout to today; cross-spine invocations gain exactly one footer line | unit | `go test ./cmd/engram/... -run 'Footer|CrossSpine'` | ❌ Wave 0 |
| D-07 catalog carries the new flag with non-empty Usage | Already covered — no new test needed | unit (pre-existing) | `go test ./cmd/engram/... -run TestCatalogEnumeratesEveryFlag` | ✅ pre-existing, passes automatically once the flag lands |
| Existing behavior regression (no `--scope`, no `--cross-spine`) | New test: 0 RPC calls, exit 2 | unit | `go test ./cmd/engram/... -run TestClientSearchMissingScope` (name TBD) | ❌ Wave 0 — no existing test covers this input at all (confirmed above) |

### Sampling Rate

- **Per task commit:** `go test ./cmd/engram/...` (fast, in-process, no live Qdrant dependency — see
  the `ENGRAM_REQUIRE_QDRANT` section above; this package's tests are never silently skipped).
- **Per wave merge:** `task` (full lint + repo-wide test suite), plus the phase-close gates this
  milestone has used throughout: `go vet ./...`, `task license:check`, `task proto:lint`,
  `task proto:gen`/`task ui:build` zero-drift checks, `git diff --exit-code <phase-base> -- go.mod
  go.sum` (this phase adds zero dependencies, so this must stay clean), `git diff --exit-code
  <phase-base> -- internal/` (should stay clean **unless** the Priority 2 decision is (a), in which case
  exactly the one-line export in `internal/server/tools.go` — and nothing else in `internal/` — is the
  expected, sole exception; state this explicitly in the plan if option (a) is chosen).
- **Phase gate:** Full suite green before `/gsd-verify-work`; additionally re-run
  `TestCatalogEnumeratesEveryFlag`, `TestCatalogExitCodesMatchMapper`, and
  `TestCatalogDocumentsFlagParseExitCode` explicitly as a named check in the phase-close summary, since
  these three pre-existing tests are what *prove* D-07 without any new code — their continued green
  status is direct evidence for the catalog side of D-00's acceptance bar.

### Wave 0 Gaps

- [ ] No test file exists yet for the new `validateScopeCrossSpine` (or equivalently-named) guard
  helper — needs a new `_test.go` addition (likely in `client_common_test.go`, alongside the existing
  `TestExitCodeForConnectErrTable`/`TestResolveOutputFormat` table tests, following their idiom).
- [ ] No test exists for `engram search --query x` with no `--scope` at all (confirmed above) — this
  is a genuine coverage gap in the *current* tree, not something this phase regresses; the new guard
  test doubles as filling it.
- [ ] Framework install: none — `go test`/`task` are already fully configured; no new test
  infrastructure needed.
- [ ] Blocked pending the Priority 2 decision: the exact shape of the D-03 parity test (and which
  package it lives in) cannot be finalized until the export question is resolved.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---|---|---|
| V2 Authentication | no | This phase does not touch bearer-token verification; unchanged from Phase 1/2. |
| V3 Session Management | no | Not applicable — stateless RPC calls, unchanged. |
| V4 Access Control | yes (indirectly) | The client guard is a **usability/fail-fast** improvement, not an authorization boundary — the real authz gate remains server-side (`ownerScopeFilter`, Phase 3, untouched by this phase). The client guard must never be treated as a substitute for the server's authz check; it only prevents a confusing round trip. |
| V5 Input Validation | yes | `usageErrorf`-wrapped client-side validation, mirroring the existing `--query`-required check; the pattern is already standard in this codebase. |
| V6 Cryptography | no | Not touched. |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---|---|---|
| Client-side guard drifting looser than the server's actual rule, silently widening what a caller believes is being enforced | Tampering / Information Disclosure (via false sense of restriction) | The D-03 parity test (Priority 2) exists precisely to catch this drift — this is why its compile-reachability matters, not a nice-to-have. A cosmetically-similar-but-untethered duplicate test (Priority 2 option (b)) is a materially weaker mitigation and should be labeled as such in the plan if chosen. |
| A client-side "fail fast" check masking a genuine server-side authorization change (e.g., if `effectiveSearchScope`'s rule changes to something more permissive, the client could keep rejecting valid calls, or vice versa: the client could start accepting calls the server would reject, producing a confusing dial-then-fail loop) | Denial of Service (self-inflicted, availability of the feature) | Same mitigation — parity test, kept genuinely linked to the server rule. |

## Sources

### Primary (HIGH confidence — read directly this session)
- `cmd/engram/catalog.go` (full file) — D-07 verdict basis.
- `cmd/engram/catalog_test.go` (full file) — proof the catalog flag test is self-updating.
- `cmd/engram/client_search.go`, `cmd/engram/client_list.go`, `cmd/engram/client_common.go`,
  `cmd/engram/client_common_test.go:1-67`, `cmd/engram/clienttest_test.go` (full file),
  `cmd/engram/root.go` (full file) — flag wiring, guard idiom, footer baseline, exit-code taxonomy,
  test harness confirmation.
- `internal/server/tools.go:1360-1410` — `effectiveSearchScope`/`searchedScopes` exact logic and doc
  comments.
- `internal/server/connectapi.go:150-215` — the `cursor_mode`/`offset` server-side enforcement (the
  corrected precedent).
- `gen/go/engram/v1/engram.pb.go` (grep + line reads at 457-843) — confirmed `CrossSpine`/
  `SearchedScopes`/`ScopesTruncated` field presence and JSON tags on both request/response pairs.
- `CLAUDE.md:80-93` — current Memory-contract wording for `cross_spine` (D-09's edit target).
- `docs-site/src/content/docs/guides/cli.md` (full file), `docs-site/src/content/docs/guides/upgrade.md`
  (headings + lines 1-45) — D-08's edit targets and the v0.12.0 precedent structure.
- `docs-site/src/content/docs/reference/tools.md` (grep for `cross_spine`) — existing MCP-facing
  documentation pattern for the same field, for consistency.
- `.planning/config.json` — confirmed `workflow.nyquist_validation: true`.
- Empirical Go-toolchain test (built in scratchpad, this session) — confirmed `package main` cannot be
  imported by any other package.

### Secondary (MEDIUM confidence)
- None — every claim in this document was verified directly against source in this session; no
  claim relies on an unverified web search or training-data recollection.

### Tertiary (LOW confidence)
- None.

## Metadata

**Confidence breakdown:**
- Standard stack: N/A — no new libraries; this phase adds zero dependencies (confirmed against
  CONTEXT.md's explicit statement and this repo's standing zero-new-Go-deps constraint for the
  milestone).
- Architecture: HIGH — every file, line range, and behavior claim was read from the live tree this
  session, including one empirical Go-toolchain test to settle the package-import question definitively
  rather than reasoning from memory.
- Pitfalls: HIGH — each pitfall traces to a specific, cited line range that would otherwise mislead
  the planner if trusted uncritically from CONTEXT.md's summaries alone.

**Research date:** 2026-08-02
**Valid until:** Effectively indefinite for the cited line ranges (this is a closed, already-shipped
codebase with no external API drift risk) — but re-verify line numbers if any of Phase 3's or Phase 2's
files are touched by an intervening change before this phase executes.
