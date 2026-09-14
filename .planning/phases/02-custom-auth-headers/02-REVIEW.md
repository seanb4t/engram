---
phase: 02-custom-auth-headers
reviewed: 2026-09-14T14:14:44Z
depth: standard
files_reviewed: 24
files_reviewed_list:
  - internal/setup/runtime.go
  - internal/setup/claudecode.go
  - internal/setup/opencode.go
  - internal/setup/codex.go
  - internal/setup/generic.go
  - internal/setup/claudecode_test.go
  - internal/setup/opencode_test.go
  - internal/setup/codex_test.go
  - internal/setup/generic_test.go
  - internal/setup/plan_test.go
  - cmd/engram/setup.go
  - cmd/engram/setup_test.go
  - cmd/engram/setup_delegation_test.go
  - cmd/engram/operator_view_setup_test.go
  - cmd/engram/clienttest_test.go
  - cmd/engram/golden_test.go
  - cmd/engram/destructive_test.go
  - cmd/engram/testdata/help.golden
  - cmd/engram/testdata/catalog.golden
  - internal/setupgen/setupgen.go
  - internal/setupgen/setupgen_test.go
  - internal/store/redevidence_harness_test.go
  - skill/engram/commands/engram-setup.md
  - docs-site/src/content/docs/guides/agent-setup.md
findings:
  critical: 0
  warning: 1
  info: 2
  total: 3
status: issues_found
---

# Phase 02: Code Review Report

**Reviewed:** 2026-09-14T14:14:44Z
**Depth:** standard
**Files Reviewed:** 24
**Status:** issues_found

## Summary

This phase adds `engram setup --header NAME=ENVVAR` (an additional, orthogonal HTTP header
alongside `--auth`), per-runtime rendering, the Codex decline, `ENGRAM_HEADERS`, the flat
`headers` row facet, the 5th `setupgen` case, docs, and phase red-evidence.

I traced the five highest-value security properties named in the review brief end to end:

1. **No header VALUE is ever read or emitted.** Confirmed by grep: the only `env.Getenv` call
   anywhere in `internal/setup` is opencode's pre-existing `XDG_CONFIG_HOME` read
   (`internal/setup/opencode.go:190`); no code path reads a header's `EnvVar` via `Getenv`, and
   `HeaderSpec` only ever carries `Name`/`EnvVar` (names), never a resolved value. No log
   statement exists anywhere in the diffed files.
2. **`setupParseHeaders` validation** (`cmd/engram/setup.go:276-304`) is airtight against every
   crafted-spec class I tried: embedded CR/LF (rejected — the anchored `^...$` regexes contain no
   `.` and no newline in either character class, and Go's RE2 engine does not implicitly allow a
   multi-line match without `(?m)`), Unicode lookalikes (rejected — both regexes are strictly
   ASCII), empty NAME/ENVVAR, a bare `=`, multiple `=` (the extra `=` lands in ENVVAR and fails
   the POSIX-identifier regex), leading/trailing whitespace — all rejected. No error message
   ever echoes the right-hand side of a spec (verified by reading the four `usageErrorf` call
   sites and cross-checking against `TestSetupHeaderRejectsMalformedEnvVar`'s negative-echo
   assertions).
3. **Ordering determinism.** `sortedHeaders` (runtime.go) sorts by `strings.ToLower(Name)`;
   `genericHeaders.MarshalJSON` reproduces `encoding/json`'s exact HTML-escaped byte output by
   calling `json.Marshal` per key/value (confirmed via `TestGenericHeaders`'s
   `zero-header-byte-identity` subtest, which pins the literal `<...>` escaping); the
   `Authorization` detection is `strings.EqualFold` in both `setupParseHeaders` and
   `genericHeaders.MarshalJSON`. See WR-01 below for one narrow gap in this area.
4. **The Codex decline** (`internal/setup/codex.go:87-96`) is unconditional, runs before
   `opts.Auth` is even inspected or `env.HomeDir()` is called (proven by
   `TestCodexDeclinesHeaders`, which fails the test if `HomeDir` is invoked), and wraps
   `ErrHeaderUnsupported` (not `ErrAuthModeUnsupported` — confirmed by `errors.Is` assertions in
   both directions). The reason text names only header NAMEs, never an EnvVar name or value.
5. **Red-evidence harness.** I ran `go test ./internal/store/ -run TestRedEvidencePatchesAreLive
   -count=1 -v`: both phase 01 and phase 02 subtests pass, phase 01's four-entry mapping is
   unmodified (pure addition in the diff), and phase 02's five patches each apply cleanly, drive
   their mapped target test RED, and revert cleanly.

I also ran `go test ./internal/setup/... ./cmd/engram/... ./internal/setupgen/... -count=1` (all
pass) and `go run ./internal/surfacesgen --check-setup` (exit 0 — the checked-in
`skill/engram/commands/engram-setup.md` region is not drifted from `Render()`).

I found no Critical issues. One Warning (a trust-boundary gap inside `internal/setup` itself,
not reachable through the shipped CLI) and two Info items (documentation/maintenance nits).

## Warnings

### WR-01: `internal/setup` has no Authorization-collision or duplicate-name guard of its own for `Options.Headers`

**File:** `internal/setup/runtime.go:183-206`, `internal/setup/generic.go:213-229`,
`internal/setup/claudecode.go:143-205`, `internal/setup/opencode.go:112-152`

**Issue:** Every header-name safety property this phase claims — "one owner per header"
(`Authorization` is reserved for `--auth`) and "no two headers collide case-insensitively" — is
enforced *only* at the CLI boundary, in `cmd/engram/setup.go`'s `setupParseHeaders`
(`runtime.go`'s own doc comment on `HeaderSpec.Name` even says so: "validated at the CLI
boundary... this package carries already-validated data"). `internal/setup.Runtime.Plan()` never
re-checks this itself. Concretely, a direct caller of the package (a future CLI surface, a test
helper, or any other in-repo consumer that isn't `cmd/engram/setup.go`) that supplies
`Options{Auth: "bearer", Headers: []HeaderSpec{{Name: "authorization", EnvVar: "OTHER"}}}` would
get:

- claude-code: `--header 'Authorization: Bearer ${ENGRAM_TOKEN}' --header 'authorization: ${OTHER}'`
  — two colliding `Authorization` headers on the same `claude mcp add` invocation.
- generic: a `genericHeaders` map with two *distinct* Go map keys, `"Authorization"` (added by
  the bearer arm) and `"authorization"` (added from `opts.Headers`), both surviving into the
  marshaled JSON document as separate header entries.

Separately, `sortedHeaders` (`runtime.go:197-206`) sorts via `slices.SortFunc`, which the
standard library explicitly does **not** guarantee to be stable (unlike `slices.SortStableFunc`).
Today this can't produce visible nondeterminism because `setupParseHeaders` guarantees every
`HeaderSpec.Name` in a validated set is unique case-insensitively, so no two elements ever compare
equal — but that invariant lives entirely in the CLI layer, one file away from the sort that
depends on it. If `internal/setup` ever grows a second production caller (or the CLI's dedup logic
regresses), D-08's "deterministic ordering" guarantee would silently stop holding for
case-colliding names, in addition to the double-`Authorization`-header defect above.

This is not reachable through the shipped `engram setup` binary today — `cmd/engram/setup.go`'s
`setupResolve` always calls `setupParseHeaders` before any `Runtime.Plan()` is reached, and that
is the only production call site with headers wired in. It is a defense-in-depth gap in an
`internal/setup` package whose own doc comments elsewhere describe fairly strong guarantees ("no
code path in `internal/setup` can place a header VALUE on argv, in Config, in preview text, or in
a log" — `runtime.go:29-37`) that stop just short of covering header-NAME collisions.

**Fix:** Either (a) add a cheap case-insensitive uniqueness/`Authorization`-collision check inside
`internal/setup` itself (e.g., a small helper `validateHeaders(hs []HeaderSpec) error` called at
the top of each `Runtime.Plan()`, mirroring the existing per-runtime error-wrapping style), so the
package's own safety property does not depend entirely on a caller it does not control; or (b), at
minimum, switch `sortedHeaders` to `slices.SortStableFunc` (a one-line change) so that if the
CLI-layer invariant is ever violated, the resulting order is at least deterministic rather than
silently flapping between runs — and add a doc-comment note on `Options.Headers` making the
"caller must pre-validate uniqueness" contract explicit rather than implicit.

## Info

### IN-01: `setupHeadersSummary` duplicates `sortedHeaders`'s comparator instead of sharing it

**File:** `cmd/engram/setup.go:91-101`, `internal/setup/runtime.go:197-206`

**Issue:** `cmd/engram/setup.go`'s `setupHeadersSummary` re-implements the exact same
case-insensitive sort comparator (`strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))`)
that `internal/setup`'s unexported `sortedHeaders` already implements, because `sortedHeaders` is
unexported and cannot be called from `cmd/engram`. This is a reasonable package-boundary
consequence rather than a bug, and D-08's own design rationale explicitly avoids a shared
*formatter* across runtimes — but the *sort key* itself is duplicated verbatim in two files with
no test tying them together beyond end-to-end byte comparisons. A future change to one comparator
(e.g., switching to a locale-aware fold) without the other would silently desynchronize the
`headers` row facet's order from the rendered command's own header order, without any compiler
error to catch it.

**Fix:** Export a small `setup.SortHeaders(hs []HeaderSpec) []HeaderSpec` (or keep
`sortedHeaders` unexported but expose the comparator as a package-level `var
HeaderNameLess func(a, b HeaderSpec) bool`) so `cmd/engram` reuses the single canonical
comparator instead of re-typing it.

### IN-02: Help text's description of the ENVVAR grammar undersells what is actually rejected

**File:** `cmd/engram/setup.go:768-772` (long description), `docs-site/src/content/docs/guides/agent-setup.md:126-127`

**Issue:** Both the CLI long-help text and the docs-site guide describe the ENVVAR restriction as
"a right-hand side containing `$`, `{`, whitespace, or `:` is rejected." The actual grammar
(`setupHeaderEnvVarRe = ^[A-Za-z_][A-Za-z0-9_]*$`) rejects *any* character outside
`[A-Za-z0-9_]` (plus a leading digit) — including, for example, a hyphen (`x-key=MY-KEY` is
rejected per `TestSetupHeaderRejectsMalformedEnvVar`) or a period. An operator who reads the help
text and tries `--header x-key=MY-API-KEY` (a very plausible env-var-naming choice) will be
rejected for a reason the help text's stated character list does not warn about, and will have to
infer the real rule ("POSIX identifier") from the trailing clause alone.

**Fix:** Either broaden the example list to note "or any character other than letters, digits, and
underscore," or drop the specific character examples and rely solely on the already-present "the
right-hand side must be a POSIX identifier" wording, which is the actually-accurate statement of
the rule.

---

_Reviewed: 2026-09-14T14:14:44Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
