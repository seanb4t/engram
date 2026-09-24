# Phase 2 — Live Decisions check (DEC-03)

Run 2026-09-23 by the orchestrator via `TestJevLive` (`ENGRAM_DECISIONS_LIVE=1`, same as `task eval:decisions`). No key material recorded.

| Base URL | Key source | Result | Model snapshot | Latency | Tokens in/out | Cost (USD) |
|---|---|---|---|---|---|---|
| `https://openrouter.ai/api` | local OpenRouter key (`ENGRAM_OPENAI_API_KEY` fallback, D-03) | PASS | `typesafe/jev-1.13-20260917` | 421 ms | 453/70 | 1.9026e-05 |
| `https://llm.fzymgc.house/openrouter` | local OpenRouter key | 401 → `ErrDecisionAuth` (LiteLLM string-`code` dialect classified correctly; key not valid on LiteLLM) | — | — | — | — |
| `https://llm.fzymgc.house/openrouter` | deployed engram LiteLLM key (k8s `agent-memory/memory-mcp-litellm`, injected via `ENGRAM_DECISIONS_API_KEY`, never printed) | PASS | `typesafe/jev-1.13-20260917` | 470 ms | 453/70 | 1.9026e-05 |
