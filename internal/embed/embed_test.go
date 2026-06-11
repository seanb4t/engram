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
