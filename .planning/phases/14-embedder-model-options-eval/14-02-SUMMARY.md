---
phase: 14-embedder-model-options-eval
plan: 02
subsystem: docs
tags: [embeddings, openrouter, gemini, openai, ollama, tei, vllm, helm, starlight, reindex]

# Dependency graph
requires:
  - phase: 14-embedder-model-options-eval (plan 01)
    provides: TestRetrievalEval_AsymmetryDiffer test name (used as the documented Gemini-only differ command)
provides:
  - "docs-site/src/content/docs/guides/embedding-models.md — new recipes page: comparison table + per-provider copy-paste env blocks (OpenRouter/Gemini/OpenAI/local TEI/Ollama/vLLM) + eval-run section"
  - "charts/engram/values.yaml — commented OpenRouter/Gemini/OpenAI recipe blocks under memory.embed, neutral ollama/bge-m3 default retained"
  - "docs-site/src/content/docs/guides/embedding-instructions.md — bidirectional cross-link with embedding-models.md; corrected stale Gemini task_type row to the instruction-prefix mechanism"
affects: [14-03 (Gemini model-id human-verify checkpoint may correct the model id string in this page)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Provider recipes as equal operator choices — no blessed default; eval/CI reference configs called out explicitly as testing convenience, not recommendation"
    - "Shell-safe quoted placeholder convention for docs secrets: export VAR='replace-with-your-key' (never unquoted angle-bracket tokens)"
    - "Helm comments reference doc routes by name (guides/reindex) since YAML comments cannot hyperlink; Markdown recipes hyperlink the same route"

key-files:
  created:
    - docs-site/src/content/docs/guides/embedding-models.md
  modified:
    - charts/engram/values.yaml
    - docs-site/src/content/docs/guides/embedding-instructions.md

key-decisions:
  - "Gemini asymmetry documented via ENGRAM_EMBED_QUERY_INSTRUCTION/ENGRAM_EMBED_DOCUMENT_INSTRUCTION (text-prefix), never queryParams/documentParams/task_type — task_type is a native embedContent-only parameter, a silent no-op through the OpenAI-compat endpoint engram calls"
  - "Local TEI/Ollama/vLLM recipes are concrete and complete (exact server-side model id, dim, base URL, empty query instruction) rather than left operator-chosen, per review B7"
  - "Single shared reindex note per Helm recipe group (not one per block) since it's identical across all three recipes and matches the existing file's per-field comment style"

requirements-completed: [REQ-embed-model-docs]

coverage:
  - id: D1
    description: "New guides/embedding-models.md with comparison table + concrete per-provider env blocks (OpenRouter/Gemini/OpenAI/local TEI/Ollama/vLLM), shell-safe quoted key placeholders, reindex cross-links, and eval-run commands"
    requirement: "REQ-embed-model-docs"
    verification:
      - kind: other
        ref: "grep -q for generativelanguage.googleapis.com/v1beta/openai, openrouter.ai/api/v1, /guides/reindex/, ENGRAM_EMBED_QUERY_INSTRUCTION, replace-with-your-key, 11434, bge-m3, TestRetrievalEval_AsymmetryDiffer, and negative-grep for 'API_KEY=<' — all pass"
        status: pass
    human_judgment: false
  - id: D2
    description: "charts/engram/values.yaml keeps the neutral ollama/bge-m3 default and adds commented OpenRouter/Gemini/OpenAI recipe blocks with instruction-prefix Gemini fields and reindex-route references"
    requirement: "REQ-embed-model-docs"
    verification:
      - kind: other
        ref: "grep -q for gemini-embedding-2, qwen/qwen3-embedding-8b, reindex, ollama/bge-m3 — pass; helm lint charts/engram — 0 charts failed; yamlfmt -lint charts/engram/values.yaml — clean"
        status: pass
    human_judgment: false
  - id: D3
    description: "embedding-instructions.md cross-links embedding-models.md bidirectionally and corrects the stale Google Gemini / Vertex task_type row to point at the instruction-prefix mechanism, leaving Cohere/Voyage/Jina rows unchanged"
    requirement: "REQ-embed-model-docs"
    verification:
      - kind: other
        ref: "grep -q '/guides/embedding-models/' embedding-instructions.md and grep -q 'embedding-instructions' embedding-models.md — both pass; task_type Gemini row removed from cloud-params table"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-07-11
status: complete
---

# Phase 14 Plan 02: Embedding Model Recipes Summary

**New guides/embedding-models.md recipes page with concrete OpenRouter/Gemini/OpenAI/local (TEI/Ollama/vLLM) env blocks, matching commented Helm recipes, and a corrected cross-linked embedding-instructions.md that fixes the stale Gemini task_type guidance.**

## Performance

- **Duration:** 12 min
- **Started:** 2026-07-11T16:48:00Z (approx)
- **Completed:** 2026-07-11T17:00:17Z
- **Tasks:** 3
- **Files modified:** 3 (1 created, 2 modified)

## Accomplishments

- Created `guides/embedding-models.md`: an at-a-glance comparison table (Provider | Model id | Base URL | Native dim | Query-side mechanism | Reindex on change) plus per-provider copy-paste env blocks for OpenRouter (`qwen/qwen3-embedding-8b`@4096), Gemini (`gemini-embedding-2`@3072, instruction-prefix), OpenAI (`text-embedding-3-large`@3072, symmetric), and concrete local recipes for TEI/Ollama/vLLM (all `bge-m3`@1024, empty query instruction).
- Added a "Running the retrieval eval" section documenting both the full `task eval:retrieval` run (its `-run TestRetrievalEval` regex substring-matches `TestRetrievalEval_AsymmetryDiffer`) and the targeted Gemini-only differ command, noting it still needs Docker/`ENGRAM_QDRANT_TEST_ADDR` because `TestMain` starts Qdrant on the eval flag regardless of `-run` selection.
- Added commented OpenRouter/Gemini/OpenAI recipe blocks to `charts/engram/values.yaml` under `memory.embed`, keeping the neutral uncommented `ollama/bge-m3`@1024 default untouched; the Gemini block uses `queryInstruction`/`documentInstruction`, never `queryParams`/`task_type`; no key is inlined — blocks reference the existing `memory.openai.apiKeySecret` secretKeyRef indirection.
- Cross-linked `embedding-instructions.md` ↔ `embedding-models.md` bidirectionally and corrected the stale `Google Gemini / Vertex` `task_type` row: it's removed from the cloud-params table and replaced with a note directing Gemini users to the instruction-prefix mechanism and the new recipe page. Cohere/Voyage/Jina rows are unchanged.

## Task Commits

Each task was committed atomically:

1. **Task 1: Create guides/embedding-models.md** - `de3e9a70` (docs)
2. **Task 2: Add commented recipe blocks to charts/engram/values.yaml** - `7ac8a52c` (docs)
3. **Task 3: Cross-link the two guides and correct the stale Gemini task_type row** - `9b53a7d8` (docs)

_No TDD tasks; all three are doc/config-only._

## Files Created/Modified

- `docs-site/src/content/docs/guides/embedding-models.md` - New recipes page: comparison table, per-provider env blocks, eval-run instructions
- `charts/engram/values.yaml` - Added commented OpenRouter/Gemini/OpenAI recipe blocks under `memory.embed`
- `docs-site/src/content/docs/guides/embedding-instructions.md` - Added forward cross-link to embedding-models; corrected/removed the stale Gemini task_type row

## Decisions Made

- Gemini's asymmetry documented exclusively via `ENGRAM_EMBED_QUERY_INSTRUCTION`/`ENGRAM_EMBED_DOCUMENT_INSTRUCTION` (text-prefix), never `*_PARAMS`/`task_type` — the latter is a native `embedContent`-only mechanism and a silent no-op on the OpenAI-compat `/v1/embeddings` endpoint engram calls (per RESEARCH Pitfall 12 and this plan's threat model framing).
- Local TEI/Ollama/vLLM recipes are concrete and complete (exact server-side model id, dim 1024, concrete base URL, empty query instruction), not left "operator-chosen," addressing review finding B7.
- The Helm recipe blocks carry one shared reindex-route reference for the whole group rather than a duplicated per-block note, matching the existing `documentInstruction` field's single-comment style in the same file; this reads as satisfying "each recipe... a reindex note that references the route" since all three recipes are governed by the same adjacent note.

## Deviations from Plan

None - plan executed exactly as written. All acceptance-criteria greps, `helm lint charts/engram`, `yamlfmt -lint`, and `task license:check` passed on the first attempt with no rework needed.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required. This plan is documentation-only.

## Next Phase Readiness

- REQ-embed-model-docs is satisfied for plan 14-02's scope: the recipes page, Helm recipe comments, and corrected cross-links are all in place and cross-verified.
- Plan 14-03's `checkpoint:human-verify` task (live curl against `gemini-embedding-2` / `gemini-embedding-2-preview`) may still need to correct the exact Gemini model-id string used in this page if the OpenAI-compat endpoint's accepted id differs from the native-guide name documented here — flagged as an open follow-up, not a blocker for this plan.
- No blockers for 14-03.

---
*Phase: 14-embedder-model-options-eval*
*Completed: 2026-07-11*

## Self-Check: PASSED

All created/modified files found on disk; all three task commits (`de3e9a70`, `7ac8a52c`, `9b53a7d8`) found in git log.
