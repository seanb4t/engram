---
phase: 01-test-harness-fixture-helper
verified: 2026-09-18T22:20:00Z
status: passed
score: 3/3 roadmap success criteria verified; 5/5 plans' must_haves verified
covered_files: [".github/workflows/ci.yaml", ".planning/phases/01-test-harness-fixture-helper/01-01-PLAN.md", ".planning/phases/01-test-harness-fixture-helper/01-01-SUMMARY.md", ".planning/phases/01-test-harness-fixture-helper/01-02-PLAN.md", ".planning/phases/01-test-harness-fixture-helper/01-02-SUMMARY.md", ".planning/phases/01-test-harness-fixture-helper/01-03-PLAN.md", ".planning/phases/01-test-harness-fixture-helper/01-03-SUMMARY.md", ".planning/phases/01-test-harness-fixture-helper/01-04-PLAN.md", ".planning/phases/01-test-harness-fixture-helper/01-04-SUMMARY.md", ".planning/phases/01-test-harness-fixture-helper/01-05-PLAN.md", ".planning/phases/01-test-harness-fixture-helper/01-05-SUMMARY.md", ".planning/phases/01-test-harness-fixture-helper/01-CONTEXT.md", ".planning/phases/01-test-harness-fixture-helper/01-REVIEW-FIX.md", ".planning/phases/01-test-harness-fixture-helper/01-REVIEW.md", ".planning/phases/01-test-harness-fixture-helper/red-evidence/01-01-storetest-raw-client-write.patch", ".planning/phases/01-test-harness-fixture-helper/red-evidence/01-04-listscopes-full-payload-selector.patch", ".planning/phases/01-test-harness-fixture-helper/red-evidence/01-05-bare-qdrant-newclient-in-test.patch", ".planning/phases/01-test-harness-fixture-helper/red-evidence/01-05-ci-qdrant-image-drift.patch", "internal/e2e/console_browser_test.go", "internal/e2e/harness_test.go", "internal/e2e/spine_review_test.go", "internal/retrievaleval/retrieval_eval_test.go", "internal/server/schemaversion_wire_test.go", "internal/server/tools.go", "internal/server/tools_test.go", "internal/store/collectionprefix_conformance_test.go", "internal/store/export_test.go", "internal/store/instrument_test.go", "internal/store/listscopes_oversized_test.go", "internal/store/main_test.go", "internal/store/migrate_converge_test.go", "internal/store/migrate_faultinject_test.go", "internal/store/migrate_status_test.go", "internal/store/qdrant_client_convergence_test.go", "internal/store/qdrantclient_test.go", "internal/store/redevidence_harness_test.go", "internal/store/revert_test.go", "internal/store/schemaversion_recallgate_test.go", "internal/store/schemaversion_stamp_gate_test.go", "internal/store/store.go", "internal/store/store_test.go", "internal/store/storetest/cipin_test.go", "internal/store/storetest/seed.go", "internal/store/storetest/seed_test.go", "internal/store/storetest/storetest.go", "internal/store/storetest/storetest_test.go", "internal/store/testdata/qdrantclient/bad_aliased_test.go.txt", "internal/store/testdata/qdrantclient/bad_store.go.txt", "internal/store/testdata/qdrantclient/bad_store_aliased_import.go.txt", "internal/store/testdata/qdrantclient/bad_store_funcalias_holder.go.txt", "internal/store/testdata/qdrantclient/bad_test.go.txt", "internal/store/testdata/qdrantclient/bad_valueref_test.go.txt", "internal/store/testdata/qdrantclient/good_store.go.txt"]
covered_digest: "v1:sha256:1059de0afac7c3ffe18c8b4007419b883e03fdb4a5494698fef914f6590573ff"
behavior_unverified: 0
overrides_applied: 0
---

# Phase 1: Test Harness & Fixture Helper Verification Report

**Phase Goal:** Every later phase's regression tests need a real-Qdrant fixture holding more than 4 MiB of payload, RED before its fix — this phase extracts `TestListScopesFullPayloadsOverGRPCLimit`'s (#583) fixture-seeding shape into one shared, reusable helper covering both a many-small-records shape and a few-large-records shape, and moves every test Qdrant client onto one shared constructor so a regression test proves the mechanism keeps responses bounded rather than relying on a client-side accident.
**Verified:** 2026-09-18T22:20:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A shared test helper seeds a scope whose full payloads exceed a named receive limit in two shapes (many-small, few-large), self-asserting the logical byte count | ✓ VERIFIED | `storetest.SeedOversized` (`internal/store/storetest/seed.go`) implements `Shape{ManySmall,FewLarge}`, `layout`/`checkOversized` (ceiling-division, ≥1.25× margin, strictly-greater self-check). Live run: `TestSeedOversizedShapes`, `TestLayout`, `TestCheckOversized`, `TestValidateSpec` all pass against real Qdrant (see Behavioral Spot-Checks). |
| 2 | Every test Qdrant client in this milestone's regression tests is constructed through one shared constructor applying the same dial options as the production client, with each test naming its receive limit explicitly | ✓ VERIFIED | `store.NewQdrantClient` (`internal/store/store.go:518`) is the sole constructor; production (`internal/server/tools.go:120`) and every test path (`storetest.Dial`, in-package `dialTestClient`) route through it. `rg 'qdrant[.]NewClient\('` across the whole module returns only comment lines (0 live call sites outside `store.go`). `TestNewQdrantClientAppliesBaseAndCallerOptions` proves a test client carries the production otelgrpc span AND a caller interceptor on the same RPC. |
| 3 | The independent `qdrant.NewClient` test call sites converge on the shared constructor, so a passing test proves the bounded-read mechanism keeps responses bounded rather than a client-side accident | ✓ VERIFIED | `TestQdrantClientConstructedOnlyByNewQdrantClient` (D-11 AST gate, `internal/store/qdrant_client_convergence_test.go`) scans every `.go` file in the module including `_test.go` files and permits exactly one sanctioned construction site. Live run: all 6 subtests pass, real-module scan logs `scanned 139 non-test and 216 test .go files` (both non-zero, proving `_test.go` files are in scope) with zero violations. |

**Score:** 3/3 roadmap success criteria verified.

### Plan-Level Must-Haves (five plans, 01-01..01-05)

All plan-frontmatter `must_haves.truths`, `artifacts`, and `key_links` were checked against the codebase and against live test runs (not SUMMARY claims). Representative direct evidence, beyond the roadmap-level truths above:

- `store.NewQdrantClient(host string, port int, opts ...grpc.DialOption) (*qdrant.Client, error)` exists exactly once in `store.go`, base option `grpc.WithStatsHandler(otelgrpc.NewClientHandler())` applied first, caller options appended (verified by source read + `TestNewQdrantClientAppliesBaseAndCallerOptions` passing).
- `storetest.RecvLimit = 4 << 20` (4194304), `storetest.QdrantImage = "qdrant/qdrant:v1.19.1"` — verified by source and `TestRecvLimitIsFourMiB`.
- `dialOptions`/`TestDialOptions` reject 0/-1, accept 1, preserve caller-option order, append the named limit last — verified passing in the full `internal/store/storetest` suite run.
- Fail-closed `ENGRAM_REQUIRE_QDRANT` parsing (never coerced to false on an invalid value) verified live for `internal/server`, `internal/e2e` (`treu` → non-zero exit, `invalid value "treu"`); `internal/retrievaleval` verified to ignore it via `IgnoreRequireQdrant` (still PASSes with `treu` set).
- D-13 generalized never-writes check: `qdrantClientHolderAllowlist` has exactly 3 entries (`store.go`, `tools.go`, `storetest.go`); `TestQdrantClientIsHeldOnlyByStorePackage` passes live.
- Collection-prefix/seam gates cover 5 packages / 20 ordered pairs (`TestEveryStoreConstructionRoutesThroughSeam` log: "compared 20 ordered pairs across 5 packages").
- The migrated `TestListScopesFullPayloadsOverGRPCLimit` runs over both shapes (`few-large`, `many-small`) at the named `storetest.RecvLimit`, passing live against real Qdrant; `TestManySmallShapeFitsOneListPage` pins `storetest.ManySmallRecords == store.MaxListLimit`.
- CI's `services.qdrant` image is gate-enforced (`TestQdrantImageMatchesCIService`), not merely comment-enforced; `.github/workflows/ci.yaml` stays yamlfmt/actionlint-clean.
- Four red-evidence patches registered in `redEvidenceDirs` and independently confirmed RED-then-restored by this verifier (see Probe Execution below) — not merely claimed by SUMMARY.

No plan-level must-have contradicted the codebase; no artifact was a stub.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/store/store.go` `NewQdrantClient` | Shared constructor | ✓ VERIFIED | Exists at line 518, exact signature, base+caller option composition confirmed by source and by `TestNewQdrantClientAppliesBaseAndCallerOptions` (span + interceptor both fire). |
| `internal/store/storetest/storetest.go` (301 lines) | `RecvLimit`, `QdrantImage`, `Run`, `Dial`, etc. | ✓ VERIFIED | All exported identifiers present; `TestDialRoundTrip` passes against real Qdrant. |
| `internal/store/storetest/seed.go` (252 lines) | `SeedOversized`, `Shape`, `Spec`, `Fixture` | ✓ VERIFIED | `TestSeedOversizedShapes` passes both shapes live; self-assertion (`layout`/`checkOversized`) confirmed by table tests. |
| `internal/store/main_test.go` / `export_test.go` | External TestMain + D-07 hook | ✓ VERIFIED | `TestMain` delegates to `storetest.Run`; `SetNoQdrantHandler` wires the skip-or-fail decision across the package boundary; confirmed live. |
| `internal/store/listscopes_oversized_test.go` | Migrated #583 regression, both shapes | ✓ VERIFIED | `TestListScopesFullPayloadsOverGRPCLimit` passes both subtests against real Qdrant at the named 4 MiB limit. |
| `internal/store/qdrant_client_convergence_test.go` (444 lines) + 7 testdata fixtures | D-11 AST gate | ✓ VERIFIED | All 6 subtests pass (5 fixture shapes including the code-review-added function-value-alias/aliased-import fixtures, plus the real-module scan). |
| `internal/store/storetest/cipin_test.go` | CI image pin gate | ✓ VERIFIED | `TestQdrantImageMatchesCIService` passes; CI comments reworded to name storetest. |
| `internal/store/redevidence_harness_test.go` | 4 registered red-evidence patches | ✓ VERIFIED | `redEvidenceDirs` maps all 4 patches; `TestRedEvidencePatchesAreLive` passes with 4 `confirmed RED:` lines (independently re-run by this verifier). |

### Key Link Verification

| From | To | Via | Status |
|------|-----|-----|--------|
| `internal/server/tools.go` | `internal/store/store.go` | `store.NewQdrantClient(host, port)` | ✓ WIRED (source-confirmed, no extra option — production dial-option set unchanged) |
| `internal/store/storetest/storetest.go` (`Dial`) | `internal/store/store.go` | `store.NewQdrantClient(host, port, dialOpts...)` | ✓ WIRED |
| `internal/store/store_test.go` (`dialTestClient`) | `internal/store/store.go` | `NewQdrantClient(host, port, opts...)` (bare name, same package) | ✓ WIRED |
| Five interceptor-wrapping helpers (`migrate_status_test.go` etc.) | `dialTestClient` | one-line delegation, e.g. `dialTestClient(t, grpc.WithUnaryInterceptor(...))` | ✓ WIRED (confirmed via `rg` — 5 delegations, 0 remaining bare `qdrant.NewClient(` in `internal/store/*_test.go`) |
| `internal/server`, `internal/e2e`, `internal/retrievaleval` TestMains | `storetest.Run` | `os.Exit(storetest.Run(m))` / `storetest.Run(m, storetest.IgnoreRequireQdrant())` | ✓ WIRED (live PASS/SKIP counts match CI's pinned 3 PASS + 1 SKIP) |
| `internal/store/main_test.go` | `internal/store/export_test.go` | `store.SetNoQdrantHandler(storetest.SkipOrFailNoQdrant)` | ✓ WIRED |
| `internal/store/redevidence_harness_test.go` | 4 red-evidence patches | glob-discovered, mapped, applied and reverted | ✓ WIRED (independently re-run, all 4 confirmed RED) |

### Data-Flow Trace (Level 4)

Not applicable in the strict UI-rendering sense — this phase's "data" is test infrastructure. The equivalent trace (does the seeded fixture data actually reach and get counted by the code under test) was directly exercised: `TestSeedOversizedShapes` and `TestListScopesFullPayloadsOverGRPCLimit` both assert `ListScopes` counts exactly the number of records `SeedOversized` wrote, via real Qdrant round trips — confirmed passing live, not via static return or mock.

### Behavioral Spot-Checks / Live Test Runs (this verifier, independent of SUMMARY claims)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Build | `go build ./...` | clean, no output | ✓ PASS |
| D-11 convergence gate, all shapes + real module | `go test ./internal/store/ -run '^TestQdrantClientConstructedOnlyByNewQdrantClient$' -v` | 6/6 subtests PASS, 139 non-test + 216 test files scanned | ✓ PASS |
| Holder / seam / prefix gates | `go test ./internal/store/ -run '^(TestQdrantClientIsHeldOnlyByStorePackage\|TestEveryStoreConstructionRoutesThroughSeam\|TestCollectionPrefixesAreDisjoint)$'` | 3/3 PASS, "compared 20 ordered pairs across 5 packages" | ✓ PASS |
| CI image pin gate | `go test ./internal/store/storetest/ -run '^TestQdrantImageMatchesCIService$'` | PASS | ✓ PASS |
| Migrated #583 regression, both shapes | `go test ./internal/store/ -run '^(TestListScopesFullPayloadsOverGRPCLimit\|TestManySmallShapeFitsOneListPage)$'` (real Qdrant) | 2/2 PASS (few-large, many-small) + 1 PASS | ✓ PASS |
| Full `internal/store` suite | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -count=1 -timeout 20m` (real Qdrant) | `ok` — no failures, no expected reds remain | ✓ PASS |
| Shared-address invariant | `ENGRAM_REQUIRE_QDRANT=1 go test -run '^TestSharedQdrantAddressHonored$' ./internal/store/... ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/...` | `PASS=3 SKIP=1` exactly | ✓ PASS |
| Fail-closed parsing (server, e2e) | `ENGRAM_REQUIRE_QDRANT=treu go test ... -run '^TestSharedQdrantAddressHonored$'` | non-zero exit, `invalid value "treu"` for both server and e2e | ✓ PASS |
| Fail-closed opt-out (retrievaleval) | `ENGRAM_RETRIEVAL_EVAL=1 ENGRAM_REQUIRE_QDRANT=treu ... go test ./internal/retrievaleval/ -run '^TestSharedQdrantAddressHonored$'` | PASS (invalid value silently ignored, as designed) | ✓ PASS |
| server/e2e/retrievaleval suites | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ ./internal/e2e/ ./internal/retrievaleval/ -count=1` | all `ok` | ✓ PASS |
| keylinks gate | `go test ./internal/keylinks/ -count=1` | `ok` | ✓ PASS |
| `task lint` (golangci-lint, actionlint, yamlfmt, rumdl, ruff) | `task lint` | "All checks passed!" | ✓ PASS |
| go.mod/go.sum drift | `git diff --exit-code main...HEAD -- go.mod go.sum` | exit 0 | ✓ PASS |
| Working tree clean after mutation-heavy runs | `git status --porcelain` | only untracked `.planning/milestone.lock` | ✓ PASS |

### Probe Execution (red-evidence patches)

Per orchestrator note, this verifier independently re-ran `TestRedEvidencePatchesAreLive` (the harness that applies/reverts each patch and requires the target test to fail) rather than trusting SUMMARY's claimed RED observations:

| Probe | Command | Result | Status |
|-------|---------|--------|--------|
| `01-01-storetest-raw-client-write.patch` → `TestQdrantClientIsHeldOnlyByStorePackage` | `go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -v` | `confirmed RED: ... applied -> TestQdrantClientIsHeldOnlyByStorePackage failed as expected` | ✓ PASS |
| `01-04-listscopes-full-payload-selector.patch` → `TestListScopesFullPayloadsOverGRPCLimit` | (same run) | `confirmed RED: ... failed as expected` | ✓ PASS |
| `01-05-bare-qdrant-newclient-in-test.patch` → `TestQdrantClientConstructedOnlyByNewQdrantClient` | (same run) | `confirmed RED: ... failed as expected` | ✓ PASS |
| `01-05-ci-qdrant-image-drift.patch` → `TestQdrantImageMatchesCIService` | (same run) | `confirmed RED: ... failed as expected` | ✓ PASS |

Overall: `--- PASS: TestRedEvidencePatchesAreLive (9.72s)` with all 4 sub-results PASS; working tree confirmed clean (`git status --porcelain`) for all 4 touched files after the run.

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|--------------|--------------|-------------|--------|----------|
| REQ-oversized-fixture-helper | 01-01, 01-04, 01-05 | Shared real-Qdrant helper, two shapes, self-asserting, used by every read-path regression test, RED against pre-fix code | ✓ SATISFIED | `storetest.SeedOversized`, migrated `TestListScopesFullPayloadsOverGRPCLimit`, registered red-evidence patch 01-04 |
| REQ-test-client-parity | 01-01, 01-02, 01-03, 01-05 | One shared constructor, same dial options as production, named receive limit, independent call sites converge | ✓ SATISFIED | `store.NewQdrantClient`, `storetest.Dial`/`dialTestClient`, D-11 AST gate |

No orphaned requirements: `REQUIREMENTS.md` maps only these two IDs to Phase 1, both marked Complete and both accounted for in plan frontmatter. `REQ-ci-store-green` and `REQ-recv-limit-backstop` are explicitly Phase 5's per REQUIREMENTS.md's own phase mapping and per this phase's CONTEXT.md — correctly out of scope here (not reported as gaps).

### Anti-Patterns Found

None. Scanned all files this phase created/modified for `TBD`/`FIXME`/`XXX` (0 matches) and `TODO`/`HACK`/`PLACEHOLDER` (0 matches). No stub return values, no hardcoded empty data flowing to assertions — every test that claims to prove a property does so against real Qdrant or a real AST scan of the real module, confirmed by this verifier's own independent runs above rather than by trusting SUMMARY claims.

Rule `m45p2b4bp7` (tests must not assert grpc-go's/Qdrant's own default behavior) checked: all size/limit assertions compare against `storetest.RecvLimit` or other named constants defined in this codebase, never against a third-party default. No violation found.

### Human Verification Required

None. Every truth in this phase is mechanically verifiable (AST gates, real-Qdrant integration tests, CI YAML text checks) and was independently re-run by this verifier against the actual codebase, not inferred from documentation.

### Gaps Summary

None. All 3 ROADMAP success criteria and all plan-level must-haves are verified directly against the codebase via independent test execution (not SUMMARY claims). The code-review loop (01-REVIEW.md / 01-REVIEW-FIX.md) converged clean across 2 iterations, closing a real gap (D-11/D-13 AST gates were blind to function-value-alias and aliased-import bypasses) with new fixtures and tests, independently confirmed still green by this verifier's re-run of the full `TestQdrantClientConstructedOnlyByNewQdrantClient` suite and the full `internal/store` package.

---

_Verified: 2026-09-18T22:20:00Z_
_Verifier: Claude (gsd-verifier)_
