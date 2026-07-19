// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/qdrant/go-client/qdrant"
	flag "github.com/spf13/pflag"
	tcqdrant "github.com/testcontainers/testcontainers-go/modules/qdrant"
	"go.opentelemetry.io/otel"

	"github.com/seanb4t/engram/internal/config"
	"github.com/seanb4t/engram/internal/shortid"
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
	check("store_rule", func(s *mcp.Server) {
		mcp.AddTool(s, &mcp.Tool{Name: "store_rule", Description: "x"}, func(context.Context, *mcp.CallToolRequest, storeRuleArgs) (*mcp.CallToolResult, any, error) {
			return noop()
		})
	})
	check("list_rules", func(s *mcp.Server) {
		mcp.AddTool(s, &mcp.Tool{Name: "list_rules", Description: "x"}, func(context.Context, *mcp.CallToolRequest, listRulesArgs) (*mcp.CallToolResult, any, error) {
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
// which case the integration tests skip — unless ENGRAM_REQUIRE_QDRANT is set,
// in which case they fail (see requireQdrant).
var testQdrantAddr string

// requireQdrant is the SOLE place ENGRAM_REQUIRE_QDRANT is read/parsed
// (round-6 MED, round-7 LOW + round-8 LOW, Codex): TestMain and
// failOrSkipNoQdrant act only on its result, never parsing the env var
// themselves. Unset/empty -> (false, nil): local dev ergonomics unchanged
// (integration tests still skip without Qdrant). A truthy/falsey value
// parses via strconv.ParseBool. Any NON-EMPTY INVALID value (a CI typo like
// "treu") returns a NON-NIL error rather than being coerced to false
// (round-8 LOW) — coercing a parse error to false would silently re-enable
// skipping and defeat the fail-closed gate the CI `test` job relies on.
func requireQdrant() (bool, error) {
	v := os.Getenv("ENGRAM_REQUIRE_QDRANT")
	if v == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("ENGRAM_REQUIRE_QDRANT: invalid value %q: %w", v, err)
	}
	return b, nil
}

// failOrSkipNoQdrant is the shared "no Qdrant available" handler for every
// per-test integration call site (testDepsWithStore and the sibling
// TestBuildDepsFromEnvLoadsConfigOnce gate). Under ENGRAM_REQUIRE_QDRANT it
// t.Fatal's (fail-closed, round-6 MED) instead of skipping, so CI cannot go
// green with the real-store authz/parity gate silently skipped; otherwise it
// preserves today's t.Skip (local dev ergonomics unchanged).
func failOrSkipNoQdrant(t *testing.T) {
	t.Helper()
	required, err := requireQdrant()
	if err != nil {
		t.Fatalf("%v", err)
	}
	if required {
		t.Fatal("no Qdrant available and ENGRAM_REQUIRE_QDRANT is set: failing instead of skipping (round-6 MED)")
	}
	t.Skip("no Qdrant available: set ENGRAM_QDRANT_TEST_ADDR or start Docker (testcontainers)")
}

// TestMain provisions Qdrant for this package's integration tests. It prefers an
// existing instance via ENGRAM_QDRANT_TEST_ADDR; otherwise it boots an ephemeral
// Qdrant via testcontainers and tears it down afterward. If neither is available
// the suite still runs and the integration tests skip with a clear message —
// UNLESS ENGRAM_REQUIRE_QDRANT is set (round-6 MED, Codex: the CI `test` job
// sets it), in which case TestMain exits non-zero instead of letting the
// suite run with the real-store authz gate silently skipped.
func TestMain(m *testing.M) {
	required, err := requireQdrant()
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
	if addr := os.Getenv("ENGRAM_QDRANT_TEST_ADDR"); addr != "" {
		testQdrantAddr = addr
		os.Exit(m.Run())
	}
	// Bound startup so an unreachable daemon or a stalled image pull fails fast
	// instead of hanging the suite. os.Exit skips defers, so cancel explicitly.
	startCtx, startCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	container, cerr := tcqdrant.Run(startCtx, "qdrant/qdrant:v1.18.2")
	if cerr != nil {
		startCancel()
		fmt.Fprintf(os.Stderr, "qdrant testcontainer unavailable (%v); integration tests will skip — set ENGRAM_QDRANT_TEST_ADDR or start Docker\n", cerr)
		if required {
			fmt.Fprintln(os.Stderr, "fatal: ENGRAM_REQUIRE_QDRANT is set — failing instead of skipping (round-6 MED)")
			os.Exit(1)
		}
		os.Exit(m.Run())
	}
	testQdrantAddr, cerr = container.GRPCEndpoint(startCtx)
	startCancel()
	if cerr != nil {
		terminateQdrant(container)
		fmt.Fprintf(os.Stderr, "qdrant grpc endpoint: %v\n", cerr)
		os.Exit(1)
	}
	if required && testQdrantAddr == "" {
		terminateQdrant(container)
		fmt.Fprintln(os.Stderr, "fatal: ENGRAM_REQUIRE_QDRANT is set but no Qdrant address resolved")
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

// TestRequireQdrant pins requireQdrant's parse contract (round-7 LOW + round-8
// LOW, Codex): unset/empty and recognized truthy/falsey values parse cleanly,
// and — the round-8 fix — a malformed value (a CI typo like "treu") returns a
// NON-NIL error rather than being silently coerced to false, which would
// re-enable skipping and defeat the fail-closed gate. Driven entirely via
// t.Setenv; needs no Qdrant.
func TestRequireQdrant(t *testing.T) {
	cases := []struct {
		name    string
		val     string
		want    bool
		wantErr bool
	}{
		{name: "unset_or_empty", val: "", want: false},
		{name: "truthy_true", val: "true", want: true},
		{name: "truthy_1", val: "1", want: true},
		{name: "falsey_false", val: "false", want: false},
		{name: "falsey_0", val: "0", want: false},
		{name: "malformed", val: "treu", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ENGRAM_REQUIRE_QDRANT", tc.val)
			got, err := requireQdrant()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("requireQdrant() with %q = (%v, nil), want a non-nil error (must not coerce to false)", tc.val, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("requireQdrant() with %q: unexpected error: %v", tc.val, err)
			}
			if got != tc.want {
				t.Errorf("requireQdrant() with %q = %v, want %v", tc.val, got, tc.want)
			}
		})
	}
}

// fakeEmbedder returns a fixed vector so handler tests don't need a live embedder.
type fakeEmbedder struct{}

func (fakeEmbedder) Embed(context.Context, string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

func (fakeEmbedder) EmbedQuery(context.Context, string) ([]float32, error) {
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

func (e *countingEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	e.calls++
	return fakeEmbedder{}.Embed(ctx, text)
}

// testDeps builds a deps backed by a live Qdrant (skip-gated, same posture as the
// store integration tests) and the fake embedder. deps.em is an interface so the
// embedder is fakeable; deps.st is the memStore interface backed by a concrete
// *store.Store, hence the Qdrant gate. Delegates to testDepsWithStore and
// discards the concrete store — existing `d := testDeps(t)` call sites are
// unaffected by the deps.st retype (review round-2 BLOCKER 1).
func testDeps(t *testing.T) *deps {
	t.Helper()
	d, _ := testDepsWithStore(t)
	return d
}

// testDepsWithStore builds the same Qdrant-backed deps as testDeps AND returns
// the concrete *store.Store it wraps, for the handful of call sites that need
// a concrete store (storeFill/buildUsageQueue take *store.Store, not the
// narrower memStore deps.st now is — review round-2 BLOCKER 1).
func testDepsWithStore(t *testing.T) (*deps, *store.Store) {
	t.Helper()
	if testQdrantAddr == "" {
		failOrSkipNoQdrant(t)
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
	return &deps{st: st, em: fakeEmbedder{}}, st
}

// testDepsWithSummaryQueue builds the same Qdrant-backed deps as testDeps plus
// a live, test-controlled *summaryQueue (workers/queueSize/fill all injected),
// wired onto deps.summaryQueue so enqueue-on-success tests can drive the queue
// through the real storeMemory/scheduleMemory/storeDiscovery/storeRule
// handlers without a live summarizer. The queue is shut down via t.Cleanup.
func testDepsWithSummaryQueue(t *testing.T, workers, queueSize int, fill func(ctx context.Context, id string) error) *deps {
	t.Helper()
	d := testDeps(t)
	q := newSummaryQueue(workers, queueSize, 5*time.Second, nil, fill)
	q.Start(context.Background())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		q.Shutdown(ctx)
	})
	d.summaryQueue = q
	return d
}

// usageQueueRecorder is a test-injectable fill that records every id it
// receives (thread-safe), used to assert D-01/D-02 enqueue behavior without a
// live Qdrant IncrementAccess call.
type usageQueueRecorder struct {
	mu  sync.Mutex
	ids []string
}

func (r *usageQueueRecorder) fill(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ids = append(r.ids, id)
	return nil
}

func (r *usageQueueRecorder) recorded() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.ids)
}

// testDepsWithUsageQueue builds the same Qdrant-backed deps as testDeps plus a
// live, test-controlled *usageQueue wired onto deps.usageQueue and a
// *usageQueueRecorder capturing every id the queue's fill receives, so
// D-01/D-02 enqueue-on-success (and enqueue-never) tests can drive the queue
// through the real getMemory/Connect GetMemory handlers. The queue is shut
// down via t.Cleanup.
func testDepsWithUsageQueue(t *testing.T, workers, queueSize int) (*deps, *usageQueueRecorder) {
	t.Helper()
	d := testDeps(t)
	rec := &usageQueueRecorder{}
	q := newUsageQueue(workers, queueSize, nil, rec.fill)
	q.Start(context.Background())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		q.Shutdown(ctx)
	})
	d.usageQueue = q
	return d, rec
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
			Extra:      map[string]any{"owner_claim": sub},
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

// strp returns a pointer to s — a small literal-construction helper for the
// presence-signaled updateArgs.Content field (landmine 2), used across test
// call sites that need a non-nil *string content literal.
func strp(s string) *string { return &s }

// callerFor resolves the caller for a context carrying (or not carrying)
// TokenInfo exactly as callerFromContext does, failing the test on an
// unexpected error. The write-method test call sites this task retyped use
// this helper to keep passing a single ctx while supplying the new explicit
// caller argument.
func callerFor(ctx context.Context, t *testing.T) caller {
	t.Helper()
	c, err := callerFromContext(ctx)
	if err != nil {
		t.Fatalf("callerFromContext: %v", err)
	}
	return c
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
	if _, _, err := d.scheduleMemory(ctx, callerFor(ctx, t), a); err == nil {
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
	id, _, err := d.storeMemory(ctx, callerFor(ctx, t), storeArgs{Content: "x", Scope: "clock:project:store",
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
	if _, _, err := d.scheduleMemory(ctx, callerFor(ctx, t), base); err == nil {
		t.Error("missing window: want error, got nil")
	}
	// not_after already in the past -> rejected.
	past := base
	past.NotAfter = "2000-01-01T00:00:00Z"
	_, _, err := d.scheduleMemory(ctx, callerFor(ctx, t), past)
	if err == nil {
		t.Error("past not_after: want error, got nil")
	}
	// finding 5: parseWindow's rejections are wrapped with the existing
	// store.ErrInvalidArgument so a Connect ScheduleMemory call maps to
	// CodeInvalidArgument, not CodeInternal.
	if !errors.Is(err, store.ErrInvalidArgument) {
		t.Errorf("past not_after: want store.ErrInvalidArgument, got %v", err)
	}
	// Inverted window (not_before >= not_after) -> rejected.
	inv := base
	inv.NotBefore = "2031-01-01T00:00:00Z"
	inv.NotAfter = "2030-01-01T00:00:00Z"
	if _, _, err := d.scheduleMemory(ctx, callerFor(ctx, t), inv); err == nil {
		t.Error("inverted window: want error, got nil")
	}
	// discovery category -> rejected.
	disc := base
	disc.Category = "discovery"
	disc.NotBefore = "2030-01-01T00:00:00Z"
	if _, _, err := d.scheduleMemory(ctx, callerFor(ctx, t), disc); err == nil {
		t.Error("discovery category: want error, got nil")
	}
	// Valid future-scheduled memory -> stored, hidden from normal recall.
	ok := base
	ok.NotBefore = "2030-01-01T00:00:00Z"
	id, _, err := d.scheduleMemory(ctx, callerFor(ctx, t), ok)
	if err != nil {
		t.Fatalf("valid schedule: %v", err)
	}
	t.Cleanup(func() { _ = d.st.Delete(context.Background(), id, store.Authenticated("sub-A")) })
	hits, _ := d.listMemory(ctx, callerFor(ctx, t), coreListRequest{Scope: "sched:project:x"})
	for _, m := range hits.Memories {
		if m.ID == id {
			t.Error("future-scheduled memory leaked into normal list_memory")
		}
	}

	// A past not_before-only window is accepted (already revealed) and the record
	// is immediately active, so it surfaces through normal list_memory.
	activeNow := base
	activeNow.NotBefore = "2000-01-01T00:00:00Z"
	activeID, _, err := d.scheduleMemory(ctx, callerFor(ctx, t), activeNow)
	if err != nil {
		t.Fatalf("past not_before should be accepted (immediately active): %v", err)
	}
	t.Cleanup(func() { _ = d.st.Delete(context.Background(), activeID, store.Authenticated("sub-A")) })
	active, _ := d.listMemory(ctx, callerFor(ctx, t), coreListRequest{Scope: "sched:project:x"})
	found := false
	for _, m := range active.Memories {
		if m.ID == activeID {
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

	id, _, err := d.storeMemory(ctx, callerFor(ctx, t), storeArgs{
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

// TestStoreMemoryStampsEmbedderIdentityHandler is a Task 4 positive
// persistence guard: a missed d.embedderIdentity assignment in storeMemory
// would compile and pass every other test, so this re-reads the persisted
// record via d.st.Get and asserts the sentinel identity round-tripped.
func TestStoreMemoryStampsEmbedderIdentityHandler(t *testing.T) {
	d := testDeps(t)
	d.embedderIdentity = "v1:deadbeefdeadbeef"
	ctx := authedContext(t, "sub-identity-store")
	scope := "iso-test:project:identity-store"
	defer func() {
		cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Authenticated("sub-identity-store")))
	}()

	id, _, err := d.storeMemory(ctx, callerFor(ctx, t), storeArgs{Content: "identity check", Scope: scope, Source: "user-said", Category: "gotcha"})
	if err != nil {
		t.Fatalf("storeMemory: %v", err)
	}
	got, err := d.st.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after storeMemory: %v", err)
	}
	if got.EmbedderIdentity != "v1:deadbeefdeadbeef" {
		t.Errorf("storeMemory did not stamp embedder identity: got %q, want %q", got.EmbedderIdentity, "v1:deadbeefdeadbeef")
	}
}

// TestScheduleMemoryStampsEmbedderIdentityHandler mirrors
// TestStoreMemoryStampsEmbedderIdentityHandler for scheduleMemory.
func TestScheduleMemoryStampsEmbedderIdentityHandler(t *testing.T) {
	d := testDeps(t)
	d.embedderIdentity = "v1:deadbeefdeadbeef"
	ctx := authedContext(t, "sub-identity-schedule")
	scope := "iso-test:project:identity-schedule"
	defer func() {
		cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Authenticated("sub-identity-schedule")))
	}()

	id, _, err := d.scheduleMemory(ctx, callerFor(ctx, t), scheduleArgs{
		storeArgs: storeArgs{Content: "identity check", Scope: scope, Source: "user-said", Category: "decision"},
		NotAfter:  timeNow().Add(time.Hour).Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("scheduleMemory: %v", err)
	}
	got, err := d.st.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after scheduleMemory: %v", err)
	}
	if got.EmbedderIdentity != "v1:deadbeefdeadbeef" {
		t.Errorf("scheduleMemory did not stamp embedder identity: got %q, want %q", got.EmbedderIdentity, "v1:deadbeefdeadbeef")
	}
}

// TestStoreDiscoveryStampsEmbedderIdentityHandler mirrors
// TestStoreMemoryStampsEmbedderIdentityHandler for storeDiscovery.
func TestStoreDiscoveryStampsEmbedderIdentityHandler(t *testing.T) {
	d := testDeps(t)
	d.embedderIdentity = "v1:deadbeefdeadbeef"
	ctx := context.Background()
	scope := "discovery:repo:identity-discovery"
	defer func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) }()

	id, _, err := d.storeDiscovery(ctx, callerFor(ctx, t), storeDiscoveryArgs{
		Content: "identity check discovery", Kind: "fact", Scope: scope,
		Citations: []citationArg{{Kind: "file", Ref: "f.go"}},
	})
	if err != nil {
		t.Fatalf("storeDiscovery: %v", err)
	}
	got, err := d.st.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after storeDiscovery: %v", err)
	}
	if got.EmbedderIdentity != "v1:deadbeefdeadbeef" {
		t.Errorf("storeDiscovery did not stamp embedder identity: got %q, want %q", got.EmbedderIdentity, "v1:deadbeefdeadbeef")
	}
}

// TestUpdateMemoryReStampsEmbedderIdentityHandler proves the re-embed path
// (updateMemory -> Store.Update -> Upsert) RE-stamps the identity, not only
// the initial write: the seed record carries a stale identity, and after
// updateMemory runs with a new d.embedderIdentity, the persisted record must
// carry the NEW value.
func TestUpdateMemoryReStampsEmbedderIdentityHandler(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "iso-test:project:identity-update"
	id := "e7e7e7e7-0000-0000-0000-000000000001"
	defer func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) }()

	seed := store.Memory{
		ID: id, Content: "v1", Scope: scope, Owner: "", CreatedAt: timeNow(),
		EmbedderIdentity: "v1:stalestalestale0",
	}
	if err := d.st.Upsert(ctx, seed, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	d.embedderIdentity = "v1:deadbeefdeadbeef"
	if _, err := d.updateMemory(ctx, callerFor(ctx, t), updateArgs{ID: id, Content: strp("v2")}); err != nil {
		t.Fatalf("updateMemory: %v", err)
	}
	got, err := d.st.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after updateMemory: %v", err)
	}
	if got.EmbedderIdentity != "v1:deadbeefdeadbeef" {
		t.Errorf("updateMemory did not re-stamp embedder identity on re-embed: got %q, want %q", got.EmbedderIdentity, "v1:deadbeefdeadbeef")
	}
}

// TestStoreMemoryMintsAndReturnsShortID pins that storeMemory mints a short_id
// (after embed) and both returns it and persists it on the record.
func TestStoreMemoryMintsAndReturnsShortID(t *testing.T) {
	d := testDeps(t)
	ctx := authedContext(t, "owner-A")
	id, sid, err := d.storeMemory(ctx, callerFor(ctx, t), storeArgs{Content: "hello", Scope: "s", Category: "gotcha", Source: "user-said"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sid) != shortid.Length {
		t.Fatalf("short id %q", sid)
	}
	got, err := d.st.Get(context.Background(), id)
	if err != nil || got.ShortID != sid {
		t.Fatalf("persisted short id %q != returned %q (err %v)", got.ShortID, sid, err)
	}
}

// TestStoreMemoryNoKeyAlwaysFresh pins SC5: omitting idempotency_key preserves
// today's behavior byte-for-byte — two keyless calls with identical content
// each mint a fresh, DISTINCT random uuid.NewString() id. Adding the
// IdempotencyKey field must not perturb this.
func TestStoreMemoryNoKeyAlwaysFresh(t *testing.T) {
	d := testDeps(t)
	ctx := authedContext(t, "owner-nokey")
	scope := "iso-test:project:idempotency-nokey"
	defer func() {
		cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(context.Background(), scope, store.Authenticated("owner-nokey")))
	}()

	args := storeArgs{Content: "no key here", Scope: scope, Source: "user-said", Category: "gotcha"}
	id1, _, err := d.storeMemory(ctx, callerFor(ctx, t), args)
	if err != nil {
		t.Fatalf("storeMemory (first): %v", err)
	}
	id2, _, err := d.storeMemory(ctx, callerFor(ctx, t), args)
	if err != nil {
		t.Fatalf("storeMemory (second): %v", err)
	}
	if id1 == id2 {
		t.Fatalf("two keyless store_memory calls with identical content minted the SAME id %q; want distinct fresh ids (SC5)", id1)
	}
}

// TestStoreMemoryIdempotencyKeyTooLarge pins IN-01: an oversized
// idempotency_key is rejected before it is ever hashed into the point ID or
// touches the store — no Qdrant/embedder round trip needed, consistent with
// the size-bound discipline storeDiscoveryArgs already enforces for other
// client-supplied strings in this file.
func TestStoreMemoryIdempotencyKeyTooLarge(t *testing.T) {
	d := &deps{}
	ctx := authedContext(t, "sub-A")
	a := storeArgs{
		Content: "x", Scope: "cap:project:x", Source: "user-said", Category: "decision",
		IdempotencyKey: strings.Repeat("k", maxIdempotencyKeyBytes+1),
	}
	_, _, err := d.storeMemory(ctx, callerFor(ctx, t), a)
	if err == nil {
		t.Fatal("oversized idempotency_key should be rejected, got nil error")
	}
	if !errors.Is(err, store.ErrInvalidArgument) {
		t.Fatalf("error = %v, want wrapping store.ErrInvalidArgument", err)
	}
}

// TestStoreMemoryIdempotentReplayReturnsOriginal pins SC1: a keyed store_memory
// call, repeated with identical content, returns the ORIGINAL (id, short_id)
// unchanged — no duplicate point, and zero side-effects (no second Embed).
func TestStoreMemoryIdempotentReplayReturnsOriginal(t *testing.T) {
	d, st := testDepsWithStore(t)
	spy := &countingEmbedder{}
	d.em = spy
	ctx := authedContext(t, "owner-replay")
	scope := "iso-test:project:idempotency-replay"
	defer func() {
		cleanupErr(t, "DeleteAll "+scope, st.DeleteAll(context.Background(), scope, store.Authenticated("owner-replay")))
	}()

	args := storeArgs{
		Content: "replay me", Scope: scope, Source: "user-said", Category: "gotcha",
		IdempotencyKey: "key-1",
	}
	id1, sid1, err := d.storeMemory(ctx, callerFor(ctx, t), args)
	if err != nil {
		t.Fatalf("storeMemory (first): %v", err)
	}
	if spy.calls != 1 {
		t.Fatalf("first call: embed calls = %d, want 1", spy.calls)
	}

	id2, sid2, err := d.storeMemory(ctx, callerFor(ctx, t), args)
	if err != nil {
		t.Fatalf("storeMemory (replay): %v", err)
	}
	if id2 != id1 || sid2 != sid1 {
		t.Fatalf("replay returned a different record: (id=%q,sid=%q), want (id=%q,sid=%q)", id2, sid2, id1, sid1)
	}
	if spy.calls != 1 {
		t.Fatalf("replay triggered an embed call: embed calls = %d, want still 1 (zero side-effects, SC1)", spy.calls)
	}

	_, total, _, err := st.List(ctx, scope, store.Authenticated("owner-replay"), store.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 {
		t.Fatalf("store holds %d points for scope %q, want exactly 1 (no duplicate, SC1)", total, scope)
	}
}

// TestStoreMemoryIdempotentReplayRejectsMismatch pins SC2: same key + same
// owner + different content is rejected with store.ErrIdempotencyConflict
// (errors.Is true) — never a silent overwrite, never a 404 — and the original
// record is left unchanged.
func TestStoreMemoryIdempotentReplayRejectsMismatch(t *testing.T) {
	d, st := testDepsWithStore(t)
	ctx := authedContext(t, "owner-mismatch")
	scope := "iso-test:project:idempotency-mismatch"
	defer func() {
		cleanupErr(t, "DeleteAll "+scope, st.DeleteAll(context.Background(), scope, store.Authenticated("owner-mismatch")))
	}()

	first := storeArgs{
		Content: "original content", Scope: scope, Source: "user-said", Category: "gotcha",
		IdempotencyKey: "key-mismatch",
	}
	id1, _, err := d.storeMemory(ctx, callerFor(ctx, t), first)
	if err != nil {
		t.Fatalf("storeMemory (first): %v", err)
	}

	second := first
	second.Content = "DIFFERENT content"
	_, _, err = d.storeMemory(ctx, callerFor(ctx, t), second)
	if !errors.Is(err, store.ErrIdempotencyConflict) {
		t.Fatalf("mismatch: want errors.Is(err, store.ErrIdempotencyConflict), got %v", err)
	}

	got, gerr := st.Get(ctx, id1)
	if gerr != nil {
		t.Fatalf("Get after mismatch: %v", gerr)
	}
	if got.Content != "original content" {
		t.Fatalf("original record content was mutated by the rejected replay: got %q, want %q", got.Content, "original content")
	}
}

// TestStoreMemoryIdempotentKeyScopedPerOwner pins SC3 (Pitfall 2 matrix): two
// distinct owners using the IDENTICAL idempotency_key value and identical
// content get two independent records — owner is baked into the deterministic
// point-ID hash, so cross-owner collision is structurally impossible, not
// filter-enforced (D-09).
func TestStoreMemoryIdempotentKeyScopedPerOwner(t *testing.T) {
	d, st := testDepsWithStore(t)
	scope := "iso-test:project:idempotency-cross-owner"
	ctxA := authedContext(t, "owner-idem-A")
	ctxB := authedContext(t, "owner-idem-B")
	defer func() {
		cleanupErr(t, "DeleteAll A "+scope, st.DeleteAll(context.Background(), scope, store.Authenticated("owner-idem-A")))
		cleanupErr(t, "DeleteAll B "+scope, st.DeleteAll(context.Background(), scope, store.Authenticated("owner-idem-B")))
	}()

	args := storeArgs{
		Content: "shared key content", Scope: scope, Source: "user-said", Category: "gotcha",
		IdempotencyKey: "same-key-both-owners",
	}
	idA, _, err := d.storeMemory(ctxA, callerFor(ctxA, t), args)
	if err != nil {
		t.Fatalf("storeMemory (owner A): %v", err)
	}
	idB, _, err := d.storeMemory(ctxB, callerFor(ctxB, t), args)
	if err != nil {
		t.Fatalf("storeMemory (owner B): %v", err)
	}
	if idA == idB {
		t.Fatalf("two owners with the identical idempotency_key collided on id %q; want structurally distinct ids (SC3)", idA)
	}

	gotA, err := st.Get(context.Background(), idA)
	if err != nil || gotA.Owner != "owner-idem-A" {
		t.Fatalf("owner A record: owner=%q err=%v", gotA.Owner, err)
	}
	gotB, err := st.Get(context.Background(), idB)
	if err != nil || gotB.Owner != "owner-idem-B" {
		t.Fatalf("owner B record: owner=%q err=%v", gotB.Owner, err)
	}

	itemsA, _, _, err := st.List(ctxA, scope, store.Authenticated("owner-idem-A"), store.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("List (owner A): %v", err)
	}
	for _, m := range itemsA {
		if m.ID == idB {
			t.Fatalf("owner A's List leaked owner B's record %q", idB)
		}
	}
}

// TestStoreMemoryIdempotentConcurrentIdenticalOnePoint pins SC4 (Pitfall 3): N
// concurrent identical (same key + same content) store_memory calls resolve to
// exactly one Qdrant point — the deterministic-ID Upsert is the sole isolation
// primitive, no application lock, no search-then-insert TOCTOU check. Must run
// under `go test -race`. Per D-12, this asserts ONLY the no-duplicate
// invariant, never reject-under-simultaneous-mismatch.
func TestStoreMemoryIdempotentConcurrentIdenticalOnePoint(t *testing.T) {
	d, st := testDepsWithStore(t)
	ctx := authedContext(t, "owner-concurrent")
	scope := "iso-test:project:idempotency-concurrent"
	defer func() {
		cleanupErr(t, "DeleteAll "+scope, st.DeleteAll(context.Background(), scope, store.Authenticated("owner-concurrent")))
	}()
	c := callerFor(ctx, t)

	args := storeArgs{
		Content: "concurrent identical", Scope: scope, Source: "user-said", Category: "gotcha",
		IdempotencyKey: "concurrent-key",
	}

	const n = 20
	type result struct {
		id, shortID string
		err         error
	}
	results := make(chan result, n)
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, sid, err := d.storeMemory(ctx, c, args)
			results <- result{id, sid, err}
		}()
	}
	wg.Wait()
	close(results)

	var firstID string
	for r := range results {
		if r.err != nil {
			t.Fatalf("storeMemory (concurrent): %v", r.err)
		}
		if firstID == "" {
			firstID = r.id
		} else if r.id != firstID {
			t.Fatalf("concurrent identical keyed calls minted DIFFERENT ids: %q vs %q (SC4 no-duplicate invariant violated)", firstID, r.id)
		}
	}

	_, total, _, err := st.List(ctx, scope, store.Authenticated("owner-concurrent"), store.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 {
		t.Fatalf("store holds %d points after %d concurrent identical keyed calls, want exactly 1 (SC4)", total, n)
	}
}

// TestStoreMemoryIdempotentConcurrentMismatchConvergesOnePoint pins the D-12
// honest-concurrency boundary (fovea review, PR #404): under true simultaneity,
// N keyed store_memory calls that share one idempotency key but carry DIFFERENT
// content converge to exactly one Qdrant point — no duplicate, no corruption.
// The mismatch reject is best-effort: a racer that observes the already-written
// point rejects with store.ErrIdempotencyConflict, while racers that pass the
// pre-write existence check before any Upsert land as last-writer-wins. The
// safety invariant (one point, intact content) always holds. This is the
// deliberately-accepted boundary documented as R-24-02 in 24-SECURITY.md; a
// deterministic "reject always fires" guarantee would require per-point
// locking/singleflight we intentionally did not add.
func TestStoreMemoryIdempotentConcurrentMismatchConvergesOnePoint(t *testing.T) {
	d, st := testDepsWithStore(t)
	ctx := authedContext(t, "owner-mismatch")
	scope := "iso-test:project:idempotency-mismatch"
	defer func() {
		cleanupErr(t, "DeleteAll "+scope, st.DeleteAll(context.Background(), scope, store.Authenticated("owner-mismatch")))
	}()
	c := callerFor(ctx, t)

	const n = 20
	type result struct {
		id  string
		err error
	}
	results := make(chan result, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Same key, DISTINCT content per racer — every fingerprint differs.
			args := storeArgs{
				Content: fmt.Sprintf("mismatch content %d", i), Scope: scope,
				Source: "user-said", Category: "gotcha", IdempotencyKey: "mismatch-key",
			}
			id, _, err := d.storeMemory(ctx, c, args)
			results <- result{id, err}
		}(i)
	}
	wg.Wait()
	close(results)

	var winnerID string
	successes := 0
	for r := range results {
		switch {
		case r.err == nil:
			successes++
			// All racers share (owner, scope, key) → one deterministic point ID.
			if winnerID == "" {
				winnerID = r.id
			} else if r.id != winnerID {
				t.Fatalf("successful concurrent mismatch calls returned DIFFERENT ids: %q vs %q", winnerID, r.id)
			}
		case errors.Is(r.err, store.ErrIdempotencyConflict):
			// Best-effort reject — acceptable under D-12.
		default:
			t.Fatalf("storeMemory (concurrent mismatch): unexpected error: %v", r.err)
		}
	}
	if successes == 0 {
		t.Fatal("no concurrent mismatch call succeeded — expected at least one last-writer-wins winner")
	}

	// Safety invariant: exactly one point, content intact (one of the submitted
	// values, never a partial/corrupt/merged write).
	items, total, _, err := st.List(ctx, scope, store.Authenticated("owner-mismatch"), store.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("store holds %d points (len=%d) after %d concurrent mismatch keyed calls, want exactly 1 (D-12 converge-to-one)", total, len(items), n)
	}
	if got := items[0].Content; !strings.HasPrefix(got, "mismatch content ") {
		t.Fatalf("stored content %q is not one of the submitted values — possible corruption (D-12 last-writer-wins expected)", got)
	}
}

// TestScheduleMemoryIdempotentIgnoresWindowChange documents and pins the
// conscious resolution of RESEARCH Open Question 1: the schedule window
// (not_before/not_after) is EXCLUDED from the D-07 content fingerprint. A
// replay with the same key + identical storeArgs content but a CHANGED window
// returns the original record with its ORIGINAL window unchanged — this is an
// intentional, tested decision, not an oversight.
func TestScheduleMemoryIdempotentIgnoresWindowChange(t *testing.T) {
	d, st := testDepsWithStore(t)
	ctx := authedContext(t, "owner-window")
	scope := "iso-test:project:idempotency-window"
	defer func() {
		cleanupErr(t, "DeleteAll "+scope, st.DeleteAll(context.Background(), scope, store.Authenticated("owner-window")))
	}()

	base := storeArgs{
		Content: "windowed content", Scope: scope, Source: "user-said", Category: "gotcha",
		IdempotencyKey: "window-key",
	}
	firstNotAfter := timeNow().Add(1 * time.Hour).Format(time.RFC3339)
	id1, sid1, err := d.scheduleMemory(ctx, callerFor(ctx, t), scheduleArgs{storeArgs: base, NotAfter: firstNotAfter})
	if err != nil {
		t.Fatalf("scheduleMemory (first): %v", err)
	}

	secondNotAfter := timeNow().Add(48 * time.Hour).Format(time.RFC3339)
	id2, sid2, err := d.scheduleMemory(ctx, callerFor(ctx, t), scheduleArgs{storeArgs: base, NotAfter: secondNotAfter})
	if err != nil {
		t.Fatalf("scheduleMemory (replay with different window): %v", err)
	}
	if id2 != id1 || sid2 != sid1 {
		t.Fatalf("replay with a changed window minted a different record: (id=%q,sid=%q), want (id=%q,sid=%q)", id2, sid2, id1, sid1)
	}

	got, err := st.Get(ctx, id1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	wantNotAfter, perr := time.Parse(time.RFC3339, firstNotAfter)
	if perr != nil {
		t.Fatalf("parse firstNotAfter: %v", perr)
	}
	if got.NotAfter == nil || !got.NotAfter.Equal(wantNotAfter) {
		t.Fatalf("original schedule window was overwritten by the replay: got NotAfter=%v, want %v (window excluded from replay fingerprint, D-07/Open Question 1)", got.NotAfter, wantNotAfter)
	}
}

// TestScheduleMemoryIdempotentRetryAfterWindowLapses pins WR-02: a delayed
// retry of an already-successful schedule_memory call must be recognized as
// a replay even when its (unchanged) not_after value is no longer in the
// future by the time the retry lands — checkIdempotentReplay must run BEFORE
// parseWindow's future-only check, not after, or the retry is wrongly
// rejected with ErrInvalidArgument instead of returning the original record.
func TestScheduleMemoryIdempotentRetryAfterWindowLapses(t *testing.T) {
	d, st := testDepsWithStore(t)
	ctx := authedContext(t, "owner-retry")
	scope := "iso-test:project:idempotency-retry-lapse"
	defer func() {
		cleanupErr(t, "DeleteAll "+scope, st.DeleteAll(context.Background(), scope, store.Authenticated("owner-retry")))
	}()

	clockNow := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
	d.now = func() time.Time { return clockNow }

	base := storeArgs{
		Content: "retry after window lapses", Scope: scope, Source: "user-said", Category: "gotcha",
		IdempotencyKey: "retry-lapse-key",
	}
	notAfter := clockNow.Add(1 * time.Hour).Format(time.RFC3339)

	id1, sid1, err := d.scheduleMemory(ctx, callerFor(ctx, t), scheduleArgs{storeArgs: base, NotAfter: notAfter})
	if err != nil {
		t.Fatalf("scheduleMemory (first): %v", err)
	}

	// Advance the injected clock PAST the original not_after — the client's
	// retry (lost response, network partition, etc.) lands late enough that
	// the SAME not_after value is no longer in the future.
	clockNow = clockNow.Add(2 * time.Hour)

	id2, sid2, err := d.scheduleMemory(ctx, callerFor(ctx, t), scheduleArgs{storeArgs: base, NotAfter: notAfter})
	if err != nil {
		t.Fatalf("scheduleMemory (delayed retry, same not_after now in the past): %v (WR-02: must resolve as a replay, not a parseWindow rejection)", err)
	}
	if id2 != id1 || sid2 != sid1 {
		t.Fatalf("delayed retry minted a different record: (id=%q,sid=%q), want (id=%q,sid=%q)", id2, sid2, id1, sid1)
	}
}

// TestCheckIdempotentReplayCrossToolNamespaceShared pins IN-01: store_memory
// and schedule_memory share the SAME idempotency-key namespace — the point ID
// is derived from (owner, scope, key) alone, with no tool discriminator. A
// store_memory call followed by a schedule_memory retry reusing the same
// scope+key+content is a cross-tool replay: it returns the ORIGINAL
// (unscheduled) record with no window ever applied, not an error and not a
// newly scheduled record. This is intentional (D-07/D-08 lock the point-ID
// hash input) — this test pins the CURRENT documented behavior so a future
// change to either handler can't silently alter it unnoticed.
func TestCheckIdempotentReplayCrossToolNamespaceShared(t *testing.T) {
	d, st := testDepsWithStore(t)
	ctx := authedContext(t, "owner-cross-tool")
	scope := "iso-test:project:idempotency-cross-tool"
	defer func() {
		cleanupErr(t, "DeleteAll "+scope, st.DeleteAll(context.Background(), scope, store.Authenticated("owner-cross-tool")))
	}()

	base := storeArgs{
		Content: "cross-tool shared key", Scope: scope, Source: "user-said", Category: "gotcha",
		IdempotencyKey: "cross-tool-key",
	}

	storeID, storeSID, err := d.storeMemory(ctx, callerFor(ctx, t), base)
	if err != nil {
		t.Fatalf("storeMemory (first): %v", err)
	}

	notAfter := timeNow().Add(1 * time.Hour).Format(time.RFC3339)
	schedID, schedSID, err := d.scheduleMemory(ctx, callerFor(ctx, t), scheduleArgs{storeArgs: base, NotAfter: notAfter})
	if err != nil {
		t.Fatalf("scheduleMemory (cross-tool replay of a store_memory key): %v", err)
	}
	if schedID != storeID || schedSID != storeSID {
		t.Fatalf("cross-tool replay minted a different record: (id=%q,sid=%q), want the original store_memory record (id=%q,sid=%q)", schedID, schedSID, storeID, storeSID)
	}

	got, err := st.Get(ctx, storeID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.NotAfter != nil {
		t.Fatalf("cross-tool replay applied a schedule window to a store_memory-created record: NotAfter=%v, want nil (original record has no window)", got.NotAfter)
	}
}

// TestStoreMemoryEnqueuesOnSuccess pins SC#1: a successful storeMemory
// enqueues the record id for async summary fill, and the worker drains it —
// asserted deterministically via the Wait() drain seam (no time.Sleep).
func TestStoreMemoryEnqueuesOnSuccess(t *testing.T) {
	var mu sync.Mutex
	var handled []string
	d := testDepsWithSummaryQueue(t, 2, 8, func(_ context.Context, id string) error {
		mu.Lock()
		handled = append(handled, id)
		mu.Unlock()
		return nil
	})
	ctx := authedContext(t, "sub-enqueue")
	scope := "iso-test:project:store-enqueue"
	defer func() {
		cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(context.Background(), scope, store.Authenticated("sub-enqueue")))
	}()

	id, _, err := d.storeMemory(ctx, callerFor(ctx, t), storeArgs{Content: "enqueue me", Scope: scope, Source: "user-said", Category: "gotcha"})
	if err != nil {
		t.Fatalf("storeMemory: %v", err)
	}
	d.summaryQueue.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(handled) != 1 || handled[0] != id {
		t.Fatalf("fill handled %v, want exactly [%q]", handled, id)
	}
}

// TestStoreMemoryReturnsWhenSummarizerHangs pins SC#2: the write path is
// never on the synchronous summarizer path. With a fill that blocks
// indefinitely, storeMemory must still return success promptly — enqueue is a
// fire-and-forget, non-blocking tryEnqueue.
func TestStoreMemoryReturnsWhenSummarizerHangs(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	d := testDepsWithSummaryQueue(t, 1, 4, func(ctx context.Context, _ string) error {
		select {
		case <-block:
		case <-ctx.Done():
		}
		return ctx.Err()
	})
	ctx := authedContext(t, "sub-hang")
	scope := "iso-test:project:store-hang"
	defer func() {
		cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(context.Background(), scope, store.Authenticated("sub-hang")))
	}()

	done := make(chan error, 1)
	go func() {
		_, _, err := d.storeMemory(ctx, callerFor(ctx, t), storeArgs{Content: "hangs summarizer", Scope: scope, Source: "user-said", Category: "gotcha"})
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("storeMemory: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("storeMemory did not return promptly while the summarizer fill hung (SC#2)")
	}
}

// TestDiscoveryAndRuleNeverEnqueue pins D-06: store_discovery and store_rule
// own their own summaries and must never enqueue for async fill, even with a
// live queue wired in.
func TestDiscoveryAndRuleNeverEnqueue(t *testing.T) {
	var enqueued atomic.Int64
	d := testDepsWithSummaryQueue(t, 1, 8, func(context.Context, string) error {
		enqueued.Add(1)
		return nil
	})

	discCtx := context.Background()
	discScope := "discovery:repo:negative-space-test"
	defer func() {
		cleanupErr(t, "DeleteAll "+discScope, d.st.DeleteAll(discCtx, discScope, store.Anonymous()))
	}()
	if _, _, err := d.storeDiscovery(discCtx, callerFor(discCtx, t), storeDiscoveryArgs{
		Content: "never enqueues a discovery", Kind: "fact", Scope: discScope,
		Citations: []citationArg{{Kind: "file", Ref: "a.go"}},
	}); err != nil {
		t.Fatalf("storeDiscovery: %v", err)
	}

	ruleCtx := authedContext(t, "sub-rule-no-enqueue")
	ruleScope := "rule:project:negative-space-test"
	defer func() {
		cleanupErr(t, "DeleteAll "+ruleScope, d.st.DeleteAll(ruleCtx, ruleScope, store.Authenticated("sub-rule-no-enqueue")))
	}()
	if _, _, err := d.storeRule(ruleCtx, callerFor(ruleCtx, t), storeRuleArgs{
		Content: "never enqueue rules", Scope: ruleScope, Summary: "no enqueue",
	}); err != nil {
		t.Fatalf("storeRule: %v", err)
	}

	d.summaryQueue.Wait()
	if n := enqueued.Load(); n != 0 {
		t.Errorf("discovery/rule enqueued %d fills, want 0 (D-06 negative space)", n)
	}
}

// upsertFailStore wraps a memStore and overrides only Upsert to return an
// injected error — every other method delegates to the embedded store. It
// exists to make the Upsert-failure branch reachable in tests: spyStore's
// Upsert (fakestore_test.go) always returns nil and has no error-injection
// hook.
type upsertFailStore struct {
	memStore
	err error
}

func (s *upsertFailStore) Upsert(context.Context, store.Memory, []float32) error {
	return s.err
}

// TestPersistAndEnqueueSkipsEnqueueOnUpsertFailure pins the IN-01 (D-05)
// ordering invariant: an enqueue happens only after a confirmed-successful
// Upsert, and never on a failed one, for BOTH storeMemory and
// scheduleMemory. Qdrant-free (spyStore-backed): asserts absence
// deterministically via the Wait() drain seam, no time.Sleep.
//
// This is a characterization test (behavior-preserving refactor, not
// RED->GREEN): it was written and confirmed passing against the
// UN-refactored duplicated-block code before the persistAndEnqueue
// extraction landed, and re-confirmed passing after — see
// 21-02-SUMMARY.md.
func TestPersistAndEnqueueSkipsEnqueueOnUpsertFailure(t *testing.T) {
	injectedErr := errors.New("upsert boom")

	var enqueued atomic.Int64
	q := newSummaryQueue(1, 8, 5*time.Second, nil, func(context.Context, string) error {
		enqueued.Add(1)
		return nil
	})
	q.Start(context.Background())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		q.Shutdown(ctx)
	})

	_, sp := newSpyDeps()
	failing := &upsertFailStore{memStore: sp, err: injectedErr}
	d := &deps{st: failing, em: fakeEmbedder{}, summaryQueue: q}

	c := caller{Subj: store.Authenticated("actor-fail"), Actor: "actor-fail"}

	t.Run("storeMemory", func(t *testing.T) {
		_, _, err := d.storeMemory(context.Background(), c, storeArgs{
			Content: "should not persist", Scope: "s", Category: "gotcha", Source: "user-said",
		})
		if !errors.Is(err, injectedErr) {
			t.Fatalf("storeMemory error = %v, want %v", err, injectedErr)
		}
	})

	t.Run("scheduleMemory", func(t *testing.T) {
		_, _, err := d.scheduleMemory(context.Background(), c, scheduleArgs{
			storeArgs: storeArgs{Content: "should not persist", Scope: "s", Category: "decision", Source: "user-said"},
			NotAfter:  timeNow().Add(time.Hour).Format(time.RFC3339),
		})
		if !errors.Is(err, injectedErr) {
			t.Fatalf("scheduleMemory error = %v, want %v", err, injectedErr)
		}
	})

	q.Wait()
	if n := enqueued.Load(); n != 0 {
		t.Fatalf("enqueued %d fills after a failing Upsert, want 0 (D-05 ordering invariant)", n)
	}
}

// TestUpdateMemoryStaleSummaryGuard pins that updateMemory rejects a content
// change when a caller-authored summary would go stale and the caller did not
// address it (errStaleSummary). Integration: needs Qdrant.
func TestUpdateMemoryStaleSummaryGuard(t *testing.T) {
	d := testDeps(t)
	ctx := authedContext(t, "sub-stale")

	id := "e0000000-0000-0000-0000-000000000001"
	scope := "stale:project:summary"
	m := store.Memory{
		ID: id, Content: "original content", Scope: scope,
		Category: "convention", Source: "agent-inferred",
		Owner: "sub-stale", Summary: "hand-written", SummarySource: store.SummarySourceClient,
		CreatedAt: timeNow(),
	}
	if err := d.st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Cleanup(func() {
		cleanupErr(t, "Delete "+id, d.st.Delete(context.Background(), id, store.Authenticated("sub-stale")))
	})

	// Change content but omit summary — must be rejected with errStaleSummary.
	_, err := d.updateMemory(ctx, callerFor(ctx, t), updateArgs{
		ID:      id,
		Content: strp("changed content"),
	})
	if !errors.Is(err, errStaleSummary) {
		t.Errorf("updateMemory with changed content + unaddressed client summary: want errStaleSummary, got %v", err)
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

	anonCaller := callerFor(ctx, t)

	// searchMemory with anonymous context must return ownerless, not shared.
	hits, err := d.searchMemory(ctx, anonCaller, coreSearchRequest{Query: "content", Scope: scope, K: 10})
	if err != nil {
		t.Fatalf("searchMemory: %v", err)
	}
	foundOwnerless, foundShared := false, false
	for _, m := range hits {
		if m.ID == ownerlessID {
			foundOwnerless = true
		}
		if m.ID == sharedID {
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
	res, err := d.listMemory(ctx, anonCaller, coreListRequest{Scope: scope, Limit: 10})
	if err != nil {
		t.Fatalf("listMemory: %v", err)
	}
	foundOwnerless, foundShared = false, false
	for _, mem := range res.Memories {
		if mem.ID == ownerlessID {
			foundOwnerless = true
		}
		if mem.ID == sharedID {
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

	hits, err := d.searchDiscovery(ctx, callerFor(ctx, t), searchDiscoveryArgs{Query: "discovery", Scope: scope, K: 10})
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
	id, _, err := d.storeDiscovery(ctx, callerFor(ctx, t), storeDiscoveryArgs{
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
	id2, _, err := d.storeDiscovery(ctx, callerFor(ctx, t), storeDiscoveryArgs{
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
	hits, err := d.searchDiscovery(ctx, callerFor(ctx, t), searchDiscoveryArgs{Query: "understanding", Scope: scope})
	if err != nil {
		t.Fatalf("searchDiscovery: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("expected >= 1 hit")
	}
	// scope required unless cross_spine
	if _, err := d.searchDiscovery(ctx, callerFor(ctx, t), searchDiscoveryArgs{Query: "x"}); err == nil {
		t.Error("expected error: scope required when cross_spine=false")
	}
	// cross_spine path (with a scope present, the ignore-warn branch) must not error
	if _, err := d.searchDiscovery(ctx, callerFor(ctx, t), searchDiscoveryArgs{Query: "x", CrossSpine: true, Scope: scope}); err != nil {
		t.Errorf("cross_spine search errored: %v", err)
	}
}

func TestStoreDiscoveryMintsThenReplacePreservesShortID(t *testing.T) {
	d := testDeps(t)
	ctx := authedContext(t, "owner-A")
	cites := []citationArg{{Kind: "file", Ref: "a.go", Pin: "abc"}}
	id, sid, err := d.storeDiscovery(ctx, callerFor(ctx, t), storeDiscoveryArgs{Content: "map1", Kind: "map", Scope: "discovery:repo:x", Citations: cites})
	if err != nil || len(sid) != shortid.Length {
		t.Fatalf("create: sid=%q err=%v", sid, err)
	}
	// Replace by UUID → same point, same short id.
	id2, sid2, err := d.storeDiscovery(ctx, callerFor(ctx, t), storeDiscoveryArgs{ID: id, Content: "map1b", Kind: "map", Scope: "discovery:repo:x", Citations: cites})
	if err != nil || id2 != id || sid2 != sid {
		t.Fatalf("replace-by-uuid: id %q->%q sid %q->%q err %v", id, id2, sid, sid2, err)
	}
	// Replace by SHORT ID → resolves to the same point, still same short id.
	id3, sid3, err := d.storeDiscovery(ctx, callerFor(ctx, t), storeDiscoveryArgs{ID: sid, Content: "map1c", Kind: "map", Scope: "discovery:repo:x", Citations: cites})
	if err != nil || id3 != id || sid3 != sid {
		t.Fatalf("replace-by-shortid: id %q->%q sid %q->%q err %v", id, id3, sid, sid3, err)
	}
}

func TestStoreDiscoveryRejectsNonexistentShortIDAsNew(t *testing.T) {
	d := testDeps(t)
	ctx := authedContext(t, "owner-A")
	_, _, err := d.storeDiscovery(ctx, callerFor(ctx, t), storeDiscoveryArgs{ID: "zzzzzzzzzz", Content: "x", Kind: "fact", Scope: "discovery:repo:x", Citations: []citationArg{{Kind: "file", Ref: "a", Pin: "p"}}})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

// TestStoreDiscoveryCrossOwnerShortIDDoesNotLeakUUID pins that a replace attempt
// against another owner's short id fails with an error echoing only the
// caller-supplied input — never the resolved point UUID, which would leak the
// private record's existence and identity.
func TestStoreDiscoveryCrossOwnerShortIDDoesNotLeakUUID(t *testing.T) {
	d := testDeps(t)
	ctxA := authedContext(t, "owner-A")
	cites := []citationArg{{Kind: "file", Ref: "a.go", Pin: "abc"}}
	id, sid, err := d.storeDiscovery(ctxA, callerFor(ctxA, t), storeDiscoveryArgs{Content: "m", Kind: "map", Scope: "discovery:repo:x", Citations: cites})
	if err != nil {
		t.Fatal(err)
	}
	ctxB := authedContext(t, "owner-B")
	_, _, err = d.storeDiscovery(ctxB, callerFor(ctxB, t), storeDiscoveryArgs{ID: sid, Content: "m2", Kind: "map", Scope: "discovery:repo:x", Citations: cites})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if strings.Contains(err.Error(), id) {
		t.Fatalf("error leaks resolved UUID: %v", err)
	}
	if !strings.Contains(err.Error(), sid) {
		t.Fatalf("error should echo caller-supplied id only: %v", err)
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
	c := callerFor(ctx, t)

	// Single tag: both alpha-carrying records, never the untagged one — on both handlers.
	hits, err := d.searchMemory(ctx, c, coreSearchRequest{Query: "x", Scope: scope, K: 10, Tags: []string{"alpha"}})
	if err != nil {
		t.Fatalf("searchMemory alpha: %v", err)
	}
	if g := ids(hits); !g[alphaID] || !g[bothID] || g[plainID] {
		t.Errorf("searchMemory alpha wrong: %v", g)
	}
	mems, err := d.listMemory(ctx, c, coreListRequest{Scope: scope, Limit: 10, Tags: []string{"alpha"}})
	if err != nil {
		t.Fatalf("listMemory alpha: %v", err)
	}
	if g := ids(mems.Memories); !g[alphaID] || !g[bothID] || g[plainID] {
		t.Errorf("listMemory alpha wrong: %v", g)
	}

	// AND of two tags: only the record carrying both; the alpha-only record is excluded.
	hits, err = d.searchMemory(ctx, c, coreSearchRequest{Query: "x", Scope: scope, K: 10, Tags: []string{"alpha", "beta"}})
	if err != nil {
		t.Fatalf("searchMemory AND: %v", err)
	}
	if g := ids(hits); !g[bothID] || g[alphaID] || g[plainID] {
		t.Errorf("searchMemory AND-subset wrong: %v", g)
	}

	// Omitted tags: passthrough returns all three — on both handlers.
	hits, err = d.searchMemory(ctx, c, coreSearchRequest{Query: "x", Scope: scope, K: 10})
	if err != nil {
		t.Fatalf("searchMemory passthrough: %v", err)
	}
	if g := ids(hits); !g[alphaID] || !g[bothID] || !g[plainID] {
		t.Errorf("searchMemory passthrough wrong: %v", g)
	}
	mems, err = d.listMemory(ctx, c, coreListRequest{Scope: scope, Limit: 10})
	if err != nil {
		t.Fatalf("listMemory passthrough: %v", err)
	}
	if g := ids(mems.Memories); !g[alphaID] || !g[bothID] || !g[plainID] {
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
	if _, err := d.updateMemory(ctx, callerFor(ctx, t), updateArgs{ID: id, Content: strp("v2")}); err != nil {
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

// TestUpdateMemoryReturnsMutationResult pins that deps.updateMemory returns a
// typed mutationResult sourced from the already-fetched record (review HIGH):
// the ID is always the canonical UUID and the ShortID is the persisted handle
// — even when the caller supplied the short_id as a.ID (resolved, not echoed).
func TestUpdateMemoryReturnsMutationResult(t *testing.T) {
	d := testDeps(t)
	ctx := authedContext(t, "sub-mutresult")
	id, sid, err := d.storeMemory(ctx, callerFor(ctx, t), storeArgs{Content: "v1", Scope: "s", Category: "gotcha", Source: "user-said"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupErr(t, "Delete "+id, d.st.Delete(ctx, id, store.Authenticated("sub-mutresult"))) })

	// Call by SHORT ID: mutationResult.ID must be the canonical UUID, not the
	// caller-supplied short id.
	mr, err := d.updateMemory(ctx, callerFor(ctx, t), updateArgs{ID: sid, Content: strp("v2")})
	if err != nil {
		t.Fatalf("updateMemory: %v", err)
	}
	if mr.ID != id {
		t.Errorf("mutationResult.ID = %q, want canonical UUID %q", mr.ID, id)
	}
	if mr.ShortID != sid {
		t.Errorf("mutationResult.ShortID = %q, want %q", mr.ShortID, sid)
	}

	// Same contract on the payload-only (shared/summary-only) route.
	shared := true
	mr2, err := d.updateMemory(ctx, callerFor(ctx, t), updateArgs{ID: sid, Shared: &shared})
	if err != nil {
		t.Fatalf("updateMemory (payload-only): %v", err)
	}
	if mr2.ID != id || mr2.ShortID != sid {
		t.Errorf("payload-only mutationResult = %+v, want ID=%q ShortID=%q", mr2, id, sid)
	}
}

// TestSetVisibilityReturnsMutationResult mirrors
// TestUpdateMemoryReturnsMutationResult for deps.setVisibility.
func TestSetVisibilityReturnsMutationResult(t *testing.T) {
	d := testDeps(t)
	ctx := authedContext(t, "sub-mutresult-vis")
	id, sid, err := d.storeMemory(ctx, callerFor(ctx, t), storeArgs{Content: "v1", Scope: "s", Category: "gotcha", Source: "user-said"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupErr(t, "Delete "+id, d.st.Delete(ctx, id, store.Authenticated("sub-mutresult-vis"))) })

	mr, err := d.setVisibility(ctx, callerFor(ctx, t), setVisibilityArgs{ID: sid, Shared: true})
	if err != nil {
		t.Fatalf("setVisibility: %v", err)
	}
	if mr.ID != id {
		t.Errorf("mutationResult.ID = %q, want canonical UUID %q", mr.ID, id)
	}
	if mr.ShortID != sid {
		t.Errorf("mutationResult.ShortID = %q, want %q", mr.ShortID, sid)
	}
}

// TestSupersedeMemory pins the supersede_memory handler contract (SC1-SC4):
// storing with supersedes stamps the target's superseded_by, hides the target
// from search_memory/list_memory while keeping it fetchable via get_memory,
// and a non-owner caller supersede attempt on someone else's target is
// rejected with store.ErrNotFound re-wrapped with the caller's ORIGINAL
// unresolved a.Supersedes input (404-indistinguishability), never the
// resolved target UUID.
func TestSupersedeMemory(t *testing.T) {
	d := testDeps(t)
	ctx := authedContext(t, "sub-supersede-a")
	c := callerFor(ctx, t)
	scope := "iso-test:project:supersede-handler"

	targetID, targetSID, err := d.storeMemory(ctx, c, storeArgs{
		Content: "original content", Scope: scope, Category: "gotcha", Source: "user-said",
	})
	if err != nil {
		t.Fatalf("seed store: %v", err)
	}
	t.Cleanup(func() {
		cleanupErr(t, "Delete target", d.st.Delete(context.Background(), targetID, store.Authenticated("sub-supersede-a")))
	})

	newID, newSID, err := d.supersedeMemory(ctx, c, supersedeArgs{
		storeArgs:  storeArgs{Content: "corrected content", Scope: scope, Category: "gotcha", Source: "user-said"},
		Supersedes: targetSID,
	})
	if err != nil {
		t.Fatalf("supersedeMemory: %v", err)
	}
	if newID == "" || newSID == "" {
		t.Fatalf("supersedeMemory returned empty id/short_id: id=%q short_id=%q", newID, newSID)
	}
	t.Cleanup(func() {
		cleanupErr(t, "Delete new record", d.st.Delete(context.Background(), newID, store.Authenticated("sub-supersede-a")))
	})

	// The target must show superseded_by == new id, content untouched.
	target, err := d.getMemory(ctx, c, idArgs{ID: targetID})
	if err != nil {
		t.Fatalf("get target: %v", err)
	}
	if target.SupersededBy == nil || *target.SupersededBy != newID {
		t.Errorf("target.SupersededBy = %v, want %q", target.SupersededBy, newID)
	}
	if target.Content != "original content" {
		t.Errorf("target content mutated: %q", target.Content)
	}

	// The new record must carry supersedes == target.
	newRec, err := d.getMemory(ctx, c, idArgs{ID: newID})
	if err != nil {
		t.Fatalf("get new record: %v", err)
	}
	if newRec.Supersedes == nil || *newRec.Supersedes != targetID {
		t.Errorf("newRec.Supersedes = %v, want %q", newRec.Supersedes, targetID)
	}

	// The target must be absent from list_memory.
	listRes, err := d.listMemory(ctx, c, coreListRequest{Scope: scope, Limit: 50})
	if err != nil {
		t.Fatalf("listMemory: %v", err)
	}
	for _, m := range listRes.Memories {
		if m.ID == targetID {
			t.Errorf("target %s still present in list_memory after supersede", targetID)
		}
	}

	// The target must be absent from search_memory.
	hits, err := d.searchMemory(ctx, c, coreSearchRequest{Scope: scope, Query: "original content", K: 10})
	if err != nil {
		t.Fatalf("searchMemory: %v", err)
	}
	for _, m := range hits {
		if m.ID == targetID {
			t.Errorf("target %s still present in search_memory after supersede", targetID)
		}
	}

	// Owner-gate re-wrap: a different owner cannot supersede sub-supersede-a's
	// target; the error re-wraps store.ErrNotFound with the caller's ORIGINAL
	// a.Supersedes input (the target's short_id), never the resolved UUID —
	// mirrors setVisibility/storeDiscovery's 404-indistinguishability.
	otherCtx := authedContext(t, "sub-supersede-b")
	otherC := callerFor(otherCtx, t)
	_, _, err = d.supersedeMemory(otherCtx, otherC, supersedeArgs{
		storeArgs:  storeArgs{Content: "attacker content", Scope: scope, Category: "gotcha", Source: "user-said"},
		Supersedes: targetSID,
	})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("cross-owner supersede err = %v, want store.ErrNotFound", err)
	}
	if !strings.Contains(err.Error(), targetSID) {
		t.Errorf("cross-owner err %v does not echo caller's original input %q", err, targetSID)
	}
	if strings.Contains(err.Error(), targetID) {
		t.Errorf("cross-owner err %v leaks resolved target UUID %q (404-indistinguishability violation)", err, targetID)
	}
}

// TestSupersedeMemoryRejectsRule (CR-02) pins that supersede_memory rejects a
// rule-category target with errRuleImmutable, mirroring
// TestUpdateMemoryRuleGuardRejectsUnshare/TestSetVisibilityRejectsRule — a
// rule must be deleted, never superseded, so it never silently vanishes from
// list_rules' "complete rule set" index (Store.List's unconditional
// superseded_by gate).
func TestSupersedeMemoryRejectsRule(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "rule:repo:supersede-rule-guard-test"
	t.Cleanup(func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(ctx, scope, store.Anonymous())) })

	id, sid, err := d.storeRule(ctx, callerFor(ctx, t), storeRuleArgs{
		Content: "some rule", Scope: scope, Summary: "some rule",
	})
	if err != nil {
		t.Fatalf("seed rule: %v", err)
	}

	_, _, err = d.supersedeMemory(ctx, callerFor(ctx, t), supersedeArgs{
		storeArgs:  storeArgs{Content: "corrected rule text", Scope: scope, Category: "rule", Source: "user-said"},
		Supersedes: sid,
	})
	if err == nil {
		t.Fatal("expected supersede_memory on a rule to be rejected")
	}
	if !errors.Is(err, errRuleImmutable) {
		t.Errorf("want errRuleImmutable, got %v", err)
	}

	// The rule is untouched: not back-stamped, and still present in list_rules.
	got, err := d.st.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get rule: %v", err)
	}
	if got.SupersededBy != nil {
		t.Errorf("rule.SupersededBy = %v, want nil (rejected supersede must not stamp)", got.SupersededBy)
	}
	rules, _, err := d.listRules(ctx, callerFor(ctx, t), listRulesArgs{Scopes: []string{scope}})
	if err != nil {
		t.Fatalf("listRules: %v", err)
	}
	found := false
	for _, r := range rules {
		if rv, ok := r.(ruleView); ok && rv.ID == id {
			found = true
		}
	}
	if !found {
		t.Errorf("rule %s missing from list_rules after rejected supersede: %+v", id, rules)
	}
}

// TestSupersedeMemoryEmbedNotCalledForNonOwner (CR-03) mirrors
// TestUpdateMemoryEmbedNotCalledForNonOwner: a caller superseding a target
// they do not own must be rejected BEFORE the billable Embed call and the
// Qdrant-hitting MintShortID call — reproducing update_memory's
// cost-amplification hardening for supersede_memory.
func TestSupersedeMemoryEmbedNotCalledForNonOwner(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "iso-test:project:supersede-embed-gate"

	stampedID := "a8a8a8a8-0000-0000-0000-000000000001"
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

	counter := &countingEmbedder{}
	d.em = counter

	// Non-owner call (anonymous ctx -> sub=="" != "sub-owner") must fail
	// without embedding or minting a short id.
	_, _, err := d.supersedeMemory(ctx, callerFor(ctx, t), supersedeArgs{
		storeArgs:  storeArgs{Content: "hijack", Scope: scope, Category: "convention", Source: "agent-inferred"},
		Supersedes: stampedID,
	})
	if err == nil {
		t.Fatal("non-owner supersedeMemory: expected error, got nil")
	}
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("non-owner supersedeMemory: want ErrNotFound, got %v", err)
	}
	if counter.calls != 0 {
		t.Errorf("non-owner supersedeMemory: embed must not be called; got %d call(s)", counter.calls)
	}
}

// TestSupersedeMemorySchemaExcludesIdempotencyKey (WR-03) pins that
// supersede_memory's advertised JSON schema does NOT include
// idempotency_key: supersede's idempotency was deliberately deferred (plan
// T-25-10), and deps.supersedeMemory never reads the field, so the field
// must not be promoted onto the wire schema (it would silently mislead a
// client into believing supersede retries are safe).
func TestSupersedeMemorySchemaExcludesIdempotencyKey(t *testing.T) {
	schema, err := jsonschema.For[supersedeArgs](nil)
	if err != nil {
		t.Fatalf("jsonschema.For[supersedeArgs]: %v", err)
	}
	if _, ok := schema.Properties["idempotency_key"]; ok {
		t.Errorf("supersede_memory schema advertises idempotency_key, want it excluded (WR-03)")
	}
	// Sanity: the schema still carries the fields supersede_memory DOES
	// support, proving the exclusion is targeted, not a broken reflection.
	for _, want := range []string{"content", "scope", "supersedes"} {
		if _, ok := schema.Properties[want]; !ok {
			t.Errorf("supersede_memory schema missing expected field %q: %+v", want, schema.Properties)
		}
	}
}

// TestSupersedeArgsDecodePopulatesPromotedIdempotencyKey (WR-04) pins the
// wire-decode half of supersedeArgs' IdempotencyKey shadow that WR-03's
// schema-only test did not cover: a json:"-" field has no JSON name, so it
// never enters encoding/json's same-name shadowing contest — it excuses
// itself, leaving the promoted storeArgs.IdempotencyKey
// (json:"idempotency_key,omitempty") as the sole decode target. So a raw
// idempotency_key on the wire STILL lands in a.storeArgs.IdempotencyKey,
// contrary to what an earlier (now-corrected) doc comment claimed. This is
// exactly why supersedeMemory defensively clears the field itself rather
// than relying on the shadow — see TestSupersedeMemoryIgnoresIdempotencyKey.
func TestSupersedeArgsDecodePopulatesPromotedIdempotencyKey(t *testing.T) {
	var a supersedeArgs
	in := `{"idempotency_key":"probe-key","content":"x","scope":"s","supersedes":"y"}`
	if err := json.Unmarshal([]byte(in), &a); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if a.storeArgs.IdempotencyKey != "probe-key" {
		t.Errorf("a.storeArgs.IdempotencyKey (promoted) = %q, want %q (the json:\"-\" shadow does not block wire decode)", a.storeArgs.IdempotencyKey, "probe-key")
	}
	if a.IdempotencyKey != "" {
		t.Errorf("a.IdempotencyKey (outer json:\"-\" shadow) = %q, want empty (it has no JSON name to decode into)", a.IdempotencyKey)
	}
}

// TestSupersedeMemoryIgnoresIdempotencyKey (WR-04) pins that a caller-sent
// idempotency_key on supersede_memory is silently IGNORED end to end: no
// replay lookup, no error, a normal supersede happens — the defensive clear
// at the top of supersedeMemory (tools.go) is what makes the decoded-but-
// unadvertised field inert, not the schema-only json:"-" shadow (see
// TestSupersedeArgsDecodePopulatesPromotedIdempotencyKey above).
func TestSupersedeMemoryIgnoresIdempotencyKey(t *testing.T) {
	d := testDeps(t)
	ctx := authedContext(t, "sub-supersede-idem")
	c := callerFor(ctx, t)
	scope := "iso-test:project:supersede-idempotency-ignored"

	targetID, targetSID, err := d.storeMemory(ctx, c, storeArgs{
		Content: "original content", Scope: scope, Category: "gotcha", Source: "user-said",
	})
	if err != nil {
		t.Fatalf("seed store: %v", err)
	}
	t.Cleanup(func() {
		cleanupErr(t, "Delete target", d.st.Delete(context.Background(), targetID, store.Authenticated("sub-supersede-idem")))
	})

	newID, newSID, err := d.supersedeMemory(ctx, c, supersedeArgs{
		storeArgs: storeArgs{
			Content: "corrected content", Scope: scope, Category: "gotcha", Source: "user-said",
			IdempotencyKey: "some-key-that-must-be-ignored",
		},
		Supersedes: targetSID,
	})
	if err != nil {
		t.Fatalf("supersedeMemory with idempotency_key set: want a normal supersede (ignored key), got error: %v", err)
	}
	if newID == "" || newSID == "" {
		t.Fatalf("supersedeMemory returned empty id/short_id: id=%q short_id=%q", newID, newSID)
	}
	t.Cleanup(func() {
		cleanupErr(t, "Delete new record", d.st.Delete(context.Background(), newID, store.Authenticated("sub-supersede-idem")))
	})

	newRec, err := d.getMemory(ctx, c, idArgs{ID: newID})
	if err != nil {
		t.Fatalf("get new record: %v", err)
	}
	if newRec.Content != "corrected content" {
		t.Errorf("newRec.Content = %q, want %q (a normal supersede, not a replay)", newRec.Content, "corrected content")
	}

	// Calling again with the SAME idempotency_key must NOT be treated as a
	// replay: it is a brand-new supersede attempt against an
	// already-superseded target, so it fails ErrAlreadySuperseded — never
	// returns the first call's id, which would indicate the key was
	// (incorrectly) honored as a replay key.
	replayID, _, err := d.supersedeMemory(ctx, c, supersedeArgs{
		storeArgs: storeArgs{
			Content: "corrected content", Scope: scope, Category: "gotcha", Source: "user-said",
			IdempotencyKey: "some-key-that-must-be-ignored",
		},
		Supersedes: targetSID,
	})
	if err == nil {
		t.Errorf("second supersedeMemory with same idempotency_key + already-superseded target: want error, got id=%q (key was NOT ignored — looks like a replay)", replayID)
	}
}

// TestSupersedeMemoryDiscoveryTarget (IN-02) exercises superseding a
// discovery-category target (no category restriction analogous to CR-02's
// rule guard applies to discoveries — D-07 allows it) and pins WR-01's
// SearchDiscovery soft-hide gate end to end through the handler.
func TestSupersedeMemoryDiscoveryTarget(t *testing.T) {
	d := testDeps(t)
	ctx := authedContext(t, "sub-supersede-disc")
	c := callerFor(ctx, t)
	scope := "discovery:repo:supersede-disc-test"
	t.Cleanup(func() { cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(context.Background(), scope, store.Anonymous())) })

	targetID, targetSID, err := d.storeDiscovery(ctx, c, storeDiscoveryArgs{
		Content: "original discovery", Kind: "fact", Scope: scope,
		Citations: []citationArg{{Kind: "file", Ref: "f.go"}},
	})
	if err != nil {
		t.Fatalf("seed storeDiscovery: %v", err)
	}

	newID, _, err := d.supersedeMemory(ctx, c, supersedeArgs{
		storeArgs:  storeArgs{Content: "corrected discovery", Scope: scope, Category: "discovery", Source: "agent-inferred"},
		Supersedes: targetSID,
	})
	if err != nil {
		t.Fatalf("supersedeMemory on discovery target: %v", err)
	}
	t.Cleanup(func() {
		cleanupErr(t, "Delete new record", d.st.Delete(context.Background(), newID, store.Authenticated("sub-supersede-disc")))
	})

	// The target still fetchable (get_memory ungated) and back-stamped.
	target, err := d.getMemory(ctx, c, idArgs{ID: targetID})
	if err != nil {
		t.Fatalf("get target: %v", err)
	}
	if target.SupersededBy == nil || *target.SupersededBy != newID {
		t.Errorf("target.SupersededBy = %v, want %q", target.SupersededBy, newID)
	}

	// The superseded discovery must be soft-hidden from search_discovery (WR-01).
	hits, err := d.searchDiscovery(ctx, c, searchDiscoveryArgs{Query: "original discovery", Scope: scope, K: 10})
	if err != nil {
		t.Fatalf("searchDiscovery: %v", err)
	}
	for _, h := range hits {
		if h.ID == targetID {
			t.Errorf("superseded discovery %s still present in search_discovery", targetID)
		}
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
	if _, err := d.updateMemory(ctx, callerFor(ctx, t), updateArgs{ID: id, Content: strp("v2"), Tags: &newTags}); err != nil {
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
	if _, err := d.updateMemory(ctx, callerFor(ctx, t), updateArgs{ID: id, Content: strp("v3")}); err != nil {
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
	if _, err := d.updateMemory(ctx, callerFor(ctx, t), updateArgs{ID: id, Content: strp("v4"), Tags: &empty}); err != nil {
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
	_, err := d.updateMemory(ctx, callerFor(ctx, t), updateArgs{ID: stampedID, Content: strp("hijack")})
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
	if _, err := d.updateMemory(ctx, callerFor(ctx, t), updateArgs{ID: ownerlessID, Content: strp("v2")}); err != nil {
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
	bCaller := callerFor(bctx, t)

	// searchMemory: B sees A's shared, not A's private.
	hits, err := d.searchMemory(bctx, bCaller, coreSearchRequest{Query: "content", Scope: scope, K: 10})
	if err != nil {
		t.Fatalf("searchMemory: %v", err)
	}
	assertVisibility(t, "searchMemory", hits, sharedID, privateID)

	// listMemory: same guarantee through the no-query handler path.
	mems, err := d.listMemory(bctx, bCaller, coreListRequest{Scope: scope, Limit: 10})
	if err != nil {
		t.Fatalf("listMemory: %v", err)
	}
	assertVisibility(t, "listMemory", mems.Memories, sharedID, privateID)
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
	if _, err := Register(s, http.NewServeMux(), tm, nil, nil, nil, nil, nil); err == nil {
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
	id, _, err := d.scheduleMemory(ctx, callerFor(ctx, t), scheduleArgs{storeArgs: storeArgs{Content: "future",
		Scope: "ls:project:x", Source: "user-said", Category: "decision"},
		NotBefore: "2099-01-01T00:00:00Z"})
	if err != nil {
		t.Fatalf("schedule: %v", err)
	}
	t.Cleanup(func() { _ = d.st.Delete(context.Background(), id, store.Authenticated("sub-A")) })

	got, err := d.listScheduled(ctx, callerFor(ctx, t), listScheduledArgs{Scope: "ls:project:x"}) // default state=scheduled
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
	_, _, _, _, err := StoreAndEmbedderFromEnvNoEnsure()
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
		failOrSkipNoQdrant(t)
	}
	// buildDepsFromEnv reads the data-plane config from the process env; point it
	// at the test Qdrant with a dedicated collection so EnsureCollection succeeds.
	// Also isolate from ambient ENGRAM_SUMMARY_* in the dev/CI shell (IN-02):
	// empty values preserve the registry default (the documented empty-env
	// invariant), so an ambient summary-on-write env can never start a real
	// summary queue this test never shuts down — that would leak 2 worker
	// goroutines for the test binary's lifetime.
	t.Setenv("ENGRAM_QDRANT_ADDR", testQdrantAddr)
	t.Setenv("ENGRAM_QDRANT_COLLECTION", "mem_load_once_test")
	t.Setenv("ENGRAM_EMBED_DIM", "3")
	t.Setenv("ENGRAM_SUMMARY_MODEL", "")
	t.Setenv("ENGRAM_SUMMARY_ON_WRITE", "")

	loads := 0
	orig := configLoad
	configLoad = func(flags *flag.FlagSet) (*config.Config, error) {
		loads++
		return orig(flags)
	}
	t.Cleanup(func() { configLoad = orig })

	d, err := buildDepsFromEnv(nil, nil)
	if err != nil {
		t.Fatalf("buildDepsFromEnv: %v", err)
	}

	if loads != 1 {
		t.Errorf("buildDepsFromEnv loaded config %d times, want exactly 1", loads)
	}

	// Review round-2 MEDIUM: prove the PRODUCTION builder actually computes a
	// non-empty identity — Task 4's sentinel tests set d.embedderIdentity
	// directly, which proves handlers PERSIST a provided identity but not
	// that buildDepsFromEnv COMPUTES one. A missed builder assignment would
	// ship an empty identity while every sentinel test stays green.
	if d.embedderIdentity == "" {
		t.Error("buildDepsFromEnv did not compute a non-empty embedderIdentity")
	}
	if !strings.HasPrefix(d.embedderIdentity, "v1:") {
		t.Errorf("buildDepsFromEnv embedderIdentity = %q, want v1: prefix", d.embedderIdentity)
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

	st, dim, em, identity, err := StoreAndEmbedderFromEnvNoEnsure()
	if err != nil {
		t.Fatalf("StoreAndEmbedderFromEnvNoEnsure: %v", err)
	}
	if st == nil || em == nil || dim != 3 {
		t.Fatalf("got store=%v dim=%d embedder=%v, want non-nil store/embedder and dim 3", st, dim, em)
	}

	if loads != 1 {
		t.Errorf("StoreAndEmbedderFromEnvNoEnsure loaded config %d times, want exactly 1", loads)
	}

	// Review round-2 MEDIUM: behavior-test the returned identity, not just its
	// arity — a helper that returned an empty identity would still pass an
	// arity-only compile check. Load the expected cfg via the unwrapped
	// loader (orig) so this expected-value load does not inflate the loads
	// counter asserted above.
	expectedCfg, err := orig(nil)
	if err != nil {
		t.Fatalf("orig config load: %v", err)
	}
	wantIdentity, err := config.EmbedderIdentity(expectedCfg)
	if err != nil {
		t.Fatalf("config.EmbedderIdentity: %v", err)
	}
	if identity == "" {
		t.Error("StoreAndEmbedderFromEnvNoEnsure did not compute a non-empty embedder identity")
	}
	if !strings.HasPrefix(identity, "v1:") {
		t.Errorf("StoreAndEmbedderFromEnvNoEnsure identity = %q, want v1: prefix", identity)
	}
	if identity != wantIdentity {
		t.Errorf("StoreAndEmbedderFromEnvNoEnsure identity = %q, want %q (config.EmbedderIdentity for the same config)", identity, wantIdentity)
	}
}

func TestToMemorySetsClientSummarySource(t *testing.T) {
	withSummary := storeArgs{Content: "c", Scope: "s", Source: "user-said", Category: "decision", Summary: "terse"}
	m := withSummary.toMemory("owner", "actor", time.Now())
	if m.Summary != "terse" || m.SummarySource != store.SummarySourceClient {
		t.Fatalf("client summary not mapped: summary=%q source=%q", m.Summary, m.SummarySource)
	}
	noSummary := storeArgs{Content: "c", Scope: "s", Source: "user-said", Category: "decision"}
	m2 := noSummary.toMemory("owner", "actor", time.Now())
	if m2.Summary != "" || m2.SummarySource != store.SummarySourceNone {
		t.Fatalf("absent summary must leave source empty: %+v", m2)
	}
}

func TestListMemoryReturnsNextCursorField(t *testing.T) {
	d := testDeps(t) // skips without Qdrant
	ctx := authedContext(t, "sub-A")
	c := callerFor(ctx, t)
	scope := "tool:project:nextcursor"
	ids := []string{
		"f0000000-0000-0000-0000-000000000001",
		"f0000000-0000-0000-0000-000000000002",
	}
	// Distinct created_at so paging crosses a boundary deterministically.
	base := d.clock().UTC().Truncate(time.Second)
	for i, id := range ids {
		if err := d.st.Upsert(ctx, store.Memory{ID: id, Content: "c", Scope: scope,
			Owner: "sub-A", CreatedAt: base.Add(time.Duration(i) * time.Hour)}, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
		t.Cleanup(func() { _ = d.st.Delete(context.Background(), id, store.Authenticated("sub-A")) })
	}

	// Page 1: a full page (limit 1, two records) MUST issue a cursor. The core
	// no longer hard-codes cursor mode (round-6 MED, grok) — this deps-level
	// call sets CursorMode explicitly, mirroring the MCP list_memory closure.
	page1, err := d.listMemory(ctx, c, coreListRequest{Scope: scope, Limit: 1, CursorMode: true})
	if err != nil {
		t.Fatalf("listMemory page 1: %v", err)
	}
	if len(page1.Memories) != 1 {
		t.Fatalf("page 1: got %d records want 1", len(page1.Memories))
	}
	if page1.NextToken == "" {
		t.Fatal("page 1: empty next_cursor on a full page; want a resume token")
	}

	// Page 2: resuming with that cursor returns the OTHER record.
	page2, err := d.listMemory(ctx, c, coreListRequest{Scope: scope, Limit: 1, Cursor: page1.NextToken, CursorMode: true})
	if err != nil {
		t.Fatalf("listMemory page 2: %v", err)
	}
	if len(page2.Memories) != 1 {
		t.Fatalf("page 2: got %d records want 1", len(page2.Memories))
	}
	id1, id2 := page1.Memories[0].ID, page2.Memories[0].ID
	if id1 == id2 {
		t.Errorf("page 2 returned the same record as page 1 (%s); cursor did not advance", id1)
	}
}

// TestListMemorySupersetOffsetAndCursorModes proves the typed core list
// contract (D-07) preserves the FULL Connect list superset — offset,
// categories, visibility, and the exact matched total — on the shared path,
// not just the MCP-shaped subset. Round-4 HIGH-2: offset mode and cursor mode
// are mutually exclusive in the store (store.go:817/:865), so this SPLITS the
// assertion into two subtests rather than pairing Offset>0 with a
// non-empty-NextToken assertion (impossible against the live store; the prior
// combined assertion passed only against a token-fabricating fake).
func TestListMemorySupersetOffsetAndCursorModes(t *testing.T) {
	d := testDeps(t) // skips without Qdrant
	ctx := authedContext(t, "sub-A")
	c := callerFor(ctx, t)
	base := d.clock().UTC().Truncate(time.Second)

	t.Run("offset_mode_exact_total_no_next_token", func(t *testing.T) {
		scope := "tool:project:superset-offset"
		// Three private "decision" records (the matched set) plus one "gotcha"
		// record the category filter must exclude.
		decisionIDs := []string{
			"c0000000-0000-0000-0000-000000000001",
			"c0000000-0000-0000-0000-000000000002",
			"c0000000-0000-0000-0000-000000000003",
		}
		for i, id := range decisionIDs {
			if err := d.st.Upsert(ctx, store.Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A",
				Category: "decision", Visibility: "private", CreatedAt: base.Add(time.Duration(i) * time.Hour)},
				[]float32{0.1, 0.2, 0.3}); err != nil {
				t.Fatalf("seed %s: %v", id, err)
			}
			t.Cleanup(func() { _ = d.st.Delete(context.Background(), id, store.Authenticated("sub-A")) })
		}
		gotchaID := "c0000000-0000-0000-0000-000000000004"
		if err := d.st.Upsert(ctx, store.Memory{ID: gotchaID, Content: "c", Scope: scope, Owner: "sub-A",
			Category: "gotcha", Visibility: "private", CreatedAt: base}, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", gotchaID, err)
		}
		t.Cleanup(func() { _ = d.st.Delete(context.Background(), gotchaID, store.Authenticated("sub-A")) })

		// Offset + Categories + Visibility, a small Limit — Total must reflect
		// the full matched set (3 "decision" records), not the page length (1);
		// NextToken must be EMPTY (offset mode emits no token, store.go:817).
		res, err := d.listMemory(ctx, c, coreListRequest{
			Scope: scope, Offset: 1, Limit: 1, Categories: []string{"decision"}, Visibility: "private",
		})
		if err != nil {
			t.Fatalf("listMemory offset mode: %v", err)
		}
		if len(res.Memories) != 1 {
			t.Fatalf("offset mode: got %d records want 1 (page size)", len(res.Memories))
		}
		if res.Total != 3 {
			t.Errorf("offset mode: Total = %d, want 3 (exact matched count, not page length)", res.Total)
		}
		if res.NextToken != "" {
			t.Errorf("offset mode: NextToken = %q, want empty (offset and cursor mode are mutually exclusive)", res.NextToken)
		}
	})

	t.Run("cursor_mode_first_page_issues_next_token", func(t *testing.T) {
		scope := "tool:project:superset-cursor"
		ids := []string{
			"c1000000-0000-0000-0000-000000000001",
			"c1000000-0000-0000-0000-000000000002",
		}
		for i, id := range ids {
			if err := d.st.Upsert(ctx, store.Memory{ID: id, Content: "c", Scope: scope, Owner: "sub-A",
				CreatedAt: base.Add(time.Duration(i) * time.Hour)}, []float32{0.1, 0.2, 0.3}); err != nil {
				t.Fatalf("seed %s: %v", id, err)
			}
			t.Cleanup(func() { _ = d.st.Delete(context.Background(), id, store.Authenticated("sub-A")) })
		}

		// A full first page in cursor mode MUST issue a non-empty NextToken
		// (store.go:865) that resumes to the OTHER record on page 2.
		page1, err := d.listMemory(ctx, c, coreListRequest{Scope: scope, Limit: 1, CursorMode: true})
		if err != nil {
			t.Fatalf("listMemory cursor mode page 1: %v", err)
		}
		if len(page1.Memories) != 1 {
			t.Fatalf("cursor mode page 1: got %d records want 1", len(page1.Memories))
		}
		if page1.NextToken == "" {
			t.Fatal("cursor mode page 1: empty NextToken on a full page; want a resume token")
		}
		page2, err := d.listMemory(ctx, c, coreListRequest{Scope: scope, Limit: 1, Cursor: page1.NextToken, CursorMode: true})
		if err != nil {
			t.Fatalf("listMemory cursor mode page 2: %v", err)
		}
		if len(page2.Memories) != 1 || page2.Memories[0].ID == page1.Memories[0].ID {
			t.Errorf("cursor mode page 2 did not advance: page1=%v page2=%v", page1.Memories, page2.Memories)
		}
	})
}

// TestListMemoryRejectsMalformedCursor pins g0ne.8 at the MCP boundary: a garbage
// cursor surfaces an error rather than silently returning the first page.
func TestListMemoryRejectsMalformedCursor(t *testing.T) {
	d := testDeps(t) // skips without Qdrant
	ctx := authedContext(t, "sub-A")
	if _, err := d.listMemory(ctx, callerFor(ctx, t), coreListRequest{Scope: "tool:project:badcursor", Limit: 1, Cursor: "!!!garbage!!!"}); err == nil {
		t.Error("malformed cursor accepted; want error")
	}
}

// TestListMemoryRejectsBadWindow pins the transport-boundary time parse
// (round-4 MED-6; relocated round-6 MED, grok): the typed core's
// CreatedAfter/CreatedBefore are time.Time and structurally cannot carry an
// invalid RFC3339 string, so the bad-window rejection now lives at
// parseRFC3339 — the exact call the MCP list_memory/search_memory tool
// closures make to build a coreListRequest/coreSearchRequest — not on
// deps.listMemory/deps.searchMemory. No test hands an invalid time string to
// the typed core.
func TestListMemoryRejectsBadWindow(t *testing.T) {
	if _, err := parseRFC3339("nope"); err == nil {
		t.Error("bad created_after accepted; want error")
	}
}

func TestEmbedderFromConfigPassesParamsAndInstructions(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"embedding": []float32{0.1}}},
		})
	}))
	defer srv.Close()

	cfg := &config.Config{
		OpenAI: config.OpenAIConfig{BaseURL: srv.URL, APIKey: "k"},
		Embed: config.EmbedConfig{
			Model:               "m",
			QueryParams:         `{"input_type":"search_query"}`,
			DocumentParams:      `{"input_type":"search_document"}`,
			DocumentInstruction: "passage: ",
		},
	}
	em, err := embedderFromConfig(cfg)
	if err != nil {
		t.Fatalf("embedderFromConfig: %v", err)
	}
	if em == nil {
		t.Fatal("embedderFromConfig returned nil")
	}

	if _, err := em.EmbedQuery(context.Background(), "q"); err != nil {
		t.Fatalf("EmbedQuery: %v", err)
	}
	if got["input_type"] != "search_query" || got["input"] != "q" {
		t.Errorf("EmbedQuery body = %v; want input_type=search_query, input=q", got)
	}

	got = nil
	if _, err := em.Embed(context.Background(), "d"); err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if got["input_type"] != "search_document" || got["input"] != "passage: d" {
		t.Errorf("Embed body = %v; want input_type=search_document, input=%q", got, "passage: d")
	}
}

func TestByIDToolsAcceptShortID(t *testing.T) {
	d := testDeps(t)
	ctx := authedContext(t, "owner-A")
	id, sid, err := d.storeMemory(ctx, callerFor(ctx, t), storeArgs{Content: "hi", Scope: "s", Category: "gotcha", Source: "user-said"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := d.getMemory(ctx, callerFor(ctx, t), idArgs{ID: sid})
	if err != nil || got.ID != id {
		t.Fatalf("get by short id → %q (err %v)", got.ID, err)
	}
	if _, err := d.updateMemory(ctx, callerFor(ctx, t), updateArgs{ID: sid, Content: strp("hi-edited")}); err != nil {
		t.Fatal(err)
	}
	after, err := d.st.Get(context.Background(), id)
	if err != nil || after.ShortID != sid || after.Content != "hi-edited" {
		t.Fatalf("update via short id: content=%q short=%q err=%v", after.Content, after.ShortID, err)
	}
	if _, err := d.setVisibility(ctx, callerFor(ctx, t), setVisibilityArgs{ID: sid, Shared: true}); err != nil {
		t.Fatal(err)
	}
	if err := d.deleteMemory(ctx, callerFor(ctx, t), idArgs{ID: sid}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.st.Get(context.Background(), id); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("record not deleted (err %v)", err)
	}
}

func TestShortIDCrossOwnerVisibility(t *testing.T) {
	d := testDeps(t)
	ctxA := authedContext(t, "owner-A")
	privID, privSid, err := d.storeMemory(ctxA, callerFor(ctxA, t), storeArgs{Content: "secret", Scope: "s", Category: "gotcha", Source: "user-said"})
	if err != nil {
		t.Fatal(err)
	}
	sharedID, sharedSid, err := d.storeMemory(ctxA, callerFor(ctxA, t), storeArgs{Content: "public", Scope: "s", Category: "gotcha", Source: "user-said"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.setVisibility(ctxA, callerFor(ctxA, t), setVisibilityArgs{ID: sharedID, Shared: true}); err != nil {
		t.Fatal(err)
	}
	// owner-B: resolution is owner-agnostic, the read gate governs.
	ctxB := authedContext(t, "owner-B")
	cB := callerFor(ctxB, t)
	// item 4: another owner's private record → ErrNotFound (404, not 403; no leak)
	_, err = d.getMemory(ctxB, cB, idArgs{ID: privSid})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("cross-owner private must be ErrNotFound, got %v", err)
	}
	// no-leak: the gate resolved privSid to another owner's real UUID; the error
	// must echo only the caller-supplied short id, never that resolved UUID.
	if strings.Contains(err.Error(), privID) {
		t.Fatalf("error leaks resolved UUID: %v", err)
	}
	if !strings.Contains(err.Error(), privSid) {
		t.Fatalf("error should echo caller-supplied short id only: %v", err)
	}
	// item 5: another owner's shared record → readable
	if got, err := d.getMemory(ctxB, cB, idArgs{ID: sharedSid}); err != nil || got.ID != sharedID {
		t.Fatalf("cross-owner shared must be readable, got %q err %v", got.ID, err)
	}
	// updateMemory: same no-leak re-wrap as getMemory.
	_, err = d.updateMemory(ctxB, callerFor(ctxB, t), updateArgs{ID: privSid, Content: strp("x")})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("cross-owner update must be ErrNotFound, got %v", err)
	}
	if strings.Contains(err.Error(), privID) {
		t.Fatalf("update error leaks resolved UUID: %v", err)
	}
	if !strings.Contains(err.Error(), privSid) {
		t.Fatalf("update error should echo caller-supplied short id only: %v", err)
	}
	// deleteMemory: same no-leak re-wrap as getMemory.
	err = d.deleteMemory(ctxB, callerFor(ctxB, t), idArgs{ID: privSid})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("cross-owner delete must be ErrNotFound, got %v", err)
	}
	if strings.Contains(err.Error(), privID) {
		t.Fatalf("delete error leaks resolved UUID: %v", err)
	}
	if !strings.Contains(err.Error(), privSid) {
		t.Fatalf("delete error should echo caller-supplied short id only: %v", err)
	}
	// setVisibility: same no-leak re-wrap as getMemory.
	_, err = d.setVisibility(ctxB, callerFor(ctxB, t), setVisibilityArgs{ID: privSid, Shared: true})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("cross-owner set_visibility must be ErrNotFound, got %v", err)
	}
	if strings.Contains(err.Error(), privID) {
		t.Fatalf("set_visibility error leaks resolved UUID: %v", err)
	}
	if !strings.Contains(err.Error(), privSid) {
		t.Fatalf("set_visibility error should echo caller-supplied short id only: %v", err)
	}
	// the record must be unchanged/still present for owner-A after all three attempts.
	after, err := d.st.Get(context.Background(), privID)
	if err != nil || after.Content != "secret" || after.ShortID != privSid {
		t.Fatalf("cross-owner attempts mutated owner-A's record: content=%q short=%q err=%v", after.Content, after.ShortID, err)
	}
}

// TestGetMemoryEnqueuesUsageSignalOnSuccessOnly pins the D-01 counting
// boundary end-to-end: a successful get_memory enqueues exactly the fetched
// point id; a denied/ErrNotFound get_memory enqueues nothing.
// TestGetMemoryNeverSurfacesEmbedderIdentity is a D-06 regression guard
// (review round-1 HIGH blocker): get_memory's AddTool handler emits the raw
// store.Memory getMemory returns as its structured MCP result — one of the
// three verbatim full-response wire paths. With d.embedderIdentity set to a
// sentinel, the record round-trips through a real storeMemory + getMemory
// call, and the marshaled result must carry no embedder_identity key (the
// json:"-" struct tag on store.Memory.EmbedderIdentity is what makes this
// structurally impossible, not handler-level filtering).
func TestGetMemoryNeverSurfacesEmbedderIdentity(t *testing.T) {
	d := testDeps(t)
	d.embedderIdentity = "v1:deadbeefdeadbeef"
	ctx := authedContext(t, "sub-identity-wire")
	scope := "iso-test:project:identity-wire"
	defer func() {
		cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(context.Background(), scope, store.Authenticated("sub-identity-wire")))
	}()

	id, _, err := d.storeMemory(ctx, callerFor(ctx, t), storeArgs{Content: "wire check", Scope: scope, Source: "user-said", Category: "gotcha"})
	if err != nil {
		t.Fatalf("storeMemory: %v", err)
	}
	got, err := d.getMemory(ctx, callerFor(ctx, t), idArgs{ID: id})
	if err != nil {
		t.Fatalf("getMemory: %v", err)
	}
	if got.EmbedderIdentity != "v1:deadbeefdeadbeef" {
		t.Fatalf("sanity: persisted identity = %q, want sentinel (store layer must have stamped it)", got.EmbedderIdentity)
	}

	wire, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal get_memory structured result: %v", err)
	}
	if strings.Contains(string(wire), "embedder_identity") || strings.Contains(string(wire), "deadbeefdeadbeef") {
		t.Fatalf("get_memory leaked embedder identity onto the wire: %s", wire)
	}
}

func TestGetMemoryEnqueuesUsageSignalOnSuccessOnly(t *testing.T) {
	d, rec := testDepsWithUsageQueue(t, 2, 16)
	ctxA := authedContext(t, "owner-A")
	id, _, err := d.storeMemory(ctxA, callerFor(ctxA, t), storeArgs{Content: "hi", Scope: "s", Category: "gotcha", Source: "user-said"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.getMemory(ctxA, callerFor(ctxA, t), idArgs{ID: id}); err != nil {
		t.Fatal(err)
	}
	d.usageQueue.Wait()
	if got := rec.recorded(); len(got) != 1 || got[0] != id {
		t.Fatalf("successful get enqueued %v, want exactly [%s]", got, id)
	}

	// Denied: owner-B fetching owner-A's private record must not enqueue.
	ctxB := authedContext(t, "owner-B")
	if _, err := d.getMemory(ctxB, callerFor(ctxB, t), idArgs{ID: id}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("cross-owner get must be ErrNotFound, got %v", err)
	}
	d.usageQueue.Wait()
	if got := rec.recorded(); len(got) != 1 {
		t.Fatalf("denied get must not enqueue; recorded = %v", got)
	}
}

// TestSearchListMemoryDoNotEnqueueUsageSignal pins the D-02 hard invariant:
// result-set membership in search_memory/list_memory/list_scheduled never
// increments the usage-signal payload counter, even when the same record that
// was just get-fetched (and did enqueue) appears in the result set.
func TestSearchListMemoryDoNotEnqueueUsageSignal(t *testing.T) {
	d, rec := testDepsWithUsageQueue(t, 2, 16)
	ctx := authedContext(t, "owner-A")
	scope := "usage-d02-scope"
	if _, _, err := d.storeMemory(ctx, callerFor(ctx, t), storeArgs{Content: "hi", Scope: scope, Category: "gotcha", Source: "user-said"}); err != nil {
		t.Fatal(err)
	}
	future := timeNow().Add(time.Hour).Format(time.RFC3339)
	if _, _, err := d.scheduleMemory(ctx, callerFor(ctx, t), scheduleArgs{
		storeArgs: storeArgs{Content: "later", Scope: scope, Category: "gotcha", Source: "user-said"},
		NotBefore: future,
	}); err != nil {
		t.Fatal(err)
	}

	c := callerFor(ctx, t)
	// deps.searchMemory applies no internal k default (round-4 finding-7): the
	// core rejects K==0, so this direct call supplies the MCP lane's k=8.
	if _, err := d.searchMemory(ctx, c, coreSearchRequest{Query: "hi", Scope: scope, K: 8}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.listMemory(ctx, c, coreListRequest{Scope: scope}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.listScheduled(ctx, c, listScheduledArgs{Scope: scope}); err != nil {
		t.Fatal(err)
	}
	d.usageQueue.Wait()
	if got := rec.recorded(); len(got) != 0 {
		t.Fatalf("search/list/list_scheduled must never enqueue usage signals (D-02); recorded = %v", got)
	}
}

// TestUpdateMemoryIncrementsAccessCountOnceNoAsyncEnqueue pins D-04: the
// update path bumps access_count exactly once, synchronously, free inside
// store.Update — it does NOT also fire a usageQueue enqueue (that would
// double-count a single update as both a "get" and an "update").
func TestUpdateMemoryIncrementsAccessCountOnceNoAsyncEnqueue(t *testing.T) {
	d, rec := testDepsWithUsageQueue(t, 2, 16)
	ctx := authedContext(t, "owner-A")
	id, _, err := d.storeMemory(ctx, callerFor(ctx, t), storeArgs{Content: "v1", Scope: "s", Category: "gotcha", Source: "user-said"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.updateMemory(ctx, callerFor(ctx, t), updateArgs{ID: id, Content: strp("v2")}); err != nil {
		t.Fatal(err)
	}
	after, err := d.st.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if after.AccessCount != 1 {
		t.Fatalf("access_count after one update = %d, want 1", after.AccessCount)
	}
	d.usageQueue.Wait()
	if got := rec.recorded(); len(got) != 0 {
		t.Fatalf("update_memory must not enqueue an async usage signal (free bump only); recorded = %v", got)
	}
}

// TestBuildUsageQueueConfigGate pins D-09: ENGRAM_USAGE_SIGNALS (parsed at
// point-of-use as cfg.Usage.Signals) false/unparseable disables the queue
// (nil, zero read-path counter writes); true builds and starts a live queue.
// A nil usageQueue must still let get_memory return the record (nil-safe
// no-op call site).
func TestBuildUsageQueueConfigGate(t *testing.T) {
	if q := buildUsageQueue(&config.Config{Usage: config.UsageConfig{Signals: "false"}}, nil, nil); q != nil {
		t.Fatalf("Signals=false must disable the queue, got %v", q)
	}
	if q := buildUsageQueue(&config.Config{Usage: config.UsageConfig{Signals: "not-a-bool"}}, nil, nil); q != nil {
		t.Fatalf("unparseable Signals must disable the queue, got %v", q)
	}

	d, st := testDepsWithStore(t)
	q := buildUsageQueue(&config.Config{Usage: config.UsageConfig{Signals: "true"}}, st, nil)
	if q == nil {
		t.Fatal("Signals=true must build a live queue")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		q.Shutdown(ctx)
	})

	// Nil-safe call site: deps.usageQueue is nil by default (testDeps), and
	// get_memory must still return the record with zero counter writes.
	ctx := authedContext(t, "owner-A")
	id, _, err := d.storeMemory(ctx, callerFor(ctx, t), storeArgs{Content: "hi", Scope: "s", Category: "gotcha", Source: "user-said"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := d.getMemory(ctx, callerFor(ctx, t), idArgs{ID: id})
	if err != nil || got.ID != id {
		t.Fatalf("get with disabled usage queue: got %q err %v", got.ID, err)
	}
	if got.AccessCount != 0 {
		t.Fatalf("disabled usage queue must perform zero counter writes; access_count = %d", got.AccessCount)
	}
}

// TestStoreDiscoveryArgsIDSchemaAdvertisesShortID pins storeDiscoveryArgs.ID's
// jsonschema tag so a future silent drop of the short_id wording is caught —
// exactly how #303 was originally filed. Verification-only (#303): the field
// has already advertised short_id support since commit 92a6f610 (PR #288);
// this test asserts the existing wording, no production code change.
func TestStoreDiscoveryArgsIDSchemaAdvertisesShortID(t *testing.T) {
	f, ok := reflect.TypeOf(storeDiscoveryArgs{}).FieldByName("ID")
	if !ok {
		t.Fatal("storeDiscoveryArgs has no ID field")
	}
	tag := f.Tag.Get("jsonschema")
	if !strings.Contains(tag, "short_id") {
		t.Fatalf("storeDiscoveryArgs.ID jsonschema tag = %q, want it to mention short_id", tag)
	}
}
