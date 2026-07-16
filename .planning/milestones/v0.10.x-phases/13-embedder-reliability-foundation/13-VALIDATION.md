---
phase: 13
slug: embedder-reliability-foundation
status: ready
nyquist_compliant: true
wave_0_complete: false
created: 2026-07-10
---

# Phase 13 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `13-RESEARCH.md` § Validation Architecture. Per-task rows are
> filled once the planner assigns task IDs.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `testify` v1.11.1 |
| **Config file** | none — `go test` via `Taskfile.yaml` |
| **Quick run command** | `go test ./internal/embed/... ./internal/config/... ./internal/store/... ./internal/server/...` |
| **Full suite command** | `task test` (= `go test ./...` + python hook tests) |
| **Estimated runtime** | ~30–90 seconds (unit); integration adds testcontainers-qdrant spin-up |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/embed/... ./internal/config/... ./internal/store/... ./internal/server/...`
- **After every plan wave:** Run `task test`
- **Before `/gsd-verify-work`:** `task` (lint + test) must be green
- **Max feedback latency:** ~90 seconds (unit); longer only for the qdrant integration cases

---

## Per-Task Verification Map

> Task IDs assigned post-planning (positional `13-PP-TT`). The plan-checker
> verified each row maps 1:1 to a task's `<automated>` command in 13-01/02/03.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 13-01-01 | 01 | 1 | REQ-embed-timeout | — | timeout override honored; `0`=infinite; negative rejected (UNGATED) | unit | `go test ./internal/config/... -run TestValidate -v` | ✅ extend | ⬜ pending |
| 13-01-01 | 01 | 1 | REQ-embed-timeout | — | slow/hung embed call cut short by configured timeout | unit (httptest+sleep) | `go test ./internal/embed/... -run TestEmbedTimeout -v` | ❌ W0 | ⬜ pending |
| 13-01-03 | 01 | 1 | REQ-embed-timeout | — | D-09: `maxElapsed` derives from `ENGRAM_SUMMARY_TIMEOUT`, independent of embed | regression | `go test ./internal/server/... -run TestSummaryQueue -v` | ✅ confirm | ⬜ pending |
| 13-01-02 | 01 | 1 | REQ-embed-baseurl-join | — | provider-shape table (OpenRouter/OpenAI `/v1`/OpenAI bare/trailing-slash/Gemini/override) | unit (table) | `go test ./internal/embed/... -run TestJoinEmbeddingsURL -v` | ❌ W0 | ⬜ pending |
| 13-01-02 | 01 | 1 | REQ-embed-baseurl-join | — | override escape hatch wins over heuristic; validated at load | unit | `go test ./internal/config/... -run TestValidate -v` | ❌ W0 | ⬜ pending |
| 13-02-01 | 02 | 2 | REQ-embed-config-identity | — | `EmbedderIdentity(cfg)` deterministic; excludes query-side/base_url/api_key/timeout (D-01) | unit (table) | `go test ./internal/config/... -run TestEmbedderIdentity -v` | ❌ W0 | ⬜ pending |
| 13-02-02 | 02 | 2 | REQ-embed-config-identity | — | `Memory.EmbedderIdentity` round-trips payload; legacy-missing reads `""` | unit | `go test ./internal/store/... -run TestPayload -v` | ✅ extend | ⬜ pending |
| 13-02-03 | 02 | 2 | REQ-embed-config-identity | — | 4 non-reindex write sites (store/update/discovery/rule) stamp identity | unit+integration | `go test ./internal/server/... -run 'TestRecallView\|TestStore' -v` | ✅ extend | ⬜ pending |
| 13-03-01 | 03 | 3 | REQ-embed-config-identity | — | 5th write site: reindex raw-map stamp (verbatim-payload landmine) | integration (qdrant) | `go test ./internal/store/... -run TestReindex -v` | ✅ extend | ⬜ pending |
| 13-02-03 | 02 | 2 | REQ-embed-config-identity | — | D-06: `recallView` does NOT surface the identity field | unit (negative) | `go test ./internal/server/... -run TestRecallView -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/embed/embed_test.go` — `TestJoinEmbeddingsURL` table test (D-12): OpenRouter, OpenAI `/v1`, OpenAI **bare host (no `/v1`)**, trailing-slash, Gemini `/v1beta/openai`, override
- [ ] `internal/embed/embed_test.go` — `TestEmbedTimeout…` mirroring `internal/summarize/summarize_test.go:199` (`TestSummarizeWithTimeoutCancelsSlowRequest`)
- [ ] `internal/config/validate_test.go` — `embed.timeout` duration cases mirroring `summarize.timeout` (`validate_test.go:95-96`) but **UNGATED** (embed is always active — Research Pitfall 3)
- [ ] `internal/config/validate_test.go` (or new `internal/config/identity_test.go`) — `embedderIdentity(cfg)` determinism + field-exclusion table test
- [ ] `internal/server/summary_test.go` — negative assertion that `recallView`/`toRecallView` never surfaces the identity field (D-06)
- [ ] `internal/store/store_test.go` — reindex-stamps-identity integration test (requires `ReindexOptions.Identity` to exist first — see Research Pitfall 1/2)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real provider brownout (529) resolves within timeout against a live OpenAI-compat endpoint | REQ-embed-timeout | Requires a real external provider outage; httptest slow-server covers the mechanism automatically | Optional smoke: point `ENGRAM_OPENAI_BASE_URL` at a throttled endpoint, set `ENGRAM_EMBED_TIMEOUT=2s`, confirm `store_memory` returns an error within ~2s |

*All primary phase behaviors have automated verification; the row above is an optional real-provider smoke check only.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags (`go test`, no watch)
- [x] Feedback latency < 90s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-07-10 — plan-checker verified each test-map row maps 1:1 to a task `<automated>` command across 13-01/02/03 (`wave_0_complete` flips true when the Wave-0 stubs land during execution).
