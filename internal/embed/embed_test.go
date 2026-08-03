// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package embed

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/seanb4t/engram/internal/testhttp"
)

func TestWithHTTPTransportIsApplied(t *testing.T) {
	marker := &markerTransport{}
	c := New("http://x", "k", "m", WithHTTPTransport(marker))
	if c.http.Transport != marker {
		t.Fatal("WithHTTPTransport did not set the client transport")
	}
}

// TestEmbedWithTimeoutCancelsSlowRequest proves a slow/unreachable embedder
// call is cut short within the configured WithTimeout bound instead of hanging
// the calling MCP tool (SC4), mirroring
// summarize.TestSummarizeWithTimeoutCancelsSlowRequest.
func TestEmbedWithTimeoutCancelsSlowRequest(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		<-release // never responds before the client's timeout elapses
	}))
	// Defers run LIFO: close(release) first unblocks the handler, then Close()
	// returns promptly instead of waiting on the still-running request.
	defer srv.Close()
	defer close(release)

	start := time.Now()
	_, err := New(srv.URL, "k", "m", WithTimeout(20*time.Millisecond)).Embed(context.Background(), "x")
	if err == nil {
		t.Fatal("want timeout error from slow embedder, got nil")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("Embed took %v; want bounded by the tiny WithTimeout, not an unbounded hang", elapsed)
	}
}

// TestWithTimeoutComposesWithHTTPTransport locks that WithTimeout survives
// WithHTTPTransport (both mutate the shared c.http), regardless of option
// order — addresses review round-1 LOW (option composition).
func TestWithTimeoutComposesWithHTTPTransport(t *testing.T) {
	marker := &markerTransport{}
	want := 42 * time.Millisecond

	t.Run("transport then timeout", func(t *testing.T) {
		c := New("http://x", "k", "m", WithHTTPTransport(marker), WithTimeout(want))
		if c.http.Timeout != want {
			t.Errorf("c.http.Timeout = %v, want %v", c.http.Timeout, want)
		}
		if c.http.Transport != marker {
			t.Error("WithHTTPTransport did not survive composition with WithTimeout")
		}
	})

	t.Run("timeout then transport", func(t *testing.T) {
		c := New("http://x", "k", "m", WithTimeout(want), WithHTTPTransport(marker))
		if c.http.Timeout != want {
			t.Errorf("c.http.Timeout = %v, want %v", c.http.Timeout, want)
		}
		if c.http.Transport != marker {
			t.Error("WithHTTPTransport did not survive composition with WithTimeout")
		}
	})
}

// TestJoinEmbeddingsURL covers every documented provider base-URL shape (D-12)
// plus the query/fragment edge case, which is pinned as accepted operator-error
// scope (review round-2 LOW — see joinEmbeddingsURL's doc comment).
func TestJoinEmbeddingsURL(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		want    string
	}{
		{"OpenRouter /v1", "https://openrouter.ai/api/v1", "https://openrouter.ai/api/v1/embeddings"},
		{"OpenAI /v1", "https://api.openai.com/v1", "https://api.openai.com/v1/embeddings"},
		{"OpenAI bare host, no /v1", "https://api.openai.com", "https://api.openai.com/v1/embeddings"},
		{"trailing slash", "http://localhost:4000/", "http://localhost:4000/v1/embeddings"},
		{"Gemini /v1beta/openai", "https://generativelanguage.googleapis.com/v1beta/openai", "https://generativelanguage.googleapis.com/v1beta/openai/embeddings"},
		// Operator-error scope (accepted, not canonicalized — T-13-01 trust
		// boundary parity with ENGRAM_OPENAI_BASE_URL validation): a
		// query/fragment-bearing base URL is not stripped before the suffix
		// check, so it never matches "/v1" and falls through to the
		// "/v1/embeddings" append, producing a URL with the query string stuck
		// mid-path. No documented provider shape carries a query/fragment.
		{"query/fragment base URL (operator-error scope, pinned)", "http://localhost:4000/v1?debug=1", "http://localhost:4000/v1?debug=1/v1/embeddings"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := joinEmbeddingsURL(tc.baseURL); got != tc.want {
				t.Errorf("joinEmbeddingsURL(%q) = %q, want %q", tc.baseURL, got, tc.want)
			}
		})
	}
}

// TestEmbedRequestPathUsesResolvedEmbeddingsURL proves the resolved-once
// Client.embeddingsURL field is what Embed actually POSTs to on a live
// request, not merely what the pure joinEmbeddingsURL helper returns —
// addresses review round-1 LOW (live request path).
func TestEmbedRequestPathUsesResolvedEmbeddingsURL(t *testing.T) {
	t.Run("heuristic join", func(t *testing.T) {
		var gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"embedding": []float32{0.1}}},
			})
		}))
		defer srv.Close()

		c := New(srv.URL+"/v1", "k", "m")
		if _, err := c.Embed(context.Background(), "hello"); err != nil {
			t.Fatalf("Embed: %v", err)
		}
		if gotPath != "/v1/embeddings" {
			t.Errorf("request path = %q, want /v1/embeddings", gotPath)
		}
	})

	t.Run("WithEmbeddingsURL override bypasses the heuristic", func(t *testing.T) {
		var gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"embedding": []float32{0.1}}},
			})
		}))
		defer srv.Close()

		c := New(srv.URL+"/v1", "k", "m", WithEmbeddingsURL(srv.URL+"/custom/path"))
		if _, err := c.Embed(context.Background(), "hello"); err != nil {
			t.Fatalf("Embed: %v", err)
		}
		if gotPath != "/custom/path" {
			t.Errorf("request path = %q, want /custom/path (override verbatim)", gotPath)
		}
	})
}

type markerTransport struct{}

func (m *markerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return http.DefaultTransport.RoundTrip(r)
}

func TestEmbedReturnsVector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("missing/wrong auth header: %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"embedding": []float32{0.1, 0.2, 0.3}}},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key", "ollama/bge-m3")
	vec, err := c.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vec) != 3 || vec[0] != 0.1 {
		t.Fatalf("unexpected vector: %v", vec)
	}
}

// captureInput starts a server that records the decoded `input` of each
// embeddings request and returns a fixed vector.
func captureInput(t *testing.T, got *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Input string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		*got = body.Input
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"embedding": []float32{0.1}}},
		})
	}))
}

// EmbedQuery prepends the Qwen3-style instruction to the query side only when an
// instruction is configured; documents (Embed) are always sent raw. This is the
// asymmetric-embedding fix for GH#261 — and because only the query is wrapped,
// stored document vectors need no reindex.
func TestEmbedQueryInstruction(t *testing.T) {
	instr := "Given a search query, retrieve relevant memory records"

	t.Run("instruction wraps query", func(t *testing.T) {
		var got string
		srv := captureInput(t, &got)
		defer srv.Close()
		c := New(srv.URL, "k", "m", WithQueryInstruction(instr))
		if _, err := c.EmbedQuery(context.Background(), "why did pushover fail"); err != nil {
			t.Fatalf("EmbedQuery: %v", err)
		}
		want := "Instruct: " + instr + "\nQuery: why did pushover fail"
		if got != want {
			t.Errorf("query input = %q, want %q", got, want)
		}
	})

	t.Run("document is never wrapped", func(t *testing.T) {
		var got string
		srv := captureInput(t, &got)
		defer srv.Close()
		c := New(srv.URL, "k", "m", WithQueryInstruction(instr))
		if _, err := c.Embed(context.Background(), "raw document"); err != nil {
			t.Fatalf("Embed: %v", err)
		}
		if got != "raw document" {
			t.Errorf("document input = %q, want %q", got, "raw document")
		}
	})

	t.Run("query placeholder is substituted literally", func(t *testing.T) {
		var got string
		srv := captureInput(t, &got)
		defer srv.Close()
		// A {query} placeholder lets prefix-style models (e.g. bge-*-v1.5) express
		// their own query wrapping instead of the Qwen3 Instruct/Query template.
		c := New(srv.URL, "k", "m", WithQueryInstruction("Represent this sentence for searching relevant passages: {query}"))
		if _, err := c.EmbedQuery(context.Background(), "gravity"); err != nil {
			t.Fatalf("EmbedQuery: %v", err)
		}
		want := "Represent this sentence for searching relevant passages: gravity"
		if got != want {
			t.Errorf("query input = %q, want %q", got, want)
		}
	})

	t.Run("no instruction leaves query raw", func(t *testing.T) {
		var got string
		srv := captureInput(t, &got)
		defer srv.Close()
		c := New(srv.URL, "k", "m") // no WithQueryInstruction
		if _, err := c.EmbedQuery(context.Background(), "raw query"); err != nil {
			t.Fatalf("EmbedQuery: %v", err)
		}
		if got != "raw query" {
			t.Errorf("query input = %q, want %q (no instruction configured)", got, "raw query")
		}
	})
}

// captureBody records the full decoded request body of each embeddings request.
func captureBody(t *testing.T, got *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		*got = body
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"embedding": []float32{0.1}}},
		})
	}))
}

func TestEmbedParamsMergedIntoBody(t *testing.T) {
	t.Run("query params merged by EmbedQuery", func(t *testing.T) {
		var got map[string]any
		srv := captureBody(t, &got)
		defer srv.Close()
		c := New(srv.URL, "k", "m", WithQueryParams(map[string]any{"input_type": "search_query"}))
		if _, err := c.EmbedQuery(context.Background(), "hello"); err != nil {
			t.Fatalf("EmbedQuery: %v", err)
		}
		if got["input_type"] != "search_query" || got["model"] != "m" || got["input"] != "hello" {
			t.Errorf("body = %v; want input_type=search_query, model=m, input=hello", got)
		}
	})

	t.Run("document params merged by Embed", func(t *testing.T) {
		var got map[string]any
		srv := captureBody(t, &got)
		defer srv.Close()
		c := New(srv.URL, "k", "m", WithDocumentParams(map[string]any{"input_type": "search_document"}))
		if _, err := c.Embed(context.Background(), "doc"); err != nil {
			t.Fatalf("Embed: %v", err)
		}
		if got["input_type"] != "search_document" || got["input"] != "doc" {
			t.Errorf("body = %v; want input_type=search_document, input=doc", got)
		}
	})

	t.Run("reserved keys cannot be clobbered by params", func(t *testing.T) {
		var got map[string]any
		srv := captureBody(t, &got)
		defer srv.Close()
		// Even a caller that bypasses config validation cannot override model/input.
		c := New(srv.URL, "k", "m", WithQueryParams(map[string]any{"model": "evil", "input": "evil"}))
		if _, err := c.EmbedQuery(context.Background(), "real"); err != nil {
			t.Fatalf("EmbedQuery: %v", err)
		}
		if got["model"] != "m" || got["input"] != "real" {
			t.Errorf("body = %v; model/input must be authoritative", got)
		}
	})

	t.Run("no params configured produces exactly model+input", func(t *testing.T) {
		var got map[string]any
		srv := captureBody(t, &got)
		defer srv.Close()
		c := New(srv.URL, "k", "m") // no WithQueryParams/WithDocumentParams
		if _, err := c.Embed(context.Background(), "doc"); err != nil {
			t.Fatalf("Embed: %v", err)
		}
		if len(got) != 2 || got["model"] != "m" || got["input"] != "doc" {
			t.Errorf("body = %v; want exactly {model: m, input: doc}", got)
		}
	})
}

func TestEmbedDocumentInstruction(t *testing.T) {
	t.Run("placeholder substituted", func(t *testing.T) {
		var got string
		srv := captureInput(t, &got)
		defer srv.Close()
		c := New(srv.URL, "k", "m", WithDocumentInstruction("search_document: {document}"))
		if _, err := c.Embed(context.Background(), "the fox"); err != nil {
			t.Fatalf("Embed: %v", err)
		}
		if got != "search_document: the fox" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("no placeholder is a prefix", func(t *testing.T) {
		var got string
		srv := captureInput(t, &got)
		defer srv.Close()
		c := New(srv.URL, "k", "m", WithDocumentInstruction("passage: "))
		if _, err := c.Embed(context.Background(), "the fox"); err != nil {
			t.Fatalf("Embed: %v", err)
		}
		if got != "passage: the fox" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("document instruction does not affect EmbedQuery", func(t *testing.T) {
		var got string
		srv := captureInput(t, &got)
		defer srv.Close()
		c := New(srv.URL, "k", "m", WithDocumentInstruction("passage: "))
		if _, err := c.EmbedQuery(context.Background(), "a query"); err != nil {
			t.Fatalf("EmbedQuery: %v", err)
		}
		if got != "a query" {
			t.Errorf("document instruction must not affect queries; got %q", got)
		}
	})

	t.Run("no document instruction leaves document raw", func(t *testing.T) {
		var got string
		srv := captureInput(t, &got)
		defer srv.Close()
		c := New(srv.URL, "k", "m") // no WithDocumentInstruction
		if _, err := c.Embed(context.Background(), "the fox"); err != nil {
			t.Fatalf("Embed: %v", err)
		}
		if got != "the fox" {
			t.Errorf("got %q", got)
		}
	})
}

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
	if len(vec) != 3 {
		t.Fatalf("got %d dims, want 3", len(vec))
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
	if attrs["engram.embed.dims"] != "3" {
		t.Errorf("engram.embed.dims = %q, want 3", attrs["engram.embed.dims"])
	}
}

// TestEmbedNon2xxIncludesStatusAndBody proves a non-2xx embeddings response
// surfaces both the status code and the provider's own error text. Direct
// twin of summarize.TestSummarizeNon200IncludesStatusAndBody.
func TestEmbedNon2xxIncludesStatusAndBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"embedder overloaded"}`))
	}))
	defer srv.Close()

	_, err := New(srv.URL, "k", "m").Embed(context.Background(), "x")
	if err == nil {
		t.Fatal("want error on 503, got nil")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Fatalf("error missing status code: %v", err)
	}
	if !strings.Contains(err.Error(), "embedder overloaded") {
		t.Fatalf("error missing body detail: %v", err)
	}
}

// TestEmbedNon2xxDrainsForReuse proves a non-2xx embeddings response body is
// drained after the bounded error read, so the connection it arrived on is
// returned to the pool and reused by a second request to the same server.
// Without the drain, the second call opens a fresh connection and
// tracker.Reused() stays 0 — see the SUMMARY for the recorded RED transcript
// (drain temporarily commented out) that confirms this assertion can fail.
func TestEmbedNon2xxDrainsForReuse(t *testing.T) {
	// The fake error body is deliberately larger than maxErrorBodyBytes
	// (4096): if it fit inside the bound, the bounded read alone would
	// consume it entirely and the connection would be reusable with or
	// without the drain, proving nothing.
	bigBody := strings.Repeat("x", maxErrorBodyBytes*2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, bigBody)
	}))
	defer srv.Close()

	tracker := &testhttp.ReuseTracker{}
	// Both calls go through the same Client (and thus the same underlying
	// http.Client/Transport connection pool) — httptest's default client
	// would not share a pool across independently-constructed clients.
	c := New(srv.URL, "k", "m")
	ctx := tracker.Context(context.Background())

	if _, err := c.Embed(ctx, "x"); err == nil {
		t.Fatal("want error on 503, got nil")
	}
	if _, err := c.Embed(ctx, "y"); err == nil {
		t.Fatal("want error on 503, got nil")
	}

	if tracker.Reused() < 1 {
		t.Fatalf("want at least one reused connection, got Reused()=%d Total()=%d", tracker.Reused(), tracker.Total())
	}
}

// TestEmbedSuccessDecodeBounded proves the success-path decode is bounded:
// a response padded past a deliberately tiny WithMaxResponseBytes is
// rejected rather than read unbounded. Paired with a generous-bound control
// that succeeds over the identical body — without the control, a client that
// rejected everything would look green here too.
func TestEmbedSuccessDecodeBounded(t *testing.T) {
	big := strings.Repeat("0.1,", 1000)
	body := `{"data":[{"embedding":[` + big + `0.1]}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	tiny := New(srv.URL, "k", "m", WithMaxResponseBytes(16))
	if _, err := tiny.Embed(context.Background(), "x"); err == nil {
		t.Fatal("want decode error with a tiny WithMaxResponseBytes, got nil")
	}

	generous := New(srv.URL, "k", "m", WithMaxResponseBytes(int64(len(body)+1024)))
	vec, err := generous.Embed(context.Background(), "x")
	if err != nil {
		t.Fatalf("Embed with generous bound: %v", err)
	}
	if len(vec) == 0 {
		t.Fatal("want non-empty vector with generous bound")
	}
}
