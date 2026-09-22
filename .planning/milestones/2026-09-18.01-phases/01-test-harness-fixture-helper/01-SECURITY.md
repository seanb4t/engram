---
phase: "1"
slug: "test-harness-fixture-helper"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-19"
---

# Phase 1 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Test process ↔ Qdrant | Test suites reach only the test Qdrant (`ENGRAM_QDRANT_TEST_ADDR` or a booted testcontainer), never an operator's instance | Synthetic fixture records (no real memories) |
| Test-support package ↔ production binary | `internal/store/storetest` imports `testing`/testcontainers and must never enter `cmd/engram`'s import graph | Build-time dependency edges |
| Test code ↔ store write boundary | Fixtures must enter Qdrant only through `Store.Upsert`/`Store.DeleteAll` (schema stamping, owner) | Memory payloads |
| CI workflow ↔ testcontainer image | CI's shared `services.qdrant` image must match the harness's pinned image | Image tag |
| Red-evidence harness ↔ working tree | The harness applies a defect patch, runs a test, and reverts | Source files under `internal/`, `.github/workflows/ci.yaml` |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-01-01 | Tampering | storetest writing to an operator's Qdrant | high | mitigate | `storetest.go` reads only `ENGRAM_QDRANT_TEST_ADDR` / the booted container; `ENGRAM_QDRANT_ADDR` absent from `internal/store/storetest/` (0 matches) | closed |
| T-01-02 | Tampering | raw write path in test support bypassing schema stamping | high | mitigate | `seed.go:227` `st.Upsert`, `seed.go:205` `st.DeleteAll` only; D-13 write-check covers `storetest.go` (`TestQdrantClientIsHeldOnlyByStorePackage` PASS; red-evidence `01-01-storetest-raw-client-write.patch` confirmed RED) | closed |
| T-01-03 | Repudiation | Qdrant suite reporting green by skipping under `ENGRAM_REQUIRE_QDRANT` | medium | mitigate | Single `func RequireQdrant` (1 definition, 0 legacy `requireQdrant` copies), `strconv.ParseBool` error path; `TestRequireQdrant` PASS; `treu` exits non-zero for store/server/e2e | closed |
| T-01-04 | Tampering | test dial options diverging from production | medium | mitigate | `internal/server/tools.go` builds via `store.NewQdrantClient`; D-11 gate `TestQdrantClientConstructedOnlyByNewQdrantClient` PASS (0 non-comment `qdrant.NewClient(` outside `store.go`) | closed |
| T-01-05 | Denial of Service | oversized fixtures pressuring shared CI Qdrant (#497) | medium | mitigate | One `testing.Short()` skip in `seed.go`; 0 `t.Parallel()` in oversized tests; cleanup registered before first write | closed |
| T-01-06 | Elevation of Privilege | test-only code entering the production binary | low | mitigate | `go list -deps ./cmd/engram`: 0 storetest, 0 testcontainers; `go.mod`/`go.sum` unchanged vs `main` | closed |
| T-01-SC | Tampering | package-manager installs | low | accept | No installs, no new Go modules | closed |
| T-02-01 | Tampering | in-package test client diverging from production dial options | medium | mitigate | `TestNewQdrantClientAppliesBaseAndCallerOptions` PASS (production span + caller interceptor both fire) | closed |
| T-02-02 | Repudiation | converged helper dropping its interceptor (vacuous pass) | medium | mitigate | Interceptor helpers delegate to `dialTestClient` with the interceptor as caller option; whole `internal/store` package green under `ENGRAM_REQUIRE_QDRANT=1` | closed |
| T-02-03 | Information Disclosure | test spans leaking into span-count assertions | low | accept | Span assertions scope to `sr.Ended()[before:]` by name; no total-count assertions | closed |
| T-02-SC | Tampering | package-manager installs | low | accept | No installs | closed |
| T-03-01 | Tampering | retrievaleval/e2e dialing an operator's Qdrant | high | mitigate | e2e child env sets `ENGRAM_QDRANT_ADDR` from `storetest.Addr()` (`harness_test.go:238`, `spine_review_test.go:100`); retrievaleval mentions the ambient var only in "NEVER" doc comments | closed |
| T-03-02 | Repudiation | a package silently skipping its Qdrant tier in CI | medium | mitigate | Shared-address run PASS=3 SKIP=1 (re-run 2026-09-19 against a temporary Qdrant); `treu` fail-closed for server/e2e; retrievaleval ignores it via `IgnoreRequireQdrant` by design | closed |
| T-03-03 | Denial of Service | CI booting extra Qdrant containers (#497 regression) | medium | mitigate | Every TestMain takes storetest's `ENGRAM_QDRANT_TEST_ADDR` fast path first; PASS=3 on the shared address proves no package booted its own | closed |
| T-03-SC | Tampering | package-manager installs | low | accept | No installs | closed |
| T-04-01 | Repudiation | internal/store fail-closed gate lost in the TestMain relocation | high | mitigate | `TestDialTestClientFailsWhenRequiredAndUnavailable` PASS through the relocated `SetNoQdrantHandler` hook | closed |
| T-04-02 | Repudiation | `TestSharedQdrantAddressHonored` duplicated or lost | medium | mitigate | Exactly 1 definition in `internal/store` (`main_test.go`); CI command PASS=3 SKIP=1 | closed |
| T-04-03 | Repudiation | migrated regression passing by client-side accident | medium | mitigate | Test names `storetest.RecvLimit`; red-evidence `01-04-listscopes-full-payload-selector.patch` confirmed RED | closed |
| T-04-04 | Denial of Service | oversized fixtures accumulating in shared CI Qdrant | medium | mitigate | Fresh collection per subtest deleted in `t.Cleanup`; seeder cleanup registered before first write; no parallel subtests | closed |
| T-04-SC | Tampering | package-manager installs | low | accept | No installs | closed |
| T-05-01 | Tampering | a file dialing Qdrant outside the shared constructor | medium | mitigate | D-11 gate scans all `.go` incl. tests, closes alias/dot-import/function-value/aliased-store-import shapes (post-review fix 56723405, be18a467); `01-05-bare-qdrant-newclient-in-test.patch` confirmed RED | closed |
| T-05-02 | Repudiation | a gate scanning nothing reporting clean | medium | mitigate | Zero-applicability guards: `qdrant_client_convergence_test.go:424` (non-test and test counts), `cipin_test.go:58` (zero image matches) | closed |
| T-05-03 | Repudiation | red-evidence patch counted RED by breaking the build | medium | mitigate | Each patch hand-verified by the target's own `--- FAIL:` line before registration (01-05-SUMMARY) | closed |
| T-05-04 | Tampering | harness leaving source files mutated | medium | mitigate | Harness refuses dirty files and reverts; working tree clean after every harness run (orchestrator, verifier, reviewer runs) | closed |
| T-05-05 | Tampering | CI Qdrant image drifting from testcontainer image | low | mitigate | `TestQdrantImageMatchesCIService` PASS; `01-05-ci-qdrant-image-drift.patch` confirmed RED | closed |
| T-05-SC | Tampering | package-manager installs | low | accept | No installs | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-01 | T-01-SC, T-02-SC, T-03-SC, T-04-SC, T-05-SC | No package-manager installs and no new Go modules this phase (`go.mod`/`go.sum` unchanged vs `main`); supply-chain surface unchanged | plan-time register (01-01..01-05 PLAN) | 2026-09-18 |
| AR-02 | T-02-03 | Store package span recorder is process-wide by design; every span assertion scopes by name to spans ended after its own start, so no test asserts a global span count | plan-time register (01-02 PLAN) | 2026-09-18 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-19 | 26 | 26 | 0 | orchestrator (secure-phase, ASVS L1 grep-depth short-circuit — register authored at plan time) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-19
