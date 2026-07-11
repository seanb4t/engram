# Phase 13: Embedder Reliability Foundation - Research

**Researched:** 2026-07-10
**Domain:** Go HTTP client hardening, koanf config validation, Qdrant payload stamping
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Embedder-Config-Identity (REQ-embed-config-identity / DECISION 3)**
- **D-01:** Identity fields = model + dim + document_instruction + document_params (document-side only). EXCLUDED: query_instruction/query_params, base_url, api_key, timeout.
- **D-02:** The stamp carries a version/scheme prefix (`v1:`).
- **D-03:** Representation = `v1:` + first 16 hex chars of SHA-256 over a canonical serialization of the D-01 fields.
- **D-04:** Computed by a pure helper `embedderIdentity(cfg)` in the config or store layer (holds both embed config AND `Embed.Dim`). NOT split across `embed.Client`.
- **D-05:** Stamped on every document-embed write path: `store_memory`, `update_memory` (re-embed), `store_discovery`, `store_rule`, and `reindex` (`tools.go:603/642/700/932`, `store.go:2135`). New `Memory` payload field, round-tripped through `payload()`/`fromPayload()`; legacy records missing the key read empty, no backfill (mirrors AccessCount/LastAccessedAt precedent).
- **D-06:** Payload-only audit field — NOT added to `recallView` allowlist (`internal/server/summary.go`), NOT on the proto/Connect wire (no proto bump this phase).

**Embed Timeout (REQ-embed-timeout / GH #333)**
- **D-07:** New koanf field `embed.timeout` → `ENGRAM_EMBED_TIMEOUT`, default `30s`. Replaces the hardcoded literal at `embed.go:77`; threaded through `embedderFromConfig` (`tools.go:306`).
- **D-08:** Validation mirrors `ENGRAM_SUMMARY_TIMEOUT` (`validate.go:~98`): parse as Go duration, reject negative. `0` = no timeout (infinite).
- **D-09:** Summary-queue coupling is an ASSERT-ONLY INVARIANT, not new wiring. Embed timeout and the summary-queue backoff budget are independent. Add/confirm a regression test that `maxElapsed` tracks `ENGRAM_SUMMARY_TIMEOUT`. Researcher MUST confirm no hidden embed→queue coupling. No `summaryqueue.go` code change expected.

**Base-URL Join (REQ-embed-baseurl-join / GH #332)**
- **D-10:** Smart heuristic join: normalize (trim trailing slash), then — if the path already terminates at an OpenAI-compat root (ends in `/v1` or `/v1beta/openai`) append `/embeddings`; else append `/v1/embeddings`.
- **D-11:** PLUS an explicit operator override escape hatch (new config env) for unanticipated shapes (e.g. Azure). When set it wins and bypasses the heuristic; when empty (default), the heuristic applies. Must be validated (valid URL / well-formed path). Exact env name + full-URL-vs-path form = planner discretion.
- **D-12:** Proven by a provider-shape table test enumerating all four shapes + trailing-slash variant + the override path. Resolve the embeddings URL once (in `embed.New` or a pure `joinEmbeddingsURL` helper), not per-request.

### Claude's Discretion
- Exact `embed.Client` timeout wiring: new `New()` signature vs a `WithTimeout` functional option (follow the existing `Option` pattern).
- Exact override env var name and its full-URL-vs-path form + validation (D-11).
- The canonical serialization feeding the SHA-256 (key order, separators) — must be deterministic and documented; the `v1:` prefix covers future changes.
- Package placement of the `embedderIdentity` and `joinEmbeddingsURL` helpers.
- Optional OTEL span attribute exposing the identity hash on embed spans (nice-to-have).

### Deferred Ideas (OUT OF SCOPE)
- Reindex-boundary AUDIT CLI (the consumer of the identity stamp) — this phase only stamps.
- Surfacing the identity on `get_memory` / Connect wire — payload-only for now. Would need a `recallView` allowlist edit + proto bump if a consumer appears.
- Azure OpenAI-style deployment URLs (`/openai/deployments/{id}/embeddings?api-version=`) — out of scope for the join heuristic; the D-11 override escape hatch is the intended path.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-embed-timeout | Operator-configurable `ENGRAM_EMBED_TIMEOUT`, koanf-validated, replacing hardcoded 30s; must not silently break the async summary-queue backoff budget | Confirmed exact current code at `embed.go:76-77`; confirmed `summaryqueue.go` has NO hardcoded 30s literal and already derives `maxElapsed` from `attemptTimeout` (itself from `ENGRAM_SUMMARY_TIMEOUT` via `summaryTimeout(cfg)`) — D-09 is a real assert-only invariant, no coupling found. See "D-09 Verification" below. |
| REQ-embed-baseurl-join | `ENGRAM_OPENAI_BASE_URL` joins correctly across OpenRouter/OpenAI/Gemini shapes, proven by table test | Confirmed naive concat at `embed.go:191`; confirmed all four documented shapes via web search (OpenRouter `/api/v1`, Gemini `/v1beta/openai`); wrote the exact heuristic algorithm and edge cases below. |
| REQ-embed-config-identity | Every stored record stamped with an embedder-config-identity hash | Confirmed `Memory` struct, `payload()`/`fromPayload()` round-trip pattern via the `AccessCount` precedent; **found a landmine**: `Reindex()` copies `p.Payload` VERBATIM (does not go through `payload()`) and `StoreAndEmbedderFromEnvNoEnsure()` discards `cfg`, so the reindex write site needs bespoke handling — see "Decisions to Revisit". |
</phase_requirements>

## Summary

The three fixes are small, independent, and land on exactly the lines CONTEXT.md predicted — no `~line` hint in the CONTEXT was materially wrong. `embed.go`'s hardcoded `30 * time.Second` (line 77) and naive `baseURL+"/v1/embeddings"` concat (line 191) are both confirmed live. `summaryqueue.go` genuinely has no stale `30 * time.Second` literal — `maxElapsed` is derived from `attemptTimeout`, which is passed in from `summaryTimeout(cfg)` (`ENGRAM_SUMMARY_TIMEOUT`), so D-09 is correctly scoped as an assert-only regression test, not new wiring.

The one real landmine is in D-05's reindex write site. `Store.Reindex` deliberately upserts the **verbatim** source payload map (`Payload: p.Payload`) rather than round-tripping through `Memory`/`payload()` — this is a load-bearing invariant that preserves the "owner key absent = needs backfill" semantic for pre-isolation records (documented in the `Reindex` doc comment). Stamping the identity hash during reindex therefore cannot reuse the `Memory.EmbedderIdentity` + `payload()` path used by the other four write sites; it needs a direct one-key write onto the raw `map[string]*qdrant.Value` via `qdrant.NewValueString`. Additionally, `StoreAndEmbedderFromEnvNoEnsure` (the function `cmd/engram/reindex.go` uses to build the store+embedder) discards the loaded `*config.Config` after computing `dim`, so the reindex CLI currently has no path to the identity string — its signature (or a sibling helper) needs to expose it. This is a 3-callsite ripple (`reindex.go`, `retrieval_eval_test.go`, `tools_test.go`) but does not require any decision change — D-05 is correctly scoped, just needs a slightly different mechanism at the reindex site than at the other four.

**Primary recommendation:** Implement the three fixes as independent, parallelizable waves exactly as CONTEXT.md scopes them, but budget explicit tasks for the reindex identity-stamping mechanism (raw-map write + `StoreAndEmbedderFromEnvNoEnsure` signature change) — do not assume it falls out "for free" from the `Memory`/`payload()` pattern used elsewhere.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| HTTP client timeout (embed calls) | Backend / embedder client (`internal/embed`) | Config (`internal/config`) | The timeout is enforced on `http.Client` inside `embed.Client`; the operator-tunable value flows in from koanf config at construction time (`embedderFromConfig`). |
| Base-URL → embeddings-path join | Backend / embedder client (`internal/embed`) | — | Pure string transform, no I/O; belongs with the HTTP call it feeds (`embed.go`'s `embed()` method or a `New()`-time helper), not config (config only validates well-formedness of the raw URL/override). |
| Embedder-config-identity computation | Config / Store boundary (`internal/config` or `internal/store`) | — | Per D-04, must see both `EmbedConfig` (model, instruction, params) AND `Embed.Dim` — neither `embed.Client` (no dim) nor a config-only helper (no natural home for a store-payload concern) is the sole owner; a pure function taking `*config.Config` and returning a string is the right shape, callable from either layer. |
| Identity payload stamping (store/update/discovery/rule) | Store (`internal/store.Memory` + `payload()`/`fromPayload()`) | Server handlers (`internal/server/tools.go`, `rules.go`) that populate `Memory.EmbedderIdentity` before calling `Upsert`/`Update` | Follows the existing `AccessCount` precedent exactly: server sets the field on the `Memory` value, store round-trips it through the payload map. |
| Identity payload stamping (reindex) | Store (`internal/store.Reindex`) | CLI (`cmd/engram/reindex.go`) supplies the identity string | Reindex does NOT go through `Memory`/`payload()` (verbatim-payload invariant) — the stamp must be written as a direct key on the raw `map[string]*qdrant.Value`, with the identity string threaded in from the CLI's config load. |

## Standard Stack

No new third-party dependencies. All three fixes use Go stdlib only:

| Package | Purpose | Why Standard |
|---------|---------|--------------|
| `time` | Duration parsing/timeout (`time.ParseDuration`, `http.Client.Timeout`) | Already used identically for `ENGRAM_SUMMARY_TIMEOUT` |
| `crypto/sha256` | Embedder-config-identity hash (D-03) | Stdlib, no existing sha256 usage in the codebase to conflict with (`store_test.go`'s "sha256:abc" hits are unrelated test fixture strings for discovery `Citation.Pin`, not a real hash call) |
| `encoding/hex` | Render the SHA-256 digest's first 16 hex chars (D-03) | Stdlib |
| `strconv`, `net/url` (optional) | Override-URL validation for D-11, mirroring `validate.go`'s existing `ENGRAM_OPENAI_BASE_URL` check | Already imported in `internal/config/validate.go` |
| `strings` | Trailing-slash trim + suffix checks for the D-10 heuristic | Already imported in `embed.go` |

**Installation:** none — no `go.mod` changes required for this phase.

**Version verification:** N/A — no new packages.

## Package Legitimacy Audit

No external packages are installed by this phase. Every helper (`crypto/sha256`, `encoding/hex`, `time`, `strings`, `strconv`, `net/url`) is Go stdlib.

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
                 ┌─────────────────────────────┐
                 │  koanf config load           │
                 │  (registry.go + validate.go) │
                 │  ENGRAM_EMBED_TIMEOUT (new)  │
                 │  ENGRAM_OPENAI_*_URL(new,D-11)│
                 └───────────────┬───────────────┘
                                 │ *config.Config
                                 ▼
                 ┌─────────────────────────────┐
                 │ embedderFromConfig(cfg)      │◄── tools.go:306
                 │  embed.New(baseURL, key,     │
                 │    model, WithTimeout(...))  │
                 └───────────────┬───────────────┘
                                 │ *embed.Client
                 ┌───────────────┴───────────────┐
                 │  embed.New (construction time) │
                 │  joinEmbeddingsURL(baseURL,    │
                 │    override) → resolved URL    │  (D-10/D-11/D-12)
                 │  http.Client{Timeout: d}        │  (D-07/D-08)
                 └───────────────┬───────────────┘
                                 │
     store_memory / schedule_memory / update_memory / store_discovery / store_rule
                                 │
                                 ▼
                 ┌─────────────────────────────┐
                 │ d.em.Embed(ctx, EmbedText())  │──▶ POST resolvedURL
                 └───────────────┬───────────────┘
                                 │ vec []float32
                                 ▼
     ┌────────────────────────────────────────────────────┐
     │ deps.embedderIdentity (computed once at startup via │
     │ embedderIdentity(cfg), stored on deps struct)        │  (D-04)
     └───────────────────────┬────────────────────────────┘
                              │ stamped onto Memory.EmbedderIdentity
                              ▼
     store.Upsert(ctx, m, vec) ──▶ payload(m) ──▶ Qdrant point payload
                              (m.EmbedderIdentity round-trips via
                               payload()/fromPayload(), D-05/D-06)

     engram reindex (separate CLI path, does NOT reuse deps):
     StoreAndEmbedderFromEnvNoEnsure() ──▶ (st, dim, em, cfg-derived identity)
                              │
                              ▼
     Store.Reindex(opts{..., Identity: identity}, em.Embed)
        for each source point p:
           vec = embed(EmbedText(p.Content, p.Tags))
           p.Payload[embedderIdentityKey] = qdrant.NewValueString(opts.Identity)  ◄── NEW, raw-map write
           Upsert(Payload: p.Payload)   // still verbatim otherwise
```

### Recommended Project Structure

No new files required (this is a hardening phase touching existing files):

```
internal/embed/
├── embed.go          # + WithTimeout option, + joinEmbeddingsURL helper, + override field
└── embed_test.go     # + provider-shape table test (D-12), + timeout tests

internal/config/
├── config.go          # + EmbedConfig.Timeout, + OpenAIConfig (or EmbedConfig) override field (D-11)
├── registry.go         # + "embed.timeout" / ENGRAM_EMBED_TIMEOUT, + override field/env
├── validate.go          # + embed.timeout duration validation (UNGATED, unlike summarize.timeout), + override URL/well-formedness validation
└── embedparams.go        # possible home for embedderIdentity (holds JSON-param parsing already) — OR internal/store (D-04 discretion)

internal/store/
├── store.go            # + Memory.EmbedderIdentity field, + payload()/fromPayload() round-trip,
│                        #   + ReindexOptions.Identity field, + raw-map stamp write in Reindex
└── store_test.go        # + payload round-trip test, + reindex-stamps-identity test

internal/server/
├── tools.go             # + deps.embedderIdentity field, populate in buildDepsFromEnv,
│                        #   stamp on storeMemory/scheduleMemory/updateMemory (tools.go:603/642/700/932),
│                        #   + StoreAndEmbedderFromEnvNoEnsure must also expose identity/cfg to reindex.go
├── rules.go              # + stamp on storeRule
└── summaryqueue_test.go  # + regression test: maxElapsed tracks ENGRAM_SUMMARY_TIMEOUT (D-09, likely already covered by existing TestSummaryQueueMaxElapsedDerivedFromAttemptTimeout-style test — confirm/extend)

cmd/engram/
└── reindex.go           # thread the identity string into ReindexOptions
```

### Pattern 1: Functional-option timeout, mirroring `internal/summarize`

**What:** `internal/summarize.Client` already has an identical `WithTimeout` option (`d <= 0 disables it`) built on the exact same `http.Client{Timeout: defaultTimeout}` construction as `embed.Client`. This is a live, in-repo precedent for D-07 — do not invent a new shape.
**When to use:** Adding `embed.WithTimeout(d time.Duration) Option` to `internal/embed/embed.go`.
**Example:**
```go
// Source: internal/summarize/summarize.go:69-72 (existing precedent)
// WithTimeout sets the per-request HTTP client timeout. d <= 0 disables it.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.http.Timeout = d }
}
```
Applied to `embed.go`, `New()` would change from:
```go
// CURRENT — embed.go:76-82 (verified exact text)
func New(baseURL, apiKey, model string, opts ...Option) *Client {
	c := &Client{baseURL: baseURL, apiKey: apiKey, model: model, http: &http.Client{Timeout: 30 * time.Second}}
	for _, o := range opts {
		o(c)
	}
	return c
}
```
to a `defaultTimeout = 30 * time.Second` constant (mirroring `summarize.go`'s `defaultTimeout`) applied at construction, with `WithTimeout` able to override it — and `embedderFromConfig` (`tools.go:306-324`) passing `embed.WithTimeout(embedTimeout(cfg))` alongside the existing options. `WithHTTPTransport`'s doc comment ("The 30s timeout is preserved") at `embed.go:69-70` must be updated — it will no longer be universally true once the timeout is configurable (see Common Pitfalls).

### Pattern 2: koanf field + validate.go duration check, mirroring `summarize.timeout` — but UNGATED

**What:** D-08 says "mirrors `ENGRAM_SUMMARY_TIMEOUT`". The *parsing/rejection logic* should be copied verbatim; the *gating* must NOT be copied, because `summarize.timeout`'s validation only runs `if c.Summarize.Model != ""` (summarization is optional — validate.go:84-104), while the embedder is **always** active (there is no "embedder disabled" state). `embed.timeout` validation must run unconditionally, alongside the existing `embed.model`/`embed.dim` checks near the top of `Validate()`.
**When to use:** `internal/config/validate.go`, `internal/config/registry.go`, `internal/config/config.go`.
**Example:**
```go
// Source: internal/config/validate.go:97-103 (existing summarize.timeout precedent —
// copy the duration-parsing SHAPE, not the surrounding `if c.Summarize.Model != ""` gate)
switch d, err := time.ParseDuration(c.Summarize.Timeout); {
case err != nil:
	errs = append(errs, fmt.Errorf("ENGRAM_SUMMARY_TIMEOUT %q: must be a Go duration (e.g. 30s, 2m): %w", c.Summarize.Timeout, err))
case d < 0:
	errs = append(errs, fmt.Errorf("ENGRAM_SUMMARY_TIMEOUT %q: must not be negative", c.Summarize.Timeout))
}
```
New registry entry (mirrors `summarize.timeout`'s shape at `registry.go:39`):
```go
{Key: "embed.timeout", Env: "ENGRAM_EMBED_TIMEOUT", Default: "30s"},
```
New `EmbedConfig` field (`config.go`):
```go
Timeout string `koanf:"timeout"` // ENGRAM_EMBED_TIMEOUT; "0" disables the timeout (D-08)
```

### Pattern 3: Base-URL → embeddings-path join heuristic (D-10/D-12)

**What:** Replace the naive concat at `embed.go:191` (`c.baseURL+"/v1/embeddings"`) with a heuristic that recognizes an already-OpenAI-compat-rooted base URL.
**Verified provider shapes** (web search, MEDIUM confidence — official docs):
| Provider | Base URL (as configured) | Correct embeddings URL |
|----------|---------------------------|-------------------------|
| OpenRouter | `https://openrouter.ai/api/v1` | `https://openrouter.ai/api/v1/embeddings` |
| OpenAI | `https://api.openai.com/v1` | `https://api.openai.com/v1/embeddings` |
| OpenAI (bare host, no `/v1`) | `https://api.openai.com` | `https://api.openai.com/v1/embeddings` |
| Gemini OpenAI-compat | `https://generativelanguage.googleapis.com/v1beta/openai` (docs show a trailing slash: `.../v1beta/openai/`) | `https://generativelanguage.googleapis.com/v1beta/openai/embeddings` |
| Local gateway (Ollama/vLLM/LiteLLM), current default `http://localhost:4000` | no `/v1` suffix | `http://localhost:4000/v1/embeddings` (prior/default behavior, must NOT regress) |

**Example (pure helper, D-12 discretion — no per-request re-resolution):**
```go
// joinEmbeddingsURL resolves baseURL to its /embeddings endpoint. Normalizes a
// trailing slash first, then recognizes bases already rooted at an
// OpenAI-compat path ("/v1" or "/v1beta/openai") and appends only "/embeddings";
// otherwise appends the full "/v1/embeddings" (prior behavior, still correct
// for OpenAI-compat gateways with a bare host).
func joinEmbeddingsURL(baseURL string) string {
	trimmed := strings.TrimRight(baseURL, "/")
	switch {
	case strings.HasSuffix(trimmed, "/v1beta/openai"):
		return trimmed + "/embeddings"
	case strings.HasSuffix(trimmed, "/v1"):
		return trimmed + "/embeddings"
	default:
		return trimmed + "/v1/embeddings"
	}
}
```
Call site replaces `embed.go:191`'s `c.baseURL+"/v1/embeddings"` with the resolved URL (computed once at `New()` time per D-12, stored on `Client`, or resolved lazily inside `embed()` — either satisfies "resolve once, not per-request" as long as it isn't recomputed on every call... actually resolving inside `embed()` on every call IS per-request; **prefer storing the resolved URL on `Client` at `New()` time** to strictly satisfy D-12).

**Override escape hatch (D-11):** when the operator sets the override env (exact name is planner discretion, e.g. `ENGRAM_OPENAI_EMBEDDINGS_URL`), `joinEmbeddingsURL` is bypassed entirely and the override value is used verbatim as the full embeddings URL. Validate it the same way `validate.go` already validates `ENGRAM_OPENAI_BASE_URL` (scheme http/https, non-empty host) — this is a full-URL form, which is simpler to validate and simpler for an operator to reason about than a path-suffix form, and needs no interaction with `joinEmbeddingsURL` at all.

### Pattern 4: Embedder-config-identity hash (D-01 through D-04)

**What:** A pure helper over `*config.Config`, hashing model + dim + document_instruction + document_params.
**Canonical serialization pitfall:** `cfg.Embed.DocumentParams` is stored as a raw, operator-supplied JSON **string** (`config.go:64`). Hashing that raw string directly means two operationally-identical configs (`{"a":1,"b":2}` vs `{"b":2,"a":1}`) would hash to *different* identities — a false positive for "different embedding space." The canonical serialization must **parse then re-marshal** via `config.ParseEmbedParams` (already exists, already used by `embedderFromConfig`) so Go's `encoding/json` sorts map keys deterministically, before feeding the SHA-256.
**Example:**
```go
// Source: derived from D-01..D-04 + existing config.ParseEmbedParams (internal/config/embedparams.go)
// embedderIdentity computes the v1 embedder-config-identity stamp: a short,
// version-prefixed hash over the fields that change the STORED DOCUMENT vector
// (model, dim, document_instruction, document_params). Query-side fields,
// base_url, api_key, and timeout are excluded — they never alter what gets
// written for a document embed (D-01).
func embedderIdentity(cfg *Config) (string, error) {
	docParams, err := ParseEmbedParams("ENGRAM_EMBED_DOCUMENT_PARAMS", cfg.Embed.DocumentParams)
	if err != nil {
		return "", err // already validated at startup; defensive only
	}
	canonicalParams, err := json.Marshal(docParams) // nil map marshals to "null"; sorted keys
	if err != nil {
		return "", err
	}
	preimage := strings.Join([]string{
		cfg.Embed.Model, cfg.Embed.Dim, cfg.Embed.DocumentInstruction, string(canonicalParams),
	}, "\x1f") // unit separator: any of the four fields could itself contain "|" or ":"
	sum := sha256.Sum256([]byte(preimage))
	return "v1:" + hex.EncodeToString(sum[:])[:16], nil
}
```
Note: `cfg.Embed.Dim` is a `string` on `EmbedConfig` (not parsed to int at this layer — matches the existing convention that `Config` keeps values as strings where the consumer validates, per `config.go:19-21`'s doc comment), so it can be joined directly.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON-object canonicalization for the identity hash preimage | A custom sorted-key JSON serializer | `encoding/json.Marshal` on the already-`map[string]any`-typed output of `config.ParseEmbedParams` | Go's `encoding/json` already sorts map keys on marshal (documented behavior) — no custom canonicalizer needed, and reusing `ParseEmbedParams` avoids a second JSON-validation code path that could drift from the one `embedderFromConfig` already uses. |
| Override-URL validation (D-11) | A bespoke URL parser | The existing `net/url.Parse` + scheme/host check already in `validate.go:61-72` for `ENGRAM_OPENAI_BASE_URL` | Identical shape of check, already battle-tested in this file. |

**Key insight:** every helper this phase needs already has a same-shape precedent living in the codebase (`summarize.WithTimeout`, `validate.go`'s duration/URL checks, `AccessCount`'s payload round-trip). The main engineering risk is not "what pattern to invent" but "which existing pattern does NOT transfer cleanly" — see Reindex below.

## Common Pitfalls

### Pitfall 1: Reindex bypasses `Memory`/`payload()` — the D-05 stamping mechanism differs at this one write site
**What goes wrong:** A plan that assumes "stamp `Memory.EmbedderIdentity` and call `payload()`" for all five D-05 write sites will silently fail to stamp reindexed records, because `Store.Reindex` (store.go:2139-2149) constructs its `qdrant.PointStruct` with `Payload: p.Payload` — the **raw payload retrieved from the source collection**, never touched by `payload()`. This is intentional (see the doc comment at store.go:2004-2011: preserving an absent `owner` key for pre-isolation records).
**Why it happens:** The other four write sites (`store_memory`, `schedule_memory`, `update_memory`, `store_discovery`, `store_rule`) all construct a fresh or fetched `Memory` value and call `Upsert`/`Update`, which do go through `payload()`. Reindex is the outlier because it operates on payloads it never fully decodes.
**How to avoid:** Add `Identity string` to `ReindexOptions`; inside `Reindex`, after computing `vec` and before the `Upsert` call, when `opts.Identity != ""`, set `p.Payload[embedderIdentityKey] = qdrant.NewValueString(opts.Identity)` directly on the raw map (verified API: `qdrant.NewValueString(v string) *Value` exists and is the same helper family as `qdrant.NewValueMap` already used elsewhere in `store.go`). This is a single additive key-write, not a violation of the "payload preserved VERBATIM" invariant's actual purpose (owner-key absence) — but the `Reindex` doc comment claiming payload is "preserved VERBATIM" will need a one-line amendment noting this one intentional exception.
**Warning signs:** A plan that lists only `tools.go:603/642/700/932` + `store.go:2135` as "the same kind of change" without calling out that `2135` is inside `Reindex`'s loop and needs the raw-map write, not a `Memory` field.

### Pitfall 2: `StoreAndEmbedderFromEnvNoEnsure` discards `cfg`, but `reindex.go` needs the identity string
**What goes wrong:** `cmd/engram/reindex.go` calls `server.StoreAndEmbedderFromEnvNoEnsure()` which internally does `cfg, err := loadAndValidate()` then returns only `(*store.Store, uint64, *embed.Client, error)` — the loaded `cfg` is dropped (`tools.go:143-157`). Reindex has no config value to feed `embedderIdentity(cfg)`.
**Why it happens:** This function predates the identity requirement; it was built to return exactly what reindex needed at the time (store, dim, embedder).
**How to avoid:** Extend the function to also return the computed identity string (simplest: `func StoreAndEmbedderFromEnvNoEnsure() (*store.Store, uint64, *embed.Client, string, error)`), OR return `*config.Config` alongside and let the caller compute it. Either way, this is a signature change with **3 call sites to update**: `cmd/engram/reindex.go`, `internal/retrievaleval/retrieval_eval_test.go`, `internal/server/tools_test.go` (`TestStoreAndEmbedderFromEnvNoEnsureValidatesConfig`, `TestStoreAndEmbedderFromEnvNoEnsureLoadsConfigOnce`). Budget an explicit task for this — it is not a drive-by one-liner.
**Warning signs:** Tests referencing `StoreAndEmbedderFromEnvNoEnsure` failing to compile after the signature change; a plan that treats reindex identity-stamping as "same effort as the other 4 sites."

### Pitfall 3: `embed.timeout` validation must NOT copy `summarize.timeout`'s conditional gate
**What goes wrong:** `validate.go`'s `summarize.timeout` check only runs inside `if c.Summarize.Model != ""` (validate.go:84-104), because summarization is an optional feature. If a plan copies this gate verbatim for `embed.timeout`, a malformed `ENGRAM_EMBED_TIMEOUT` would only be caught when some unrelated condition is true — but the embedder is **always active** (no "embed disabled" state exists), so the check must be unconditional, alongside `embed.model`/`embed.dim` at the top of `Validate()` (validate.go:48-59).
**Why it happens:** D-08 says "mirrors `ENGRAM_SUMMARY_TIMEOUT`" — read literally that could mean copying the surrounding `if` block along with the duration-parsing logic.
**How to avoid:** Copy only the `switch d, err := time.ParseDuration(...)` shape; place it unconditionally near the existing `embed.model`/`embed.dim` checks.
**Warning signs:** A test setting `ENGRAM_EMBED_TIMEOUT=garbage` with `ENGRAM_SUMMARY_MODEL` unset passing `Validate()` when it should fail.

### Pitfall 4: `WithHTTPTransport`'s doc comment goes stale
**What goes wrong:** `embed.go:69-70`'s doc comment on `WithHTTPTransport` says "The 30s timeout is preserved" — true today (hardcoded), false once `embed.timeout` is configurable.
**Why it happens:** Easy to miss a doc-comment update when the code change is in a different function (`New`) than the comment (`WithHTTPTransport`).
**How to avoid:** Update or remove that clause as part of the D-07 change; `golangci-lint`/`revive` won't catch a doc-comment factual drift, only a human/reviewer will.
**Warning signs:** Stale doc surfaces in a future code review or godoc.

### Pitfall 5: Table-testing the join heuristic without covering the "bare host, no /v1" OpenAI shape
**What goes wrong:** D-12's four shapes + trailing-slash + override cover OpenRouter/OpenAI(`/v1`)/Gemini/trailing-slash/override, but CONTEXT.md's specifics list also calls out "OpenAI... (and bare host with no `/v1`)" — a fifth distinct shape (`https://api.openai.com` with NO path at all) that must resolve to `/v1/embeddings`, same as the local-gateway default (`http://localhost:4000`). Skipping this case in the table test would leave the existing default behavior (the whole reason the naive concat worked for the common case) unverified.
**How to avoid:** Include a bare-host-no-path case in the table test explicitly, distinct from the OpenAI-with-`/v1` case.

## Code Examples

### D-09 regression test target (verify, do not re-derive)
```go
// Source: internal/server/summaryqueue.go:97-109 (verified exact current code)
func newSummaryQueue(workers, queueSize int, attemptTimeout time.Duration, metrics *telemetry.SummaryQueueMetrics, fill func(ctx context.Context, id string) error) *summaryQueue {
	return &summaryQueue{
		ch:             make(chan string, queueSize),
		fill:           fill,
		attemptTimeout: attemptTimeout,
		maxElapsed: (attemptTimeout + summaryQueueMaxInterval) * time.Duration(summaryQueueMaxTries),
		workers:    workers,
		metrics:    metrics,
	}
}
```
`attemptTimeout` is supplied at the ONE production call site, `tools.go:229`: `q := newSummaryQueue(workers, queueSize, summaryTimeout(cfg), sqm, fill)` — `summaryTimeout(cfg)` reads `cfg.Summarize.Timeout` (`ENGRAM_SUMMARY_TIMEOUT`), never anything embed-related. A regression test already partially exists in intent at `internal/server/summaryqueue_test.go:342-349` (`attemptTimeout := 30 * time.Second; ... if want := attemptTimeout * time.Duration(summaryQueueMaxTries); q.maxElapsed < want`) — confirm this test exists and is named clearly enough to serve as the D-09 assertion, or add a small one asserting `newSummaryQueue`'s `maxElapsed` scales with whatever `attemptTimeout` value is passed (proving it is NOT a fixed 30s-derived constant).

### `Memory` payload round-trip precedent (AccessCount) — the shape to copy for `EmbedderIdentity`
```go
// Source: internal/store/store.go:308 (payload writer) — unconditional write, like access_count
p["access_count"] = m.AccessCount
// ... new line, same shape:
p["embedder_identity"] = m.EmbedderIdentity

// Source: internal/store/store.go:395-397 (fromPayload reader) — conditional read, legacy = zero value
if v, ok := p["access_count"]; ok {
	m.AccessCount = uint64(v.GetIntegerValue())
}
// ... new block, same shape (string zero value is "", matching "legacy reads empty" in D-05):
if v, ok := p["embedder_identity"]; ok {
	m.EmbedderIdentity = v.GetStringValue()
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Hardcoded 30s embed HTTP timeout | Operator-tunable `ENGRAM_EMBED_TIMEOUT`, `0` = infinite | This phase | A provider 529 brownout no longer hangs the calling MCP tool call indefinitely; matches the `ENGRAM_SUMMARY_TIMEOUT` convention already shipped in Phase 11. |
| Naive `baseURL + "/v1/embeddings"` concat | Shape-aware join recognizing `/v1` and `/v1beta/openai` roots, plus an explicit override | This phase | OpenRouter (`/v1` base) previously 404'd on `/v1/v1/embeddings`; Gemini's `/v1beta/openai` base would have the same bug. Fixed without breaking the existing local-gateway default. |
| No per-record embedding-space provenance | `v1:`-prefixed short-hash stamp on every document-embed write | This phase | Enables a FUTURE reindex-boundary audit CLI (deferred) to detect mixed-embedding-space records; this phase only stamps, does not enforce. |

**Deprecated/outdated:** none — this is additive hardening, not a migration away from a prior approach.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|----------------|
| A1 | Gemini's OpenAI-compat embeddings base URL is documented with a trailing slash (`.../v1beta/openai/`) in Google's own quickstart example, which the D-10 heuristic's "trim trailing slash first" step already handles correctly | Pattern 3 / Common Pitfall 5 | Low — this is CITED (Google AI for Developers docs, official source), not ASSUMED; if Google changes the path shape the table test would need a new case, not a heuristic redesign. |
| A2 | The exact override env var name and full-URL-vs-path form (D-11) is left to planner discretion per CONTEXT.md; this research recommends a full-URL form (`ENGRAM_OPENAI_EMBEDDINGS_URL`) validated the same way as `ENGRAM_OPENAI_BASE_URL` | Pattern 3 | Low-medium — purely a naming/shape choice already explicitly delegated to the planner by CONTEXT.md; no functional risk either way as long as it's validated and documented. |
| A3 | `StoreAndEmbedderFromEnvNoEnsure`'s signature should grow a 5th return value (identity string) rather than returning `*config.Config` | Common Pitfall 2 | Low — either shape works; returning the pre-computed identity string keeps `embedderIdentity` a config-package-internal concern and avoids exporting more of `*config.Config` than reindex needs. Planner discretion. |

**If this table is empty:** N/A — see rows above; all are low-risk naming/shape choices already flagged as discretion in CONTEXT.md, not open technical unknowns.

## Open Questions

1. **Does an existing `summaryqueue_test.go` test already satisfy D-09's "regression test that maxElapsed tracks ENGRAM_SUMMARY_TIMEOUT" requirement, or does a new one need to be added?**
   - What we know: `summaryqueue_test.go:342-349` contains a test using a hardcoded `attemptTimeout := 30 * time.Second` and asserting `q.maxElapsed >= attemptTimeout * maxTries`. This proves `maxElapsed` scales with `attemptTimeout`, which is the load-bearing property D-09 needs.
   - What's unclear: Whether this test's *name* and *doc comment* explicitly tie it to "no embed-timeout coupling" (the D-09 framing) or whether it needs a sibling test/rename to make that framing explicit for future readers.
   - Recommendation: Planner should treat this as "confirm and possibly rename/extend an existing test" rather than "write a new test from scratch" — low effort either way, worth a single explicit task so it isn't skipped.

2. **Where exactly should `embedderIdentity(cfg)` live — `internal/config` or `internal/store`?**
   - What we know: D-04 says "in the config or store layer" explicitly leaving this open; `internal/config/embedparams.go` already hosts the closest-shaped helper (`ParseEmbedParams`), which `embedderIdentity` would call.
   - What's unclear: `internal/store` cannot import `internal/config` cleanly if there's a reverse dependency risk — need to check import direction.
   - Recommendation: Placing it in `internal/config` (new file `internal/config/identity.go`, or appended to `embedparams.go`) is the more natural fit since it only needs `*Config` + stdlib and can be called from both `internal/server` (deps construction) and `cmd/engram` (reindex) without a store-layer import. Verified: `internal/store` does not currently import `internal/config` (confirmed via `store.go`'s import block), so placing it in `store` would be a NEW cross-package dependency — `internal/config` is the lower-friction placement.

## Environment Availability

Skipped — this phase has no external runtime dependencies beyond what's already required to build/test the repo (Go 1.26.3 toolchain, already verified present via `go.mod`). No new services, CLIs, or databases are introduced.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `testify` v1.11.1 |
| Config file | none — `go test` via `Taskfile.yaml` |
| Quick run command | `go test ./internal/embed/... ./internal/config/... ./internal/store/... ./internal/server/...` |
| Full suite command | `task test` (= `go test ./...` + python hook tests) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|---------------------|--------------|
| REQ-embed-timeout | `ENGRAM_EMBED_TIMEOUT` overrides the client timeout; `0` = infinite; negative rejected | unit | `go test ./internal/config/... -run TestValidate -v` | ✅ `internal/config/validate_test.go` (extend with embed.timeout cases) |
| REQ-embed-timeout | A slow/hung embedder call is cut short by the configured timeout | unit (httptest server + `time.Sleep`) | `go test ./internal/embed/... -run TestEmbedTimeout -v` | ❌ Wave 0 — mirror `internal/summarize/summarize_test.go:199` (`TestSummarizeWithTimeoutCancelsSlowRequest`) |
| REQ-embed-timeout | D-09 invariant: `maxElapsed` derives from `attemptTimeout` (`ENGRAM_SUMMARY_TIMEOUT`), independent of embed timeout | unit/regression | `go test ./internal/server/... -run TestSummaryQueue -v` | ✅ `internal/server/summaryqueue_test.go` (confirm/rename existing coverage, see Open Question 1) |
| REQ-embed-baseurl-join | Provider-shape table test: OpenRouter, OpenAI(`/v1`), OpenAI(bare host), trailing-slash, Gemini, override | unit (table-driven) | `go test ./internal/embed/... -run TestJoinEmbeddingsURL -v` | ❌ Wave 0 — new test in `embed_test.go` |
| REQ-embed-baseurl-join | Override escape hatch wins over heuristic when set; validated at config load | unit | `go test ./internal/config/... -run TestValidate -v` | ❌ Wave 0 — extend `validate_test.go` |
| REQ-embed-config-identity | `embedderIdentity(cfg)` is deterministic and excludes query-side/base_url/api_key/timeout fields (D-01) | unit (table-driven, pure function) | `go test ./internal/config/... -run TestEmbedderIdentity -v` | ❌ Wave 0 — new test |
| REQ-embed-config-identity | `Memory.EmbedderIdentity` round-trips through `payload()`/`fromPayload()`; legacy record (missing key) reads `""` | unit | `go test ./internal/store/... -run TestPayload -v` | ✅ pattern exists for AccessCount in `store_test.go` — extend |
| REQ-embed-config-identity | Every document-embed write site (store/update/discovery/rule/reindex) stamps the current identity | integration (testcontainers-qdrant) | `go test ./internal/server/... -run TestStore -v` and `go test ./internal/store/... -run TestReindex -v` | ✅ existing integration test scaffolding in `store_test.go`/`tools_test.go` — extend |
| REQ-embed-config-identity | `recallView` does NOT surface the identity field (D-06 negative-space check) | unit | `go test ./internal/server/... -run TestRecallView -v` | ❌ Wave 0 — small negative-assertion test |

### Sampling Rate
- **Per task commit:** `go test ./internal/embed/... ./internal/config/... ./internal/store/... ./internal/server/...`
- **Per wave merge:** `task test`
- **Phase gate:** `task` (lint + test) green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/embed/embed_test.go` — `TestJoinEmbeddingsURL` table test (D-12) covering OpenRouter/OpenAI(`/v1`)/OpenAI(bare host)/trailing-slash/Gemini/override
- [ ] `internal/embed/embed_test.go` — `TestEmbedTimeout...` mirroring `summarize_test.go:199`'s slow-request-cancellation pattern
- [ ] `internal/config/validate_test.go` — embed.timeout duration cases (mirror `summarize.timeout` cases at `validate_test.go:95-96`, but UNGATED per Common Pitfall 3)
- [ ] `internal/config/validate_test.go` or new `internal/config/identity_test.go` — `embedderIdentity(cfg)` determinism + field-exclusion table test
- [ ] `internal/server/summary_test.go` (or similar) — negative assertion that `recallView`/`toRecallView` never surfaces the identity field (D-06)
- [ ] `internal/store/store_test.go` — reindex-stamps-identity integration test (requires the `ReindexOptions.Identity` field to exist first)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|----------------|---------|--------------------|
| V2 Authentication | no | Phase does not touch auth/session — no OIDC/token paths modified. |
| V3 Session Management | no | Out of scope. |
| V4 Access Control | no | No new authz surface — the identity stamp is a payload-only field with no read/write gate change. |
| V5 Input Validation | yes | `ENGRAM_EMBED_TIMEOUT` (Go duration parse, reject negative — mirrors existing pattern) and the D-11 override URL/path (mirrors existing `ENGRAM_OPENAI_BASE_URL` scheme/host validation) both need the same fail-closed validation rigor already applied to sibling fields in `validate.go`. |
| V6 Cryptography | yes (light) | SHA-256 via stdlib `crypto/sha256` for the identity hash — this is a non-secret, non-adversarial content-addressing use (equality/grouping only, explicitly "opaque is fine" per D-03), NOT a security control; no key material, no HMAC needed. Do not hand-roll a hash function; stdlib is correct here. |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|------------------------|
| SSRF via operator-supplied override URL (D-11) — an operator (not an external attacker; this is a trusted-operator config value, same trust level as `ENGRAM_OPENAI_BASE_URL` already) pointing the embedder at an internal service | Tampering (low severity — same trust boundary as the existing base-URL config) | No new mitigation needed beyond what `ENGRAM_OPENAI_BASE_URL` already has (scheme/host well-formedness check only) — this is operator-trusted config, not user input; consistent with the existing `ENGRAM_OPENAI_BASE_URL` threat model (unchanged by this phase). |
| Unbounded embed timeout (`0` = infinite, D-08) reintroducing the exact hang this phase fixes, if operator-misconfigured | Denial of Service (self-inflicted, operator choice) | Documented as the explicit "operator escape hatch for very slow local models" (D-08) — same accepted trade-off already shipped for `ENGRAM_SUMMARY_TIMEOUT=0`. No new mitigation; consistency with existing precedent is the correct answer. |

## Sources

### Primary (HIGH confidence)
- `internal/embed/embed.go` (read in full) — exact current timeout literal (line 77), naive URL join (line 191), `Option` pattern (lines 45-73)
- `internal/server/summaryqueue.go` (read in full) — confirmed no `30 * time.Second` literal; `maxElapsed` derivation (line 105)
- `internal/store/summarize.go` (read in full) — `FillSummary`/`SummarizeMissing` context for the summary-queue independence claim
- `internal/config/validate.go`, `internal/config/config.go`, `internal/config/registry.go`, `internal/config/embedparams.go` (all read in full) — exact `summarize.timeout` validation block, `EmbedConfig`/`OpenAIConfig` shapes, registry field pattern
- `internal/server/tools.go` (relevant sections read) — `embedderFromConfig` (line 306), write sites (603/642/700/932), `StoreAndEmbedderFromEnvNoEnsure` (line 143), `summaryTimeout` (line 292)
- `internal/store/store.go` (relevant sections read) — `Memory` struct, `payload()`/`fromPayload()`, `Reindex` (including its "payload preserved VERBATIM" doc comment), `ReindexOptions`
- `internal/server/rules.go` — `storeRule` write site
- `internal/server/summary.go` — `recallView` allowlist (confirmed hand-written, D-06 target)
- `cmd/engram/reindex.go` — confirmed `StoreAndEmbedderFromEnvNoEnsure` call site and its 2 other callers (via grep)
- `internal/summarize/summarize.go` — `WithTimeout` precedent (lines 69-72), `defaultTimeout` constant pattern
- `go doc github.com/qdrant/go-client/qdrant.PointStruct` / `.NewValueString` / `.NewValueMap` — confirmed the raw-map stamping API for the reindex landmine

### Secondary (MEDIUM confidence)
- [Gemini OpenAI compatibility docs](https://ai.google.dev/gemini-api/docs/openai) — confirmed `/v1beta/openai` base URL and trailing-slash example
- [OpenRouter Quickstart](https://openrouter.ai/docs/quickstart) — confirmed `/api/v1` base URL and `/embeddings` endpoint shape

### Tertiary (LOW confidence)
- none

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - no new dependencies; every helper is stdlib with an in-repo precedent already read
- Architecture: HIGH - every referenced line number and function signature was read directly from the current source, not inferred
- Pitfalls: HIGH - the reindex landmine (Pitfall 1) and `StoreAndEmbedderFromEnvNoEnsure` gap (Pitfall 2) were found by reading `Reindex`'s actual implementation and doc comment, not assumed from the CONTEXT's line hints

**Research date:** 2026-07-10
**Valid until:** 30 days (stable internal codebase; the only external-facing claims — provider URL shapes — are pinned to official docs checked this session)
