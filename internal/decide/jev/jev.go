// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package jev implements the Jev backend over OpenRouter's Decisions API at
// {base}/alpha/decisions. It works against OpenRouter directly
// (https://openrouter.ai/api) and through a LiteLLM pass-through
// (https://<gateway>/openrouter), and it never uses the chat base URL (D-03):
// the decision base URL is a dedicated setting, never inherited from the
// embeddings/chat lane.
//
// Hand-written on net/http + encoding/json (DEC-05/D-06): OpenRouter's Go SDK
// was evaluated and rejected — see
// .planning/phases/02-decision-interface-jev-backend/02-SDK-EVALUATION.md for
// the full record. This mirrors internal/embed and internal/summarize's
// New+Option client shape.
package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"github.com/seanb4t/engram/internal/decide"
	"github.com/seanb4t/engram/internal/httpdrain"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("github.com/seanb4t/engram/internal/decide/jev")

// DefaultModel is the pinned Jev model used when New is called with an empty
// model. Pinned deliberately (RESEARCH Pitfall 3): the floating alias
// "typesafe/jev-latest" moves thresholds out from under a caller and is never
// used here.
const DefaultModel = "typesafe/jev-1.13"

// decisionsPath is the fixed suffix appended to the operator's base URL
// (D-03) — never the chat or embeddings URL, and never shape-aware like
// internal/openaiurl.Join (that heuristic is OpenAI-/v1-specific and does not
// apply to Decisions).
const decisionsPath = "/alpha/decisions"

const (
	defaultTimeout      = 10 * time.Second
	defaultMaxTimeout   = 10 * time.Minute
	defaultDrainBytes   = 256 << 10
	defaultDrainTimeout = 2 * time.Second
	defaultConcurrency  = 4
)

// maxErrorBodyBytes bounds how much of a non-2xx provider response body is
// read before it is surfaced in an error. Copied verbatim from
// internal/embed and internal/summarize rather than invented, so all three
// provider lanes stay consistent.
const maxErrorBodyBytes = 4096

// defaultMaxResponseBytes bounds the success-path decode when
// WithMaxResponseBytes is not set (or is given a non-positive value). A
// Decisions response is small JSON (a handful of typed answers plus usage),
// so this is a hardcoded internal constant rather than an operator knob
// (RESEARCH Pitfall 4: no ENGRAM_DECISIONS_MAX_RESPONSE_BYTES registry row),
// following internal/summarize's identical precedent.
const defaultMaxResponseBytes = 1 << 20

// defaultRetryBase and defaultRetryJitter build the default retryDelay: a
// fixed 100ms base plus up to 300ms of jitter (D-11), so concurrent callers
// retrying after the same upstream 429/5xx do not all retry in lockstep.
const (
	defaultRetryBase   = 100 * time.Millisecond
	defaultRetryJitter = 300 * time.Millisecond
)

// Client calls the Jev Decisions API.
type Client struct {
	baseURL          string
	apiKey           string
	model            string
	endpoint         string
	http             *http.Client
	timeout          time.Duration
	maxTimeout       time.Duration
	drainBytes       int64
	drainTimeout     time.Duration
	concurrency      int
	maxResponseBytes int64
	// retryDelay computes the jittered wait before D-11's single retry.
	// Tests override this field directly (same package) to avoid sleeping
	// the production jitter.
	retryDelay func() time.Duration
}

// Option customizes a Client.
type Option func(*Client)

// WithHTTPTransport sets the underlying RoundTripper (e.g. otelhttp.NewTransport)
// so Jev HTTP calls can be traced. The configured timeout is preserved.
func WithHTTPTransport(rt http.RoundTripper) Option {
	return func(c *Client) { c.http.Transport = rt }
}

// WithTimeout sets the per-call budget used both as the context.WithTimeout
// deadline and as the http.Client.Timeout backstop. A non-positive d resolves
// to WithMaxTimeout's ceiling in New, never unbounded (mirrors
// internal/embed.WithTimeout).
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.timeout = d }
}

// WithMaxTimeout sets the ceiling a non-positive WithTimeout resolves to. A
// non-positive d is ignored and defaultMaxTimeout survives — there is
// deliberately no way to ask for an unbounded ceiling (mirrors
// internal/embed.WithMaxTimeout).
func WithMaxTimeout(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.maxTimeout = d
		}
	}
}

// WithDrainBytes bounds the byte axis of the shared post-response drain
// (httpdrain.Drain). 0 is honored, not swallowed: New sets defaultDrainBytes
// in the struct literal before options run, so there is no post-loop
// fallback to re-silence an explicit 0 (mirrors internal/embed.WithDrainBytes).
func WithDrainBytes(n int64) Option {
	return func(c *Client) { c.drainBytes = n }
}

// WithDrainTimeout bounds the time axis of the shared post-response drain.
// 0 is honored, not swallowed, for the same reason as WithDrainBytes.
func WithDrainTimeout(d time.Duration) Option {
	return func(c *Client) { c.drainTimeout = d }
}

// WithMaxResponseBytes bounds the success-path response decode (DEC-04). A
// non-positive n leaves defaultMaxResponseBytes in place — mirrors
// internal/embed.WithMaxResponseBytes's post-loop-fallback semantics: an
// out-of-range override is silently ignored rather than producing an
// unbounded read.
func WithMaxResponseBytes(n int64) Option {
	return func(c *Client) {
		if n > 0 {
			c.maxResponseBytes = n
		}
	}
}

// WithConcurrency bounds the worker pool DecideMany (plan 02-04) uses. A
// non-positive n is ignored and defaultConcurrency survives — applied after
// the options loop in New, mirroring WithMaxTimeout's convention rather than
// WithDrainBytes/WithDrainTimeout's honored-zero convention: concurrency has
// no "0 means disabled" semantics.
func WithConcurrency(n int) Option {
	return func(c *Client) { c.concurrency = n }
}

// New returns a Jev Client for the given base URL, API key, and model. An
// empty model becomes DefaultModel. The Authorization header sent by Decide
// is "Bearer <apiKey>", omitted entirely when apiKey is empty.
func New(baseURL, apiKey, model string, opts ...Option) *Client {
	if model == "" {
		model = DefaultModel
	}
	c := &Client{
		baseURL: baseURL, apiKey: apiKey, model: model,
		endpoint: strings.TrimRight(baseURL, "/") + decisionsPath,
		http:     &http.Client{},
		timeout:  defaultTimeout,
		// Drain defaults are set HERE, in the struct literal, before the
		// options loop runs below — mirrors internal/embed's New: this is
		// what lets WithDrainBytes(0)/WithDrainTimeout(0) be honored as 0
		// instead of silently overwritten by a post-loop default.
		drainBytes:   defaultDrainBytes,
		drainTimeout: defaultDrainTimeout,
		// The default retryDelay: fixed base plus jitter, computed fresh on
		// every call (D-11). Tests replace this field directly to avoid
		// sleeping the production jitter.
		retryDelay: func() time.Duration {
			return defaultRetryBase + rand.N(defaultRetryJitter)
		},
	}
	for _, o := range opts {
		o(c)
	}
	if c.maxTimeout <= 0 {
		c.maxTimeout = defaultMaxTimeout
	}
	// A non-positive timeout resolves to the maxTimeout ceiling, never
	// unbounded (mirrors internal/embed's http.Client.Timeout resolution).
	if c.timeout <= 0 {
		c.timeout = c.maxTimeout
	}
	if c.concurrency <= 0 {
		c.concurrency = defaultConcurrency
	}
	if c.maxResponseBytes <= 0 {
		c.maxResponseBytes = defaultMaxResponseBytes
	}
	// http.Client.Timeout is set to the resolved timeout as a backstop,
	// alongside the per-call context.WithTimeout budget Decide derives below.
	c.http.Timeout = c.timeout
	return c
}

var _ decide.Decider = (*Client)(nil)

// wireRequest, wireQuestion, wireResponse, wireAnswer and wireUsage are the
// unexported wire structs for the Decisions API (D-06 reject-hand-write
// branch): plain net/http + encoding/json, no generated SDK types.
type wireRequest struct {
	Model     string                  `json:"model"`
	State     map[string]any          `json:"state"`
	Questions map[string]wireQuestion `json:"questions"`
}

type wireQuestion struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria"`
}

type wireResponse struct {
	Model    string                `json:"model"`
	Answers  map[string]wireAnswer `json:"answers"`
	Usage    wireUsage             `json:"usage"`
	ID       string                `json:"id"`
	Provider string                `json:"provider"`
}

type wireAnswer struct {
	Type string  `json:"type"`
	Noul float64 `json:"noul"`
}

type wireUsage struct {
	InputTokens  int64    `json:"input_tokens"`
	OutputTokens int64    `json:"output_tokens"`
	Cost         *float64 `json:"cost"`
}

// Decide sends req to {base}/alpha/decisions and decodes the typed noul
// answers, model snapshot, id, provider and usage into a decide.Response.
// Every call emits one "decide" span (D-13) and exactly one debug-level slog
// line; neither carries State, Instructions, criteria or the API key.
func (c *Client) Decide(ctx context.Context, req decide.Request) (resp decide.Response, err error) {
	ctx, span := tracer.Start(ctx, "decide", trace.WithAttributes(
		attribute.String("engram.decide.provider", "jev"),
		attribute.String("engram.decide.model", c.model),
		attribute.Int("engram.decide.questions", len(req.Questions)),
	))
	defer span.End()
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		// decide.Status classifies err into the D-12/D-09 vocabulary — a
		// validation failure below reports "invalid_request" here, not the
		// generic "error" a hardcoded ok/error pair would give it. Full
		// status-class classification of network/HTTP failures into named
		// *decide.Error values is plan 02-07's job; until then an
		// unclassified transport error falls through Status's default case
		// to "error", identical to today's behavior.
		status := decide.Status(err)
		var modelSnapshot string
		var inputTokens, outputTokens int64
		var costUSD *float64
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			modelSnapshot = resp.Model
			if resp.Usage != nil {
				inputTokens = resp.Usage.InputTokens
				outputTokens = resp.Usage.OutputTokens
				costUSD = resp.Usage.CostUSD
			}
			span.SetAttributes(
				attribute.String("engram.decide.model_snapshot", modelSnapshot),
				attribute.Int64("engram.decide.input_tokens", inputTokens),
				attribute.Int64("engram.decide.output_tokens", outputTokens),
			)
			if costUSD != nil {
				span.SetAttributes(attribute.Float64("engram.decide.cost_usd", *costUSD))
			}
		}
		span.SetAttributes(attribute.String("engram.decide.status", status))
		slog.DebugContext(ctx, "decide",
			"provider", "jev",
			"model", c.model,
			"model_snapshot", modelSnapshot,
			"questions", len(req.Questions),
			"input_tokens", inputTokens,
			"output_tokens", outputTokens,
			"cost_usd", costUSD,
			"status", status,
			"duration_ms", duration.Milliseconds(),
		)
	}()

	// D-09: cheap structural validation runs before any network I/O. A
	// validation failure still goes through the status/slog defer above
	// (engram.decide.status = decide.Status(err), "invalid_request" for
	// every D-09 sentinel) even though no request was ever sent.
	if verr := req.Validate(); verr != nil {
		err = verr
		return decide.Response{}, err
	}

	// One timeout budget covers both the first attempt and D-11's single
	// retry — there is never a per-attempt timeout, only this one deadline.
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	wireQ := make(map[string]wireQuestion, len(req.Questions))
	for name, q := range req.Questions {
		if q.Type != decide.QuestionNoul {
			err = fmt.Errorf("decide: unsupported question type %q for question %q", q.Type, name)
			return decide.Response{}, err
		}
		wireQ[name] = wireQuestion{
			Type:         string(q.Type),
			Instructions: q.Instructions,
			Criteria:     map[string]string{"true": q.WhenTrue, "false": q.WhenFalse},
		}
	}

	body, merr := json.Marshal(wireRequest{
		Model:     c.model,
		State:     map[string]any(req.State),
		Questions: wireQ,
	})
	if merr != nil {
		err = fmt.Errorf("decide: marshal request body: %w", merr)
		return decide.Response{}, err
	}

	resp, err = c.attempt(ctx, body)
	if err != nil && isRetryable(err) {
		if dl, ok := ctx.Deadline(); ok {
			d := c.retryDelay()
			if time.Until(dl) > d {
				timer := time.NewTimer(d)
				select {
				case <-timer.C:
					resp, err = c.attempt(ctx, body)
				case <-ctx.Done():
					timer.Stop()
					// The budget expired while waiting to retry: report the
					// original failure, not the wait's cancellation — there
					// was never a second attempt.
				}
			}
		}
	}
	return resp, err
}

// attempt performs exactly one HTTP exchange against the Decisions endpoint
// and classifies its outcome (D-12): a transport failure goes through
// classifyTransport, a non-200 status through classifyStatus, and a success
// body larger than c.maxResponseBytes becomes a named too-large error. It
// never retries — Decide owns the single retry (D-11).
func (c *Client) attempt(ctx context.Context, body []byte) (decide.Response, error) {
	httpReq, nerr := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if nerr != nil {
		return decide.Response{}, nerr
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	httpResp, derr := c.http.Do(httpReq)
	if derr != nil {
		return decide.Response{}, classifyTransport(ctx, derr)
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(httpResp.Body, maxErrorBodyBytes))
		httpdrain.Drain(httpResp.Body, c.drainBytes, c.drainTimeout) // bounded by bytes and time
		return decide.Response{}, classifyStatus(httpResp.StatusCode, errBody)
	}

	// Read one byte past the bound: exactly maxResponseBytes bytes is within
	// bound, but maxResponseBytes+1 proves the body was larger.
	raw, rerr := io.ReadAll(io.LimitReader(httpResp.Body, c.maxResponseBytes+1))
	if rerr != nil {
		httpdrain.Drain(httpResp.Body, c.drainBytes, c.drainTimeout) // bounded by bytes and time
		return decide.Response{}, fmt.Errorf("decide: read response: %w", rerr)
	}
	if int64(len(raw)) > c.maxResponseBytes {
		httpdrain.Drain(httpResp.Body, c.drainBytes, c.drainTimeout) // bounded by bytes and time
		return decide.Response{}, &decide.Error{Kind: decide.ErrDecisionResponseTooLarge, Status: http.StatusOK}
	}

	var wr wireResponse
	if decErr := json.Unmarshal(raw, &wr); decErr != nil {
		httpdrain.Drain(httpResp.Body, c.drainBytes, c.drainTimeout) // bounded by bytes and time
		return decide.Response{}, fmt.Errorf("decide: decode response: %w", decErr)
	}
	httpdrain.Drain(httpResp.Body, c.drainBytes, c.drainTimeout) // bounded by bytes and time

	answers := make(map[string]decide.Answer, len(wr.Answers))
	for name, a := range wr.Answers {
		if a.Type != string(decide.QuestionNoul) {
			return decide.Response{}, fmt.Errorf("decisions: unsupported answer type %q for question %q", a.Type, name)
		}
		answers[name] = decide.Answer{Type: decide.QuestionNoul, Probability: a.Noul}
	}

	return decide.Response{
		Answers:  answers,
		Model:    wr.Model,
		ID:       wr.ID,
		Provider: wr.Provider,
		Usage: &decide.Usage{
			InputTokens:  wr.Usage.InputTokens,
			OutputTokens: wr.Usage.OutputTokens,
			CostUSD:      wr.Usage.Cost,
		},
	}, nil
}

// DecideMany answers many Requests through the shared decide.DecideMany
// bounded worker pool (D-10), at this Client's configured concurrency
// (ENGRAM_DECISIONS_CONCURRENCY / WithConcurrency).
func (c *Client) DecideMany(ctx context.Context, reqs []decide.Request) []decide.Result {
	return decide.DecideMany(ctx, c.Decide, reqs, c.concurrency)
}
