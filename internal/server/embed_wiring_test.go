// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/seanb4t/engram/internal/config"
	"github.com/seanb4t/engram/internal/store"
)

// recordingEmbedder captures which embed method the handler called and with what
// input, then returns an error so the handler returns before touching the store
// (letting these assertions run without a live Qdrant).
type recordingEmbedder struct {
	method string
	input  string
}

var errStopBeforeStore = errors.New("stop before store")

func (r *recordingEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	r.method, r.input = "Embed", text
	return nil, errStopBeforeStore
}

func (r *recordingEmbedder) EmbedQuery(_ context.Context, text string) ([]float32, error) {
	r.method, r.input = "EmbedQuery", text
	return nil, errStopBeforeStore
}

// search_memory must embed the query via EmbedQuery (the instruction-aware,
// asymmetric path), not the document Embed path.
func TestSearchMemoryUsesEmbedQuery(t *testing.T) {
	rec := &recordingEmbedder{}
	d := &deps{em: rec}
	// Explicit anonymous caller (round-5 HIGH, Codex): the recordingEmbedder
	// returns errStopBeforeStore before any store access, so the anonymous
	// subject is never used for authz — the store-less stop-before-store
	// intent is preserved.
	_, err := d.searchMemory(context.Background(), caller{Subj: store.Anonymous()}, coreSearchRequest{Query: "why did pushover fail", Scope: "s"})
	if !errors.Is(err, errStopBeforeStore) {
		t.Fatalf("expected stop-before-store error, got %v", err)
	}
	if rec.method != "EmbedQuery" {
		t.Errorf("search embedded via %q, want EmbedQuery", rec.method)
	}
}

// store_memory must fold the record's tags into the embedded document text so
// curated tags contribute to vector recall.
func TestStoreMemoryEmbedsContentPlusTags(t *testing.T) {
	rec := &recordingEmbedder{}
	d := &deps{em: rec}
	// Explicit anonymous caller (round-5 HIGH, Codex): the recordingEmbedder
	// returns errStopBeforeStore before any store access, so the anonymous
	// subject is never used for authz — the store-less stop-before-store
	// intent is preserved.
	_, _, err := d.storeMemory(context.Background(), caller{Subj: store.Anonymous()}, storeArgs{
		Content:  "musl getaddrinfo fails on NODATA",
		Scope:    "s",
		Source:   "user-said",
		Category: "gotcha",
		Tags:     []string{"dns", "musl"},
	})
	if !errors.Is(err, errStopBeforeStore) {
		t.Fatalf("expected stop-before-store error, got %v", err)
	}
	if rec.method != "Embed" {
		t.Errorf("store embedded via %q, want Embed", rec.method)
	}
	if !strings.Contains(rec.input, "musl getaddrinfo fails on NODATA") ||
		!strings.Contains(rec.input, "tags: dns, musl") {
		t.Errorf("embed input missing content or tags line: %q", rec.input)
	}
}

// TestSummarizerFromConfigChatBaseURL pins D-12/D-13: the summarizer targets
// its own ENGRAM_OPENAI_CHAT_BASE_URL when configured, while the embedder
// keeps using the shared ENGRAM_OPENAI_BASE_URL regardless; and when the chat
// base URL is unset, the summarizer falls back to the shared base URL. srvA
// stands in for the shared/embeddings gateway (it can also serve chat
// completions, mirroring a real shared LiteLLM-style gateway); srvB stands in
// for a distinct hosted chat gateway.
func TestSummarizerFromConfigChatBaseURL(t *testing.T) {
	var aPath, bPath string
	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		aPath = r.URL.Path
		if strings.Contains(r.URL.Path, "chat/completions") {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"embedding": []float32{0.1}}},
		})
	}))
	defer srvA.Close()
	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bPath = r.URL.Path
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srvB.Close()

	t.Run("chat base URL set routes summarize to the second server", func(t *testing.T) {
		aPath, bPath = "", ""
		cfg := &config.Config{
			OpenAI:    config.OpenAIConfig{BaseURL: srvA.URL, ChatBaseURL: srvB.URL + "/v1", APIKey: "k"},
			Embed:     config.EmbedConfig{Model: "m"},
			Summarize: config.SummarizeConfig{Model: "m"},
		}

		em, err := embedderFromConfig(cfg)
		if err != nil {
			t.Fatalf("embedderFromConfig: %v", err)
		}
		if _, err := em.Embed(context.Background(), "x"); err != nil {
			t.Fatalf("Embed: %v", err)
		}
		if !strings.Contains(aPath, "embeddings") {
			t.Errorf("embed request did not reach the shared server: path=%q", aPath)
		}

		sm := summarizerFromConfig(cfg)
		if _, err := sm.Summarize(context.Background(), "x"); err != nil {
			t.Fatalf("Summarize: %v", err)
		}
		if bPath != "/v1/chat/completions" {
			t.Errorf("summarize request path = %q, want /v1/chat/completions (no doubled /v1) on the chat server", bPath)
		}
	})

	t.Run("chat base URL empty falls back to the shared server", func(t *testing.T) {
		aPath, bPath = "", ""
		cfg := &config.Config{
			OpenAI:    config.OpenAIConfig{BaseURL: srvA.URL, APIKey: "k"},
			Summarize: config.SummarizeConfig{Model: "m"},
		}

		sm := summarizerFromConfig(cfg)
		if _, err := sm.Summarize(context.Background(), "x"); err != nil {
			t.Fatalf("Summarize: %v", err)
		}
		if aPath != "/v1/chat/completions" {
			t.Errorf("summarize request path = %q, want /v1/chat/completions on the shared server", aPath)
		}
		if bPath != "" {
			t.Errorf("summarize request unexpectedly reached the chat-only server: path=%q", bPath)
		}
	})
}
