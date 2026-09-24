---
phase: "2"
slug: "decision-interface-jev-backend"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-23"
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`, `net/http/httptest`, `otel/sdk/trace/tracetest`) |
| **Config file** | none — `Taskfile.yaml` |
| **Quick run command** | `go test ./internal/decide/... ./internal/config/... -count=1` |
| **Full suite command** | `task` (lint + `go test ./...`) |
| **Estimated runtime** | ~30–90 seconds hermetic |

---

## Sampling Rate

- **After every task commit:** Run the quick run command
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** `task` green
- **Max feedback latency:** 90 seconds

---

## Per-Task Verification Map

Requirement → test coverage (from RESEARCH.md § Validation Architecture; the planner/executor binds task IDs):

| Requirement | Behavior | Test Type | Automated Command | File Exists | Status |
|-------------|----------|-----------|-------------------|-------------|--------|
| DEC-01 | Provider unset → no decider constructed, zero decision outbound calls | unit | `go test ./internal/server/... -run TestDecider -count=1` | ✅ | ✅ green |
| DEC-01/03 | Config: provider enum, base URL required when provider=jev, key fallback | unit | `go test ./internal/config/... -run TestDecisions -count=1` | ✅ | ✅ green |
| DEC-02 | One interface: batched Choice/Score/Noul → typed answers; structural validation; DecideMany per-item results | unit | `go test ./internal/decide/... -run 'TestDecider|TestValidate|TestDecideMany' -count=1` | ✅ | ✅ green |
| DEC-03 | Jev reaches `{base}/alpha/decisions` for both OpenRouter and LiteLLM bases | unit (httptest) | `go test ./internal/decide/jev/... -run TestJevRequestShape -count=1` | ✅ | ✅ green |
| DEC-04 | Both error dialects, timeout, oversized response, 429/5xx single retry → named errors | unit (httptest) | `go test ./internal/decide/jev/... -run TestJevErrorClassification -count=1` | ✅ | ✅ green |
| DEC-05 | SDK evaluated; adopt/reject recorded before client code | doc + spike test | `go -C .planning/phases/02-decision-interface-jev-backend/sdk-eval test -run '^TestSDKEvaluation$' -count=1 -race -v ./...` (nested module, plan 02-02) + `rg '^resolution: ' 02-SDK-EVALUATION.md` | ✅ | ✅ green |
| DEC-06 | `decide` span with provider/model/questions/tokens/cost/status attributes | unit (tracetest) | `go test ./internal/decide/jev/... -run TestJevDecideEmitsSpan -count=1` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] SDK evaluation spike: replay the spike's captured LiteLLM (string `code`) and OpenRouter (numeric `code`) error bodies through the SDK; record decode behavior (resolves D-06)
- [x] `internal/decide/decide_test.go` — interface/type/validation contract tests
- [x] `internal/decide/jev/jev_test.go` — httptest fixtures for both dialects, timeout, retry, response-too-large, span attributes

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live call against OpenRouter and the LiteLLM pass-through | DEC-03 | Needs live keys and network | With `ENGRAM_DECISIONS_PROVIDER=jev` and each base URL, issue one Decide call (e.g. a small test/CLI harness) and confirm a typed answer |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 90s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-09-23

## Validation Audit 2026-09-23
| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

Mapped tests re-run green (internal/decide, internal/decide/jev, internal/config, internal/server) plus the nested sdk-eval TestSDKEvaluation. Manual-only live check executed and PASSED on both bases (02-LIVE-CHECK.md).
