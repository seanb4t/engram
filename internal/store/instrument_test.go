// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// withSpanRecorder installs a SpanRecorder-backed global TracerProvider for the
// duration of the test and returns it. otel.Tracer delegates to the global
// provider at call time, so the package-level tracer picks this up.
func withSpanRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr)))
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return sr
}

func spanByName(spans []sdktrace.ReadOnlySpan, name string) sdktrace.ReadOnlySpan {
	for _, s := range spans {
		if s.Name() == name {
			return s
		}
	}
	return nil
}

func TestStoreSearchEmitsSpan(t *testing.T) {
	st := testStore(t) // skips if testQdrantAddr == "" (see store_test.go helpers)
	sr := withSpanRecorder(t)

	// testStore ensures a 3-dim collection (store_test.go: EnsureCollection(ctx, 3)).
	_, err := st.Search(context.Background(), "repo:spans", anonymous{}, make([]float32, 3), 5, nil)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	sp := spanByName(sr.Ended(), "store.Search")
	if sp == nil {
		t.Fatal("no store.Search span recorded")
	}
	attrs := map[string]string{}
	for _, kv := range sp.Attributes() {
		attrs[string(kv.Key)] = kv.Value.String()
	}
	if attrs["engram.scope"] != "repo:spans" {
		t.Errorf("engram.scope = %q, want repo:spans", attrs["engram.scope"])
	}
	if _, ok := attrs["engram.result_count"]; !ok {
		t.Error("missing engram.result_count attribute")
	}
	if sp.Status().Code == codes.Error {
		t.Errorf("unexpected error status: %s", sp.Status().Description)
	}
}
