# Phase 2: Decision Interface & Jev Backend - Pattern Map

**Mapped:** 2026-09-23
**Files analyzed:** 12
**Analogs found:** 12 / 12

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `internal/decide/decide.go` (new) | model/interface (pure Go contract, no I/O) | transform | `internal/embed/embed.go` (public surface shape: Option pattern, named consts) + `internal/config/config.go` (struct/koanf doc-comment style for typed fields) | role-match |
| `internal/decide/decide_test.go` (new) | test | transform | `internal/embed/embed_test.go` (table-driven unit tests over a pure function/interface) | role-match |
| `internal/decide/jev/jev.go` (new) | service (outbound HTTP provider client) | request-response | `internal/embed/embed.go` (New+Option client, bounded HTTP call, error classification, span) | exact |
| `internal/decide/jev/jev_test.go` (new) | test | request-response | `internal/embed/embed_test.go` (httptest fixtures, `TestEmbedEmitsSpan`, non-2xx body assertions) | exact |
| `internal/decide/jev/spike_evaluation_test.go` (new, Wave 0) | test (SDK evaluation spike) | request-response | No direct analog — closest is `internal/embed/embed_test.go`'s httptest-fixture-and-assert shape, replayed against the OpenRouter SDK instead of hand-rolled `net/http` | no analog (see below) |
| `internal/config/registry.go` (modified) | config | CRUD (config field registration) | itself — `embed.*`/`summarize.*` rows (lines 34-93) are the direct precedent for the new `decisions.*` rows | exact |
| `internal/config/config.go` (modified) | config/model | transform | `EmbedConfig`/`SummarizeConfig` structs (lines 62-107, 142-159) | exact |
| `internal/config/validate.go` (modified) | config | transform | `Config.Validate()`'s `Summarize.Model`-gated block (lines 238-281) and `OpenAI.BaseURL`/`ChatBaseURL` URL-shape checks (lines 138-180) | exact |
| `internal/config/decisions_test.go` or extended `providerbounds_test.go` (new/modified) | test | transform | `internal/config/providerbounds_test.go` (table-driven registry-shape + Validate() assertions) | exact |
| `internal/server/tools.go` (modified — add `deciderFromConfig`) | service (wiring seam) | request-response | `embedderFromConfig`/`summarizerFromConfig` (lines 602-663) | exact |
| `charts/engram/values.yaml` (modified) | config | CRUD | `memory.summarize` block (lines ~100-116) | exact |
| `charts/engram/templates/_helpers.tpl` (modified) | config/template | transform | `engram.containerEnv`'s `memory.summarize.*`/`memory.openai.apiKeySecret` blocks (lines 1-46) | exact |
| `docs-site/src/content/docs/guides/configure.md` (modified) | config/docs | transform | `## Auto-summary` section (lines 66-150) | exact |
| `CLAUDE.md` (modified — Layout table row, optional) | config/docs | transform | existing `internal/embed/` row | exact |

## Pattern Assignments

### `internal/decide/decide.go` (interface + types, pure Go, no I/O)

**Analog:** `internal/embed/embed.go` (public-surface conventions) — no existing pure-interface
package in this repo to mirror structurally (D-08 is genuinely new shape), so this borrows only
the *conventions* (SPDX header, doc-comment density, exported-error-var naming), not a structural
template.

**Package doc-comment convention** (`internal/embed/embed.go:1-6`):
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package embed produces text embeddings via an OpenAI-compatible
// /v1/embeddings endpoint (e.g. Ollama, vLLM, or a LiteLLM gateway).
package embed
```
Mirror this exactly for `package decide`: SPDX header, one-paragraph doc comment naming what the
package is and is not (e.g. "Package decide defines a provider-neutral, advisory-only typed-
decision contract in System One vocabulary... Never calls out over the network itself; backends
under internal/decide/* do that.").

**Named-error convention** (no direct analog in embed/summarize — both return bare `fmt.Errorf`
wrapped strings, never `errors.Is`-able sentinels). The closest sentinel-error convention in this
codebase is `internal/config/validate.go`'s aggregation via `errors.Join` (see Shared Patterns
below) plus Go stdlib idiom. Write named errors as package-level `var Err... = errors.New(...)`
(D-12's seven errors: `ErrDecisionAuth`, `ErrDecisionBadRequest`, `ErrDecisionContextTooLarge`,
`ErrDecisionRateLimited`, `ErrDecisionUnavailable`, `ErrDecisionTimeout`, plus the shared
response-too-large sentinel), each with a doc comment naming the HTTP status class it maps to
(D-12) — no codebase precedent to copy verbatim; this is genuinely new but should read like the
rest of this file's dense "why" comments (see embed.go's `WithDrainBytes` comment style,
lines 192-201, as the tone/density bar).

**Interface doc-comment obligation (D-12's caller contract):** state explicitly on the `Decider`
interface's doc comment that "a decision failure never fails the surrounding read or sweep" —
this is a design invariant the interface itself must carry (RESEARCH.md's Security Domain table),
not something to leave implicit.

---

### `internal/decide/jev/jev.go` (Jev backend — provider HTTP client)

**Analog:** `internal/embed/embed.go` (full file read, verbatim pattern to mirror)

**Imports pattern** (`internal/embed/embed.go:1-26`):
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package embed produces text embeddings via an OpenAI-compatible
// /v1/embeddings endpoint (e.g. Ollama, vLLM, or a LiteLLM gateway).
package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/seanb4t/engram/internal/config"
	"github.com/seanb4t/engram/internal/httpdrain"
	"github.com/seanb4t/engram/internal/openaiurl"
	"github.com/seanb4t/engram/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("github.com/seanb4t/engram/internal/embed")
```
For `internal/decide/jev`, use `otel.Tracer("github.com/seanb4t/engram/internal/decide/jev")`
(D-13) and import `internal/decide` (for `Decider`/`Question`/`Answer`/named errors) instead of
`internal/openaiurl`/`internal/config` — Jev's endpoint is `{base}/alpha/decisions`, a fixed
suffix appended to `ENGRAM_DECISIONS_BASE_URL` verbatim (D-03), not the shape-aware
`openaiurl.Join` heuristic (that heuristic is OpenAI-`/v1`-specific and D-03 explicitly says the
chat base URL must NOT be assumed to serve Decisions).

**Client struct + New()+Option constructor pattern** (`internal/embed/embed.go:65-106, 214-257`):
```go
// Client embeds text via an OpenAI-compatible embeddings API.
type Client struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
	maxResponseBytes int64
	drainBytes   int64
	drainTimeout time.Duration
	maxTimeout time.Duration
}

// New returns an embedding Client for the given base URL, API key, and model.
func New(baseURL, apiKey, model string, opts ...Option) *Client {
	c := &Client{
		baseURL: baseURL, apiKey: apiKey, model: model,
		http:         &http.Client{Timeout: defaultEmbedTimeout},
		drainBytes:   defaultDrainBytes,   // set BEFORE options loop (D-05/D-06)
		drainTimeout: defaultDrainTimeout, // set BEFORE options loop (D-05/D-06)
	}
	for _, o := range opts {
		o(c)
	}
	if c.maxResponseBytes <= 0 {
		c.maxResponseBytes = defaultMaxResponseBytes
	}
	if c.maxTimeout <= 0 {
		c.maxTimeout = defaultMaxTimeout
	}
	if c.http.Timeout <= 0 {
		c.http.Timeout = c.maxTimeout // non-positive Timeout resolves to ceiling, never unbounded
	}
	return c
}
```
Copy this exactly for `jev.New(baseURL, apiKey, model string, opts ...jev.Option) *jev.Client`
with `WithConcurrency` added (D-10's worker-pool bound for `DecideMany`) and `concurrency int`
added to the struct, following `defaultDecideConcurrency = 4` (D-10's measured value) as the
struct-literal-set-before-loop default (mirrors `drainBytes`/`drainTimeout`, NOT the post-loop
`maxTimeout` convention — concurrency has no "0 means fall back to ceiling" semantics, so decide
at implementation time which convention fits; RESEARCH.md's Pattern 1 already documents the
distinction between the two defaulting conventions).

**Core HTTP-call + bounded-drain + error pattern** (`internal/embed/embed.go:348-422`, the `embed`
method — mirror this shape for `jev.Decide`):
```go
func (c *Client) embed(ctx context.Context, text string, params map[string]any, kind string) (vec []float32, err error) {
	ctx, span := tracer.Start(ctx, "embed.Embed", trace.WithAttributes(
		attribute.String("engram.embed.model", c.model),
		attribute.String("engram.embed.kind", kind),
	))
	defer span.End()
	// ... build request, req.Header.Set("Authorization", "Bearer "+c.apiKey) ...
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		httpdrain.Drain(resp.Body, c.drainBytes, c.drainTimeout) // bounded by bytes and time (D-01, D-02)
		return nil, fmt.Errorf("embeddings: status %d: %s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}
	var out embedResp
	if err := json.NewDecoder(io.LimitReader(resp.Body, c.maxResponseBytes)).Decode(&out); err != nil {
		return nil, err
	}
	httpdrain.Drain(resp.Body, c.drainBytes, c.drainTimeout) // bounded by bytes and time (D-01, D-02)
	return out.Data[0].Embedding, nil
}
```
**Divergence jev.Decide MUST make from this template (D-06 checkpoint, RESEARCH.md Common
Pitfall #1/#2):** if the OpenRouter SDK is adopted, `jev.Decide` does NOT read `resp.Body`
directly — the SDK's `Alpha.Decisions.Create` owns the decode and returns a typed
`*components.DecisionsResponse`. The `maxErrorBodyBytes`/`io.LimitReader`/`httpdrain.Drain` calls
above move into a custom `http.RoundTripper` (wrapping `resp.Body` before handing it back to the
SDK) instead of living inline in `Decide`. Error classification (D-12) must happen off
`resp.StatusCode` at the transport layer, not by relying on the SDK's typed `sdkerrors.*` structs
decoding cleanly — see the D-06 Wave-0 spike task before writing this.

**Span-attribute pattern** (`internal/embed/embed.go:349-363`, adapt attribute names per D-13):
```go
ctx, span := tracer.Start(ctx, "embed.Embed", trace.WithAttributes(
	attribute.String("engram.embed.model", c.model),
))
defer span.End()
defer func() {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetAttributes(attribute.Int("engram.embed.dims", len(vec)))
	}
}()
```
For `jev.Decide`, span name `"decide"` (D-13 says "a `decide` span per call") with attributes
`engram.decide.provider`, `engram.decide.model`, `engram.decide.model_snapshot`,
`engram.decide.questions`, `engram.decide.input_tokens`, `engram.decide.output_tokens`,
`engram.decide.cost_usd`, `engram.decide.status` — latency is span duration (no separate
attribute needed, mirrors embed/summarize not adding a redundant duration attribute).

**Error-body-bounding constant** (`internal/embed/embed.go:34-38`, verbatim-shared value):
```go
// maxErrorBodyBytes bounds how much of a non-2xx provider response body is
// read before it is surfaced in an error. Copied verbatim from
// internal/summarize/summarize.go:181 (D-13) rather than invented, so the
// two provider lanes stay consistent.
const maxErrorBodyBytes = 4096
```
Reuse `4096` verbatim for `jev`'s third copy of this constant, with the same "copied verbatim,
not invented" comment style pointing at both existing copies.

**Success-path response-byte-cap:** RESEARCH.md's Common Pitfall #4 is explicit — do NOT add an
`ENGRAM_DECISIONS_MAX_RESPONSE_BYTES` registry row. Follow `internal/summarize/summarize.go:308`'s
precedent instead (`json.NewDecoder(io.LimitReader(resp.Body, 1<<20))` — a hardcoded internal
constant, not an operator knob), sized appropriately for Decisions' small JSON payload.

**Retry-on-429/5xx pattern:** no existing analog in embed/summarize (neither retries). D-11 wants
exactly one jittered retry. Write this as a small, self-contained ~15-line wrapper around the
single `c.http.Do(req)` call (see RESEARCH.md's Don't-Hand-Roll table) — do not import a generic
backoff library; this codebase has zero retry-library dependencies today.

---

### `internal/decide/jev/jev_test.go`

**Analog:** `internal/embed/embed_test.go` (span test + non-2xx test, full patterns below)

**OTel span assertion pattern** (`internal/embed/embed_test.go:392-427`, verbatim, reusable
structure):
```go
func TestEmbedEmitsSpan(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"embedding":[0.1,0.2,0.3]}]}`))
	}))
	defer srv.Close()

	sr := tracetest.NewSpanRecorder()
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr)))
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	c := New(srv.URL, "k", "bge-m3")
	vec, err := c.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	spans := sr.Ended()
	if len(spans) == 0 || spans[0].Name() != "embed.Embed" {
		t.Fatalf("want an embed.Embed span, got %v", spans)
	}
	attrs := map[string]string{}
	for _, kv := range spans[0].Attributes() {
		attrs[string(kv.Key)] = kv.Value.String()
	}
	if attrs["engram.embed.model"] != "bge-m3" {
		t.Errorf("engram.embed.model = %q, want bge-m3", attrs["engram.embed.model"])
	}
}
```
Mirror exactly as `TestJevDecideEmitsSpan`, asserting the D-13 attribute set instead
(`engram.decide.provider/model/model_snapshot/questions/input_tokens/output_tokens/cost_usd/status`).

**Probability-tolerance testing rule (CONTEXT.md Specifics, verified against no code but stated
as a hard constraint):** any test asserting on a decoded probability/confidence value MUST use a
range/tolerance comparison (e.g. `if got < 0.85 || got > 0.95`), never `==` — the spike measured
±0.03 variance on identical requests at p=0.9.

**Both error-dialect fixtures (D-06 Wave-0 requirement):** `jev_test.go` needs at minimum two
httptest fixtures replaying the spike's captured OpenRouter (`code: <int>`) and LiteLLM
(`code: "<string>"`) error bodies — see `.claude/skills/spike-findings-engram/references/decision-transport.md`
for the exact captured shapes (read that file directly at implementation time; not reproduced
here to avoid staleness).

---

### `internal/config/registry.go` (add `decisions.*` rows)

**Analog:** itself — the `embed.*`/`summarize.*` provider-tuning rows are the direct precedent.

**Existing row-comment + shape convention** (`internal/config/registry.go:40-58`):
```go
{Key: "embed.timeout", Env: "ENGRAM_EMBED_TIMEOUT", Default: "30s"},
// embed.drain_bytes / embed.drain_timeout (07-bounded-provider-responses
// D-04, D-05): brand-new keys, no Legacy value (nothing retired to guard
// against) and no Flag (a provider-tuning value, never typed at a
// prompt). Zero is a deliberately supported operator setting on either —
// it closes the connection immediately after draining nothing rather
// than let it be reused — while a negative value is rejected outright...
{Key: "embed.drain_bytes", Env: "ENGRAM_EMBED_DRAIN_BYTES", Default: "262144"},
{Key: "embed.drain_timeout", Env: "ENGRAM_EMBED_DRAIN_TIMEOUT", Default: "2s"},
{Key: "embed.max_timeout", Env: "ENGRAM_EMBED_MAX_TIMEOUT", Default: "10m"},
```
RESEARCH.md's own "Code Examples" section already drafted the exact nine rows to add — copy
verbatim, adapting only if the planner's chosen field names differ:
```go
{Key: "decisions.provider", Env: "ENGRAM_DECISIONS_PROVIDER", Default: ""},
{Key: "decisions.base_url", Env: "ENGRAM_DECISIONS_BASE_URL"},
{Key: "decisions.api_key", Env: "ENGRAM_DECISIONS_API_KEY"}, // falls back to ENGRAM_OPENAI_API_KEY at the wiring seam (cmp.Or), like ChatAPIKey (D-03)
{Key: "decisions.model", Env: "ENGRAM_DECISIONS_MODEL", Default: "typesafe/jev-1.13"},
{Key: "decisions.timeout", Env: "ENGRAM_DECISIONS_TIMEOUT", Default: "30s"},
{Key: "decisions.max_timeout", Env: "ENGRAM_DECISIONS_MAX_TIMEOUT", Default: "10m"},
{Key: "decisions.drain_bytes", Env: "ENGRAM_DECISIONS_DRAIN_BYTES", Default: "262144"},
{Key: "decisions.drain_timeout", Env: "ENGRAM_DECISIONS_DRAIN_TIMEOUT", Default: "2s"},
{Key: "decisions.concurrency", Env: "ENGRAM_DECISIONS_CONCURRENCY", Default: "4"}, // D-10's measured c=4
```
Every new row: no `Legacy` (brand-new), no `Flag` (provider-tuning value, per the
`embed.drain_bytes` precedent — D-02 explicitly says "following the existing ... pattern").

---

### `internal/config/config.go` (add `DecisionsConfig` struct)

**Analog:** `EmbedConfig`/`SummarizeConfig` (lines 62-107, 142-159) — same doc-comment density
and per-field rationale-comment convention.

**Struct + doc-comment pattern** (`internal/config/config.go:142-159`, `SummarizeConfig`):
```go
// SummarizeConfig selects the recall-summary model and the character cap shared
// by the summarizer and recall truncation. Empty Model disables auto-summary
// (presence-enables, like OIDC issuer); MaxChars defaults to "280".
// ...
type SummarizeConfig struct {
	Model     string `koanf:"model"`
	...
}
```
Add `Decisions DecisionsConfig `koanf:"decisions"`` to the top-level `Config` struct (alongside
the existing `Summarize`/`OpenAI` fields, config.go:22-37) and a new `DecisionsConfig` struct
with fields `Provider`, `BaseURL`, `APIKey`, `Model`, `Timeout`, `MaxTimeout`, `DrainBytes`,
`DrainTimeout`, `Concurrency` (all `string`, per this package's "keep as strings, consumer
validates" convention stated at config.go:20-22) — presence-enables via `Provider != ""` per D-01,
mirroring `Summarize.Model`'s presence-enables pattern exactly (doc comment should say so
explicitly, the way `SummarizeConfig`'s own doc comment calls out the OIDC-issuer analogy).

---

### `internal/config/validate.go` (add `Decisions.*` validation block)

**Analog:** `Config.Validate()`'s `Summarize.Model`-gated block (lines 238-281) for the gating
shape, and the `OpenAI.BaseURL`/`ChatBaseURL` URL checks (lines 138-180) for the URL-shape
validation shape.

**CORRECTED per RESEARCH.md** (overriding CONTEXT.md D-03's literal wording): `Config.Validate()`
never uses a `field=<name> hint=<code>` envelope — that grammar is MCP/Connect-only
(`internal/server/argerror.go`). Follow the real style:

**Gated block pattern** (`internal/config/validate.go:238`):
```go
if c.Summarize.Model != "" {
	switch n, err := strconv.ParseUint(c.Summarize.MaxChars, 10, 64); {
	case err != nil:
		errs = append(errs, fmt.Errorf("ENGRAM_SUMMARY_MAX_CHARS %q: must be a positive integer: %w", c.Summarize.MaxChars, err))
	case n == 0:
		errs = append(errs, errors.New("ENGRAM_SUMMARY_MAX_CHARS must be greater than 0"))
	}
	// ... timeout, drain_bytes, drain_timeout, max_timeout, all gated inside this block ...
}
```

**URL-required-when-provider-set + no-fallback pattern** (RESEARCH.md's own drafted snippet,
adapted from `validate.go:138-149,238`):
```go
if c.Decisions.Provider != "" && c.Decisions.Provider != "jev" {
	errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_PROVIDER %q: must be one of \"\" or \"jev\"", c.Decisions.Provider))
}
if c.Decisions.Provider == "jev" {
	switch u, err := url.Parse(c.Decisions.BaseURL); {
	case c.Decisions.BaseURL == "":
		errs = append(errs, errors.New("ENGRAM_DECISIONS_BASE_URL is empty: required when ENGRAM_DECISIONS_PROVIDER=jev (does not fall back to ENGRAM_OPENAI_BASE_URL)"))
	case err != nil:
		errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_BASE_URL %q: must be a valid URL: %w", c.Decisions.BaseURL, err))
	case u.Scheme != "http" && u.Scheme != "https":
		errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_BASE_URL %q: scheme must be http or https", c.Decisions.BaseURL))
	case u.Host == "":
		errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_BASE_URL %q: missing host", c.Decisions.BaseURL))
	}
	// timeout/max_timeout/drain_bytes/drain_timeout/concurrency all validated
	// here too, inside the provider=="jev" gate, using the SAME
	// ParseNonNegativeIntCap / time.ParseDuration idioms embed.* and
	// summarize.* already use (validate.go:85-98, 105-110).
}
```

---

### `internal/config/providerbounds_test.go` (extend, or add sibling `decisions_bounds_test.go`)

**Analog:** itself, in full (171 lines) — the `providerBoundFields` table-driven shape.

**Table-driven registry+validate pattern** (`internal/config/providerbounds_test.go:24-31,42-95`):
```go
type providerBound struct {
	key string
	env string
	def string
	get func(*Config) string
}

var providerBoundFields = []providerBound{
	{"embed.drain_bytes", "ENGRAM_EMBED_DRAIN_BYTES", "262144", func(c *Config) string { return c.Embed.DrainBytes }},
	// ...
}

func TestProviderBoundRegistryEntries(t *testing.T) {
	t.Run("registry entry", func(t *testing.T) { /* exactly one row, correct Legacy="", Flag="", Default */ })
	t.Run("from env", func(t *testing.T) { /* t.Setenv + Load(nil) round-trip */ })
	t.Run("unset", func(t *testing.T) { /* reads back as Default */ })
}
```
Extend `providerBoundFields` with the new `decisions.drain_bytes`/`decisions.drain_timeout`/
`decisions.max_timeout` rows (the three that share the exact zero-valid/ceiling-always-rejected
semantics the existing table already tests), OR add a sibling `decisions_bounds_test.go` with an
equivalent table if `decisions.provider`/`decisions.base_url`'s gating logic doesn't fit the
existing table's shape cleanly (those two need `TestValidateProviderBounds`-style gated cases,
see `providerbounds_test.go:106-170` for the `summarizeEnabled`-fixture gating pattern to mirror
for `provider == "jev"`).

---

### `internal/server/tools.go` (add `deciderFromConfig`)

**Analog:** `embedderFromConfig`/`summarizerFromConfig` (lines 602-663)

**Wiring-seam pattern** (`internal/server/tools.go:653-663`, `summarizerFromConfig`):
```go
// summarizerFromConfig builds the chat-completions summarizer from config.
func summarizerFromConfig(cfg *config.Config) *summarize.Client {
	chatBaseURL := cmp.Or(cfg.OpenAI.ChatBaseURL, cfg.OpenAI.BaseURL)
	chatAPIKey := cmp.Or(cfg.OpenAI.ChatAPIKey, cfg.OpenAI.APIKey)
	return summarize.New(chatBaseURL, chatAPIKey, cfg.Summarize.Model, summaryMaxChars(cfg),
		summarize.WithHTTPTransport(otelhttp.NewTransport(http.DefaultTransport)),
		summarize.WithMaxTokens(summaryMaxTokens(cfg)),
		summarize.WithTimeout(summaryTimeout(cfg)),
		summarize.WithDrainBytes(summaryDrainBytes(cfg)),
		summarize.WithDrainTimeout(summaryDrainTimeout(cfg)),
		summarize.WithMaxTimeout(summaryMaxTimeout(cfg)))
}
```
Write `deciderFromConfig(cfg *config.Config) (decide.Decider, error)` following this exact shape:
`apiKey := cmp.Or(cfg.Decisions.APIKey, cfg.OpenAI.APIKey)` (D-03's key-only fallback — per
RESEARCH.md Open Question 2, resolve this at the wiring seam via `cmp.Or`, NOT inside
`internal/config`, exactly mirroring `ChatAPIKey`'s precedent — note the base URL does NOT get
this treatment, it fails `Config.Validate()` when empty and `provider=jev` instead, per D-03).
Construct the Jev client only when `cfg.Decisions.Provider == "jev"`; return `nil, nil` (no
decider, no error) when `cfg.Decisions.Provider == ""` — mirror `summarizerFromConfig`'s
caller-checks-first gate (`StoreAndSummarizerFromEnv` checks `cfg.Summarize.Model == ""` before
calling `summarizerFromConfig`), i.e. `buildDepsFromEnv` should check
`cfg.Decisions.Provider == ""` before calling `deciderFromConfig`, not push that branch inside
the constructor.

**`buildDepsFromEnv` gated-construction integration point** (`internal/server/tools.go:301-330`):
```go
func buildDepsFromEnv(sqm *telemetry.SummaryQueueMetrics, uqm *telemetry.UsageQueueMetrics) (*deps, error) {
	cfg, err := loadAndValidate()
	// ...
	em, err := embedderFromConfig(cfg)
	// ...
	return &deps{
		st: st, em: em,
		summaryMaxChars: summaryMaxChars(cfg),
		// ...
	}, nil
}
```
RESEARCH.md's Pattern 3 flags this as an open discretion point (not resolved by CONTEXT.md):
whether `deciderFromConfig` gets wired into the `deps` struct (line 36) now, as an
unused-this-phase field, or left as a standalone unwired constructor for Phase 3/4 to call. Either
choice satisfies D-01; the planner should record which was chosen as an explicit plan decision
(RESEARCH.md Assumption A4).

---

### `charts/engram/values.yaml` / `charts/engram/templates/_helpers.tpl`

**Analog:** `memory.summarize` block (`values.yaml` ~lines 100-116) + its corresponding
`_helpers.tpl` block (lines 27-46).

**Values.yaml block pattern** (`charts/engram/values.yaml`, `memory.summarize`):
```yaml
summarize:
  model: "" # ENGRAM_SUMMARY_MODEL, e.g. "gpt-4o-mini"; empty disables auto-summary
  chatBaseURL: ""
  chatApiKeySecret:
    name: ""
    key: ""
  maxChars: 280
  maxTokens: 1024
  timeout: 30s
```
RESEARCH.md's own drafted `decisions:` block (Code Examples section) mirrors this exactly — use
it verbatim, adapting field names to whatever `internal/config`'s final `DecisionsConfig` field
names are:
```yaml
decisions:
  provider: "" # ENGRAM_DECISIONS_PROVIDER; "" (default, off) | "jev"
  baseURL: "" # ENGRAM_DECISIONS_BASE_URL; required when provider=jev, does NOT fall back
  apiKeySecret:
    name: "" # empty inherits memory.openai.apiKeySecret
    key: ""
  model: "typesafe/jev-1.13" # ENGRAM_DECISIONS_MODEL (pinned; do not use ~typesafe/jev-latest)
  timeout: 30s
  concurrency: 4
```

**`_helpers.tpl` env-var-emission pattern** (`charts/engram/templates/_helpers.tpl:1-46`,
`engram.containerEnv`):
```gotemplate
{{- with .Values.memory.summarize.model }}
- { name: ENGRAM_SUMMARY_MODEL, value: "{{ . }}" }
{{- end }}
{{- if .Values.memory.summarize.chatApiKeySecret.name }}
- name: ENGRAM_OPENAI_CHAT_API_KEY
  valueFrom:
    secretKeyRef:
      name: "{{ .Values.memory.summarize.chatApiKeySecret.name }}"
      key: "{{ .Values.memory.summarize.chatApiKeySecret.key }}"
{{- end }}
```
Add a `decisions.*` block following this exact `{{- with ... }}` (plain value, omits var if
empty) / `{{- if ...Secret.name }}` (secret ref) alternation — an empty `provider` must omit
`ENGRAM_DECISIONS_PROVIDER` entirely so the server constructs nothing (D-01), matching how an
empty `summarize.model` omits `ENGRAM_SUMMARY_MODEL`.

---

### `docs-site/src/content/docs/guides/configure.md`

**Analog:** `## Auto-summary` section (lines 66-150) — a `##` heading, a short prose intro, then
a `| Variable | Flag | Default | Description |`-shaped markdown table.

**Section pattern** (verified structure, `configure.md:66-77,110`):
```markdown
## Auto-summary

When `ENGRAM_SUMMARY_MODEL` is set, the server digests memories that lack a
summary...
...
| `ENGRAM_SUMMARY_MODEL` | — | _(empty)_ | Chat model for auto-summary; empty disables auto-summary |
```
Add a `## Decisions (Jev / typed-decision provider)` section, same table shape, one row per
`ENGRAM_DECISIONS_*` var, prose intro explaining the provider enum (D-01) and the key-fallback/
no-base-URL-fallback asymmetry (D-03) — mirror the `### Async-on-write summaries` subsection
pattern (lines 129-150) if a manual-evaluation-gate callout is warranted for the D-05/D-06 SDK
checkpoint (unlikely to be operator-facing, but the pinned-model-vs-floating-alias warning
(Pitfall #3) IS operator-facing and belongs in this doc).

---

## Shared Patterns

### Provider client New()+Option construction
**Source:** `internal/embed/embed.go:65-257`, `internal/summarize/summarize.go:69-212`
**Apply to:** `internal/decide/jev/jev.go`
Struct-literal-set-before-options-loop for drain bounds (honors an explicit zero); post-loop
fallback for `maxTimeout`/`maxResponseBytes` (a zero/negative override is always rejected/ignored
— there is no way to ask for "unbounded"). See both files' `New()` (embed.go:214-257,
summarize.go:176-212) — identical shape, copy the shape not the field names.

### Bounded response drain
**Source:** `internal/httpdrain/httpdrain.go` (full file, 64 lines)
**Apply to:** `internal/decide/jev/jev.go` — every response path, success and error
```go
func Drain(body io.ReadCloser, maxBytes int64, maxTime time.Duration) {
	if maxBytes <= 0 || maxTime <= 0 {
		_ = body.Close()
		return
	}
	t := time.AfterFunc(maxTime, func() { _ = body.Close() })
	defer t.Stop()
	_, _ = io.Copy(io.Discard, io.LimitReader(body, maxBytes))
}
```
Called identically at both post-response sites in embed.go (lines 403, 417) and summarize.go
(lines 302, 311): `httpdrain.Drain(resp.Body, c.drainBytes, c.drainTimeout)`. If the OpenRouter
SDK is adopted, this call moves inside a custom `http.RoundTripper` (see jev.go's Pattern
Assignment above, Common Pitfall #2) since the SDK owns `resp.Body` directly.

### Config registry + gated Validate()
**Source:** `internal/config/registry.go` (embed.*/summarize.* rows), `internal/config/validate.go`
(Summarize.Model-gated block, lines 238-281)
**Apply to:** `internal/config/registry.go`, `internal/config/validate.go`, `internal/config/config.go`
Env-first `ENGRAM_` prefix, one `field{}` row per var, no `Flag` for provider-tuning values,
`fmt.Errorf("ENGRAM_VAR %q: <reason>")` aggregated via `errors.Join` — never the `field=/hint=`
MCP/Connect envelope (that is `internal/server/argerror.go`'s grammar exclusively, confirmed by
direct read of `Config.Validate()`, see `internal/config/validate.go:16-28` and
`docs-site/src/content/docs/reference/errors.md:6-11`).

### OTel span + slog debug line
**Source:** `internal/embed/embed.go:28,349-363`, `internal/embed/embed_test.go:392-427`
**Apply to:** `internal/decide/jev/jev.go`, `internal/decide/jev/jev_test.go`
`var tracer = otel.Tracer("github.com/seanb4t/engram/internal/decide/jev")` at package scope;
`tracer.Start(ctx, "decide", trace.WithAttributes(...))` inside `Decide`; `span.RecordError`/
`span.SetStatus(codes.Error, ...)` on failure. D-13 additionally wants a debug-level `slog` line
per call — no existing embed/summarize precedent for this (neither logs per-call at debug level);
write it as a plain `slog.Debug("decide", "provider", ..., "model", ..., "status", ...)` call
alongside the span, following this codebase's general `slog` key-value convention (see
`internal/server/tools.go`'s existing `slog.Warn` calls, e.g. line 355, for the key-naming style).

### Kubernetes Secret ref for credentials (never a plain value)
**Source:** `charts/engram/values.yaml:76-78,111-113`, `charts/engram/templates/_helpers.tpl`'s
`apiKeySecret`/`chatApiKeySecret` blocks
**Apply to:** `charts/engram/values.yaml`'s `decisions.apiKeySecret`, `_helpers.tpl`'s
`ENGRAM_DECISIONS_API_KEY` emission
`ENGRAM_DECISIONS_API_KEY` must follow the `{{- if .Values.memory.X.apiKeySecret.name }}` +
`secretKeyRef` pattern, never a plain-value `{{- with }}` block — this is a bearer credential
(ASVS V6, RESEARCH.md Security Domain table), same class as `ENGRAM_OIDC_CLIENT_SECRET`/
`ENGRAM_SERVICE_AUTH_STATIC_TOKENS`.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/decide/jev/spike_evaluation_test.go` (or equivalent Wave-0 spike file) | test | request-response | No existing engram code replays a vendor SDK's typed error decode against a captured fixture — this is the D-06/D-07 checkpoint itself, genuinely novel. Use `internal/embed/embed_test.go`'s httptest-fixture-and-assert shape as the closest structural template, but the assertion target (SDK's `sdkerrors.*` decode behavior) has no precedent. |
| `internal/decide/decide.go`'s `Decider` interface + `Question`/`Answer` discriminated-union types | model | transform | No existing pure-interface (zero I/O) package in this codebase to mirror structurally — `internal/store`, `internal/embed`, `internal/summarize` are all concrete clients, not interfaces. RESEARCH.md's Standard Stack section points at the OpenRouter SDK's own `components.Questions`/`components.Answers` discriminated unions as the target shape reference (external, not in-repo) if D-06 resolves ADOPT. |
| Retry-on-429/5xx wrapper (D-11) | utility | transform | Neither `internal/embed` nor `internal/summarize` retries at all — this is new logic. RESEARCH.md's Don't-Hand-Roll table recommends a small (~15 line) manual wrapper, or the SDK's `WithRetryConfig` overridden to cap at one retry if adopted. |

## Metadata

**Analog search scope:** `internal/embed/`, `internal/summarize/`, `internal/httpdrain/`,
`internal/config/`, `internal/server/tools.go`, `charts/engram/`, `docs-site/src/content/docs/guides/configure.md`
— all directories RESEARCH.md's canonical_refs already named; no additional Glob/Grep search was
needed since RESEARCH.md (produced this session, HIGH confidence) already did exhaustive verified
reads with line numbers for every one of these files.
**Files scanned:** 12 (all read in full or by targeted offset this session; `internal/server/tools.go`
read only at its wiring region, ~lines 590-700 and 301-361, not in full — file is 2600+ lines).
**Pattern extraction date:** 2026-09-23
