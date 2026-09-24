---
phase: "3"
slug: "curation-verdicts"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-23"
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`, `httptest`) |
| **Config file** | none — `Taskfile.yaml`; eval gate is package-local koanf (never registered) |
| **Quick run command** | `go test ./cmd/engram/... ./internal/store/... ./internal/config/... ./internal/surfaces/...` |
| **Full suite command** | `task` (lint + `go test ./...`) |
| **Estimated runtime** | ~2–5 minutes (store package includes Qdrant testcontainer suites) |

---

## Sampling Rate

- **After every task commit:** narrowest relevant package from the quick run command
- **After every plan wave:** `go test ./...`
- **Before `/gsd-verify-work`:** `task` green; `go test ./cmd/engram -update` golden diff reviewed, not blindly committed
- **Max feedback latency:** 300 seconds

---

## Per-Task Verification Map

Requirement → test coverage (from RESEARCH.md § Validation Architecture; planner/executor binds task IDs):

| Requirement | Behavior | Test Type | Automated Command | File Exists | Status |
|-------------|----------|-----------|-------------------|-------------|--------|
| CUR-01 | Verdict object present in JSON when decider configured; absent otherwise; `--no-verdicts` suppresses | unit | `go test ./cmd/engram/ -run TestSpineReviewConsolidate -v` | ✅ | ✅ green |
| CUR-01 | Text view renders verdict / p / same-subject / needs-review, sanitized nesting | unit | `go test ./cmd/engram/ -run TestOperatorViewFixturesHaveNoUnsanitizedNesting -v` | ✅ | ✅ green |
| CUR-02 | Below-threshold → `needs_review: true`; no write RPC ever issued by the verdict pass | unit | `go test ./cmd/engram/ -run TestConsolidate -v` + store mutation-proof test for the new fetch | ✅ | ✅ green |
| CUR-02 | Threshold config/flag parsing (probability range) | unit | `go test ./internal/config/ -run 'TestDecisions|TestParseProbability' -v` | ✅ | ✅ green |
| CUR-03 | Accuracy by confidence bucket + Brier; p≥threshold accuracy ≥ 0.9 on committed set | integration (gated, live) | `task eval:curation` (new target) | ✅ | ✅ green |
| CUR-03 | Metrics + fixture integrity (hermetic) | unit | `go test ./internal/<eval pkg>/ -run 'TestBrier|TestAccuracyByBucket|TestPairFixtureIntegrity' -v` | ✅ | ✅ green |
| CUR-04 | Neither `--scope` nor `--all-scopes` → registered rule error, `exitUsage` | unit | `go test ./cmd/engram/ -run TestSweepLeavesRejectMissingScopeIdentically -v` | ✅ | ✅ green |
| Docs | New `ENGRAM_DECISIONS_*` keys documented | unit | `go test ./internal/config/ -run TestDecisionsVarsDocumented -v` | ✅ | ✅ green |
| Golden | CLI help/catalog reflect new flags and enforced scope rule | golden | `go test ./cmd/engram/ -run TestGolden -v` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] Store: bounded-read view + batched by-id fetch for pair state (summary + content head), with mutation-proof and byte-ceiling tests
- [x] Wiring: exported `StoreAndDeciderFromEnv`-shaped constructor + threshold/truncation resolvers, with tests
- [x] Config: probability parser + positive/negative boundary tests
- [x] New curation eval package (gate, fixtures, blind-authoring doc, local-file loader, metrics, gated entry point) + `eval:curation` Taskfile target
- [x] Operator view: sanitized nested-object rendering for `verdict`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live eval against Jev | CUR-03 | Needs Decisions provider credentials | `ENGRAM_DECISIONS_PROVIDER=jev ENGRAM_DECISIONS_BASE_URL=… task eval:curation`; record the bucket table |
| Live consolidate with verdicts on a real spine | CUR-01/02 | Needs Qdrant with real data + provider | Run `engram spine-review consolidate --scope <spine> --output json` with decisions enabled; confirm verdict objects and no record changes |
| Blind labeling procedure | CUR-03 | Process | Confirm the fixture doc records the author/blind-labeler procedure and only agreed pairs were kept |

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

Mapped tests re-run green across cmd/engram, internal/config, internal/verdict, internal/curationeval, internal/store (incl. TestRecordStatesDoesNotMutate, TestSpineReviewConsolidateNoProviderByteIdentical, TestOperatorViewFixturesHaveNoUnsanitizedNesting, TestSweepLeavesRejectMissingScopeIdentically). Live eval executed: D-03 gate PASS (40/40 at p≥0.9), results in 03-EVAL-RESULTS.md. Private real-spine mode not exercised (optional).
