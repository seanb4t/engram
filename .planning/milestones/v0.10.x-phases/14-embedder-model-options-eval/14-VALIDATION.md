---
phase: 14
slug: embedder-model-options-eval
status: planned
nyquist_compliant: true
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
| **Full suite command** | `task eval:retrieval` (`ENGRAM_RETRIEVAL_EVAL=1 go test ./internal/retrievaleval/ -run TestRetrievalEval -v`) — manual, needs a live embedding gateway + Docker or `ENGRAM_QDRANT_TEST_ADDR`. The `-run TestRetrievalEval` regex substring-matches the Phase-14 differ test `TestRetrievalEval_AsymmetryDiffer`, so this target now runs BOTH the recall case and the differ assertion (review finding A). |
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
| 14-01-T1/T2 | 14-01 | 1 | REQ-embed-gemini-direct | T-14-01 (info-disclosure: synthetic probe, no keys) | Skip-gated `TestRetrievalEval_AsymmetryDiffer` compiles + zero-cost skips by default; symmetric-config guard skips inequality; live PASS = query vec ≠ document vec at `len==dim` (instruction-prefix takes effect; Pitfall 12 guard) | unit compile/skip (default gate) → integration live (14-03) | `go test ./internal/retrievaleval/ -run TestRetrievalEval_AsymmetryDiffer -count=1` (skips clean); live: `ENGRAM_RETRIEVAL_EVAL=1 … go test … -run TestRetrievalEval_AsymmetryDiffer -v` (also reachable via `task eval:retrieval`) | ❌ W0 — new differ-case in `fixtures.go`/`retrieval_eval_test.go` (built by 14-01) | ⬜ pending |
| 14-02-T1/T2/T3 | 14-02 | 1 | REQ-embed-model-docs | T-14-01 | Recipes render/lint + cross-link `guides/reindex`; instruction-prefix Gemini recipe; placeholder secrets only | positive grep + lint (docs correctness manual) | `grep -q 'v1beta/openai' … && grep -q '/guides/reindex/' …` + `task lint` + `helm lint charts/engram` | ❌ W0 — new `embedding-models.md` (built by 14-02) | ⬜ pending |
| 14-03-T1/T2 | 14-03 | 2 | REQ-embed-gemini-direct, REQ-embed-prod-parity-eval | T-14-01 | Live differ PASS (3072 dim) + `gh261Case` Record T within default k=8 on qwen3@4096, captured in redacted structured committed evidence | integration (live gateway; human-verify checkpoint) → fail-closed evidence grep | `task eval:retrieval` (both recipe envs, manual/local D-06); evidence (fail-closed): `grep -q 'recall@8=1.00' … && [ $(grep -c 'hard rank bar: PASS' …) -ge 2 ] && grep -Eq 'asymmetry differ PASS\|PASS: TestRetrievalEval_AsymmetryDiffer' … && grep -q '261' … && ! grep -Eq 'recall@8=0\|rank bar FAILED\|--- FAIL' …` | ❌ W0 — new `14-EVAL-EVIDENCE.md` (built by 14-03) | ⬜ pending |

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

- [x] All tasks have `<automated>` verify or Wave 0 dependencies (14-03 T1 is a `checkpoint:human-verify`, Nyquist-exempt; its live evals are documented Wave 0 manual deps captured by 14-03 T2's evidence grep)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (differ-case fixture → 14-01; `embedding-models.md` → 14-02; `14-EVAL-EVIDENCE.md` → 14-03)
- [x] No watch-mode flags
- [x] Feedback latency < default-gate seconds
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** planned 2026-07-11 (task IDs mapped to 14-01/14-02/14-03)
