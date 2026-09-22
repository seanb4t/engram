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

// defaultEmbedTimeout is the per-request HTTP client timeout applied at
// construction when WithTimeout is not supplied (preserves prior behavior).
const defaultEmbedTimeout = 30 * time.Second

// maxErrorBodyBytes bounds how much of a non-2xx provider response body is
// read before it is surfaced in an error. Copied verbatim from
// internal/summarize/summarize.go:181 (D-13) rather than invented, so the
// two provider lanes stay consistent.
const maxErrorBodyBytes = 4096

// defaultMaxResponseBytes bounds the success-path decode when
// WithMaxResponseBytes is not supplied, matching the sibling's 1 MiB success
// bound (summarize.go:187). Plan 04-06 wires a dimension-derived value from
// ENGRAM_EMBED_DIM via WithMaxResponseBytes; this default protects a Client
// built without that wiring.
const defaultMaxResponseBytes = 1 << 20

// defaultDrainBytes and defaultDrainTimeout bound the shared httpdrain.Drain
// call at both post-response sites (D-01, D-02, D-04) when WithDrainBytes /
// WithDrainTimeout are not supplied. Plan 07-04 wires the operator-facing
// ENGRAM_EMBED_DRAIN_BYTES / ENGRAM_EMBED_DRAIN_TIMEOUT equivalents; these
// are the in-package fallbacks for a Client built without that wiring,
// exactly as defaultMaxResponseBytes is above.
const (
	defaultDrainBytes   = 256 << 10 // 256 KiB
	defaultDrainTimeout = 2 * time.Second
)

// defaultMaxTimeout bounds the ceiling a non-positive http.Client.Timeout
// resolves to (D-07, D-08) when WithMaxTimeout is not supplied. Plan 07-04
// wires the operator-facing ENGRAM_EMBED_MAX_TIMEOUT equivalent; this is the
// in-package fallback for a Client built without that wiring, exactly as
// defaultMaxResponseBytes is above.
const defaultMaxTimeout = 10 * time.Minute

// Client embeds text via an OpenAI-compatible embeddings API.
type Client struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
	// embeddingsURL is the fully-resolved /embeddings endpoint, computed once
	// in New (verbatim from WithEmbeddingsURL if set, else joinEmbeddingsURL(baseURL)).
	// embed() always uses this field, never baseURL + "/v1/embeddings" directly.
	embeddingsURL string
	// queryInstruction, when non-empty, is prepended to query text by EmbedQuery
	// as "Instruct: <instruction>\nQuery: <text>". Documents (Embed) are never
	// wrapped. Empty = symmetric embedding (queries sent raw), preserving prior
	// behavior for embedders that do not use instructions.
	queryInstruction string
	// documentInstruction, when non-empty, wraps document text in Embed per the
	// same rules as queryInstruction: literal "{document}" template if present,
	// otherwise a prefix. Empty = raw document text (prior behavior).
	documentInstruction string
	queryParams         map[string]any
	documentParams      map[string]any
	// maxResponseBytes bounds the success-path response decode (D-16). Set via
	// WithMaxResponseBytes; falls back to defaultMaxResponseBytes in New when
	// left at zero.
	maxResponseBytes int64
	// drainBytes and drainTimeout bound the shared httpdrain.Drain call at
	// both post-response sites (D-01, D-02). UNLIKE maxResponseBytes above,
	// their defaults are set in New's struct literal BEFORE options run, so
	// an explicit WithDrainBytes(0)/WithDrainTimeout(0) is honored as 0
	// rather than swallowed — see WithDrainBytes's doc comment for why (D-05,
	// D-06).
	drainBytes   int64
	drainTimeout time.Duration
	// maxTimeout is the ceiling a non-positive http.Client.Timeout resolves
	// to, applied in New after all options have run (D-07, D-09). Set via
	// WithMaxTimeout; falls back to defaultMaxTimeout when left at zero —
	// this option DOES follow the sibling post-loop-fallback convention
	// (unlike drainBytes/drainTimeout above): a zero ceiling would be
	// exactly the unbounded value this phase exists to remove, so there is
	// no honored-zero escape hatch here.
	maxTimeout time.Duration
}

// Option customizes a Client.
type Option func(*Client)

// WithQueryInstruction sets the instruction prepended to query embeddings
// (EmbedQuery). Instruction-tuned asymmetric models (e.g. Qwen3-Embedding)
// retrieve markedly better when the query carries a task instruction while
// documents stay raw. Empty (the default) keeps queries raw.
func WithQueryInstruction(instruction string) Option {
	return func(c *Client) { c.queryInstruction = instruction }
}

// WithQueryParams sets request-body params merged into query embeds (EmbedQuery),
// e.g. {"input_type":"search_query"} for OpenRouter/Cohere. Reserved keys
// "model"/"input" in the map are ignored (applied last, always authoritative).
func WithQueryParams(params map[string]any) Option {
	return func(c *Client) { c.queryParams = params }
}

// WithDocumentParams sets request-body params merged into document embeds (Embed),
// e.g. {"input_type":"search_document"}.
func WithDocumentParams(params map[string]any) Option {
	return func(c *Client) { c.documentParams = params }
}

// WithHTTPTransport sets the underlying RoundTripper (e.g. otelhttp.NewTransport)
// so embedder HTTP calls can be traced. The configured timeout is preserved.
func WithHTTPTransport(rt http.RoundTripper) Option {
	return func(c *Client) { c.http.Transport = rt }
}

// WithTimeout sets the per-request HTTP client timeout. An explicit
// positive d is honored UNCAPPED, however large — the operator named a
// number, so it is respected. A non-positive d no longer disables the
// timeout (that promise changed in this release, D-07): it now resolves to
// a configurable ceiling (see WithMaxTimeout, default 10m,
// ENGRAM_EMBED_MAX_TIMEOUT), applied in New after every option has run so
// option order is preserved. Composes with WithHTTPTransport regardless of
// option order — both mutate the shared c.http, and this is the last field
// either touches.
//
// Known limitation, stated rather than oversold: the ceiling is itself
// configurable, so a large enough value is effectively unbounded. This is a
// speed bump that forces an operator to write a number they can see, not a
// hard guarantee (D-08).
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.http.Timeout = d }
}

// WithMaxTimeout sets the ceiling a non-positive WithTimeout resolves to
// (D-07, D-08). UNLIKE WithDrainBytes/WithDrainTimeout above, a non-positive
// ceiling is ignored and defaultMaxTimeout survives — this DOES follow the
// sibling WithMaxResponseBytes convention, because a zero drain bound is a
// meaningful, safe setting (give up the connection at once) whereas a zero
// ceiling would be exactly the unbounded request timeout this phase exists
// to remove. There is deliberately no way to ask for an unbounded ceiling.
func WithMaxTimeout(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.maxTimeout = d
		}
	}
}

// WithEmbeddingsURL sets the fully-resolved /embeddings endpoint verbatim,
// bypassing joinEmbeddingsURL's heuristic entirely. Empty (the default) keeps
// the heuristic. The operator override escape hatch (D-11) for provider shapes
// the heuristic does not anticipate (e.g. Azure deployment URLs).
func WithEmbeddingsURL(u string) Option {
	return func(c *Client) { c.embeddingsURL = u }
}

// WithMaxResponseBytes bounds the success-path response decode (D-16). A
// non-positive n leaves defaultMaxResponseBytes in place — this mirrors the
// existing option shapes (e.g. WithMaxTokens in internal/summarize), where an
// out-of-range override is silently ignored rather than producing an
// unbounded read.
func WithMaxResponseBytes(n int64) Option {
	return func(c *Client) {
		if n > 0 {
			c.maxResponseBytes = n
		}
	}
}

// WithDrainBytes bounds the byte axis of the shared post-response drain
// (httpdrain.Drain) at both call sites. UNLIKE WithMaxResponseBytes above, 0
// is honored, not swallowed: New sets defaultDrainBytes in the struct
// literal BEFORE options run, so there is no post-loop fallback to
// re-silence an explicit 0. 0 is a deliberate, safe setting — close the body
// at once and burn a TCP handshake next time rather than ever risk a stall —
// and there is no way to ask for an unbounded drain (D-05, D-06).
func WithDrainBytes(n int64) Option {
	return func(c *Client) { c.drainBytes = n }
}

// WithDrainTimeout bounds the time axis of the shared post-response drain
// (httpdrain.Drain) at both call sites. UNLIKE WithMaxResponseBytes above, 0
// is honored, not swallowed: New sets defaultDrainTimeout in the struct
// literal BEFORE options run, so there is no post-loop fallback to
// re-silence an explicit 0. 0 is a deliberate, safe setting — abandon the
// body immediately rather than ever risk a stall — and there is no way to
// ask for an unbounded drain (D-05, D-06).
func WithDrainTimeout(d time.Duration) Option {
	return func(c *Client) { c.drainTimeout = d }
}

// New returns an embedding Client for the given base URL, API key, and model.
func New(baseURL, apiKey, model string, opts ...Option) *Client {
	c := &Client{
		baseURL: baseURL, apiKey: apiKey, model: model,
		http: &http.Client{Timeout: defaultEmbedTimeout},
		// Drain defaults are set HERE, in the struct literal, before the
		// options loop runs below — the deliberate divergence from
		// maxResponseBytes's post-loop fallback a few lines down. This is
		// what lets WithDrainBytes(0)/WithDrainTimeout(0) be honored as 0
		// instead of silently overwritten by a default applied afterward
		// (D-05, D-06).
		drainBytes:   defaultDrainBytes,
		drainTimeout: defaultDrainTimeout,
	}
	for _, o := range opts {
		o(c)
	}
	// Resolve the embeddings URL exactly once, after options are applied: the
	// override (WithEmbeddingsURL) wins verbatim when set; otherwise the
	// shape-aware heuristic runs against baseURL (D-12).
	if c.embeddingsURL == "" {
		c.embeddingsURL = joinEmbeddingsURL(c.baseURL)
	}
	if c.maxResponseBytes <= 0 {
		c.maxResponseBytes = defaultMaxResponseBytes
	}
	// No post-loop fallback for drainBytes/drainTimeout here — see the
	// struct-literal comment above and WithDrainBytes/WithDrainTimeout's own
	// doc comments (D-05, D-06).
	if c.maxTimeout <= 0 {
		c.maxTimeout = defaultMaxTimeout
	}
	// D-07/D-09: applied here, after the options loop and never inside
	// WithTimeout itself, so last-writer-wins option ordering between
	// WithTimeout and WithMaxTimeout is preserved regardless of which was
	// supplied first. ONLY a non-positive timeout resolves to the ceiling —
	// an explicit positive d is honored UNCAPPED, however large, because the
	// operator named a number (D-07 explicitly rejected clamping every
	// value, which would override a deliberately-chosen longer duration).
	if c.http.Timeout <= 0 {
		c.http.Timeout = c.maxTimeout
	}
	return c
}

// joinEmbeddingsURL resolves baseURL to its OpenAI-compatible /embeddings
// endpoint. The shape-aware provider-endpoint join itself now lives in
// internal/openaiurl.Join (D-14) — the single shared copy of the heuristic —
// so this wrapper just supplies the "embeddings" suffix. See
// openaiurl.Join's doc comment for the shape rules and the query/fragment
// operator-error-scope caveat (T-13-01 trust boundary), still pinned by
// TestJoinEmbeddingsURL below.
func joinEmbeddingsURL(baseURL string) string {
	return openaiurl.Join(baseURL, "embeddings")
}

// ReservedParamKeys are the request-body keys the embedder sets authoritatively;
// operator-supplied params (ENGRAM_EMBED_*_PARAMS) must never override them.
// Aliases internal/config.ReservedEmbedParamKeys, the canonical single source
// consumed by both this package's wire contract and
// internal/config.ParseEmbedParams (#304). The list lives in internal/config
// rather than here to avoid an import cycle: internal/embed already imports
// internal/telemetry, which imports internal/config.
var ReservedParamKeys = config.ReservedEmbedParamKeys

type embedResp struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// Embed returns the embedding vector for a document. Behavior by configured
// document instruction:
//   - empty: document sent raw (prior behavior).
//   - contains "{document}": used as a literal template with the placeholder
//     replaced by the document text.
//   - otherwise: the instruction is prepended as a prefix.
//
// Use at store/update/reindex time. This never affects EmbedQuery.
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	switch {
	case c.documentInstruction == "":
		// raw
	case strings.Contains(c.documentInstruction, documentPlaceholder):
		text = strings.ReplaceAll(c.documentInstruction, documentPlaceholder, text)
	default:
		text = c.documentInstruction + text
	}
	return c.embed(ctx, text, c.documentParams, "document")
}

// queryPlaceholder, when present in the configured query instruction, is
// replaced by the raw query text — letting prefix-style models supply their own
// wrapping (e.g. "Represent this sentence for searching relevant passages:
// {query}"). Without it, the instruction is wrapped in the Qwen3-family
// "Instruct: <instruction>\nQuery: <text>" template.
const queryPlaceholder = "{query}"

// documentPlaceholder, when present in the document instruction, is replaced by
// the raw document text (e.g. "search_document: {document}"); otherwise the
// instruction is prepended as a prefix.
const documentPlaceholder = "{document}"

// WithDocumentInstruction sets the document-side text applied by Embed: empty =
// raw; contains "{document}" = literal template; otherwise prepended as a prefix
// (e.g. "passage: "). For both-side-prefix models like E5 / nomic. Changing it
// alters stored document vectors, so it requires a reindex to take effect.
func WithDocumentInstruction(instruction string) Option {
	return func(c *Client) { c.documentInstruction = instruction }
}

// EmbedQuery returns the embedding vector for a search query. Behavior by
// configured query instruction:
//   - empty: query sent raw (symmetric embedding; prior behavior).
//   - contains "{query}": used as a literal template with the placeholder
//     replaced by the query (prefix-style models, e.g. bge-*-v1.5).
//   - otherwise: wrapped as "Instruct: <instruction>\nQuery: <text>"
//     (Qwen3-family instruct embedders).
//
// Only the query side is wrapped, so stored document vectors need no reindex.
func (c *Client) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	switch {
	case c.queryInstruction == "":
		// raw
	case strings.Contains(c.queryInstruction, queryPlaceholder):
		text = strings.ReplaceAll(c.queryInstruction, queryPlaceholder, text)
	default:
		text = "Instruct: " + c.queryInstruction + "\nQuery: " + text
	}
	return c.embed(ctx, text, c.queryParams, "query")
}

// embed performs the OpenAI-compatible /v1/embeddings call for a fully-formed
// input string (document or query, already wrapped as needed).
func (c *Client) embed(ctx context.Context, text string, params map[string]any, kind string) (vec []float32, err error) {
	ctx, span := tracer.Start(ctx, "embed.Embed", trace.WithAttributes(
		attribute.String("engram.embed.model", c.model),
		attribute.String("engram.embed.kind", kind),
	))
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordEmbed(ctx, start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int("engram.embed.dims", len(vec)))
		}
	}()

	// Single map-based body build: merge operator params first, then set
	// model/input last so they are always authoritative. Go sorts map keys on
	// marshal; that is JSON-semantically identical, so callers compare decoded
	// objects, not raw bytes.
	m := make(map[string]any, len(params)+2)
	for k, v := range params {
		m[k] = v
	}
	m["model"] = c.model
	m["input"] = text
	body, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("embeddings: marshal request body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.embeddingsURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// Bounded, verbatim (D-15) — this is the provider's own diagnostic
		// text, not caller data being echoed unexamined: m["input"] above DOES
		// carry caller content (the memory being stored or the query being
		// searched), so a provider that reflects its request back inside an
		// error body would surface that content here. That reflection is
		// nonetheless same-actor on this return path — it travels back to the
		// same caller who supplied it — so it is not a cross-actor
		// disclosure. The residual exposure is that connectError's default
		// arm logs this error server-side (internal/server/connecterror.go),
		// so a reflecting provider could put one caller's content into an
		// operator log, bounded here at maxErrorBodyBytes (T-04-05, accepted).
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		httpdrain.Drain(resp.Body, c.drainBytes, c.drainTimeout) // bounded by bytes and time (D-01, D-02)
		return nil, fmt.Errorf("embeddings: status %d: %s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}
	var out embedResp
	// Bound the success-path decode too (D-16): an embeddings response is
	// larger than the chat lane's one-line summary, so this is sized from the
	// configured ENGRAM_EMBED_DIM via WithMaxResponseBytes (wired in plan
	// 04-06) rather than blind-copying the sibling's 1 MiB — defaultMaxResponseBytes
	// is only the fallback for a Client built without that wiring. Mirrors
	// summarize.go's misbehaving-gateway rationale: without a bound, a
	// misbehaving gateway can force an unbounded read.
	if err := json.NewDecoder(io.LimitReader(resp.Body, c.maxResponseBytes)).Decode(&out); err != nil {
		return nil, err
	}
	httpdrain.Drain(resp.Body, c.drainBytes, c.drainTimeout) // bounded by bytes and time (D-01, D-02)
	if len(out.Data) == 0 {
		return nil, fmt.Errorf("embeddings: empty data")
	}
	return out.Data[0].Embedding, nil
}
