// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package summarize

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSummarizePostsChatCompletionAndReturnsContent(t *testing.T) {
	var gotModel, gotPath, gotSystem, gotUser string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		var req chatReq
		_ = json.Unmarshal(body, &req)
		gotModel = req.Model
		for _, m := range req.Messages {
			switch m.Role {
			case "system":
				gotSystem = m.Content
			case "user":
				gotUser = m.Content
			}
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"  do NOT remove --flag  "}}]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "k", "summary-cheap", 280)
	out, err := c.Summarize(context.Background(), "the full memory content here")
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if out != "do NOT remove --flag" {
		t.Fatalf("summary not trimmed/returned: %q", out)
	}
	if gotPath != "/v1/chat/completions" || gotModel != "summary-cheap" {
		t.Fatalf("wrong request: path=%q model=%q", gotPath, gotModel)
	}
	if !strings.Contains(gotSystem, "Preserve") || gotUser != "the full memory content here" {
		t.Fatalf("messages malformed: system=%q user=%q", gotSystem, gotUser)
	}
}

func TestSummarizeErrorsOnEmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()
	if _, err := New(srv.URL, "k", "m", 280).Summarize(context.Background(), "x"); err == nil {
		t.Fatal("want error on empty choices, got nil")
	}
}

func TestSummarizeNon200IncludesStatusAndBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"model overloaded"}`))
	}))
	defer srv.Close()
	_, err := New(srv.URL, "k", "m", 280).Summarize(context.Background(), "x")
	if err == nil {
		t.Fatal("want error on 503, got nil")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Fatalf("error missing status code: %v", err)
	}
	if !strings.Contains(err.Error(), "model overloaded") {
		t.Fatalf("error missing body detail: %v", err)
	}
}

func TestSummarizeDefaultMaxTokensDecoupledFromMaxChars(t *testing.T) {
	var gotMaxTokens int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req chatReq
		_ = json.Unmarshal(body, &req)
		gotMaxTokens = req.MaxTokens
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	// maxChars=8 would have produced the old starving budget (8/3+32 = 34);
	// the default must instead be the generous, decoupled ceiling.
	if _, err := New(srv.URL, "k", "m", 8).Summarize(context.Background(), "x"); err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if gotMaxTokens != defaultMaxTokens {
		t.Fatalf("max_tokens = %d, want default %d (decoupled from maxChars)", gotMaxTokens, defaultMaxTokens)
	}
}

func TestSummarizeWithMaxTokensOverrideAndOmit(t *testing.T) {
	var rawBody string
	var gotMaxTokens int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rawBody = string(body)
		var req chatReq
		_ = json.Unmarshal(body, &req)
		gotMaxTokens = req.MaxTokens
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	if _, err := New(srv.URL, "k", "m", 280, WithMaxTokens(4096)).Summarize(context.Background(), "x"); err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if gotMaxTokens != 4096 {
		t.Fatalf("max_tokens = %d, want 4096", gotMaxTokens)
	}

	// 0 (and negatives, clamped to 0) must omit the field so the gateway decides.
	if _, err := New(srv.URL, "k", "m", 280, WithMaxTokens(0)).Summarize(context.Background(), "x"); err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if strings.Contains(rawBody, "max_tokens") {
		t.Fatalf("max_tokens must be omitted when 0, got body: %s", rawBody)
	}
}

func TestSummarizeWithTimeoutCancelsSlowRequest(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		<-release // never responds before the client's timeout elapses
	}))
	// Defers run LIFO: close(release) first unblocks the handler, then Close()
	// returns promptly instead of waiting on the still-running request.
	defer srv.Close()
	defer close(release)

	_, err := New(srv.URL, "k", "m", 280, WithTimeout(20*time.Millisecond)).Summarize(context.Background(), "x")
	if err == nil {
		t.Fatal("want timeout error from slow gateway, got nil")
	}
}

func TestSummarizeTruncatesToMaxChars(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"aaaaaaaaaaaaaaaaaaaa"}}]}`)) // 20 chars
	}))
	defer srv.Close()
	out, err := New(srv.URL, "k", "m", 8).Summarize(context.Background(), "x")
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if len([]rune(out)) > 8 {
		t.Fatalf("summary not truncated to 8: %q (len %d)", out, len([]rune(out)))
	}
}
