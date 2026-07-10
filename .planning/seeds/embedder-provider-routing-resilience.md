---
title: Embedder resilience — OpenRouter provider routing / fallback (or multi-endpoint failover)
trigger_condition: >-
  When embedder brownouts/timeouts recur, when hardening recall availability is
  scoped, or when Phase 10 (#305) touches embedder request params.
planted_date: 2026-07-09
tags: [embedding, reliability, openrouter, failover, phase-10, internal-embed]
---

# Embedder resilience — provider routing / fallback

## The idea

engram's embedder is a plain OpenAI-compatible client. When the configured model
is served through OpenRouter by budget upstreams, those upstreams overload
(`529 Provider Overloaded`) and recall degrades or fails. OpenRouter already
supports a remedy engram can't currently reach: a `provider` routing block in the
request body —

```json
{
  "model": "qwen/qwen3-embedding-8b",
  "input": "...",
  "provider": { "order": ["..."], "allow_fallbacks": true, "data_collection": "deny" }
}
```

## Possible shapes (decide when triggered)

1. **Pass-through extra body params.** Let `internal/embed` inject configurable
   extra JSON fields (`provider`, and for Google-direct `task_type`) via an
   `ENGRAM_EMBED_*` knob. Cheapest; solves both OpenRouter fallbacks and the
   Phase 10 (#305) asymmetric `task_type` need with one mechanism.
2. **Multi-endpoint failover.** Primary + fallback embedder endpoints in config;
   engram retries the next on 429/529/timeout. Provider-agnostic (works for
   local + hosted), but more code + more failure semantics to define.
3. **Do nothing in engram; document routing at the gateway.** Rely on
   direct-to-provider (e.g. Gemini→Google) for prod stability and treat
   OpenRouter as eval-only. Zero code; loses cheap-model resilience.

## Why it's a seed, not a todo

Not needed if the near-term decision is "prod embedder = direct-to-provider
(Gemini/Google)." Becomes worth doing if we want to keep using cheap OpenRouter
models in prod, or when Phase 10 makes us touch embedder request params anyway —
at which point option 1 likely rides along for near-zero marginal cost.

## Links

- Context: `.planning/notes/embedding-model-options.md`
- Related: Phase 10 / issue #305 (asymmetric embedder params — the `WithQuery/DocumentParams` plumbing already shipped in `internal/embed/embed.go`)
- **Filed: #331** (support talking to the Gemini API directly — verify OpenAI-compat path first, native transport only if lossy) — the direct-to-provider slice of this seed
- Verify first: `internal/embed/` already allows extra request-body fields via `WithQueryParams`/`WithDocumentParams`; open question is whether Gemini's OpenAI-compat layer honors `output_dimensionality`/`task_type` end-to-end.
