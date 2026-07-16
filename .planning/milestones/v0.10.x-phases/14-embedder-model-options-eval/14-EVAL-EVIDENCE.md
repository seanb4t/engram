# Phase 14 Plan 03 — Live Eval Evidence

Live, operator-run evidence closing success criteria #1 (Gemini differ-assertion
PASS) and #2 (gh261 recall@8 PASS on qwen3-embedding-8b@4096), satisfying
REQ-embed-gemini-direct (#331) and REQ-embed-prod-parity-eval (#334), and
closing #261.

**Redaction note (T-14-01):** this file contains no API keys, no
`Authorization: Bearer` values, and no raw terminal dumps — only sanitized
commands (`$ENGRAM_OPENAI_API_KEY` placeholder), the (public) model-id/dim,
recorded exit status, and the specific success/metric lines transcribed
verbatim from the operator's run.

## Model-ID Confirmation (Pitfall 2, fail-closed)

- Run date: 2026-07-11
- Both `gemini-embedding-2` and `gemini-embedding-2-preview` returned HTTP 200
  against `https://generativelanguage.googleapis.com/v1beta/openai/embeddings`,
  each with an embedding length of exactly 3072.
- Selected GA id: `gemini-embedding-2`.
- Reconciliation: this **matches** the shipped 14-02 recipe (`gemini-embedding-2`
  @3072) in `docs-site/src/content/docs/guides/embedding-models.md` and
  `charts/engram/values.yaml` — **confirmed unchanged**. Neither file was
  edited by this plan.

## Run 1 — Gemini Differ Eval (success criterion #1)

- Run date: 2026-07-11
- Provider / model: Gemini `gemini-embedding-2` (OpenAI-compat), native dim 3072
- Observed vector dimensions: 3072

Sanitized command:

```sh
ENGRAM_OPENAI_API_KEY=$ENGRAM_OPENAI_API_KEY \
ENGRAM_OPENAI_BASE_URL=https://generativelanguage.googleapis.com/v1beta/openai \
ENGRAM_EMBED_MODEL=gemini-embedding-2 ENGRAM_EMBED_DIM=3072 \
ENGRAM_EMBED_QUERY_INSTRUCTION='task: search result | query: {query}' \
ENGRAM_EMBED_DOCUMENT_INSTRUCTION='title: none | text: {document}' \
ENGRAM_RETRIEVAL_EVAL=1 \
go test ./internal/retrievaleval/ -run TestRetrievalEval_AsymmetryDiffer -v -count=1
```

Success lines (transcribed verbatim):

```text
asymmetry differ PASS: query vector != document vector (dim=3072) — instruction-prefix took effect
--- PASS: TestRetrievalEval_AsymmetryDiffer (1.21s)
```

Recorded: exit status: 0

## Run 2 — qwen3-embedding-8b@4096 Recall Eval (success criterion #2, #261 parity)

- Run date: 2026-07-11
- Provider / model: OpenRouter `qwen/qwen3-embedding-8b` @4096

Sanitized command:

```sh
ENGRAM_OPENAI_API_KEY=$ENGRAM_OPENAI_API_KEY \
ENGRAM_OPENAI_BASE_URL=https://openrouter.ai/api/v1 \
ENGRAM_EMBED_MODEL=qwen/qwen3-embedding-8b ENGRAM_EMBED_DIM=4096 \
ENGRAM_EMBED_QUERY_INSTRUCTION='Given a web search query, retrieve relevant passages that answer the query' \
ENGRAM_RETRIEVAL_EVAL=1 \
task eval:retrieval
```

Success lines (transcribed verbatim):

```text
gh261-sticky-neighbor-crowding/query-a: rank=1/8 (hard rank bar: PASS)
gh261-sticky-neighbor-crowding/query-b: rank=1/8 (hard rank bar: PASS)
gh261-sticky-neighbor-crowding: recall@8=1.00 MRR=1.000
--- PASS: TestRetrievalEval/gh261-sticky-neighbor-crowding (10.42s)
```

Recorded: exit status: 0

## Issue Closure Handoff (review B10)

These two live runs satisfy the #261 / #334 / #331 acceptance gates. This
evidence file records the proof; it performs no closure itself. The committing
PR **MUST** use closing keywords (`Closes #261`, `Closes #334`, `Closes #331`)
so GitHub closes them on merge.
