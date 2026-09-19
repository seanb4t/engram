---
phase: 01-test-harness-fixture-helper
reviewed: 2026-09-19T01:25:58Z
depth: standard
files_reviewed: 33
files_reviewed_list:
  - .github/workflows/ci.yaml
  - internal/e2e/console_browser_test.go
  - internal/e2e/harness_test.go
  - internal/e2e/spine_review_test.go
  - internal/retrievaleval/retrieval_eval_test.go
  - internal/server/schemaversion_wire_test.go
  - internal/server/tools.go
  - internal/server/tools_test.go
  - internal/store/collectionprefix_conformance_test.go
  - internal/store/export_test.go
  - internal/store/instrument_test.go
  - internal/store/listscopes_oversized_test.go
  - internal/store/main_test.go
  - internal/store/migrate_converge_test.go
  - internal/store/migrate_faultinject_test.go
  - internal/store/migrate_status_test.go
  - internal/store/qdrant_client_convergence_test.go
  - internal/store/qdrantclient_test.go
  - internal/store/redevidence_harness_test.go
  - internal/store/revert_test.go
  - internal/store/schemaversion_recallgate_test.go
  - internal/store/schemaversion_stamp_gate_test.go
  - internal/store/store.go
  - internal/store/store_test.go
  - internal/store/storetest/cipin_test.go
  - internal/store/storetest/seed.go
  - internal/store/storetest/seed_test.go
  - internal/store/storetest/storetest.go
  - internal/store/storetest/storetest_test.go
  - internal/store/testdata/qdrantclient/bad_aliased_test.go.txt
  - internal/store/testdata/qdrantclient/bad_store.go.txt
  - internal/store/testdata/qdrantclient/bad_test.go.txt
  - internal/store/testdata/qdrantclient/good_store.go.txt
findings:
  critical: 0
  warning: 1
  info: 1
  total: 2
status: issues_found
---

# Phase 01: Code Review Report

**Reviewed:** 2026-09-19T01:25:58Z
**Depth:** standard
**Files Reviewed:** 33
**Status:** issues_found

## Summary

Reviewed the phase-01 "Test Harness & Fixture Helper" changeset: `store.NewQdrantClient`
(the shared prod+test Qdrant client constructor), the new `internal/store/storetest`
package (`Dial`, `SeedOversized`, `Run`/`RequireQdrant`/`SkipOrFailNoQdrant`), the
convergence of 11 test-client dial sites and 4 duplicated `TestMain`s onto that harness,
the D-11 AST convergence gate, the generalized D-13 client-holder write-check, the CI
image-pin gate, and the red-evidence patch registration.

Verification performed beyond static reading: `go build ./...` and `go vet ./...` (both
clean for the changed packages), and — since Docker/Qdrant was available in this
environment — live runs of the new/changed tests against a real Qdrant testcontainer:
`TestSeedOversizedShapes`, `TestDialRoundTrip`, `TestListScopesFullPayloadsOverGRPCLimit`,
`TestLayout`/`TestCheckOversized`/`TestValidateSpec`, `TestQdrantClientConstructedOnlyByNewQdrantClient`
(all four fixtures + real-module scan), `TestQdrantClientIsHeldOnlyByStorePackage`,
`TestCollectionPrefixesAreDisjoint`, `TestEveryStoreConstructionRoutesThroughSeam`, and
`TestQdrantImageMatchesCIService` — all passed. A repo-wide grep confirms exactly one
production call site constructs a `*qdrant.Client` (inside `store.NewQdrantClient`
itself), matching what the D-11 gate asserts. `TestRedEvidencePatchesAreLive` was
deliberately NOT executed, since it mutates the working tree (apply/revert via `git
apply`) even though it reverts on completion — out of bounds for a read-only review.

The fail-closed `ENGRAM_REQUIRE_QDRANT` parsing (`storetest.RequireQdrant`), the
skip-vs-fail decision (`SkipOrFailNoQdrant`), the cross-package `TestMain` hand-off
(`store.SetNoQdrantHandler` / `noQdrantHandler`), the retrievaleval package's deliberate
non-adoption of the fail-closed gate (`IgnoreRequireQdrant`), the oversized-fixture
sizing math (`layout`/`checkOversized`, verified both by table tests and by tracing the
arithmetic by hand), and the `t.Cleanup`-before-first-write ordering in `SeedOversized`
all check out correctly, including the edge cases named in the phase's areas of focus
(no leaked collections, correct cleanup-registration-before-write ordering, no coercion
of an invalid `ENGRAM_REQUIRE_QDRANT` value to `false`).

One WARNING is raised on the AST-based convergence gates themselves (D-11 and the
generalized D-13 write-check): both are call-shape scanners that a determined bypass
(a function-value alias or an aliased `store` import) would render blind to, and that
specific gap is not named alongside the gates' own documented exclusions. This does not
indicate a defect in current behavior — the real-module scans pass cleanly — but it is a
provable false-negative shape the gates cannot currently catch, which the phase context
explicitly asked reviewers to probe for.

## Warnings

### WR-01: D-11/D-13 AST convergence gates cannot detect a function-value or aliased-import bypass

**File:** `internal/store/qdrant_client_convergence_test.go:137-160` (D-11 gate) and
`internal/store/schemaversion_stamp_gate_test.go:800-829, 906-944` (D-13
`fileRefsQdrantClient` / `qdrantClientLocalNames`)

**Issue:** Both scanners only recognize a qdrant-client construction when it appears as
a direct selector call, i.e. `ast.CallExpr.Fun` is an `*ast.SelectorExpr` whose `X` is an
`*ast.Ident` bound to the `qdrant` (or `store`) import. Two real bypass shapes fall
outside that pattern and are silently invisible to both gates:

1. **Function-value indirection.** `var dial = qdrant.NewClient; c, err := dial(cfg)` —
   here `call.Fun` is a plain `*ast.Ident` ("dial"), never a `*ast.SelectorExpr`, so
   neither `scanQdrantClientConstructions` (D-11) nor `qdrantClientLocalNames`/
   `fileRefsQdrantClient` (D-13) ever see it. The construction happens, and a write on
   the resulting client would be completely unguarded by the write-boundary gate, since
   the client-bound identifier itself was never registered.
2. **Aliased `internal/store` import.** `fileRefsQdrantClient`'s `store.NewQdrantClient`
   branch (schemaversion_stamp_gate_test.go:814-816) and `qdrantClientLocalNames`'s
   matching branch (:926-927) both hardcode `pkg.Name == "store"`. A file that imports
   `internal/store` under an alias (`st "github.com/seanb4t/engram/internal/store"`) and
   constructs via `st.NewQdrantClient(...)` without ever separately naming the
   `*qdrant.Client` type would not be flagged as a derived holder by
   `scanRepoForQdrantClientRefs`, so a new allowlist violation in such a file would go
   undetected. (No file in the current tree does this — confirmed via
   `rg '"github.com/seanb4t/engram/internal/store"'` — so this is not an active bypass,
   only a latent one.)

The package doc comment on `qdrant_client_convergence_test.go` already names two
deliberately out-of-scope escapes ("an interface-carried or reflection-built client, and
the generated conn-wrapping service-client constructors") but does not name either shape
above, so a future reader has no signal that these particular bypasses are unguarded.

**Fix:** Either close the gap or document it alongside the existing "what this gate does
NOT close" list. A cheap partial close: in `scanQdrantClientConstructions` and
`fileRefsQdrantClient`, also flag any BARE (non-call) `*ast.SelectorExpr` reference to
one of the sanctioned constructor names (e.g. `qdrant.NewClient` appearing as a value,
not just `qdrant.NewClient(...)`) as a violation requiring manual review — this catches
the common `var dial = qdrant.NewClient` shape without needing full value-flow analysis.
For the aliasing gap, resolve the local name for `internal/store`'s import path the same
way `scanQdrantClientConstructions` already does for the `qdrant` import (collect every
import spec whose path matches, not just the literal `store` identifier), rather than
hardcoding the default package name.

## Info

### IN-01: Stray mid-sentence line break in SeedOversized's doc comment

**File:** `internal/store/storetest/seed.go:170-178`

**Issue:** The doc comment reads "...It validates spec before writing anything, holds
the phase's single -short skip via the testing package (every oversized test inherits
it, D-12)," with the sentence broken across a comment line boundary right after the
open-paren-clause, then continuing "registers cleanup via Store.DeleteAll BEFORE its
first write..." on the next comment line. The line wrap reads as if a word was
dropped ("single -short skip" scans oddly) even though the logic itself (documented
elsewhere in this review) is correct.

**Fix:** Reflow the comment, e.g.: "It validates spec before writing anything, honors
the phase's single -short skip (every oversized test inherits it, D-12), registers
cleanup via Store.DeleteAll BEFORE its first write so a partial seed is reclaimed,
writes every record sequentially..., and self-asserts what it actually wrote before
returning the Fixture."

---

_Reviewed: 2026-09-19T01:25:58Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
