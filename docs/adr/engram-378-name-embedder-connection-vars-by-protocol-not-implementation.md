<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-378; do not edit manually; use `/adr update engram-378` -->

# Name embedder connection vars by protocol, not implementation

**Date:** 2026-06-14
**Status:** Accepted
**Decision:** engram-378
**Deciders:** Sean Brandt

## Context

`MEM_LITELLM_URL` and `MEM_LITELLM_KEY` baked a specific proxy product (LiteLLM) into public env var names, even though the server only ever required an **OpenAI-compatible `/v1/embeddings` endpoint**. Ollama, vLLM, TEI, and OpenAI itself all speak the same wire protocol. The name confused readers of the code and docs and tied a generic project to one operator's homelab topology.

## Decision

Rename the embedder connection vars to `ENGRAM_OPENAI_BASE_URL` / `ENGRAM_OPENAI_API_KEY`, where "OpenAI" names the **wire protocol** (the OpenAI-compatible `/v1/embeddings` API), not the vendor. The embedder model/dimension vars keep the `ENGRAM_EMBED_` stem (`ENGRAM_EMBED_MODEL`, `ENGRAM_EMBED_DIM`).

## Rationale

- Mirrors the OpenAI SDK's own canonical env vars (`OPENAI_BASE_URL` / `OPENAI_API_KEY`), giving operators a familiar mental model; the SDK documents `base_url` as the mechanism for OpenAI-compatible APIs.
- "OpenAI" in the var name describes the protocol; the docs make explicit that any compatible backend (Ollama, vLLM, TEI, OpenAI, LiteLLM) is valid.
- Two clean families: `ENGRAM_OPENAI_*` is the protocol connection; `ENGRAM_EMBED_*` is engram's embedding choice. The OpenAI SDK has no env var for model, so grouping model under `OPENAI_` would invent a non-standard name.
- `embed.New`'s signature is already provider-neutral (`baseURL, apiKey, model`); only call sites and var names change.

## Alternatives Considered

- **Keep `MEM_LITELLM_URL` / `MEM_LITELLM_KEY`** — no migration cost, but leaks an operator-specific implementation detail into a generic project's public interface and confuses users not running LiteLLM. Rejected.
- **`ENGRAM_EMBED_URL` / `ENGRAM_EMBED_KEY` (fully generic stem)** — no vendor association at all, but does not align with the established OpenAI SDK canonical var names, sacrificing discoverability for users who already know `OPENAI_BASE_URL`. Rejected.

## Consequences

- **Positive:** the public interface no longer exposes an operator-specific implementation detail; any OpenAI-compatible backend is a first-class citizen without renaming; alignment with OpenAI SDK canonical vars aids discoverability.
- **Negative:** breaking rename for all existing deployments using `MEM_LITELLM_*` (mitigated by the legacy guard); the `OPENAI_` prefix may still suggest vendor lock-in to uninformed readers, so docs must clarify it names the protocol.
- **Neutral:** `embed.New`'s signature is unchanged; impact is limited to call sites and env var names.
