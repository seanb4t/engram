---
phase: 01-test-harness-fixture-helper
fixed_at: 2026-09-19T02:07:37Z
review_path: .planning/phases/01-test-harness-fixture-helper/01-REVIEW.md
iteration: 2
findings_in_scope: 1
fixed: 1
skipped: 0
status: all_fixed
---

# Phase 01: Code Review Fix Report

**Fixed at:** 2026-09-19T02:07:37Z
**Source review:** .planning/phases/01-test-harness-fixture-helper/01-REVIEW.md
**Iteration:** 2

**Summary:**
- Findings in scope (fix_scope=critical_warning: Critical + Warning): 1
- Fixed: 1
- Skipped: 0

Note: IN-01 (carried-forward, out of scope — `seed.go` doc-comment line break)
is Info severity and out of `critical_warning` scope for this run, so it was
not attempted.

**Verification environment:** main checkout (no worktree — per this run's
`repo_constraints`, which explicitly direct working in the main checkout on
branch `feat/2026-09-18.01`, never a worktree, with an explicit pathspec on
every commit).

## Fixed Issues

### WR-01 (iteration 2): `qdrantClientLocalNames`'s aliased-`internal/store`-import fix has no dedicated regression test

**Files modified:**
- `internal/store/schemaversion_stamp_gate_test.go`
- `internal/store/testdata/qdrantclient/bad_store_aliased_import.go.txt`

**Commit:** `be18a467`

**Applied fix:** The review's own Fix guidance asked for a fourth test,
`TestQdrantClientLocalNamesDetectsAliasedStoreImport`, parsing the existing
`bad_store_aliased_import.go.txt` fixture and asserting `qdrantClientLocalNames`
returns `{"c": true}`. Reading the actual fixture first (per fix_strategy)
showed it didn't match that description: its `dial` function returned the
constructor call directly (`return st.NewQdrantClient(host, port)`), with no
assignment at all, so `qdrantClientLocalNames` — which only inspects
`*ast.AssignStmt`/`*ast.ValueSpec` — would trivially return an empty map
regardless of whether the aliased-import resolution being tested actually
worked. That fixture shape could not have exercised the write-boundary-relevant
quadrant the review identified as missing.

Adapted the fix: changed the fixture's `dial` body to bind the client through a
direct assignment (`c, err := st.NewQdrantClient(host, port); return c, err`)
so it actually exercises `qdrantClientLocalNames`'s identifier-binding pass
under an aliased `internal/store` import, while still tripping
`fileRefsQdrantClient`'s CallExpr-shape match (which doesn't care about
assignment vs. return) — confirmed the existing
`TestFileRefsQdrantClientDetectsAliasedStoreImport` still passes with the
adapted fixture, since that test call site is the fixture's only other
consumer (verified via `rg` for the fixture's filename before editing it).
Added `TestQdrantClientLocalNamesDetectsAliasedStoreImport` beside
`TestQdrantClientLocalNamesFollowsFunctionValueAlias`, mirroring its shape,
asserting `qdrantClientLocalNames` returns exactly `{"c": true}` for the
adapted fixture.

**RED-then-GREEN proof:** Temporarily hardcoded `isConstructorSelector`'s store
check inside `qdrantClientLocalNames` back to the literal identifier `"store"`
(reverting only the alias-resolution half of the iteration-1 fix, scoped to
this one function), ran the new test, observed:
```
--- FAIL: TestQdrantClientLocalNamesDetectsAliasedStoreImport
    qdrantClientLocalNames = map[], want map[c:true]
```
confirming the new test genuinely exercises the fix rather than passing
vacuously. Restored the alias-resolved check (git-diff-verified: no leftover
temporary markers remained in the diff against HEAD before commit), then
re-ran the same test plus the five related tests
(`TestFileRefsQdrantClientDetectsAliasedStoreImport`,
`TestFileRefsQdrantClientDetectsFunctionValueAlias`,
`TestQdrantClientLocalNamesFollowsFunctionValueAlias`,
`TestQdrantClientIsHeldOnlyByStorePackage`,
`TestQdrantClientConstructedOnlyByNewQdrantClient`) — all pass.

**Verification performed:**
- Tier 1 (re-read): both modified files re-read after editing; fix text
  present, surrounding code intact, no corruption.
- Tier 2 (syntax/build): `go build ./...`, `go vet ./internal/store/...`,
  `gofmt -l` on both modified files — all clean.
- RED proof: targeted revert of `isConstructorSelector`'s alias resolution →
  confirmed `--- FAIL:` for the new test only, then precisely restored (verified
  via `git diff` showing no leftover revert markers) and re-confirmed GREEN.
- `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -count=1` — both
  `internal/store` and `internal/store/storetest` packages green (35.8s / 2.5s).
- `task lint` (golangci-lint, actionlint, yamlfmt, rumdl, ruff) — all clean,
  exit 0.

## Skipped Issues

None.

---

_Fixed: 2026-09-19T02:07:37Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 2_

## Iteration 1 (carried forward, for continuity)

The prior iteration fixed WR-01 as originally reported (D-11/D-13 AST
convergence gates blind to a function-value alias and an aliased-import
bypass) in commit `56723405`. That fix report is preserved verbatim below.

---

# Phase 01: Code Review Fix Report

**Fixed at:** 2026-09-19T01:40:35Z
**Source review:** .planning/phases/01-test-harness-fixture-helper/01-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope (fix_scope=critical_warning: Critical + Warning): 1
- Fixed: 1
- Skipped: 0

Note: IN-01 (stray mid-sentence line break in `SeedOversized`'s doc comment)
is Info severity and out of `critical_warning` scope for this run, so it was
not attempted.

**Verification environment:** main checkout (no worktree — `workflow.use_worktrees`
override per this run's `repo_constraints`: the working tree is the main
checkout on branch `feat/2026-09-18.01`; all edits, test runs, and the commit
below happened there directly, with an explicit pathspec on the commit).

## Fixed Issues

### WR-01: D-11/D-13 AST convergence gates cannot detect a function-value or aliased-import bypass

**Files modified:**
- `internal/store/qdrant_client_convergence_test.go`
- `internal/store/schemaversion_stamp_gate_test.go`
- `internal/store/testdata/qdrantclient/bad_valueref_test.go.txt` (new)
- `internal/store/testdata/qdrantclient/bad_store_aliased_import.go.txt` (new)
- `internal/store/testdata/qdrantclient/bad_store_funcalias_holder.go.txt` (new)

**Commit:** `56723405`

**Applied fix:** Closed both named false-negative shapes rather than only
documenting them, per this run's repo_constraints:

1. **Function-value alias** (`var dial = qdrant.NewClient; dial(cfg)`).
   `scanQdrantClientConstructions` (D-11) now also flags a bare, non-call
   `*ast.SelectorExpr` reference to a sanctioned constructor name as a
   violation, recorded under the same `callee`/`enclosingFunc` shape as an
   ordinary call, with a node-identity guard so an ordinary
   `qdrant.NewClient(cfg)` call is never double-counted. New fixture
   `bad_valueref_test.go.txt` + subtest `bad function-value-alias fixture`.

2. **Aliased `internal/store` import.** Added `importLocalNames` (mirrors
   D-11's own import-resolution loop) and switched both `fileRefsQdrantClient`
   and `qdrantClientLocalNames` off the hardcoded literal `"store"` onto
   alias-resolved name sets for both the `qdrant` and `internal/store` import
   paths. New fixture `bad_store_aliased_import.go.txt` +
   `TestFileRefsQdrantClientDetectsAliasedStoreImport`.

3. **Function-value alias reaching D-13's write-check** — the shape with the
   real consequence named in the review ("a write on the resulting client
   would be completely unguarded"). `fileRefsQdrantClient` now also treats a
   bare reference to `store.NewQdrantClient` as holding-evidence, and
   `qdrantClientLocalNames` follows one level of constructor-value aliasing
   (`var dial = store.NewQdrantClient; c, err := dial(...)`) so the
   client-bound identifier (`c`) is still registered and a later write on it
   would still trip `TestQdrantClientIsHeldOnlyByStorePackage`. New fixture
   `bad_store_funcalias_holder.go.txt` +
   `TestFileRefsQdrantClientDetectsFunctionValueAlias` and
   `TestQdrantClientLocalNamesFollowsFunctionValueAlias`.

Each new fixture/subtest was proven RED against the pre-fix gate code first
(observed `--- FAIL:` for all three new tests, including a re-check after an
initial fixture draft that passed for the wrong reason — see below), then the
gate code changed, then re-verified green.

**Fixture-design note (self-caught during RED-proving):** the first drafts of
`bad_store_aliased_import.go.txt` and `bad_store_funcalias_holder.go.txt` both
declared a `*qdrant.Client`-typed field/return value, which made their
`fileRefsQdrantClient` subtests pass even against the UNFIXED gate — for the
wrong reason (the pre-existing `qdrant.Client` type-reference check, not the
aliased-call path under test). Both fixtures were rewritten to avoid naming
the `qdrant.Client` type at all (return type `any`) so they isolate exactly
the bypass shape they're meant to prove, then re-confirmed RED before the fix.

**Verification performed:**
- `go build ./...` and `go vet ./internal/store/...` — clean.
- `go test ./internal/store/ -run '^TestQdrantClientConstructedOnlyByNewQdrantClient$'` — all subtests pass, including the "real module" repo-wide scan (no new false positives against the real tree).
- `go test ./internal/store/ -run '^(TestFileRefsQdrantClientDetectsAliasedStoreImport|TestFileRefsQdrantClientDetectsFunctionValueAlias|TestQdrantClientLocalNamesFollowsFunctionValueAlias|TestQdrantClientIsHeldOnlyByStorePackage)$'` — all pass.
- `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -count=1 -timeout 15m` — full package suite green (40.9s).
- `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1 -v -timeout 20m` — all four registered red-evidence patches (including `01-05-bare-qdrant-newclient-in-test.patch`, the one most adjacent to this change) still apply, still go RED as expected, still revert cleanly. No patch needed regeneration — this fix only closes gaps in aliasing/indirection detection; it doesn't change behavior for the direct-call bypass shape that patch exercises.
- `ENGRAM_REQUIRE_QDRANT=1 task` (lint + full test suite, all packages) — exit 0.

## Skipped Issues

None.

---

_Fixed: 2026-09-19T01:40:35Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
