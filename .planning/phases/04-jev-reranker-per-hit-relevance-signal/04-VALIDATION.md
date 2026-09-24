---
phase: "4"
slug: "jev-reranker-per-hit-relevance-signal"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-24"
---

# Phase 4 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`, `httptest`) |
| **Config file** | none — `Taskfile.yaml` |
| **Quick run command** | `go test ./internal/store/... ./internal/server/... ./internal/config/... ./internal/decide/jev/... ./cmd/engram/...` |
| **Full suite command** | `task` (lint + test); plus `task proto:lint` and `buf breaking` for the proto field |
| **Estimated runtime** | ~2–5 minutes (Qdrant testcontainer suites) |

---

## Sampling Rate

- **After every task commit:** targeted packages from the quick run command
- **After every plan wave:** `go test ./...`
- **Before `/gsd-verify-work`:** `task` green, `task proto:lint`, buf breaking clean; live `task eval:retrieval` with the Jev slot enabled recorded as the D-02 artifact
- **Max feedback latency:** 300 seconds

---

## Per-Task Verification Map

Requirement → test coverage (from RESEARCH.md § Validation Architecture; planner/executor binds task IDs):

| Requirement | Behavior | Test Type | Automated Command | File Exists | Status |
|-------------|----------|-----------|-------------------|-------------|--------|
| RANK-03 | Jev re-sorts lexical order; falls back to lexical on error/timeout; call succeeds | unit + integration | `go test ./internal/store/ -run '^TestRankWithHook' -v` and `go test ./internal/server/ -run '^(TestSearchRerankJevTracer\|TestSearchRerankNoRetryAndTimeout)$' -v` (plans 04-01, 04-03) | ❌ W0 | ⬜ pending |
| RANK-03 | `rankCandidates` still the D-05 winner (pin unchanged) | unit | `go test ./internal/store/ -run '^TestRankCandidatesIsTheD05Winner$' -v` | ✅ keep | ⬜ pending |
| RANK-03 | MCP / Connect parity incl. jev-enabled hook | integration | `go test ./internal/server/ -run '^TestRerankParityMCPAndConnect$' -v` (plan 04-04 adds five `jev_hook` subtests) | ✅ extend | ⬜ pending |
| RANK-03 | search_discovery default byte-identical; jev path reranks | unit + integration | `go test ./internal/store/ -run '^TestSearchDiscoveryReranked' -v` and `go test ./internal/server/ -run '^(TestSearchDiscoveryDefaultPathUnchanged\|TestSearchDiscoveryRelevanceBothLanes)$' -v` (plan 04-05) | ❌ W0 | ⬜ pending |
| RANK-04 | `relevance` on MCP JSON, Connect proto, CLI when jev succeeded; absent otherwise | unit | `go test ./internal/server/ -run '^(TestToRecallViewCarriesRelevance\|TestShapeRecallRelevanceFullAndCompact\|TestSearchRerankJevTracer)$' -v` and `go test ./cmd/engram/ -run '^(TestClientSearchTextOutputRelevanceColumn\|TestClientSearchJSONCarriesRelevance\|TestClientListNeverShowsRelevance)$' -v` (plans 04-01, 04-04) | ❌ W0 | ⬜ pending |
| RANK-04 | Proto field additive | CI gate | `task proto:lint` + `go tool buf breaking --against '.git#branch=main'` | ✅ | ⬜ pending |
| RANK-05 | Query + 100 candidates under the token budget | unit | `go test ./internal/relevance/ -run '^(TestNewRequest\|TestEstimateTokensCeil)' -v` and `go test ./internal/store/ -run '^TestSearchRerankedRankHookPoolAtRecallMaximum$' -v` (plans 04-02, 04-01) | ❌ W0 | ⬜ pending |
| D-01 | ranker=jev without provider fails validation | unit | `go test ./internal/config/ -run '^(TestSearchRegistryEntries\|TestSearchConfigValidate\|TestSearchVarsDocumented)$' -v` (plan 04-03) | ❌ W0 | ⬜ pending |
| D-09 | Search-path client never retries; sweep client still retries once | unit | `go test ./internal/decide/jev/ -run '^TestJevNoRetryOption$' -v` and `go test ./internal/server/ -run '^TestDeciderFromConfigStillRetries$' -v` (plan 04-03) | ❌ W0 | ⬜ pending |
| D-02 | Jev eval row opt-in, outside D-05 | unit | `go test ./internal/retrievaleval/ -run '^(TestEvalRankersJevEnabled\|TestDecideRanking)$' -v` (plan 04-06) | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Rank-hook composition + fallback tests (store)
- [ ] Search config tests (ranker enum, rerank timeout, jev-requires-provider)
- [ ] jev no-retry option test
- [ ] Relevance field tests on MCP / Connect / CLI
- [ ] Token-budget guard test at 100 candidates

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live Jev ranking numbers (D-02 artifact) | RANK-03 | Needs Qdrant + embedder + Decisions credentials | `ENGRAM_DECISIONS_PROVIDER=jev ENGRAM_DECISIONS_BASE_URL=… task eval:retrieval`; record the Jev row and no-answer relevance values |
| Live search with ranker=jev | RANK-03/04 | Needs a running server with data | `ENGRAM_SEARCH_RANKER=jev` server; `engram search` shows relevance; kill provider → results still return in lexical order |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 300s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
