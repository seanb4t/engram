// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package jev

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/seanb4t/engram/internal/decide"
)

// globalSpanRecorder and globalSpanRecorderOnce back ensureGlobalSpanRecorder
// below. OTel's global package permanently binds a Tracer obtained via
// otel.Tracer(name) (this package's package-level "tracer" var, obtained
// once at package init) to whichever TracerProvider is installed FIRST via
// otel.SetTracerProvider — a later call does not rebind an
// already-delegated Tracer. Every span test in this file therefore shares
// ONE recorder, installed exactly once, and each test filters to the spans
// it produced by index range instead of swapping providers per subtest.
var (
	globalSpanRecorder     = tracetest.NewSpanRecorder()
	globalSpanRecorderOnce sync.Once
)

// ensureGlobalSpanRecorder installs globalSpanRecorder as the process-wide
// OTel TracerProvider exactly once and returns it.
func ensureGlobalSpanRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	globalSpanRecorderOnce.Do(func() {
		otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(globalSpanRecorder)))
	})
	return globalSpanRecorder
}

// spansSince returns the spans recorded by sr after the mark returned from
// an earlier len(sr.Ended()) call.
func spansSince(sr *tracetest.SpanRecorder, mark int) []sdktrace.ReadOnlySpan {
	ended := sr.Ended()
	if mark > len(ended) {
		return nil
	}
	return ended[mark:]
}

// spanAttrs collects a recorded span's attribute set into a plain map keyed
// by attribute name, values rendered via String() (Value.AsString() only
// works for STRING-kind values and silently returns "" for every other
// kind, which would make every int/float attribute below compare as empty).
func spanAttrs(s sdktrace.ReadOnlySpan) map[string]string {
	m := make(map[string]string, len(s.Attributes()))
	for _, kv := range s.Attributes() {
		m[string(kv.Key)] = kv.Value.String()
	}
	return m
}

// TestJevDecideEmitsSpan proves D-13: every Decide call's "decide" span
// carries the shared status word, failure spans stay fully attributed, the
// usage attributes are omitted (not zeroed) when the response has none, and
// the attribute key set never exceeds D-13's eight names.
func TestJevDecideEmitsSpan(t *testing.T) {
	sr := ensureGlobalSpanRecorder(t)

	t.Run("success", func(t *testing.T) {
		mark := len(sr.Ended())
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fixtureNoulOK))
		}))
		defer srv.Close()

		c := New(srv.URL, "k", "m")
		noRetryDelay(c)
		resp, err := c.Decide(context.Background(), oneNoulRequest())
		if err != nil {
			t.Fatalf("Decide: %v", err)
		}
		if resp.Usage == nil {
			t.Fatal("resp.Usage is nil, want fixtureNoulOK's usage")
		}

		spans := spansSince(sr, mark)
		if len(spans) != 1 || spans[0].Name() != "decide" {
			t.Fatalf("want exactly one decide span, got %v", spans)
		}
		attrs := spanAttrs(spans[0])
		want := map[string]string{
			"engram.decide.provider":       "jev",
			"engram.decide.model":          "m",
			"engram.decide.model_snapshot": "typesafe/jev-1.13-20260917",
			"engram.decide.questions":      "1",
			"engram.decide.input_tokens":   "497",
			"engram.decide.output_tokens":  "67",
			"engram.decide.status":         "ok",
		}
		for k, v := range want {
			if attrs[k] != v {
				t.Errorf("attrs[%q] = %q, want %q", k, attrs[k], v)
			}
		}
		if len(attrs) != 8 {
			t.Errorf("attribute count = %d, want 8: %v", len(attrs), attrs)
		}
		gotCostStr, ok := attrs["engram.decide.cost_usd"]
		if !ok {
			t.Fatal("engram.decide.cost_usd missing")
		}
		gotCost, perr := strconv.ParseFloat(gotCostStr, 64)
		if perr != nil {
			t.Fatalf("cost_usd attribute %q did not parse as float64: %v", gotCostStr, perr)
		}
		const wantCost = 0.000020874
		if gotCost < wantCost-1e-12 || gotCost > wantCost+1e-12 {
			t.Errorf("cost_usd attribute = %v, want within 1e-12 of %v", gotCost, wantCost)
		}
		if code := spans[0].Status().Code; code != codes.Unset && code != codes.Ok {
			t.Errorf("span status code = %v, want Unset or Ok", code)
		}
	})

	t.Run("http failure", func(t *testing.T) {
		mark := len(sr.Ended())
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(fixtureOpenRouter401))
		}))
		defer srv.Close()

		c := New(srv.URL, "k", "m")
		noRetryDelay(c)
		_, err := c.Decide(context.Background(), oneNoulRequest())
		if !errors.Is(err, decide.ErrDecisionAuth) {
			t.Fatalf("err = %v, want ErrDecisionAuth", err)
		}

		spans := spansSince(sr, mark)
		if len(spans) != 1 {
			t.Fatalf("want exactly one decide span, got %v", spans)
		}
		attrs := spanAttrs(spans[0])
		want := map[string]string{
			"engram.decide.provider":  "jev",
			"engram.decide.model":     "m",
			"engram.decide.questions": "1",
			"engram.decide.status":    "auth",
		}
		for k, v := range want {
			if attrs[k] != v {
				t.Errorf("attrs[%q] = %q, want %q", k, attrs[k], v)
			}
		}
		if len(attrs) != 4 {
			t.Errorf("attribute count = %d, want 4: %v", len(attrs), attrs)
		}
		if spans[0].Status().Code != codes.Error {
			t.Errorf("span status code = %v, want Error", spans[0].Status().Code)
		}
		events := spans[0].Events()
		if len(events) != 1 {
			t.Fatalf("want exactly one exception event, got %d: %v", len(events), events)
		}
	})

	t.Run("validation failure", func(t *testing.T) {
		mark := len(sr.Ended())
		var requests int
		srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			requests++
		}))
		defer srv.Close()

		c := New(srv.URL, "k", "m")
		noRetryDelay(c)
		_, err := c.Decide(context.Background(), tooManyChoicesRequest())
		if !errors.Is(err, decide.ErrTooManyChoices) {
			t.Fatalf("err = %v, want decide.ErrTooManyChoices", err)
		}

		spans := spansSince(sr, mark)
		if len(spans) != 1 {
			t.Fatalf("want exactly one decide span, got %v", spans)
		}
		attrs := spanAttrs(spans[0])
		if attrs["engram.decide.status"] != "invalid_request" {
			t.Errorf("status attribute = %q, want invalid_request", attrs["engram.decide.status"])
		}
		if attrs["engram.decide.questions"] != "1" {
			t.Errorf("questions attribute = %q, want 1", attrs["engram.decide.questions"])
		}
		if requests != 0 {
			t.Errorf("requests = %d, want 0 (validation runs before any network I/O)", requests)
		}
	})

	t.Run("no usage", func(t *testing.T) {
		mark := len(sr.Ended())
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fixtureNoulNoUsage))
		}))
		defer srv.Close()

		c := New(srv.URL, "k", "m")
		noRetryDelay(c)
		resp, err := c.Decide(context.Background(), oneNoulRequest())
		if err != nil {
			t.Fatalf("Decide: %v", err)
		}
		if resp.Usage != nil {
			t.Errorf("resp.Usage = %+v, want nil: decide.Response.Usage's doc comment says nil when the provider omits it", resp.Usage)
		}

		spans := spansSince(sr, mark)
		if len(spans) != 1 {
			t.Fatalf("want exactly one decide span, got %v", spans)
		}
		attrs := spanAttrs(spans[0])
		if attrs["engram.decide.status"] != "ok" {
			t.Errorf("status attribute = %q, want ok", attrs["engram.decide.status"])
		}
		if _, ok := attrs["engram.decide.model_snapshot"]; !ok {
			t.Error("engram.decide.model_snapshot missing, want present")
		}
		for _, k := range []string{"engram.decide.input_tokens", "engram.decide.output_tokens", "engram.decide.cost_usd"} {
			if _, ok := attrs[k]; ok {
				t.Errorf("%s present, want omitted when the response has no usage", k)
			}
		}
	})
}

// jevSentinelState, jevSentinelInstructions, jevSentinelCriterion and
// jevSentinelKey are the four sentinel values TestJevTelemetryCarriesNoContent
// drives through Decide, proving prohibition P2: no decision state, question
// instructions, criteria text or API key ever reaches engram-authored
// telemetry or error text.
const (
	jevSentinelState        = "SENTINEL-STATE-7f3a"
	jevSentinelInstructions = "SENTINEL-INSTR-7f3a"
	jevSentinelCriterion    = "SENTINEL-CRIT-7f3a"
	jevSentinelKey          = "SENTINEL-KEY-7f3a"
)

// TestJevTelemetryCarriesNoContent proves prohibition P2 by driving one
// success call and one failure call with sentinel values through State,
// Instructions, WhenTrue and the API key, then scanning every span
// attribute, every span event attribute, the debug slog output and both
// calls' err.Error() for the sentinels.
func TestJevTelemetryCarriesNoContent(t *testing.T) {
	sr := ensureGlobalSpanRecorder(t)
	mark := len(sr.Ended())

	var logBuf bytes.Buffer
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prevLogger) })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+jevSentinelKey {
			t.Errorf("Authorization header = %q, want Bearer %s", r.Header.Get("Authorization"), jevSentinelKey)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureNoulOK))
	}))
	defer srv.Close()

	c := New(srv.URL, jevSentinelKey, "m")
	noRetryDelay(c)

	sentinelReq := decide.Request{
		State: decide.State{"k": jevSentinelState},
		Questions: map[string]decide.Question{
			"q": decide.Noul(jevSentinelInstructions, jevSentinelCriterion, "false"),
		},
	}

	_, err := c.Decide(context.Background(), sentinelReq)
	if err != nil {
		t.Fatalf("success Decide: %v", err)
	}

	// The 400 fixture does not echo the request: its body is a fixed
	// spike-captured fixture, unrelated to sentinelReq's content, so any
	// sentinel found in this call's telemetry or error text can only have
	// come from engram's own code building it from the request.
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+jevSentinelKey {
			t.Errorf("Authorization header = %q, want Bearer %s", r.Header.Get("Authorization"), jevSentinelKey)
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(fixtureOpenRouter400Choices))
	}))
	defer failSrv.Close()

	cFail := New(failSrv.URL, jevSentinelKey, "m")
	noRetryDelay(cFail)
	_, failErr := cFail.Decide(context.Background(), sentinelReq)
	if !errors.Is(failErr, decide.ErrDecisionBadRequest) {
		t.Fatalf("failure Decide err = %v, want ErrDecisionBadRequest", failErr)
	}

	spans := spansSince(sr, mark)
	if len(spans) != 2 {
		t.Fatalf("want exactly two decide spans (success + failure), got %d: %v", len(spans), spans)
	}

	sentinels := []string{jevSentinelState, jevSentinelInstructions, jevSentinelCriterion, jevSentinelKey}

	for _, span := range spans {
		for _, kv := range span.Attributes() {
			assertNoSentinel(t, "span attribute "+string(kv.Key), kv.Value.String(), sentinels)
		}
		for _, ev := range span.Events() {
			assertNoSentinel(t, "span event name", ev.Name, sentinels)
			for _, kv := range ev.Attributes {
				assertNoSentinel(t, "span event attribute "+string(kv.Key), kv.Value.String(), sentinels)
			}
		}
	}

	assertNoSentinel(t, "slog output", logBuf.String(), sentinels)
	assertNoSentinel(t, "success err.Error()", errString(err), sentinels)
	assertNoSentinel(t, "failure err.Error()", errString(failErr), sentinels)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func assertNoSentinel(t *testing.T, where, haystack string, sentinels []string) {
	t.Helper()
	for _, s := range sentinels {
		if strings.Contains(haystack, s) {
			t.Errorf("%s contains sentinel %q: %q", where, s, haystack)
		}
	}
}
