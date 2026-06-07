// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package embed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
