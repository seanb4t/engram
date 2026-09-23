# Spike Manifest

## Ideas

### jev-typed-decisions

Use TypeSafe's Jev (System One typed-decision model) behind a provider-neutral,
advisory-only decision interface in engram, for spine-curation verdicts, recall reranking,
and write-time hints. Context: `.planning/notes/jev-system-one-decisions.md`,
`.planning/seeds/typed-decision-provider.md`.

**Requirements:**

- Provider-neutral interface using System One vocabulary (state + Choice/Score/Noul →
  probabilities); Jev is one backend, a chat-LLM emulator another.
- Off by default; advisory only — verdicts are surfaced, never acted on (reranking allowed).
- Transport goes through `/api/alpha/decisions`; engram's chat client cannot reach Jev.
- The decision client gets its own base-URL setting; it must not assume the chat/embeddings
  gateway serves Decisions (LiteLLM does not, without a pass-through entry).
- Spike fixtures built from real memory content are gitignored (public repo); commit only
  code, viewers and aggregate results.

## Spikes

| # | Idea | Name | Type | Validates | Verdict | Tags |
|---|------|------|------|-----------|---------|------|
| 001 | jev-typed-decisions | jev-openrouter-transport | standard | Given the OpenRouter key, when a Decisions request is POSTed, then typed probabilities return with usable latency and known limits | VALIDATED | jev, openrouter, decisions-api, latency |
| 002 | jev-typed-decisions | jev-gateway-passthrough | standard | Given the LiteLLM gateway, when a Decisions request is sent through it, then it reaches Jev intact | VALIDATED | jev, litellm, gateway, pass-through |
| 003 | jev-typed-decisions | jev-consolidate-verdicts | standard | Given 39 labeled real spine pairs, when Jev classifies the relation, then accuracy and confidence are usable for advisory consolidate verdicts | VALIDATED | jev, spine-review, consolidate, calibration |
| 004 | jev-typed-decisions | jev-rerank-eval | comparison | Given the #261 corpus + paraphrase queries, when ranked by vector / lexical / Jev, then Jev's quality justifies its latency | VALIDATED | jev, rerank, retrieval-eval, gh-261 |
