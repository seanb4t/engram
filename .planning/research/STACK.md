# Stack Research

**Domain:** Go CLI + MCP server — spine curation, drift detection, near-duplicate
consolidation, and correct-by-reading interface audit (v0.13.x "Curation & Self-Evidence")
**Researched:** 2026-08-03
**Confidence:** HIGH — every recommendation is grounded in reading the actual vendored
source (`go.mod`, `$(go env GOMODCACHE)`) and the current `cmd/engram`/`internal/store`
call sites, not from memory.

## Headline Finding

**This milestone needs zero new Go dependencies.** All four investigated questions resolve
to "stdlib, or a capability already vendored, is sufficient" — continuing the same track
record as v0.10.x–v0.12.x. The only two libraries in play (`cobra` v1.10.2, `qdrant/go-client`
v1.18.3) are already in `go.mod` and already imported by the exact packages (`cmd/engram`,
`internal/store`) this milestone touches.

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `github.com/spf13/cobra` | v1.10.2 (already vendored, confirmed current upstream release) | `MarkFlagsMutuallyExclusive` / `MarkFlagsOneRequired` / `MarkFlagsRequiredTogether` for #453's declarative flag constraints | Read straight from `flag_groups.go` in the vendored module: this is exactly the "cobra ships this and the repo uses it zero times" gap #453 names. No upgrade needed — v1.10.2 already has all three APIs. |
| Go stdlib (`os`, `strings`, `bufio`) | Go 1.26 (project's pinned toolchain) | `file:line` citation-anchor drift detection for `engram spine-review` | `store.Citation.Excerpt` (`internal/store/store.go:290`) already caches the cited span's content **at store time**. Drift detection is therefore: read the current file at `Ref`, slice the current `Locator` line range, compare bytes to the cached `Excerpt`. That is `os.ReadFile` + `strings.Split(..., "\n")` + a slice compare — no parsing library needed for the general case, because citations are not Go-source-only (`Kind` is `file\|commit\|url\|repo` and the file may be docs, YAML, TS, anything). |
| Go stdlib (`go/parser`, `go/ast`, `go/token`) | Go 1.26 stdlib | Optional secondary signal, `.go`-file citations only: distinguish "content moved to a new line" from "content changed" | When `Ref` ends in `.go`, parse the file and check whether the `Excerpt` text still appears as (part of) some declaration's body anywhere in the AST, even if not at the exact cited `Locator`. This turns a naive line-shift (gofmt reformat, an import added above) from a false "drifted" into a correctly-classified "moved — update the anchor," without needing any third-party diff/AST library. Confine this to `.go` files only; don't try to generalize it to Markdown/YAML, where a plain text-contains fallback (still stdlib `strings.Contains`) does the same job. |
| `github.com/qdrant/go-client` | v1.18.3 (already vendored, already imported by `internal/store`) | Near-duplicate consolidation candidate generation for the curation skill | `qdrant.NewQueryID(id)` (confirmed present in `oneof_factory.go` of the vendored client) lets a caller run a similarity query using an **already-stored point's vector**, with no re-embedding round-trip. `internal/store.Store.Search` already calls `s.client.Query(ctx, &qdrant.QueryPoints{...})` twice (`store.go:943`, `store.go:1030`) — the near-dup scan is the same `Query` call shape, just built with `qdrant.NewQueryID(existingPointID)` instead of a fresh embedding, filtered by owner/scope the same way every other bulk read already is. Nothing to add. |
| Go stdlib (`net/http`, `context`) | Go 1.26 | CLI `--timeout` flag (#452) | `newHTTPClient` (`cmd/engram/client_common.go:113`) currently sets no `http.Client.Timeout` and no call site applies a `context.WithTimeout`. Both are one-line stdlib additions: set `Timeout` on the constructed `*http.Client` from a new `--timeout` flag (finite default, e.g. 30s), and/or wrap `cmd.Context()` with `context.WithTimeout` before the RPC call. No library required — this is precisely the kind of gap the zero-dependency constraint expects to be closed with stdlib. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| *(none)* | — | — | No supporting library is needed for structural drift detection, near-dup scoring, timeout handling, or flag-constraint declaration. Every one of those is either stdlib or already-vendored (see Core Technologies). |
| `github.com/modelcontextprotocol/go-sdk` | v1.7.0 (already vendored) | MCP tool/argument conditional-requirement prose (item C, MCP half) | Already the mechanism in use: `jsonschema:"required unless cross_spine"` on `SearchMemoriesArgs.Scope` (`internal/server/tools.go:598`) is the existing pattern. The audit's job is to **apply this same free-text jsonschema-tag convention** to every conditionally-required MCP argument that doesn't yet carry it (`effectiveSearchScope`'s siblings) — not to add a schema-description library. go-sdk's jsonschema generation already round-trips a struct-tag string into the tool's advertised schema; nothing else is needed. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| Hand-rolled golden-file test helper (stdlib `os`, `path/filepath`, `testing`, `os.Getenv`) | CLI `--help` / self-describe-catalog output-completeness regression tests | No golden-file library (`goldie`, `cupaloy`, `testscript`) is vendored or referenced anywhere in the repo (repo-wide search for `golden`/`testdata` returns nothing). The repo already has the harder half of this built: `cmd/engram/clienttest_test.go` invokes `rootCmd` with `SetOut(&outBuf)`/`SetArgs(args)` and captures output as a `bytes.Buffer` (pattern at `clienttest_test.go:145-150`). A golden-file layer is ~20 lines on top of that harness: write `testdata/<cmd>.golden`, compare `outBuf.String()` against it, and gate a rewrite behind an env var (e.g. `if os.Getenv("UPDATE_GOLDEN") == "1" { os.WriteFile(...) }`). This is the same pattern Go's own toolchain (`cmd/go`, `gofmt`) and most stdlib-only Go CLIs use; it needs no dependency. |
| `go test` building the full command tree | Catch a `MarkFlagsMutuallyExclusive`/`MarkFlagsOneRequired` typo before it ships | These calls **panic at command-construction time** (i.e., at `init()`/binary startup, confirmed in `flag_groups.go`: `panic(fmt.Sprintf("Failed to find flag %q...))`) if a named flag doesn't exist on that exact `*cobra.Command`. That means any existing `go test ./cmd/engram/...` run that constructs the command tree already catches a misspelled flag name — no new lint rule needed, just don't skip building the command tree in tests. |

## Installation

No `go get` is required. Every capability in this milestone is reachable from what's already
in `go.mod`:

```bash
# Nothing to install — cobra v1.10.2 and qdrant/go-client v1.18.3 are already vendored,
# and everything else is Go 1.26 stdlib (os, strings, bufio, go/parser, go/ast, go/token,
# net/http, context, path/filepath, testing).
go build ./...
```

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|--------------------------|
| stdlib `os.ReadFile` + line-slice compare against cached `Excerpt` | `go-git` (pure-Go git library) or shelling out to the system `git` binary (`git log -L`/`git blame -L`) for commit-level line-history tracking | Only if a future requirement needs "which commit introduced the drift" diagnostics rather than just "is it drifted." The system `git` binary via `os/exec` is the right escalation path (zero new Go module, and git is already a build/CI dependency of this repo) — `go-git` would be a new, fairly heavy dependency for a capability the excerpt-diff approach doesn't need. Do not add `go-git`. |
| `qdrant.NewQueryID` reusing a point's stored vector | Re-embedding every record's content through `internal/embed` and comparing fresh vectors | Never for the near-dup scan itself — that would double the embedder API cost and add nondeterminism (a re-embed of unchanged text isn't guaranteed byte-identical across provider-side model updates) for no benefit over querying the already-stored vector. Re-embedding is only appropriate for the existing `engram reindex` use case (migrating to a *different* embedder config), which this milestone doesn't touch. |
| Hand-rolled golden-file helper (stdlib) | `github.com/sebdah/goldie/v2` or `github.com/bradleyjkemp/cupaloy` | If the test surface grows enough that the hand-rolled ~20-line helper needs real features (diff rendering, `-update` CLI flag wired through `go test` flags, multiple golden formats). At the current scope (a handful of `--help` and self-describe-catalog snapshots) that threshold isn't met, and the zero-new-dependency track record argues for not crossing it preemptively. |
| stdlib `net/http.Client.Timeout` + `context.WithTimeout` | A retry/backoff library (e.g. `cenkalti/backoff`, already vendored elsewhere for the summary queue) | Only if the milestone scope grows to include automatic retry-on-timeout semantics. #452 only asks for a timeout that turns "hangs forever" into "fails after N seconds" — that's a `Timeout` field and a context deadline, not a retry policy; reusing `cenkalti/backoff` here would conflate two unrelated concerns for no requirement that asks for it. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|--------------|
| `go-git` (or any pure-Go git implementation) | Would be the milestone's first new dependency for a job the cached `Excerpt` field already makes unnecessary; also heavier than shelling to system `git` if commit history is ever needed | stdlib file read + `Excerpt` compare; `os/exec` + system `git` binary only if commit-level provenance is explicitly required later |
| `github.com/stretchr/testify` (direct import) | Already present only as an **indirect** transitive dependency (`go.mod:122`, pulled in by something else) — no engram package imports it directly today. Adding a direct import for this milestone's new tests would be the first direct testify usage in a project with a zero-new-dependency streak across four milestones | stdlib `testing` + manual comparisons, matching the existing test style throughout `cmd/engram/*_test.go` and `internal/store/*_test.go` |
| A golden-file testing library (`goldie`, `cupaloy`, `testscript`) | Not present in the repo; the existing `clienttest_test.go` harness already does the hard part (capturing cobra output to a buffer) — a library adds a dependency to save ~20 lines of `os.ReadFile`/`os.WriteFile` | Hand-rolled golden helper on top of the existing `SetOut`/`SetArgs` harness |
| A general-purpose AST/semantic-diff library (e.g. tree-sitter bindings, `go/types`-based semantic diffing) for citation drift | Massive overkill for "did this line's content change" — and citations aren't Go-only, so a Go-specific heavy tool can't cover the general case anyway | stdlib text compare (general case) + optional stdlib `go/ast` containment check (`.go`-file case only) |
| A CLI flag-validation framework beyond cobra's own (`urfave/cli` re-implementation patterns, `kong`, hand-rolled reflection-based validators) | cobra v1.10.2, already the CLI framework in use, already ships the exact three primitives #453 asks for | `cobra.Command.MarkFlagsMutuallyExclusive` / `MarkFlagsOneRequired` / `MarkFlagsRequiredTogether` |

## Stack Patterns by Variant

**If the citation `Ref` is a `.go` file:**
- Use a two-tier check: exact `Locator`-range byte compare against `Excerpt` first (cheap, catches the common case); if that fails, fall back to a `go/ast` containment scan of the whole file for the `Excerpt` text before declaring "drifted" rather than "moved."
- Because a Go-specific secondary signal meaningfully reduces false positives from routine refactors (gofmt, import reordering, a preceding function growing by a few lines) without needing a diff library — this is exactly the failure mode #355 (drifted `tools.go` line-number citations in test comments) demonstrates, and is the live fixture the spine-review verifier exists to catch.

**If the citation `Ref` is anything else (docs, YAML, config, a URL/commit citation):**
- Use only the plain-text compare (`file` kind) or skip structural drift checking entirely (`commit`/`url`/`repo` kinds have no line-range to drift).
- Because a general-purpose textual approach is the only one that covers non-Go anchors, and adding language-specific parsers for every possible `Ref` filetype is unbounded scope for zero marginal benefit over the text compare.

**If `Excerpt` is empty (it is optional — `validateCitations` in `internal/server/tools.go:834` enforces no minimum length):**
- Report the citation as **unverifiable**, a status distinct from **drifted** and **clean**.
- Because without a cached baseline there is nothing to diff against; treating "no excerpt" as "drifted" would produce a false-positive flood across every pre-existing citation stored before this milestone, and treating it as "clean" would silently skip real drift. A third bucket is the only fail-closed-but-honest option.

**If a cobra flag-group violation needs to surface through `engram`'s existing exit-code taxonomy:**
- Do not adopt `MarkFlagsMutuallyExclusive`/`MarkFlagsOneRequired`/`MarkFlagsRequiredTogether` without also deciding how their errors map to `exitUsage`/`exitGeneric`.
- Because `Command.ValidateFlagGroups()` (confirmed in the vendored `command.go:1010`, called after `PreRunE` but before `RunE`) returns a **plain `fmt.Errorf`** with no `ExitCode() int` method. Engram's own `exitCodeFromError` (`cmd/engram/root.go:75-84`) only special-cases errors satisfying that interface via `errors.As`; a native cobra flag-group violation therefore falls through to the `1` (`exitGeneric`) default — **not** `2` (`exitUsage`), which is what every hand-rolled `usageErrorf` call in this codebase already returns for the identical error class (see `validateScopeCrossSpine`, `client_common.go:234-242`). Adopting cobra's native validators as-is would silently introduce a *second* undocumented exit-code split, one command over from the exact problem #467 was opened to close. Either (a) wrap `ValidateFlagGroups()`'s return in a `*cliError{code: exitUsage}` at each command (e.g., in a shared `PreRunE`/post-parse hook), or (b) explicitly document that native-cobra-validated flag errors are exit 1 by design, matching the `migrate-remap-owner` D-09 carve-out already on record. Pick one and record it — this is exactly the kind of decision #467 exists to make explicit rather than incidental.

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|------------------|-------|
| `github.com/spf13/cobra` v1.10.2 | `github.com/spf13/pflag` v1.0.10 (already vendored) | No known incompatibility. `MarkFlags*` calls `c.mergePersistentFlags()` internally, so declare the constraint on the exact `*cobra.Command` whose `Flags()` will contain all named flags post-merge (i.e., after persistent flags from a parent are attached) — declaring it too early (before a child command's own flags are registered) causes the `panic("Failed to find flag ...")` fail-fast, which is a feature (caught at binary construction / in any test that builds the command tree) not a bug. |
| `github.com/qdrant/go-client` v1.18.3 | Qdrant server (CI-pinned per the `requireQdrant` gate to v1.18.2 per v0.10.x Phase 17 notes) | `NewQueryID` is a client-side request-shape helper; no server-side feature-flag or minimum-server-version concern beyond what `Query`/`QueryPoints` already require, which the codebase already depends on for every existing `Search` call. |
| cobra's `MarkFlags*` annotations | cobra's own `--help`/usage-template rendering | **Confirmed by reading `flag_groups.go`: none of the three annotation constants (`requiredAsGroupAnnotation`, `oneRequiredAnnotation`, `mutuallyExclusiveAnnotation`) appear anywhere outside that file** — cobra does **not** auto-generate help text describing the constraint (no "(cannot be used with --x)" hint gets added to `--help` output). The declarative APIs give you *enforcement* and shell-completion hiding, not *documentation*. Item C's "correct-by-reading" bar still requires hand-writing the constraint into each flag's help string (as `client_list.go` already attempts in prose today) — `MarkFlagsMutuallyExclusive` makes that prose *true* (enforced), it doesn't generate it. |

## Sources

- `/Volumes/Code/github.com/seanb4t/engram/go.mod` — confirmed exact vendored versions: cobra v1.10.2, pflag v1.0.10, qdrant/go-client v1.18.3, modelcontextprotocol/go-sdk v1.7.0, testify v1.11.1 (indirect only).
- `$(go env GOMODCACHE)/github.com/spf13/cobra@v1.10.2/flag_groups.go` and `command.go` (read directly) — confirmed `MarkFlagsMutuallyExclusive`/`MarkFlagsOneRequired`/`MarkFlagsRequiredTogether` semantics, panic-on-missing-flag behavior, `ValidateFlagGroups()` call site (`command.go:1010`, between `PreRunE` and `RunE`), and the absence of any help-text/usage-template integration for these annotations.
- `$(go env GOMODCACHE)/github.com/qdrant/go-client@v1.18.3/qdrant/oneof_factory.go` (read directly) — confirmed `NewQueryID(id *PointId) *Query` exists alongside `NewQueryDense`/`NewQueryNearest`, usable in the same `qdrant.QueryPoints{Query: ...}` shape `internal/store.Store.Search` already builds.
- `/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go` (read directly, lines 286-300, 908-1030, 2087-2260, 2614-2680) — confirmed `Citation.Excerpt`/`Locator` fields, existing `Search`/`Query` call shape, and the existing `Scroll`/`ScrollAndOffset` full-spine-walk pattern already used by `Reindex`/`PruneExpired` (so `spine-review` needs no new store-layer primitive to walk every record).
- `/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go` (read directly, lines 489-856) — confirmed the existing `jsonschema:"required unless cross_spine"` conditional-requirement-in-prose pattern already in production use, and `validateCitations`'s lack of a minimum `Excerpt` length (so "unverifiable" must be a distinct spine-review status, not folded into "drifted").
- `/Volumes/Code/github.com/seanb4t/engram/cmd/engram/{root,client_common,client_list}.go` (read directly) — confirmed the existing `cliError`/`ExitCode()`/`exitCodeFromError` taxonomy machinery (`exitGeneric=1`, `exitUsage=2`, `exitAuth=3`, `exitNotFound=4`, `exitUnavailable=5`), `newHTTPClient`'s missing `Timeout`, and the hand-rolled `validateScopeCrossSpine` mutual-exclusivity guard #453 wants replaced/backed by cobra's native primitive.
- `/Volumes/Code/github.com/seanb4t/engram/cmd/engram/clienttest_test.go` (read directly) — confirmed the existing `rootCmd.SetOut`/`SetArgs` output-capture test harness a golden-file layer would sit on top of, and confirmed (via repo-wide search) zero existing golden-file/testdata convention to extend rather than invent.
- Context7 `/spf13/cobra` (`llms.txt`) — cross-checked `MarkFlagsRequiredTogether`/`MarkFlagsMutuallyExclusive`/`MarkFlagsOneRequired` usage examples against the vendored source; agreed.
- Web search (2026-08-03) — confirmed v1.10.2 is cobra's current latest tagged release upstream (no newer version to adopt).

---
*Stack research for: engram v0.13.x — Curation & Self-Evidence*
*Researched: 2026-08-03*
