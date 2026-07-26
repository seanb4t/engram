// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package openaiurl

import "testing"

// TestJoin is the invariant test for D-14: it crosses every documented
// provider base-URL shape against both lane suffixes (embeddings,
// chat/completions), plus the query/fragment operator-error-scope edge case.
// It goes red the instant a future change reintroduces a lane-specific join,
// which is why the heuristic lives here once instead of being duplicated.
func TestJoin(t *testing.T) {
	shapes := []struct {
		name    string
		baseURL string
		root    string // the OpenAI-compat root the shape resolves to, empty when it appends "/v1/"
	}{
		{"bare host", "https://api.openai.com", ""},
		{"bare host, trailing slash", "https://api.openai.com/", ""},
		{"LiteLLM bare host, trailing slash", "http://localhost:4000/", ""},
		{"/v1 path", "https://api.openai.com/v1", "/v1"},
		{"/v1 path, trailing slash", "https://api.openai.com/v1/", "/v1"},
		{"/v1beta/openai path (Gemini)", "https://generativelanguage.googleapis.com/v1beta/openai", "/v1beta/openai"},
		{"/v10 near-miss, NOT a /v1 root", "https://example.com/v10", ""},
	}
	suffixes := []string{"embeddings", "chat/completions"}

	for _, shape := range shapes {
		for _, suffix := range suffixes {
			t.Run(shape.name+"/"+suffix, func(t *testing.T) {
				trimmed := shape.baseURL
				for len(trimmed) > 0 && trimmed[len(trimmed)-1] == '/' {
					trimmed = trimmed[:len(trimmed)-1]
				}
				var want string
				if shape.root != "" {
					want = trimmed + "/" + suffix
				} else {
					want = trimmed + "/v1/" + suffix
				}
				if got := Join(shape.baseURL, suffix); got != want {
					t.Errorf("Join(%q, %q) = %q, want %q", shape.baseURL, suffix, got, want)
				}
			})
		}
	}

	// Operator-error scope (accepted, not canonicalized — T-13-01 trust
	// boundary parity with ENGRAM_OPENAI_BASE_URL validation): a
	// query/fragment-bearing base URL is not stripped before the suffix check,
	// so it never matches "/v1" and falls through to the "/v1/<suffix>" append,
	// producing a URL with the query string stuck mid-path. No documented
	// provider shape carries a query/fragment.
	t.Run("query/fragment base URL (operator-error scope, pinned)", func(t *testing.T) {
		if got, want := Join("http://localhost:4000/v1?debug=1", "embeddings"), "http://localhost:4000/v1?debug=1/v1/embeddings"; got != want {
			t.Errorf("Join(...) = %q, want %q", got, want)
		}
		if got, want := Join("http://localhost:4000/v1?debug=1", "chat/completions"), "http://localhost:4000/v1?debug=1/v1/chat/completions"; got != want {
			t.Errorf("Join(...) = %q, want %q", got, want)
		}
	})

	// Behavior-preservation pin (SC3): the shipped default must produce the
	// byte-identical endpoint to today's naive concat for both lanes.
	t.Run("shipped default is byte-identical to today", func(t *testing.T) {
		if got, want := Join("http://localhost:4000", "embeddings"), "http://localhost:4000/v1/embeddings"; got != want {
			t.Errorf("Join(default, embeddings) = %q, want %q", got, want)
		}
		if got, want := Join("http://localhost:4000", "chat/completions"), "http://localhost:4000/v1/chat/completions"; got != want {
			t.Errorf("Join(default, chat/completions) = %q, want %q", got, want)
		}
	})
}
