// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	flag "github.com/spf13/pflag"

	"github.com/seanb4t/engram/internal/config"
	"github.com/seanb4t/engram/internal/decide"
	"github.com/seanb4t/engram/internal/decide/jev"
	"github.com/seanb4t/engram/internal/store/storetest"
	"github.com/seanb4t/engram/internal/verdict"
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

// TestDeciderFromConfigProviderUnset proves the D-01/prohibition-P1
// off-by-default guarantee: with the provider unset, deciderFromConfig
// constructs nothing and touches no network, even with every other decisions
// field populated.
func TestDeciderFromConfigProviderUnset(t *testing.T) {
	var count int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt64(&count, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{}
	cfg.Decisions.BaseURL = srv.URL
	cfg.Decisions.APIKey = "k"
	cfg.Decisions.Timeout = "x"
	cfg.Decisions.Concurrency = "0"
	cfg.Decisions.Provider = ""

	for i := 0; i < 3; i++ {
		d, err := deciderFromConfig(cfg)
		if err != nil {
			t.Fatalf("deciderFromConfig (call %d): %v", i, err)
		}
		if d != nil {
			t.Fatalf("deciderFromConfig (call %d) returned a non-nil Decider for an unset provider", i)
		}
	}
	if got := atomic.LoadInt64(&count); got != 0 {
		t.Errorf("server received %d requests, want 0", got)
	}
}

// TestDeciderFromConfigUnknownProvider proves an unknown provider value fails
// startup by naming ENGRAM_DECISIONS_PROVIDER, rather than silently
// constructing nothing or panicking.
func TestDeciderFromConfigUnknownProvider(t *testing.T) {
	cfg := &config.Config{}
	cfg.Decisions.Provider = "bogus"

	d, err := deciderFromConfig(cfg)
	if d != nil {
		t.Error("deciderFromConfig returned a non-nil Decider for an unknown provider")
	}
	if err == nil || !strings.Contains(err.Error(), "ENGRAM_DECISIONS_PROVIDER") {
		t.Errorf("deciderFromConfig error = %v, want an error naming ENGRAM_DECISIONS_PROVIDER", err)
	}
}

// TestDeciderFromConfigAppliesOptions proves every config-driven jev.Option
// actually takes effect through the real wiring, not just that it's passed
// syntactically (D-02, D-10).
func TestDeciderFromConfigAppliesOptions(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(2 * time.Second)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(tracerNoulResponse))
		}))
		defer srv.Close()

		cfg := &config.Config{}
		cfg.Decisions.Provider = "jev"
		cfg.Decisions.BaseURL = srv.URL
		cfg.Decisions.Timeout = "150ms"

		d, err := deciderFromConfig(cfg)
		if err != nil {
			t.Fatalf("deciderFromConfig: %v", err)
		}

		req := decide.Request{
			State: decide.State{"a": "x"},
			Questions: map[string]decide.Question{
				"same_subject": decide.Noul("is it true", "true", "false"),
			},
		}

		start := time.Now()
		_, err = d.Decide(context.Background(), req)
		elapsed := time.Since(start)
		if err == nil {
			t.Fatal("Decide against a 2s-sleeping server with a 150ms timeout = nil error, want a timeout error")
		}
		if elapsed >= 1500*time.Millisecond {
			t.Errorf("Decide took %v to return, want under 1.5s (ENGRAM_DECISIONS_TIMEOUT=150ms not applied)", elapsed)
		}
	})

	t.Run("concurrency", func(t *testing.T) {
		var inFlight, peak int64
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			cur := atomic.AddInt64(&inFlight, 1)
			for {
				p := atomic.LoadInt64(&peak)
				if cur <= p || atomic.CompareAndSwapInt64(&peak, p, cur) {
					break
				}
			}
			time.Sleep(100 * time.Millisecond)
			atomic.AddInt64(&inFlight, -1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(tracerNoulResponse))
		}))
		defer srv.Close()

		cfg := &config.Config{}
		cfg.Decisions.Provider = "jev"
		cfg.Decisions.BaseURL = srv.URL
		cfg.Decisions.Concurrency = "2"

		d, err := deciderFromConfig(cfg)
		if err != nil {
			t.Fatalf("deciderFromConfig: %v", err)
		}

		req := decide.Request{
			State: decide.State{"a": "x"},
			Questions: map[string]decide.Question{
				"same_subject": decide.Noul("is it true", "true", "false"),
			},
		}
		reqs := make([]decide.Request, 6)
		for i := range reqs {
			reqs[i] = req
		}

		results := d.DecideMany(context.Background(), reqs)
		for i, r := range results {
			if r.Err != nil {
				t.Errorf("result[%d].Err = %v, want nil", i, r.Err)
			}
		}

		if got := atomic.LoadInt64(&peak); got != 2 {
			t.Errorf("peak in-flight = %d, want exactly 2 (ENGRAM_DECISIONS_CONCURRENCY=2 not applied)", got)
		}
	})
}

// TestDeciderResolverDefaults tables the five decisions* resolvers: empty or
// unparseable input falls back to its default; drain bytes/timeout honor an
// explicit 0; max_timeout and concurrency reject a non-positive value; a
// valid value passes through unchanged.
func TestDeciderResolverDefaults(t *testing.T) {
	t.Run("decisionsTimeout", func(t *testing.T) {
		cfg := &config.Config{}
		if got := decisionsTimeout(cfg); got != 10*time.Second {
			t.Errorf("empty = %v, want 10s", got)
		}
		cfg.Decisions.Timeout = "not-a-duration"
		if got := decisionsTimeout(cfg); got != 10*time.Second {
			t.Errorf("unparseable = %v, want 10s", got)
		}
		cfg.Decisions.Timeout = "250ms"
		if got := decisionsTimeout(cfg); got != 250*time.Millisecond {
			t.Errorf("valid = %v, want 250ms", got)
		}
	})

	t.Run("decisionsMaxTimeout", func(t *testing.T) {
		cfg := &config.Config{}
		if got := decisionsMaxTimeout(cfg); got != 10*time.Minute {
			t.Errorf("empty = %v, want 10m", got)
		}
		cfg.Decisions.MaxTimeout = "0"
		if got := decisionsMaxTimeout(cfg); got != 10*time.Minute {
			t.Errorf("zero = %v, want 10m", got)
		}
		cfg.Decisions.MaxTimeout = "-1s"
		if got := decisionsMaxTimeout(cfg); got != 10*time.Minute {
			t.Errorf("negative = %v, want 10m", got)
		}
		cfg.Decisions.MaxTimeout = "5m"
		if got := decisionsMaxTimeout(cfg); got != 5*time.Minute {
			t.Errorf("valid = %v, want 5m", got)
		}
	})

	t.Run("decisionsDrainBytes", func(t *testing.T) {
		cfg := &config.Config{}
		if got := decisionsDrainBytes(cfg); got != 262144 {
			t.Errorf("empty = %v, want 262144", got)
		}
		cfg.Decisions.DrainBytes = "not-a-number"
		if got := decisionsDrainBytes(cfg); got != 262144 {
			t.Errorf("unparseable = %v, want 262144", got)
		}
		cfg.Decisions.DrainBytes = "0"
		if got := decisionsDrainBytes(cfg); got != 0 {
			t.Errorf("zero = %v, want 0 (honored)", got)
		}
		cfg.Decisions.DrainBytes = "1024"
		if got := decisionsDrainBytes(cfg); got != 1024 {
			t.Errorf("valid = %v, want 1024", got)
		}
	})

	t.Run("decisionsDrainTimeout", func(t *testing.T) {
		cfg := &config.Config{}
		if got := decisionsDrainTimeout(cfg); got != 2*time.Second {
			t.Errorf("empty = %v, want 2s", got)
		}
		cfg.Decisions.DrainTimeout = "not-a-duration"
		if got := decisionsDrainTimeout(cfg); got != 2*time.Second {
			t.Errorf("unparseable = %v, want 2s", got)
		}
		cfg.Decisions.DrainTimeout = "0"
		if got := decisionsDrainTimeout(cfg); got != 0 {
			t.Errorf("zero = %v, want 0 (honored)", got)
		}
		cfg.Decisions.DrainTimeout = "5s"
		if got := decisionsDrainTimeout(cfg); got != 5*time.Second {
			t.Errorf("valid = %v, want 5s", got)
		}
	})

	t.Run("decisionsConcurrency", func(t *testing.T) {
		cfg := &config.Config{}
		if got := decisionsConcurrency(cfg); got != 4 {
			t.Errorf("empty = %v, want 4", got)
		}
		cfg.Decisions.Concurrency = "0"
		if got := decisionsConcurrency(cfg); got != 4 {
			t.Errorf("zero = %v, want 4", got)
		}
		cfg.Decisions.Concurrency = "-1"
		if got := decisionsConcurrency(cfg); got != 4 {
			t.Errorf("negative = %v, want 4", got)
		}
		cfg.Decisions.Concurrency = "8"
		if got := decisionsConcurrency(cfg); got != 8 {
			t.Errorf("valid = %v, want 8", got)
		}
	})
}

// TestDecisionsModelDefaultMatchesJev pins the registry's ENGRAM_DECISIONS_MODEL
// default to jev.DefaultModel by test, so the two literals cannot drift.
func TestDecisionsModelDefaultMatchesJev(t *testing.T) {
	t.Setenv("ENGRAM_DECISIONS_MODEL", "")
	cfg, err := config.Load(nil)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if cfg.Decisions.Model != jev.DefaultModel {
		t.Errorf("cfg.Decisions.Model = %q, want %q (jev.DefaultModel)", cfg.Decisions.Model, jev.DefaultModel)
	}
}

// TestDeciderEnabledLogLine proves logDeciderEnabled names the key source and
// the base URL's host only — never the key value, and never any userinfo,
// path or query from the base URL (T-02-08, T-02-05).
func TestDeciderEnabledLogLine(t *testing.T) {
	const base = "https://user:s3cret-userinfo@gateway.example/openrouter?x=1"

	cases := []struct {
		name       string
		ownKey     string
		openaiKey  string
		wantSource string
	}{
		{"own key wins", "own-key-VALUE", "", "ENGRAM_DECISIONS_API_KEY"},
		{"falls back to openai key", "", "openai-key-VALUE", "ENGRAM_OPENAI_API_KEY"},
		{"neither set", "", "", "none"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
			t.Cleanup(func() { slog.SetDefault(prev) })

			cfg := &config.Config{}
			cfg.Decisions.Provider = "jev"
			cfg.Decisions.BaseURL = base
			cfg.Decisions.Model = "typesafe/jev-1.13"
			cfg.Decisions.APIKey = tc.ownKey
			cfg.OpenAI.APIKey = tc.openaiKey

			logDeciderEnabled(cfg)

			out := buf.String()
			for _, forbidden := range []string{"s3cret-userinfo", "own-key-VALUE", "openai-key-VALUE", "/openrouter", "x=1"} {
				if strings.Contains(out, forbidden) {
					t.Errorf("log output contains forbidden substring %q: %s", forbidden, out)
				}
			}

			var found bool
			for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
				if line == "" {
					continue
				}
				var rec map[string]any
				if err := json.Unmarshal([]byte(line), &rec); err != nil {
					t.Fatalf("unmarshal log line %q: %v", line, err)
				}
				if rec["msg"] != "typed decisions enabled" {
					continue
				}
				found = true
				if rec["endpoint_host"] != "gateway.example" {
					t.Errorf("endpoint_host = %v, want gateway.example", rec["endpoint_host"])
				}
				if rec["api_key_source"] != tc.wantSource {
					t.Errorf("api_key_source = %v, want %v", rec["api_key_source"], tc.wantSource)
				}
			}
			if !found {
				t.Fatal("no 'typed decisions enabled' record found in log output")
			}
		})
	}
}

// TestStoreAndDeciderFromEnv proves the curation-verdict wiring seam
// (D-04): with ENGRAM_DECISIONS_PROVIDER empty it returns a non-nil store,
// a nil decider, a nil error, and settings equal to
// verdict.DefaultThreshold/verdict.DefaultStateChars; with provider "jev"
// and a base URL it returns a non-nil decider; each call loads config
// exactly once through the configLoad seam.
func TestStoreAndDeciderFromEnv(t *testing.T) {
	addr := storetest.Addr()
	if addr == "" {
		storetest.SkipOrFailNoQdrant(t)
	}

	t.Run("provider unset", func(t *testing.T) {
		t.Setenv("ENGRAM_QDRANT_ADDR", addr)
		t.Setenv("ENGRAM_QDRANT_COLLECTION", testCollection("mem_store_and_decider_unset_test"))
		t.Setenv("ENGRAM_EMBED_DIM", "3")
		t.Setenv("ENGRAM_SUMMARY_MODEL", "")
		t.Setenv("ENGRAM_SUMMARY_ON_WRITE", "")
		t.Setenv("ENGRAM_DECISIONS_PROVIDER", "")

		loads := 0
		orig := configLoad
		configLoad = func(flags *flag.FlagSet) (*config.Config, error) {
			loads++
			return orig(flags)
		}
		t.Cleanup(func() { configLoad = orig })

		st, dec, settings, err := StoreAndDeciderFromEnv()
		if err != nil {
			t.Fatalf("StoreAndDeciderFromEnv: %v", err)
		}
		if st == nil {
			t.Error("StoreAndDeciderFromEnv returned a nil store")
		}
		if dec != nil {
			t.Error("StoreAndDeciderFromEnv with ENGRAM_DECISIONS_PROVIDER unset returned a non-nil decider, want nil (D-04)")
		}
		// Threshold/StateChars only — Provider/Model/EndpointHost are
		// populated from cfg.Decisions unconditionally (03-05's D-08/D-09
		// resolver has no provider gate), so this subtest, which does not
		// clear ENGRAM_DECISIONS_MODEL, is not the place to pin their
		// value; TestDeciderFromEnv/provider_unset covers that.
		if settings.Threshold != verdict.DefaultThreshold {
			t.Errorf("settings.Threshold = %v, want %v (verdict.DefaultThreshold)", settings.Threshold, verdict.DefaultThreshold)
		}
		if settings.StateChars != verdict.DefaultStateChars {
			t.Errorf("settings.StateChars = %v, want %v (verdict.DefaultStateChars)", settings.StateChars, verdict.DefaultStateChars)
		}
		if loads != 1 {
			t.Errorf("StoreAndDeciderFromEnv loaded config %d times, want exactly 1", loads)
		}
	})

	t.Run("provider jev", func(t *testing.T) {
		t.Setenv("ENGRAM_QDRANT_ADDR", addr)
		t.Setenv("ENGRAM_QDRANT_COLLECTION", testCollection("mem_store_and_decider_jev_test"))
		t.Setenv("ENGRAM_EMBED_DIM", "3")
		t.Setenv("ENGRAM_SUMMARY_MODEL", "")
		t.Setenv("ENGRAM_SUMMARY_ON_WRITE", "")
		t.Setenv("ENGRAM_DECISIONS_PROVIDER", "jev")
		t.Setenv("ENGRAM_DECISIONS_BASE_URL", "https://example.invalid/api")

		st, dec, _, err := StoreAndDeciderFromEnv()
		if err != nil {
			t.Fatalf("StoreAndDeciderFromEnv: %v", err)
		}
		if st == nil {
			t.Error("StoreAndDeciderFromEnv returned a nil store")
		}
		if dec == nil {
			t.Error("StoreAndDeciderFromEnv with ENGRAM_DECISIONS_PROVIDER=jev returned a nil decider, want non-nil")
		}
	})
}

// clearVerdictEnv isolates a test from ambient ENGRAM_DECISIONS_* env vars
// (D-08/D-09's knobs and the provider/base-url fields verdictSettings and
// DeciderFromEnv read), so each subtest below starts from a known state
// regardless of what the host environment or a sibling test left set.
func clearVerdictEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"ENGRAM_DECISIONS_PROVIDER", "ENGRAM_DECISIONS_BASE_URL", "ENGRAM_DECISIONS_MODEL",
		"ENGRAM_DECISIONS_API_KEY", "ENGRAM_DECISIONS_VERDICT_THRESHOLD", "ENGRAM_DECISIONS_VERDICT_STATE_CHARS",
	} {
		t.Setenv(name, "")
	}
}

// TestVerdictSettingsDefaultsMatchVerdictPackage proves the assumption-delta
// invariant (D-08, D-09): with both registered knobs empty,
// config.Load(nil)'s registered defaults resolve through verdictSettings to
// exactly verdict.DefaultThreshold/verdict.DefaultStateChars — the registry
// default and the package fallback cannot drift.
func TestVerdictSettingsDefaultsMatchVerdictPackage(t *testing.T) {
	clearVerdictEnv(t)

	cfg, err := config.Load(nil)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	got := verdictSettings(cfg)
	if got.Threshold != verdict.DefaultThreshold {
		t.Errorf("Threshold = %v, want %v (verdict.DefaultThreshold)", got.Threshold, verdict.DefaultThreshold)
	}
	if got.StateChars != verdict.DefaultStateChars {
		t.Errorf("StateChars = %v, want %v (verdict.DefaultStateChars)", got.StateChars, verdict.DefaultStateChars)
	}
}

// TestVerdictSettingsResolvesKnobs proves 0.75/800 resolve as given, and
// that an unparseable or out-of-range threshold (2, NaN, abc) and an
// unparseable or non-positive state-chars value (0, abc) each fall back to
// the package defaults.
func TestVerdictSettingsResolvesKnobs(t *testing.T) {
	t.Run("valid values resolve as given", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Decisions.VerdictThreshold = "0.75"
		cfg.Decisions.VerdictStateChars = "800"
		got := verdictSettings(cfg)
		if got.Threshold != 0.75 {
			t.Errorf("Threshold = %v, want 0.75", got.Threshold)
		}
		if got.StateChars != 800 {
			t.Errorf("StateChars = %v, want 800", got.StateChars)
		}
	})

	for _, bad := range []string{"2", "NaN", "abc"} {
		t.Run("threshold falls back on "+bad, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Decisions.VerdictThreshold = bad
			cfg.Decisions.VerdictStateChars = "800"
			got := verdictSettings(cfg)
			if got.Threshold != verdict.DefaultThreshold {
				t.Errorf("Threshold = %v, want %v (default)", got.Threshold, verdict.DefaultThreshold)
			}
		})
	}

	for _, bad := range []string{"0", "abc"} {
		t.Run("state chars falls back on "+bad, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Decisions.VerdictThreshold = "0.75"
			cfg.Decisions.VerdictStateChars = bad
			got := verdictSettings(cfg)
			if got.StateChars != verdict.DefaultStateChars {
				t.Errorf("StateChars = %v, want %v (default)", got.StateChars, verdict.DefaultStateChars)
			}
		})
	}
}

// TestVerdictSettingsEndpointHostOnly proves T-03-04's mitigation: a base
// URL carrying userinfo, path and query resolves to EndpointHost
// "decisions.example.com:8443" only — the settings value contains no
// "secret", "openrouter" or "k=v".
func TestVerdictSettingsEndpointHostOnly(t *testing.T) {
	cfg := &config.Config{}
	cfg.Decisions.BaseURL = "https://user:secret@decisions.example.com:8443/openrouter?k=v"

	got := verdictSettings(cfg)
	if got.EndpointHost != "decisions.example.com:8443" {
		t.Errorf("EndpointHost = %q, want %q", got.EndpointHost, "decisions.example.com:8443")
	}
	full := fmt.Sprintf("%+v", got)
	for _, forbidden := range []string{"secret", "openrouter", "k=v"} {
		if strings.Contains(full, forbidden) {
			t.Errorf("VerdictSettings %+v contains forbidden substring %q", got, forbidden)
		}
	}
}

// TestDeciderFromEnv proves the curation-eval wiring seam: provider empty
// returns a nil decider and a nil error with exactly one config load;
// provider "jev" plus a base URL returns a non-nil decider and settings
// with Provider "jev" and the configured model. Isolated from ambient
// ENGRAM_DECISIONS_* env with t.Setenv.
func TestDeciderFromEnv(t *testing.T) {
	t.Run("provider unset", func(t *testing.T) {
		clearVerdictEnv(t)

		loads := 0
		orig := configLoad
		configLoad = func(flags *flag.FlagSet) (*config.Config, error) {
			loads++
			return orig(flags)
		}
		t.Cleanup(func() { configLoad = orig })

		dec, settings, err := DeciderFromEnv()
		if err != nil {
			t.Fatalf("DeciderFromEnv: %v", err)
		}
		if dec != nil {
			t.Error("DeciderFromEnv with ENGRAM_DECISIONS_PROVIDER unset returned a non-nil decider, want nil")
		}
		// Threshold/StateChars/Provider only — Model resolves to the
		// registry default (jev.DefaultModel) regardless of provider, since
		// 03-05's resolver copies cfg.Decisions.Model unconditionally.
		if settings.Threshold != verdict.DefaultThreshold {
			t.Errorf("settings.Threshold = %v, want %v (verdict.DefaultThreshold)", settings.Threshold, verdict.DefaultThreshold)
		}
		if settings.StateChars != verdict.DefaultStateChars {
			t.Errorf("settings.StateChars = %v, want %v (verdict.DefaultStateChars)", settings.StateChars, verdict.DefaultStateChars)
		}
		if settings.Provider != "" {
			t.Errorf("settings.Provider = %q, want \"\"", settings.Provider)
		}
		if loads != 1 {
			t.Errorf("DeciderFromEnv loaded config %d times, want exactly 1", loads)
		}
	})

	t.Run("provider jev", func(t *testing.T) {
		clearVerdictEnv(t)
		t.Setenv("ENGRAM_DECISIONS_PROVIDER", "jev")
		t.Setenv("ENGRAM_DECISIONS_BASE_URL", "https://example.invalid/api")
		t.Setenv("ENGRAM_DECISIONS_MODEL", "typesafe/jev-1.13")

		dec, settings, err := DeciderFromEnv()
		if err != nil {
			t.Fatalf("DeciderFromEnv: %v", err)
		}
		if dec == nil {
			t.Error("DeciderFromEnv with ENGRAM_DECISIONS_PROVIDER=jev returned a nil decider, want non-nil")
		}
		if settings.Provider != "jev" {
			t.Errorf("settings.Provider = %q, want %q", settings.Provider, "jev")
		}
		if settings.Model != "typesafe/jev-1.13" {
			t.Errorf("settings.Model = %q, want %q", settings.Model, "typesafe/jev-1.13")
		}
	})
}

// TestSearchRerankTimeoutResolver proves searchRerankTimeout's resolution
// table (D-09): 2s default on empty; an explicit valid value honored; 0,
// negative and unparseable values all fall back to 2s.
func TestSearchRerankTimeoutResolver(t *testing.T) {
	cfg := &config.Config{}
	if got := searchRerankTimeout(cfg); got != 2*time.Second {
		t.Errorf("empty = %v, want 2s", got)
	}
	cfg.Search.RerankTimeout = "150ms"
	if got := searchRerankTimeout(cfg); got != 150*time.Millisecond {
		t.Errorf("150ms = %v, want 150ms", got)
	}
	for _, bad := range []string{"0", "-1s", "soon"} {
		cfg.Search.RerankTimeout = bad
		if got := searchRerankTimeout(cfg); got != 2*time.Second {
			t.Errorf("%q = %v, want fallback 2s", bad, got)
		}
	}
}

// TestSearchRankHookFromConfig proves searchRankHook's D-01 gate: provider
// jev with ranker lexical gives a nil hook (the ranker decides, never the
// provider alone); ranker jev with provider jev gives a non-nil hook;
// provider empty (even with ranker jev, a misconfiguration Config.Validate
// normally rejects before this is reached in production) gives a nil hook
// and no error.
func TestSearchRankHookFromConfig(t *testing.T) {
	t.Run("provider jev, ranker lexical gives nil hook", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Decisions.Provider = "jev"
		cfg.Decisions.BaseURL = "https://example.invalid/api"
		cfg.Search.Ranker = "lexical"

		hook, err := searchRankHook(cfg)
		if err != nil {
			t.Fatalf("searchRankHook: %v", err)
		}
		if hook != nil {
			t.Error("searchRankHook with ranker=lexical returned a non-nil hook, want nil")
		}
	})

	t.Run("ranker jev gives non-nil hook", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Decisions.Provider = "jev"
		cfg.Decisions.BaseURL = "https://example.invalid/api"
		cfg.Search.Ranker = "jev"
		cfg.Search.RerankTimeout = "2s"

		hook, err := searchRankHook(cfg)
		if err != nil {
			t.Fatalf("searchRankHook: %v", err)
		}
		if hook == nil {
			t.Error("searchRankHook with ranker=jev returned a nil hook, want non-nil")
		}
	})

	t.Run("provider empty gives nil hook and no error", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Search.Ranker = "jev"

		hook, err := searchRankHook(cfg)
		if err != nil {
			t.Fatalf("searchRankHook: %v", err)
		}
		if hook != nil {
			t.Error("searchRankHook with an empty provider returned a non-nil hook, want nil")
		}
	})
}

// TestSearchRankerEnabledLogLine mirrors TestDeciderEnabledLogLine over the
// same three key-source cases: one Info record "search reranking enabled"
// carrying ranker, model, endpoint_host (host only), rerank_timeout and
// api_key_source — the key values, userinfo, path and query never appear.
func TestSearchRankerEnabledLogLine(t *testing.T) {
	const base = "https://user:s3cret-userinfo@gateway.example/openrouter?x=1"

	cases := []struct {
		name       string
		ownKey     string
		openaiKey  string
		wantSource string
	}{
		{"own key wins", "own-key-VALUE", "", "ENGRAM_DECISIONS_API_KEY"},
		{"falls back to openai key", "", "openai-key-VALUE", "ENGRAM_OPENAI_API_KEY"},
		{"neither set", "", "", "none"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
			t.Cleanup(func() { slog.SetDefault(prev) })

			cfg := &config.Config{}
			cfg.Decisions.Provider = "jev"
			cfg.Decisions.BaseURL = base
			cfg.Decisions.Model = "typesafe/jev-1.13"
			cfg.Decisions.APIKey = tc.ownKey
			cfg.OpenAI.APIKey = tc.openaiKey
			cfg.Search.Ranker = "jev"
			cfg.Search.RerankTimeout = "2s"

			logSearchRankerEnabled(cfg)

			out := buf.String()
			for _, forbidden := range []string{"s3cret-userinfo", "own-key-VALUE", "openai-key-VALUE", "/openrouter", "x=1"} {
				if strings.Contains(out, forbidden) {
					t.Errorf("log output contains forbidden substring %q: %s", forbidden, out)
				}
			}

			var found bool
			for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
				if line == "" {
					continue
				}
				var rec map[string]any
				if err := json.Unmarshal([]byte(line), &rec); err != nil {
					t.Fatalf("unmarshal log line %q: %v", line, err)
				}
				if rec["msg"] != "search reranking enabled" {
					continue
				}
				found = true
				if rec["ranker"] != "jev" {
					t.Errorf("ranker = %v, want jev", rec["ranker"])
				}
				if rec["endpoint_host"] != "gateway.example" {
					t.Errorf("endpoint_host = %v, want gateway.example", rec["endpoint_host"])
				}
				if rec["api_key_source"] != tc.wantSource {
					t.Errorf("api_key_source = %v, want %v", rec["api_key_source"], tc.wantSource)
				}
				if _, ok := rec["rerank_timeout"]; !ok {
					t.Error("rerank_timeout missing from log record")
				}
			}
			if !found {
				t.Fatal("no 'search reranking enabled' record found in log output")
			}
		})
	}
}

// TestDeciderFromConfigStillRetries proves the consolidate-path client
// deciderFromConfig builds is UNCHANGED by this plan: against a
// 503-then-200 handler it makes two requests and succeeds — the search
// path's dedicated no-retry client (searchDeciderFromConfig) never affects
// this one (D-09).
func TestDeciderFromConfigStillRetries(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&n, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(tracerNoulResponse))
	}))
	defer srv.Close()

	cfg := &config.Config{}
	cfg.Decisions.Provider = "jev"
	cfg.Decisions.BaseURL = srv.URL

	d, err := deciderFromConfig(cfg)
	if err != nil {
		t.Fatalf("deciderFromConfig: %v", err)
	}

	req := decide.Request{
		State: decide.State{"a": "x"},
		Questions: map[string]decide.Question{
			"same_subject": decide.Noul("is it true", "true", "false"),
		},
	}
	if _, err := d.Decide(context.Background(), req); err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if got := atomic.LoadInt32(&n); got != 2 {
		t.Errorf("requests = %d, want 2 (one retry)", got)
	}
}

// TestSearchRankHookFromEnv proves the retrieval eval's wiring seam (D-02):
// provider unset gives a nil hook and Enabled false; provider set (ranker
// lexical, the eval measures the Jev row regardless) gives a non-nil hook,
// Enabled true, EndpointHost host-only and Timeout 2s; one config load per
// call.
func TestSearchRankHookFromEnv(t *testing.T) {
	t.Run("provider unset", func(t *testing.T) {
		clearVerdictEnv(t)
		t.Setenv("ENGRAM_SEARCH_RANKER", "")

		loads := 0
		orig := configLoad
		configLoad = func(flags *flag.FlagSet) (*config.Config, error) {
			loads++
			return orig(flags)
		}
		t.Cleanup(func() { configLoad = orig })

		hook, info, err := SearchRankHookFromEnv()
		if err != nil {
			t.Fatalf("SearchRankHookFromEnv: %v", err)
		}
		if hook != nil {
			t.Error("SearchRankHookFromEnv with provider unset returned a non-nil hook, want nil")
		}
		if info.Enabled {
			t.Error("SearchRankHookFromEnv with provider unset returned Enabled=true, want false")
		}
		if loads != 1 {
			t.Errorf("SearchRankHookFromEnv loaded config %d times, want exactly 1", loads)
		}
	})

	t.Run("provider set, ranker lexical", func(t *testing.T) {
		clearVerdictEnv(t)
		t.Setenv("ENGRAM_DECISIONS_PROVIDER", "jev")
		t.Setenv("ENGRAM_DECISIONS_BASE_URL", "https://user:secret@example.invalid:8443/openrouter?k=v")
		t.Setenv("ENGRAM_DECISIONS_MODEL", "typesafe/jev-1.13")
		t.Setenv("ENGRAM_SEARCH_RANKER", "lexical")
		t.Setenv("ENGRAM_SEARCH_RERANK_TIMEOUT", "")

		hook, info, err := SearchRankHookFromEnv()
		if err != nil {
			t.Fatalf("SearchRankHookFromEnv: %v", err)
		}
		if hook == nil {
			t.Fatal("SearchRankHookFromEnv with provider set returned a nil hook, want non-nil (measures the Jev row regardless of ranker)")
		}
		if !info.Enabled {
			t.Error("info.Enabled = false, want true")
		}
		if info.EndpointHost != "example.invalid:8443" {
			t.Errorf("info.EndpointHost = %q, want %q", info.EndpointHost, "example.invalid:8443")
		}
		if info.Timeout != 2*time.Second {
			t.Errorf("info.Timeout = %v, want 2s", info.Timeout)
		}
		if info.Model != "typesafe/jev-1.13" {
			t.Errorf("info.Model = %q, want %q", info.Model, "typesafe/jev-1.13")
		}
	})
}
