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
	"net/http"
	"strings"
	"time"

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

// WithTimeout sets the per-request HTTP client timeout. d <= 0 disables it
// (Go's http.Client treats a zero Timeout as no bound), the explicit D-08
// operator escape hatch for very slow local models. Composes with
// WithHTTPTransport regardless of option order — both mutate the shared
// c.http, and this is the last field either touches.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.http.Timeout = d }
}

// WithEmbeddingsURL sets the fully-resolved /embeddings endpoint verbatim,
// bypassing joinEmbeddingsURL's heuristic entirely. Empty (the default) keeps
// the heuristic. The operator override escape hatch (D-11) for provider shapes
// the heuristic does not anticipate (e.g. Azure deployment URLs).
func WithEmbeddingsURL(u string) Option {
	return func(c *Client) { c.embeddingsURL = u }
}

// New returns an embedding Client for the given base URL, API key, and model.
func New(baseURL, apiKey, model string, opts ...Option) *Client {
	c := &Client{baseURL: baseURL, apiKey: apiKey, model: model, http: &http.Client{Timeout: defaultEmbedTimeout}}
	for _, o := range opts {
		o(c)
	}
	// Resolve the embeddings URL exactly once, after options are applied: the
	// override (WithEmbeddingsURL) wins verbatim when set; otherwise the
	// shape-aware heuristic runs against baseURL (D-12).
	if c.embeddingsURL == "" {
		c.embeddingsURL = joinEmbeddingsURL(c.baseURL)
	}
	return c
}

// joinEmbeddingsURL resolves baseURL to its OpenAI-compatible /embeddings
// endpoint, shape-aware across documented provider bases: trims a trailing
// slash, then appends "/embeddings" directly if the path already terminates at
// an OpenAI-compat root ("/v1" or "/v1beta/openai" — Gemini's shape), else
// appends "/v1/embeddings" (the prior naive-concat behavior, still correct for
// bare-host gateways like a plain OpenAI or LiteLLM base URL). Does not
// canonicalize query/fragment-bearing base URLs — see the
// TestJoinEmbeddingsURL query/fragment case for the accepted, operator-error-
// scope behavior (T-13-01 trust boundary).
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

// ReservedParamKeys are the request-body keys the embedder sets authoritatively;
// operator-supplied params (ENGRAM_EMBED_*_PARAMS) must never override them.
// Exported so internal/config.ParseEmbedParams shares this exact list (#304).
var ReservedParamKeys = []string{"model", "input"}

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
	body, _ := json.Marshal(m)
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
		return nil, fmt.Errorf("embeddings: status %d", resp.StatusCode)
	}
	var out embedResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Data) == 0 {
		return nil, fmt.Errorf("embeddings: empty data")
	}
	return out.Data[0].Embedding, nil
}
