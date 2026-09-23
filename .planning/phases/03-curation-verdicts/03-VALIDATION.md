---
phase: "3"
slug: "curation-verdicts"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
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
| CUR-01 | Verdict object present in JSON when decider configured; absent otherwise; `--no-verdicts` suppresses | unit | `go test ./cmd/engram/ -run TestSpineReviewConsolidate -v` | ✅ extend | ⬜ pending |
| CUR-01 | Text view renders verdict / p / same-subject / needs-review, sanitized nesting | unit | `go test ./cmd/engram/ -run TestOperatorViewFixturesHaveNoUnsanitizedNesting -v` | ✅ extend | ⬜ pending |
| CUR-02 | Below-threshold → `needs_review: true`; no write RPC ever issued by the verdict pass | unit | `go test ./cmd/engram/ -run TestConsolidate -v` + store mutation-proof test for the new fetch | ❌ W0 | ⬜ pending |
| CUR-02 | Threshold config/flag parsing (probability range) | unit | `go test ./internal/config/ -run 'TestDecisions|TestParseProbability' -v` | ❌ W0 | ⬜ pending |
| CUR-03 | Accuracy by confidence bucket + Brier; p≥threshold accuracy ≥ 0.9 on committed set | integration (gated, live) | `task eval:curation` (new target) | ❌ W0 | ⬜ pending |
| CUR-03 | Metrics + fixture integrity (hermetic) | unit | `go test ./internal/<eval pkg>/ -run 'TestBrier|TestAccuracyByBucket|TestPairFixtureIntegrity' -v` | ❌ W0 | ⬜ pending |
| CUR-04 | Neither `--scope` nor `--all-scopes` → registered rule error, `exitUsage` | unit | `go test ./cmd/engram/ -run TestSweepLeavesRejectMissingScopeIdentically -v` | ✅ extend | ⬜ pending |
| Docs | New `ENGRAM_DECISIONS_*` keys documented | unit | `go test ./internal/config/ -run TestDecisionsVarsDocumented -v` | ✅ update count | ⬜ pending |
| Golden | CLI help/catalog reflect new flags and enforced scope rule | golden | `go test ./cmd/engram/ -run TestGolden -v` | ✅ regenerate | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Store: bounded-read view + batched by-id fetch for pair state (summary + content head), with mutation-proof and byte-ceiling tests
- [ ] Wiring: exported `StoreAndDeciderFromEnv`-shaped constructor + threshold/truncation resolvers, with tests
- [ ] Config: probability parser + positive/negative boundary tests
- [ ] New curation eval package (gate, fixtures, blind-authoring doc, local-file loader, metrics, gated entry point) + `eval:curation` Taskfile target
- [ ] Operator view: sanitized nested-object rendering for `verdict`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live eval against Jev | CUR-03 | Needs Decisions provider credentials | `ENGRAM_DECISIONS_PROVIDER=jev ENGRAM_DECISIONS_BASE_URL=… task eval:curation`; record the bucket table |
| Live consolidate with verdicts on a real spine | CUR-01/02 | Needs Qdrant with real data + provider | Run `engram spine-review consolidate --scope <spine> --output json` with decisions enabled; confirm verdict objects and no record changes |
| Blind labeling procedure | CUR-03 | Process | Confirm the fixture doc records the author/blind-labeler procedure and only agreed pairs were kept |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 300s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
