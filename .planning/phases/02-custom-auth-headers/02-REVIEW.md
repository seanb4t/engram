---
phase: 02-custom-auth-headers
reviewed: 2026-09-14T15:05:00Z
depth: standard
files_reviewed: 5
files_reviewed_list:
  - internal/setup/runtime.go
  - internal/setup/claudecode_test.go
  - cmd/engram/setup.go
  - cmd/engram/testdata/help.golden
  - docs-site/src/content/docs/guides/agent-setup.md
findings:
  critical: 0
  warning: 0
  info: 0
  total: 0
status: clean
---

# Phase 02: Code Review Report (iteration 2)

**Reviewed:** 2026-09-14T15:05:00Z
**Depth:** standard
**Files Reviewed:** 5
**Status:** clean

## Summary

Re-review of the exact two-commit delta since iteration 1 (`c6e26877..HEAD`): `2578e426`
(WR-01 fix — `sortedHeaders` gains a byte-wise tiebreak, `Options.Headers`' doc comment states the
validation boundary, `TestSortedHeadersTotalOrder` added) and `68aad2c3` (IN-02 fix — the ENVVAR
grammar is restated positively in the CLI long-help and `agent-setup.md`, `help.golden`
regenerated). Both prior findings (WR-01, IN-02) are verified fixed and correct; no new defect was
introduced by either commit.

**1. `sortedHeaders`'s tiebreak is a genuine total order.** The comparator now falls back to
`strings.Compare(a.Name, b.Name)` (raw byte comparison) whenever the primary
`strings.ToLower`-keyed comparison ties. Since `strings.Compare` returns 0 only for byte-identical
strings, two *distinct* `HeaderSpec.Name` values can never compare as equal under the combined
comparator — this holds unconditionally, independent of any Unicode case-folding edge case (e.g.
Turkish dotted/dotless I, or any other rune where `strings.ToLower`'s simple per-rune mapping might
behave unexpectedly): the byte-wise fallback only needs the *inputs* to differ, not the folding
semantics to be "correct" in any locale sense. `slices.SortFunc`'s lack of a stability guarantee is
therefore fully neutralized — the output order can no longer depend on sort-implementation
internals. D-08 is preserved: `sortedHeaders` only orders the *extra* headers; the auth-mode header
(`Authorization`, when present) is still authored first by each runtime's own case arm, ahead of
the call into `sortedHeaders`, unchanged by this diff (`runtime.go:213-215`'s doc comment states
this explicitly and no runtime file in the diff touches that ordering).

**2. `TestSortedHeadersTotalOrder` pins the property non-vacuously.** It feeds `"a-key"`/`"A-key"`
(which tie under the primary `ToLower` key) in both input orders and asserts the *same* output
order (`upper` before `lower`, matching `strings.Compare("A-key","a-key") < 0`) in both cases. Prior
to the fix, `slices.SortFunc` over a two-element equal-under-primary-key slice could plausibly
preserve input order pre-swap, so the two subtests (`lower-then-upper`, `upper-then-lower`) are a
real differential check on the tiebreak, not a tautology. I confirmed by reading the fixer's
recorded RED/GREEN evidence (`02-REVIEW-FIX.md`) that reverting the tiebreak line makes exactly the
`lower-then-upper` subtest fail while `upper-then-lower` still passes — consistent with a
stability-dependent bug being caught by this specific test shape, not a coincidence.

**3. The new `Options.Headers` doc-comment contract is accurate.** It states that headers arrive
pre-validated by `cmd/engram/setup.go`'s `setupParseHeaders` (no `Authorization` collision, no
case-insensitive duplicate names, every `EnvVar` a POSIX identifier), that `internal/setup` orders
and renders but does not re-validate, and that a direct caller skipping that validation owns the
consequences. This matches the actual code: no `Runtime.Plan()` implementation re-checks
`Authorization` collision or duplicate names (confirmed by reading `runtime.go`'s `Select`/
`sortedHeaders`, which contain no such guard), and the only production call site
(`cmd/engram/setup.go`'s `setupResolve` → `setupParseHeaders`) does perform that validation before
any `Runtime.Plan()` is reached. The comment does not overclaim a guarantee the code doesn't
provide, and doesn't underclaim what `sortedHeaders` now guarantees (deterministic ordering even
under a hypothetical case-collision).

**4. The restated ENVVAR grammar is factually correct and `help.golden` is live-consistent.**
`setupHeaderEnvVarRe = ^[A-Za-z_][A-Za-z0-9_]*$` requires: first character a letter or underscore
(never a digit), remaining characters letters/digits/underscore. The new sentence — "a POSIX-shell
identifier (ASCII letters, digits, and underscore, not starting with a digit)" — matches this
exactly, in both `cmd/engram/setup.go`'s long-help and `docs-site/.../agent-setup.md`. This is
materially more accurate than the iteration-1 wording it replaced, which listed only `$`, `{`,
whitespace, `:` as rejected and would have wrongly implied a hyphen (a very plausible env-var-naming
choice, e.g. `MY-API-KEY`) was accepted. I confirmed `cmd/engram/testdata/help.golden`'s "Additional
headers" section is byte-identical to the new long-help text, and ran the full `cmd/engram` test
suite (which includes the golden-vs-live-cobra-tree test at `golden_test.go:305`) — it passes,
confirming no drift between the golden file and the live `--help` tree. `catalog.golden` is
untouched by this diff (confirmed via `git diff --stat`), consistent with the `Short` summary line
being unchanged.

**5. No new defect.** The diff is minimal and surgical: `runtime.go` gains one `if` branch plus
doc-comment prose; `claudecode_test.go` gains one net-new test function; `cmd/engram/setup.go`,
`help.golden`, and `agent-setup.md` each get a one-paragraph wording substitution. The `Accepted
--auth modes:` block (D-01's "must stay untouched" constraint) is untouched in both the CLI source
and the golden file — confirmed by diffing only the "Additional headers" paragraph that follows it.
`gofmt -l` is clean on all three touched Go/text-adjacent files. No `os.Getenv` of a header value
was introduced, no new call site reaches `sortedHeaders` with unvalidated input, and no existing
test was weakened or deleted.

**Verification run this iteration:**
- `go test ./internal/setup/ -count=1` — PASS
- `go test ./cmd/engram -count=1` — PASS
- `go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1 -v` — PASS (all five
  phase-02 patches, including the two header-behavior ones, still apply/RED/revert cleanly)
- `gofmt -l` on all touched files — clean
- Manual diff of `cmd/engram/testdata/help.golden`'s "Additional headers" section against the
  `setupLongDescription()` source string — byte-identical
- `git diff --stat` confirms `catalog.golden` is not in the changed-file set

All reviewed files meet quality standards. No issues found this iteration.

---

_Reviewed: 2026-09-14T15:05:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
