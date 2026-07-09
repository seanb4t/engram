---
title: "Generic param-map passthrough over embedder profiles for asymmetric/cloud embedders"
---

<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-zyhq; do not edit manually; use `/adr update engram-zyhq` -->

**Date:** 2026-07-01
**Status:** Accepted
**Decision:** engram-zyhq
**Deciders:** sean

## Context

Cloud/gateway embedders (OpenRouter, Cohere, Voyage, Jina, Google) signal query-vs-document retrieval intent via a native request-body field, but the field name and enum values differ per provider (input_type / task / task_type). engram must choose how to expose this to operators and how future providers get supported. engram calls an OpenAI-compatible /v1/embeddings endpoint whose gateways (OpenRouter natively, LiteLLM via map_openai_params) forward such fields to the provider.

## Decision

engram exposes query/document embedding params as raw, provider-agnostic JSON objects (ENGRAM_EMBED_QUERY_PARAMS / ENGRAM_EMBED_DOCUMENT_PARAMS) merged into the /v1/embeddings request body, rather than a curated single field or a maintained per-provider profile registry. The reserved keys model and input are applied last and can never be overridden by operator params.

## Rationale

- No single field name (input_type / task / task_type) covers all target providers, so a hardcoded field cannot generalize.
- A profile registry is an ongoing maintenance burden for something structurally simple (inserting a field into the request body).
- Keeps the reserved-key guard (model/input applied last, never overridable) as the only engram-side contract, leaving provider knowledge in operator config + docs.

## Alternatives Considered

- **Provider-agnostic JSON param map (chosen):** field-name-agnostic; supports current and future providers with zero engram-side provider logic.
- **Focused input_type toggle (rejected):** simplest surface, but hardcodes one field name/enum and cannot express Google task_type or any future provider field.
- **Embedder profiles / named registry (rejected):** nicer operator UX, but requires engram to maintain a provider registry indefinitely; over-engineered for inserting a body field.

## Consequences

- Positive: extensible to any future gateway without an engram code change; small, stable public surface (embed.Client options + two env vars).
- Negative: no engram-side validation that a provider actually honors the configured field; operators must source correct per-provider values themselves.
- Neutral: the per-provider value burden lives in documentation (guides/embedding-instructions) rather than code.
