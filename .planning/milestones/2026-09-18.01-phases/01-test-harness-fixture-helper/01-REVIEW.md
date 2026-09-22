---
phase: 01-test-harness-fixture-helper
reviewed: 2026-09-19T02:15:00Z
depth: standard
files_reviewed: 2
files_reviewed_list:
  - internal/store/schemaversion_stamp_gate_test.go
  - internal/store/testdata/qdrantclient/bad_store_aliased_import.go.txt
findings:
  critical: 0
  warning: 0
  info: 0
  total: 0
status: clean
---

# Phase 01: Code Review Report

**Reviewed:** 2026-09-19T02:15:00Z
**Depth:** standard
**Files Reviewed:** 2
**Status:** clean

## Summary

Iteration 3 (final --auto iteration), incremental scope only: the diff introduced by fix
commit `be18a467`, which closed the iteration-2 WR-01 gap (`qdrantClientLocalNames`'s
aliased-`internal/store`-import fix had no dedicated regression test).

The fix report (`01-REVIEW-FIX.md`) claims the fixture `bad_store_aliased_import.go.txt`
needed adapting from a bare `return st.NewQdrantClient(host, port)` to a direct-assignment
shape (`c, err := st.NewQdrantClient(host, port); return c, err`), because
`qdrantClientLocalNames` only inspects `*ast.AssignStmt`/`*ast.ValueSpec` and the original
shape would have made the new test pass vacuously (trivially returning an empty map
regardless of whether alias resolution worked). Verified this claim directly by reading
`qdrantClientLocalNames`'s implementation (`internal/store/schemaversion_stamp_gate_test.go:980-1063`):
its second pass only walks `*ast.AssignStmt` nodes and inspects `assign.Rhs` for a
`*ast.CallExpr`, so a bare `return <call>` with no assignment is correctly outside its scan
surface — the report's reasoning holds.

Traced the new test (`TestQdrantClientLocalNamesDetectsAliasedStoreImport`,
`schemaversion_stamp_gate_test.go:1252-1277`) against the adapted fixture by hand:
`c, err := st.NewQdrantClient(host, port)` produces an `*ast.AssignStmt` with `Rhs[0]` a
`CallExpr` whose `Fun` is `*ast.SelectorExpr{X: Ident("st"), Sel: "NewQdrantClient"}`;
`storeNames` (from `importLocalNames`, resolving the `st "…/internal/store"` alias) contains
`"st"`, so `isConstructorSelector` returns true and `assign.Lhs[0]` (`"c"`) is registered —
matching the asserted `map[string]bool{"c": true}` exactly (the test also asserts no extra
keys). This is not a vacuous assertion: it requires alias resolution to actually work.

Independently reproduced RED/GREEN rather than trusting the fix report's own account:
temporarily hardcoded `isConstructorSelector` inside `qdrantClientLocalNames` back to the
literal identifiers `"qdrant"`/`"store"` (reverting only the alias-resolution half, scoped to
this one function, via a scripted edit — not `git stash`), ran
`TestQdrantClientLocalNamesDetectsAliasedStoreImport`, observed the exact failure the fix
report describes (`qdrantClientLocalNames = map[], want map[c:true]`), then restored the file
and confirmed byte-for-byte identity against the pre-edit copy via `diff`, and reran the same
test plus the five related tests (`TestFileRefsQdrantClientDetectsAliasedStoreImport`,
`TestFileRefsQdrantClientDetectsFunctionValueAlias`, `TestQdrantClientLocalNamesFollowsFunctionValueAlias`,
`TestQdrantClientIsHeldOnlyByStorePackage`, `TestQdrantClientConstructedOnlyByNewQdrantClient`)
— all green.

Confirmed via `rg` that the fixture has exactly two consumers
(`TestFileRefsQdrantClientDetectsAliasedStoreImport` and the new
`TestQdrantClientLocalNamesDetectsAliasedStoreImport`), and that the former still exercises its
own path correctly against the adapted fixture: `fileRefsQdrantClient`
(`schemaversion_stamp_gate_test.go:848-889`) matches on the `CallExpr`'s `Fun` selector
regardless of whether the call result is assigned or returned directly, so changing the
fixture's `dial` body from a bare return to an assign-then-return does not affect that test's
`CallExpr`-shape match — confirmed both by static trace and by the passing test run.

`go build ./...`, `go vet ./internal/store/...`, and `gofmt -l` on both files are clean. No
regressions found in the incremental scope. IN-01 (seed.go doc comment) is out of scope per
task instructions and was not re-raised.

All reviewed files meet quality standards. No issues found.

---

_Reviewed: 2026-09-19T02:15:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
