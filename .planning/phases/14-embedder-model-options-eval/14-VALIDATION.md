---
phase: 14
slug: embedder-model-options-eval
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-11
---

# Phase 14 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Source of truth for the reasoning behind this map: `14-RESEARCH.md` § Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (stdlib) + `testcontainers-go` (ephemeral Qdrant) |
| **Config file** | none — env-gated (`ENGRAM_RETRIEVAL_EVAL`, `ENGRAM_QDRANT_TEST_ADDR`, embed-provider env) |
| **Quick run command** | `task` (lint + `go test ./...`) — `internal/retrievaleval`'s `TestMain` short-circuits to zero cost when `ENGRAM_RETRIEVAL_EVAL` is unset, so the default gate stays fast |
| **Full suite command** | `task eval:retrieval` (`ENGRAM_RETRIEVAL_EVAL=1 go test ./internal/retrievaleval/ -run TestRetrievalEval -v`) — manual, needs a live embedding gateway + Docker or `ENGRAM_QDRANT_TEST_ADDR` |
| **Estimated runtime** | default `task` ~seconds; `task eval:retrieval` minutes (network-bound, cloud gateway) |

---

## Sampling Rate

- **After every task commit:** Run `task` (lint + default `go test ./...`; retrieval-eval is a zero-cost skip when the manual gate is unset).
- **After every plan wave:** `task` green; run `task eval:retrieval` **manually** once Gemini/OpenRouter env + Docker (or `ENGRAM_QDRANT_TEST_ADDR`) are available (D-06 — local/manual, no CI this phase).
- **Before `/gsd-verify-work`:** `task` green AND the committed eval-evidence artifact captured (D-07 — closes success criteria #1/#2, since the eval is not a CI gate).
- **Max feedback latency:** default gate seconds; manual eval run is an explicitly out-of-band pre-merge step.

---

## Per-Task Verification Map

> Task IDs are assigned by the planner (`*-PLAN.md`). Rows below are keyed by requirement until then; the executor fills `Task ID` / `Status` as tasks land.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 14-??-?? | ?? | ? | REQ-embed-gemini-direct | T-14-01 (info-disclosure: no real keys/content) | Gemini query vector ≠ document vector for identical text (instruction-prefix takes effect; Pitfall 12 guard) | integration (live gateway) | `ENGRAM_RETRIEVAL_EVAL=1 ENGRAM_EMBED_MODEL=gemini-embedding-2 … go test ./internal/retrievaleval/ -run TestRetrievalEval -v` | ❌ W0 — new differ-case in `fixtures.go`/`retrieval_eval_test.go` | ⬜ pending |
| 14-??-?? | ?? | ? | REQ-embed-prod-parity-eval | — | `gh261Case` Record T within default k=8 against qwen3@4096 | integration (hard rank assertion, already implemented) | `ENGRAM_RETRIEVAL_EVAL=1 ENGRAM_EMBED_MODEL=qwen/qwen3-embedding-8b ENGRAM_EMBED_DIM=4096 … task eval:retrieval` | ✅ existing gate — env-only change, no new test code | ⬜ pending |
| 14-??-?? | ?? | ? | REQ-embed-model-docs | T-14-01 | Recipes render/lint + cross-link `guides/reindex`; placeholder secrets only | manual-only (docs correctness not unit-testable) | `task lint` (structure/lint only) | ❌ W0 — new `embedding-models.md` | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Gemini differ-case fixture/assertion in `internal/retrievaleval/fixtures.go` + `retrieval_eval_test.go` — covers REQ-embed-gemini-direct (D-04; minimal 2-record/1-string probe recommended per RESEARCH Open Question 2)
- [ ] `docs-site/src/content/docs/guides/embedding-models.md` — covers REQ-embed-model-docs (D-11)
- [ ] Committed eval-evidence artifact (e.g. `14-EVAL-EVIDENCE.md`, location per RESEARCH Open Question 3) — closes success criteria #1/#2 per D-07

*No new test-framework install needed — `testing` + `testcontainers-go` already power `TestRetrievalEval`.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Gemini query≠document differ-assertion PASS | REQ-embed-gemini-direct | Needs a live Gemini API key + gateway; no secrets in CI (D-06) | Set Gemini recipe env + `ENGRAM_RETRIEVAL_EVAL=1`, run `task eval:retrieval`, capture PASS line into the evidence artifact |
| `gh261Case` recall@8 PASS on qwen3@4096 | REQ-embed-prod-parity-eval | Needs a live OpenRouter API key + gateway (D-08) | Set qwen3@4096 recipe env + `ENGRAM_RETRIEVAL_EVAL=1`, run `task eval:retrieval`, capture recall@8 numbers into the evidence artifact |
| Docs content correctness (values, dims, cross-links resolve) | REQ-embed-model-docs | Prose/recipe accuracy is not unit-testable; `task lint` only checks structure | Human review of `embedding-models.md` against the RESEARCH Provider Recipe Matrix + a live `curl` model-id check (RESEARCH Open Question 1) |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < default-gate seconds
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
