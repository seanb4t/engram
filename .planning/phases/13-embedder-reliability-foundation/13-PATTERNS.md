# Phase 13: Embedder Reliability Foundation - Pattern Map

**Mapped:** 2026-07-10
**Files analyzed:** 10 (new/modified)
**Analogs found:** 10 / 10

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `internal/embed/embed.go` (`WithTimeout` option) | service (HTTP client) | request-response | `internal/summarize/summarize.go` `WithTimeout` (lines 69-72) | exact |
| `internal/embed/embed.go` (`joinEmbeddingsURL` helper) | utility (pure transform) | transform | `internal/embed/embed.go:191` (existing naive concat call site) | role-match (new pure fn, no direct precedent) |
| `internal/config/config.go` (`EmbedConfig.Timeout`, override field) | config (struct field) | CRUD (config load) | `internal/config/config.go` `SummarizeConfig.Timeout` field | exact |
| `internal/config/registry.go` (`embed.timeout` entry, override env entry) | config (field registry) | CRUD (config load) | `registry.go` `summarize.timeout` entry (~line 39) | exact |
| `internal/config/validate.go` (`embed.timeout` validation — UNGATED; override URL validation) | config (validator) | CRUD (config load) | `validate.go` `summarize.timeout` block (~97-103) for duration shape; `validate.go:61-72` (`ENGRAM_OPENAI_BASE_URL` check) for override URL validation | exact (shape only — gating diverges, see below) |
| `internal/config/embedparams.go` (`embedderIdentity(cfg)` helper) | utility (pure hash fn) | transform | `internal/config/embedparams.go` `ParseEmbedParams` (same file, called by the new helper) | role-match (new pure fn, colocated with closest sibling) |
| `internal/store/store.go` (`Memory.EmbedderIdentity` field + payload round-trip, 4 write sites) | model / store (payload codec) | CRUD | `internal/store/store.go` `AccessCount`/`LastAccessedAt` payload precedent (writer ~:308, reader ~:395-397) | exact |
| `internal/store/store.go` (`Reindex` raw-map stamp, ~:2135) | store (batch/migration) | batch | `internal/store/store.go` `Reindex`'s existing verbatim-payload upsert (its own code, NOT the `Memory`/`payload()` path) | **role-match, DIFFERENT mechanism** — see Divergent Mechanisms below |
| `internal/server/tools.go` (`deps.embedderIdentity`, stamp at 4 write sites: `store_memory`/`schedule_memory`/`update_memory`/`store_discovery`; `StoreAndEmbedderFromEnvNoEnsure` signature change) | controller (MCP tool handlers) | request-response | `internal/server/tools.go` existing `AccessCount`-style field population before `Upsert`/`Update` calls at the same call sites | exact |
| `internal/server/rules.go` (stamp on `storeRule`) | controller (MCP tool handler) | request-response | same `Memory`-field-then-`Upsert` pattern as `tools.go` write sites | exact |
| `cmd/engram/reindex.go` (thread identity into `ReindexOptions`) | CLI command | batch | `cmd/engram/reindex.go`'s existing call to `StoreAndEmbedderFromEnvNoEnsure` + `Store.Reindex` | exact (signature change ripples here) |

## Pattern Assignments

### `internal/embed/embed.go` — `WithTimeout` option + timeout wiring (service, request-response)

**Analog:** `internal/summarize/summarize.go:69-72`

**Core pattern to copy verbatim (functional option shape):**
```go
// Source: internal/summarize/summarize.go:69-72
// WithTimeout sets the per-request HTTP client timeout. d <= 0 disables it.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.http.Timeout = d }
}
```

**Current code being replaced** (`embed.go:76-82`, verified exact):
```go
func New(baseURL, apiKey, model string, opts ...Option) *Client {
	c := &Client{baseURL: baseURL, apiKey: apiKey, model: model, http: &http.Client{Timeout: 30 * time.Second}}
	for _, o := range opts {
		o(c)
	}
	return c
}
```
Follow `embed.go:45-73`'s existing `Option` type/pattern (`WithQueryParams`, `WithDocumentInstruction`, `WithHTTPTransport`) for placement of the new `WithTimeout`. Introduce a `defaultTimeout = 30 * time.Second` constant (mirrors `summarize.go`'s own `defaultTimeout` constant) so `New()`'s default is named, not a magic literal.

**Caller wiring:** `internal/server/tools.go:306-324` (`embedderFromConfig`) must pass `embed.WithTimeout(embedTimeout(cfg))` alongside existing options — same call-site shape as however `WithDocumentInstruction`/`WithQueryParams` are already threaded there.

**Stale doc-comment landmine:** `embed.go:69-70`'s `WithHTTPTransport` doc comment states "The 30s timeout is preserved" — this becomes false once timeout is configurable; update/remove that clause in the same change.

---

### `internal/embed/embed.go` — `joinEmbeddingsURL` helper (utility, transform)

**Analog:** none exact (new pure function); nearest existing code is the call site it replaces, `embed.go:191` (`c.baseURL+"/v1/embeddings"` naive concat).

**Target implementation** (already fully specified by research, D-10/D-12):
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
Resolve **once** at `New()`-construction time (store the resolved URL on `Client`), not per-request inside `embed()` — per D-12's explicit "resolve once" requirement. `strings` is already imported in `embed.go`.

**Override escape hatch (D-11):** when the new override env/config value is set, it wins and bypasses `joinEmbeddingsURL` entirely (full-URL form recommended by research, validated the same way as `ENGRAM_OPENAI_BASE_URL`).

**Table test target:** `internal/embed/embed_test.go` — `TestJoinEmbeddingsURL`, must cover 6 cases: OpenRouter (`/api/v1`), OpenAI (`/v1`), OpenAI bare host (no path — distinct from the `/v1` case, do not conflate, see Pitfall 5), trailing-slash variant, Gemini (`/v1beta/openai`), override path.

---

### `internal/config/*` — `embed.timeout` field, registry, validation (config, CRUD)

**Analog:** `summarize.timeout` (`internal/config/config.go`, `registry.go`, `validate.go:~97-103`)

**Registry entry to mirror** (`registry.go` shape):
```go
{Key: "embed.timeout", Env: "ENGRAM_EMBED_TIMEOUT", Default: "30s"},
```

**Config struct field to mirror** (`config.go`):
```go
Timeout string `koanf:"timeout"` // ENGRAM_EMBED_TIMEOUT; "0" disables the timeout (D-08)
```

**Validation shape to copy — SHAPE ONLY, not the gate** (`validate.go:97-103`, verified exact):
```go
switch d, err := time.ParseDuration(c.Summarize.Timeout); {
case err != nil:
	errs = append(errs, fmt.Errorf("ENGRAM_SUMMARY_TIMEOUT %q: must be a Go duration (e.g. 30s, 2m): %w", c.Summarize.Timeout, err))
case d < 0:
	errs = append(errs, fmt.Errorf("ENGRAM_SUMMARY_TIMEOUT %q: must not be negative", c.Summarize.Timeout))
}
```

**CRITICAL DIVERGENCE (Pitfall 3):** `summarize.timeout`'s validation only runs inside `if c.Summarize.Model != ""` (validate.go:84-104) because summarization is optional. `embed.timeout` validation is **UNGATED** — the embedder is always active. Place the copied switch statement unconditionally, alongside the existing `embed.model`/`embed.dim` checks near the top of `Validate()` (validate.go:48-59). Do NOT wrap it in any conditional.

**Override URL validation analog:** `validate.go:61-72` (existing `ENGRAM_OPENAI_BASE_URL` scheme/host check) — reuse the same `net/url.Parse` + scheme/host-non-empty shape for the D-11 override value.

---

### `internal/config/embedparams.go` — `embedderIdentity(cfg)` helper (utility, transform)

**Analog:** same file's existing `ParseEmbedParams` (which the new helper calls directly).

**Target implementation** (fully specified, D-01–D-04):
```go
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
	}, "\x1f") // unit separator
	sum := sha256.Sum256([]byte(preimage))
	return "v1:" + hex.EncodeToString(sum[:])[:16], nil
}
```
Placement: `internal/config` (new `identity.go` or appended to `embedparams.go`) — verified `internal/store` does NOT currently import `internal/config`, so `internal/config` is the lower-friction placement (avoids a new cross-package dependency), callable from both `internal/server` (deps construction) and `cmd/engram` (reindex).

**Canonicalization pitfall:** `cfg.Embed.DocumentParams` is a raw operator-supplied JSON string — hash the raw string directly and two operationally-identical configs with different key order produce different hashes (false positive). Must parse via `ParseEmbedParams` then re-`json.Marshal` (Go sorts map keys on marshal) before hashing.

---

### `internal/store/store.go` — `Memory.EmbedderIdentity` field + payload round-trip (model/store, CRUD)

**Analog:** `AccessCount`/`LastAccessedAt` payload precedent, same file.

**Writer shape to copy** (store.go ~:308, verified exact — unconditional write):
```go
p["access_count"] = m.AccessCount
// new line, same shape:
p["embedder_identity"] = m.EmbedderIdentity
```

**Reader shape to copy** (store.go ~:395-397, verified exact — conditional read, legacy = zero value):
```go
if v, ok := p["access_count"]; ok {
	m.AccessCount = uint64(v.GetIntegerValue())
}
// new block, same shape (string zero value "" == "legacy reads empty" per D-05):
if v, ok := p["embedder_identity"]; ok {
	m.EmbedderIdentity = v.GetStringValue()
}
```
Applies to the 4 write sites that go through `Memory`/`payload()`: `store_memory`, `schedule_memory`, `update_memory` (re-embed), `store_discovery`, `store_rule` — server handlers (`tools.go:603/642/700/932`, `rules.go`) set `Memory.EmbedderIdentity` before calling `Upsert`/`Update`; the store layer round-trips it automatically via this existing codec, no new store-layer logic needed beyond the two blocks above.

---

### `internal/store/store.go` `Reindex` (~:2135) — DIVERGENT MECHANISM (store, batch)

**Analog:** NOT the `Memory`/`payload()` precedent above. `Reindex` upserts the source payload **verbatim** (`Payload: p.Payload`) — never constructs or reads through `Memory`. This is a load-bearing invariant (preserves absent `owner` key for pre-isolation records; see `Reindex`'s doc comment at store.go:2004-2011).

**Mechanism:** direct raw-map key write, using the same `qdrant.NewValueString` helper family already used elsewhere in `store.go` (e.g. `qdrant.NewValueMap`):
```go
// Inside Reindex's per-point loop, after computing vec, before Upsert:
if opts.Identity != "" {
	p.Payload[embedderIdentityKey] = qdrant.NewValueString(opts.Identity)
}
```
Add `Identity string` to `ReindexOptions`. Amend the `Reindex` doc comment's "preserved VERBATIM" claim with a one-line note about this single intentional additive-key exception.

**Ripple — `StoreAndEmbedderFromEnvNoEnsure` signature change:** this function (`tools.go:143-157`) currently discards the loaded `*config.Config` after computing `dim`, so reindex has no path to `embedderIdentity(cfg)`. Extend its return signature (research recommends a 5th return value, the pre-computed identity string) — this has **3 call sites to update**: `cmd/engram/reindex.go`, `internal/retrievaleval/retrieval_eval_test.go`, `internal/server/tools_test.go`. Do not treat this as a drive-by one-liner; budget it as its own task.

**Do not tell the executor** to reuse `Memory.EmbedderIdentity` + `payload()` at this site — it is architecturally incompatible with `Reindex`'s verbatim-copy contract.

---

### `internal/server/tools.go` / `rules.go` — stamp at write sites (controller, request-response)

**Analog:** existing `Memory`-field-population-before-`Upsert` pattern already present at each of these call sites (same shape as however `AccessCount` or other server-set fields are populated before the store call).

**Pattern:** at `tools.go:603` (`store_memory`), `:642` (`schedule_memory`), `:700` (`update_memory` re-embed branch), `:932` (`store_discovery`), and the equivalent site in `rules.go` (`store_rule`), set:
```go
m.EmbedderIdentity = deps.embedderIdentity
```
before the `Upsert`/`Update` call. `deps.embedderIdentity` is computed once via `embedderIdentity(cfg)` at deps-construction time (wherever `embedderFromConfig` / `buildDepsFromEnv` already runs) and stored on the `deps` struct — not recomputed per-request.

---

## Shared Patterns

### Functional-option construction (`embed.Client`, `summarize.Client`)
**Source:** `internal/summarize/summarize.go` `Option` type + `WithTimeout`
**Apply to:** `internal/embed/embed.go`'s new `WithTimeout`

### koanf field + validate.go duration/URL check
**Source:** `internal/config/validate.go` (`summarize.timeout` duration shape at ~97-103; `ENGRAM_OPENAI_BASE_URL` URL shape at ~61-72)
**Apply to:** `embed.timeout` (UNGATED — divergence, see Pitfall 3) and the D-11 override URL validation

### Memory payload round-trip (server-set, legacy-missing-reads-zero-value)
**Source:** `internal/store/store.go` `AccessCount`/`LastAccessedAt` writer/reader pair
**Apply to:** `Memory.EmbedderIdentity` at the 4 non-reindex write sites only — NOT `Reindex` (see Divergent Mechanisms)

### `qdrant.NewValueString` raw-map payload write
**Source:** `internal/store/store.go` (existing usage alongside `qdrant.NewValueMap` elsewhere in the file)
**Apply to:** `Reindex`'s identity stamp only

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/embed/embed.go` `joinEmbeddingsURL` | utility | transform | No existing pure-string-transform helper in `internal/embed`; treat as a new function, structured per D-10's spec in RESEARCH.md Pattern 3 |
| `internal/config/embedparams.go` `embedderIdentity` | utility | transform | No existing hash/identity helper in the codebase (only unrelated test-fixture strings matching `sha256:` in `store_test.go`'s `Citation.Pin` fixtures — not a real precedent) |

## Metadata

**Analog search scope:** `internal/embed/`, `internal/config/`, `internal/store/`, `internal/server/`, `cmd/engram/`, `internal/summarize/` (cross-cutting `WithTimeout` precedent)
**Files scanned:** RESEARCH.md's Sources section lists 12 files read in full or in relevant part by the researcher; this pass consumed those citations directly (line numbers/excerpts already verified against live source) rather than re-reading, per the no-redundant-reads directive.
**Pattern extraction date:** 2026-07-10
**Note:** All code excerpts above are carried forward from RESEARCH.md, which explicitly verified them against the current repository source (not inferred). Executors should still `Read` the exact target file before editing, since line numbers will shift as earlier waves land changes.
</content>
</invoke>
