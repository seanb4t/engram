// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qdrant/go-client/qdrant"
	tcqdrant "github.com/testcontainers/testcontainers-go/modules/qdrant"
	"go.opentelemetry.io/otel"

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
}

// testQdrantAddr is the gRPC host:port the integration tests run against. Set by
// TestMain: MEM_QDRANT_TEST_ADDR if provided (fast path / override), else an
// ephemeral testcontainer. Empty when neither is available (Docker absent), in
// which case the integration tests skip.
var testQdrantAddr string

// TestMain provisions Qdrant for this package's integration tests. It prefers an
// existing instance via MEM_QDRANT_TEST_ADDR; otherwise it boots an ephemeral
// Qdrant via testcontainers and tears it down afterward. If neither is available
// the suite still runs — the integration tests skip with a clear message.
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

// fakeEmbedder returns a fixed vector so handler tests don't need a live embedder.
type fakeEmbedder struct{}

func (fakeEmbedder) Embed(context.Context, string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

// testDeps builds a deps backed by a live Qdrant (skip-gated, same posture as the
// store integration tests) and the fake embedder. deps.em is an interface so the
// embedder is fakeable; deps.st is concrete, hence the Qdrant gate.
func testDeps(t *testing.T) *deps {
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
	st := store.New(c, "mem_eval_test")
	if err := st.EnsureCollection(context.Background(), 3); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	return &deps{st: st, em: fakeEmbedder{}}
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

// TestOwnerFromContextNoToken pins the auth-disabled half of the contract: no
// token in context is the anonymous bucket ("", nil), NOT an error — otherwise a
// no-issuer deployment would reject every request. The fail-closed path (a
// validated token lacking a non-empty sub → error) cannot be unit-tested here
// because the go-sdk stores TokenInfo under an unexported context key, so there
// is no way to inject one (same constraint as the deferred handler-isolation gap).
func TestOwnerFromContextNoToken(t *testing.T) {
	sub, err := ownerFromContext(context.Background())
	if sub != "" || err != nil {
		t.Errorf("no token: want (\"\", nil), got (%q, %v)", sub, err)
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
	defer func() { _ = d.st.DeleteAll(ctx, scope, "") }()

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

func TestUpdateMemoryPreservesSharingHandler(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "iso-test:project:handler-upd"
	id := "e5e5e5e5-0000-0000-0000-000000000001"
	defer func() { _ = d.st.DeleteAll(ctx, scope, "") }()

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

// timeNow is a tiny indirection so the test reads cleanly; store records require
// a CreatedAt.
func timeNow() time.Time { return time.Now().UTC().Truncate(time.Second) }

func TestRegisterReturnsErrorOnStoreInitFailure(t *testing.T) {
	// Point Qdrant at an address that fails fast so StoreFromEnv errors and
	// Register surfaces it instead of calling log.Fatal.
	t.Setenv("MEM_QDRANT_ADDR", "bad-host:1") // SplitHostPort ok, dial later fails
	t.Setenv("MEM_EMBED_DIM", "not-a-number") // forces StoreFromEnv error pre-dial
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	tm := telemetry.NewToolMetrics(otel.Meter("test"))
	if err := Register(s, tm); err == nil {
		t.Fatal("Register must return an error when store init fails, not exit")
	}
}
