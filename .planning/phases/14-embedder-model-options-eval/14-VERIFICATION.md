---
phase: 14-embedder-model-options-eval
verified: 2026-07-11T00:00:00Z
status: passed
score: 3/3 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification: null
---

# Phase 14: Embedder Model Options & Eval Verification Report

**Phase Goal:** Operators can point engram at Gemini's embeddings API and trust that the documented model recipes (OpenRouter/Gemini/OpenAI/local) actually deliver working asymmetric query/document embeddings, with the last v0.9.x eval follow-up closed.
**Verified:** 2026-07-11
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | engram can embed queries/documents against Gemini's API and the shipped config's query vs document vectors actually DIFFER (Pitfall-12 gate), proven by a live `task eval:retrieval` differ run | ✓ VERIFIED | `TestRetrievalEval_AsymmetryDiffer` exists in `internal/retrievaleval/retrieval_eval_test.go:223-278`, asserts dimension contract (`len(queryVec)==len(documentVec)==dim`) then `reflect.DeepEqual` inequality via hard `t.Fatal`, reachable via `task eval:retrieval`'s `-run TestRetrievalEval` (Taskfile.yaml:59) substring-match — confirmed by inspection. CI-safe skip confirmed live: `go test ./internal/retrievaleval/ -run TestRetrievalEval_AsymmetryDiffer -count=1 -v` → `--- SKIP` with `ENGRAM_RETRIEVAL_EVAL` unset. The live-gateway run (dim=3072, PASS) is recorded in the committed fail-closed evidence artifact `.planning/phases/14-embedder-model-options-eval/14-EVAL-EVIDENCE.md` (commit `0a1372f0`) per the design decision (D-07) that eval is local/manual, not CI — this environment's keys are not assumed and re-running was correctly out of scope for this verification. |
| 2 | The #261 regression fixture re-confirms recall@8 parity on the prod-parity qwen3-embedding-8b@4096 config, closing #261/#334 | ✓ VERIFIED | `gh261Case` fixture (`internal/retrievaleval/fixtures.go:61-85`) and `TestRetrievalEval`'s recall@k/MRR assertions (`internal/retrievaleval/retrieval_eval_test.go`) are unmodified and intact from Phase 9/13. Committed evidence records a live run against the OpenRouter `qwen/qwen3-embedding-8b`@4096 recipe: both `hard rank bar: PASS` lines, exact `recall@8=1.00 MRR=1.000`, `--- PASS: TestRetrievalEval/gh261-sticky-neighbor-crowding`, `exit status: 0` — matching the fail-closed template required by 14-03's plan. |
| 3 | docs-site + Helm values.yaml document each supported embedding model (OpenRouter/Gemini/OpenAI/local TEI-Ollama-vLLM) pairing base URL + model + dim + query instruction, with every model/dim change calling out `engram reindex` | ✓ VERIFIED | `docs-site/src/content/docs/guides/embedding-models.md` (157 lines) exists with a full comparison table (6 rows: OpenRouter, Gemini, OpenAI, TEI, Ollama, vLLM — each pairing base URL + model id + dim + query mechanism + reindex note) and concrete, complete per-provider copy-paste env blocks (all API keys as shell-safe quoted placeholders `'replace-with-your-key'`, local providers with empty-quoted keys and exact server-side model ids). `charts/engram/values.yaml` (lines 44-69) keeps the neutral uncommented `ollama/bge-m3`@1024 default and adds commented OpenRouter/Gemini/OpenAI recipe blocks referencing `memory.openai.apiKeySecret` (no inline keys) and the `guides/reindex` route. `embedding-instructions.md` cross-links `/guides/embedding-models/` and its stale Gemini `task_type` row is corrected to point at the instruction-prefix mechanism (verified: Cohere/Voyage/Jina rows unchanged). Bidirectional cross-link confirmed both directions. |

**Score:** 3/3 truths verified (0 present-but-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/retrievaleval/fixtures.go` | `differProbe` synthetic string constant | ✓ VERIFIED | Line 108: `const differProbe = "Run \`task lint\` before every commit; golangci-lint's config lives in .golangci.yaml."` — synthetic, secret-free, documented (lines 87-107). |
| `internal/retrievaleval/retrieval_eval_test.go` | `TestRetrievalEval_AsymmetryDiffer` | ✓ VERIFIED | Lines 223-278: exact name, `ENGRAM_RETRIEVAL_EVAL` gate first statement, symmetric-config skip guard, dimension-contract check, `t.Fatal` on equality, `t.Logf` PASS token. `go build`/`go vet` exit 0. |
| `docs-site/src/content/docs/guides/embedding-models.md` | New recipes page | ✓ VERIFIED | 157 lines, Starlight frontmatter, no SPDX (license-exempt path confirmed), comparison table + 6 concrete provider recipes + eval-run section. |
| `docs-site/src/content/docs/guides/embedding-instructions.md` | Cross-link + corrected Gemini row | ✓ VERIFIED | Lines 15, 113-120: cross-link to embedding-models present; stale `task_type` Gemini/Vertex row replaced with instruction-prefix guidance; Cohere/Voyage/Jina rows unchanged. |
| `charts/engram/values.yaml` | Commented recipe blocks | ✓ VERIFIED | Lines 44-69: OpenRouter/Gemini/OpenAI commented blocks present; neutral default (lines 22-23) untouched; `helm lint charts/engram` and `yamlfmt -lint` both exit 0. |
| `.planning/phases/14-embedder-model-options-eval/14-EVAL-EVIDENCE.md` | Committed live-eval evidence | ✓ VERIFIED | Present, committed at `0a1372f0`, contains confirmed model-id (`gemini-embedding-2`, 3072-dim), sanitized commands, differ PASS + `--- PASS` lines, both `hard rank bar: PASS` lines, exact `recall@8=1.00`, 2x `exit status: 0`, issue-closure handoff (#261/#334/#331). `rumdl check` on this file: clean. No API keys or raw terminal dumps present (redaction confirmed by inspection). |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `task eval:retrieval` (Taskfile.yaml:59) | `TestRetrievalEval_AsymmetryDiffer` | `-run TestRetrievalEval` substring-match | ✓ WIRED | Confirmed by reading Taskfile.yaml and the test name; Go's `-run` is an unanchored regex so `TestRetrievalEval` matches `TestRetrievalEval_AsymmetryDiffer`. No Taskfile change was needed or made. |
| Differ test | `server.StoreAndEmbedderFromEnvNoEnsure` → `em.EmbedQuery`/`em.Embed` | Same prod-parity path as `gh261Case` | ✓ WIRED | Line 246: identical builder call to `TestRetrievalEval` (line 62). No bespoke embed shortcut. |
| `embedding-models.md` | `embedding-instructions.md` | Cross-link | ✓ WIRED | Bidirectional: line 17 (models→instructions) and line 15 (instructions→models). |
| Every recipe | `guides/reindex` | Markdown link / Helm comment reference | ✓ WIRED | 8+ `/guides/reindex/` links in embedding-models.md; `values.yaml` line 43 references `guides/reindex` route in comment. |
| `14-EVAL-EVIDENCE.md` model-id | `embedding-models.md` + `values.yaml` Gemini recipe | Reconciliation (single source of truth) | ✓ WIRED | Confirmed `gemini-embedding-2` matches unchanged; evidence explicitly states "confirmed unchanged," and both files retain `gemini-embedding-2`@3072 with instruction-prefix fields. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Differ test skips cleanly with zero cost when eval flag unset | `go test ./internal/retrievaleval/ -run TestRetrievalEval_AsymmetryDiffer -count=1 -v` | `--- SKIP: TestRetrievalEval_AsymmetryDiffer (0.00s)`, `PASS`, `ok` | ✓ PASS |
| Full build/vet clean | `go build ./...` and `go vet ./internal/retrievaleval/...` | exit 0, no output | ✓ PASS |
| Scoped Go lint clean | `task lint:go` | `0 issues.` | ✓ PASS |
| Full test suite green (single run) | `go test ./...` | all packages `ok` (including `internal/retrievaleval`) | ✓ PASS |
| License headers clean | `task license:check` | `708 files, valid: 168, invalid: 0` | ✓ PASS |
| Helm chart valid | `helm lint charts/engram` | `1 chart(s) linted, 0 chart(s) failed` | ✓ PASS |
| YAML formatting clean | `yamlfmt -lint charts/engram/values.yaml` | exit 0, no diffs | ✓ PASS |
| Evidence file markdown clean | `rumdl check .planning/phases/14-embedder-model-options-eval/14-EVAL-EVIDENCE.md` | `Success: No issues found` | ✓ PASS |

Live Gemini/OpenRouter gateway runs (the actual differ-PASS and recall@8=1.00 assertions) were NOT re-executed in this verification per explicit instruction (D-07: eval is local/manual; committed evidence at `14-EVAL-EVIDENCE.md` is the intended proof artifact, not a re-runnable CI check). The evidence artifact's exact PASS tokens, dimension, recall figure, and exit statuses were confirmed present by direct file inspection and match the fail-closed template plan 14-03 specified.

### Probe Execution

No `scripts/*/tests/probe-*.sh` probes declared or found for this phase — N/A.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| REQ-embed-gemini-direct | 14-01, 14-03 | Gemini OpenAI-compat embedding with task_type/dimension behavior confirmed by eval-harness run | ✓ SATISFIED | Differ-case code (14-01) + live PASS evidence (14-03), truth #1 above. |
| REQ-embed-prod-parity-eval | 14-03 | #261 rank bar re-confirmed on qwen3-embedding-8b@4096 | ✓ SATISFIED | Live recall@8=1.00 evidence (14-03), truth #2 above. |
| REQ-embed-model-docs | 14-02 | docs-site + Helm values.yaml document supported embedding models with reindex callouts | ✓ SATISFIED | embedding-models.md + values.yaml + cross-links, truth #3 above. |

No orphaned requirements — REQUIREMENTS.md maps exactly these three IDs to Phase 14, and all three appear in a plan's `requirements` frontmatter.

### Anti-Patterns Found

None. Scanned all phase-modified files (`fixtures.go`, `retrieval_eval_test.go`, `embedding-models.md`, `embedding-instructions.md`, `values.yaml`, `14-EVAL-EVIDENCE.md`) for TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER/"coming soon"/"not yet implemented" markers. Two incidental matches on the word "placeholder" are legitimate documentation prose describing the shell-safe key convention, not debt markers.

### Scoped-Lint Note (informational, not a gap)

Per explicit phase design (review B2, all three plans), the whole-tree `task lint` was intentionally not run as a gate — it fails on pre-existing `.planning/` rumdl noise scoped to Phase 21's REQ-lint-planning-exclude. The narrower, phase-appropriate gates (`task lint:go`, `helm lint`, `yamlfmt -lint`, `rumdl check` on the one new evidence file, `task license:check`) were run directly by this verification and are all clean. This is consistent with the phase's documented scope and is not treated as a gap.

### Human Verification Required

None. All three success criteria are verifiable from committed codebase artifacts: the differ-case code and its CI-safe skip behavior were directly exercised; the live-gateway PASS evidence is a committed, auditable artifact per the phase's explicit design (D-06/D-07), not a claim requiring re-execution.

### Gaps Summary

No gaps found. All three ROADMAP success criteria are met:
1. The Pitfall-12 differ-case exists, is reachable via the documented `task eval:retrieval` command, is CI-safe (skips with zero cost by default), and its live-gateway PASS (dim=3072) is recorded in committed evidence.
2. The #261 recall@8 parity fixture is unmodified and intact; a live re-confirmation run against the qwen3-embedding-8b@4096 config is recorded in committed evidence with exact `recall@8=1.00` and both hard-rank-bar PASS lines.
3. docs-site and Helm values.yaml fully document all four provider families (OpenRouter/Gemini/OpenAI/local TEI-Ollama-vLLM) with concrete base URL + model + dim + query instruction pairings, and every recipe calls out `engram reindex` with a working cross-link/reference.

---

_Verified: 2026-07-11_
_Verifier: Claude (gsd-verifier)_
