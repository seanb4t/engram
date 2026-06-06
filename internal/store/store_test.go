// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	addr := os.Getenv("MEM_QDRANT_TEST_ADDR") // host:port (gRPC 6334)
	if addr == "" {
		t.Skip("set MEM_QDRANT_TEST_ADDR to run store integration tests")
	}
	host, portStr, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portStr)
	c, err := qdrant.NewClient(&qdrant.Config{Host: host, Port: port})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	s := New(c, "mem_eval_test")
	if err := s.EnsureCollection(context.Background(), 3); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	return s
}

func TestUpsertGetDeleteRoundtrip(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	m := Memory{
		ID:        "11111111-1111-1111-1111-111111111111",
		Content:   "uses jj for VCS",
		Scope:     "eval-test:project:selfhosted-cluster",
		Repo:      "selfhosted-cluster",
		Source:    "user-said",
		Category:  "preference",
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := s.Get(ctx, m.ID)
	if err != nil || got.Content != m.Content {
		t.Fatalf("get: %v / %+v", err, got)
	}
	if got.Scope != m.Scope {
		t.Errorf("scope mismatch: got %q want %q", got.Scope, m.Scope)
	}
	if err := s.Delete(ctx, m.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, m.ID); err == nil {
		t.Fatalf("expected not-found after delete")
	}
}

func TestDiscoveryRoundtrip(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	m := Memory{
		ID:       "44444444-4444-4444-4444-444444444444",
		Content:  "retry logic lives in client.go, keyed off ctx deadline",
		Scope:    "discovery:repo:github.com/seanb4t/engram",
		Repo:     "github.com/seanb4t/engram",
		Source:   "agent-inferred",
		Category: "discovery",
		Kind:     "fact",
		Summary:  "retry = ctx deadline",
		Citations: []Citation{
			{Kind: "file", Ref: "internal/client.go", Locator: "200-240", Pin: "sha256:abc", Excerpt: "for ... { select { <-ctx.Done() } }"},
			{Kind: "repo", Ref: "github.com/qdrant/go-client", Pin: "@v1.18.2"},
		},
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := s.Get(ctx, m.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Kind != "fact" || got.Summary != "retry = ctx deadline" {
		t.Errorf("kind/summary mismatch: %+v", got)
	}
	if len(got.Citations) != 2 {
		t.Fatalf("citations: got %d want 2: %+v", len(got.Citations), got.Citations)
	}
	c0 := got.Citations[0]
	if c0.Kind != "file" || c0.Ref != "internal/client.go" || c0.Locator != "200-240" || c0.Pin != "sha256:abc" || c0.Excerpt == "" {
		t.Errorf("citation[0] mismatch: %+v", c0)
	}
	if got.Citations[1].Ref != "github.com/qdrant/go-client" || got.Citations[1].Pin != "@v1.18.2" {
		t.Errorf("citation[1] mismatch: %+v", got.Citations[1])
	}
	_ = s.Delete(ctx, m.ID)
}

func TestSearchDiscoveryFilters(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scopeA := "discovery:repo:A"
	scopeB := "discovery:repo:B"
	curated := "repo:A"
	defer func() { _ = s.DeleteAll(ctx, scopeA); _ = s.DeleteAll(ctx, scopeB); _ = s.DeleteAll(ctx, curated) }()

	mk := func(id, scope, cat, kind string, vec []float32) {
		m := Memory{ID: id, Content: "x", Scope: scope, Category: cat, Kind: kind,
			Source: "agent-inferred", CreatedAt: time.Now().UTC()}
		if cat == "discovery" {
			m.Citations = []Citation{{Kind: "file", Ref: "f.go"}}
		}
		if err := s.Upsert(ctx, m, vec); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	mk("55555555-5555-5555-5555-555555555555", scopeA, "discovery", "map", []float32{0.9, 0.1, 0.0})
	mk("66666666-6666-6666-6666-666666666666", scopeA, "discovery", "fact", []float32{0.8, 0.2, 0.0})
	mk("77777777-7777-7777-7777-777777777777", scopeB, "discovery", "map", []float32{0.7, 0.3, 0.0})
	mk("88888888-8888-8888-8888-888888888888", curated, "preference", "", []float32{0.9, 0.1, 0.0})

	q := []float32{0.9, 0.1, 0.0}

	// scope-constrained: only scopeA discoveries, not curated, not scopeB.
	hits, err := s.SearchDiscovery(ctx, scopeA, "", q, 10)
	if err != nil {
		t.Fatalf("search scopeA: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("scopeA: got %d want 2", len(hits))
	}
	for _, h := range hits {
		if h.Category != "discovery" || h.Scope != scopeA {
			t.Errorf("leaked non-discovery/other-scope: %+v", h)
		}
	}

	// kind filter.
	maps, err := s.SearchDiscovery(ctx, scopeA, "map", q, 10)
	if err != nil {
		t.Fatalf("search kind=map: %v", err)
	}
	if len(maps) != 1 || maps[0].Kind != "map" {
		t.Fatalf("kind=map: got %d %+v", len(maps), maps)
	}

	// cross-spine: empty scope spans scopeA + scopeB discoveries (3), still no curated.
	all, err := s.SearchDiscovery(ctx, "", "", q, 10)
	if err != nil {
		t.Fatalf("search cross-spine: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("cross-spine: got %d want 3", len(all))
	}
}

func TestFromPayloadSkipsNonStructCitation(t *testing.T) {
	// A malformed citations list (a stray non-struct item) must not yield an
	// empty Citation{}; the struct items still read back. Pure unit test — no Qdrant.
	p := qdrant.NewValueMap(map[string]any{
		"citations": []any{
			"not-a-struct",
			map[string]any{"kind": "file", "ref": "f.go", "locator": "1-2"},
		},
	})
	m := fromPayload("id-1", p)
	if len(m.Citations) != 1 {
		t.Fatalf("expected 1 citation (non-struct skipped), got %d: %+v", len(m.Citations), m.Citations)
	}
	if m.Citations[0].Ref != "f.go" || m.Citations[0].Kind != "file" {
		t.Errorf("citation mismatch: %+v", m.Citations[0])
	}
}

func TestSearchAndDeleteAll(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "eval-test:project:search-test"

	m1 := Memory{
		ID: "22222222-2222-2222-2222-222222222222", Content: "prefers Go over Python",
		Scope: scope, Source: "user-said", Category: "preference", CreatedAt: time.Now().UTC(),
	}
	m2 := Memory{
		ID: "33333333-3333-3333-3333-333333333333", Content: "uses conventional commits",
		Scope: scope, Source: "agent-inferred", Category: "convention", CreatedAt: time.Now().UTC(),
	}

	if err := s.Upsert(ctx, m1, []float32{0.9, 0.1, 0.0}); err != nil {
		t.Fatalf("upsert m1: %v", err)
	}
	if err := s.Upsert(ctx, m2, []float32{0.0, 0.1, 0.9}); err != nil {
		t.Fatalf("upsert m2: %v", err)
	}

	hits, err := s.Search(ctx, scope, []float32{0.9, 0.1, 0.0}, 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least 1 search hit")
	}

	if err := s.DeleteAll(ctx, scope); err != nil {
		t.Fatalf("delete_all: %v", err)
	}

	hits2, err := s.Search(ctx, scope, []float32{0.9, 0.1, 0.0}, 5)
	if err != nil {
		t.Fatalf("search after delete_all: %v", err)
	}
	if len(hits2) != 0 {
		t.Fatalf("expected 0 hits after delete_all, got %d", len(hits2))
	}
}
