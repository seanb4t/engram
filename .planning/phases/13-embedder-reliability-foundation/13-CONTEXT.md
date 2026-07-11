# Phase 13: Embedder Reliability Foundation - Context

**Gathered:** 2026-07-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Harden the existing embedding-client subsystem with three **isolated** reliability
fixes. Scope is confined to `internal/embed`, its koanf config, and the store
write-path stamping. No changes to recall semantics, the reranker, the embedding
model, or the Connect/proto wire.

Deliverables:
1. **Operator-tunable HTTP timeout** — `ENGRAM_EMBED_TIMEOUT` replaces the
   hardcoded `30 * time.Second` at `embed.go:77` so a provider 529 brownout no
   longer hangs the calling MCP tool.
2. **Correct base-URL joining** — `embed.go:191`'s naive `baseURL+"/v1/embeddings"`
   is replaced with a shape-aware join proven by a provider-shape table test.
3. **Per-record embedder-config-identity stamp** — every stored record carries a
   hash of its embedding-space identity so a future reindex-boundary audit can
   detect mixed-embedding-space records.

Fully isolated: zero import-graph overlap with the write-lane track (Phases
15-20). Ships first as low-risk throughput (ROADMAP `Depends on: Nothing`).

</domain>

<decisions>
## Implementation Decisions

### Embedder-Config-Identity (REQ-embed-config-identity / DECISION 3)

- **D-01:** Identity fields = **model + dim + document_instruction + document_params**
  (document-side only). EXCLUDED: `query_instruction`/`query_params` (only affect
  query-side embeds, never the stored document vector), `base_url`, `api_key`,
  `timeout` (do not change the embedding space; same model name is assumed to be
  the same space). Rationale: the stored vector is a *document* embed, and the
  identity's job is to detect when a record was written under a different
  embedding space — so only fields that change the stored vector belong.
- **D-02:** The stamp carries a **scheme/version prefix** (`v1:`) so the hashing
  scheme can evolve later without old records ambiguously reading as space-drift.
- **D-03:** Representation = **version-prefixed short hash**: `v1:` + first 16 hex
  chars of `SHA-256` over a canonical serialization of the D-01 fields. Compact,
  fixed-size, stable. Opaque is fine — the audit only needs equality/grouping; a
  debug log may print the pre-image if a human needs to see why two differ.
- **D-04:** Computed by a **pure helper `embedderIdentity(cfg)`** in the config or
  store layer, because that layer holds BOTH the embed config AND `Embed.Dim`.
  Single source of truth, trivially table-testable. NOT split across
  `embed.Client` (which does not hold `dim`).
- **D-05:** **Stamped on every document-embed write path**: `store_memory`,
  `update_memory` (re-embed), `store_discovery`, `store_rule`, and `reindex`
  (`tools.go:603/642/700/932`, `store.go:2135`). New `Memory` payload field,
  round-tripped through `payload()`/`fromPayload()`; legacy records missing the
  key read empty, no backfill (mirrors the AccessCount/LastAccessedAt precedent).
- **D-06:** **Payload-only audit field** — NOT added to the `recallView` allowlist
  (`internal/server/summary.go`), NOT on the proto/Connect wire (**no proto bump
  this phase**). The future reindex-audit CLI reads it directly from Qdrant.

### Embed Timeout (REQ-embed-timeout / GH #333)

- **D-07:** New koanf field `embed.timeout` → `ENGRAM_EMBED_TIMEOUT`, **default
  `30s`** (preserves current behavior). Replaces the hardcoded literal at
  `embed.go:77`; threaded through `embedderFromConfig` (`tools.go:306`).
- **D-08:** Validation **mirrors `ENGRAM_SUMMARY_TIMEOUT`** (`validate.go:~98`):
  parse as Go duration, reject negative. **`0` = no timeout (infinite)**,
  consistent with `summarize.timeout` semantics — the explicit operator escape
  hatch for very slow local models. (Success criterion #4 — "fails within the
  *configured* timeout" — is still met; `0` is the operator configuring "no bound".)
- **D-09:** Summary-queue coupling is an **ASSERT-ONLY INVARIANT**, not new wiring.
  Embed timeout and the summary-queue backoff budget are independent
  (`summaryqueue.go` `attemptTimeout = ENGRAM_SUMMARY_TIMEOUT`; `maxElapsed`
  already derived, no `30 * time.Second` literal present). Add/confirm a regression
  test that `maxElapsed` tracks `ENGRAM_SUMMARY_TIMEOUT`. **Researcher MUST confirm
  no hidden embed→queue coupling** before finalizing; if a real coupling is found,
  it surfaces as a plan change. No `summaryqueue.go` code change expected.

### Base-URL Join (REQ-embed-baseurl-join / GH #332)

- **D-10:** **Smart heuristic join**: normalize (trim trailing slash), then — if the
  path already terminates at an OpenAI-compat root (ends in `/v1` or
  `/v1beta/openai`) append `/embeddings`; else append `/v1/embeddings`. Replaces
  the naive concat at `embed.go:191`. Covers OpenRouter (`/v1` → no `/v1/v1`),
  OpenAI (no `/v1`), trailing-slash, and Gemini (`/v1beta/openai` → not
  `/v1beta/openai/v1`).
- **D-11:** **PLUS an explicit operator override escape hatch** (new config env,
  e.g. `ENGRAM_OPENAI_EMBEDDINGS_URL` or a path suffix) for unanticipated shapes
  (e.g. Azure). When set it **wins and bypasses the heuristic**; when empty (the
  default), the heuristic applies. Must be validated (valid URL / well-formed path).
  Exact env name + full-URL-vs-path form = planner discretion.
- **D-12:** Proven by a **provider-shape table test** enumerating all four shapes +
  trailing-slash variant + the override path. Resolve the embeddings URL **once**
  (in `embed.New` or a pure `joinEmbeddingsURL` helper), not per-request — planner
  discretion.

### Claude's Discretion

- Exact `embed.Client` timeout wiring: new `New()` signature vs a `WithTimeout`
  functional option (follow the existing `Option` pattern).
- Exact override env var name and its full-URL-vs-path form + validation (D-11).
- The canonical serialization feeding the SHA-256 (key order, separators) — must be
  deterministic and documented; the `v1:` prefix covers future changes.
- Package placement of the `embedderIdentity` and `joinEmbeddingsURL` helpers.
- Optional OTEL span attribute exposing the identity hash on embed spans (nice-to-have).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Scope anchors
- `.planning/ROADMAP.md` § Phase 13 — goal + 4 success criteria (the fixed boundary).
- `.planning/REQUIREMENTS.md` — `REQ-embed-timeout` (#333), `REQ-embed-baseurl-join`
  (#332), `REQ-embed-config-identity` (DECISION 3).

### Embedder subsystem (source of truth)
- `internal/embed/embed.go` — `Client`, `New()` (30s timeout at :77), `embed()`
  (baseURL join at :191), functional `Option`s (`WithQueryParams`,
  `WithDocumentInstruction`, `WithHTTPTransport`, …).
- `internal/embed/embed_test.go` — existing embed tests; extend with the
  provider-shape table test (D-12).
- `internal/server/tools.go` — `embedderFromConfig` (~306, construction from cfg);
  document-embed write sites at ~603/642/700/932.
- `internal/config/registry.go` — koanf field registry (add `embed.timeout`);
  `internal/config/config.go` `EmbedConfig`/`OpenAIConfig` structs;
  `internal/config/validate.go` (embed + `summarize.timeout` validation pattern ~98);
  `internal/config/embedparams.go` — `ParseEmbedParams`.

### Store / payload
- `internal/store/store.go` — `Memory` struct (~86), `fromPayload` (~337) + payload
  writer, `EmbedText` (~159), doc-embed site (~2135).
- `internal/server/summaryqueue.go`, `internal/store/summarize.go` — summary-queue
  backoff budget (`maxElapsed` derivation; the D-09 assert-only invariant target).
- `internal/server/summary.go` — `recallView`/`toRecallView` allowlist; the identity
  field must **NOT** be added here (payload-only, D-06).

### Codebase maps
- `.planning/codebase/CONVENTIONS.md`, `.planning/codebase/STACK.md` — Go conventions
  and stack.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`embed.Option` functional-options pattern** (`embed.go:45-73`): add
  `WithTimeout` the same way; `embed.New` is the single construction seam (D-07).
- **Config registry + validate pattern** (`registry.go` field list + `validate.go`
  per-field checks): add `embed.timeout` mirroring `summarize.timeout` exactly (D-08).
- **Memory payload round-trip** (`payload()`/`fromPayload()`): add the identity
  field following the `AccessCount`/`LastAccessedAt` precedent — server-set,
  legacy-missing reads zero-value, no backfill (D-05).
- **`summaryqueue.go` `maxElapsed = (attemptTimeout + maxInterval) * maxTries`**:
  the invariant to lock with a regression test (D-09).

### Established Patterns / Constraints
- `recallView` is a **hand-written allowlist** — deliberately NOT touched (D-06).
- Connect exposure requires a proto bump + `task proto:gen` + `gen/` drift check —
  deliberately **avoided** this phase (no wire surface).
- Optional timestamps use `*time.Time` (nil = never) — N/A for the string identity,
  but the "legacy-missing reads zero-value, no backfill" convention applies.
- Every Go/Markdown file carries the Apache-2.0 SPDX header (`task license:check`).
- `task` (lint + test) must be clean; `.planning/` rumdl markdown-lint is a systemic
  pre-existing warning (excluded from the Go gate), not a regression.
- gopls can emit stale diagnostics after codegen — trust `go build` over the IDE
  (no codegen expected this phase, but relevant if any struct/proto touch creeps in).

### Integration Points
- `embed.New` (`internal/embed`) ← `embedderFromConfig` (`tools.go`) ← serve/deps wiring.
- New `embedderIdentity` helper ← store write paths (store/update/discovery/rule/reindex).
- config `registry.go`/`validate.go` ← `ENGRAM_EMBED_TIMEOUT` + the optional override env (D-11).

</code_context>

<specifics>
## Specific Ideas

- Provider shapes the table test (D-12) must cover: OpenRouter
  `https://openrouter.ai/api/v1`, OpenAI `https://api.openai.com/v1` (and bare host
  with no `/v1`), a trailing-slash variant, Gemini
  `https://generativelanguage.googleapis.com/v1beta/openai`, plus the explicit
  override path.
- The three fixes are independent and can be planned as parallel waves; only D-09's
  regression test touches the summary-queue area (read-only assertion).

</specifics>

<deferred>
## Deferred Ideas

- **Reindex-boundary AUDIT CLI** — the *consumer* of the identity hash (a command
  that reads/compares stamps to flag mixed-embedding-space records). Phase 13 only
  STAMPS the data; the audit command is future work.
- **Surfacing the identity on `get_memory` / Connect wire** — deferred; payload-only
  for now. Would need a `recallView` allowlist edit + proto bump if a consumer appears.
- **Azure OpenAI-style deployment URLs**
  (`/openai/deployments/{id}/embeddings?api-version=`) — out of scope for the join
  heuristic; the explicit override escape hatch (D-11) is the intended path for such
  shapes.

</deferred>

---

*Phase: 13-embedder-reliability-foundation*
*Context gathered: 2026-07-10*
