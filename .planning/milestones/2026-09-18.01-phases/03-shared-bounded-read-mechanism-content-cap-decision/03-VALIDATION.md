---
phase: "3"
slug: "shared-bounded-read-mechanism-content-cap-decision"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-19"
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test + `internal/store/storetest` (Phase 1) |
| **Config file** | none |
| **Quick run command** | `go test ./internal/store/... ./internal/server/... ./internal/config/... -short -count=1` |
| **Full suite command** | `task` |
| **Estimated runtime** | ~300 seconds (full, real Qdrant) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/store/... ./internal/server/... ./internal/config/... -short -count=1`
- **After every plan wave:** Run `ENGRAM_REQUIRE_QDRANT=1 task`
- **Before `/gsd-verify-work`:** `task` green
- **Max feedback latency:** 120 seconds (quick run)

---

## Per-Task Verification Map

Requirement-level rows seeded from RESEARCH.md § Validation Architecture; the planner refines them
to task IDs and real test names (re-resolve every `-run` against `go test -list`, durable record
`bsbsvn4hbc`).

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 03-01-T1 | 03-01 | 1 | REQ-content-cap-decided | T-03-01-01 | configured content cap reaches the MCP `store_memory` rejection | unit (hermetic, spy store) | `go test ./internal/server/ -run '^TestMemoryContentCapFlowsFromConfig$' -count=1` | ✅ | ✅ green |
| 03-01-T2 | 03-01 | 1 | REQ-content-cap-decided | T-03-01-03 | `0` / non-positive caps fail startup | unit | `go test ./internal/config/ -run '^TestMemoryCap' -count=1` (`TestMemoryCapsRejectZeroAndNonPositive`, `TestMemoryCapDefaultsAndEnv`) | ✅ | ✅ green |
| 03-01-T3 | 03-01 | 1 | REQ-content-cap-decided | T-03-01-01, T-03-01-02, T-03-01-04 | every create lane rejects over-cap content/tags; no value echo | unit (hermetic MCP + Connect) | `go test ./internal/server/ -run '^TestMemoryWriteCap' -count=1` (`…RejectOnEveryCreateLane`, `…Boundaries`, `…DefaultsMatchRegistry`) plus 03-01 Task 3 verify for `TestHintNeverEchoesValue` | ✅ | ✅ green |
| 03-02-T1 | 03-02 | 1 | REQ-byte-budget-pages | T-03-02-01 | per-RPC count derived from the view ceiling | integration (real Qdrant) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestScrollAllPointsByteBudget$' -count=1` | ✅ | ✅ green |
| 03-02-T2 | 03-02 | 1 | REQ-byte-budget-pages (D-07) | T-03-02-02, T-03-02-03 | never skip a record; ceiling holds at every cap | integration (legacy records via `Store.Upsert`) + unit | 03-02 Task 2 verify (`TestScrollAllPointsBatchOfOneFallback`, `TestScrollAllPointsSingleOversizedRecordFailsNamed`, `TestRecordCeilingHoldsForMaxCapRecord`, `TestRecordCeilingDerivesFromCaps`, `TestWithRecordCapsNormalizesNonPositive`, `TestSweepLimit`) | ✅ | ✅ green |
| 03-03-T1 | 03-03 | 2 | REQ-content-cap-decided | T-03-03-01 | Connect field-mask update lane capped inside `deps.updateMemory` | unit (hermetic Connect) | `go test ./internal/server/ -run '^TestUpdateMemoryContentCap$' -count=1` | ✅ | ✅ green |
| 03-03-T2 | 03-03 | 2 | REQ-content-cap-decided | T-03-03-02 | existing over-cap records stay readable and trimmable | unit (hermetic MCP + Connect) | `go test ./internal/server/ -run '^TestUpdateMemoryTagsCap$' -count=1`; `go test ./internal/server/ -run '^TestUpdateMemoryLegacyOversizedRecord$' -count=1` | ✅ | ✅ green |
| 03-03-T3 | 03-03 | 2 | REQ-byte-budget-pages | T-03-03-04 | read ceilings derive from the enforced caps | unit + integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run 'RecordCaps' -count=1` | ✅ | ✅ green |
| 03-04-T1 | 03-04 | 2 | REQ-byte-budget-pages | T-03-04-01, T-03-04-04 | ordered page stops on accumulated bytes | integration (real Qdrant) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestScrollOrderedPageByteBudget$' -count=1` | ✅ | ✅ green |
| 03-04-T2 | 03-04 | 2 | REQ-byte-budget-pages (D-07) | T-03-04-02, T-03-04-03 | caller filter never widened; budget-cut page never final; nothing skipped | integration (real Qdrant) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestScrollOrderedPage' -count=1` | ✅ | ✅ green |
| 03-04-T3 | 03-04 | 2 | REQ-byte-budget-pages (D-08 inventory) | — | N/A | artifact set-equality | 03-04 Task 3 verify (derived set equals the inventory table, 27 functions) | ✅ | ✅ green |
| 03-05-T1 | 03-05 | 3 | REQ-content-cap-decided | T-03-05-01 | CLI lane rejected through the real binary | e2e (real server + Qdrant) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/e2e/ -run '^TestCLIStoreRejectsOversizedContentAndTags$' -count=1` | ✅ | ✅ green |
| 03-05-T2 | 03-05 | 3 | REQ-content-cap-decided | T-03-05-03 | N/A | docs build + doc gates | 03-05 Task 2 verify (docs build, four doc-gate tests, errors.md patch still applies) | ✅ | ✅ green |
| 03-05-T3 | 03-05 | 3 | REQ-content-cap-decided | — | N/A | drift gate | `go test ./internal/skills/ -run '^TestSkillsEmbedMatchesVendored$' -count=1` | ✅ | ✅ green |
| 03-06-T1/T2 | 03-06 | 4 | REQ-byte-budget-pages, REQ-content-cap-decided | T-03-06-01, T-03-06-02 | every guarantee has a live RED proof | red-evidence harness | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1 -timeout 60m` (25 `confirmed RED:`) | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] ordered-page helper test file — `internal/store/orderedpage_oversized_test.go` (03-04)
- [x] `scrollAllPoints` byte-budget test coverage — `internal/store/boundedread_oversized_test.go`, `internal/store/boundedread_test.go` (03-02)
- [x] legacy over-cap fixtures written with `Store.Upsert` (the raw write; `SeedOversized` refuses a single record at or over the limit) — in `boundedread_oversized_test.go` and `orderedpage_oversized_test.go` (`package store_test`, so they can name `storetest.RecvLimit`)
- [x] content/tags cap rejection tests (all write paths, both lanes, CLI) — `internal/server/contentcap_test.go` (03-01), `internal/server/updatecap_test.go` (03-03), `internal/e2e/contentcap_cli_test.go` (03-05)
- [x] this phase's `redEvidenceDirs` entry + 13 hand-verified patches (03-06)

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

Orchestrator re-ran every per-task command at HEAD (post code-review fix `7e6113cc`) in one `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ ./internal/server/ ./internal/config/ ./internal/e2e/ ./internal/skills/ -count=1 -v` run: exit 0, 0 top-level `--- FAIL`, all 23 named tests `--- PASS` (none skipped), `TestRedEvidencePatchesAreLive` with 25 `confirmed RED`. Post-merge gate `ENGRAM_REQUIRE_QDRANT=1 task` green after the last plan.
