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
