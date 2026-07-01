// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package embed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestWithHTTPTransportIsApplied(t *testing.T) {
	marker := &markerTransport{}
	c := New("http://x", "k", "m", WithHTTPTransport(marker))
	if c.http.Transport != marker {
		t.Fatal("WithHTTPTransport did not set the client transport")
	}
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
