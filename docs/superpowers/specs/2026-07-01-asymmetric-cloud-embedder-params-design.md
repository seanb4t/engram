<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Design: Asymmetric / cloud embedder param passthrough (query vs document)

- **Design bead:** `engram-0qed`
- **Umbrella feature:** `engram-wd89.1` (follow-up to `engram-wd89` / GH #261, shipped in PR #262)
- **Status:** design

## Context

PR #262 gave engram a query-side text instruction (`ENGRAM_EMBED_QUERY_INSTRUCTION`)
so instruction-tuned, asymmetric embedders (Qwen3-Embedding, bge-\*-v1.5) rank
recall correctly. That knob is a **text-prefix** mechanism: it only edits the
`input` string of the `/v1/embeddings` request, and only on the query side.

Two model classes it does not serve:

1. **Cloud / gateway embedders** signal query-vs-document intent through a
   **native request field**, not text — and the field name differs by provider:
   OpenRouter / Cohere / Voyage use `input_type` (`search_query` /
   `search_document`); Jina uses `task` (`retrieval.query` /
   `retrieval.passage`); Google Gemini/Vertex uses `task_type`
   (`RETRIEVAL_QUERY` / `RETRIEVAL_DOCUMENT`). A text prefix cannot set these;
   Cohere v3 in particular **requires** `input_type`. Three distinct field
   names, which is exactly why the mechanism below is field-name-agnostic.
2. **Both-side prefix models** (intfloat/e5-\*, nomic-embed-text) need a prefix
   on the **document** side too (`passage:` / `search_document:`). A query-only
   prefix on E5 is worse than none.

Grounding (recorded on `engram-0qed`):

- OpenRouter `/v1/embeddings` exposes `input_type` as a first-class body field
  (plus `provider` routing, `dimensions`, `encoding_format`).
- LiteLLM forwards provider-specific embedding params (`input_type` /
  `task_type`) via per-provider mapping (`map_openai_params`, e.g. Vertex
  `task_type` → `taskType`), not a blanket raw passthrough of arbitrary fields.

So the gateways engram already targets **accept the retrieval-intent field**
(under each provider's own name) in the embeddings request body. engram does not
need provider-specific logic — it needs to place a configurable field in the
request body that differs for query vs document, plus a document-side text
prefix for the self-hosted both-side models.

## Goals

- Let operators embed queries and documents asymmetrically for cloud/gateway
  models via a **provider-agnostic** request-body parameter.
- Add a **document-side text instruction** mirroring the shipped query knob, for
  self-hosted both-side-prefix models (E5, nomic).
- Keep every knob **orthogonal and opt-in**; default behavior is byte-for-byte
  unchanged.
- Make the **reindex boundary explicit**: query-side changes are hot; any
  document-side change requires a reindex.

## Non-goals

- Provider auto-detection or named embedder "profiles" (rejected: a registry to
  maintain for what is "put a field in the body").
- Changing the query text-instruction contract from #262.
- Streaming, multimodal, or `provider`-routing convenience wrappers (operators
  can still pass `provider` via the param maps if they want).

## Design

### New surface on `embed.Client`

The client gains three inputs (the shipped `queryInstruction` is unchanged):

- `documentInstruction string` — doc-side text, applied by `Embed`.
- `queryParams map[string]any` — merged into the request body by `EmbedQuery`.
- `documentParams map[string]any` — merged into the request body by `Embed`.

Constructed via new options `WithDocumentInstruction`, `WithQueryParams`,
`WithDocumentParams`.

### Request building

Today the body is a fixed struct `embedReq{model, input}`. The build gains a
two-path shape:

- **Empty params (default path):** marshal the existing `embedReq{model, input}`
  struct unchanged — this preserves the exact current wire bytes, so default
  deployments see no change at all. Path selection is by parsed param **count**:
  zero params — unset **or** an explicit `{}` — takes this fast path; one or more
  params takes the map path.
- **Non-empty params:** build a `map[string]any`, merging the side-specific
  params (`queryParams` for `EmbedQuery`, `documentParams` for `Embed`) **first**,
  then applying `model` and `input` **last** so they are always authoritative and
  can never be overridden by a param key.

Note: Go's `encoding/json` marshals map keys in sorted order, so the non-empty
path emits keys sorted (`input` before `model`), not declaration order. This is
JSON-semantically identical — object key order is not significant — so tests on
this path assert the **decoded** request object, not raw bytes.

### Document instruction

`Embed` wraps the document text before embedding, mirroring `EmbedQuery` but
without the Qwen3 `Instruct:/Query:` template (documents never take that form):

- empty → raw (default).
- contains `{document}` → used as a literal template with the placeholder
  replaced (e.g. `passage: {document}`, `search_document: {document}`).
- otherwise → prepended as a prefix (`value + document`).

### Wiring

- `embedderFromConfig` passes the three new options from config.
- `store.Reindex` already embeds through the document `Embed` path, so it
  re-embeds with `documentInstruction` + `documentParams` automatically — no
  reindex-specific code.
- MCP/Connect search handlers are unchanged; they call `EmbedQuery`, which now
  carries `queryParams`.

### Configuration

New registry keys (all default empty = current behavior):

| Key | Env | Meaning |
| --- | --- | --- |
| `embed.query_params` | `ENGRAM_EMBED_QUERY_PARAMS` | JSON object merged into query embeds |
| `embed.document_params` | `ENGRAM_EMBED_DOCUMENT_PARAMS` | JSON object merged into document embeds |
| `embed.document_instruction` | `ENGRAM_EMBED_DOCUMENT_INSTRUCTION` | doc-side text prefix/template |

`EmbedConfig` gains `QueryParams string` / `DocumentParams string` (raw JSON
text) and `DocumentInstruction string`, following the string-typed convention
(cf. `Embed.Dim`): the JSON is stored verbatim and parsed in `Config.Validate()`,
never silently coerced by the koanf unmarshal.

Helm: `memory.embed.queryParams` / `documentParams` are JSON **strings** emitted
verbatim as the env vars (the operator writes the JSON), and
`memory.embed.documentInstruction` is a plain string — all emitted only when
non-empty, mirroring the existing `memory.embed.queryInstruction` wiring.

### The reindex-gating invariant

- **Hot (no reindex):** `query_instruction`, `query_params`. They change only the
  query vector at search time.
- **Reindex-gated:** `document_instruction`, `document_params`, and the
  tags-in-vector from #262. They change the stored document vector, so existing
  records must be re-embedded via `engram reindex --target <new>` and a
  collection cutover before the change takes effect.

This distinction is documented prominently; mixing a document-side param change
without a reindex yields query/document vectors from different regimes and
degrades recall.

## Error handling

- **Config validation (fail-fast, in `Config.Validate()`):** the JSON-object
  parse and reserved-key checks live in `Config.Validate()` (invoked via
  `loadAndValidate`), **not** in `Load` — `Load` stays assembly-only per ADR
  `engram-wtw` (a `Load` error means a malformed koanf layer, never operator
  input).
- **Empty is valid and is the default:** an empty `query_params` /
  `document_params` string is a **no-op** — `Validate()` skips parsing entirely,
  so every unconfigured deployment passes. Only a **non-empty**
  value is parsed; a non-empty value that is not a JSON object (including the
  `null` literal), or that contains a reserved key (`model` / `input`), yields a
  clear operator-facing error. Note these fields **self-gate on their own
  emptiness** (empty skips parsing), unlike the required-field, error-on-empty
  branches of `embed.dim` / `openai.base_url`; the presence-gated `summarize.*`
  group is a near (sibling-gated) rather than exact precedent.
- **Reserved keys (defense in depth):** `Config.Validate()` rejects a params
  object containing `model` or `input` (ambiguous operator intent, surfaced
  early). This is **complementary to**, not a replacement for, the
  request-building order above, which applies `model`/`input` last so any caller
  bypassing config validation (e.g. a test constructing a `Client` directly)
  still cannot clobber them.
- **Unknown fields at the provider:** engram cannot know whether a given gateway
  honors a field; a wrong `input_type`/`task_type` is the operator's
  responsibility (documented), the same posture as `ENGRAM_EMBED_MODEL`.

## Telemetry

`embed.Embed` spans gain a string attribute `engram.embed.kind` (`query` |
`document`) distinguishing the call, plus boolean attributes for whether
params/instruction were applied — without recording the values. Optional but
cheap; helps confirm the asymmetric path is live in production traces.

## Testing (TDD)

- **embed:** `EmbedQuery` merges `queryParams` into the body; `Embed` merges
  `documentParams`; reserved keys never clobbered; document instruction
  (placeholder + prefix + empty) — all via an `httptest` server capturing the
  decoded request body.
- **config:** empty string is valid — a no-op that `Validate()` passes (the
  default-install guard); a non-empty valid JSON object parses to a map; invalid
  JSON, non-object, and reserved-key cases are rejected by `Config.Validate()`
  (not `Load`).
- **reindex:** a re-embed applies `documentInstruction` + `documentParams`
  (integration, mirrors `TestReindexRoundtrip`).
- **regression:** empty config takes the struct fast-path and produces the exact
  prior wire bytes `{model, input}`; the non-empty path is asserted by decoding
  the request body (JSON object equality), not by byte comparison.

## Documentation

Extend `docs-site/guides/embedding-instructions`:

- Replace the "leave empty, set at gateway" cloud rows with concrete
  `ENGRAM_EMBED_QUERY_PARAMS` / `_DOCUMENT_PARAMS` values per provider.
- Give E5/nomic a `document_instruction` value and flag the required reindex.
- Add a "hot vs reindex-gated" callout for the query/document boundary.

## Rollout

Additive and opt-in; no migration for existing deployments. To adopt on a cloud
model: set the query/document param maps (query side is hot); if document params
or a document instruction are set, run `engram reindex` and repoint
`ENGRAM_QDRANT_COLLECTION` (see guides/reindex).

## Risks

- **Silent recall degradation** if a document-side knob (`document_params`,
  `document_instruction`) changes without a reindex: query and document vectors
  then come from different regimes. Mitigation: the reindex-gating invariant is
  documented prominently and the guide flags it per knob.
- **Provider ignores or renames the field:** a gateway may not honor a given
  `input_type` / `task_type`, or map it differently. engram cannot detect this;
  the value is the operator's responsibility (same posture as
  `ENGRAM_EMBED_MODEL`), and the guide gives verified per-provider values.

## Alternatives considered

- **Focused `input_type` toggle** — simplest, but hardcodes the field name/values
  and can't express Google `task_type` or future fields. Rejected for the
  "various models / possible switch" requirement.
- **Embedder profiles** — named presets bundling per-model handling. Nicer UX but
  a maintained registry; over-engineered. Rejected.

## References

- PR #262 (query instruction + tags-in-vector + score); GH #261.
- `docs-site/src/content/docs/guides/embedding-instructions.md`.
- `docs-site/src/content/docs/guides/reindex.md`.
- Grounding traces on `engram-0qed` (OpenRouter embeddings API; LiteLLM param passthrough).
<!-- adr-capture: sha256=508b7270562f0544; session=cli; ts=2026-07-01T23:24:14Z; adrs=engram-zyhq -->
