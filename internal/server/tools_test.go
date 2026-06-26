// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qdrant/go-client/qdrant"
	flag "github.com/spf13/pflag"
	tcqdrant "github.com/testcontainers/testcontainers-go/modules/qdrant"
	"go.opentelemetry.io/otel"

	"github.com/seanb4t/engram/internal/config"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/telemetry"
)

// TestToolArgSchemasDoNotPanic exercises jsonschema schema generation for every
// tool's argument type via mcp.AddTool — the exact path that panicked at startup
// in v0.4.2 ("tag must not begin with 'WORD='") because no test covered the
// AddTool calls in Register (the handler tests use deps directly). Pure unit
// test: no Qdrant or embedder. A bad jsonschema tag panics here instead of in
// production at first serve.
func TestToolArgSchemasDoNotPanic(t *testing.T) {
	check := func(name string, register func(*mcp.Server)) {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("AddTool schema generation panicked: %v", r)
				}
			}()
			register(mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil))
		})
	}
	noop := func() (*mcp.CallToolResult, any, error) { return nil, nil, nil }
	check("store_memory", func(s *mcp.Server) {
		mcp.AddTool(s, &mcp.Tool{Name: "store_memory", Description: "x"}, func(context.Context, *mcp.CallToolRequest, storeArgs) (*mcp.CallToolResult, any, error) {
			return noop()
		})
	})
	check("search_memory", func(s *mcp.Server) {
		mcp.AddTool(s, &mcp.Tool{Name: "search_memory", Description: "x"}, func(context.Context, *mcp.CallToolRequest, searchArgs) (*mcp.CallToolResult, any, error) {
			return noop()
		})
	})
	check("list_memory", func(s *mcp.Server) {
		mcp.AddTool(s, &mcp.Tool{Name: "list_memory", Description: "x"}, func(context.Context, *mcp.CallToolRequest, listArgs) (*mcp.CallToolResult, any, error) { return noop() })
	})
	check("get_memory", func(s *mcp.Server) {
		mcp.AddTool(s, &mcp.Tool{Name: "get_memory", Description: "x"}, func(context.Context, *mcp.CallToolRequest, idArgs) (*mcp.CallToolResult, any, error) { return noop() })
	})
	check("update_memory", func(s *mcp.Server) {
		mcp.AddTool(s, &mcp.Tool{Name: "update_memory", Description: "x"}, func(context.Context, *mcp.CallToolRequest, updateArgs) (*mcp.CallToolResult, any, error) {
			return noop()
		})
	})
	check("delete_all", func(s *mcp.Server) {
		mcp.AddTool(s, &mcp.Tool{Name: "delete_all", Description: "x"}, func(context.Context, *mcp.CallToolRequest, scopeArgs) (*mcp.CallToolResult, any, error) {
			return noop()
		})
	})
	check("store_discovery", func(s *mcp.Server) {
		mcp.AddTool(s, &mcp.Tool{Name: "store_discovery", Description: "x"}, func(context.Context, *mcp.CallToolRequest, storeDiscoveryArgs) (*mcp.CallToolResult, any, error) {
			return noop()
		})
	})
	check("search_discovery", func(s *mcp.Server) {
		mcp.AddTool(s, &mcp.Tool{Name: "search_discovery", Description: "x"}, func(context.Context, *mcp.CallToolRequest, searchDiscoveryArgs) (*mcp.CallToolResult, any, error) {
			return noop()
		})
	})
	check("set_visibility", func(s *mcp.Server) {
		mcp.AddTool(s, &mcp.Tool{Name: "set_visibility", Description: "x"}, func(context.Context, *mcp.CallToolRequest, setVisibilityArgs) (*mcp.CallToolResult, any, error) {
			return noop()
		})
	})
	check("schedule_memory", func(s *mcp.Server) {
		mcp.AddTool(s, &mcp.Tool{Name: "schedule_memory", Description: "x"}, func(context.Context, *mcp.CallToolRequest, scheduleArgs) (*mcp.CallToolResult, any, error) {
			return noop()
		})
	})
	check("list_scheduled", func(s *mcp.Server) {
		mcp.AddTool(s, &mcp.Tool{Name: "list_scheduled", Description: "x"}, func(context.Context, *mcp.CallToolRequest, listScheduledArgs) (*mcp.CallToolResult, any, error) {
			return noop()
		})
	})
}

// testQdrantAddr is the gRPC host:port the integration tests run against. Set by
// TestMain: ENGRAM_QDRANT_TEST_ADDR if provided (fast path / override), else an
// ephemeral testcontainer. Empty when neither is available (Docker absent), in
// which case the integration tests skip.
var testQdrantAddr string

// TestMain provisions Qdrant for this package's integration tests. It prefers an
// existing instance via ENGRAM_QDRANT_TEST_ADDR; otherwise it boots an ephemeral
// Qdrant via testcontainers and tears it down afterward. If neither is available
// the suite still runs — the integration tests skip with a clear message.
func TestMain(m *testing.M) {
	if addr := os.Getenv("ENGRAM_QDRANT_TEST_ADDR"); addr != "" {
		testQdrantAddr = addr
		os.Exit(m.Run())
	}
	// Bound startup so an unreachable daemon or a stalled image pull fails fast
	// instead of hanging the suite. os.Exit skips defers, so cancel explicitly.
	startCtx, startCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	container, err := tcqdrant.Run(startCtx, "qdrant/qdrant:v1.18.2")
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

// fakeEmbedder returns a fixed vector so handler tests don't need a live embedder.
type fakeEmbedder struct{}

func (fakeEmbedder) Embed(context.Context, string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

// countingEmbedder wraps fakeEmbedder and records how many Embed calls were made.
// Used to assert that the embedder is NOT called when the ownership pre-check
// rejects the request early (cost-amplification hardening, eu8.4/eu8.2).
type countingEmbedder struct {
	calls int
}

func (e *countingEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	e.calls++
	return fakeEmbedder{}.Embed(ctx, text)
}

// testDeps builds a deps backed by a live Qdrant (skip-gated, same posture as the
// store integration tests) and the fake embedder. deps.em is an interface so the
// embedder is fakeable; deps.st is concrete, hence the Qdrant gate.
func testDeps(t *testing.T) *deps {
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
	st := store.New(c, "mem_eval_test")
	if err := st.EnsureCollection(context.Background(), 3); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	return &deps{st: st, em: fakeEmbedder{}}
}

// cleanupErr surfaces a deferred-cleanup failure so leftover records can't
// silently contaminate later tests in the run. store.ErrNotFound is tolerated:
// the record is already gone, which is exactly what cleanup wanted.
func cleanupErr(t *testing.T, what string, err error) {
	t.Helper()
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		t.Errorf("cleanup %s: %v", what, err)
	}
}

// authedContext builds a context carrying a verified OIDC subject by running a
// request through the go-sdk's RequireBearerToken middleware with a stub
// verifier. The go-sdk stores TokenInfo under an unexported context key, so this
// middleware round-trip is the only way to inject an authenticated identity that
// subjectFromContext can read — it is what makes authenticated handler-path tests
// possible.
func authedContext(t *testing.T, sub string) context.Context {
	t.Helper()
	verifier := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return &mcpauth.TokenInfo{
			Expiration: timeNow().Add(time.Hour),
			Extra:      map[string]any{"sub": sub},
		}, nil
	}
	var captured context.Context
	h := mcpauth.RequireBearerToken(verifier, nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = r.Context()
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if captured == nil {
		t.Fatal("authedContext: middleware did not pass request through (verification failed)")
	}
	return captured
}

// TestScheduleMemoryUsesInjectedClock pins hr2.3/7: schedule_memory window
// validation reads the deps clock, not the wall clock, so tests can pin "now"
// deterministically. With the clock pinned to the year 2100, a not_after in 2099
// — still future by the wall clock but PAST the injected now — must be rejected.
func TestScheduleMemoryUsesInjectedClock(t *testing.T) {
	// No Qdrant/embedder needed: the not_after-in-the-past rejection happens in
	// parseWindow, before the store or embedder are touched — so this covers the
	// clock seam even in a Docker-less CI run (hx5x.4).
	d := &deps{now: func() time.Time { return time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC) }}
	ctx := authedContext(t, "sub-A")
	a := scheduleArgs{storeArgs: storeArgs{Content: "x", Scope: "clock:project:x",
		Source: "user-said", Category: "decision"}, NotAfter: "2099-01-01T00:00:00Z"}
	if _, err := d.scheduleMemory(ctx, a); err == nil {
		t.Error("not_after before the injected now should be rejected; the handler must read the deps clock, not the wall clock")
	}
}

// TestStoreMemoryUsesInjectedClock pins that store_memory stamps CreatedAt from
// the deps clock seam, not the wall clock (hx5x.7): clock() is documented as the
// handler time source, so both store_memory and schedule_memory must read it.
func TestStoreMemoryUsesInjectedClock(t *testing.T) {
	d := testDeps(t)
	fixed := time.Date(2055, 3, 4, 5, 6, 7, 0, time.UTC)
	d.now = func() time.Time { return fixed }
	ctx := authedContext(t, "sub-A")
	id, err := d.storeMemory(ctx, storeArgs{Content: "x", Scope: "clock:project:store",
		Source: "user-said", Category: "decision"})
	if err != nil {
		t.Fatalf("storeMemory: %v", err)
	}
	t.Cleanup(func() { _ = d.st.Delete(context.Background(), id, store.Authenticated("sub-A")) })
	got, err := d.st.Get(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.CreatedAt.Equal(fixed) {
		t.Errorf("store_memory CreatedAt = %v, want injected %v (handler must read d.clock())", got.CreatedAt, fixed)
	}
}

func TestScheduleMemoryValidation(t *testing.T) {
	d := testDeps(t) // skips if no Qdrant
	ctx := authedContext(t, "sub-A")
	base := scheduleArgs{storeArgs: storeArgs{Content: "do X next week",
		Scope: "sched:project:x", Source: "user-said", Category: "decision"}}

	// No window at all -> rejected.
	if _, err := d.scheduleMemory(ctx, base); err == nil {
		t.Error("missing window: want error, got nil")
	}
	// not_after already in the past -> rejected.
	past := base
	past.NotAfter = "2000-01-01T00:00:00Z"
	if _, err := d.scheduleMemory(ctx, past); err == nil {
		t.Error("past not_after: want error, got nil")
	}
	// Inverted window (not_before >= not_after) -> rejected.
	inv := base
	inv.NotBefore = "2031-01-01T00:00:00Z"
	inv.NotAfter = "2030-01-01T00:00:00Z"
	if _, err := d.scheduleMemory(ctx, inv); err == nil {
		t.Error("inverted window: want error, got nil")
	}
	// discovery category -> rejected.
	disc := base
	disc.Category = "discovery"
	disc.NotBefore = "2030-01-01T00:00:00Z"
	if _, err := d.scheduleMemory(ctx, disc); err == nil {
		t.Error("discovery category: want error, got nil")
	}
	// Valid future-scheduled memory -> stored, hidden from normal recall.
	ok := base
	ok.NotBefore = "2030-01-01T00:00:00Z"
	id, err := d.scheduleMemory(ctx, ok)
	if err != nil {
		t.Fatalf("valid schedule: %v", err)
	}
	t.Cleanup(func() { _ = d.st.Delete(context.Background(), id, store.Authenticated("sub-A")) })
	hits, _ := d.listMemory(ctx, listArgs{Scope: "sched:project:x"})
	for _, h := range hits {
		if h.ID == id {
			t.Error("future-scheduled memory leaked into normal list_memory")
		}
	}

	// A past not_before-only window is accepted (already revealed) and the record
	// is immediately active, so it surfaces through normal list_memory.
	activeNow := base
	activeNow.NotBefore = "2000-01-01T00:00:00Z"
	activeID, err := d.scheduleMemory(ctx, activeNow)
	if err != nil {
		t.Fatalf("past not_before should be accepted (immediately active): %v", err)
	}
	t.Cleanup(func() { _ = d.st.Delete(context.Background(), activeID, store.Authenticated("sub-A")) })
	active, _ := d.listMemory(ctx, listArgs{Scope: "sched:project:x"})
	found := false
	for _, h := range active {
		if h.ID == activeID {
			found = true
		}
	}
	if !found {
		t.Error("active (past not_before) memory should appear in normal list_memory")
	}
}

// TestStoreMemoryStampsOwnerHandler pins that the storeMemory handler stamps
// Memory.Owner from the authenticated subject (subj.Owner()) through the full
// handler path. Every other owner-bearing test seeds records via st.Upsert, so
// the handler's own owner-stamping line was never exercised end-to-end through
// authedContext. Integration: needs Qdrant.
func TestStoreMemoryStampsOwnerHandler(t *testing.T) {
	d := testDeps(t)
	ctx := authedContext(t, "sub-stamp")
	scope := "iso-test:project:store-stamp"
	defer func() {
		cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Authenticated("sub-stamp")))
	}()

	id, err := d.storeMemory(ctx, storeArgs{
		Content: "owned via handler", Scope: scope,
		Source: "user-said", Category: "preference",
	})
	if err != nil {
		t.Fatalf("storeMemory: %v", err)
	}
	got, err := d.st.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after storeMemory: %v", err)
	}
	if got.Owner != "sub-stamp" {
		t.Errorf("storeMemory did not stamp owner from subject: owner=%q, want %q", got.Owner, "sub-stamp")
	}
}

func TestValidateStoreDiscovery(t *testing.T) {
	good := storeDiscoveryArgs{
		Content: "x", Kind: "map", Scope: "discovery:repo:X",
		Citations: []citationArg{{Kind: "file", Ref: "f.go"}},
	}
	if err := validateStoreDiscovery(good); err != nil {
		t.Errorf("valid args rejected: %v", err)
	}
	const sc = "discovery:repo:X"
	cite := []citationArg{{Kind: "file", Ref: "f"}}
	bad := []struct {
		name string
		a    storeDiscoveryArgs
	}{
		{"bad kind", storeDiscoveryArgs{Content: "x", Kind: "blob", Scope: sc, Citations: cite}},
		{"empty kind", storeDiscoveryArgs{Content: "x", Kind: "", Scope: sc, Citations: cite}},
		{"no citations", storeDiscoveryArgs{Content: "x", Kind: "fact", Scope: sc}},
		{"empty content", storeDiscoveryArgs{Content: "", Kind: "fact", Scope: sc, Citations: cite}},
		{"empty scope", storeDiscoveryArgs{Content: "x", Kind: "fact", Scope: "", Citations: cite}},
		{"non-discovery scope", storeDiscoveryArgs{Content: "x", Kind: "fact", Scope: "repo:X", Citations: cite}},
		{"empty citation ref", storeDiscoveryArgs{Content: "x", Kind: "fact", Scope: sc, Citations: []citationArg{{Kind: "file", Ref: ""}}}},
		{"invalid citation kind", storeDiscoveryArgs{Content: "x", Kind: "fact", Scope: sc, Citations: []citationArg{{Kind: "blob", Ref: "f"}}}},
		{"empty citation kind", storeDiscoveryArgs{Content: "x", Kind: "fact", Scope: sc, Citations: []citationArg{{Kind: "", Ref: "f"}}}},
		{"second citation bad", storeDiscoveryArgs{Content: "x", Kind: "fact", Scope: sc, Citations: []citationArg{{Kind: "file", Ref: "ok"}, {Kind: "url", Ref: ""}}}},
		{"content too large", storeDiscoveryArgs{Content: strings.Repeat("a", maxDiscoveryContentBytes+1), Kind: "fact", Scope: sc, Citations: cite}},
		{"too many citations", storeDiscoveryArgs{Content: "x", Kind: "fact", Scope: sc, Citations: make([]citationArg, maxDiscoveryCitations+1)}},
		{"excerpt too large", storeDiscoveryArgs{Content: "x", Kind: "fact", Scope: sc, Citations: []citationArg{{Kind: "file", Ref: "f", Excerpt: strings.Repeat("a", maxCitationExcerptBytes+1)}}}},
	}
	for _, tc := range bad {
		if err := validateStoreDiscovery(tc.a); err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}

// TestAnonReadIsolationHandlers verifies that handler methods called with an
// anonymous context (context.Background() → sub=="") return anonymous-bucket
// records (explicit owner=="") but NOT owner-stamped shared records, satisfying
// the acceptance criteria for the anonymous-bucket read restriction through the
// full handler→store path. Integration: needs Qdrant.
func TestAnonReadIsolationHandlers(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "iso-test:project:anon-handler"

	ownerlessID := "a0000000-0000-0000-0000-000000000001"
	sharedID := "a0000000-0000-0000-0000-000000000002"

	// Seed anonymous-bucket record (explicit owner=="").
	ownerless := store.Memory{
		ID: ownerlessID, Content: "ownerless content", Scope: scope,
		Owner: "", Visibility: "private", Category: "convention",
		Source: "agent-inferred", CreatedAt: timeNow(),
	}
	if err := d.st.Upsert(ctx, ownerless, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed ownerless: %v", err)
	}

	// Seed owner-stamped shared record.
	shared := store.Memory{
		ID: sharedID, Content: "shared content", Scope: scope,
		Owner: "sub-owner", Visibility: "shared", Category: "convention",
		Source: "agent-inferred", CreatedAt: timeNow(),
	}
	if err := d.st.Upsert(ctx, shared, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed shared: %v", err)
	}

	defer func() {
		cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) // removes ownerless record
		cleanupErr(t, "Delete "+sharedID, d.st.Delete(ctx, sharedID, store.Authenticated("sub-owner")))
	}()

	// searchMemory with anonymous context must return ownerless, not shared.
	hits, err := d.searchMemory(ctx, searchArgs{Query: "content", Scope: scope, K: 10})
	if err != nil {
		t.Fatalf("searchMemory: %v", err)
	}
	foundOwnerless, foundShared := false, false
	for _, h := range hits {
		if h.ID == ownerlessID {
			foundOwnerless = true
		}
		if h.ID == sharedID {
			foundShared = true
		}
	}
	if !foundOwnerless {
		t.Error("searchMemory: ownerless record not returned for anonymous caller")
	}
	if foundShared {
		t.Error("searchMemory: owner-stamped shared record must not be returned to anonymous caller")
	}

	// list_memory handler with anonymous context must return ownerless, not shared.
	mems, err := d.listMemory(ctx, listArgs{Scope: scope, Limit: 10})
	if err != nil {
		t.Fatalf("listMemory: %v", err)
	}
	foundOwnerless, foundShared = false, false
	for _, m := range mems {
		if m.ID == ownerlessID {
			foundOwnerless = true
		}
		if m.ID == sharedID {
			foundShared = true
		}
	}
	if !foundOwnerless {
		t.Error("List: ownerless record not returned for anonymous caller")
	}
	if foundShared {
		t.Error("List: owner-stamped shared record must not be returned to anonymous caller")
	}

	// get_memory (GetReadable) on the shared record must return not-found for anon.
	if _, err := d.st.GetReadable(ctx, sharedID, store.Anonymous()); err == nil {
		t.Error("GetReadable: anonymous caller must not read owner-stamped shared record")
	}

	// get_memory on the ownerless record must succeed for anon.
	if _, err := d.st.GetReadable(ctx, ownerlessID, store.Anonymous()); err != nil {
		t.Errorf("GetReadable: anonymous caller must read ownerless record, got %v", err)
	}
}

// TestAnonReadIsolationDiscoveryHandler verifies the search_discovery handler
// obeys the same anonymous-bucket restriction: anonymous-bucket discovery
// records (explicit owner=="") are returned; owner-stamped shared discovery
// records are not. Integration: needs Qdrant.
func TestAnonReadIsolationDiscoveryHandler(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "discovery:repo:anon-handler-test"

	ownerlessID := "d0000000-0000-0000-0000-000000000001"
	sharedID := "d0000000-0000-0000-0000-000000000002"

	ownerlessDisc := store.Memory{
		ID: ownerlessID, Content: "ownerless discovery", Scope: scope,
		Owner: "", Visibility: "private", Category: "discovery",
		Kind: "fact", Source: "agent-inferred", CreatedAt: timeNow(),
		Citations: []store.Citation{{Kind: "file", Ref: "internal/auth/auth.go"}},
	}
	if err := d.st.Upsert(ctx, ownerlessDisc, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed ownerless discovery: %v", err)
	}

	sharedDisc := store.Memory{
		ID: sharedID, Content: "shared discovery", Scope: scope,
		Owner: "sub-owner", Visibility: "shared", Category: "discovery",
		Kind: "fact", Source: "agent-inferred", CreatedAt: timeNow(),
		Citations: []store.Citation{{Kind: "file", Ref: "internal/store/store.go"}},
	}
	if err := d.st.Upsert(ctx, sharedDisc, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed shared discovery: %v", err)
	}

	defer func() {
		cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous()))
		cleanupErr(t, "Delete "+sharedID, d.st.Delete(ctx, sharedID, store.Authenticated("sub-owner")))
	}()

	hits, err := d.searchDiscovery(ctx, searchDiscoveryArgs{Query: "discovery", Scope: scope, K: 10})
	if err != nil {
		t.Fatalf("searchDiscovery: %v", err)
	}
	foundOwnerless, foundShared := false, false
	for _, h := range hits {
		if h.ID == ownerlessID {
			foundOwnerless = true
		}
		if h.ID == sharedID {
			foundShared = true
		}
	}
	if !foundOwnerless {
		t.Error("searchDiscovery: ownerless discovery not returned for anonymous caller")
	}
	if foundShared {
		t.Error("searchDiscovery: owner-stamped shared discovery must not be returned to anonymous caller")
	}
}

func TestEffectiveDiscoveryScope(t *testing.T) {
	// cross_spine=false requires a scope.
	if _, err := effectiveDiscoveryScope(searchDiscoveryArgs{CrossSpine: false, Scope: ""}); err == nil {
		t.Error("expected error: scope required when cross_spine=false")
	}
	got, err := effectiveDiscoveryScope(searchDiscoveryArgs{CrossSpine: false, Scope: "discovery:repo:X"})
	if err != nil || got != "discovery:repo:X" {
		t.Errorf("scoped: got %q err %v", got, err)
	}
	// cross_spine=true spans all scopes (effective scope empty), scope ignored.
	got, err = effectiveDiscoveryScope(searchDiscoveryArgs{CrossSpine: true, Scope: "discovery:repo:X"})
	if err != nil || got != "" {
		t.Errorf("cross_spine: got %q err %v", got, err)
	}
}

// TestStoreAndSearchDiscoveryHandlers exercises the handler bodies end-to-end
// (embed → citation conversion → Memory assembly → Upsert → search), including
// the id-replace branch and the cross_spine path. Integration: needs Qdrant.
func TestStoreAndSearchDiscoveryHandlers(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "discovery:repo:handler-test"
	defer func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) }()

	// create
	id, err := d.storeDiscovery(ctx, storeDiscoveryArgs{
		Content: "auth flow maps token -> jwks -> actor", Kind: "fact", Scope: scope,
		Summary:   "auth flow",
		Citations: []citationArg{{Kind: "file", Ref: "internal/auth/auth.go", Locator: "1-50", Pin: "sha:abc", Excerpt: "verify(token)"}},
	})
	if err != nil {
		t.Fatalf("storeDiscovery create: %v", err)
	}
	if id == "" {
		t.Fatal("create returned empty id")
	}
	got, err := d.st.Get(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Category != "discovery" || got.Source != "agent-inferred" || got.Kind != "fact" {
		t.Errorf("handler set wrong fields: category=%q source=%q kind=%q", got.Category, got.Source, got.Kind)
	}
	if len(got.Citations) != 1 || got.Citations[0].Ref != "internal/auth/auth.go" {
		t.Errorf("citations not persisted: %+v", got.Citations)
	}

	// id-replace branch: same id replaces in place
	id2, err := d.storeDiscovery(ctx, storeDiscoveryArgs{
		ID: id, Content: "updated understanding", Kind: "map", Scope: scope,
		Citations: []citationArg{{Kind: "repo", Ref: "github.com/x/y", Pin: "@v1"}},
	})
	if err != nil {
		t.Fatalf("storeDiscovery replace: %v", err)
	}
	if id2 != id {
		t.Errorf("id-replace should reuse id: got %q want %q", id2, id)
	}
	rep, err := d.st.Get(ctx, id)
	if err != nil {
		t.Fatalf("get after replace: %v", err)
	}
	if rep.Content != "updated understanding" || rep.Kind != "map" {
		t.Errorf("replace did not update record: %+v", rep)
	}

	// scope-constrained search finds it
	hits, err := d.searchDiscovery(ctx, searchDiscoveryArgs{Query: "understanding", Scope: scope})
	if err != nil {
		t.Fatalf("searchDiscovery: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("expected >= 1 hit")
	}
	// scope required unless cross_spine
	if _, err := d.searchDiscovery(ctx, searchDiscoveryArgs{Query: "x"}); err == nil {
		t.Error("expected error: scope required when cross_spine=false")
	}
	// cross_spine path (with a scope present, the ignore-warn branch) must not error
	if _, err := d.searchDiscovery(ctx, searchDiscoveryArgs{Query: "x", CrossSpine: true, Scope: scope}); err != nil {
		t.Errorf("cross_spine search errored: %v", err)
	}
}

// TestSearchListMemoryTagsHandler pins that the optional tags filter threads
// through both the search_memory and list_memory handlers: a single tag narrows
// to records carrying it, AND of two tags excludes a record with only a subset,
// and omitting tags returns everything — verified on both handlers.
func TestSearchListMemoryTagsHandler(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "iso-test:project:tags-handler"
	alphaID := "d7000000-0000-0000-0000-000000000001" // alpha only
	bothID := "d7000000-0000-0000-0000-000000000002"  // alpha + beta
	plainID := "d7000000-0000-0000-0000-000000000003" // untagged
	defer func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) }()

	for _, m := range []store.Memory{
		{ID: alphaID, Content: "x", Scope: scope, Owner: "", Tags: []string{"alpha"}, CreatedAt: timeNow()},
		{ID: bothID, Content: "x", Scope: scope, Owner: "", Tags: []string{"alpha", "beta"}, CreatedAt: timeNow()},
		{ID: plainID, Content: "x", Scope: scope, Owner: "", CreatedAt: timeNow()},
	} {
		if err := d.st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}
	ids := func(ms []store.Memory) map[string]bool {
		out := map[string]bool{}
		for _, m := range ms {
			out[m.ID] = true
		}
		return out
	}

	// Single tag: both alpha-carrying records, never the untagged one — on both handlers.
	hits, err := d.searchMemory(ctx, searchArgs{Query: "x", Scope: scope, K: 10, Tags: []string{"alpha"}})
	if err != nil {
		t.Fatalf("searchMemory alpha: %v", err)
	}
	if g := ids(hits); !g[alphaID] || !g[bothID] || g[plainID] {
		t.Errorf("searchMemory alpha wrong: %v", g)
	}
	mems, err := d.listMemory(ctx, listArgs{Scope: scope, Limit: 10, Tags: []string{"alpha"}})
	if err != nil {
		t.Fatalf("listMemory alpha: %v", err)
	}
	if g := ids(mems); !g[alphaID] || !g[bothID] || g[plainID] {
		t.Errorf("listMemory alpha wrong: %v", g)
	}

	// AND of two tags: only the record carrying both; the alpha-only record is excluded.
	hits, err = d.searchMemory(ctx, searchArgs{Query: "x", Scope: scope, K: 10, Tags: []string{"alpha", "beta"}})
	if err != nil {
		t.Fatalf("searchMemory AND: %v", err)
	}
	if g := ids(hits); !g[bothID] || g[alphaID] || g[plainID] {
		t.Errorf("searchMemory AND-subset wrong: %v", g)
	}

	// Omitted tags: passthrough returns all three — on both handlers.
	hits, err = d.searchMemory(ctx, searchArgs{Query: "x", Scope: scope, K: 10})
	if err != nil {
		t.Fatalf("searchMemory passthrough: %v", err)
	}
	if g := ids(hits); !g[alphaID] || !g[bothID] || !g[plainID] {
		t.Errorf("searchMemory passthrough wrong: %v", g)
	}
	mems, err = d.listMemory(ctx, listArgs{Scope: scope, Limit: 10})
	if err != nil {
		t.Fatalf("listMemory passthrough: %v", err)
	}
	if g := ids(mems); !g[alphaID] || !g[bothID] || !g[plainID] {
		t.Errorf("listMemory passthrough wrong: %v", g)
	}
}

func TestUpdateMemoryPreservesSharingHandler(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "iso-test:project:handler-upd"
	id := "e5e5e5e5-0000-0000-0000-000000000001"
	defer func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) }()

	// Seed a shared record owned by the anonymous caller (sub == "").
	m := store.Memory{ID: id, Content: "v1", Scope: scope, Owner: "", Visibility: "shared", CreatedAt: timeNow()}
	if err := d.st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Content-only update (Shared nil) must preserve "shared".
	if err := d.updateMemory(ctx, updateArgs{ID: id, Content: "v2"}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := d.st.Get(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Content != "v2" || got.Visibility != "shared" {
		t.Errorf("handler content-only update lost sharing: content=%q visibility=%q", got.Content, got.Visibility)
	}
}

// TestUpdateMemoryTagsHandler pins the full tag-mutation contract: supplying
// tags replaces them, an empty slice clears them, omitting tags (nil) preserves
// the existing set, and the record's id/created_at survive the update (no
// delete-and-recreate). Mirrors TestUpdateMemoryPreservesSharingHandler's
// preserve-on-omit shape.
func TestUpdateMemoryTagsHandler(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "iso-test:project:handler-upd-tags"
	id := "e6e6e6e6-0000-0000-0000-000000000001"
	defer func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) }()

	// Seed a record with an initial tag, owned by the anonymous caller (sub == "").
	m := store.Memory{ID: id, Content: "v1", Scope: scope, Owner: "", Tags: []string{"old"}, CreatedAt: timeNow()}
	if err := d.st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	before, err := d.st.Get(ctx, id)
	if err != nil {
		t.Fatalf("get seed: %v", err)
	}

	// Supplying tags replaces them; id and created_at are preserved.
	newTags := []string{"alpha", "beta"}
	if err := d.updateMemory(ctx, updateArgs{ID: id, Content: "v2", Tags: &newTags}); err != nil {
		t.Fatalf("update with tags: %v", err)
	}
	got, err := d.st.Get(ctx, id)
	if err != nil {
		t.Fatalf("get after replace: %v", err)
	}
	if got.Content != "v2" {
		t.Errorf("content not updated: %q", got.Content)
	}
	if !slices.Equal(got.Tags, newTags) {
		t.Errorf("tags not replaced: got %v want %v", got.Tags, newTags)
	}
	if got.ID != id || !got.CreatedAt.Equal(before.CreatedAt) {
		t.Errorf("identity/history not preserved: id=%q created_at=%v want id=%q created_at=%v",
			got.ID, got.CreatedAt, id, before.CreatedAt)
	}

	// Omitting tags (nil) preserves the existing set.
	if err := d.updateMemory(ctx, updateArgs{ID: id, Content: "v3"}); err != nil {
		t.Fatalf("update without tags: %v", err)
	}
	got, err = d.st.Get(ctx, id)
	if err != nil {
		t.Fatalf("get after omit: %v", err)
	}
	if got.Content != "v3" {
		t.Errorf("content not updated on omit: %q", got.Content)
	}
	if !slices.Equal(got.Tags, newTags) {
		t.Errorf("omitting tags did not preserve existing: got %v want %v", got.Tags, newTags)
	}

	// Supplying an empty slice clears all tags — distinct from nil-omit. This
	// guards the boundary: if the store gated on len(*tags)>0 instead of
	// tags!=nil, clear would silently degrade to preserve.
	empty := []string{}
	if err := d.updateMemory(ctx, updateArgs{ID: id, Content: "v4", Tags: &empty}); err != nil {
		t.Fatalf("update with empty tags: %v", err)
	}
	got, err = d.st.Get(ctx, id)
	if err != nil {
		t.Fatalf("get after clear: %v", err)
	}
	if len(got.Tags) != 0 {
		t.Errorf("empty slice did not clear tags: got %v", got.Tags)
	}
}

// TestUpdateMemoryEmbedNotCalledForNonOwner verifies that updateMemory does NOT
// invoke the embedder when the caller does not own the record (cost-amplification
// hardening for eu8.4/eu8.2). A non-owner call must return ErrNotFound with zero
// embed calls. The happy path (owner == record owner) must embed exactly once.
// Integration: needs Qdrant.
func TestUpdateMemoryEmbedNotCalledForNonOwner(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "iso-test:project:upd-embed-gate"

	// Record stamped to "sub-owner"; anonymous caller (sub=="") is a non-owner.
	stampedID := "a7a7a7a7-0000-0000-0000-000000000001"
	stamped := store.Memory{
		ID: stampedID, Content: "original", Scope: scope,
		Owner: "sub-owner", Category: "convention",
		Source: "agent-inferred", CreatedAt: timeNow(),
	}
	if err := d.st.Upsert(ctx, stamped, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed stamped record: %v", err)
	}
	defer func() {
		cleanupErr(t, "Delete "+stampedID, d.st.Delete(ctx, stampedID, store.Authenticated("sub-owner")))
	}()

	// Swap in a counting embedder.
	counter := &countingEmbedder{}
	d.em = counter

	// Non-owner call (anonymous ctx → sub=="" ≠ "sub-owner") must fail without embedding.
	err := d.updateMemory(ctx, updateArgs{ID: stampedID, Content: "hijack"})
	if err == nil {
		t.Fatal("non-owner updateMemory: expected error, got nil")
	}
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("non-owner updateMemory: want ErrNotFound, got %v", err)
	}
	if counter.calls != 0 {
		t.Errorf("non-owner updateMemory: embed must not be called; got %d call(s)", counter.calls)
	}

	// Happy path: ownerless record (owner=="") is in the anonymous bucket and must
	// succeed with exactly one embed call.
	ownerlessID := "a7a7a7a7-0000-0000-0000-000000000002"
	ownerless := store.Memory{
		ID: ownerlessID, Content: "v1", Scope: scope,
		Owner: "", Category: "convention",
		Source: "agent-inferred", CreatedAt: timeNow(),
	}
	if err := d.st.Upsert(ctx, ownerless, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed ownerless record: %v", err)
	}
	defer func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) }()

	counter.calls = 0
	if err := d.updateMemory(ctx, updateArgs{ID: ownerlessID, Content: "v2"}); err != nil {
		t.Fatalf("ownerless updateMemory: unexpected error: %v", err)
	}
	if counter.calls != 1 {
		t.Errorf("ownerless updateMemory: expected exactly 1 embed call, got %d", counter.calls)
	}
}

// TestAuthedCrossActorSharedReadHandlers verifies the authenticated read grant
// at the handler boundary: caller B (a distinct verified sub) CAN read caller
// A's shared record through searchMemory/listMemory, but NOT A's private record.
// This regression-guards the ownerOrSharedCondition tightening from PR #54.
// Integration: needs Qdrant.
func TestAuthedCrossActorSharedReadHandlers(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "iso-test:project:authed-xactor"

	sharedID := "b0000000-0000-0000-0000-000000000001"
	privateID := "b0000000-0000-0000-0000-000000000002"

	// Seed caller A's shared + private records.
	shared := store.Memory{
		ID: sharedID, Content: "shared content", Scope: scope,
		Owner: "actor-A", Visibility: "shared", Category: "convention",
		Source: "agent-inferred", CreatedAt: timeNow(),
	}
	if err := d.st.Upsert(ctx, shared, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed shared: %v", err)
	}
	priv := store.Memory{
		ID: privateID, Content: "private content", Scope: scope,
		Owner: "actor-A", Visibility: "private", Category: "convention",
		Source: "agent-inferred", CreatedAt: timeNow(),
	}
	if err := d.st.Upsert(ctx, priv, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed private: %v", err)
	}
	defer func() {
		cleanupErr(t, "Delete "+sharedID, d.st.Delete(ctx, sharedID, store.Authenticated("actor-A")))
		cleanupErr(t, "Delete "+privateID, d.st.Delete(ctx, privateID, store.Authenticated("actor-A")))
	}()

	// Caller B: a distinct authenticated subject.
	bctx := authedContext(t, "actor-B")

	// searchMemory: B sees A's shared, not A's private.
	hits, err := d.searchMemory(bctx, searchArgs{Query: "content", Scope: scope, K: 10})
	if err != nil {
		t.Fatalf("searchMemory: %v", err)
	}
	assertVisibility(t, "searchMemory", hits, sharedID, privateID)

	// listMemory: same guarantee through the no-query handler path.
	mems, err := d.listMemory(bctx, listArgs{Scope: scope, Limit: 10})
	if err != nil {
		t.Fatalf("listMemory: %v", err)
	}
	assertVisibility(t, "listMemory", mems, sharedID, privateID)
}

// assertVisibility checks that wantID appears in got and denyID does not.
func assertVisibility(t *testing.T, where string, got []store.Memory, wantID, denyID string) {
	t.Helper()
	var sawWant, sawDeny bool
	for _, m := range got {
		switch m.ID {
		case wantID:
			sawWant = true
		case denyID:
			sawDeny = true
		}
	}
	if !sawWant {
		t.Errorf("%s: shared record not visible to authenticated cross-actor caller", where)
	}
	if sawDeny {
		t.Errorf("%s: private record of another actor must NOT be visible", where)
	}
}

// timeNow is a tiny indirection so the test reads cleanly; store records require
// a CreatedAt.
func timeNow() time.Time { return time.Now().UTC().Truncate(time.Second) }

func TestRegisterReturnsErrorOnStoreInitFailure(t *testing.T) {
	// Point Qdrant at an address that fails fast so StoreFromEnv errors and
	// Register surfaces it instead of calling log.Fatal.
	t.Setenv("ENGRAM_QDRANT_ADDR", "bad-host:1") // SplitHostPort ok, dial later fails
	t.Setenv("ENGRAM_EMBED_DIM", "not-a-number") // forces StoreFromEnv error pre-dial
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	tm := telemetry.NewToolMetrics(otel.Meter("test"))
	if err := Register(s, http.NewServeMux(), tm, nil); err == nil {
		t.Fatal("Register must return an error when store init fails, not exit")
	}
}

// TestSubjectFromContextNoToken pins the auth-disabled half of the contract: no
// token in context yields (Anonymous, nil), NOT an error — otherwise a no-issuer
// deployment would reject every request. The fail-closed path (a validated token
// lacking a non-empty sub → error) cannot be unit-tested here: the go-sdk stores
// TokenInfo under an unexported context key, so there is no way to inject a
// subject-less validated token. authedContext always supplies a non-empty sub,
// so it covers only the Authenticated arm — the fail-closed branch has no direct
// test, but a discarded extraction error still denies at the store default arm.
func TestSubjectFromContextNoToken(t *testing.T) {
	subj, err := subjectFromContext(context.Background())
	if err != nil {
		t.Fatalf("no token: unexpected error %v", err)
	}
	if subj == nil || subj.Owner() != "" {
		t.Errorf("no token: want non-nil Anonymous (Owner==\"\"), got %#v", subj)
	}
}

func TestListScheduledTool(t *testing.T) {
	d := testDeps(t)
	ctx := authedContext(t, "sub-A")
	// A far-future scheduled memory is hidden from normal recall but shows in list_scheduled.
	id, err := d.scheduleMemory(ctx, scheduleArgs{storeArgs: storeArgs{Content: "future",
		Scope: "ls:project:x", Source: "user-said", Category: "decision"},
		NotBefore: "2099-01-01T00:00:00Z"})
	if err != nil {
		t.Fatalf("schedule: %v", err)
	}
	t.Cleanup(func() { _ = d.st.Delete(context.Background(), id, store.Authenticated("sub-A")) })

	got, err := d.listScheduled(ctx, listScheduledArgs{Scope: "ls:project:x"}) // default state=scheduled
	if err != nil {
		t.Fatalf("list_scheduled: %v", err)
	}
	found := false
	for _, m := range got {
		if m.ID == id {
			found = true
		}
	}
	if !found {
		t.Error("list_scheduled (default scheduled) did not return the future memory")
	}
}

func TestStoreAndEmbedderFromEnvNoEnsureValidatesConfig(t *testing.T) {
	// A malformed (non-empty) data-plane value reaches Config.Validate via
	// StoreAndEmbedderFromEnvNoEnsure's loadAndValidate. Validation runs BEFORE any
	// Qdrant client construction, so this returns fast and needs no live Qdrant.
	t.Setenv("ENGRAM_OPENAI_BASE_URL", "ftp://nope")
	_, _, _, err := StoreAndEmbedderFromEnvNoEnsure()
	if err == nil {
		t.Fatal("StoreAndEmbedderFromEnvNoEnsure with bad ENGRAM_OPENAI_BASE_URL = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "invalid configuration") ||
		!strings.Contains(err.Error(), "ENGRAM_OPENAI_BASE_URL") {
		t.Errorf("error %q, want aggregated validation error naming ENGRAM_OPENAI_BASE_URL", err)
	}
}

// TestBuildDepsFromEnvLoadsConfigOnce pins the bead-635 invariant: wiring the
// store + embedder at startup must parse the env config exactly once, not once
// per dependency. The configLoad seam counts loads across the whole build.
func TestBuildDepsFromEnvLoadsConfigOnce(t *testing.T) {
	if testQdrantAddr == "" {
		t.Skip("no Qdrant available: set ENGRAM_QDRANT_TEST_ADDR or start Docker (testcontainers)")
	}
	// buildDepsFromEnv reads the data-plane config from the process env; point it
	// at the test Qdrant with a dedicated collection so EnsureCollection succeeds.
	t.Setenv("ENGRAM_QDRANT_ADDR", testQdrantAddr)
	t.Setenv("ENGRAM_QDRANT_COLLECTION", "mem_load_once_test")
	t.Setenv("ENGRAM_EMBED_DIM", "3")

	loads := 0
	orig := configLoad
	configLoad = func(flags *flag.FlagSet) (*config.Config, error) {
		loads++
		return orig(flags)
	}
	t.Cleanup(func() { configLoad = orig })

	if _, err := buildDepsFromEnv(); err != nil {
		t.Fatalf("buildDepsFromEnv: %v", err)
	}

	if loads != 1 {
		t.Errorf("buildDepsFromEnv loaded config %d times, want exactly 1", loads)
	}
}

// TestStoreAndEmbedderFromEnvNoEnsureLoadsConfigOnce pins the engram-mbnw
// single-load invariant: the reindex build path must parse the env config
// exactly once, not once per dependency (previously StoreFromEnvNoEnsure +
// EmbedderFromEnv each loaded). No live Qdrant is needed: storeFromConfig only
// constructs the client. qdrant.NewClient does fire a one-shot version
// HealthCheck at construction, but against the refused loopback port it
// fast-fails and is ignored, so the build still completes in milliseconds.
func TestStoreAndEmbedderFromEnvNoEnsureLoadsConfigOnce(t *testing.T) {
	t.Setenv("ENGRAM_QDRANT_ADDR", "localhost:6334")
	t.Setenv("ENGRAM_EMBED_DIM", "3")

	loads := 0
	orig := configLoad
	configLoad = func(flags *flag.FlagSet) (*config.Config, error) {
		loads++
		return orig(flags)
	}
	t.Cleanup(func() { configLoad = orig })

	st, dim, em, err := StoreAndEmbedderFromEnvNoEnsure()
	if err != nil {
		t.Fatalf("StoreAndEmbedderFromEnvNoEnsure: %v", err)
	}
	if st == nil || em == nil || dim != 3 {
		t.Fatalf("got store=%v dim=%d embedder=%v, want non-nil store/embedder and dim 3", st, dim, em)
	}

	if loads != 1 {
		t.Errorf("StoreAndEmbedderFromEnvNoEnsure loaded config %d times, want exactly 1", loads)
	}
}

func TestToMemorySetsClientSummarySource(t *testing.T) {
	withSummary := storeArgs{Content: "c", Scope: "s", Source: "user-said", Category: "decision", Summary: "terse"}
	m := withSummary.toMemory("owner", "actor", time.Now())
	if m.Summary != "terse" || m.SummarySource != "client" {
		t.Fatalf("client summary not mapped: summary=%q source=%q", m.Summary, m.SummarySource)
	}
	noSummary := storeArgs{Content: "c", Scope: "s", Source: "user-said", Category: "decision"}
	m2 := noSummary.toMemory("owner", "actor", time.Now())
	if m2.Summary != "" || m2.SummarySource != "" {
		t.Fatalf("absent summary must leave source empty: %+v", m2)
	}
}
