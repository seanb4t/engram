// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

var (
	spanRecorderOnce sync.Once
	spanRecorder     *tracetest.SpanRecorder
)

// withSpanRecorder installs a process-wide SpanRecorder-backed TracerProvider on
// first use and returns it. OTel's global delegate upgrades the package tracer to
// a real provider only ONCE, so a per-test swap+restore would leave every span
// test after the first recording into a dead provider. A single shared recorder
// installed once sidesteps that; tests filter sr.Ended() by span name, so span
// accumulation across tests is benign.
func withSpanRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	spanRecorderOnce.Do(func() {
		spanRecorder = tracetest.NewSpanRecorder()
		otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder)))
	})
	return spanRecorder
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

// TestStorePruneExpiredEmitsResultCount pins observability parity with Search and
// ListScheduled (hr2.11): the store.PruneExpired success span must carry the
// deleted tally under engram.result_count, not just engram.before.
func TestStorePruneExpiredEmitsResultCount(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "repo:prune-span"
	subj := Authenticated("sub-span")
	now := time.Now().UTC()
	expired := now.Add(-48 * time.Hour)
	id := "d0000000-0000-0000-0000-000000000001"
	m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-span", CreatedAt: now, NotAfter: &expired}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	t.Cleanup(func() { cleanupErr(t, id, s.Delete(ctx, id, subj)) })

	sr := withSpanRecorder(t)
	n, err := s.PruneExpired(ctx, now)
	if err != nil {
		t.Fatalf("PruneExpired: %v", err)
	}
	if n != 1 {
		t.Fatalf("PruneExpired deleted %d, want 1", n)
	}
	sp := spanByName(sr.Ended(), "store.PruneExpired")
	if sp == nil {
		t.Fatal("no store.PruneExpired span recorded")
	}
	attrs := map[string]string{}
	for _, kv := range sp.Attributes() {
		attrs[string(kv.Key)] = kv.Value.String()
	}
	if got := attrs["engram.result_count"]; got != "1" {
		t.Errorf("engram.result_count = %q, want \"1\"", got)
	}
	if sp.Status().Code == codes.Error {
		t.Errorf("unexpected error status: %s", sp.Status().Description)
	}
}
