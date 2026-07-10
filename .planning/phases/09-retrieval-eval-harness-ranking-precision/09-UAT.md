---
status: complete
phase: 09-retrieval-eval-harness-ranking-precision
source: [09-VERIFICATION.md]
started: 2026-07-10
updated: 2026-07-10
---

## Current Test

[testing complete]

## Tests

### 1. Confirm SC3 on the production embedder (qwen3-embedding-8b @4096 + PR#262 query instruction)
expected: |
  `ENGRAM_RETRIEVAL_EVAL=1 task eval:retrieval` passes the hard rank bar on prod-parity config
  (Record T within default k for Query A/B; recall@8=1.00, MRR=1.000).
result: pass
source: accepted-with-note
note: |
  Accepted by user 2026-07-10 on the strength of the accumulated evidence, with the exact
  prod-model confirmation deferred to issue #334 (blocked by #333). Evidence: (a) automated
  verification 3/3 PASS against the codebase; (b) D-06 rerank MECHANISM proven embedder-
  independently by internal/server/connectapi_test.go TestRerankParityMCPAndConnect (tied-score
  corpus → only the lexical rerank can promote Record T); (c) end-to-end #261 rank bar PASS on
  two real embedders — gemini-embedding-2 @3072 and gemini-embedding-001 @3072, both recall@8=1.00,
  MRR=1.000, Record T rank 1/8 for Query A and Query B; (d) prod embedder qwen3-embedding-8b @4096
  confirmed reachable (HTTP 200, dim 4096) but browning out at ~36s/call, over the hardcoded 30s
  embed-client timeout — so the clean prod-parity run is blocked by #333, not by any code defect.
  Follow-up #334 (milestone v0.9.x) captures the exact qwen3 recall@k/MRR/rank number once #333
  lands or OpenRouter latency recovers.

## Summary

total: 1
passed: 1
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none]
