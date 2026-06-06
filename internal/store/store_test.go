// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
	tcqdrant "github.com/testcontainers/testcontainers-go/modules/qdrant"
)

// testQdrantAddr is the gRPC host:port the integration tests run against. Set by
// TestMain: MEM_QDRANT_TEST_ADDR if provided (fast path / override), else an
// ephemeral testcontainer. Empty when neither is available (Docker absent), in
// which case the integration tests skip.
var testQdrantAddr string

// TestMain provisions Qdrant for this package's integration tests. It prefers an
// existing instance via MEM_QDRANT_TEST_ADDR; otherwise it boots an ephemeral
// Qdrant via testcontainers and tears it down afterward. If neither is available
// the suite still runs — the integration tests skip with a clear message — so
// unit-only tests are unaffected.
func TestMain(m *testing.M) {
	if addr := os.Getenv("MEM_QDRANT_TEST_ADDR"); addr != "" {
		testQdrantAddr = addr
		os.Exit(m.Run())
	}
	// Bound startup so an unreachable daemon or a stalled image pull fails fast
	// instead of hanging the suite. os.Exit skips defers, so cancel explicitly.
	startCtx, startCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	container, err := tcqdrant.Run(startCtx, "qdrant/qdrant:v1.18.2")
	if err != nil {
		startCancel()
		fmt.Fprintf(os.Stderr, "qdrant testcontainer unavailable (%v); integration tests will skip — set MEM_QDRANT_TEST_ADDR or start Docker\n", err)
		os.Exit(m.Run())
	}
	testQdrantAddr, err = container.GRPCEndpoint(startCtx)
	startCancel()
	if err != nil {
		terminateQdrant(container)
		fmt.Fprintf(os.Stderr, "qdrant grpc endpoint: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	terminateQdrant(container)
	os.Exit(code)
}

// terminateQdrant tears down the container under a bounded context so a slow
// Docker shutdown cannot hang the suite.
func terminateQdrant(c *tcqdrant.QdrantContainer) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = c.Terminate(ctx)
}

func testStore(t *testing.T) *Store {
	t.Helper()
	if testQdrantAddr == "" {
		t.Skip("no Qdrant available: set MEM_QDRANT_TEST_ADDR or start Docker (testcontainers)")
	}
	host, portStr, err := net.SplitHostPort(testQdrantAddr)
	if err != nil {
		t.Fatalf("invalid Qdrant address %q: %v", testQdrantAddr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		t.Fatalf("invalid Qdrant port %q (from %q): %v", portStr, testQdrantAddr, err)
	}
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
	if err := s.Delete(ctx, m.ID, ""); err != nil {
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
	_ = s.Delete(ctx, m.ID, "")
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
	hits, err := s.SearchDiscovery(ctx, scopeA, "", "", q, 10)
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
	maps, err := s.SearchDiscovery(ctx, scopeA, "map", "", q, 10)
	if err != nil {
		t.Fatalf("search kind=map: %v", err)
	}
	if len(maps) != 1 || maps[0].Kind != "map" {
		t.Fatalf("kind=map: got %d %+v", len(maps), maps)
	}

	// cross-spine: empty scope spans scopeA + scopeB discoveries (3), still no curated.
	all, err := s.SearchDiscovery(ctx, "", "", "", q, 10)
	if err != nil {
		t.Fatalf("search cross-spine: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("cross-spine: got %d want 3", len(all))
	}
}

func TestSearchDiscoveryOwnerIsolation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scopeA := "discovery:repo:isoA"
	scopeB := "discovery:repo:isoB"
	defer func() { _ = s.DeleteAllRaw(ctx, scopeA); _ = s.DeleteAllRaw(ctx, scopeB) }()

	mk := func(id, scope, owner, vis string) {
		m := Memory{ID: id, Content: "d", Scope: scope, Category: "discovery", Kind: "fact",
			Owner: owner, Visibility: vis, CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	mk("cccccccc-0000-0000-0000-000000000001", scopeA, "sub-A", "")       // A private, scopeA
	mk("cccccccc-0000-0000-0000-000000000002", scopeA, "sub-B", "")       // B private, scopeA
	mk("cccccccc-0000-0000-0000-000000000003", scopeB, "sub-A", "")       // A private, scopeB
	mk("cccccccc-0000-0000-0000-000000000004", scopeB, "sub-B", "shared") // B shared, scopeB
	q := []float32{0.1, 0.2, 0.3}

	// Scoped: A in scopeA sees only A's record (1), not B's private.
	hits, err := s.SearchDiscovery(ctx, scopeA, "", "sub-A", q, 10)
	if err != nil {
		t.Fatalf("scoped: %v", err)
	}
	if len(hits) != 1 || hits[0].Owner != "sub-A" {
		t.Fatalf("scoped: got %d %+v want 1 A-owned", len(hits), hits)
	}
	// cross_spine (scope=""): A sees A across both scopes (2) + B's shared (1) = 3, never B's private.
	all, err := s.SearchDiscovery(ctx, "", "", "sub-A", q, 10)
	if err != nil {
		t.Fatalf("cross_spine: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("cross_spine: got %d want 3", len(all))
	}
	for _, h := range all {
		if h.Owner == "sub-B" && h.Visibility != "shared" {
			t.Errorf("cross_spine leaked B's private discovery: %s", h.ID)
		}
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

	hits, err := s.Search(ctx, scope, "", []float32{0.9, 0.1, 0.0}, 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least 1 search hit")
	}

	if err := s.DeleteAll(ctx, scope); err != nil {
		t.Fatalf("delete_all: %v", err)
	}

	hits2, err := s.Search(ctx, scope, "", []float32{0.9, 0.1, 0.0}, 5)
	if err != nil {
		t.Fatalf("search after delete_all: %v", err)
	}
	if len(hits2) != 0 {
		t.Fatalf("expected 0 hits after delete_all, got %d", len(hits2))
	}
}

func TestSearchListOwnerIsolation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:search"
	defer func() { _ = s.DeleteAllRaw(ctx, scope) }()

	mk := func(id, owner, vis string) {
		m := Memory{ID: id, Content: "x", Scope: scope, Owner: owner, Visibility: vis,
			CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	mk("bbbbbbbb-0000-0000-0000-000000000001", "sub-A", "")       // A private
	mk("bbbbbbbb-0000-0000-0000-000000000002", "sub-B", "")       // B private
	mk("bbbbbbbb-0000-0000-0000-000000000003", "sub-B", "shared") // B shared

	// A sees only A-private + B-shared (2), never B-private.
	hits, err := s.Search(ctx, scope, "sub-A", []float32{0.1, 0.2, 0.3}, 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("Search: got %d want 2", len(hits))
	}
	for _, h := range hits {
		if h.Owner == "sub-B" && h.Visibility != "shared" {
			t.Errorf("leaked B's private record: %s", h.ID)
		}
	}
	// List honors the same filter.
	lst, err := s.List(ctx, scope, "sub-A", 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(lst) != 2 {
		t.Errorf("List: got %d want 2", len(lst))
	}
}

func TestOwnerVisibilityRoundtrip(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	m := Memory{
		ID:         "aaaaaaaa-0000-0000-0000-000000000001",
		Content:    "owned + shared record",
		Scope:      "iso-test:project:r",
		Owner:      "sub-abc",
		Visibility: "shared",
		CreatedAt:  time.Now().UTC().Truncate(time.Second),
	}
	defer func() {
		_, _ = s.client.Delete(ctx, &qdrant.DeletePoints{
			CollectionName: s.collection, Wait: qdrant.PtrOf(true),
			Points: qdrant.NewPointsSelector(qdrant.NewID(m.ID)),
		})
	}()
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := s.Get(ctx, m.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Owner != "sub-abc" || got.Visibility != "shared" {
		t.Errorf("round-trip lost owner/visibility: owner=%q visibility=%q", got.Owner, got.Visibility)
	}
}

func TestGetReadableOwnerGate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:getr"
	defer func() { _ = s.DeleteAllRaw(ctx, scope) }()
	priv := Memory{ID: "dddddddd-0000-0000-0000-000000000001", Content: "p", Scope: scope, Owner: "sub-B", CreatedAt: time.Now().UTC()}
	shar := Memory{ID: "dddddddd-0000-0000-0000-000000000002", Content: "s", Scope: scope, Owner: "sub-B", Visibility: "shared", CreatedAt: time.Now().UTC()}
	for _, m := range []Memory{priv, shar} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}
	// Owner reads own private record.
	if _, err := s.GetReadable(ctx, priv.ID, "sub-B"); err != nil {
		t.Errorf("owner denied own record: %v", err)
	}
	// Non-owner reads a shared record.
	if _, err := s.GetReadable(ctx, shar.ID, "sub-A"); err != nil {
		t.Errorf("shared record denied to other actor: %v", err)
	}
	// Non-owner denied a private record — and it looks like not-found.
	_, err := s.GetReadable(ctx, priv.ID, "sub-A")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-actor private read: want ErrNotFound, got %v", err)
	}
}

func TestDeleteOwnerGate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:del"
	defer func() { _ = s.DeleteAllRaw(ctx, scope) }()
	// Even a SHARED record is not deletable by a non-owner.
	m := Memory{ID: "eeeeeeee-0000-0000-0000-000000000001", Content: "s", Scope: scope, Owner: "sub-B", Visibility: "shared", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := s.Delete(ctx, m.ID, "sub-A"); !errors.Is(err, ErrNotFound) {
		t.Errorf("non-owner delete: want ErrNotFound, got %v", err)
	}
	if _, err := s.Get(ctx, m.ID); err != nil {
		t.Errorf("record should survive non-owner delete: %v", err)
	}
	if err := s.Delete(ctx, m.ID, "sub-B"); err != nil {
		t.Errorf("owner delete failed: %v", err)
	}
	if _, err := s.Get(ctx, m.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("owner delete did not remove record: %v", err)
	}
}

func TestUpdateOwnerGateAndSharedFlag(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:upd"
	defer func() { _ = s.DeleteAllRaw(ctx, scope) }()
	m := Memory{ID: "ffffffff-0000-0000-0000-000000000001", Content: "v1", Scope: scope, Owner: "sub-B", Visibility: "shared", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	vec := []float32{0.4, 0.5, 0.6}
	// Non-owner cannot update even a shared record.
	if err := s.Update(ctx, m.ID, "sub-A", "hijack", nil, vec); !errors.Is(err, ErrNotFound) {
		t.Errorf("non-owner update: want ErrNotFound, got %v", err)
	}
	// Owner content-only update (shared == nil) PRESERVES visibility.
	if err := s.Update(ctx, m.ID, "sub-B", "v2", nil, vec); err != nil {
		t.Fatalf("owner update: %v", err)
	}
	got, err := s.Get(ctx, m.ID)
	if err != nil {
		t.Fatalf("get after content-only update: %v", err)
	}
	if got.Content != "v2" || got.Visibility != "shared" {
		t.Errorf("content-only update lost sharing: content=%q visibility=%q", got.Content, got.Visibility)
	}
	// Explicit unshare.
	no := false
	if err := s.Update(ctx, m.ID, "sub-B", "v3", &no, vec); err != nil {
		t.Fatalf("unshare update: %v", err)
	}
	got, err = s.Get(ctx, m.ID)
	if err != nil {
		t.Fatalf("get after unshare: %v", err)
	}
	if got.Visibility != "" {
		t.Errorf("unshare failed: visibility=%q", got.Visibility)
	}
}

func TestSetVisibilityOwnerGate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:vis"
	defer func() { _ = s.DeleteAllRaw(ctx, scope) }()
	m := Memory{ID: "a1a1a1a1-0000-0000-0000-000000000001", Content: "v", Scope: scope, Owner: "sub-B", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// Non-owner denied.
	if err := s.SetVisibility(ctx, m.ID, "sub-A", true); !errors.Is(err, ErrNotFound) {
		t.Errorf("non-owner set_visibility: want ErrNotFound, got %v", err)
	}
	// Owner shares, then unshares; vector is preserved (SetPayload, not Upsert).
	if err := s.SetVisibility(ctx, m.ID, "sub-B", true); err != nil {
		t.Fatalf("share: %v", err)
	}
	got, err := s.Get(ctx, m.ID)
	if err != nil {
		t.Fatalf("get after share: %v", err)
	}
	if got.Visibility != "shared" {
		t.Errorf("share failed: %q", got.Visibility)
	}
	if err := s.SetVisibility(ctx, m.ID, "sub-B", false); err != nil {
		t.Fatalf("unshare: %v", err)
	}
	got, err = s.Get(ctx, m.ID)
	if err != nil {
		t.Fatalf("get after unshare: %v", err)
	}
	if got.Visibility != "" {
		t.Errorf("unshare failed: %q", got.Visibility)
	}
}

func TestOwnedOrAbsent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "discovery:repo:owned"
	defer func() { _ = s.DeleteAllRaw(ctx, scope) }()
	id := "b2b2b2b2-0000-0000-0000-000000000001"
	// Absent id → ok (caller will create).
	if err := s.OwnedOrAbsent(ctx, id, "sub-A"); err != nil {
		t.Errorf("absent id should be ok: %v", err)
	}
	m := Memory{ID: id, Content: "d", Scope: scope, Category: "discovery", Kind: "fact", Owner: "sub-A", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// Owner re-supplies id → ok (replace).
	if err := s.OwnedOrAbsent(ctx, id, "sub-A"); err != nil {
		t.Errorf("owner replace should be ok: %v", err)
	}
	// Other actor supplies id → ErrNotFound (refuse overwrite).
	if err := s.OwnedOrAbsent(ctx, id, "sub-B"); !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-owner overwrite: want ErrNotFound, got %v", err)
	}
}

// DeleteAllRaw removes every point in scope regardless of owner — test cleanup only.
func (s *Store) DeleteAllRaw(ctx context.Context, scope string) error {
	_, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelectorFilter(&qdrant.Filter{
			Must: []*qdrant.Condition{qdrant.NewMatch("scope", scope)},
		}),
	})
	return err
}
