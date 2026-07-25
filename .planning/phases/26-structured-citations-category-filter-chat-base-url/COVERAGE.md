# API Coverage — OpenAI-compatible gateway (embeddings + chat)

> Full coverage by default. Opt-outs are explicit, reasoned decisions.

**Scope note.** The deterministic detector fires on this phase because the plans say "MCP",
"Connect", and "API" constantly. Those are engram's **own served protocols** (the MCP tool surface
and the Connect RPC surface), not external APIs engram consumes — so most of the signal is a false
positive. There is exactly one external API in this phase's blast radius: the **OpenAI-compatible
gateway** engram calls outbound for embeddings and for auto-summary chat completions. That
integration is pre-existing; Track C changes only which base URL the chat lane targets
(`ENGRAM_OPENAI_CHAT_BASE_URL`) and how the endpoint path is joined. No new external capability is
consumed.

The matrix below is nonetheless decided in full, from the full-coverage baseline, so the un-built
surface is a recorded decision rather than an invisible hole.

| capability | decision | reason |
|---|---|---|
| `POST /v1/embeddings` | INTEGRATE | |
| `POST /v1/chat/completions` (non-streaming) | INTEGRATE | |
| Bearer-token request auth | INTEGRATE | |
| Per-lane base URL override | INTEGRATE | this phase, Track C — `ENGRAM_OPENAI_CHAT_BASE_URL` |
| Provider-shape endpoint join (`/v1`, `/v1beta/openai`, bare host) | INTEGRATE | this phase, Track C — one shared `internal/openaiurl.Join` |
| Full-URL embeddings endpoint override | INTEGRATE | already shipped as `ENGRAM_OPENAI_EMBEDDINGS_URL` |
| Per-lane API key | OPT-OUT | not needed yet — the target deployment is a local embedder plus a hosted chat model, and local embedders ignore the Authorization header, so the shared key already works. Explicitly deferred in `26-CONTEXT.md`; becomes required when both lanes are hosted with different providers. |
| Per-lane request timeout | OPT-OUT | not needed yet — same per-lane-provider family as the key; explicitly deferred in `26-CONTEXT.md`. |
| Streaming chat completions (SSE) | OPT-OUT | not needed — a summary is one short non-interactive line generated off the write path by a background worker; there is no consumer that could use incremental tokens. |
| Tool / function calling | OPT-OUT | explicitly out of scope — the summarizer is a single-shot text compressor operating on fenced untrusted content, and granting it tool-call capability would widen the prompt-injection surface for no benefit. |
| Structured outputs / JSON mode | OPT-OUT | not needed yet — the summary contract is one plain line hard-capped at `ENGRAM_SUMMARY_MAX_CHARS`; a schema-constrained response would add provider-compatibility risk across the OpenAI-compatible gateways engram supports. |
| `GET /v1/models` (model discovery) | OPT-OUT | not needed — model ids are operator-configured (`ENGRAM_EMBED_MODEL`, `ENGRAM_SUMMARY_MODEL`) and are payload-stamped for reindex provenance; runtime discovery would make the stamped identity non-deterministic. |
| `POST /v1/completions` (legacy text) | OPT-OUT | explicitly out of scope — superseded by chat completions across every supported gateway. |
| `POST /v1/moderations` | OPT-OUT | not needed — engram stores explicit, user-blessed memories; there is no content-moderation gate in the product and adding one would contradict the correctable-recall design. |
| Reranking endpoints (provider-side rerank) | OPT-OUT | explicitly out of scope — reranking is deliberately in-process and dependency-free (`store.SearchReranked`, the stdlib lexical reranker chosen in v0.9.x); an outbound rerank call would egress query text on every recall. |
| Files / batches | OPT-OUT | not needed — engram has no batch or file-upload workflow; summaries are per-record and generated on demand. |
| Assistants / threads / responses runtime | OPT-OUT | explicitly out of scope — provider-specific agent runtimes; engram is the memory substrate those agents call, not a client of them. |
| Images / audio / speech | OPT-OUT | not needed — engram stores and recalls text memories only. |
| Fine-tuning | OPT-OUT | not needed — engram never trains or adapts a model. |
