// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/shortid"
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
// TestMain: ENGRAM_QDRANT_TEST_ADDR if provided (fast path / override), else an
// ephemeral testcontainer. Empty when neither is available (Docker absent), in
// which case the integration tests skip.
var testQdrantAddr string

// TestMain provisions Qdrant for this package's integration tests. It prefers an
// existing instance via ENGRAM_QDRANT_TEST_ADDR; otherwise it boots an ephemeral
// Qdrant via testcontainers and tears it down afterward. If neither is available
// the suite still runs — the integration tests skip with a clear message — so
// unit-only tests are unaffected.
func TestMain(m *testing.M) {
	if addr := os.Getenv("ENGRAM_QDRANT_TEST_ADDR"); addr != "" {
		testQdrantAddr = addr
		os.Exit(m.Run())
	}
	// Bound startup so an unreachable daemon or a stalled image pull fails fast
	// instead of hanging the suite. os.Exit skips defers, so cancel explicitly.
	startCtx, startCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	container, err := tcqdrant.Run(startCtx, qdrantImageTag)
	if err != nil {
		startCancel()
		fmt.Fprintf(os.Stderr, "qdrant testcontainer unavailable (%v); integration tests will skip — set ENGRAM_QDRANT_TEST_ADDR or start Docker\n", err)
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

// dialTestClient dials the integration-test Qdrant and returns the bare client.
// Skips when no Qdrant is available. It is the single dialing primitive: testStore
// wraps it with a ready collection, and the reindex tests use it directly to drive
// two collections and read points back verbatim.
func dialTestClient(t *testing.T) *qdrant.Client {
	t.Helper()
	if testQdrantAddr == "" {
		t.Skip("no Qdrant available: set ENGRAM_QDRANT_TEST_ADDR or start Docker (testcontainers)")
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
	return c
}

func testStore(t *testing.T) *Store {
	t.Helper()
	s := New(dialTestClient(t), "mem_eval_test")
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

func TestEnsureIndexesCreatesShortIDIndex(t *testing.T) {
	st := testStore(t) // ensureIndexes ran during construction
	info, err := st.client.GetCollectionInfo(context.Background(), st.collection)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := info.GetPayloadSchema()["short_id"]; !ok {
		t.Fatalf("short_id payload index not created; schema keys: %v", info.GetPayloadSchema())
	}
	// Idempotence: a second ensureIndexes is AlreadyExists-tolerant.
	if err := st.ensureIndexes(context.Background(), st.collection); err != nil {
		t.Fatalf("second ensureIndexes: %v", err)
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

	hits, err := s.Search(ctx, scope, Anonymous(), []float32{0.9, 0.1, 0.0}, 5, nil, time.Time{}, time.Time{})
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

	hits2, err := s.Search(ctx, scope, Anonymous(), []float32{0.9, 0.1, 0.0}, 5, nil, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("search after delete_all: %v", err)
	}
	if len(hits2) != 0 {
		t.Fatalf("expected 0 hits after delete_all, got %d", len(hits2))
	}
	// DeleteAll is owner-scoped: the foreign-owned record must survive an
	// anonymous DeleteAll.
	survivors, err := s.Search(ctx, scope, Authenticated("sub-foreign"), []float32{0.9, 0.1, 0.0}, 5, nil, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("search as foreign owner after delete_all: %v", err)
	}
	if len(survivors) != 1 {
		t.Fatalf("foreign-owned record must survive anon delete_all: got %d want 1", len(survivors))
	}
}

// TestSearchAndListTagsFilter pins the tag filter on both recall paths: an
// optional tags filter narrows results to records carrying ALL requested tags
// (AND), an empty filter is a passthrough, and the filter composes with — never
// bypasses — the owner/visibility authz gate.
func TestSearchAndListTagsFilter(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:tags-filter"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	mk := func(id, owner string, tags ...string) {
		m := Memory{ID: id, Content: "x", Scope: scope, Owner: owner, Tags: tags,
			CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	// Anonymous-bucket (owner=="") records the anonymous caller can read.
	mk("cccccccc-0000-0000-0000-000000000001", "", "go", "perf") // both
	mk("cccccccc-0000-0000-0000-000000000002", "", "go")         // go only
	mk("cccccccc-0000-0000-0000-000000000003", "", "python")     // neither
	// Owned by another actor and carrying "go": the tag filter must NOT surface it
	// to the anonymous caller (filter narrows, authz still gates).
	mk("cccccccc-0000-0000-0000-000000000004", "sub-other", "go")

	q := []float32{0.1, 0.2, 0.3}
	ids := func(ms []Memory) []string {
		out := make([]string, len(ms))
		for i, m := range ms {
			out[i] = m.ID
		}
		slices.Sort(out)
		return out
	}

	cases := []struct {
		name string
		tags []string
		want []string
	}{
		{"no filter is passthrough", nil, []string{
			"cccccccc-0000-0000-0000-000000000001",
			"cccccccc-0000-0000-0000-000000000002",
			"cccccccc-0000-0000-0000-000000000003",
		}},
		{"single tag", []string{"go"}, []string{
			"cccccccc-0000-0000-0000-000000000001",
			"cccccccc-0000-0000-0000-000000000002",
		}},
		{"AND of two tags", []string{"go", "perf"}, []string{
			"cccccccc-0000-0000-0000-000000000001",
		}},
		{"non-matching tag", []string{"rust"}, []string{}},
		{"empty-string tag is passthrough", []string{""}, []string{
			"cccccccc-0000-0000-0000-000000000001",
			"cccccccc-0000-0000-0000-000000000002",
			"cccccccc-0000-0000-0000-000000000003",
		}},
		{"empty-string element is dropped", []string{"go", ""}, []string{
			"cccccccc-0000-0000-0000-000000000001",
			"cccccccc-0000-0000-0000-000000000002",
		}},
	}
	for _, tc := range cases {
		hits, err := s.Search(ctx, scope, Anonymous(), q, 10, tc.tags, time.Time{}, time.Time{})
		if err != nil {
			t.Fatalf("Search %s: %v", tc.name, err)
		}
		if got := ids(hits); !slices.Equal(got, tc.want) {
			t.Errorf("Search %s: got %v want %v", tc.name, got, tc.want)
		}
		lst, _, _, err := s.List(ctx, scope, Anonymous(), ListOptions{Limit: 10, Tags: tc.tags})
		if err != nil {
			t.Fatalf("List %s: %v", tc.name, err)
		}
		if got := ids(lst); !slices.Equal(got, tc.want) {
			t.Errorf("List %s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

// TestTagsFilterComposesWithWindow confirms the tag filter and the active-window
// recall gate compose: an expired record is dropped even when its tags match,
// because both are appended to the same Must envelope. The tag filter narrows;
// it never resurrects a windowed-out record.
func TestTagsFilterComposesWithWindow(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:tags-window"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	fixed := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return fixed } // white-box override of the recall clock

	active := fixed.Add(time.Hour)   // not_after in the future → still recalled
	expired := fixed.Add(-time.Hour) // not_after in the past → windowed out
	mk := func(id string, notAfter time.Time) {
		m := Memory{ID: id, Content: "x", Scope: scope, Owner: "", Tags: []string{"go"},
			CreatedAt: fixed, NotAfter: &notAfter}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	activeID := "cdcdcdcd-0000-0000-0000-000000000001"
	expiredID := "cdcdcdcd-0000-0000-0000-000000000002"
	mk(activeID, active)
	mk(expiredID, expired)

	// Tag "go" matches both records, but the window gate must drop the expired one.
	hits, err := s.Search(ctx, scope, Anonymous(), []float32{0.1, 0.2, 0.3}, 10, []string{"go"}, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 || hits[0].ID != activeID {
		t.Errorf("Search tag+window: got %d (%v) want just %s", len(hits), hits, activeID)
	}
	lst, _, _, err := s.List(ctx, scope, Anonymous(), ListOptions{Limit: 10, Tags: []string{"go"}})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(lst) != 1 || lst[0].ID != activeID {
		t.Errorf("List tag+window: got %d (%v) want just %s", len(lst), lst, activeID)
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
	hits, err := s.Search(ctx, scope, Authenticated("sub-A"), []float32{0.1, 0.2, 0.3}, 10, nil, time.Time{}, time.Time{})
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
	lst, _, _, err := s.List(ctx, scope, Authenticated("sub-A"), ListOptions{Limit: 10})
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
	if err := s.Update(ctx, cur, "v2", nil, nil, nil, vec); err != nil {
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
	if err := s.Update(ctx, cur, "v3", &no, nil, nil, vec); err != nil {
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

// TestUpdateTags pins the store-level tag presence-signal at the layer that
// owns it (mirroring TestUpdateOwnerGateAndSharedFlag's coverage of shared):
// nil preserves, non-nil replaces, an empty slice clears — each surviving the
// full Qdrant payload round-trip.
func TestUpdateTags(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:upd-tags"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()
	id := "ffffffff-0000-0000-0000-0000000000a1"
	m := Memory{ID: id, Content: "v1", Scope: scope, Owner: "sub-T", Tags: []string{"old"}, CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	vec := []float32{0.4, 0.5, 0.6}

	// nil tags preserves the existing set.
	cur, err := s.FetchForUpdate(ctx, id, Authenticated("sub-T"))
	if err != nil {
		t.Fatalf("fetch (preserve): %v", err)
	}
	if err := s.Update(ctx, cur, "v2", nil, nil, nil, vec); err != nil {
		t.Fatalf("update nil tags: %v", err)
	}
	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("get (preserve): %v", err)
	}
	if !slices.Equal(got.Tags, []string{"old"}) {
		t.Errorf("nil tags should preserve: got %v", got.Tags)
	}

	// non-nil replaces the full set.
	cur, err = s.FetchForUpdate(ctx, id, Authenticated("sub-T"))
	if err != nil {
		t.Fatalf("fetch (replace): %v", err)
	}
	repl := []string{"x", "y"}
	if err := s.Update(ctx, cur, "v3", nil, &repl, nil, vec); err != nil {
		t.Fatalf("update replace: %v", err)
	}
	got, err = s.Get(ctx, id)
	if err != nil {
		t.Fatalf("get (replace): %v", err)
	}
	if !slices.Equal(got.Tags, repl) {
		t.Errorf("non-nil tags should replace: got %v want %v", got.Tags, repl)
	}

	// empty slice clears.
	cur, err = s.FetchForUpdate(ctx, id, Authenticated("sub-T"))
	if err != nil {
		t.Fatalf("fetch (clear): %v", err)
	}
	empty := []string{}
	if err := s.Update(ctx, cur, "v4", nil, &empty, nil, vec); err != nil {
		t.Fatalf("update clear: %v", err)
	}
	got, err = s.Get(ctx, id)
	if err != nil {
		t.Fatalf("get (clear): %v", err)
	}
	if len(got.Tags) != 0 {
		t.Errorf("empty slice should clear tags: got %v", got.Tags)
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
	// suite boots the pinned testcontainer (no ENGRAM_QDRANT_TEST_ADDR override),
	// enforce that the running server matches — so bumping qdrantImageTag without
	// re-verifying Parts 1-2 and updating qdrantTOCTOUVerifiedVersion fails loudly
	// here. An operator-supplied external instance owns its own version, so the
	// check is skipped on that path to avoid a spurious failure.
	if os.Getenv("ENGRAM_QDRANT_TEST_ADDR") == "" {
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
	hits, err := s.Search(ctx, scope, Anonymous(), []float32{0.1, 0.2, 0.3}, 10, nil, time.Time{}, time.Time{})
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
	lst, _, _, err := s.List(ctx, scope, Anonymous(), ListOptions{Limit: 10})
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
	ownerHits, err := s.Search(ctx, scope, Authenticated("sub-owner"), []float32{0.1, 0.2, 0.3}, 10, nil, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("Search sub-owner: %v", err)
	}
	if len(ownerHits) != 2 {
		t.Errorf("Search sub-owner: got %d want 2 (private+shared)", len(ownerHits))
	}
	otherHits, err := s.Search(ctx, scope, Authenticated("sub-other"), []float32{0.1, 0.2, 0.3}, 10, nil, time.Time{}, time.Time{})
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
	if err := s.Update(ctx, cur, "v2", nil, nil, nil, []float32{0.2, 0.3, 0.4}); err != nil {
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
	if hits, err := s.Search(ctx, scope, nilSubj, []float32{0.1, 0.2, 0.3}, 10, nil, time.Time{}, time.Time{}); err != nil || len(hits) != 0 {
		t.Errorf("Search(nil): want 0 hits nil err, got %d hits, %v", len(hits), err)
	}
	if mems, _, _, err := s.List(ctx, scope, nilSubj, ListOptions{Limit: 20}); err != nil || len(mems) != 0 {
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

func TestListPagination(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "page-test:project:x"
	base := time.Now().UTC().Truncate(time.Second)
	// 5 owned records, descending CreatedAt so order is deterministic.
	for i := 0; i < 5; i++ {
		m := Memory{
			ID:      fmt.Sprintf("d0000000-0000-0000-0000-00000000000%d", i),
			Content: fmt.Sprintf("rec %d", i), Scope: scope, Owner: "owner-A",
			Visibility: "", Category: "convention", Source: "agent-inferred",
			CreatedAt: base.Add(time.Duration(-i) * time.Minute),
		}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	defer func() {
		for i := 0; i < 5; i++ {
			_ = s.Delete(ctx, fmt.Sprintf("d0000000-0000-0000-0000-00000000000%d", i), Authenticated("owner-A"))
		}
	}()
	subj := Authenticated("owner-A")

	// Page 1: limit 2, offset 0 -> 2 records, total 5.
	got, total, _, err := s.List(ctx, scope, subj, ListOptions{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 5 {
		t.Fatalf("total=%d, want 5", total)
	}
	if len(got) != 2 || got[0].Content != "rec 0" {
		t.Fatalf("page1 = %d records, first=%q", len(got), got[0].Content)
	}
	// Page 3: offset 4, limit 2 -> 1 record (the tail).
	got, _, _, _ = s.List(ctx, scope, subj, ListOptions{Limit: 2, Offset: 4})
	if len(got) != 1 {
		t.Fatalf("page3 = %d records, want 1", len(got))
	}
	// Offset past total -> empty page, no panic, real total.
	got, total, _, err = s.List(ctx, scope, subj, ListOptions{Limit: 2, Offset: 99})
	if err != nil || len(got) != 0 || total != 5 {
		t.Fatalf("oob: err=%v len=%d total=%d, want nil/0/5", err, len(got), total)
	}
}

// TestListExactTotalPastOldCap proves the scanCap ceiling is gone: with > 1000
// readable records, List returns an exact total (Count), not a capped 1000.
func TestListExactTotalPastOldCap(t *testing.T) {
	if testing.Short() {
		t.Skip("writes 1001 points; skipped in -short")
	}
	s := testStore(t)
	ctx := context.Background()
	scope := "cap-test:project:x"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	base := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	const n = 1001
	for i := 0; i < n; i++ {
		m := Memory{
			ID:      fmt.Sprintf("c0000000-0000-0000-0000-%012d", i),
			Content: "c", Scope: scope, Owner: "sub-A",
			CreatedAt: base.Add(time.Duration(i) * time.Second),
		}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("Upsert %d: %v", i, err)
		}
	}
	_, total, _, err := s.List(ctx, scope, Authenticated("sub-A"), ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != n {
		t.Errorf("exact total: got %d want %d (scanCap not retired?)", total, n)
	}
}

func TestListCategoryAndVisibilityFilter(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "filter-test:project:x"
	now := time.Now().UTC().Truncate(time.Second)
	// Private records are stored with Visibility=="" (canonical representation).
	seed := []Memory{
		{ID: "e0000000-0000-0000-0000-000000000001", Content: "conv shared", Scope: scope, Owner: "owner-A", Visibility: "shared", Category: "convention", Source: "agent-inferred", CreatedAt: now},
		{ID: "e0000000-0000-0000-0000-000000000002", Content: "gotcha private", Scope: scope, Owner: "owner-A", Visibility: "", Category: "gotcha", Source: "agent-inferred", CreatedAt: now},
		{ID: "e0000000-0000-0000-0000-000000000003", Content: "preference private", Scope: scope, Owner: "owner-A", Visibility: "", Category: "preference", Source: "agent-inferred", CreatedAt: now},
	}
	for _, m := range seed {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}
	defer func() {
		for _, m := range seed {
			_ = s.Delete(ctx, m.ID, Authenticated("owner-A"))
		}
	}()
	subj := Authenticated("owner-A")

	// Single-category filter.
	got, total, _, _ := s.List(ctx, scope, subj, ListOptions{Limit: 10, Categories: []string{"gotcha"}})
	if total != 1 || len(got) != 1 || got[0].Category != "gotcha" {
		t.Fatalf("category filter: total=%d len=%d", total, len(got))
	}

	// Multi-category OR filter (af5.14): both gotcha and preference must match.
	got, total, _, _ = s.List(ctx, scope, subj, ListOptions{Limit: 10, Categories: []string{"gotcha", "preference"}})
	if total != 2 || len(got) != 2 {
		t.Fatalf("multi-category OR filter: total=%d len=%d, want 2", total, len(got))
	}
	for _, r := range got {
		if r.Category != "gotcha" && r.Category != "preference" {
			t.Errorf("multi-category OR returned unexpected category: %q", r.Category)
		}
	}

	// Shared visibility filter: returns only the shared record.
	got, total, _, _ = s.List(ctx, scope, subj, ListOptions{Limit: 10, Visibility: "shared"})
	if total != 1 || len(got) != 1 || got[0].Visibility != "shared" {
		t.Fatalf("visibility=shared filter: total=%d len=%d", total, len(got))
	}

	// Private visibility filter: returns records with stored visibility=="" (2 private records),
	// never the shared one.
	got, total, _, _ = s.List(ctx, scope, subj, ListOptions{Limit: 10, Visibility: "private"})
	if total != 2 || len(got) != 2 {
		t.Fatalf("visibility=private filter: total=%d len=%d, want 2 private records", total, len(got))
	}
	for _, r := range got {
		if r.Visibility == "shared" {
			t.Errorf("visibility=private filter leaked shared record: id=%s", r.ID)
		}
		if r.ID == "e0000000-0000-0000-0000-000000000001" {
			t.Errorf("visibility=private filter returned the shared record")
		}
	}
}

// TestListPrivateFilterCrossActorIsolation verifies that the private visibility
// filter preserves authz isolation: caller B must not see caller A's private
// records even when Visibility=="private" is specified in ListOptions.
func TestListPrivateFilterCrossActorIsolation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-private-filter:project:x"
	now := time.Now().UTC().Truncate(time.Second)
	// A private, B private, B shared.
	aPriv := Memory{ID: "e1000000-0000-0000-0000-000000000001", Content: "A private", Scope: scope, Owner: "owner-A", Visibility: "", Category: "convention", Source: "agent-inferred", CreatedAt: now}
	bPriv := Memory{ID: "e1000000-0000-0000-0000-000000000002", Content: "B private", Scope: scope, Owner: "owner-B", Visibility: "", Category: "convention", Source: "agent-inferred", CreatedAt: now}
	bShared := Memory{ID: "e1000000-0000-0000-0000-000000000003", Content: "B shared", Scope: scope, Owner: "owner-B", Visibility: "shared", Category: "convention", Source: "agent-inferred", CreatedAt: now}
	for _, m := range []Memory{aPriv, bPriv, bShared} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}
	defer func() {
		_ = s.Delete(ctx, aPriv.ID, Authenticated("owner-A"))
		_ = s.Delete(ctx, bPriv.ID, Authenticated("owner-B"))
		_ = s.Delete(ctx, bShared.ID, Authenticated("owner-B"))
	}()

	// Caller A with Visibility=="private": sees only A's private record; never B's private or B's shared.
	got, total, _, err := s.List(ctx, scope, Authenticated("owner-A"), ListOptions{Limit: 10, Visibility: "private"})
	if err != nil {
		t.Fatalf("List owner-A private: %v", err)
	}
	if total != 1 || len(got) != 1 {
		t.Fatalf("owner-A private filter: total=%d len=%d, want 1", total, len(got))
	}
	if got[0].ID != aPriv.ID {
		t.Errorf("owner-A private filter: got record %q, want A's private %q", got[0].ID, aPriv.ID)
	}
	for _, r := range got {
		if r.Owner != "owner-A" {
			t.Errorf("private filter leaked another owner's record: id=%s owner=%q", r.ID, r.Owner)
		}
	}

	// Caller B with Visibility=="private": sees only B's private record; never A's private.
	got, total, _, err = s.List(ctx, scope, Authenticated("owner-B"), ListOptions{Limit: 10, Visibility: "private"})
	if err != nil {
		t.Fatalf("List owner-B private: %v", err)
	}
	if total != 1 || len(got) != 1 {
		t.Fatalf("owner-B private filter: total=%d len=%d, want 1", total, len(got))
	}
	if got[0].ID != bPriv.ID {
		t.Errorf("owner-B private filter: got record %q, want B's private %q", got[0].ID, bPriv.ID)
	}
}

func TestListFilterPreservesIsolation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-filter:project:x"
	now := time.Now().UTC().Truncate(time.Second)
	// owner-A private + owner-B private, both convention.
	a := Memory{ID: "f0000000-0000-0000-0000-000000000001", Content: "A priv", Scope: scope, Owner: "owner-A", Visibility: "", Category: "convention", Source: "agent-inferred", CreatedAt: now}
	b := Memory{ID: "f0000000-0000-0000-0000-000000000002", Content: "B priv", Scope: scope, Owner: "owner-B", Visibility: "", Category: "convention", Source: "agent-inferred", CreatedAt: now}
	for _, m := range []Memory{a, b} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	defer func() {
		_ = s.Delete(ctx, a.ID, Authenticated("owner-A"))
		_ = s.Delete(ctx, b.ID, Authenticated("owner-B"))
	}()
	// Caller B with a category filter must still never see A's private record.
	got, total, _, _ := s.List(ctx, scope, Authenticated("owner-B"), ListOptions{Limit: 10, Categories: []string{"convention"}})
	if total != 1 || len(got) != 1 || got[0].Owner != "owner-B" {
		t.Fatalf("isolation breach: total=%d, %+v", total, got)
	}
}

func TestPayloadRoundTripWindow(t *testing.T) {
	nb := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	na := time.Date(2031, 6, 7, 8, 9, 10, 0, time.UTC)
	m := Memory{
		ID: "22222222-2222-2222-2222-222222222222", Content: "windowed",
		Scope: "win-test:project:x", Owner: "sub-A", CreatedAt: time.Now().UTC(),
		NotBefore: &nb, NotAfter: &na,
	}
	got := fromPayload(m.ID, qdrant.NewValueMap(payload(m)))
	if got.NotBefore == nil || !got.NotBefore.Equal(nb) {
		t.Errorf("NotBefore round-trip: got %v want %v", got.NotBefore, nb)
	}
	if got.NotAfter == nil || !got.NotAfter.Equal(na) {
		t.Errorf("NotAfter round-trip: got %v want %v", got.NotAfter, na)
	}
	// Unwindowed record: keys absent, pointers stay nil.
	plain := fromPayload("id", qdrant.NewValueMap(payload(Memory{ID: "id", Content: "x"})))
	if plain.NotBefore != nil || plain.NotAfter != nil {
		t.Errorf("unwindowed: want nil pointers, got nb=%v na=%v", plain.NotBefore, plain.NotAfter)
	}
}

func TestWithClockOverridesNow(t *testing.T) {
	fixed := time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC)
	s := New(nil, "c", WithClock(func() time.Time { return fixed }))
	if got := s.now(); !got.Equal(fixed) {
		t.Errorf("WithClock: got %v want %v", got, fixed)
	}
	// Default clock is time.Now (non-zero, recent).
	d := New(nil, "c")
	if d.now().IsZero() {
		t.Error("default clock returned zero time")
	}
}

func TestRecallWindowGate(t *testing.T) {
	s := testStore(t)
	fixed := time.Date(2030, 6, 15, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return fixed } // white-box override
	ctx := context.Background()
	scope := "gate-test:project:x"
	subj := Authenticated("sub-A")
	past := fixed.Add(-24 * time.Hour)
	future := fixed.Add(24 * time.Hour)

	mk := func(id string, nb, na *time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A",
			CreatedAt: fixed, NotBefore: nb, NotAfter: na}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
		t.Cleanup(func() { cleanupErr(t, id, s.Delete(ctx, id, subj)) })
	}
	mkVis := func(id, owner, vis string, nb, na *time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: owner, Visibility: vis,
			CreatedAt: fixed, NotBefore: nb, NotAfter: na}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
		t.Cleanup(func() { cleanupErr(t, id, s.Delete(ctx, id, Authenticated(owner))) })
	}
	mk("a0000000-0000-0000-0000-000000000001", nil, nil)       // unwindowed -> visible
	mk("a0000000-0000-0000-0000-000000000002", &past, &future) // active -> visible
	mk("a0000000-0000-0000-0000-000000000003", &future, nil)   // scheduled -> hidden
	mk("a0000000-0000-0000-0000-000000000004", nil, &past)     // expired -> hidden
	// sub-B's SHARED but scheduled record: must stay hidden from sub-A until active
	mkVis("a0000000-0000-0000-0000-000000000005", "sub-B", "shared", &future, nil)

	// Active set is exactly the unwindowed + active records; a count-only check
	// would pass a transposition (e.g. expired slipping in as active drops out).
	wantActive := []string{
		"a0000000-0000-0000-0000-000000000001",
		"a0000000-0000-0000-0000-000000000002",
	}
	assertActiveSet := func(label string, ms []Memory) {
		t.Helper()
		got := make(map[string]bool, len(ms))
		for _, m := range ms {
			got[m.ID] = true
		}
		if len(got) != len(wantActive) {
			t.Errorf("%s: got ids %v want exactly %v", label, got, wantActive)
			return
		}
		for _, id := range wantActive {
			if !got[id] {
				t.Errorf("%s: missing expected id %s (got %v)", label, id, got)
			}
		}
	}

	hits, err := s.Search(ctx, scope, subj, []float32{0.1, 0.2, 0.3}, 10, nil, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	assertActiveSet("Search", hits)
	lst, _, _, err := s.List(ctx, scope, subj, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	assertActiveSet("List", lst)
	// By-id is ungated: the scheduled record is still fetchable directly.
	if _, err := s.GetReadable(ctx, "a0000000-0000-0000-0000-000000000003", subj); err != nil {
		t.Errorf("GetReadable on scheduled record should be ungated, got %v", err)
	}
}

func TestListScheduledStates(t *testing.T) {
	s := testStore(t)
	fixed := time.Date(2030, 6, 15, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return fixed }
	ctx := context.Background()
	scope := "sched-test:project:x"
	subj := Authenticated("sub-A")
	past := fixed.Add(-24 * time.Hour)
	future := fixed.Add(24 * time.Hour)

	mk := func(id string, nb, na *time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A",
			CreatedAt: fixed, NotBefore: nb, NotAfter: na}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
		t.Cleanup(func() { cleanupErr(t, id, s.Delete(ctx, id, subj)) })
	}
	mk("b0000000-0000-0000-0000-000000000001", &future, nil)   // scheduled
	mk("b0000000-0000-0000-0000-000000000002", nil, &past)     // expired
	mk("b0000000-0000-0000-0000-000000000003", &past, &future) // active -> never listed

	sched, err := s.ListScheduled(ctx, scope, subj, ScheduledPending, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("scheduled: %v", err)
	}
	if len(sched) != 1 || sched[0].ID != "b0000000-0000-0000-0000-000000000001" {
		t.Errorf("ScheduledPending: got %d want 1 (the future record)", len(sched))
	}
	exp, err := s.ListScheduled(ctx, scope, subj, ScheduledExpired, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("expired: %v", err)
	}
	if len(exp) != 1 || exp[0].ID != "b0000000-0000-0000-0000-000000000002" {
		t.Errorf("ScheduledExpired: got %d want 1 (the past record)", len(exp))
	}
	all, err := s.ListScheduled(ctx, scope, subj, ScheduledAll, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("ScheduledAll: got %d want 2 (scheduled+expired, never active)", len(all))
	}
}

// TestListScheduledRejectsInvalidState pins the store-layer guard (hr2.5): an
// unrecognized ScheduledState must be rejected outright, not silently treated as
// ScheduledPending. The handler validates already; this defends direct callers
// that bypass it.
func TestListScheduledRejectsInvalidState(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	_, err := s.ListScheduled(ctx, "any:project:x", Authenticated("sub-A"),
		ScheduledState("bogus"), ListOptions{Limit: 10})
	if err == nil {
		t.Fatal("ListScheduled with invalid state returned nil error; want a store-layer rejection")
	}
}

// TestListScheduledOwnerIsolation pins that ListScheduled is owner-only: a caller
// never sees another actor's scheduled/expired records — not even `shared` ones.
// A shared+scheduled memory must stay invisible to other actors until it becomes
// active (then normal recall surfaces it); the management view must not leak it
// early. Guards the deferred-reveal guarantee against an ownerOrShared regression.
func TestListScheduledOwnerIsolation(t *testing.T) {
	s := testStore(t)
	fixed := time.Date(2030, 6, 15, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return fixed }
	ctx := context.Background()
	scope := "sched-iso:project:x"
	future := fixed.Add(24 * time.Hour)
	past := fixed.Add(-24 * time.Hour)

	mkVis := func(id, owner, vis string, nb, na *time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: owner, Visibility: vis,
			CreatedAt: fixed, NotBefore: nb, NotAfter: na}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
		t.Cleanup(func() { cleanupErr(t, id, s.Delete(ctx, id, Authenticated(owner))) })
	}
	// sub-A's records: one private+scheduled, one shared+scheduled, one shared+expired.
	mkVis("d0000000-0000-0000-0000-000000000001", "sub-A", "", &future, nil)
	mkVis("d0000000-0000-0000-0000-000000000002", "sub-A", "shared", &future, nil)
	mkVis("d0000000-0000-0000-0000-000000000003", "sub-A", "shared", nil, &past)

	// sub-B must see NONE of sub-A's windowed records via any state — owner-only.
	subB := Authenticated("sub-B")
	for _, st := range []ScheduledState{ScheduledPending, ScheduledExpired, ScheduledAll} {
		got, err := s.ListScheduled(ctx, scope, subB, st, ListOptions{Limit: 10})
		if err != nil {
			t.Fatalf("ListScheduled(%s) for sub-B: %v", st, err)
		}
		if len(got) != 0 {
			t.Errorf("ListScheduled(%s): sub-B saw %d of sub-A's records (incl. shared); want 0 — owner-only", st, len(got))
		}
	}
	// sub-A still sees their own: 2 scheduled (pending) + 1 expired = 3 in `all`.
	own, err := s.ListScheduled(ctx, scope, Authenticated("sub-A"), ScheduledAll, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("ListScheduled(all) for sub-A: %v", err)
	}
	if len(own) != 3 {
		t.Errorf("ListScheduled(all) for owner sub-A: got %d want 3 (own records visible)", len(own))
	}
}

// TestNotAfterBoundaryInstant pins the epoch-second boundary semantics at the
// exact instant not_after == now (hr2.8). The three sites deliberately use
// different operators, and their interaction at the boundary is the subtle part:
//   - active recall gate:    not_after > now  (Gt)  → at ==now, NOT active (hidden)
//   - ListScheduled expired: not_after <= now (Lte) → at ==now, IS expired (shown)
//   - PruneExpired:          not_after < before (Lt) → at before==not_after, KEPT;
//     one second past the boundary, pruned.
func TestNotAfterBoundaryInstant(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	fixed := time.Date(2031, 6, 1, 12, 0, 0, 0, time.UTC) // epoch-second aligned
	s.now = func() time.Time { return fixed }             // white-box recall clock
	scope := "boundary:project:x"
	subj := Authenticated("sub-bnd")
	notAfter := fixed // record expires AT the comparison instant
	id := "e0000000-0000-0000-0000-000000000001"
	m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-bnd",
		CreatedAt: fixed.Add(-time.Hour), NotAfter: &notAfter}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	t.Cleanup(func() { cleanupErr(t, id, s.Delete(ctx, id, subj)) })

	// Active recall gate excludes it: Gt means not_after==now is already past the gate.
	items, _, _, err := s.List(ctx, scope, subj, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, it := range items {
		if it.ID == id {
			t.Error("not_after==now leaked into active recall; the gate (Gt) must hide it at the boundary")
		}
	}

	// ListScheduled(expired) includes it: Lte means not_after==now counts as expired.
	exp, err := s.ListScheduled(ctx, scope, subj, ScheduledExpired, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("ListScheduled: %v", err)
	}
	foundExpired := false
	for _, it := range exp {
		if it.ID == id {
			foundExpired = true
		}
	}
	if !foundExpired {
		t.Error("not_after==now not surfaced as expired; the expired clause (Lte) must include it at the boundary")
	}

	// PruneExpired keeps it at before==boundary (Lt is strict)...
	n, err := s.PruneExpired(ctx, fixed)
	if err != nil {
		t.Fatalf("prune at boundary: %v", err)
	}
	if n != 0 {
		t.Errorf("PruneExpired(before==not_after) deleted %d, want 0 (Lt is strict)", n)
	}
	if _, err := s.Get(ctx, id); err != nil {
		t.Errorf("record pruned at the boundary instant, want kept: %v", err)
	}

	// ...and removes it one second past the boundary.
	n, err = s.PruneExpired(ctx, fixed.Add(time.Second))
	if err != nil {
		t.Fatalf("prune past boundary: %v", err)
	}
	if n != 1 {
		t.Errorf("PruneExpired(before=not_after+1s) deleted %d, want 1", n)
	}
	if _, err := s.Get(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("record survived one second past the boundary, want pruned: %v", err)
	}
}

// TestPruneExpiredGracePeriod pins the partitioning a past cutoff produces
// (hr2.9): PruneExpired(before) with before in the past spares records that
// lapsed AFTER the cutoff (recently expired, still within the grace window) and
// removes only those that lapsed at or before it. This is the behavior the
// prune-expired --older-than flag yields via pruneCutoff(now, grace).
func TestPruneExpiredGracePeriod(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "prune-grace:project:x"
	subj := Authenticated("sub-A")
	now := time.Now().UTC()
	cutoff := now.Add(-24 * time.Hour) // grace = 24h
	longAgo := now.Add(-48 * time.Hour)
	recently := now.Add(-1 * time.Hour) // expired, but inside the grace window

	mk := func(id string, na time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A", CreatedAt: longAgo, NotAfter: &na}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
		t.Cleanup(func() { cleanupErr(t, id, s.Delete(ctx, id, subj)) })
	}
	mk("f0000000-0000-0000-0000-000000000001", longAgo)  // lapsed before cutoff -> pruned
	mk("f0000000-0000-0000-0000-000000000002", recently) // lapsed within grace -> kept

	n, err := s.PruneExpired(ctx, cutoff)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if n != 1 {
		t.Errorf("PruneExpired(now-24h grace): deleted %d, want 1 (only the record older than the grace window)", n)
	}
	if _, err := s.Get(ctx, "f0000000-0000-0000-0000-000000000001"); !errors.Is(err, ErrNotFound) {
		t.Errorf("record older than grace should be pruned, got %v", err)
	}
	if _, err := s.Get(ctx, "f0000000-0000-0000-0000-000000000002"); err != nil {
		t.Errorf("recently-expired record (within grace) should survive, got %v", err)
	}
}

func TestPruneExpired(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "prune-test:project:x"
	subj := Authenticated("sub-A")
	now := time.Now().UTC()
	old := now.Add(-48 * time.Hour)
	future := now.Add(48 * time.Hour)

	mk := func(id string, na *time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A", CreatedAt: now, NotAfter: na}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
		t.Cleanup(func() { cleanupErr(t, id, s.Delete(ctx, id, subj)) })
	}
	mk("c0000000-0000-0000-0000-000000000001", &old)    // expired -> pruned
	mk("c0000000-0000-0000-0000-000000000002", &future) // not expired -> kept
	mk("c0000000-0000-0000-0000-000000000003", nil)     // no window -> kept

	n, err := s.PruneExpired(ctx, now)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if n != 1 {
		t.Errorf("PruneExpired: deleted %d want 1", n)
	}
	if _, err := s.Get(ctx, "c0000000-0000-0000-0000-000000000001"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired record should be gone, got %v", err)
	}
	if _, err := s.Get(ctx, "c0000000-0000-0000-0000-000000000002"); err != nil {
		t.Errorf("future record should survive, got %v", err)
	}
	if _, err := s.Get(ctx, "c0000000-0000-0000-0000-000000000003"); err != nil {
		t.Errorf("unwindowed record (no not_after) should survive, got %v", err)
	}
}

func TestRemapOwner(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:remap"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	mk := func(id, owner string) {
		if err := s.Upsert(ctx, Memory{ID: id, Content: "c", Scope: scope, Owner: owner, CreatedAt: time.Now().UTC()}, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	subID := "e0e0e0e0-0000-0000-0000-000000000001"
	emailID := "e0e0e0e0-0000-0000-0000-000000000002"
	mk(subID, "old-sub-123")
	mk(emailID, "old@example.com")

	if _, err := s.RemapOwner(ctx, RemapFrom("x"), "", false); err == nil {
		t.Error("empty to: expected error")
	}
	if _, err := s.RemapOwner(ctx, RemapFrom("a"), "a", false); err == nil {
		t.Error("From==to: expected error")
	}
	// Sealing eliminates the old multi-select combination (e.g. the former
	// OwnerRemapSource{Missing: true, From: "x"}) at compile time — there is
	// no way to construct it via RemapMissing()/RemapAnon()/RemapFrom(). The
	// interface's own zero value (nil) is still trivially constructible,
	// though, and RemapOwner's type switch must still reject it explicitly.
	if _, err := s.RemapOwner(ctx, nil, "to", false); err == nil {
		t.Error("nil source: expected error")
	}

	n, err := s.RemapOwner(ctx, RemapFrom("old-sub-123"), "sean@example.com", true)
	if err != nil || n != 1 {
		t.Fatalf("dry-run: n=%d err=%v, want 1,nil", n, err)
	}
	if got, _ := s.Get(ctx, subID); got.Owner != "old-sub-123" {
		t.Errorf("dry-run mutated owner to %q", got.Owner)
	}

	if n, err = s.RemapOwner(ctx, RemapFrom("old-sub-123"), "sean@example.com", false); err != nil || n != 1 {
		t.Fatalf("sub->email: n=%d err=%v", n, err)
	}
	if got, _ := s.Get(ctx, subID); got.Owner != "sean@example.com" {
		t.Errorf("sub->email owner = %q", got.Owner)
	}

	if n, err = s.RemapOwner(ctx, RemapFrom("old@example.com"), "new@example.com", false); err != nil || n != 1 {
		t.Fatalf("email->email: n=%d err=%v", n, err)
	}
	if got, _ := s.Get(ctx, emailID); got.Owner != "new@example.com" {
		t.Errorf("email->email owner = %q", got.Owner)
	}

	// Seed an owner-less (missing key, pre-isolation) record and an explicit
	// anonymous-bucket (owner=="") record to prove the Missing (IsEmpty) vs Anon
	// (Match "") filters target DIFFERENT record sets.
	missingID := "e0e0e0e0-0000-0000-0000-000000000003"
	pm := payload(Memory{ID: missingID, Content: "legacy", Scope: scope, CreatedAt: time.Now().UTC()})
	delete(pm, "owner")
	if _, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Points: []*qdrant.PointStruct{{
			Id: qdrant.NewID(missingID), Vectors: qdrant.NewVectors(0.1, 0.2, 0.3),
			Payload: qdrant.NewValueMap(pm),
		}},
	}); err != nil {
		t.Fatalf("raw upsert owner-less record: %v", err)
	}
	anonID := "e0e0e0e0-0000-0000-0000-000000000004"
	mk(anonID, "") // explicit owner==""

	// Missing matches ONLY the owner-less record, not the owner=="" bucket.
	if n, err = s.RemapOwner(ctx, RemapMissing(), "backfill@example.com", false); err != nil || n != 1 {
		t.Fatalf("missing->email: n=%d err=%v (want exactly 1; owner=='' must not match IsEmpty)", n, err)
	}
	if got, _ := s.Get(ctx, missingID); got.Owner != "backfill@example.com" {
		t.Errorf("missing->email owner = %q", got.Owner)
	}
	if got, _ := s.Get(ctx, anonID); got.Owner != "" {
		t.Errorf("anon bucket hijacked by Missing remap: owner=%q, want \"\"", got.Owner)
	}

	// Anon matches ONLY the explicit owner=="" record.
	if n, err = s.RemapOwner(ctx, RemapAnon(), "claimed@example.com", false); err != nil || n != 1 {
		t.Fatalf("anon->email: n=%d err=%v (want exactly 1)", n, err)
	}
	if got, _ := s.Get(ctx, anonID); got.Owner != "claimed@example.com" {
		t.Errorf("anon->email owner = %q", got.Owner)
	}
}

func TestRemapFromPanicsOnEmptyValue(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("RemapFrom(\"\") did not panic; an empty From must fail at construction, not silently collapse into a different source")
		}
	}()
	RemapFrom("")
}

// TestRemapOwnerIdempotentRerun verifies re-running an IDENTICAL RemapOwner
// call is a safe no-op: after the first run re-stamps the matching record, the
// From value no longer matches anything, so the rerun counts and mutates
// nothing rather than erroring or double-applying.
func TestRemapOwnerIdempotentRerun(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:remap-idempotent"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	id := "e0e0e0e0-0000-0000-0000-000000000005"
	if err := s.Upsert(ctx, Memory{ID: id, Content: "c", Scope: scope, Owner: "old-sub", CreatedAt: time.Now().UTC()}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	src := RemapFrom("old-sub")
	n1, err := s.RemapOwner(ctx, src, "new@example.com", false)
	if err != nil || n1 != 1 {
		t.Fatalf("first remap: n=%d err=%v, want 1,nil", n1, err)
	}

	n2, err := s.RemapOwner(ctx, src, "new@example.com", false)
	if err != nil || n2 != 0 {
		t.Fatalf("rerun: n=%d err=%v, want 0,nil (idempotent no-op)", n2, err)
	}
	if got, _ := s.Get(ctx, id); got.Owner != "new@example.com" {
		t.Errorf("owner drifted on rerun: %q, want unchanged from first remap", got.Owner)
	}
}

// TestMigrateSetOwnerEquivalentToRemapOwnerMissing pins the "deprecated alias"
// claim in migrateSetOwnerCmd.Deprecated ("use: migrate-remap-owner
// --from-missing --to <owner>"): MigrateSetOwner and
// RemapOwner(RemapMissing(), ...) are two independent store-layer
// implementations, so this regression-tests that they produce identical
// results against identical fixtures rather than assuming the doc comment
// stays true by construction.
func TestMigrateSetOwnerEquivalentToRemapOwnerMissing(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:migrate-alias"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	seedOwnerless := func(id string) {
		p := payload(Memory{ID: id, Content: "legacy", Scope: scope, CreatedAt: time.Now().UTC()})
		delete(p, "owner")
		if _, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
			CollectionName: s.collection, Wait: qdrant.PtrOf(true),
			Points: []*qdrant.PointStruct{{
				Id: qdrant.NewID(id), Vectors: qdrant.NewVectors(0.1, 0.2, 0.3),
				Payload: qdrant.NewValueMap(p),
			}},
		}); err != nil {
			t.Fatalf("raw upsert owner-less record %s: %v", id, err)
		}
	}

	// Both sweeps match owner-less records across the WHOLE collection (no
	// scope filter — see ownerlessFilter), so the two fixtures cannot coexist:
	// each is seeded, swept, and verified before the next is created.
	viaMigrateID := "e0e0e0e0-0000-0000-0000-000000000006"
	seedOwnerless(viaMigrateID)
	nMigrate, err := s.MigrateSetOwner(ctx, "backfill@example.com")
	if err != nil {
		t.Fatalf("MigrateSetOwner: %v", err)
	}
	if nMigrate != 1 {
		t.Fatalf("MigrateSetOwner stamped %d, want exactly 1", nMigrate)
	}
	gotMigrate, err := s.Get(ctx, viaMigrateID)
	if err != nil {
		t.Fatalf("Get viaMigrate: %v", err)
	}

	viaRemapID := "e0e0e0e0-0000-0000-0000-000000000007"
	seedOwnerless(viaRemapID)
	nRemap, err := s.RemapOwner(ctx, RemapMissing(), "backfill@example.com", false)
	if err != nil {
		t.Fatalf("RemapOwner(RemapMissing()): %v", err)
	}
	if nRemap != 1 {
		t.Fatalf("RemapOwner(RemapMissing()) stamped %d, want exactly 1", nRemap)
	}
	gotRemap, err := s.Get(ctx, viaRemapID)
	if err != nil {
		t.Fatalf("Get viaRemap: %v", err)
	}

	if nMigrate != nRemap {
		t.Errorf("migrate-set-owner stamped %d, migrate-remap-owner --from-missing stamped %d, want equal", nMigrate, nRemap)
	}
	if gotMigrate.Owner != gotRemap.Owner {
		t.Errorf("owner mismatch: migrate-set-owner=%q, migrate-remap-owner --from-missing=%q", gotMigrate.Owner, gotRemap.Owner)
	}
}

func TestRemapOwnerHonorsCancel(t *testing.T) {
	s := testStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.RemapOwner(ctx, RemapFrom("x"), "y", false); err == nil {
		t.Error("cancelled context: expected error")
	}
}

func TestPayloadRoundTripSummaryProvenance(t *testing.T) {
	m := Memory{
		ID: "11111111-1111-1111-1111-111111111111", Content: "long original content",
		Scope: "repo:x", Category: "convention", Source: "agent-inferred",
		Summary: "terse line", SummarySource: SummarySourceAuto, SummaryModel: "summary-cheap",
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	vm := qdrant.NewValueMap(payload(m))
	got := fromPayload(m.ID, vm)
	if got.Summary != "terse line" || got.SummarySource != SummarySourceAuto || got.SummaryModel != "summary-cheap" {
		t.Fatalf("summary provenance not round-tripped: %+v", got)
	}

	// A curated record with no summary must still round-trip cleanly (empty source).
	plain := Memory{ID: "22222222-2222-2222-2222-222222222222", Content: "c", Category: "gotcha"}
	g2 := fromPayload(plain.ID, qdrant.NewValueMap(payload(plain)))
	if g2.Summary != "" || g2.SummarySource != SummarySourceNone || g2.SummaryModel != "" {
		t.Fatalf("empty-summary record drifted: %+v", g2)
	}
}

func TestPayloadRoundTripsShortID(t *testing.T) {
	m := Memory{ID: "a0000000-0000-0000-0000-000000000001", ShortID: "j7k2m9p4x0", Content: "c", Scope: "s"}
	got := fromPayload(m.ID, qdrant.NewValueMap(payload(m)))
	if got.ShortID != "j7k2m9p4x0" {
		t.Fatalf("round-trip short_id = %q", got.ShortID)
	}
	// Empty ShortID MUST be omitted (not stamped as an explicit ""), so legacy /
	// reindexed records stay key-absent and the NewIsEmpty backfill filter matches them.
	if _, ok := payload(Memory{ID: "x"})["short_id"]; ok {
		t.Fatal("empty ShortID must be omitted from payload")
	}
}

// TestEnsureCollectionCreatesIndexes pins that EnsureCollection provisions the
// owner/scope/created_at payload indexes and is idempotent on a second call.
func TestEnsureCollectionCreatesIndexes(t *testing.T) {
	s := testStore(t) // testStore already calls EnsureCollection(ctx, 3) once
	ctx := context.Background()

	info, err := s.client.GetCollectionInfo(ctx, s.collection)
	if err != nil {
		t.Fatalf("GetCollectionInfo: %v", err)
	}
	schema := info.GetPayloadSchema()
	for _, field := range []string{"owner", "scope", "created_at"} {
		if _, ok := schema[field]; !ok {
			t.Errorf("payload index missing for %q; have %v", field, keysOf(schema))
		}
	}

	// Idempotent: a second EnsureCollection must not error on existing indexes.
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("second EnsureCollection: %v", err)
	}
}

func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// recordIDs extracts ids in slice order — a shared id-extraction helper for
// table assertions. The existing TestSearchAndListTagsFilter defines a local
// `ids` closure; there is no package-level `ids`, so this lives here.
func recordIDs(ms []Memory) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.ID)
	}
	return out
}

// TestListDateWindow pins half-open [after, before): a record AT created_after is
// included (gte); a record AT created_before is excluded (lt).
func TestListDateWindow(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "win-test:project:list"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	t0 := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	mk := func(id string, at time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A", CreatedAt: at}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("Upsert %s: %v", id, err)
		}
	}
	mk("a0000000-0000-0000-0000-000000000001", t0.Add(-time.Hour))  // before window
	mk("a0000000-0000-0000-0000-000000000002", t0)                  // == after  -> included
	mk("a0000000-0000-0000-0000-000000000003", t0.Add(time.Hour))   // inside
	mk("a0000000-0000-0000-0000-000000000004", t0.Add(2*time.Hour)) // == before -> excluded

	subj := Authenticated("sub-A")
	items, total, _, err := s.List(ctx, scope, subj, ListOptions{
		Limit: 10, CreatedAfter: t0, CreatedBefore: t0.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	got := recordIDs(items)
	want := []string{
		"a0000000-0000-0000-0000-000000000003",
		"a0000000-0000-0000-0000-000000000002",
	} // CreatedAt desc
	if !slices.Equal(got, want) {
		t.Errorf("window: got %v want %v", got, want)
	}
	if total != 2 {
		t.Errorf("window total: got %d want 2", total)
	}
}

// TestSearchDateWindow pins the same half-open window as a Search pre-filter.
func TestSearchDateWindow(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "win-test:project:search"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	t0 := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	mk := func(id string, at time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A", CreatedAt: at}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("Upsert %s: %v", id, err)
		}
	}
	mk("b0000000-0000-0000-0000-000000000001", t0.Add(-time.Hour))
	mk("b0000000-0000-0000-0000-000000000002", t0.Add(time.Hour))

	hits, err := s.Search(ctx, scope, Authenticated("sub-A"),
		[]float32{0.1, 0.2, 0.3}, 10, nil, t0, time.Time{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if got := recordIDs(hits); !slices.Equal(got, []string{"b0000000-0000-0000-0000-000000000002"}) {
		t.Errorf("search window: got %v want [..002]", got)
	}
}

// TestListCursorTraversal pins order-independent cursor paging at limit=1: N
// records sharing ONE timestamp plus M with distinct timestamps, paged to
// exhaustion, must yield the full set with no duplicates and no skips. At limit=1
// this only passes if resume over-fetches limit+len(seen).
func TestListCursorTraversal(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "cursor-test:project:x"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	tie := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	want := map[string]bool{}
	mk := func(id string, at time.Time) {
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A", CreatedAt: at}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("Upsert %s: %v", id, err)
		}
		want[id] = true
	}
	mk("d0000000-0000-0000-0000-000000000001", tie)
	mk("d0000000-0000-0000-0000-000000000002", tie)
	mk("d0000000-0000-0000-0000-000000000003", tie)
	mk("d0000000-0000-0000-0000-000000000004", tie)
	mk("d0000000-0000-0000-0000-000000000005", tie.Add(-time.Hour))
	mk("d0000000-0000-0000-0000-000000000006", tie.Add(-2*time.Hour))

	subj := Authenticated("sub-A")
	seen := map[string]int{}
	cursor := ""
	for steps := 0; steps < 100; steps++ {
		// CursorMode:true makes page 1 (Cursor:"") route through listByCursor, which
		// emits a nextCursor; without it the offset path runs and returns "" → break.
		items, _, next, err := s.List(ctx, scope, subj, ListOptions{Limit: 1, Cursor: cursor, CursorMode: true})
		if err != nil {
			t.Fatalf("List page: %v", err)
		}
		for _, m := range items {
			seen[m.ID]++
		}
		if next == "" {
			break
		}
		cursor = next
	}
	if len(seen) != len(want) {
		t.Errorf("traversal coverage: got %d distinct want %d", len(seen), len(want))
	}
	for id, nn := range seen {
		if nn != 1 {
			t.Errorf("record %s returned %d times (want 1) — dup/skip bug", id, nn)
		}
		if !want[id] {
			t.Errorf("unexpected id %s", id)
		}
	}
	for id := range want {
		if seen[id] == 0 {
			t.Errorf("record %s never returned — skip bug", id)
		}
	}
}

// TestListScheduledDateWindow pins the created_at window on the scheduled view.
func TestListScheduledDateWindow(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "sched-win:project:x"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	future := s.now().Add(48 * time.Hour)
	t0 := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	mk := func(id string, created time.Time) {
		nb := future
		m := Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A",
			NotBefore: &nb, CreatedAt: created}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("Upsert %s: %v", id, err)
		}
	}
	mk("e0000000-0000-0000-0000-000000000001", t0.Add(-time.Hour)) // before window
	mk("e0000000-0000-0000-0000-000000000002", t0.Add(time.Hour))  // inside

	got, err := s.ListScheduled(ctx, scope, Authenticated("sub-A"),
		ScheduledPending, ListOptions{Limit: 10, CreatedAfter: t0})
	if err != nil {
		t.Fatalf("ListScheduled: %v", err)
	}
	if len(got) != 1 || got[0].ID != "e0000000-0000-0000-0000-000000000002" {
		t.Errorf("scheduled window: got %v want just ..002", recordIDs(got))
	}
}

// TestListOffsetCursorMutuallyExclusive pins the guard: a List that sets both a
// resume Cursor and a positive Offset is a client error tagged ErrInvalidArgument
// (so the Connect layer maps it to CodeInvalidArgument, not CodeInternal).
func TestListOffsetCursorMutuallyExclusive(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	tok := encodeCursor(listCursor{C: "2026-06-27T12:00:00Z", Seen: []string{"x"}})
	_, _, _, err := s.List(ctx, "iso:project:x", Authenticated("sub-A"), ListOptions{
		Cursor: tok, Offset: 1, Limit: 10,
	})
	if err == nil {
		t.Fatal("List with both Cursor and Offset accepted; want error")
	}
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("want ErrInvalidArgument, got %v", err)
	}
}

// TestDatetimeIndexBackfillsExistingRecords proves the no-migration claim: records
// written as RFC3339 strings BEFORE the created_at datetime index exists become
// range-filterable AFTER the index is created (Qdrant backfills; no re-stamp).
func TestDatetimeIndexBackfillsExistingRecords(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	coll := "migration-realism-" + t.Name() // unique bare collection
	// 1. Bare collection — NO indexes.
	if err := s.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: coll,
		VectorsConfig:  qdrant.NewVectorsConfig(&qdrant.VectorParams{Size: 3, Distance: qdrant.Distance_Cosine}),
	}); err != nil {
		t.Fatalf("create bare collection: %v", err)
	}
	t.Cleanup(func() { _ = s.client.DeleteCollection(ctx, coll) })
	// 2. Insert RFC3339-string created_at records BEFORE any index exists.
	old := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	for i, at := range []time.Time{old.Add(-time.Hour), old, old.Add(time.Hour)} {
		if _, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
			CollectionName: coll, Wait: qdrant.PtrOf(true),
			Points: []*qdrant.PointStruct{{
				Id:      qdrant.NewID(fmt.Sprintf("a0000000-0000-0000-0000-00000000000%d", i+1)),
				Vectors: qdrant.NewVectors(0.1, 0.2, 0.3),
				Payload: qdrant.NewValueMap(map[string]any{"created_at": at.Format(time.RFC3339)}),
			}},
		}); err != nil {
			t.Fatalf("upsert pre-index %d: %v", i, err)
		}
	}
	// 3. NOW create the datetime index over the already-written string records.
	if _, err := s.client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndexCollection{
		CollectionName: coll, FieldName: "created_at",
		FieldType: qdrant.PtrOf(qdrant.FieldType_FieldTypeDatetime), Wait: qdrant.PtrOf(true),
	}); err != nil {
		t.Fatalf("create datetime index post-insert: %v", err)
	}
	// 4. A DatetimeRange query now returns the pre-index records — proves backfill, no migration.
	got, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: coll,
		Filter:         &qdrant.Filter{Must: []*qdrant.Condition{createdRangeCondition(old, time.Time{})}},
		Limit:          qdrant.PtrOf(uint32(10)), WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		t.Fatalf("range scroll post-index: %v", err)
	}
	if len(got) != 2 { // old (==after, gte) and old+1h; the -1h record is excluded
		t.Errorf("post-index range query: got %d records want 2 (pre-index records must be range-filterable)", len(got))
	}
}

func TestResolvePointID(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	defer func() { cleanupErr(t, "DeleteAllRaw s", st.DeleteAllRaw(ctx, "s")) }()
	vec := []float32{0.1, 0.2, 0.3}
	u := "a0000000-0000-0000-0000-000000000010"
	if err := st.Upsert(ctx, Memory{ID: u, ShortID: "j7k2m9p4x0", Content: "c", Scope: "s", Owner: "o"}, vec); err != nil {
		t.Fatal(err)
	}
	u2 := "a0000000-0000-0000-0000-000000000011"
	if err := st.Upsert(ctx, Memory{ID: u2, ShortID: "1a0bcdef23", Content: "c", Scope: "s", Owner: "o"}, vec); err != nil {
		t.Fatal(err)
	}

	check := func(name, in, wantID string, wantErr error) {
		got, err := st.ResolvePointID(ctx, in)
		if wantErr != nil {
			if !errors.Is(err, wantErr) {
				t.Fatalf("%s: err=%v want %v", name, err, wantErr)
			}
			return
		}
		if err != nil || got != wantID {
			t.Fatalf("%s: got %q err %v want %q", name, got, err, wantID)
		}
	}
	check("uuid fast path", u, u, nil)
	check("raw-hex uuid canonicalized", "a0000000000000000000000000000010", u, nil)
	check("short id exact", "j7k2m9p4x0", u, nil)
	check("short id upper+glyph+space", " IaObcdef23 ", u2, nil)                                   // I->1, O->0
	check("padded canonical uuid → fast path", "  a0000000-0000-0000-0000-000000000010  ", u, nil) // item 30
	check("nonexistent short id", "zzzzzzzzzz", "", ErrNotFound)
	check("8-char uuid prefix (original bug)", "a0000000", "", ErrNotFound)
	check("empty", "   ", "", ErrInvalidArgument)
}

func TestResolvePointIDAmbiguous(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	defer func() { cleanupErr(t, "DeleteAllRaw s", st.DeleteAllRaw(ctx, "s")) }()
	vec := []float32{0.1, 0.2, 0.3}
	// Force two records sharing a short id by writing them directly (bypassing MintShortID).
	for _, id := range []string{"a0000000-0000-0000-0000-000000000020", "a0000000-0000-0000-0000-000000000021"} {
		if err := st.Upsert(ctx, Memory{ID: id, ShortID: "dupdupdup0", Content: "c", Scope: "s", Owner: "o"}, vec); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.ResolvePointID(ctx, "dupdupdup0"); !errors.Is(err, ErrAmbiguousShortID) {
		t.Fatalf("want ErrAmbiguousShortID, got %v", err)
	}
}

func TestMintShortIDUnique(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	defer func() { cleanupErr(t, "DeleteAllRaw s", st.DeleteAllRaw(ctx, "s")) }()
	a, err := st.MintShortID(ctx, nil)
	if err != nil || len(a) != shortid.Length {
		t.Fatalf("mint a: %q err %v", a, err)
	}
	if err := st.Upsert(ctx, Memory{ID: "a0000000-0000-0000-0000-000000000030", ShortID: a, Content: "c", Scope: "s", Owner: "o"}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatal(err)
	}
	b, err := st.MintShortID(ctx, nil)
	if err != nil || b == a {
		t.Fatalf("mint b collided/errored: %q err %v", b, err)
	}
}

func TestMintShortIDRetriesOnCollision(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	defer func() { cleanupErr(t, "DeleteAllRaw s", st.DeleteAllRaw(ctx, "s")) }()
	// Persist "collidecol" so the first candidate collides, forcing the retry branch.
	if err := st.Upsert(ctx, Memory{ID: "a0000000-0000-0000-0000-000000000031", ShortID: "collidecol", Content: "c", Scope: "s", Owner: "o"}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	st.mintCandidate = func() (string, error) {
		calls++
		if calls == 1 {
			return "collidecol", nil // taken → must retry
		}
		return "freshfresh", nil
	}
	got, err := st.MintShortID(ctx, nil)
	if err != nil || got != "freshfresh" || calls != 2 {
		t.Fatalf("got %q err %v calls %d (want freshfresh / 2)", got, err, calls)
	}
}

// floatsEqual reports whether two float32 slices are element-wise equal.
func floatsEqual(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// upsertRawNoOwner writes a point straight through the client whose payload omits
// the owner key (mirrors seedSource's raw point). It exists to prove the
// absent-owner-key invariant survives BackfillShortIDs' payload-only SetPayload.
func upsertRawNoOwner(t *testing.T, s *Store, id string, vec []float32) error {
	t.Helper()
	_, err := s.client.Upsert(context.Background(), &qdrant.UpsertPoints{
		CollectionName: s.collection,
		Wait:           qdrant.PtrOf(true),
		Points: []*qdrant.PointStruct{{
			Id:      qdrant.NewID(id),
			Vectors: qdrant.NewVectors(vec...),
			Payload: qdrant.NewValueMap(map[string]any{"content": "c", "scope": "s"}),
		}},
	})
	return err
}

func TestBackfillShortIDs(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	defer func() { cleanupErr(t, "DeleteAllRaw s", st.DeleteAllRaw(ctx, "s")) }()
	vec := []float32{0.1, 0.2, 0.3}
	// >reindexBatch records so the cursor loop pages more than once (item 25).
	const total = 300
	for i := 0; i < total; i++ {
		id := fmt.Sprintf("a0000000-0000-0000-0000-%012d", i)
		if err := st.Upsert(ctx, Memory{ID: id, Content: "c", Scope: "s", Owner: "o"}, vec); err != nil {
			t.Fatal(err)
		}
	}
	// One extra record written WITHOUT an owner key, to prove the absent-owner
	// invariant survives the payload-only SetPayload.
	rawID := "b0000000-0000-0000-0000-000000000000"
	if err := upsertRawNoOwner(t, st, rawID, vec); err != nil { // helper below
		t.Fatal(err)
	}

	// dry-run counts, writes nothing
	n, err := st.BackfillShortIDs(ctx, true)
	if err != nil || n != total+1 {
		t.Fatalf("dry-run n=%d err=%v", n, err)
	}
	pts := scrollPoints(t, st.client, st.collection)
	if pts["a0000000-0000-0000-0000-000000000000"].payload["short_id"].GetStringValue() != "" {
		t.Fatal("dry-run wrote a short id")
	}
	// The collection is Cosine-distance, so Qdrant stores vectors L2-normalized —
	// the read-back vector never equals the raw input. Snapshot the stored vector
	// now (dry-run wrote nothing) so the post-apply check can prove the
	// payload-only SetPayload left it untouched.
	rawVec := pts[rawID].vec

	// apply, then assert every record got a distinct short id
	n, err = st.BackfillShortIDs(ctx, false)
	if err != nil || n != total+1 {
		t.Fatalf("apply n=%d err=%v", n, err)
	}
	pts = scrollPoints(t, st.client, st.collection)
	uniq := map[string]struct{}{}
	for id, p := range pts {
		sid := p.payload["short_id"].GetStringValue()
		if len(sid) != shortid.Length {
			t.Fatalf("%s short id %q", id, sid)
		}
		uniq[sid] = struct{}{}
	}
	if len(uniq) != total+1 {
		t.Fatalf("short ids not globally unique: %d distinct of %d", len(uniq), total+1)
	}
	// vector preserved (no re-embed) + absent-owner invariant preserved
	if !floatsEqual(pts[rawID].vec, rawVec) {
		t.Fatal("backfill changed a vector")
	}
	if _, ok := pts[rawID].payload["owner"]; ok {
		t.Fatal("backfill synthesized an owner key on the raw point")
	}

	// idempotent: second run finds nothing to do
	if n, err = st.BackfillShortIDs(ctx, false); err != nil || n != 0 {
		t.Fatalf("idempotent run n=%d err=%v", n, err)
	}
}

// TestBackfillShortIDsHonorsCancel verifies the backfill propagates context
// cancellation to its Qdrant calls instead of running to completion — the
// property the CLI relies on for its --timeout / Ctrl-C bound.
func TestBackfillShortIDsHonorsCancel(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	defer func() { cleanupErr(t, "DeleteAllRaw s", st.DeleteAllRaw(ctx, "s")) }()
	vec := []float32{0.1, 0.2, 0.3}
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("c0000000-0000-0000-0000-%012d", i)
		if err := st.Upsert(ctx, Memory{ID: id, Content: "c", Scope: "s", Owner: "o"}, vec); err != nil {
			t.Fatal(err)
		}
	}
	cctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := st.BackfillShortIDs(cctx, false); err == nil {
		t.Error("cancelled context: expected error")
	}
}
