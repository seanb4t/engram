// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/seanb4t/engram/internal/config"
	"github.com/seanb4t/engram/internal/decide"
)

// tracerNoulResponse is spike 001's "happy" probe body with the relation
// choice answer removed, keeping only the same_subject noul answer, usage,
// dated model snapshot, id and provider.
const tracerNoulResponse = `{
	"model": "typesafe/jev-1.13-20260917",
	"answers": {
		"same_subject": {"type": "noul", "noul": 0.92}
	},
	"usage": {"input_tokens": 497, "output_tokens": 67, "cost": 0.000020874},
	"id": "gen-dec-test-001",
	"provider": "TypeSafe"
}`

// TestDeciderTracerEndToEnd proves the tracer slice end to end (Task 1): a
// config.Config with Decisions.Provider "jev" flows through
// deciderFromConfig, decide.Decider and the jev backend to
// {base}/alpha/decisions and back as a typed noul answer, with usage and a
// fully-attributed "decide" span.
func TestDeciderTracerEndToEnd(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(tracerNoulResponse))
	}))
	defer srv.Close()

	sr := tracetest.NewSpanRecorder()
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr)))
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	cfg := &config.Config{}
	cfg.Decisions.Provider = "jev"
	cfg.Decisions.BaseURL = srv.URL + "/api"
	cfg.OpenAI.APIKey = "fallback-key"

	d, err := deciderFromConfig(cfg)
	if err != nil {
		t.Fatalf("deciderFromConfig: %v", err)
	}
	if d == nil {
		t.Fatal("deciderFromConfig returned a nil Decider for provider=jev")
	}

	req := decide.Request{
		State: decide.State{
			"record_a": "synthetic text A",
			"record_b": "synthetic text B",
		},
		Questions: map[string]decide.Question{
			"same_subject": decide.Noul(
				"Do record_a and record_b describe the same subject?",
				"the records describe the same subject",
				"the records describe different subjects",
			),
		},
	}

	resp, err := d.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/alpha/decisions" {
		t.Errorf("path = %q, want /api/alpha/decisions", gotPath)
	}
	if gotAuth != "Bearer fallback-key" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer fallback-key")
	}

	if gotBody["model"] != "typesafe/jev-1.13" {
		t.Errorf("request model = %v, want typesafe/jev-1.13", gotBody["model"])
	}
	questions, _ := gotBody["questions"].(map[string]any)
	sameSubject, _ := questions["same_subject"].(map[string]any)
	if sameSubject["type"] != "noul" {
		t.Errorf("questions.same_subject.type = %v, want noul", sameSubject["type"])
	}
	criteria, _ := sameSubject["criteria"].(map[string]any)
	if trueVal, _ := criteria["true"].(string); trueVal == "" {
		t.Errorf("questions.same_subject.criteria.true is empty")
	}
	if falseVal, _ := criteria["false"].(string); falseVal == "" {
		t.Errorf("questions.same_subject.criteria.false is empty")
	}

	answer, ok := resp.Answers["same_subject"]
	if !ok {
		t.Fatal("resp.Answers has no same_subject entry")
	}
	if answer.Probability < 0.87 || answer.Probability > 0.97 {
		t.Errorf("Probability = %v, want in [0.87, 0.97]", answer.Probability)
	}
	if resp.Model != "typesafe/jev-1.13-20260917" {
		t.Errorf("Model = %q, want the dated snapshot", resp.Model)
	}
	if resp.Usage == nil {
		t.Fatal("Usage is nil")
	}
	if resp.Usage.InputTokens != 497 {
		t.Errorf("InputTokens = %d, want 497", resp.Usage.InputTokens)
	}
	if resp.Usage.CostUSD == nil {
		t.Fatal("CostUSD is nil")
	}
	const wantCost = 0.000020874
	if diff := *resp.Usage.CostUSD - wantCost; diff < -1e-12 || diff > 1e-12 {
		t.Errorf("CostUSD = %v, want within 1e-12 of %v", *resp.Usage.CostUSD, wantCost)
	}

	// otelhttp adds its own client span alongside the "decide" span, so find
	// it by name rather than indexing [0].
	var decideSpans int
	for _, span := range sr.Ended() {
		if span.Name() != "decide" {
			continue
		}
		decideSpans++
		attrs := map[string]string{}
		for _, kv := range span.Attributes() {
			attrs[string(kv.Key)] = kv.Value.String()
		}
		want := map[string]string{
			"engram.decide.provider":       "jev",
			"engram.decide.model":          "typesafe/jev-1.13",
			"engram.decide.questions":      "1",
			"engram.decide.model_snapshot": "typesafe/jev-1.13-20260917",
			"engram.decide.input_tokens":   "497",
			"engram.decide.output_tokens":  "67",
			"engram.decide.status":         "ok",
		}
		for k, v := range want {
			if attrs[k] != v {
				t.Errorf("span attribute %s = %q, want %q", k, attrs[k], v)
			}
		}
		if _, ok := attrs["engram.decide.cost_usd"]; !ok {
			t.Errorf("span is missing engram.decide.cost_usd attribute")
		}
	}
	if decideSpans != 1 {
		t.Fatalf("found %d spans named decide, want exactly 1 (sr.Ended()=%v)", decideSpans, sr.Ended())
	}

	t.Run("own key wins", func(t *testing.T) {
		var subAuth string
		subSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(tracerNoulResponse))
		}))
		defer subSrv.Close()

		subCfg := &config.Config{}
		subCfg.Decisions.Provider = "jev"
		subCfg.Decisions.BaseURL = subSrv.URL + "/api"
		subCfg.Decisions.APIKey = "own-key"
		subCfg.OpenAI.APIKey = "fallback-key"

		subD, err := deciderFromConfig(subCfg)
		if err != nil {
			t.Fatalf("deciderFromConfig: %v", err)
		}
		if _, err := subD.Decide(context.Background(), req); err != nil {
			t.Fatalf("Decide: %v", err)
		}
		if subAuth != "Bearer own-key" {
			t.Errorf("Authorization = %q, want %q", subAuth, "Bearer own-key")
		}
	})
}
