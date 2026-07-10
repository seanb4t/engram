// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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
// test after the first recording into a dead provider; a single shared recorder
// installed once sidesteps that. The recorder ACCUMULATES every span for the rest
// of the package run and span names repeat (many tests call PruneExpired/Search),
// so a test MUST scope its lookup to the spans it produced: snapshot
// len(sr.Ended()) before the call and search only sr.Ended()[before:].
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

// spanAttr returns the value of the named attribute on sp, or the zero Value
// and false if absent.
func spanAttr(sp sdktrace.ReadOnlySpan, key string) (attribute.Value, bool) {
	for _, kv := range sp.Attributes() {
		if string(kv.Key) == key {
			return kv.Value, true
		}
	}
	return attribute.Value{}, false
}

// seedRecallSpanRecords upserts n distinct records into scope under subj's
// owner and returns their ids in insertion order, for D-06 recall-id span
// assertions.
func seedRecallSpanRecords(t *testing.T, s *Store, scope string, subj Subject, n int) []string {
	t.Helper()
	ctx := context.Background()
	ids := make([]string, 0, n)
	for i := range n {
		id := fmt.Sprintf("cafecafe-0000-0000-0000-%012d", i)
		m := Memory{
			ID:        id,
			Content:   fmt.Sprintf("recall span record %d", i),
			Scope:     scope,
			Owner:     ownerOf(subj),
			CreatedAt: time.Now().UTC(),
		}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
		t.Cleanup(func() { cleanupErr(t, id, s.Delete(ctx, id, subj)) })
		ids = append(ids, id)
	}
	return ids
}

func TestStoreSearchEmitsSpan(t *testing.T) {
	st := testStore(t) // skips if testQdrantAddr == "" (see store_test.go helpers)
	sr := withSpanRecorder(t)

	// testStore ensures a 3-dim collection (store_test.go: EnsureCollection(ctx, 3)).
	before := len(sr.Ended()) // scope the lookup to the span this call produces
	_, err := st.Search(context.Background(), "repo:spans", anonymous{}, make([]float32, 3), 5, nil, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	sp := spanByName(sr.Ended()[before:], "store.Search")
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
	id := "cafe0001-0000-0000-0000-000000000001" // unique to this test's shared-collection seed
	m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-span", CreatedAt: now, NotAfter: &expired}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	t.Cleanup(func() { cleanupErr(t, id, s.Delete(ctx, id, subj)) })

	sr := withSpanRecorder(t)
	before := len(sr.Ended()) // scope the lookup to the span this call produces
	n, err := s.PruneExpired(ctx, now)
	if err != nil {
		t.Fatalf("PruneExpired: %v", err)
	}
	if n != 1 {
		t.Fatalf("PruneExpired deleted %d, want 1", n)
	}
	sp := spanByName(sr.Ended()[before:], "store.PruneExpired")
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

// TestStoreSearchEmitsRecallIDs pins the D-06 analytics leg: store.Search's
// success span carries engram.recall.ids (the returned record ids) alongside
// engram.recall.count, riding the existing OTLP span — never touching
// access_count or ranking.
func TestStoreSearchEmitsRecallIDs(t *testing.T) {
	s := testStore(t)
	scope := "repo:recall-search-span"
	subj := Authenticated("sub-recall-search")
	seeded := seedRecallSpanRecords(t, s, scope, subj, 3)

	sr := withSpanRecorder(t)
	before := len(sr.Ended())
	out, err := s.Search(context.Background(), scope, subj, []float32{0.1, 0.2, 0.3}, 10, nil, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(out) != len(seeded) {
		t.Fatalf("Search returned %d results, want %d", len(out), len(seeded))
	}
	sp := spanByName(sr.Ended()[before:], "store.Search")
	if sp == nil {
		t.Fatal("no store.Search span recorded")
	}
	assertRecallAttrs(t, sp, len(seeded), len(seeded))
}

// TestStoreListEmitsRecallIDs pins the same D-06 contract on store.List.
func TestStoreListEmitsRecallIDs(t *testing.T) {
	s := testStore(t)
	scope := "repo:recall-list-span"
	subj := Authenticated("sub-recall-list")
	seeded := seedRecallSpanRecords(t, s, scope, subj, 3)

	sr := withSpanRecorder(t)
	before := len(sr.Ended())
	items, _, _, err := s.List(context.Background(), scope, subj, ListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != len(seeded) {
		t.Fatalf("List returned %d items, want %d", len(items), len(seeded))
	}
	sp := spanByName(sr.Ended()[before:], "store.List")
	if sp == nil {
		t.Fatal("no store.List span recorded")
	}
	assertRecallAttrs(t, sp, len(seeded), len(seeded))
}

// TestStoreGetEmitsRecallIDs pins store.Get's data-completeness addition: a
// single-record fetch still carries engram.recall.ids=[id] and
// engram.recall.count=1, even though the id is already known to the caller.
func TestStoreGetEmitsRecallIDs(t *testing.T) {
	s := testStore(t)
	scope := "repo:recall-get-span"
	subj := Authenticated("sub-recall-get")
	seeded := seedRecallSpanRecords(t, s, scope, subj, 1)
	id := seeded[0]

	sr := withSpanRecorder(t)
	before := len(sr.Ended())
	if _, err := s.Get(context.Background(), id); err != nil {
		t.Fatalf("Get: %v", err)
	}
	sp := spanByName(sr.Ended()[before:], "store.Get")
	if sp == nil {
		t.Fatal("no store.Get span recorded")
	}
	ids, ok := spanAttr(sp, "engram.recall.ids")
	if !ok {
		t.Fatal("missing engram.recall.ids attribute")
	}
	if got := ids.AsStringSlice(); len(got) != 1 || got[0] != id {
		t.Errorf("engram.recall.ids = %v, want [%s]", got, id)
	}
	count, ok := spanAttr(sp, "engram.recall.count")
	if !ok {
		t.Fatal("missing engram.recall.count attribute")
	}
	if count.AsInt64() != 1 {
		t.Errorf("engram.recall.count = %d, want 1", count.AsInt64())
	}
}

// TestStoreListRecallIDsCappedAtLimit pins the T-12-11 mitigation: a result
// set larger than recallIDCap yields a capped engram.recall.ids slice while
// engram.recall.count still reflects the true (larger) total.
func TestStoreListRecallIDsCappedAtLimit(t *testing.T) {
	s := testStore(t)
	scope := "repo:recall-list-cap-span"
	subj := Authenticated("sub-recall-list-cap")
	total := recallIDCap + 5
	seeded := seedRecallSpanRecords(t, s, scope, subj, total)

	sr := withSpanRecorder(t)
	before := len(sr.Ended())
	items, _, _, err := s.List(context.Background(), scope, subj, ListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != len(seeded) {
		t.Fatalf("List returned %d items, want %d", len(items), len(seeded))
	}
	sp := spanByName(sr.Ended()[before:], "store.List")
	if sp == nil {
		t.Fatal("no store.List span recorded")
	}
	assertRecallAttrs(t, sp, recallIDCap, total)
}

// assertRecallAttrs asserts sp carries engram.recall.ids of length wantIDLen
// and engram.recall.count == wantCount.
func assertRecallAttrs(t *testing.T, sp sdktrace.ReadOnlySpan, wantIDLen, wantCount int) {
	t.Helper()
	ids, ok := spanAttr(sp, "engram.recall.ids")
	if !ok {
		t.Fatal("missing engram.recall.ids attribute")
	}
	if got := len(ids.AsStringSlice()); got != wantIDLen {
		t.Errorf("len(engram.recall.ids) = %d, want %d", got, wantIDLen)
	}
	count, ok := spanAttr(sp, "engram.recall.count")
	if !ok {
		t.Fatal("missing engram.recall.count attribute")
	}
	if int(count.AsInt64()) != wantCount {
		t.Errorf("engram.recall.count = %d, want %d", count.AsInt64(), wantCount)
	}
}
