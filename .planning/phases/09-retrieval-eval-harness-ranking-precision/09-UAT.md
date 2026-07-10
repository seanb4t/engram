---
status: testing
phase: 09-retrieval-eval-harness-ranking-precision
source: [09-VERIFICATION.md]
started: 2026-07-10
updated: 2026-07-10
---

## Current Test

number: 1
name: Confirm SC3 (phrasing-sensitive misses eliminated) on the ACTUAL production embedder
expected: |
  With prod-parity embedder config — ENGRAM_EMBED_MODEL=qwen3-embedding-8b, ENGRAM_EMBED_DIM=4096,
  ENGRAM_EMBED_QUERY_INSTRUCTION set to the PR #262 query-side instruction, against a reachable
  (non-brownout) OpenAI-compatible gateway + Docker — running `ENGRAM_RETRIEVAL_EVAL=1 task eval:retrieval`
  passes the hard #261 rank bar: Record T surfaces within default k for BOTH Query A and Query B,
  recall@8 = 1.00, MRR = 1.000 (matching the gemini-embedding-2 substitute run, which already passed).
awaiting: user response

## Tests

### 1. Confirm SC3 on the production embedder (qwen3-embedding-8b @4096 + PR#262 query instruction)
expected: |
  `ENGRAM_RETRIEVAL_EVAL=1 task eval:retrieval` passes the hard rank bar on prod-parity config
  (Record T within default k for Query A/B; recall@8=1.00, MRR=1.000).
  Context: the automated verification is 3/3 PASS and the D-06 rerank MECHANISM is proven
  embedder-independently by internal/server/connectapi_test.go TestRerankParityMCPAndConnect
  (tied-score corpus). The live end-to-end run recorded in 09-03-SUMMARY.md used gemini-embedding-2
  @3072 as a substitute because qwen3-via-OpenRouter was browning out (>30s/call vs the hardcoded
  30s embed-client timeout) and ENGRAM_EMBED_QUERY_INSTRUCTION was unset. This item confirms the
  bar on the exact prod embedder once a reachable gateway is available.
result: [pending]

## Summary

total: 1
passed: 0
issues: 0
pending: 1
skipped: 0
blocked: 0

## Gaps
