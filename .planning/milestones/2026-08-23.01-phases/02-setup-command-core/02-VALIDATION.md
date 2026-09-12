---
phase: 2
slug: setup-command-core
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-29
validated: 2026-09-12
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (stdlib) |
| **Config file** | none — golden fixtures under `cmd/engram/testdata/` |
| **Quick run command** | `go test ./cmd/engram/... ./internal/setup/... -run TestSetup` |
| **Full suite command** | `task test` |
| **Estimated runtime** | ~60 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/setup/... ./cmd/engram/... -run <newly-added-test-name>`
- **After every plan wave:** Run `task test`
- **Before `/gsd-verify-work`:** Full suite green, plus `task surfaces:gen` run and its golden diff committed
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 2-01-01 | 01 | 1 | REQ-setup-detects-runtimes | — | Binary is the signal; config dir alone never reads as installed | unit | `go test ./internal/setup/ -run 'TestDetectEveryRegisteredRuntime\|TestDetectIgnoresConfigDirectory' -count=1` | ✅ | ✅ green |
| 2-01-02 | 01 | 1 | REQ-setup-previews-by-default | — | No runtime CLI executed without `--apply` | integration | `go test ./cmd/engram/ -run 'TestSetupPreviewExecutesNoRuntimeCLI\|TestSetupPreviewJSONHasClaudeCodeCommand' -count=1` | ✅ | ✅ green |
| 2-01-02 | 01 | 1 | REQ-setup-non-interactive | — | N/A | integration | `go test ./cmd/engram/ -run 'TestSetupAllThreeRuntimesPresentEmitsThreeRows\|TestSetupRejectsUnknownRuntime' -count=1` | ✅ | ✅ green |
| 2-01-03 | 01 | 1 | REQ-setup-correct-by-reading | — | Bearer token never read from file; redacted by provenance (D-16) | unit | `go test ./cmd/engram/ ./internal/setup/ -run 'TestSetupHelpNamesEveryRuntimeAndAuthMode\|TestPlanBearerNeverReadsTokenFile\|TestPlanBearerRedactsCredentialByProvenance' -count=1` | ✅ | ✅ green |
| 2-02-01 | 02 | 2 | REQ-setup-partial-failure-legible | — | Exit codes 8/9 catalogued and pinned | unit+golden | `go test ./cmd/engram/ -run 'TestCatalogListsEveryExitCode\|TestCatalogExitCodesMatchMapper\|TestExitCodeBaselineClaims\|TestExitCodeBaselineRowCount\|TestCatalogGolden' -count=1` | ✅ | ✅ green |
| 2-02-02 | 02 | 2 | REQ-setup-partial-failure-legible | — | Classify is exhaustive over outcome combinations | unit | `go test ./internal/setup/ -run 'TestClassify' -count=1` | ✅ | ✅ green |
| 2-02-03 | 02 | 2 | REQ-setup-partial-failure-legible | — | Per-runtime rows survive partial failure | integration | `go test ./cmd/engram/ -run 'TestSetupApplyAllAbsentExitsZero\|TestSetupApplyAtLeastOnePresentExitsSetupFailed\|TestSetupPreviewExitsZeroRegardlessOfPresence\|TestSetupApplyJSONEmitsPerRuntimeOutcome\|TestSetupExitCodes\|TestSetupViewIdentity' -count=1` | ✅ | ✅ green |
| 2-03-01 | 03 | 3 | REQ-setup-correct-by-reading | — | `ENGRAM_URL`/`ENGRAM_AUTH` env defaults advertised by `--help` actually reach the command (CR-01) | integration | `go test ./cmd/engram/ -run 'TestSetupURLFromEnvReachesCommand\|TestSetupAuthFromEnvSelectsBearerForm\|TestSetupMissingURLIsUsageError\|TestSetupFlagBeatsEnvForURL\|TestSetupEnvURLPassedVerbatim\|TestSetupEnvLanePreviewDeterministic' -count=1` | ✅ | ✅ green |
| 2-03-02 | 03 | 3 | REQ-setup-correct-by-reading | — | Repeated `--runtime` names dedupe | unit | `go test ./internal/setup/ -run 'TestSelectDedupesRepeatedNames' -count=1` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Task IDs reconciled by `/gsd-validate-phase` on 2026-09-12 from 02-01..02-03 PLAN/SUMMARY coverage blocks. REQ-setup-idempotent is owned by Phase 3 (traceability) and is validated there.*

---

## Wave 0 Requirements

- [x] `internal/setup/detect_test.go` — covers REQ-setup-detects-runtimes via an injectable `Environment` fake (never a real `PATH` mutation; never asserts third-party CLI behavior, per rule `m45p2b4bp7`)
- [x] `cmd/engram/setup_test.go` — covers preview/apply/non-interactive/exit-code criteria; modeled on `cmd/engram/prune_test.go`
- [x] `destructive_test.go` `TestMutatingCommandNamesMembership` — `"setup": true` pinned in the same commit as the `toolclass.go` row
- [x] `catalog_test.go` `wantExitCodes` / `nonConnectProducedCodes` — the two new exit-code entries pinned in the same commit as the new consts
- [x] `testdata/catalog.golden`, `testdata/help.golden` — regenerated via `task surfaces:gen` after `setup` was wired

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real-machine runtime detection against actually-installed `claude`/`codex`/`opencode` binaries | REQ-setup-detects-runtimes | Asserting on third-party binaries' presence would violate rule `m45p2b4bp7`; automated tests use a fake `Environment` | Run `engram setup` on a machine with a known runtime mix; confirm the report matches reality |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 60s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-09-12 by `/gsd-validate-phase 2`

---

## Validation Audit 2026-09-12

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

All 5 Phase 2 requirements are COVERED by committed tests; every SUMMARY-referenced test function exists and `go test ./cmd/engram/ ./internal/setup/ -count=1` is green at `e9cf19dd`. The one Manual-Only row (real-machine detection) is retained — it asserts third-party binary presence, which rule `m45p2b4bp7` forbids automating.
