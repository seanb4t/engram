---
phase: 05-operator-correctness
reviewed: 2026-09-24T17:15:00Z
depth: standard
files_reviewed: 9
files_reviewed_list:
  - cmd/engram/exitcode_baseline_test.go
  - cmd/engram/golden_test.go
  - cmd/engram/operator_output_test.go
  - cmd/engram/operator_view.go
  - cmd/engram/operator_view_test.go
  - docs-site/src/content/docs/guides/cli.md
  - internal/keylinks/keylinks.go
  - internal/keylinks/keylinks_test.go
  - internal/store/migrate_converge_test.go
findings:
  critical: 0
  warning: 2
  info: 5
  total: 7
status: issues_found
---

# Phase 5: Code Review Report

**Reviewed:** 2026-09-24T17:15:00Z
**Depth:** standard
**Files Reviewed:** 9
**Status:** issues_found

## Summary

Scope: `git diff f69b9bd6..HEAD -- . ':!.planning'`, covering the five operator fixes OPS-01..OPS-05. The review traced each fix into the helpers it calls: `resetCommandFlagState`, `renderOperatorView`, `scrollAllPoints`, the `Store.Migrate` sweep loop, and `CheckWellFormed`.

Verified:
- `go test ./internal/keylinks/` passes.
- The targeted `cmd/engram` tests pass with `ENGRAM_RUNTIME`, `ENGRAM_HEADERS`, `ENGRAM_REINDEX_TARGET` and `ENGRAM_MIGRATE_OWNER` set.
- `TestMigrateBelowCursorInsertConverges` and `TestMigrateConvergesWithoutLock` pass against a live Qdrant testcontainer.
- The #502 keylinks split is sound. The scanner still sees fieldless items, and the exported parser filters them out.
- The below-cursor migrate test is non-vacuous. It asserts the cursor, the write ordering, and `Passes >= 2`.

Two fixes are incomplete. They are latent today, but each breaks an invariant that the phase claims to establish:
- The OPS-02 empty-row fix covers only the top-level object branch. The array-element branch still prints whitespace-only lines. This was reproduced.
- The OPS-01 helper says it neutralizes every `envDerivedFlagDefaults` entry for `TestExitCodeBaseline`. It does not do this for the two `setup` string-slice flags.

No security issues were found.

## Warnings

### WR-01: The array-element branch still emits whitespace-only rows (OPS-02 fix is incomplete)

**File:** `cmd/engram/operator_view.go:84-103` (affects `renderOperatorView`, `:464-468`)
**Issue:** Plan 05-02 made an object-valued top-level key that renders empty contribute zero rows. 05-02-SUMMARY D2 claims that "renderOperatorView never emits a whitespace-only line". The `case '[':` element loop has no such guard:
- an element that is `{}` goes through `viewRow`, which returns `""`;
- an element that is `[]` goes through `flattenNested`, which returns nil, and the join gives `""`;
- `""` and `null` go through `viewScalar`, which returns `""`.

Each case appends `""` to `rows`, and `renderOperatorView` prints it as `"    "`. A scratch copy of `operator_view.go` reproduced this:

```
  Items        <- {"items":[{}]}      (struct with all-omitempty fields)
    ␠␠␠␠
  Tags         <- {"tags":[""]}
    ␠␠␠␠
  Grid         <- {"grid":[[]]}
    ␠␠␠␠
```

This is the same defect class the plan fixed, one branch over. It also contradicts `renderOperatorView`'s documented contract: "never a trailing blank line". An array of structs with only omitempty fields, or a string slice holding `""`, hits it.
**Fix:** Keep the element count. Dropping an element would silently misstate cardinality, so fall back to the element's own JSON literal when its rendering is empty:
```go
for _, elem := range elems {
    var row string
    switch valueKind(elem) { /* ...existing cases assign row... */ }
    if row == "" {
        row = string(elem) // `{}`, `[]`, `""`, `null` — sanitization-safe: json.Marshal output
    }
    rows = append(rows, row)
}
```
Add a case to `TestViewFieldsEmptyNestedObjectRendersNoRows`, or a sibling test, that covers `[{}]` and `[""]`.

**Status:** Fixed in ccb1441e

### WR-02: `neutralizeEnvDerivedFlagDefaults` does not neutralize the `setup` string-slice flags in `TestExitCodeBaseline`

**File:** `cmd/engram/golden_test.go:62-65, 76-111`; `cmd/engram/exitcode_baseline_test.go:615-621`; root cause in `cmd/engram/clienttest_test.go:225-227`
**Issue:** The helper only blanks `pflag.Flag.DefValue`. Its doc says the caller's later `resetEveryCommandFlagState` "is what copies DefValue into the bound variable". The doc and the `envDerivedFlagDefaults` comment also say a new entry "is neutralized automatically in both the help/catalog goldens and TestExitCodeBaseline".

`resetCommandFlagState` returns early for `stringSlice` flags (`if f.Value.Type() == "stringSlice" { return }`). `setup`'s `--runtime` and `--header` are both `StringSliceVar`, so `setupRuntime` and `setupHeaders` keep their `init()`-time `os.Getenv("ENGRAM_RUNTIME")` and `os.Getenv("ENGRAM_HEADERS")` values in every baseline row. This is the #476 leak, still open for 2 of the 4 map entries.

It is latent only because the single `setup` row (`setup/bad-auth`) fails on `--auth` first. It still passes with `ENGRAM_RUNTIME=bogus ENGRAM_HEADERS='bad header'`. The next `setup` row whose outcome depends on runtime selection or header parsing will be ambient-env-dependent, despite the comment promising otherwise. The 05-01-SUMMARY D2 claim ("Neutralization covers every envDerivedFlagDefaults entry") is therefore untrue.
**Fix:** Neutralize the bound value for slice flags inside the helper itself, through pflag's `SliceValue` interface:
```go
orig := f.DefValue
f.DefValue = ""
if sv, ok := f.Value.(pflag.SliceValue); ok {
    origSlice := sv.GetSlice()
    _ = sv.Replace(nil)
    t.Cleanup(func() { _ = sv.Replace(origSlice) })
}
t.Cleanup(func() { f.DefValue = orig })
```
If you don't make that change, correct the two doc comments so they no longer claim coverage for the `setup` entries.

**Status:** Fixed in e0ef82fb — the helper now blanks the bound value too (`pflag.SliceValue.Replace` for slices, `Value.Set` otherwise) and restores value and `DefValue` in one cleanup

## Info

### IN-01: Cleanup order leaves `reindexTarget` / `migrateOwner` blank after the baseline test

**File:** `cmd/engram/exitcode_baseline_test.go:619-621`
**Issue:** `t.Cleanup` runs LIFO. The `doReset` cleanups from `resetEveryCommandFlagState` run before the `DefValue` restores from `neutralizeEnvDerivedFlagDefaults`. The reset therefore copies the still-blank `""` back into the bound variables, and only afterwards is `DefValue` restored to the ambient value. After the test, the process-global `reindexTarget` and `migrateOwner` no longer match their flag's `DefValue`. Separately, the helper's doc (`golden_test.go:79`) still says it protects against "a release version". Version pinning moved to `withGoldenDeterminism`.
**Fix:** Have the helper also set and restore the bound value (`f.Value.Set("")`, then restore in its own cleanup), so the result does not depend on cleanup order. Drop the "release version" clause from the helper's doc.

**Status:** Fixed in e0ef82fb (with WR-02)

### IN-02: The nested-command matching in the docs gate is loose

**File:** `cmd/engram/operator_output_test.go:810-828`
**Issue:** A `<group> <leaf>` key is accepted if the backticked `engram <group>` and the backticked `leaf` both appear anywhere in the list, independently. A future nested command whose leaf name matches any other backticked token (for example `` `status` ``), under a group mentioned as `` `engram <group>` ``, would pass without being listed. `strings.Index(doc, "### Operator commands")` also prefix-matches a longer heading.
**Fix:** Require the leaf inside the group's own parenthetical, or use the full `` `group leaf` `` form only. Anchor the heading with `(?m)^### Operator commands$`.

**Status:** Fixed in e04a7359 (with IN-05)

### IN-03: Line-number references in test comments were already stale when written

**File:** `cmd/engram/operator_view_test.go:638, 640, 763`
**Issue:** The comments point at "operator_view.go ~line 102" and "~line 91". After this phase's own edit, the branches are at lines 104 and 93.
**Fix:** Refer to the branches by name (`case '{':` in `viewFields`, the `case '[':` element branch) rather than by line number.

**Status:** Fixed in 6be892a8

### IN-04: "This subtest gates every other one below" is false

**File:** `internal/store/migrate_converge_test.go:443-450`
**Issue:** `t.Fatalf` inside a `t.Run` subtest ends only that subtest, so the sibling subtests still run. The siblings have independent assertions and are not vacuous. However, a fixture regression causes a cascade of misleading failures instead of the single gated failure the comment describes.
**Fix:** Move the `fires` and `cursor` precondition checks to the parent test body, before the first `t.Run`, using `t.Fatalf` there.

**Status:** Fixed in 323691ac

### IN-05: The operator-commands prose list is hard to parse

**File:** `docs-site/src/content/docs/guides/cli.md:139-144`
**Issue:** The list contains a nested "…, `migrate status`, and `migrate revert` (see …), `migrate-remap-owner`, …, and every …". It has two "and"s, which reads as the list ending twice.
**Fix:** Group the migrate verbs parenthetically, for example "`migrate` (and its `status` / `revert` subcommands; see the [Migrate guide](/guides/migrate/))". If you do this, update the docs gate as well: its nested-key fallback only accepts `` `engram migrate` `` plus the backticked leaf.

**Status:** Fixed in e04a7359 (with IN-02; the docs gate now accepts a leaf inside its group's parenthetical)

---

_Reviewed: 2026-09-24T17:15:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
