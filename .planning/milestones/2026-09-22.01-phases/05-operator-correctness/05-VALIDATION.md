---
phase: "5"
slug: "operator-correctness"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-24"
---

# Phase 5 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Reconstructed retroactively (State B) on 2026-09-24: research was skipped for this phase, so
> plan-phase never seeded this file.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`; Qdrant testcontainer for `internal/store`) |
| **Config file** | none — `Taskfile.yaml` |
| **Quick run command** | `go test ./cmd/engram/ ./internal/keylinks/ ./internal/surfaces/` |
| **Full suite command** | `task` (lint + test) |
| **Estimated runtime** | ~60 seconds quick; ~3–5 minutes full (Qdrant suites) |

---

## Sampling Rate

- **After every task commit:** Run the quick run command
- **After every plan wave:** Run `go test ./...` (with `ENGRAM_RETRIEVAL_EVAL` unset)
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 300 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 5-01-01 | 01 | 1 | OPS-01 | T-05-01 / T-05-02 | Baseline rows unaffected by ambient env; no baseline row removed | unit | `env ENGRAM_REINDEX_TARGET=ambient ENGRAM_MIGRATE_OWNER=ambient ENGRAM_RUNTIME=ambient ENGRAM_HEADERS=ambient go test ./cmd/engram/ -count=1 -run '^(TestExitCodeBaseline&#124;TestHelpGolden&#124;TestCatalogGolden)$'` | ✅ | ✅ green |
| 5-02-01 | 02 | 1 | OPS-02 | T-05-03 | Bare nested-object branch reached; hostile leaves sanitized | unit | `go test ./cmd/engram/ -count=1 -run '^TestViewFieldsBareNestedObject$'` | ✅ | ✅ green |
| 5-02-02 | 02 | 1 | OPS-02 | T-05-04 | Empty nested object → zero rows; blank array element keeps its row | unit | `go test ./cmd/engram/ -count=1 -run '^(TestViewFieldsEmptyNestedObjectRendersNoRows&#124;TestViewFieldsBlankArrayElementKeepsItsRow&#124;TestOperatorViewFixturesHaveNoUnsanitizedNesting)$'` | ✅ | ✅ green |
| 5-03-01 | 03 | 1 | OPS-03 | T-05-05 / T-05-06 | Exported parser skips fieldless items; scanner still reports them malformed | unit | `go test ./internal/keylinks/ -count=1 -run '^(TestParsePlanKeyLinksSkipsFieldlessItems&#124;TestMalformedKeyLinkEntry&#124;TestActiveMilestoneKeyLinksSatisfiable)$'` | ✅ | ✅ green |
| 5-04-01 | 04 | 1 | OPS-04 | T-05-07 / T-05-08 | Below-cursor insert converges; shared batch var restored | integration | `env ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -count=1 -run '^TestMigrate(BelowCursorInsertConverges&#124;ConvergesWithoutLock)$'` | ✅ | ✅ green |
| 5-05-01 | 05 | 1 | OPS-05 | T-05-09 / T-05-10 | Operator list derived from live command tree; rule anchors untouched | unit | `go test ./cmd/engram/ -count=1 -run '^TestCLIGuideOperatorCommandsListsEveryOperatorCommand$' && go test ./internal/surfaces/ -count=1` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements.

---

## Manual-Only Verifications

All phase behaviors have automated verification.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 300s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-09-24

## Validation Audit 2026-09-24
| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

All mapped tests re-run green on HEAD (`450da757`), `TestMigrateBelowCursorInsertConverges` against a
live Qdrant testcontainer (not skipped). Each OPS test was shown red on the pre-phase tree in
05-VERIFICATION.md, so none passes vacuously.
