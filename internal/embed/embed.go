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

// Client embeds text via an OpenAI-compatible embeddings API.
type Client struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
	// queryInstruction, when non-empty, is prepended to query text by EmbedQuery
	// as "Instruct: <instruction>\nQuery: <text>". Documents (Embed) are never
	// wrapped. Empty = symmetric embedding (queries sent raw), preserving prior
	// behavior for embedders that do not use instructions.
	queryInstruction string
	queryParams      map[string]any
	documentParams   map[string]any
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
// so embedder HTTP calls can be traced. The 30s timeout is preserved.
func WithHTTPTransport(rt http.RoundTripper) Option {
	return func(c *Client) { c.http.Transport = rt }
}

// New returns an embedding Client for the given base URL, API key, and model.
func New(baseURL, apiKey, model string, opts ...Option) *Client {
	c := &Client{baseURL: baseURL, apiKey: apiKey, model: model, http: &http.Client{Timeout: 30 * time.Second}}
	for _, o := range opts {
		o(c)
	}
	return c
}

type embedReq struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embedResp struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// Embed returns the embedding vector for a document (raw text, never wrapped
// with a query instruction). Use at store/update/reindex time.
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	return c.embed(ctx, text, c.documentParams, "document")
}

// queryPlaceholder, when present in the configured query instruction, is
// replaced by the raw query text — letting prefix-style models supply their own
// wrapping (e.g. "Represent this sentence for searching relevant passages:
// {query}"). Without it, the instruction is wrapped in the Qwen3-family
// "Instruct: <instruction>\nQuery: <text>" template.
const queryPlaceholder = "{query}"

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

	// Empty params → marshal the struct (exact prior wire bytes; default path).
	// Non-empty → merge params first, then set model/input last so they are
	// always authoritative. Go sorts map keys on marshal; that is JSON-
	// semantically identical, so callers compare decoded objects, not raw bytes.
	var body []byte
	if len(params) == 0 {
		body, _ = json.Marshal(embedReq{Model: c.model, Input: text})
	} else {
		m := make(map[string]any, len(params)+2)
		for k, v := range params {
			m[k] = v
		}
		m["model"] = c.model
		m["input"] = text
		body, _ = json.Marshal(m)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/embeddings", bytes.NewReader(body))
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
