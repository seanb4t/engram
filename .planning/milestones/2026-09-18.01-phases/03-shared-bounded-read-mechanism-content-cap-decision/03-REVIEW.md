---
phase: 03-shared-bounded-read-mechanism-content-cap-decision
reviewed: 2026-09-19T00:00:00Z
depth: standard
files_reviewed: 4
files_reviewed_list:
  - internal/config/validate.go
  - internal/config/validate_test.go
  - internal/server/tools.go
  - .planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/red-evidence/03-01-config-accepts-zero.patch
findings:
  critical: 0
  warning: 0
  info: 0
  total: 0
status: clean
---

# Phase 03: Code Review Report (re-review, iteration 2)

**Reviewed:** 2026-09-19T00:00:00Z
**Depth:** standard
**Files Reviewed:** 4
**Status:** clean

## Summary

Incremental re-review of fix commit `7e6113cc`, which addressed WR-01 from the
prior review (`03-REVIEW.iter2.md`): `Config.Validate()` previously parsed the
three always-enforced memory caps (`ENGRAM_MEMORY_MAX_CONTENT_BYTES`/`_MAX_TAGS`/
`_MAX_TAG_BYTES`) and `ENGRAM_MEMORY_MAX_SUMMARY_BYTES` with `strconv.ParseUint`
(uint64 range), while runtime enforcement (`positiveIntOrDefault`/
`maxMemorySummaryBytes` in `internal/server/tools.go`) parsed the same strings
with `strconv.Atoi` (platform `int`, effectively int64). A value between
`math.MaxInt64` and `math.MaxUint64` therefore validated cleanly at startup but
was silently replaced by the compiled-in default at runtime, with only a
`slog.Warn` — a validated-vs-enforced divergence.

The fix introduces two exported, shared parsers in `internal/config`:
`ParsePositiveIntCap` (for the three always-enforced caps) and
`ParseNonNegativeIntCap` (for `MAX_SUMMARY_BYTES`), both built on
`strconv.Atoi`. `Config.Validate` and `internal/server`'s
`positiveIntOrDefault`/`maxMemorySummaryBytes` now call the exact same
functions, so the validated range and the enforced range can never diverge
again — verified this by reading both call sites and confirming no other
parser in the codebase touches these four fields (`rg` for
`MaxContentBytes|MaxTags|MaxTagBytes|MaxSummaryBytes` across non-test `.go`
files turns up only `internal/config/validate.go`, `internal/config/config.go`
struct-tag comments, and `internal/server/tools.go`).

Independently verified, beyond re-reading the diff:
- Boundary correctness: added a throwaway test (run, then deleted, confirmed
  `git status` clean afterward — no source files were modified by this
  review) exercising both new parsers at the exact boundary. `ParsePositiveIntCap`
  and `ParseNonNegativeIntCap` both accept `"9223372036854775807"`
  (`math.MaxInt64`, e.g. `n=9223372036854775807 err=<nil>`) and both reject
  `"9223372036854775808"` (`math.MaxInt64+1`) with `strconv.Atoi: ... value out
  of range` — the boundary itself is not off-by-one in either direction.
  `ParsePositiveIntCap` still rejects `"0"` and `"-1"` (`must be greater than
  0`); `ParseNonNegativeIntCap` still accepts `"0"` (`n=0 err=<nil>`) and
  rejects `"-1"`.
- `ENGRAM_MEMORY_MAX_SUMMARY_BYTES`'s "0 disables" semantics are unchanged:
  `maxMemorySummaryBytes` still returns the parsed `n` (including 0) without
  coercing it to the default; only a parse *error* falls back to 512. Confirmed
  by re-reading the function body and by the passing
  `TestMemoryCapsRejectZeroAndNonPositive` subtest
  `"ENGRAM_MEMORY_MAX_SUMMARY_BYTES/0 still disables (D-18, unchanged)"`.
- No platform-dependent behavior introduced: both new functions use
  `strconv.Atoi`, matching the pre-fix runtime side exactly (no width change);
  the fix's whole point is that `Validate()` now uses the identical function,
  so the two sides agree regardless of platform `int` width.
- `positiveIntOrDefault` and `maxMemorySummaryBytes` (`internal/server/tools.go`)
  correctly switched to calling `config.ParsePositiveIntCap`/
  `config.ParseNonNegativeIntCap` instead of independent `strconv.Atoi` calls;
  `strconv` remains validly imported/used elsewhere in the file (e.g.
  `storeFromConfig`, `buildUsageQueue`, `summaryMaxChars`), so no dead/unused
  import was left behind.
- New test `TestOverflowValueRejectedByValidate` is meaningful: ran it (and the
  surrounding `TestMemoryCapsRejectZeroAndNonPositive`/`TestValidateHappyPath`/
  `TestValidateFieldRules`) — all pass against current `HEAD`. The test's logic
  (mutate each of the four fields to `math.MaxInt64+1`, assert `Validate()`
  errors and names the env var) is a direct regression guard for the fixed
  divergence, would fail against the pre-fix `ParseUint`-based validation (as
  the fix report documents having proven), and is not tautological — `go
  build`/`go vet` on `internal/config` and `internal/server` are clean.
- Regenerated red-evidence patch (`03-01-config-accepts-zero.patch`) still
  targets the same guarantee as before — zero-rejection for the always-enforced
  caps — just relocated to the new shared `ParsePositiveIntCap` (removes its
  `if n <= 0 { return 0, errors.New(...) }` block rather than
  `validatePositiveCap`'s old inline `case n == 0`). Read the patch and the
  current file side by side: the hunk's context lines match the current
  `ParsePositiveIntCap` body exactly, and applying it would make the function
  accept 0/negative again — the same failure mode the original patch proved,
  not a weaker one. (Per instructions, did not run
  `TestRedEvidencePatchesAreLive`; the fix report documents having reproved RED
  then reverted cleanly, which the static read here corroborates.)
- IN-01 and IN-02 from the prior review are out of scope for this fix and were
  not re-raised, per instructions.

No critical or warning issues found in the fix's scope. `status: clean`.

---

_Reviewed: 2026-09-19T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
