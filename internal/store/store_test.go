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

// qdrantImageTag is the Qdrant image the integration suite boots via
// testcontainers. The SetPayload-on-deleted-point fail-closed contract that
// TestSetVisibilityTOCTOU depends on was verified against this image; bumping it
// MUST be paired with re-verifying that contract and updating
// qdrantTOCTOUVerifiedVersion below (the version guard there will fail loudly
// until you do).
const qdrantImageTag = "qdrant/qdrant:v1.18.2"

// qdrantTOCTOUVerifiedVersion is the server version (as reported by HealthCheck)
// whose SetPayload point-ID NotFound semantics TestSetVisibilityTOCTOU was
// written against. Deliberately a SEPARATE constant from qdrantImageTag so a
// version bump trips the guard in TestSetVisibilityTOCTOU and forces a conscious
// re-verification rather than silently tracking the new image.
const qdrantTOCTOUVerifiedVersion = "1.18.2"

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
	container, err := tcqdrant.Run(startCtx, qdrantImageTag)
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

// cleanupErr surfaces a deferred-cleanup failure so leftover records can't
// silently contaminate later tests in the run. ErrNotFound is tolerated: the
// record is already gone, which is exactly what cleanup wanted.
func cleanupErr(t *testing.T, what string, err error) {
	t.Helper()
	if err != nil && !errors.Is(err, ErrNotFound) {
		t.Errorf("cleanup %s: %v", what, err)
	}
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
	if err := s.Delete(ctx, m.ID, Anonymous()); err != nil {
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
	cleanupErr(t, "Delete "+m.ID, s.Delete(ctx, m.ID, Anonymous()))
}

func TestSearchDiscoveryFilters(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scopeA := "discovery:repo:A"
	scopeB := "discovery:repo:B"
	curated := "repo:A"
	defer func() {
		cleanupErr(t, "DeleteAll "+scopeA, s.DeleteAll(ctx, scopeA, Anonymous()))
		cleanupErr(t, "DeleteAll "+scopeB, s.DeleteAll(ctx, scopeB, Anonymous()))
		cleanupErr(t, "DeleteAll "+curated, s.DeleteAll(ctx, curated, Anonymous()))
	}()

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
	hits, err := s.SearchDiscovery(ctx, scopeA, "", Anonymous(), q, 10)
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
	maps, err := s.SearchDiscovery(ctx, scopeA, "map", Anonymous(), q, 10)
	if err != nil {
		t.Fatalf("search kind=map: %v", err)
	}
	if len(maps) != 1 || maps[0].Kind != "map" {
		t.Fatalf("kind=map: got %d %+v", len(maps), maps)
	}

	// cross-spine: empty scope spans scopeA + scopeB discoveries (3), still no curated.
	all, err := s.SearchDiscovery(ctx, "", "", Anonymous(), q, 10)
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
	defer func() {
		cleanupErr(t, "DeleteAllRaw "+scopeA, s.DeleteAllRaw(ctx, scopeA))
		cleanupErr(t, "DeleteAllRaw "+scopeB, s.DeleteAllRaw(ctx, scopeB))
	}()

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
	hits, err := s.SearchDiscovery(ctx, scopeA, "", Authenticated("sub-A"), q, 10)
	if err != nil {
		t.Fatalf("scoped: %v", err)
	}
	if len(hits) != 1 || hits[0].Owner != "sub-A" {
		t.Fatalf("scoped: got %d %+v want 1 A-owned", len(hits), hits)
	}
	// cross_spine (scope=""): A sees A across both scopes (2) + B's shared (1) = 3, never B's private.
	all, err := s.SearchDiscovery(ctx, "", "", Authenticated("sub-A"), q, 10)
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
	// A foreign-owned record survives the anon-scoped DeleteAll below, so clean
	// the whole scope raw regardless of owner.
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	m1 := Memory{
		ID: "22222222-2222-2222-2222-222222222222", Content: "prefers Go over Python",
		Scope: scope, Source: "user-said", Category: "preference", CreatedAt: time.Now().UTC(),
	}
	m2 := Memory{
		ID: "33333333-3333-3333-3333-333333333333", Content: "uses conventional commits",
		Scope: scope, Source: "agent-inferred", Category: "convention", CreatedAt: time.Now().UTC(),
	}
	// A record owned by a different sub, sharing m1's vector so it WOULD rank in
	// the query below if the owner filter regressed to matching everything. Both
	// the anon search (must exclude it) and the anon DeleteAll (must not delete
	// it) then exercise the owner-match branch — without this record an absent
	// owner filter would pass the test silently.
	mForeign := Memory{
		ID: "44444444-4444-4444-4444-444444444444", Content: "foreign-owned secret",
		Scope: scope, Owner: "sub-foreign", Source: "user-said", Category: "preference",
		CreatedAt: time.Now().UTC(),
	}

	if err := s.Upsert(ctx, m1, []float32{0.9, 0.1, 0.0}); err != nil {
		t.Fatalf("upsert m1: %v", err)
	}
	if err := s.Upsert(ctx, m2, []float32{0.0, 0.1, 0.9}); err != nil {
		t.Fatalf("upsert m2: %v", err)
	}
	if err := s.Upsert(ctx, mForeign, []float32{0.9, 0.1, 0.0}); err != nil {
		t.Fatalf("upsert mForeign: %v", err)
	}

	hits, err := s.Search(ctx, scope, Anonymous(), []float32{0.9, 0.1, 0.0}, 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least 1 search hit")
	}
	// The owner-match branch must fire: an anonymous caller sees only the
	// anonymous bucket (owner==""), never the foreign-owned record.
	for _, h := range hits {
		if h.Owner != "" {
			t.Errorf("anonymous search returned a non-anon record: id=%s owner=%q", h.ID, h.Owner)
		}
	}

	if err := s.DeleteAll(ctx, scope, Anonymous()); err != nil {
		t.Fatalf("delete_all: %v", err)
	}

	hits2, err := s.Search(ctx, scope, Anonymous(), []float32{0.9, 0.1, 0.0}, 5)
	if err != nil {
		t.Fatalf("search after delete_all: %v", err)
	}
	if len(hits2) != 0 {
		t.Fatalf("expected 0 hits after delete_all, got %d", len(hits2))
	}
	// DeleteAll is owner-scoped: the foreign-owned record must survive an
	// anonymous DeleteAll.
	survivors, err := s.Search(ctx, scope, Authenticated("sub-foreign"), []float32{0.9, 0.1, 0.0}, 5)
	if err != nil {
		t.Fatalf("search as foreign owner after delete_all: %v", err)
	}
	if len(survivors) != 1 {
		t.Fatalf("foreign-owned record must survive anon delete_all: got %d want 1", len(survivors))
	}
}

func TestSearchListOwnerIsolation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:search"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

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
	hits, err := s.Search(ctx, scope, Authenticated("sub-A"), []float32{0.1, 0.2, 0.3}, 10)
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
	lst, err := s.List(ctx, scope, Authenticated("sub-A"), 10)
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
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	priv := Memory{ID: "dddddddd-0000-0000-0000-000000000001", Content: "p", Scope: scope, Owner: "sub-B", CreatedAt: time.Now().UTC()}
	shar := Memory{ID: "dddddddd-0000-0000-0000-000000000002", Content: "s", Scope: scope, Owner: "sub-B", Visibility: "shared", CreatedAt: time.Now().UTC()}
	for _, m := range []Memory{priv, shar} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}
	// Owner reads own private record.
	if _, err := s.GetReadable(ctx, priv.ID, Authenticated("sub-B")); err != nil {
		t.Errorf("owner denied own record: %v", err)
	}
	// Non-owner reads a shared record.
	if _, err := s.GetReadable(ctx, shar.ID, Authenticated("sub-A")); err != nil {
		t.Errorf("shared record denied to other actor: %v", err)
	}
	// Non-owner denied a private record — and it looks like not-found.
	_, err := s.GetReadable(ctx, priv.ID, Authenticated("sub-A"))
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-actor private read: want ErrNotFound, got %v", err)
	}
}

func TestDeleteOwnerGate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:del"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	// Even a SHARED record is not deletable by a non-owner.
	m := Memory{ID: "eeeeeeee-0000-0000-0000-000000000001", Content: "s", Scope: scope, Owner: "sub-B", Visibility: "shared", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := s.Delete(ctx, m.ID, Authenticated("sub-A")); !errors.Is(err, ErrNotFound) {
		t.Errorf("non-owner delete: want ErrNotFound, got %v", err)
	}
	if _, err := s.Get(ctx, m.ID); err != nil {
		t.Errorf("record should survive non-owner delete: %v", err)
	}
	if err := s.Delete(ctx, m.ID, Authenticated("sub-B")); err != nil {
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
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	m := Memory{ID: "ffffffff-0000-0000-0000-000000000001", Content: "v1", Scope: scope, Owner: "sub-B", Visibility: "shared", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	vec := []float32{0.4, 0.5, 0.6}
	// Non-owner cannot update even a shared record.
	if _, err := s.FetchForUpdate(ctx, m.ID, Authenticated("sub-A")); !errors.Is(err, ErrNotFound) {
		t.Errorf("non-owner update: want ErrNotFound, got %v", err)
	}
	// Owner content-only update (shared == nil) PRESERVES visibility.
	cur, err := s.FetchForUpdate(ctx, m.ID, Authenticated("sub-B"))
	if err != nil {
		t.Fatalf("FetchForUpdate owner: %v", err)
	}
	if err := s.Update(ctx, cur, "v2", nil, vec); err != nil {
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
	cur, err = s.FetchForUpdate(ctx, m.ID, Authenticated("sub-B"))
	if err != nil {
		t.Fatalf("FetchForUpdate before unshare: %v", err)
	}
	if err := s.Update(ctx, cur, "v3", &no, vec); err != nil {
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
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	m := Memory{ID: "a1a1a1a1-0000-0000-0000-000000000001", Content: "v", Scope: scope, Owner: "sub-B", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// Non-owner denied.
	if err := s.SetVisibility(ctx, m.ID, Authenticated("sub-A"), true); !errors.Is(err, ErrNotFound) {
		t.Errorf("non-owner set_visibility: want ErrNotFound, got %v", err)
	}
	// Owner shares, then unshares; vector is preserved (SetPayload, not Upsert).
	if err := s.SetVisibility(ctx, m.ID, Authenticated("sub-B"), true); err != nil {
		t.Fatalf("share: %v", err)
	}
	got, err := s.Get(ctx, m.ID)
	if err != nil {
		t.Fatalf("get after share: %v", err)
	}
	if got.Visibility != "shared" {
		t.Errorf("share failed: %q", got.Visibility)
	}
	if err := s.SetVisibility(ctx, m.ID, Authenticated("sub-B"), false); err != nil {
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

// TestSetVisibilityTOCTOU verifies the TOCTOU behaviour of SetVisibility: a
// record deleted between the ownership gate (getWritable) and the SetPayload
// call must not cause SetVisibility to return nil.
//
// Qdrant v1.18.2 with a point-ID selector returns a NotFound gRPC error from
// SetPayload when the target ID does not exist — so the error propagates
// through SetVisibility without a separate re-fetch. Parts 1 and 2 are the
// load-bearing TOCTOU assertions: they confirm at the raw Qdrant level that
// SetPayload errors on a missing point-ID (the fail-closed contract we rely
// on), including when the record vanishes after the gate has passed. Part 3
// covers the simpler pre-entry case — the record is already gone when
// SetVisibility is called, so the getWritable gate rejects it. A version guard
// at the top of this function enforces the image-version coupling: bumping
// qdrantImageTag without updating qdrantTOCTOUVerifiedVersion will fail loudly
// here, requiring conscious re-verification of Parts 1–2 before proceeding.
func TestSetVisibilityTOCTOU(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Version guard: the SetPayload-on-deleted-point fail-closed contract asserted
	// in Parts 1-2 was verified against Qdrant qdrantTOCTOUVerifiedVersion. When the
	// suite boots the pinned testcontainer (no MEM_QDRANT_TEST_ADDR override),
	// enforce that the running server matches — so bumping qdrantImageTag without
	// re-verifying Parts 1-2 and updating qdrantTOCTOUVerifiedVersion fails loudly
	// here. An operator-supplied external instance owns its own version, so the
	// check is skipped on that path to avoid a spurious failure.
	if os.Getenv("MEM_QDRANT_TEST_ADDR") == "" {
		hc, err := s.client.HealthCheck(ctx)
		if err != nil {
			t.Fatalf("qdrant health check: %v", err)
		}
		if v := hc.GetVersion(); v != qdrantTOCTOUVerifiedVersion {
			t.Fatalf("Qdrant version %q != verified %q: re-verify SetPayload point-ID NotFound semantics (Parts 1-2), then update qdrantTOCTOUVerifiedVersion and qdrantImageTag together", v, qdrantTOCTOUVerifiedVersion)
		}
	}

	scope := "iso-test:project:toctou"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	// Part 1: verify Qdrant's SetPayload (point-ID selector) errors on a
	// missing ID — this is the contract that makes SetVisibility fail-closed.
	missingID := "f0f0f0f0-0000-0000-0000-000000000001"
	_, rawErr := s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload:        qdrant.NewValueMap(map[string]any{"visibility": "shared"}),
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(missingID)}),
	})
	if rawErr == nil {
		t.Fatal("qdrant SetPayload on missing point-ID returned nil — the fail-closed contract for SetVisibility is broken; review Qdrant behaviour for this version and update SetVisibility accordingly")
	}

	// Part 2: simulate the TOCTOU window at the raw level.
	// Insert → verify gate passes → delete (concurrent race) → SetPayload.
	// SetPayload must error because the ID no longer exists.
	id := "f0f0f0f0-0000-0000-0000-000000000002"
	m := Memory{
		ID: id, Content: "toctou-target", Scope: scope,
		Owner: "sub-owner", CreatedAt: time.Now().UTC(),
	}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := s.getWritable(ctx, id, Authenticated("sub-owner")); err != nil {
		t.Fatalf("getWritable pre-delete: %v", err)
	}
	// Concurrent delete: simulates what happens in the TOCTOU window.
	if _, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelector(qdrant.NewID(id)),
	}); err != nil {
		t.Fatalf("concurrent delete: %v", err)
	}
	_, setPayloadErr := s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload:        qdrant.NewValueMap(map[string]any{"visibility": "shared"}),
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
	})
	if setPayloadErr == nil {
		t.Error("TOCTOU: SetPayload on deleted point-ID returned nil — the fail-closed contract for SetVisibility is broken; review Qdrant behaviour for this version and update SetVisibility accordingly")
	}

	// Part 3: end-to-end via the SetVisibility public API.
	// Insert → delete → SetVisibility must error. The record is absent at gate
	// entry, so this surfaces ErrNotFound from getWritable — the pre-entry
	// deletion case, not the TOCTOU window (which Part 2 covers).
	id2 := "f0f0f0f0-0000-0000-0000-000000000003"
	m2 := Memory{
		ID: id2, Content: "sv-target", Scope: scope,
		Owner: "sub-owner", CreatedAt: time.Now().UTC(),
	}
	if err := s.Upsert(ctx, m2, []float32{0.4, 0.5, 0.6}); err != nil {
		t.Fatalf("upsert m2: %v", err)
	}
	if err := s.Delete(ctx, id2, Authenticated("sub-owner")); err != nil {
		t.Fatalf("delete m2: %v", err)
	}
	// SetVisibility on a missing record must not return nil.
	if err := s.SetVisibility(ctx, id2, Authenticated("sub-owner"), true); err == nil {
		t.Error("SetVisibility on deleted record returned nil — expected an error")
	}
}

func TestOwnedOrAbsent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "discovery:repo:owned"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	id := "b2b2b2b2-0000-0000-0000-000000000001"
	// Absent id → ok (caller will create).
	if err := s.OwnedOrAbsent(ctx, id, Authenticated("sub-A")); err != nil {
		t.Errorf("absent id should be ok: %v", err)
	}
	m := Memory{ID: id, Content: "d", Scope: scope, Category: "discovery", Kind: "fact", Owner: "sub-A", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// Owner re-supplies id → ok (replace).
	if err := s.OwnedOrAbsent(ctx, id, Authenticated("sub-A")); err != nil {
		t.Errorf("owner replace should be ok: %v", err)
	}
	// Other actor supplies id → ErrNotFound (refuse overwrite).
	if err := s.OwnedOrAbsent(ctx, id, Authenticated("sub-B")); !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-owner overwrite: want ErrNotFound, got %v", err)
	}
}

// TestFetchForUpdate mirrors TestOwnedOrAbsent for the FetchForUpdate gate:
// owner → record returned, non-owner → ErrNotFound, absent → ErrNotFound (record
// must exist to update), anonymous bucket (sub=="" on ownerless record) → record
// returned.
func TestFetchForUpdate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:fetch-for-update"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	id := "b2b2b2b2-0000-0000-0000-000000000002"

	// Absent record → ErrNotFound (nothing to update).
	if _, err := s.FetchForUpdate(ctx, id, Authenticated("sub-A")); !errors.Is(err, ErrNotFound) {
		t.Errorf("absent record: want ErrNotFound, got %v", err)
	}

	m := Memory{ID: id, Content: "d", Scope: scope, Category: "convention", Owner: "sub-A", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Owner → record returned (may proceed to embed + Update).
	got, err := s.FetchForUpdate(ctx, id, Authenticated("sub-A"))
	if err != nil {
		t.Errorf("owner: unexpected error: %v", err)
	}
	if got.ID != id {
		t.Errorf("owner: want ID %q, got %q", id, got.ID)
	}

	// Non-owner → ErrNotFound (early-exit before embed).
	if _, err := s.FetchForUpdate(ctx, id, Authenticated("sub-B")); !errors.Is(err, ErrNotFound) {
		t.Errorf("non-owner: want ErrNotFound, got %v", err)
	}

	// Anonymous bucket: ownerless record (owner=="") with Anonymous() → record returned.
	ownerlessID := "b2b2b2b2-0000-0000-0000-000000000003"
	ownerless := Memory{ID: ownerlessID, Content: "x", Scope: scope, Category: "convention", Owner: "", CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, ownerless, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert ownerless: %v", err)
	}
	if _, err := s.FetchForUpdate(ctx, ownerlessID, Anonymous()); err != nil {
		t.Errorf("anonymous bucket ownerless: unexpected error: %v", err)
	}
	// Stamped record (owner!="") with Anonymous() → ErrNotFound (fail-closed write isolation).
	if _, err := s.FetchForUpdate(ctx, id, Anonymous()); !errors.Is(err, ErrNotFound) {
		t.Errorf("anon on stamped record: want ErrNotFound, got %v", err)
	}
}

func TestDeleteAllOwnerScoped(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:delall"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	a := Memory{ID: "c3c3c3c3-0000-0000-0000-000000000001", Content: "a", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
	b := Memory{ID: "c3c3c3c3-0000-0000-0000-000000000002", Content: "b", Scope: scope, Owner: "sub-B", CreatedAt: time.Now().UTC()}
	for _, m := range []Memory{a, b} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}
	// A's teardown removes only A's record; B's survives.
	if err := s.DeleteAll(ctx, scope, Authenticated("sub-A")); err != nil {
		t.Fatalf("deleteAll: %v", err)
	}
	if _, err := s.Get(ctx, a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("A's record should be gone: %v", err)
	}
	if _, err := s.Get(ctx, b.ID); err != nil {
		t.Errorf("B's record should survive A's teardown: %v", err)
	}
}

func TestMigrateSetOwner(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:migrate"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	// Refuses empty owner.
	if _, err := s.MigrateSetOwner(ctx, ""); err == nil {
		t.Error("empty owner: expected error")
	}

	// A record written WITHOUT the owner key (simulating a pre-isolation record):
	// build the payload, strip "owner", and upsert raw.
	id := "d4d4d4d4-0000-0000-0000-000000000001"
	p := payload(Memory{ID: id, Content: "legacy", Scope: scope, CreatedAt: time.Now().UTC()})
	delete(p, "owner")
	if _, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: []*qdrant.PointStruct{{
			Id: qdrant.NewID(id), Vectors: qdrant.NewVectors(0.1, 0.2, 0.3),
			Payload: qdrant.NewValueMap(p),
		}},
	}); err != nil {
		t.Fatalf("raw upsert: %v", err)
	}

	// An auth-disabled / anonymous-bucket record: the owner key is PRESENT but
	// empty (""). ownerlessFilter uses qdrant.NewIsEmpty("owner"), which matches a
	// MISSING key, not an empty-string value — so the backfill must leave this
	// record untouched. Without that distinction, enabling auth and running
	// migrate-set-owner would silently hijack every anonymous record into the
	// operator's sub.
	anonID := "d4d4d4d4-0000-0000-0000-000000000002"
	if err := s.Upsert(ctx, Memory{
		ID: anonID, Content: "anon-bucket", Scope: scope,
		Owner: "", CreatedAt: time.Now().UTC(),
	}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed anon-bucket record: %v", err)
	}

	n, err := s.MigrateSetOwner(ctx, "sub-OWNER")
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Exactly one record stamped: the missing-key legacy record. The explicit
	// owner=="" record is not matched by NewIsEmpty and must not be counted.
	if n != 1 {
		t.Fatalf("migrate stamped %d records, want exactly 1 (only the missing-key record; the explicit owner=='' bucket must be skipped)", n)
	}
	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after migrate: %v", err)
	}
	if got.Owner != "sub-OWNER" {
		t.Errorf("owner not stamped: %q", got.Owner)
	}
	// The auth-disabled owner=="" record must be untouched by the backfill.
	anon, err := s.Get(ctx, anonID)
	if err != nil {
		t.Fatalf("Get anon-bucket after migrate: %v", err)
	}
	if anon.Owner != "" {
		t.Errorf("auth-disabled record hijacked: owner=%q, want \"\" (NewIsEmpty must not match empty-string owner)", anon.Owner)
	}
	// Idempotent: a second run stamps nothing (the record now has an owner).
	n2, err := s.MigrateSetOwner(ctx, "sub-OWNER")
	if err != nil {
		t.Fatalf("migrate rerun: %v", err)
	}
	if n2 != 0 {
		t.Errorf("idempotency: rerun stamped %d, want 0", n2)
	}
}

// TestMigrateSetOwnerHonorsCancel verifies the backfill propagates context
// cancellation to its Qdrant calls instead of running to completion — the
// property the CLI relies on for its --timeout / Ctrl-C bound (engram-027), so a
// hung Qdrant cannot block forever.
func TestMigrateSetOwnerHonorsCancel(t *testing.T) {
	s := testStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call: the first Count must observe it and bail.
	if _, err := s.MigrateSetOwner(ctx, "sub-x"); err == nil {
		t.Error("MigrateSetOwner with a cancelled context: expected error, got nil")
	}
}

// TestAnonBucketReadIsolation verifies Q1 (engram-99z.13): anonymous callers
// (sub=="") may only read the anonymous bucket (owner=="") — they cannot see
// another owner's shared record via Search, List, or GetReadable.
//
// Scope: this exercises the anonymous bucket (explicit owner=="", as written by
// auth-disabled deployments). It does NOT cover pre-isolation records (those
// with a MISSING owner key): NewMatch("owner","") does not match missing keys,
// so such records are intentionally invisible to every read until backfilled
// (see ownerlessFilter / MigrateSetOwner and the README "Upgrading" note). That
// invisibility is unchanged by this tightening.
func TestAnonBucketReadIsolation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "anon-test:project:read-iso"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	// anonymous-bucket record (explicit owner==""): what an auth-disabled
	// deployment writes; readable by anonymous callers.
	ownerless := Memory{
		ID: "f1f1f1f1-0000-0000-0000-000000000001", Content: "anon-ownerless",
		Scope: scope, Owner: "", Visibility: "", CreatedAt: time.Now().UTC(),
	}
	// authenticated owner's shared record — must NOT be visible to anon callers.
	authShared := Memory{
		ID: "f1f1f1f1-0000-0000-0000-000000000002", Content: "auth-shared",
		Scope: scope, Owner: "sub-owner", Visibility: "shared", CreatedAt: time.Now().UTC(),
	}
	// authenticated owner's private record — must NOT be visible to anon callers.
	authPrivate := Memory{
		ID: "f1f1f1f1-0000-0000-0000-000000000003", Content: "auth-private",
		Scope: scope, Owner: "sub-owner", Visibility: "", CreatedAt: time.Now().UTC(),
	}
	for _, m := range []Memory{ownerless, authShared, authPrivate} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", m.ID, err)
		}
	}

	// Search: anonymous caller sees only the anonymous-bucket record (owner=="").
	hits, err := s.Search(ctx, scope, Anonymous(), []float32{0.1, 0.2, 0.3}, 10)
	if err != nil {
		t.Fatalf("Search anon: %v", err)
	}
	if len(hits) != 1 {
		t.Errorf("Search anon: got %d want 1 (ownerless only)", len(hits))
	} else if hits[0].ID != ownerless.ID {
		t.Errorf("Search anon: got record %q, want ownerless %q", hits[0].ID, ownerless.ID)
	}
	for _, h := range hits {
		if h.Owner != "" {
			t.Errorf("Search anon leaked owner-stamped record: %s owner=%q", h.ID, h.Owner)
		}
	}

	// List: same restriction.
	lst, err := s.List(ctx, scope, Anonymous(), 10)
	if err != nil {
		t.Fatalf("List anon: %v", err)
	}
	if len(lst) != 1 {
		t.Errorf("List anon: got %d want 1 (ownerless only)", len(lst))
	}
	for _, h := range lst {
		if h.Owner != "" {
			t.Errorf("List anon leaked owner-stamped record: %s owner=%q", h.ID, h.Owner)
		}
	}

	// GetReadable: anon caller reads the anonymous-bucket record (owner=="").
	got, err := s.GetReadable(ctx, ownerless.ID, Anonymous())
	if err != nil {
		t.Errorf("GetReadable anon on ownerless record: unexpected error: %v", err)
	}
	if got.ID != ownerless.ID {
		t.Errorf("GetReadable anon ownerless: got %q want %q", got.ID, ownerless.ID)
	}

	// GetReadable: anon caller denied another owner's shared record → ErrNotFound.
	_, err = s.GetReadable(ctx, authShared.ID, Anonymous())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetReadable anon on shared record: want ErrNotFound, got %v", err)
	}

	// GetReadable: anon caller denied another owner's private record → ErrNotFound.
	_, err = s.GetReadable(ctx, authPrivate.ID, Anonymous())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetReadable anon on private record: want ErrNotFound, got %v", err)
	}

	// Authenticated caller still reads shared records (no regression).
	got, err = s.GetReadable(ctx, authShared.ID, Authenticated("sub-other"))
	if err != nil {
		t.Errorf("GetReadable authenticated on shared record: unexpected error: %v", err)
	}
	if got.ID != authShared.ID {
		t.Errorf("GetReadable authenticated shared: got %q want %q", got.ID, authShared.ID)
	}

	// Authenticated Search: sub-owner sees own private + own shared (2), sub-other sees own (0) + shared (1).
	ownerHits, err := s.Search(ctx, scope, Authenticated("sub-owner"), []float32{0.1, 0.2, 0.3}, 10)
	if err != nil {
		t.Fatalf("Search sub-owner: %v", err)
	}
	if len(ownerHits) != 2 {
		t.Errorf("Search sub-owner: got %d want 2 (private+shared)", len(ownerHits))
	}
	otherHits, err := s.Search(ctx, scope, Authenticated("sub-other"), []float32{0.1, 0.2, 0.3}, 10)
	if err != nil {
		t.Fatalf("Search sub-other: %v", err)
	}
	if len(otherHits) != 1 {
		t.Errorf("Search sub-other: got %d want 1 (shared only)", len(otherHits))
	}
}

// TestAnonBucketWriteSemantics verifies Q2 (engram-99z.13): the anonymous
// bucket (explicit owner=="") is mutually writable when sub=="" — the
// auth-disabled deployment case. Owner-stamped records are NOT mutable by an
// anonymous caller (fail-closed write isolation).
func TestAnonBucketWriteSemantics(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "anon-test:project:write-sem"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	// Ownerless record — mutually writable by anonymous callers.
	ownerless := Memory{
		ID: "f1f1f1f1-0000-0000-0000-000000000011", Content: "v1",
		Scope: scope, Owner: "", Visibility: "", CreatedAt: time.Now().UTC(),
	}
	// Owner-stamped record — must NOT be writable by anonymous callers.
	stamped := Memory{
		ID: "f1f1f1f1-0000-0000-0000-000000000012", Content: "s1",
		Scope: scope, Owner: "sub-owner", Visibility: "", CreatedAt: time.Now().UTC(),
	}
	for _, m := range []Memory{ownerless, stamped} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", m.ID, err)
		}
	}

	// getWritable on ownerless record with Anonymous() → success (anon bucket mutually writable).
	if _, err := s.getWritable(ctx, ownerless.ID, Anonymous()); err != nil {
		t.Errorf("getWritable anon on ownerless record: unexpected error: %v", err)
	}

	// getWritable on owner-stamped record with Anonymous() → ErrNotFound (fail-closed write isolation).
	_, err := s.getWritable(ctx, stamped.ID, Anonymous())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("getWritable anon on owner-stamped record: want ErrNotFound, got %v", err)
	}

	// Update on ownerless record with Anonymous() → success (anon bucket).
	cur, err := s.FetchForUpdate(ctx, ownerless.ID, Anonymous())
	if err != nil {
		t.Fatalf("FetchForUpdate anon on ownerless record: unexpected error: %v", err)
	}
	if err := s.Update(ctx, cur, "v2", nil, []float32{0.2, 0.3, 0.4}); err != nil {
		t.Errorf("Update anon on ownerless record: unexpected error: %v", err)
	}
	got, err := s.Get(ctx, ownerless.ID)
	if err != nil {
		t.Fatalf("Get after anon Update: %v", err)
	}
	if got.Content != "v2" {
		t.Errorf("anon Update: content not applied, got %q", got.Content)
	}

	// Delete on owner-stamped record with Anonymous() → ErrNotFound (fail-closed).
	if err := s.Delete(ctx, stamped.ID, Anonymous()); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete anon on owner-stamped record: want ErrNotFound, got %v", err)
	}
	// Record must still exist.
	if _, err := s.Get(ctx, stamped.ID); err != nil {
		t.Errorf("owner-stamped record should survive anon Delete: %v", err)
	}

	// Delete on ownerless record with Anonymous() → success.
	if err := s.Delete(ctx, ownerless.ID, Anonymous()); err != nil {
		t.Errorf("Delete anon on ownerless record: unexpected error: %v", err)
	}
	if _, err := s.Get(ctx, ownerless.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("ownerless record should be gone after anon Delete")
	}
}

// TestAnonBucketDiscoveryReadIsolation verifies that SearchDiscovery respects
// the same fail-closed anonymous read semantics: anon callers see only
// anonymous-bucket discovery records (owner==""), not authenticated owners'
// shared discoveries.
func TestAnonBucketDiscoveryReadIsolation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "discovery:repo:anon-test"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	mk := func(id, owner, vis string) {
		m := Memory{
			ID: id, Content: "d", Scope: scope, Category: "discovery", Kind: "fact",
			Owner: owner, Visibility: vis,
			Citations: []Citation{{Kind: "file", Ref: "f.go"}},
			CreatedAt: time.Now().UTC(),
		}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	mk("f1f1f1f1-0000-0000-0000-000000000021", "", "")            // ownerless
	mk("f1f1f1f1-0000-0000-0000-000000000022", "sub-A", "shared") // A's shared

	q := []float32{0.1, 0.2, 0.3}

	// Anonymous caller sees only the ownerless discovery.
	hits, err := s.SearchDiscovery(ctx, scope, "", Anonymous(), q, 10)
	if err != nil {
		t.Fatalf("SearchDiscovery anon: %v", err)
	}
	if len(hits) != 1 {
		t.Errorf("SearchDiscovery anon: got %d want 1 (ownerless only)", len(hits))
	}
	for _, h := range hits {
		if h.Owner != "" {
			t.Errorf("SearchDiscovery anon leaked owner-stamped discovery: %s owner=%q", h.ID, h.Owner)
		}
	}

	// Authenticated caller sees own + shared.
	authHits, err := s.SearchDiscovery(ctx, scope, "", Authenticated("sub-A"), q, 10)
	if err != nil {
		t.Fatalf("SearchDiscovery sub-A: %v", err)
	}
	if len(authHits) != 1 {
		t.Errorf("SearchDiscovery sub-A: got %d want 1 (own shared only, no ownerless under sub-A)", len(authHits))
	}

	// sub-B sees sub-A's shared discovery (no regression on authenticated shared read).
	bHits, err := s.SearchDiscovery(ctx, scope, "", Authenticated("sub-B"), q, 10)
	if err != nil {
		t.Fatalf("SearchDiscovery sub-B: %v", err)
	}
	if len(bHits) != 1 {
		t.Errorf("SearchDiscovery sub-B: got %d want 1 (A's shared)", len(bHits))
	}
	if bHits[0].Owner != "sub-A" || bHits[0].Visibility != "shared" {
		t.Errorf("SearchDiscovery sub-B: unexpected record %+v", bHits[0])
	}
}

// TestNilSubjectFailsClosed pins the core guarantee of the typed-Subject
// refactor: a nil Subject (what a discarded subjectFromContext error yields)
// denies on every authz path — empty reads, ErrNotFound id-gates, and a rejected
// bulk delete — rather than silently resolving to the anonymous bucket.
func TestNilSubjectFailsClosed(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:nil-subject"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	// Seed an ownerless (anonymous-bucket) record and an owned record.
	anon := Memory{ID: "a0a0a0a0-0000-0000-0000-000000000001", Content: "anon", Scope: scope, Owner: "", CreatedAt: time.Now().UTC()}
	owned := Memory{ID: "a0a0a0a0-0000-0000-0000-000000000002", Content: "owned", Scope: scope, Owner: "sub-A", CreatedAt: time.Now().UTC()}
	for _, m := range []Memory{anon, owned} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}

	var nilSubj Subject // zero value == nil: the discarded-error case

	// Reads return nothing.
	if hits, err := s.Search(ctx, scope, nilSubj, []float32{0.1, 0.2, 0.3}, 10); err != nil || len(hits) != 0 {
		t.Errorf("Search(nil): want 0 hits nil err, got %d hits, %v", len(hits), err)
	}
	if mems, err := s.List(ctx, scope, nilSubj, 20); err != nil || len(mems) != 0 {
		t.Errorf("List(nil): want 0 mems nil err, got %d, %v", len(mems), err)
	}
	if hits, err := s.SearchDiscovery(ctx, scope, "", nilSubj, []float32{0.1, 0.2, 0.3}, 10); err != nil || len(hits) != 0 {
		t.Errorf("SearchDiscovery(nil): want 0 hits nil err, got %d, %v", len(hits), err)
	}

	// Id-gates return ErrNotFound (even for the ownerless record).
	if _, err := s.GetReadable(ctx, anon.ID, nilSubj); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetReadable(nil): want ErrNotFound, got %v", err)
	}
	if _, err := s.FetchForUpdate(ctx, anon.ID, nilSubj); !errors.Is(err, ErrNotFound) {
		t.Errorf("FetchForUpdate(nil): want ErrNotFound, got %v", err)
	}
	if err := s.Delete(ctx, anon.ID, nilSubj); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete(nil): want ErrNotFound, got %v", err)
	}
	if err := s.SetVisibility(ctx, anon.ID, nilSubj, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("SetVisibility(nil): want ErrNotFound, got %v", err)
	}
	if err := s.OwnedOrAbsent(ctx, anon.ID, nilSubj); !errors.Is(err, ErrNotFound) {
		t.Errorf("OwnedOrAbsent(nil) on existing id: want ErrNotFound, got %v", err)
	}
	// ABSENT id short-circuits before the subject switch (Get→ErrNotFound):
	// nothing to overwrite, so OwnedOrAbsent returns nil even for a nil subject —
	// the create-new arm, distinct from the existing-id default-deny above.
	if err := s.OwnedOrAbsent(ctx, "00000000-0000-0000-0000-0000000000ab", nilSubj); err != nil {
		t.Errorf("OwnedOrAbsent(nil) on absent id: want nil (create-new, nothing to overwrite), got %v", err)
	}

	// Bulk delete is rejected and removes nothing.
	if err := s.DeleteAll(ctx, scope, nilSubj); !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteAll(nil): want ErrNotFound, got %v", err)
	}
	if _, err := s.Get(ctx, anon.ID); err != nil {
		t.Errorf("DeleteAll(nil) must not delete: record gone, %v", err)
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

func TestListScopes(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	a, b := "ls-test:project:a", "ls-test:project:b"
	defer func() {
		cleanupErr(t, "DeleteAllRaw "+a, s.DeleteAllRaw(ctx, a))
		cleanupErr(t, "DeleteAllRaw "+b, s.DeleteAllRaw(ctx, b))
	}()
	mk := func(id, scope, owner string) {
		m := Memory{ID: id, Content: "x", Scope: scope, Owner: owner, CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	mk("c1111111-0000-0000-0000-000000000001", a, "sub-A")
	mk("c1111111-0000-0000-0000-000000000002", a, "sub-A")
	mk("c1111111-0000-0000-0000-000000000003", b, "sub-A")
	mk("c1111111-0000-0000-0000-000000000004", a, "sub-B") // foreign: excluded for sub-A

	scopes, approx, err := s.ListScopes(ctx, Authenticated("sub-A"))
	if err != nil {
		t.Fatalf("ListScopes: %v", err)
	}
	if approx {
		t.Errorf("approximate=true for a tiny set, want false")
	}
	counts := map[string]uint64{}
	for _, sc := range scopes {
		counts[sc.Scope] = sc.Count
	}
	if counts[a] != 2 || counts[b] != 1 {
		t.Errorf("counts = %v, want {%s:2, %s:1} (sub-B's record excluded)", counts, a, b)
	}
}

func TestCountAnonymousBucket(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	before, err := s.CountAnonymousBucket(ctx)
	if err != nil {
		t.Fatalf("CountAnonymousBucket(before): %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	anon1 := Memory{ID: "a1111111-0000-0000-0000-000000000001", Content: "anon one", Scope: "anon-count-scope", Owner: "", Source: "agent-inferred", Category: "fact", CreatedAt: now}
	anon2 := Memory{ID: "a1111111-0000-0000-0000-000000000002", Content: "anon two", Scope: "anon-count-scope", Owner: "", Source: "agent-inferred", Category: "fact", CreatedAt: now}
	owned := Memory{ID: "a1111111-0000-0000-0000-000000000003", Content: "owned", Scope: "anon-count-scope", Owner: "sub-x", Source: "agent-inferred", Category: "fact", CreatedAt: now}
	for _, m := range []Memory{anon1, anon2, owned} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}
	defer func() {
		_ = s.Delete(ctx, anon1.ID, Anonymous())
		_ = s.Delete(ctx, anon2.ID, Anonymous())
		_ = s.Delete(ctx, owned.ID, Authenticated("sub-x"))
	}()

	after, err := s.CountAnonymousBucket(ctx)
	if err != nil {
		t.Fatalf("CountAnonymousBucket(after): %v", err)
	}
	if after-before != 2 {
		t.Fatalf("anonymous-bucket delta = %d, want 2", after-before)
	}
}
