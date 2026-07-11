# Phase 14: Embedder Model Options & Eval - Research

**Researched:** 2026-07-11
**Domain:** Embedding-provider wire shapes (Gemini OpenAI-compat), eval-harness extension, operator docs
**Confidence:** HIGH (all load-bearing facts live-verified against vendor docs; local-provider facts CITED)

## Summary

The core unknown this phase hinges on — **does Gemini's OpenAI-compatibility `/embeddings`
endpoint honor `task_type`?** — is now answered definitively from live-scraped Google docs
(`ai.google.dev/gemini-api/docs/embeddings` and `.../docs/openai`, fetched 2026-07-11): **no**, and
the mechanism to use instead is a **text prefix on the input string**, not a JSON param. This
confirms and sharpens `.planning/research/PITFALLS.md` Pitfall 12.

The current-GA Gemini embedding model is `gemini-embedding-2` (Stable, latest update April 2026;
multimodal, first-party successor to `gemini-embedding-001`). It does **not** support `task_type`
at all, on either the native `embedContent` API or the OpenAI-compat endpoint — Google's docs say
verbatim: *"You cannot use the `task_type` field for the `gemini-embedding-2` model. Instead,
include the task as an instruction in your prompt."* The prescribed instruction format is a text
prefix baked into the embedded string itself (e.g. `task: search result | query: {content}` for
queries, `title: {title} | text: {content}` for documents) — which is **exactly** the mechanism
`internal/embed/embed.go`'s `WithQueryInstruction`/`WithDocumentInstruction` already implements
(literal `{query}`/`{document}` placeholder templates). No `queryParams`/`documentParams`
(`task_type` JSON field) recipe should be documented for `gemini-embedding-2` — that mechanism is
for `gemini-embedding-001`'s native API only, and even there, the OpenAI-compat page's `extra_body`
allowlist table has **zero rows** for the Embeddings endpoint, meaning `task_type` passthrough via
`ENGRAM_EMBED_QUERY_PARAMS`/`ENGRAM_EMBED_DOCUMENT_PARAMS` is undocumented/unsupported through
engram's actual wire path for either Gemini model. Native output dimension for `gemini-embedding-2`
is **3072** (flexible 128-3072 via MRL, recommended checkpoints 768/1536/3072) — D-02 ships the
recipe at the full 3072.

The `queryParams`/`documentParams` mechanism (`task_type` field) remains correctly documented for
other cloud providers (Cohere, Voyage, Jina) per the existing `guides/embedding-instructions.md`
table — this phase does not change that guidance, only adds the Gemini row with the
**instruction-prefix** mechanism instead.

The `internal/retrievaleval` harness needs no framework change — the Gemini differ-case is a new
`retrievalCase`-shaped fixture (or a lighter 2-record probe per D-04 discretion) added to
`retrievalCases`, driven by the same `TestRetrievalEval` loop, gated by
`ENGRAM_RETRIEVAL_EVAL=1` plus Gemini env vars. `StoreAndEmbedderFromEnvNoEnsure()` already builds
full prod-parity `*embed.Client` (all 6 `embed.Option`s wired via `embedderFromConfig`) from env —
no new Go code is needed in `internal/embed` or `internal/config` to support Gemini; it is a config
recipe, confirming D-03.

The OpenRouter reference config for the #261 re-point is `qwen/qwen3-embedding-8b` at its **native
maximum** dimension of **4096** (Matryoshka-capable 32–4096; 4096 is the ceiling, not a truncation
choice) — confirming D-08/D-10's "@4096" is the model's native full size. Its query-side
instruction format (`Instruct: {task}\nQuery:{text}`, documents raw) is exactly engram's existing
non-placeholder `WithQueryInstruction` wrap, already the documented row in
`guides/embedding-instructions.md`.

**Primary recommendation:** Ship the Gemini recipe on `gemini-embedding-2` @ native 3072 dims using
`ENGRAM_EMBED_QUERY_INSTRUCTION='task: search result | query: {query}'` and
`ENGRAM_EMBED_DOCUMENT_INSTRUCTION='title: none | text: {document}'` — **not**
`ENGRAM_EMBED_*_PARAMS`/`task_type`. Add a permanent skip-gated Gemini differ-case to
`retrievalCases` asserting `EmbedQuery(text) != Embed(text)` for the identical string, re-point
`gh261Case`'s prod-parity run at `qwen/qwen3-embedding-8b`@4096 via OpenRouter, and document both as
copy-paste recipes in a new `guides/embedding-models.md`, cross-linked from
`guides/embedding-instructions.md` and `guides/reindex.md`.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Gemini wire connectivity (base URL join, `/embeddings` POST) | API / Backend (`internal/embed`) | — | Phase 13's `joinEmbeddingsURL` already resolves the `/v1beta/openai` shape; no new tier touched |
| `task_type`/instruction correctness (query ≠ document vector) | API / Backend (`internal/embed` via config) | Database / Storage (Qdrant vectors written) | Correctness lives entirely in what string engram sends and how the provider embeds it — no browser/SSR/CDN involvement |
| Eval-gate proof (differ-assertion, recall@8 re-point) | API / Backend (`internal/retrievaleval`, Go test) | Database / Storage (testcontainer Qdrant) | Eval harness calls the same prod embed/store code path; it is backend-tier verification, not a new service |
| Operator-recipe docs | Docs / Static (docs-site Astro Starlight) | — | Pure content; no runtime tier |
| Helm recipe comments | Database / Storage config surface (`charts/engram/values.yaml`) | — | Chart values are deployment-time config, not a runtime tier of their own |

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-embed-gemini-direct | engram can embed against Gemini's embeddings API via its OpenAI-compat endpoint, with wire shape verified live and `task_type`/dimension behavior confirmed by an eval-harness run | Live-doc verification (§ Standard Stack, § Common Pitfalls) establishes the exact instruction-prefix mechanism and the 3072-dim native size; § Validation Architecture specifies the differ-case fixture that proves it |
| REQ-embed-prod-parity-eval | The #261 rank bar is re-confirmed on the prod-parity `qwen3-embedding-8b`@4096 config, closing #261 | Live-verified OpenRouter model id + native 4096 dim (§ Standard Stack); § Architecture Patterns / Code Examples shows the exact env + `gh261Case` re-point, no new threshold |
| REQ-embed-model-docs | A docs-site guide + commented Helm recipes document OpenRouter/Gemini/OpenAI/local models, each with base URL + model + dim + query instruction, reindex-boundary called out | § Standard Stack / Provider Recipe Matrix gives verified base URLs, model ids, dims, and instruction values for all four provider families |
</phase_requirements>

## Standard Stack

### Core

No new Go dependencies. This phase is config + eval-fixture + docs only — `internal/embed`,
`internal/config`, and `internal/retrievaleval` are reused verbatim (D-03, confirmed by source
read: `embedderFromConfig` already wires all 6 `embed.Option`s from koanf-loaded config; no new
`Option`, no new koanf key required for Gemini).

### Package Legitimacy Audit

**N/A — this phase installs no external packages** (Go, npm, or otherwise). No `go.mod`/`package.json`
changes are in scope. The Package Legitimacy Gate protocol is not applicable; skip its table.

### Provider Recipe Matrix (D-12)

All facts below are live-verified (`[VERIFIED: ...]`) except local-provider base-URL shapes, which
are `[CITED: ...]` from official docs (not re-fetched via the seam but pulled from their canonical
docs pages this session).

| Provider | Model id | Base URL | Native dim | Query-side mechanism | Source |
|----------|----------|----------|-----------|----------------------|--------|
| OpenRouter (eval reference, D-08) | `qwen/qwen3-embedding-8b` | `https://openrouter.ai/api/v1` | **4096** (MRL 32–4096; 4096 = ceiling) | `ENGRAM_EMBED_QUERY_INSTRUCTION='Given a web search query, retrieve relevant passages that answer the query'` (Qwen3-family wrap, documents raw) | `[VERIFIED: openrouter.ai/qwen/qwen3-embedding-8b, huggingface.co/Qwen/Qwen3-Embedding-8B]` |
| Gemini (D-01/D-02) | `gemini-embedding-2` | `https://generativelanguage.googleapis.com/v1beta/openai` | **3072** (MRL 128–3072; ship native full per D-02) | `ENGRAM_EMBED_QUERY_INSTRUCTION='task: search result | query: {query}'` + `ENGRAM_EMBED_DOCUMENT_INSTRUCTION='title: none | text: {document}'` — **NOT** `*_PARAMS`/`task_type` | `[VERIFIED: ai.google.dev/gemini-api/docs/embeddings, ai.google.dev/gemini-api/docs/openai — fetched 2026-07-11]` |
| OpenAI | `text-embedding-3-large` (or `-small`) | `https://api.openai.com/v1` | **3072** (`-large`) / **1536** (`-small`); optional `dimensions` request param truncates | Symmetric — leave `ENGRAM_EMBED_QUERY_INSTRUCTION` empty (already documented in `embedding-instructions.md`) | `[VERIFIED: developers.openai.com/api/docs/guides/embeddings]` |
| Local — TEI (Text Embeddings Inference) | operator-chosen (e.g. `BAAI/bge-m3`) | `http://<host>:8080/v1` | model-dependent (bge-m3 = 1024, existing Helm default) | model-dependent, per `embedding-instructions.md` table | `[CITED: huggingface.co/docs/text-embeddings-inference/en/quick_tour]` |
| Local — Ollama | operator-chosen (e.g. `ollama/bge-m3`) | `http://<host>:11434/v1` | model-dependent | model-dependent | `[CITED: docs.ollama.com/api/openai-compatibility, github.com/ollama/ollama/pull/5285]` |
| Local — vLLM | operator-chosen embedding-capable model | `http://<host>:8000/v1` | model-dependent | model-dependent | `[ASSUMED — standard vLLM OpenAI-compat serving convention; not re-verified this session, low risk since it is the same `/v1` shape already covered by `joinEmbeddingsURL`]` |

All four base-URL shapes above (`/v1`, `/v1beta/openai`, bare-host) are already covered by Phase
13's `joinEmbeddingsURL` heuristic (`internal/embed/embed.go:124-134`) — confirmed by source read.
No base-URL join code changes are needed for any of these recipes.

### Gemini model choice: why `gemini-embedding-2`, not `gemini-embedding-001`

D-01 says "current-GA Gemini embedding model." Both `gemini-embedding-001` (Stable, June 2025) and
`gemini-embedding-2` (Stable, April 2026) are GA per Google's model-version table — but
`gemini-embedding-2` is explicitly called "the latest model" and is the one meant by "Gemini
Embedder 2" per the user's own naming in D-01. `gemini-embedding-001` *does* support `task_type`
(`SEMANTIC_SIMILARITY`, `CLASSIFICATION`, `CLUSTERING`, `RETRIEVAL_DOCUMENT`, `RETRIEVAL_QUERY`,
`CODE_RETRIEVAL_QUERY`, `QUESTION_ANSWERING`, `FACT_VERIFICATION`) — but **only via the native
`embedContent` method**, per the doc structure (task_type is documented under "Task types with
Embeddings 1" as an `embedContent`-method parameter; the OpenAI-compat page's embeddings example
and its `extra_body` allowlist table show no `task_type`/dimensions row at all). Recommend
documenting `gemini-embedding-2` as the recipe (matches D-01's naming and is genuinely the newer
GA model), with `gemini-embedding-001` mentioned only as a migration/legacy note per Google's own
"Migration from gemini-embedding-001" section (embedding spaces are **incompatible** between the
two models — a full reindex, not just a param change, if an operator upgrades).

### Open documentation-lag flag (verify before finalizing the recipe)

The OpenAI-compat page's embeddings code sample (`ai.google.dev/gemini-api/docs/openai#embeddings`)
still shows `model="gemini-embedding-2-preview"` (not the bare `gemini-embedding-2` the native
embeddings guide calls Stable). This looks like doc staleness — the compat page has not been
updated post-GA-promotion — but it was not possible to confirm live which literal model-id string
the compat endpoint actually accepts without an API key. **Recommend the planner add a
`checkpoint:human-verify` task**: before locking the recipe/eval config, run one live
`curl https://generativelanguage.googleapis.com/v1beta/openai/embeddings -d '{"model":"gemini-embedding-2","input":"test"}'`
(and the `-2-preview` variant as fallback) to confirm which id string the compat endpoint accepts
today, and use that exact string in both the eval fixture and the docs recipe.

**Installation:** N/A (no packages).

## Architecture Patterns

### System Architecture Diagram

```
 operator env / Helm values (ENGRAM_EMBED_*, ENGRAM_OPENAI_*)
        │
        ▼
 config.Load + Validate (koanf)  ──►  config.ParseEmbedParams (query/document *_PARAMS JSON)
        │
        ▼
 embedderFromConfig()  ──►  embed.New(baseURL, apiKey, model, ...Option)
        │                         │
        │                         ├─ WithQueryInstruction / WithDocumentInstruction (TEXT PREFIX path — Gemini-2 uses this)
        │                         ├─ WithQueryParams / WithDocumentParams (JSON task_type path — Cohere/Voyage/Jina use this; NOT Gemini-2)
        │                         ├─ WithTimeout (Phase 13)
        │                         └─ WithEmbeddingsURL override or joinEmbeddingsURL heuristic (Phase 13 — resolves /v1beta/openai)
        ▼
 *embed.Client.EmbedQuery(text) / .Embed(text)
        │  POST {embeddingsURL} {"model","input", ...params}
        ▼
 provider (Gemini / OpenRouter / OpenAI / local) → []float32 vector
        │
        ▼
 store.Store.Upsert (document path) / SearchReranked (query path) — Qdrant

 ── Eval harness (internal/retrievaleval), parallel path ──
 TestRetrievalEval (ENGRAM_RETRIEVAL_EVAL=1 gate)
        │
        ├─ StoreAndEmbedderFromEnvNoEnsure() — SAME embedderFromConfig, from env
        ├─ seed retrievalCases[].seedRecords via em.Embed → st.Upsert   (gh261Case; existing)
        ├─ NEW: Gemini differ-case → em.EmbedQuery(text) vs em.Embed(text) on identical string → assert vectors differ
        └─ query retrievalCases[].queries via em.EmbedQuery → st.SearchReranked → recallAtK (gh261 hard gate, re-pointed at qwen3@4096)
```

### Recommended Project Structure

No new directories. Touched paths only:

```
internal/retrievaleval/
├── fixtures.go       # add Gemini differ-case to retrievalCases (or a dedicated small case)
├── retrieval_eval_test.go  # TestRetrievalEval unchanged in structure; may add a differ-only subtest branch if the case shape differs from retrievalCase
docs-site/src/content/docs/guides/
├── embedding-models.md        # NEW — recipes page (D-11)
├── embedding-instructions.md  # cross-link only, no content change
├── reindex.md                 # cross-link target only
charts/engram/values.yaml      # commented recipe blocks (D-13)
.planning/phases/14-embedder-model-options-eval/
└── <evidence file>            # committed eval run artifact (D-07 — location is planner discretion)
```

### Pattern 1: Text-prefix instruction for Gemini asymmetry (NOT JSON params)

**What:** Use engram's existing literal-placeholder instruction mechanism, not the params/task_type
mechanism, for `gemini-embedding-2`.
**When to use:** Any Gemini recipe on `gemini-embedding-2` via the OpenAI-compat endpoint.
**Example:**
```go
// Source: internal/embed/embed.go (existing, unmodified) — the placeholder
// substitution this recipe relies on.
// ENGRAM_EMBED_QUERY_INSTRUCTION='task: search result | query: {query}'
// ENGRAM_EMBED_DOCUMENT_INSTRUCTION='title: none | text: {document}'
//
// EmbedQuery("run task lint") sends body.input =
//   "task: search result | query: run task lint"
// Embed("run task lint") sends body.input =
//   "title: none | text: run task lint"
```
```env
# Gemini recipe (operator env / Helm values)
ENGRAM_EMBED_MODEL=gemini-embedding-2
ENGRAM_EMBED_DIM=3072
ENGRAM_EMBED_QUERY_INSTRUCTION=task: search result | query: {query}
ENGRAM_EMBED_DOCUMENT_INSTRUCTION=title: none | text: {document}
ENGRAM_OPENAI_BASE_URL=https://generativelanguage.googleapis.com/v1beta/openai
ENGRAM_OPENAI_API_KEY=<Gemini API key>
```

### Pattern 2: OpenRouter qwen3@4096 — #261 eval reference (D-08)

**What:** The reproducible reference config for re-pointing `gh261Case`.
**When to use:** `task eval:retrieval` local runs; documented as one recipe option (not a default).
**Example:**
```env
ENGRAM_EMBED_MODEL=qwen/qwen3-embedding-8b
ENGRAM_EMBED_DIM=4096
ENGRAM_EMBED_QUERY_INSTRUCTION=Given a web search query, retrieve relevant passages that answer the query
ENGRAM_OPENAI_BASE_URL=https://openrouter.ai/api/v1
ENGRAM_OPENAI_API_KEY=<OpenRouter API key>
ENGRAM_RETRIEVAL_EVAL=1
ENGRAM_QDRANT_TEST_ADDR=<optional: skip testcontainer, point at a running Qdrant>
```
```sh
task eval:retrieval
# == ENGRAM_RETRIEVAL_EVAL=1 go test ./internal/retrievaleval/ -run TestRetrievalEval -v
```

### Anti-Patterns to Avoid

- **Setting `ENGRAM_EMBED_QUERY_PARAMS='{"task_type":"RETRIEVAL_QUERY"}'` for Gemini:** silently
  no-ops through the OpenAI-compat endpoint (Pitfall 12) — no error, degraded recall, and nothing in
  engram's request/response cycle signals the failure. This is precisely what the differ-case eval
  gate (D-04) exists to catch.
- **Assuming `gemini-embedding-001` and `gemini-embedding-2` share an embedding space:** Google's
  docs state they are **incompatible** — switching between them (not just changing params) requires
  a full `engram reindex`, same as any other model change (D-14).
- **Documenting `gemini-embedding-2` as an MRL-truncated recipe:** D-02 explicitly ships native full
  3072; do not set `output_dimensionality`/truncate in the shipped recipe (it can still be mentioned
  as an option, per the model's flexible-dim support, but the default recipe is full-size).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Proving a provider honors asymmetric embedding | A request-body-only unit test asserting the JSON payload contains the right field | The D-04 differ-case: embed the same string via `EmbedQuery` and `Embed` through the real HTTP call and assert the two vectors differ | A body-shape assertion would pass even against a provider that accepts and silently discards the field — exactly Pitfall 12's failure mode |
| Gemini connectivity | A new native `google.golang.org/genai` client / new `embed.Option` | The existing `embed.Client` OpenAI-compat path (base URL + instruction env vars only) | D-03; explicitly out of scope per ROADMAP; the OpenAI-compat endpoint is fully sufficient for the instruction-prefix mechanism this model needs |
| Eval framework | A second eval harness/package for the Gemini case | `internal/retrievaleval`'s existing `retrievalCases`/`TestRetrievalEval` loop | Reuse of the exact same skip-gate, seeding sequence, and production embed/search path that `gh261Case` already validates |

**Key insight:** Every piece of this phase's "new" surface is a **config value or a fixture data
row** — no new Go abstraction is warranted, and building one would violate D-03/D-09's "no new
wiring" framing.

## Common Pitfalls

### Pitfall 1 (project PITFALLS.md #12, sharpened this session): Gemini `task_type` silent no-op via OpenAI-compat

**What goes wrong:** Setting `ENGRAM_EMBED_QUERY_PARAMS='{"task_type":"RETRIEVAL_QUERY"}'` (the
pattern documented for Cohere/Voyage/Jina) against a Gemini base URL produces a **valid 200
response** with a real embedding vector — engram has no way to detect that the `task_type` field
was ignored. Recall silently degrades to symmetric-embedding quality.
**Why it happens:** `gemini-embedding-2`'s task-type mechanism is a text-prefix convention on the
input string, not a request-body field at all — Google's own docs distinguish the two models by
exactly this mechanism. `gemini-embedding-001`'s `task_type` field exists only in the native
`embedContent` method's parameter schema; the OpenAI-compat page's `extra_body` allowlist table
has no Embeddings-endpoint row for it, so it is presumptively unsupported through that endpoint too
— but this specific negative (`gemini-embedding-001` + `task_type` + OpenAI-compat) was **not**
directly testable without an API key; document it as the safer inference, not a directly-observed
rejection.
**How to avoid:** Ship the Gemini recipe using `ENGRAM_EMBED_QUERY_INSTRUCTION`/
`ENGRAM_EMBED_DOCUMENT_INSTRUCTION` (text-prefix), never `*_PARAMS`. Gate the recipe behind the D-04
differ-case eval before documenting it as verified.
**Warning signs:** Recall quality regression with no error logs; the differ-case failing (query
vector == document vector) is the automated warning sign this phase is building.

### Pitfall 2: Doc-vs-compat-endpoint model-id drift

**What goes wrong:** The native embeddings guide says `gemini-embedding-2` is Stable; the
OpenAI-compat guide's code sample still shows `gemini-embedding-2-preview`. Using the wrong string
in the recipe could 404 or route to a different (possibly to-be-deprecated) model alias.
**Why it happens:** Google's OpenAI-compat docs page appears to lag the native embeddings guide's
GA promotion (observed 2026-07-11; cannot be dated further without page-history access).
**How to avoid:** A `checkpoint:human-verify` task doing one live curl against both id strings
before locking the recipe (see § Standard Stack "Open documentation-lag flag").
**Warning signs:** 404 or "model not found" from the compat endpoint; silently succeeding against
an unexpected model would be caught by the differ-case only if the wrong model also mishandles
`task_type` — recommend the human-verify step regardless of eval-gate coverage.

### Pitfall 3: Reindex boundary crossed silently on model/dim change

**What goes wrong:** An operator changes `ENGRAM_EMBED_MODEL`/`ENGRAM_EMBED_DIM` (e.g. adopting
Gemini) without reindexing; old vectors (different model/space) and new vectors coexist in one
Qdrant collection, corrupting similarity search silently (project PITFALLS.md Pitfall 13, HIGH
recovery cost — "no way to selectively repair mixed-space vectors after the fact").
**Why it happens:** Qdrant accepts any vector of the configured dimension; a same-dimension
model swap (e.g. `text-embedding-3-large` at 3072 → `gemini-embedding-2` at 3072) produces no
dimension-mismatch error even though the vector spaces are incompatible.
**How to avoid:** Every recipe (docs table row + Helm comment block) explicitly calls out "reindex
required," cross-linking `guides/reindex.md` (D-14). Phase 13's embedder-config-identity stamp
(D-01 of Phase 13) records model+dim+document_instruction+document_params per record for future
audit, but this phase does not add enforcement — documentation is the only mitigation shipped now.
**Warning signs:** N/A at write time (silent); a future reindex-audit CLI (deferred, per Phase 13
Deferred Ideas) would be the detection mechanism.

## Code Examples

### Gemini differ-case fixture shape (source-grounded sketch for the planner)

```go
// Source: internal/retrievaleval/fixtures.go (existing retrievalCase shape) +
// internal/retrievaleval/retrieval_eval_test.go (existing TestRetrievalEval loop).
// Sketch only — planner/implementer decides exact case shape per D-04 discretion
// (full retrievalCase reuse vs a minimal 2-record differ probe).

// Option A: reuse retrievalCase, add a differ-specific assertion in
// TestRetrievalEval alongside the existing recall/rank assertions — gated on
// whether Gemini env vars are set (skip this sub-check otherwise), e.g.:
if os.Getenv("ENGRAM_EMBED_MODEL") == "gemini-embedding-2" {
    qVec, _ := em.EmbedQuery(ctx, "run task lint before every commit")
    dVec, _ := em.Embed(ctx, "run task lint before every commit")
    if reflect.DeepEqual(qVec, dVec) {
        t.Errorf("Gemini task_type/instruction had no effect: query vector == document vector (Pitfall 12)")
    }
}

// Option B (D-04 "minimal 2-record differ probe"): a standalone small
// retrievalCase-shaped fixture — e.g. gemini2DifferCase — seeded and queried
// through the SAME production Embed/EmbedQuery/SearchReranked path as
// gh261Case, added to retrievalCases, with its own t.Run subtest name.
```

### Existing `embed.go` instruction wrap (verbatim, for planner reference)

```go
// Source: internal/embed/embed.go:196-206 (read this session, unmodified by
// this phase)
func (c *Client) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	switch {
	case c.queryInstruction == "":
		// raw
	case strings.Contains(c.queryInstruction, queryPlaceholder): // "{query}"
		text = strings.ReplaceAll(c.queryInstruction, queryPlaceholder, text)
	default:
		text = "Instruct: " + c.queryInstruction + "\nQuery: " + text
	}
	return c.embed(ctx, text, c.queryParams, "query")
}
```
`ENGRAM_EMBED_QUERY_INSTRUCTION='task: search result | query: {query}'` hits the `{query}`-contains
branch (literal substitution) — exactly the Gemini prefix format. `ENGRAM_EMBED_DOCUMENT_INSTRUCTION`
follows the analogous `{document}`-contains branch in `Embed()`.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `gemini-embedding-001` + native `task_type` param | `gemini-embedding-2` + text-prefix task instruction | `gemini-embedding-2` GA, per Google's model-version table "Latest update: April 2026" | Any Gemini recipe written against older `task_type`-based guidance (including engram's own PITFALLS.md #12, written referencing a March-2026 forum thread) is now confirmed against the current shipped docs, not just a forum report |
| N/A | `embed.go` embedding-space incompatibility explicitly documented by Google between `-001` and `-2` | Same GA promotion | Reinforces D-14's reindex-boundary framing — this is not engram-specific caution, it's the vendor's own stated behavior |

**Deprecated/outdated:**
- `gemini-embedding-2-preview` as the primary recipe id — superseded by the Stable `gemini-embedding-2`
  per the native embeddings guide, though the OpenAI-compat page's own example has not yet been
  updated to match (§ Standard Stack open flag).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | vLLM's OpenAI-compat embeddings endpoint follows the standard `http://<host>:8000/v1` base-URL shape | Standard Stack / Provider Recipe Matrix | Low — same `/v1` shape `joinEmbeddingsURL` already handles; wrong port number in the docs example only, easily operator-corrected |
| A2 | `gemini-embedding-001`'s `task_type` field is unsupported (not merely undocumented) through the OpenAI-compat `/embeddings` endpoint | Common Pitfalls #1 | If wrong (i.e. it IS silently accepted and applied), the recipe recommendation to avoid `*_PARAMS` for Gemini-001 is overly conservative but not unsafe — worst case is a documented-but-unused capability, not a correctness bug |
| A3 | The OpenAI-compat page's `gemini-embedding-2-preview` example reflects doc staleness rather than the compat endpoint genuinely requiring the `-preview` suffix | Standard Stack open flag; Common Pitfalls #2 | If wrong, the shipped recipe's model id 404s until corrected — mitigated by the recommended `checkpoint:human-verify` task before finalizing |

## Open Questions (RESOLVED)

> All three resolved during Phase 14 planning (plans committed `1648b12c`):
> - **Q1** (exact Gemini compat model-id) → RESOLVED by 14-03's `checkpoint:human-verify` live-curl task, which locks the exact string in both the eval fixture and the docs recipe.
> - **Q2** (differ-case dataset shape) → RESOLVED by 14-01's minimal 2-record/1-string probe (`TestRetrievalEval_AsymmetryDiffer`; renamed from `TestEmbedAsymmetryDiffer` in the reviews pass so `task eval:retrieval`'s `-run TestRetrievalEval` regex reaches it).
> - **Q3** (eval-evidence artifact location) → RESOLVED by 14-03's committed `14-EVAL-EVIDENCE.md`.

1. **Exact model-id string the OpenAI-compat endpoint accepts for the GA Gemini model**
   - What we know: the native embeddings guide calls `gemini-embedding-2` (no suffix) Stable/GA;
     the OpenAI-compat guide's own code sample still uses `gemini-embedding-2-preview`.
   - What's unclear: whether the compat endpoint accepts both, only the `-preview` alias, or has
     been updated server-side ahead of its own docs.
   - Recommendation: `checkpoint:human-verify` — one live curl against both strings before locking
     the eval fixture and docs recipe (see § Standard Stack).

2. **Whether the differ-case should reuse `gh261Case`'s dataset or a minimal 2-record probe (D-04
   discretion)**
   - What we know: the differ-assertion only needs ONE string embedded twice (query-side vs
     document-side); it doesn't need `gh261Case`'s 15-distractor recall-ranking setup at all.
   - What's unclear: whether the planner wants it as a fully separate lightweight case (faster,
     clearer intent) or folded into an existing case's flow (less new fixture code).
   - Recommendation: minimal 2-record/1-string probe (Option B in § Code Examples) — it directly
     tests the crux (vector equality) without conflating it with ranking-quality assertions gh261
     already owns.

3. **Committed eval-evidence artifact location/format (D-07 discretion)**
   - What we know: no existing precedent in this repo for a committed eval-results file (Phase 9's
     baseline was captured only via `t.Logf`, not a committed artifact).
   - What's unclear: exact filename/format the planner should use.
   - Recommendation: a new `.planning/phases/14-embedder-model-options-eval/14-EVAL-EVIDENCE.md`
     (or similar) capturing the `task eval:retrieval -v` output (recall@8 numbers for the re-pointed
     `gh261Case`, plus the Gemini differ-assertion PASS line) — colocated with the phase's other
     artifacts, consistent with how `commit_docs: true` already treats this phase dir as
     version-controlled.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker (testcontainers) | `task eval:retrieval` (ephemeral Qdrant) | not probed this session (developer-machine-dependent) | — | `ENGRAM_QDRANT_TEST_ADDR` points at an already-running Qdrant, per `TestMain` |
| Gemini API key | Gemini differ-case eval run | operator-supplied, not present in this environment | — | Eval skips via `ENGRAM_RETRIEVAL_EVAL` gate if unset; documented as a manual pre-merge step (D-06) |
| OpenRouter API key | qwen3@4096 #261 re-point eval run | operator-supplied, not present in this environment | — | Same skip-gate fallback |

**Missing dependencies with no fallback:** none — both API keys and Docker have documented
fallbacks (skip gate / `ENGRAM_QDRANT_TEST_ADDR`) already built into the harness.

**Missing dependencies with fallback:** Gemini API key, OpenRouter API key, Docker — all fall back
to the existing skip-gate/env-override mechanism; this phase adds no new environment requirement
beyond what `gh261Case` already needs.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` (stdlib) + `testcontainers-go` (Qdrant) |
| Config file | none — gated entirely by env vars (`ENGRAM_RETRIEVAL_EVAL`, `ENGRAM_QDRANT_TEST_ADDR`, embed-provider env) |
| Quick run command | `task eval:retrieval` (requires live gateway + optional Docker; not part of the default `task` gate) |
| Full suite command | `task` (lint + `go test ./...`) — `internal/retrievaleval`'s `TestMain` short-circuits to zero-cost when `ENGRAM_RETRIEVAL_EVAL` is unset, so this phase adds no cost to the default gate |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-embed-gemini-direct | Gemini query vector ≠ document vector for identical text (proves `task_type`/instruction takes effect) | integration (live gateway) | `ENGRAM_RETRIEVAL_EVAL=1 ENGRAM_EMBED_MODEL=gemini-embedding-2 ... go test ./internal/retrievaleval/ -run TestRetrievalEval -v` | ❌ Wave 0 — new fixture/assertion to add to `fixtures.go`/`retrieval_eval_test.go` |
| REQ-embed-prod-parity-eval | `gh261Case` Record T surfaces within default k=8 against qwen3@4096 | integration (live gateway, hard rank assertion — already implemented) | `ENGRAM_RETRIEVAL_EVAL=1 ENGRAM_EMBED_MODEL=qwen/qwen3-embedding-8b ENGRAM_EMBED_DIM=4096 ENGRAM_EMBED_QUERY_INSTRUCTION='...' ENGRAM_OPENAI_BASE_URL=https://openrouter.ai/api/v1 task eval:retrieval` | ✅ — `gh261Case`/`TestRetrievalEval` already implement the hard gate; only the env config changes, no test code |
| REQ-embed-model-docs | Docs page + Helm comments render/lint correctly and cross-link `guides/reindex` | manual-only — docs correctness is not unit-testable | `task lint:markdown` (structure/lint only, not content correctness) | ❌ Wave 0 — new `embedding-models.md` file |

### Sampling Rate

- **Per task commit:** `task` (lint + default `go test ./...` — the retrieval-eval package is a
  zero-cost skip when the manual gate is unset, so this stays fast).
- **Per wave merge:** manually run `task eval:retrieval` once Gemini/OpenRouter env + Docker or
  `ENGRAM_QDRANT_TEST_ADDR` are available (D-06 — local/manual, no CI job this phase), then capture
  its output into the committed evidence artifact (D-07 / Open Question 3).
- **Phase gate:** `task` green (lint+test) is required before `/gsd-verify-work`; the manual eval
  run is a **separate, explicitly documented pre-merge step** (not part of the automated gate) per
  D-06 — success criteria #1/#2 are closed by the committed evidence artifact, not by CI.

### Wave 0 Gaps

- [ ] Gemini differ-case fixture/assertion in `internal/retrievaleval/fixtures.go` +
      `retrieval_eval_test.go` — covers REQ-embed-gemini-direct
- [ ] `docs-site/src/content/docs/guides/embedding-models.md` — covers REQ-embed-model-docs
- [ ] Committed eval-evidence artifact (location per Open Question 3) — closes success criteria
      #1/#2 per D-07
- No new test-framework install needed — `testing` + `testcontainers-go` already used by
  `TestRetrievalEval`.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | no | This phase touches no engram-side auth surface — it configures *outbound* calls to third-party embedding APIs |
| V3 Session Management | no | N/A |
| V4 Access Control | no | N/A |
| V5 Input Validation | yes (pre-existing, unchanged) | `config.ParseEmbedParams` already rejects reserved keys (`model`/`input`) and non-object JSON — no new validation surface added by this phase's Gemini recipe, since Gemini uses the instruction (string) path, not the params (JSON) path |
| V6 Cryptography | yes (pre-existing, unchanged) | Embedding-provider API keys (Gemini/OpenRouter/OpenAI) are handled exactly like the existing `ENGRAM_OPENAI_API_KEY`/Helm `apiKeySecret` pattern — `secretKeyRef`-backed, never plaintext in values.yaml. This phase adds no new secret-handling code, only new documented recipe values referencing the same existing secret mechanism |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|----------------------|
| API key committed to docs/Helm example in plaintext | Information Disclosure | D-13's commented recipes must use placeholder values (`<your-key>` style), never a real key — mirror the existing `values.yaml` `apiKeySecret.name`/`key` indirection pattern, never inline the secret |
| Eval fixture leaking real content to a third-party embedding API | Information Disclosure | `gh261Case`/differ-case content is already synthetic public-tooling text (no secrets) — new Gemini fixture content must follow the same synthetic-content convention documented in `fixtures.go`'s `gh261Distractors` comment |

## Sources

### Primary (HIGH confidence)
- `ai.google.dev/gemini-api/docs/embeddings` (scraped 2026-07-11) — model-version table, task-type
  mechanism per model, native dims, migration-incompatibility note
- `ai.google.dev/gemini-api/docs/openai` (scraped 2026-07-11) — OpenAI-compat embeddings example,
  `extra_body` allowlist table (no Embeddings-endpoint row)
- `developers.openai.com/api/docs/guides/embeddings` — OpenAI `text-embedding-3-small/-large` dims,
  `dimensions` truncation param
- `openrouter.ai/qwen/qwen3-embedding-8b` (+ `/api`, `/pricing`) — OpenRouter model id, context
  length
- `huggingface.co/Qwen/Qwen3-Embedding-8B` (README) — native embedding dimension table (8B = 4096),
  instruction format
- `internal/embed/embed.go`, `internal/config/embedparams.go`, `internal/config/registry.go`,
  `internal/config/config.go`, `internal/server/tools.go` (this repo, read this session) — exact
  param plumbing, env var names, `embedderFromConfig` wiring

### Secondary (MEDIUM confidence)
- `huggingface.co/docs/text-embeddings-inference/en/quick_tour` — TEI OpenAI-compat `/v1` base URL
- `docs.ollama.com/api/openai-compatibility`, `github.com/ollama/ollama/pull/5285` — Ollama
  `:11434/v1` base URL
- `.planning/research/PITFALLS.md` Pitfall 12 (this repo's own prior research, 2026-07-10) —
  corroborates and predates this session's live-doc confirmation

### Tertiary (LOW confidence)
- vLLM base-URL convention (`:8000/v1`) — not re-verified this session; standard/well-known
  convention, flagged `[ASSUMED]` (Assumption A1)

## Metadata

**Confidence breakdown:**
- Standard stack (Gemini model/dim/task_type mechanism): HIGH — directly scraped from
  `ai.google.dev` this session, cross-checked against two pages (embeddings guide + OpenAI-compat
  guide) that agree
- Architecture (eval-harness extension shape): HIGH — read verbatim from
  `internal/retrievaleval/fixtures.go` and `retrieval_eval_test.go`
- Pitfalls: HIGH for the Gemini `task_type` no-op mechanism (live-confirmed); MEDIUM for the
  `gemini-embedding-001` + OpenAI-compat negative (inferred from the `extra_body` table's absence
  of an Embeddings row, not a directly observed rejection)
- Local-provider recipe rows (TEI/Ollama/vLLM): MEDIUM (TEI/Ollama CITED) / LOW (vLLM ASSUMED)

**Research date:** 2026-07-11
**Valid until:** ~30 days for the local-provider rows (stable ecosystem conventions); ~7-14 days
for the Gemini model-id/GA-status claims specifically, given the observed doc-staleness between
Google's own pages this session — re-verify the exact compat-endpoint model-id string at
implementation time via the recommended `checkpoint:human-verify` step regardless of this
research's age.
