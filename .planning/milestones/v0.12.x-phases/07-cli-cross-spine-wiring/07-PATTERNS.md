<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright 2026 Sean Brandt -->

# Phase 7: CLI Cross-Spine Wiring - Pattern Map

**Mapped:** 2026-08-02
**Files analyzed:** 5 (4 edits in `cmd/engram`, 1 additive export in `internal/server`)
**Analogs found:** 5 / 5 — every file to be touched has a live in-file analog; nothing routes to RESEARCH.md's generic examples.

Note on line numbers: CONTEXT.md's cited ranges are close but a few are off by 1-4 lines against
the tree read this session (2026-08-02). Real ranges are cited below; where they diverge from
CONTEXT.md the divergence is called out.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog (same file, prior line range) | Match Quality |
|---|---|---|---|---|
| `cmd/engram/client_search.go` | CLI command (route+controller) | request-response | itself — `--full` flag (`:76`), empty-query guard (`:35-37`), request literal (`:46-55`) | exact (extend in place) |
| `cmd/engram/client_list.go` | CLI command (route+controller) | request-response | itself — `--offset`/`--cursor-mode` flags (`:86`, `:94`), request literal (`:44-56`), footer (`:64-79`) | exact (extend in place) |
| `cmd/engram/client_common.go` | utility/shared helper | request-response (validation) | itself — `usageErrorf` (`:216-218`), `resolveOutputFormat` (`:174-188`) as the "returns typed value + usageErrorf" shape | exact |
| `cmd/engram/client_common_test.go` | test | N/A | itself — `TestExitCodeForConnectErrTable` (`:29-67`) count-assertion table idiom | exact |
| `internal/server/tools.go` | service/domain rule | CRUD (validation) | itself — `effectiveSearchScope` (`:1374-1382`) gains an exported wrapper right beside it | exact (additive only) |

## Pattern Assignments

### `cmd/engram/client_search.go` (CLI command, request-response)

**Analog:** itself (existing flags/guard/request-literal in the same file)

**Flag declaration pattern** — verbatim, lines 70-81 (confirmed current):
```go
func init() {
	addClientFlags(searchCmd)
	searchCmd.Flags().StringVar(&searchQuery, "query", "", "search query (required)")
	searchCmd.Flags().StringVar(&searchScope, "scope", "", "scope filter")
	searchCmd.Flags().Uint64Var(&searchK, "k", 0, "max results (0 = server default)")
	searchCmd.Flags().StringSliceVar(&searchTags, "tags", nil, "tag filter (records must carry ALL listed tags)")
	searchCmd.Flags().BoolVar(&searchFull, "full", false, "return full content instead of summaries")
	searchCmd.Flags().StringVar(&searchCreatedAfter, "created-after", "", "RFC3339 inclusive lower bound on created_at")
	searchCmd.Flags().StringVar(&searchCreatedBefore, "created-before", "", "RFC3339 exclusive upper bound on created_at")
	searchCmd.Flags().StringSliceVar(&searchCategories, "categories", nil, "category filter (ANY listed category)")
	rootCmd.AddCommand(searchCmd)
}
```
Add `var searchCrossSpine bool` to the `var (...)` block (lines 15-24) and one more `BoolVar` line
here, e.g.:
```go
searchCmd.Flags().BoolVar(&searchCrossSpine, "cross-spine", false,
	"span every scope you can read; mutually exclusive with --scope")
```
Also update the existing `--scope` line's Usage string to name `--cross-spine` back (D-00
bidirectional naming — see the `client_list.go` `--offset`/`--cursor-mode` analog below for the
exact bidirectional-naming shape to copy).

**Guard call site pattern** — verbatim, lines 32-37 (the empty-`--query` check to extend, not
replace):
```go
RunE: func(cmd *cobra.Command, _ []string) error {
	// Reject an empty --query before building anything: the client's
	// own semantic validation, which is exactly what D-17 reserves
	// exit 2 for.
	if searchQuery == "" {
		return usageErrorf("--query is required")
	}
```
New guard call goes here, before `resolveOutputFormat`/`clientFromFlags` (i.e., before any
network-adjacent work), following the exact same "return usageErrorf(...) inline, no wrapping"
shape:
```go
	if err := validateScopeCrossSpine(searchScope, searchCrossSpine); err != nil {
		return err
	}
```

**Request literal pattern** — verbatim, lines 46-55 (add one field, same struct-literal style, no
reordering of existing fields):
```go
resp, err := client.SearchMemories(cmd.Context(), connect.NewRequest(&engramv1.SearchMemoriesRequest{
	Query:         searchQuery,
	Scope:         searchScope,
	K:             searchK,
	Tags:          searchTags,
	Full:          searchFull,
	CreatedAfter:  searchCreatedAfter,
	CreatedBefore: searchCreatedBefore,
	Categories:    searchCategories,
	// new: CrossSpine: searchCrossSpine,
}))
```

**No footer today** — lines 61-66, the baseline D-06 must keep byte-identical when
`--cross-spine` is absent:
```go
// D-12: return nil regardless of how many memories came back — an
// empty result set is a legitimate answer, not a failure.
if format == formatText {
	return renderMemoryTable(cmd.OutOrStdout(), resp.Msg.GetMemories(), true)
}
return renderJSON(cmd.OutOrStdout(), resp.Msg)
```
The new coverage-footer call must be inserted between `renderMemoryTable` and the `return`,
conditioned on `resp.Msg.GetScopesTruncated()` / non-empty `resp.Msg.GetSearchedScopes()` per
D-06 — i.e. only print anything when a cross-spine query actually ran.

---

### `cmd/engram/client_list.go` (CLI command, request-response)

**Analog:** itself (existing flags/footer/request-literal in the same file)

**Bidirectional flag-naming precedent** — verbatim, lines 86 and 94 (the exact "each flag names
the other" shape D-04's help text must copy):
```go
listCmd.Flags().Uint64Var(&listOffset, "offset", 0, "offset-for-UI paging; mutually exclusive with --cursor-mode")
...
listCmd.Flags().BoolVar(&listCursorMode, "cursor-mode", false, "opt into cursor paging on the first (tokenless) page; mutually exclusive with a non-zero --offset")
```
Copy this exact pattern for `--scope` / `--cross-spine`: each Usage string names the other flag by
its literal `--flag-name` spelling. Note this precedent is **documentation-only** — see the
Shared Patterns section below; the enforcement half has no client-side precedent in this repo and
must be newly authored via `usageErrorf`.

**Existing footer block — D-05's exact reuse target**, lines 64-79 (CONTEXT.md cited `:70-75`;
actual conditional block is `:70-76`, table call at `:65`):
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
return renderJSON(cmd.OutOrStdout(), resp.Msg)
```
Exact shape to copy for the coverage footer: conditional-on-presence, plain `fmt.Fprintf` to
`cmd.OutOrStdout()` (never `tabwriter`, never a second call to `renderMemoryTable`), placed after
the existing `total:` line so text output reads table → total-footer → coverage-footer, appended
only, never interleaved or reordered — this preserves D-06's byte-identical baseline for the
non-cross-spine case.

**Guard call site** — insert directly after `resolveOutputFormat`/before `clientFromFlags`
(there is no pre-existing guard in `listCmd` to extend, unlike `client_search.go` — this is the
first client-side pre-flight check in this file):
```go
if err := validateScopeCrossSpine(listScope, listCrossSpine); err != nil {
	return err
}
```

**Request literal pattern** — verbatim, lines 44-56 (add one field, same style):
```go
resp, err := client.ListMemories(cmd.Context(), connect.NewRequest(&engramv1.ListMemoriesRequest{
	Scope:         listScope,
	Limit:         listLimit,
	Offset:        listOffset,
	Categories:    listCategories,
	Visibility:    listVisibility,
	Tags:          listTags,
	Full:          listFull,
	CreatedAfter:  listCreatedAfter,
	CreatedBefore: listCreatedBefore,
	PageToken:     listPageToken,
	CursorMode:    listCursorMode,
	// new: CrossSpine: listCrossSpine,
}))
```

**Flag declaration block** — lines 82-96; add `listCrossSpine bool` to the `var (...)` block
(lines 16-28) and one more line in `init()`:
```go
listCmd.Flags().BoolVar(&listCrossSpine, "cross-spine", false,
	"span every scope you can read; mutually exclusive with --scope")
```

---

### `cmd/engram/client_common.go` (shared helper, D-02's guard lands here)

**Analog:** `usageErrorf` (lines 214-218) as the exit-2 primitive, `resolveOutputFormat`
(lines 174-188) as the "pure function, typed error return" shape to copy for the new guard's
signature:
```go
// usageErrorf returns a *cliError carrying exitUsage — the client's own
// semantic validation (D-17 reserves exit 2 for exactly this).
func usageErrorf(format string, a ...any) error {
	return &cliError{code: exitUsage, err: fmt.Errorf(format, a...)}
}
```
```go
// resolveOutputFormat maps the --output flag value and the caller's TTY
// state to a concrete format. Taking isTTY as a parameter rather than
// calling isTerminal internally is what lets a table test force both
// branches without a pty.
func resolveOutputFormat(flagVal string, isTTY bool) (outputFormat, error) {
	switch flagVal {
	...
	default:
		return formatJSON, usageErrorf(`--output must be "json" or "text", got %q`, flagVal)
	}
}
```
New helper (D-02), same file, adjacent to `usageErrorf`:
```go
// validateScopeCrossSpine is the shared D-01/D-02/D-04 pre-flight guard used
// by both searchCmd and listCmd, before dialing. It is deliberately STRICTER
// than internal/server's effectiveSearchScope on one row of the matrix — see
// the D-03 parity test doc comment in client_common_test.go for the
// documented asymmetry.
func validateScopeCrossSpine(scope string, crossSpine bool) error {
	if crossSpine && scope != "" {
		return usageErrorf("--scope and --cross-spine are mutually exclusive")
	}
	if !crossSpine && scope == "" {
		return usageErrorf("--scope is required unless --cross-spine is set")
	}
	return nil
}
```
(Exact wording is Claude's Discretion per CONTEXT.md — the shape above is the load-bearing part.)

No changes needed to `renderJSON` (lines 263-274, already `EmitDefaultValues: true` — emits
`searched_scopes`/`scopes_truncated` unconditionally) or `renderMemoryTable` (lines 279-307,
already prints the per-result `SCOPE` column — D-11 satisfied). Confirmed by direct read this
session; do not add a JSON-lane task.

---

### `cmd/engram/client_common_test.go` (test, D-03 parity + guard unit tests)

**Analog:** `TestExitCodeForConnectErrTable`, verbatim lines 29-67 — the count-assertion table
idiom to imitate structurally for the new guard test:
```go
func TestExitCodeForConnectErrTable(t *testing.T) {
	cases := []struct {
		code connect.Code
		want int
	}{
		{connect.CodeCanceled, exitUnavailable},
		... // 16 rows total
	}
	if len(cases) != 16 {
		t.Fatalf("test table has %d entries, want 16 (one per connect.Code)", len(cases))
	}
	for _, c := range cases {
		t.Run(c.code.String(), func(t *testing.T) {
			err := connect.NewError(c.code, errors.New("boom"))
			if got := exitCodeForConnectErr(err); got != c.want {
				t.Errorf("exitCodeForConnectErr(%v) = %d, want %d", c.code, got, c.want)
			}
		})
	}
	t.Run("not a connect.Error", func(t *testing.T) { ... })
}
```
New D-03 parity test copies this shape with a 4-row 2x2 matrix (scope empty/non-empty ×
cross-spine on/off), with an explicit doc comment stating the row-2 asymmetry (client stricter
than `effectiveSearchScope` there) so a future reader does not "fix" the divergence:
```go
func TestValidateScopeCrossSpineParity(t *testing.T) {
	cases := []struct {
		name       string
		scope      string
		crossSpine bool
		clientErr  bool
		serverErr  bool
	}{
		{"empty scope, cross-spine off", "", false, true, true},
		{"empty scope, cross-spine on", "", true, false, false},
		{"scope set, cross-spine off", "repo:x", false, false, false},
		{"scope set, cross-spine on", "repo:x", true, true, false}, // D-04: client is stricter here
	}
	if len(cases) != 4 {
		t.Fatalf("test table has %d entries, want 4 (2x2 matrix)", len(cases))
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clientErr := validateScopeCrossSpine(c.scope, c.crossSpine) != nil
			if clientErr != c.clientErr {
				t.Errorf("validateScopeCrossSpine(%q, %v) error presence = %v, want %v", c.scope, c.crossSpine, clientErr, c.clientErr)
			}
			_, serverErr := server.EffectiveSearchScope(c.scope, c.crossSpine)
			if (serverErr != nil) != c.serverErr {
				t.Errorf("EffectiveSearchScope(%q, %v) error presence = %v, want %v", c.scope, c.crossSpine, serverErr != nil, c.serverErr)
			}
			// The load-bearing assertion: the client must never ACCEPT what
			// the server would REJECT (client can only be stricter, never
			// looser) — never assert blanket equality.
			if !clientErr && serverErr != nil {
				t.Errorf("client accepted (%q, %v) but server would reject it — client guard has drifted looser than effectiveSearchScope", c.scope, c.crossSpine)
			}
		})
	}
}
```
This test file already imports nothing from `internal/server` today; it will need
`"github.com/seanb4t/engram/internal/server"` added to its import block. This is compatible with
`TestClientFilesImportBoundary`'s denylist (lines 205-225 of this same file): that gate's file
walk explicitly `continue`s on any file with the `_test.go` suffix (line 172-174), so
`client_common_test.go` importing `internal/server` does not trip its own gate — only
*production* `client_*.go` files are denylisted from `internal/server`.

**Harness for "guard fires before dialing"** — copy `stubEngramService`'s call-counter idiom
verbatim from `cmd/engram/clienttest_test.go:29-43` (`searchCalls`/`listCalls int` fields
incremented in the overridden RPC methods) and assert `svc.searchCalls == 0` /
`svc.listCalls == 0` after invoking the CLI with an input the guard should reject — same shape as
the existing `TestClientSearchMissingQueryIsUsageError`-style tests this file's siblings already
use (`client_search_test.go:188-189`, `213-214`; `client_list_test.go:221-222` — not read this
session but cited consistently by RESEARCH.md; verify call-site names before citing them in the
plan).

---

### `internal/server/tools.go` (the one authorized `internal/` edit)

**Analog:** `effectiveSearchScope` itself, verbatim lines 1374-1382:
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
Add an exported, additive, behavior-preserving wrapper immediately after it (per CONTEXT.md's
2026-08-02 amendment authorizing exactly this one export):
```go
// EffectiveSearchScope is the exported form of effectiveSearchScope, for
// cmd/engram's D-03 parity test only. It changes no behavior — it exists
// solely so the client-side guard can be pinned against the server's actual
// rule at compile time, per Phase 7's D-03 amendment.
func EffectiveSearchScope(scope string, crossSpine bool) (string, error) {
	return effectiveSearchScope(scope, crossSpine)
}
```
This is the sole permitted change to any file under `internal/`; `07-VALIDATION.md`'s containment
gate (`git diff --exit-code <phase-base> -- internal/` allowing exactly this one-line addition)
enforces it stays that way.

## Shared Patterns

### Client-side usage-error exit code (D-01/D-04)
**Source:** `cmd/engram/client_common.go:214-218` (`usageErrorf`), called at
`cmd/engram/client_search.go:35-37` (existing empty-`--query` check)
**Apply to:** the new `validateScopeCrossSpine` guard, called from both `searchCmd.RunE` and
`listCmd.RunE` before `clientFromFlags`/dialing.
```go
if searchQuery == "" {
	return usageErrorf("--query is required")
}
```

### Bidirectional flag-naming in help text (D-00/D-04 documentation half)
**Source:** `cmd/engram/client_list.go:86` and `:94` (`--offset` / `--cursor-mode`)
**Apply to:** `--scope` and `--cross-spine` help strings in both `client_search.go` and
`client_list.go`. **Caveat:** this precedent covers *wording only* — the actual mutual-exclusion
*enforcement* for `--offset`/`--cursor-mode` lives server-side
(`internal/server/connectapi.go:174-186`, not read verbatim this session but confirmed present by
RESEARCH.md Priority 5) and is not itself a client-side pattern to copy structurally. D-04's guard
is the first client-side relational rejection in this codebase.

### Conditional stdout footer (D-05/D-06)
**Source:** `cmd/engram/client_list.go:70-76` (the `total:`/`next_page_token:` block)
**Apply to:** the new coverage footer in both `client_search.go` (which has no footer today — the
new footer is the *first* footer line search ever prints, only when cross-spine ran) and
`client_list.go` (append after the existing `total:` line).

### Test count-assertion anti-drift gate
**Source:** `cmd/engram/client_common_test.go:29-53` (`TestExitCodeForConnectErrTable`)
**Apply to:** the new `TestValidateScopeCrossSpineParity` (4-row matrix) and any other new table
test this phase adds.

## No Analog Found

None — every file this phase touches already has a directly-reusable in-file or same-package
pattern (RESEARCH.md's own "Don't Hand-Roll" table independently confirms this). The one place
with no prior *client-side* precedent is the D-04 mutual-exclusion enforcement itself (see Shared
Patterns caveat above) — the planner should treat `usageErrorf` + the empty-query-check idiom as
the correct primitive to compose from, not search further for a mutual-exclusion analog that does
not exist in `cmd/engram`.

## Metadata

**Analog search scope:** `cmd/engram/*.go` (all `client_*.go`, `catalog.go`, `clienttest_test.go`),
`internal/server/tools.go:1350-1410`.
**Files scanned:** `client_search.go`, `client_list.go`, `client_common.go`,
`client_common_test.go`, `clienttest_test.go`, `catalog.go` (partial), `tools.go` (partial) — 7
files read directly this session, no re-reads of overlapping ranges.
**Pattern extraction date:** 2026-08-02
