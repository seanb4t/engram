---
phase: "1"
slug: "test-harness-fixture-helper"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-18"
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`) + `testcontainers-go/modules/qdrant` |
| **Config file** | none — harness behavior lives in each package's `TestMain` (moving to `internal/store/storetest` this phase) |
| **Quick run command** | `go test -short ./internal/store/... ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/...` |
| **Full suite command** | `task test` |
| **Estimated runtime** | ~300 seconds (full, real Qdrant) |

---

## Sampling Rate

- **After every task commit:** Run `go test -short ./internal/store/... ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/...`
- **After every plan wave:** Run `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/... -count=1`
- **Before `/gsd-verify-work`:** `task test` must be green, INCLUDING `TestRedEvidencePatchesAreLive` (no `-short`)
- **Max feedback latency:** 120 seconds (quick run)

---

## Per-Task Verification Map

Requirement-level rows seeded from RESEARCH.md § Validation Architecture; the planner/executor
refines these to task IDs.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 01-01-01 | 01 | 1 | REQ-test-client-parity | T-01-01, T-01-03, T-01-04, T-01-06 | storetest resolves only the test Qdrant; single fail-closed ENGRAM_REQUIRE_QDRANT parser; `store.NewQdrantClient` + named-limit `storetest.Dial`; storetest never enters the production import graph | integration (real Qdrant) + unit — storetest `TestDialRoundTrip`, `TestRequireQdrant`, `TestSplitAddr`, `TestDialOptions`, `TestRecvLimitIsFourMiB`, `TestSkipOrFailNoQdrantSkipsWhenNotRequired`; gates `TestEveryStoreConstructionRoutesThroughSeam`, `TestCollectionPrefixesAreDisjoint` | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/storetest/ -count=1 -v` | ✅ | ✅ green |
| 01-01-02 | 01 | 1 | REQ-oversized-fixture-helper | T-01-02, T-01-05 | Seeder writes only via `Store.Upsert`, cleans up via `Store.DeleteAll`, self-asserts logical bytes strictly exceed the named limit; `-short` skip lives in the seeder | unit + integration — `TestLayout`, `TestCheckOversized`, `TestValidateSpec`, `TestFixtureIdentityIsUnique`, `TestCreatedAtStrictlyIncreasing`, `TestSeedOversizedShapes` (few-large, many-small) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/storetest/ -count=1 -v` | ✅ | ✅ green |
| 01-01-03 | 01 | 1 | REQ-test-client-parity | T-01-02, T-01-04 | Production builds through `store.NewQdrantClient` unchanged; holder gate recognizes it and write-checks every holder except store.go (D-13) | AST/static — `TestQdrantClientIsHeldOnlyByStorePackage` | `go test ./internal/store/ -run '^TestQdrantClientIsHeldOnlyByStorePackage$' -count=1 -v` | ✅ | ✅ green |
| 01-02-01 | 02 | 2 | REQ-test-client-parity | T-02-01 | An in-package test client records the production otelgrpc span AND fires a caller interceptor on the same RPC | integration (real Qdrant) — `TestNewQdrantClientAppliesBaseAndCallerOptions` | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestNewQdrantClientAppliesBaseAndCallerOptions$' -count=1 -v` | ✅ | ✅ green |
| 01-02-02 | 02 | 2 | REQ-test-client-parity | T-02-02 | Five interceptor helpers delegate to `dialTestClient`; every interceptor-driven family still passes | integration — whole internal/store package (only `TestRedEvidencePatchesAreLive` may fail until 01-05-03) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -count=1 -v` | ✅ | ✅ green |
| 01-03-01 | 03 | 2 | REQ-test-client-parity | T-03-01, T-03-02, T-03-03 | internal/server on `storetest.Run`/`storetest.Dial`; fail-closed parse intact | integration — whole internal/server package | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -count=1` | ✅ | ✅ green |
| 01-03-02 | 03 | 2 | REQ-test-client-parity | T-03-01, T-03-02, T-03-03 | e2e keeps its early parse and local build; retrievaleval keeps its opt-in gate and ignores ENGRAM_REQUIRE_QDRANT | integration — internal/e2e + internal/retrievaleval packages | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/e2e/ ./internal/retrievaleval/ -count=1` | ✅ | ✅ green |
| 01-04-01 | 04 | 3 | REQ-test-client-parity | T-04-01, T-04-02 | External TestMain + in-package hook preserve skip and fail-closed dial behavior; CI's shared-address count holds (3 PASS + 1 SKIP) | integration — `TestDialTestClientSkipsWhenNotRequired`, `TestDialTestClientFailsWhenRequiredAndUnavailable`, `TestSharedQdrantAddressHonored` | `ENGRAM_QDRANT_TEST_ADDR=127.0.0.1:6334 ENGRAM_REQUIRE_QDRANT=1 go test -run '^TestSharedQdrantAddressHonored$' -v -count=1 ./internal/store/... ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/...` | ✅ | ✅ green |
| 01-04-02 | 04 | 3 | REQ-oversized-fixture-helper | T-04-03, T-04-04 | Migrated #583 regression over both shapes at the named 4 MiB limit, observed RED against the pre-fix selector | integration (real Qdrant) — `TestListScopesFullPayloadsOverGRPCLimit`, `TestManySmallShapeFitsOneListPage` | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestListScopesFullPayloadsOverGRPCLimit$' -count=1 -v` | ✅ | ✅ green |
| 01-05-01 | 05 | 4 | REQ-test-client-parity | T-05-01, T-05-02 | D-11: no Qdrant client construction outside `store.NewQdrantClient`, test files included | AST/static — `TestQdrantClientConstructedOnlyByNewQdrantClient` | `go test ./internal/store/ -run '^TestQdrantClientConstructedOnlyByNewQdrantClient$' -count=1 -v` | ✅ | ✅ green |
| 01-05-02 | 05 | 4 | REQ-test-client-parity | T-05-05 | CI's pinned Qdrant image equals `storetest.QdrantImage` | static — `TestQdrantImageMatchesCIService` | `go test ./internal/store/storetest/ -run '^TestQdrantImageMatchesCIService$' -count=1 -v` | ✅ | ✅ green |
| 01-05-03 | 05 | 4 | REQ-oversized-fixture-helper, REQ-test-client-parity | T-05-03, T-05-04 | Four registered red-evidence patches each proven RED; tree restored | integration (mutates+reverts tree) — `TestRedEvidencePatchesAreLive` | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1 -v -timeout 20m` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Task IDs reconciled by the planner on 2026-09-18 against `01-01-PLAN.md` (3 tasks), `01-02-PLAN.md` (2), `01-03-PLAN.md` (2), `01-04-PLAN.md` (2) and `01-05-PLAN.md` (3). Threat refs point at each plan's `<threat_model>` register (T-0N-xx = plan 0N). Two reds are expected mid-phase: `TestRedEvidencePatchesAreLive` until 01-05-03, and `internal/keylinks`' `TestActiveMilestoneKeyLinksSatisfiable` until `01-01-SUMMARY.md` exists (it scans zero executed plans before then).*

---

## Wave 0 Requirements

- [x] `store.NewQdrantClient` in `internal/store/store.go` (01-01-01)
- [x] `internal/store/storetest/storetest.go` + `storetest_test.go` — `Run`, `Dial`, `RecvLimit`, `QdrantImage`, `RequireQdrant` (01-01-01)
- [x] `internal/store/storetest/seed.go` + `seed_test.go` — two-shape oversized seeder (01-01-02)
- [x] `internal/store/qdrantclient_test.go` — constructor parity test (01-02-01)
- [x] `internal/store/main_test.go`, `export_test.go`, `listscopes_oversized_test.go` (01-04)
- [x] `internal/store/qdrant_client_convergence_test.go` + `testdata/qdrantclient/` — D-11 gate (01-05-01)
- [x] `internal/store/storetest/cipin_test.go` — CI image pin (01-05-02)
- [x] `.planning/phases/01-test-harness-fixture-helper/red-evidence/` + `redEvidenceDirs` registration (01-05-03)

---

## Manual-Only Verifications

All phase behaviors have automated verification.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 120s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-09-19

## Validation Audit 2026-09-19

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

Every per-task command re-run at HEAD `be18a467` by the orchestrator (validate-phase, State A): one `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... ./internal/server/ ./internal/e2e/ ./internal/retrievaleval/ -count=1 -v` run exited 0 with all 23 named tests `--- PASS` and 0 top-level `--- FAIL` (`TestRedEvidencePatchesAreLive`: 4 `confirmed RED` lines); row 01-04-01 run verbatim against a temporary `qdrant/qdrant:v1.19.1` on 127.0.0.1:6334 exited 0 with PASS=3 SKIP=1. Full `task` gate green after the last plan (post-merge gate) and `task lint` green after the code-review fix commits.
