// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package summarize compresses a stored memory into a one-line recall summary
// via an OpenAI-compatible /v1/chat/completions endpoint (the same gateway
// engram already uses for embeddings). It is only invoked off the write path
// (the summarize-missing sweep), never during store_memory.
package summarize

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("github.com/seanb4t/engram/internal/summarize")

// Default generation/transport bounds, used when no option overrides them.
// defaultMaxTokens is deliberately generous: it is a ceiling, not a length
// control (the output is hard-capped at maxChars runes below), so it costs
// nothing for non-reasoning models yet leaves reasoning models room to think
// before emitting visible text. See SummarizeConfig for the operator knobs.
const (
	defaultMaxTokens = 1024
	defaultTimeout   = 30 * time.Second
)

// Client produces summaries via an OpenAI-compatible chat-completions API.
type Client struct {
	baseURL   string
	apiKey    string
	model     string
	maxChars  int
	maxTokens int
	http      *http.Client
}

// Option customizes a Client.
type Option func(*Client)

// WithHTTPTransport sets the underlying RoundTripper (e.g. otelhttp.NewTransport).
func WithHTTPTransport(rt http.RoundTripper) Option {
	return func(c *Client) { c.http.Transport = rt }
}

// WithMaxTokens sets the chat-completion generation ceiling. n <= 0 omits the
// max_tokens field entirely, letting the gateway apply its own default.
func WithMaxTokens(n int) Option {
	return func(c *Client) {
		if n < 0 {
			n = 0
		}
		c.maxTokens = n
	}
}

// WithTimeout sets the per-request HTTP client timeout. d <= 0 disables it.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.http.Timeout = d }
}

// New returns a summarizer for the given gateway, key, model, and character cap.
func New(baseURL, apiKey, model string, maxChars int, opts ...Option) *Client {
	c := &Client{baseURL: baseURL, apiKey: apiKey, model: model, maxChars: maxChars,
		maxTokens: defaultMaxTokens, http: &http.Client{Timeout: defaultTimeout}}
	for _, o := range opts {
		o(c)
	}
	return c
}

// systemPromptTmpl is fidelity-critical: the cheap model must keep the parts an
// agent will act on. %d is the character cap.
const systemPromptTmpl = `You compress a stored engineering memory into ONE terse line for fast recall.
Preserve VERBATIM: negations (do/don't, never, decline, avoid), imperatives, identifiers (flags, file paths, function/type names, IDs, env vars), and numbers.
Do not invent, infer, generalize, or add commentary. Compress only what is present.
Output a single line: no markdown, no surrounding quotes. Keep it under %d characters.`

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatReq struct {
	Model       string    `json:"model"`
	Messages    []chatMsg `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature"`
}

type chatResp struct {
	Choices []struct {
		Message chatMsg `json:"message"`
	} `json:"choices"`
}

// Summarize returns a one-line summary of content. Single line, trimmed, hard-
// capped at maxChars runes (defensive — the prompt also requests the cap).
func (c *Client) Summarize(ctx context.Context, content string) (sum string, err error) {
	ctx, span := tracer.Start(ctx, "summarize.Summarize",
		trace.WithAttributes(attribute.String("engram.summarize.model", c.model)))
	defer span.End()
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	reqBody, _ := json.Marshal(chatReq{
		Model:       c.model,
		Temperature: 0,
		// Generation ceiling only — the visible summary is hard-capped at
		// maxChars runes below. 0 omits the field (omitempty) so the gateway
		// applies its own default. See SummarizeConfig.MaxTokens for why this is
		// decoupled from maxChars (reasoning-model headroom).
		MaxTokens: c.maxTokens,
		Messages: []chatMsg{
			{Role: "system", Content: fmt.Sprintf(systemPromptTmpl, c.maxChars)},
			{Role: "user", Content: content},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("chat completions: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out chatResp
	// Bound the success-path decode too: a one-line summary response is tiny, so
	// cap at 1 MiB to keep a misbehaving gateway from forcing an unbounded read.
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("chat completions: empty choices")
	}
	s := strings.TrimSpace(out.Choices[0].Message.Content)
	if s == "" {
		return "", fmt.Errorf("chat completions: empty summary")
	}
	if r := []rune(s); len(r) > c.maxChars {
		s = strings.TrimRight(string(r[:c.maxChars]), " ")
	}
	return s, nil
}
