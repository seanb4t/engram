// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package storetest

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/store"
	"google.golang.org/grpc"
)

func TestMain(m *testing.M) {
	os.Exit(Run(m))
}

// testCollectionPrefix namespaces this package's OWN integration-test
// Qdrant collection names on the shared CI Qdrant instance — disjoint from
// store_, server_, e2e_, and retrievaleval_.
const testCollectionPrefix = "storetest_"

// testCollection returns name namespaced into this package's collection
// space on the shared test Qdrant instance.
func testCollection(name string) string {
	return testCollectionPrefix + name
}

// newTestStore is the prefix-asserting construction seam for every test
// Store built in this package's own tests — the ONLY function here that
// calls store.New.
func newTestStore(t testing.TB, c *qdrant.Client, name string) *store.Store {
	t.Helper()
	if !strings.HasPrefix(name, testCollectionPrefix) {
		t.Fatalf("collection name %q does not carry this package's prefix %q: route it through testCollection()", name, testCollectionPrefix)
	}
	return store.New(c, name)
}

func TestRecvLimitIsFourMiB(t *testing.T) {
	if RecvLimit != 4<<20 {
		t.Errorf("RecvLimit = %d, want %d", RecvLimit, 4<<20)
	}
	if RecvLimit != 4194304 {
		t.Errorf("RecvLimit = %d, want 4194304", RecvLimit)
	}
}

// TestRequireQdrant mirrors internal/store's own TestRequireQdrant: the
// exact six-row table for the fail-closed parser.
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
			got, err := RequireQdrant()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("RequireQdrant() with %q = (%v, nil), want a non-nil error (must not coerce to false)", tc.val, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("RequireQdrant() with %q: unexpected error: %v", tc.val, err)
			}
			if got != tc.want {
				t.Errorf("RequireQdrant() with %q = %v, want %v", tc.val, got, tc.want)
			}
		})
	}
}

func TestSplitAddr(t *testing.T) {
	cases := []struct {
		name     string
		addr     string
		wantHost string
		wantPort int
		wantErr  bool
	}{
		{name: "ok", addr: "localhost:6334", wantHost: "localhost", wantPort: 6334},
		{name: "port_one", addr: "127.0.0.1:1", wantHost: "127.0.0.1", wantPort: 1},
		{name: "empty", addr: "", wantErr: true},
		{name: "no_port", addr: "localhost", wantErr: true},
		{name: "port_zero", addr: "localhost:0", wantErr: true},
		{name: "port_negative", addr: "localhost:-1", wantErr: true},
		{name: "port_non_numeric", addr: "localhost:x", wantErr: true},
		{name: "port_too_large", addr: "localhost:65536", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			host, port, err := splitAddr(tc.addr)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("splitAddr(%q) = (%q, %d, nil), want an error", tc.addr, host, port)
				}
				return
			}
			if err != nil {
				t.Fatalf("splitAddr(%q): unexpected error: %v", tc.addr, err)
			}
			if host != tc.wantHost || port != tc.wantPort {
				t.Errorf("splitAddr(%q) = (%q, %d), want (%q, %d)", tc.addr, host, port, tc.wantHost, tc.wantPort)
			}
		})
	}
}

func TestDialOptions(t *testing.T) {
	t.Run("non-positive limit rejected", func(t *testing.T) {
		for _, limit := range []int{0, -1} {
			if _, err := dialOptions(limit, nil); err == nil {
				t.Errorf("dialOptions(%d, nil) = nil error, want an error", limit)
			}
		}
	})
	t.Run("limit of one accepted with a single option", func(t *testing.T) {
		got, err := dialOptions(1, nil)
		if err != nil {
			t.Fatalf("dialOptions(1, nil): unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("dialOptions(1, nil): len(got) = %d, want 1", len(got))
		}
	})
	t.Run("caller options kept in order, named limit appended last", func(t *testing.T) {
		opts := []grpc.DialOption{grpc.WithUserAgent("storetest-a"), grpc.WithAuthority("storetest-b")}
		got, err := dialOptions(RecvLimit, opts)
		if err != nil {
			t.Fatalf("dialOptions: unexpected error: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("len(got) = %d, want 3", len(got))
		}
		if got[0] != opts[0] {
			t.Errorf("got[0] != opts[0]: caller option identity/order not preserved")
		}
		if got[1] != opts[1] {
			t.Errorf("got[1] != opts[1]: caller option identity/order not preserved")
		}
		if got[2] == opts[0] || got[2] == opts[1] {
			t.Errorf("got[2] is one of the caller's sentinel options, want the named receive-limit option appended last")
		}
	})
}

// TestSkipOrFailNoQdrantSkipsWhenNotRequired proves SkipOrFailNoQdrant
// preserves the skip-not-fail behavior local dev without Docker relies on,
// in both the default configuration and with the package's ignore-require
// flag set.
func TestSkipOrFailNoQdrantSkipsWhenNotRequired(t *testing.T) {
	t.Run("not required", func(t *testing.T) {
		t.Setenv("ENGRAM_REQUIRE_QDRANT", "")
		passed := t.Run("inner", func(t *testing.T) {
			SkipOrFailNoQdrant(t)
			t.Fatal("SkipOrFailNoQdrant did not skip; reached past the skip call")
		})
		if !passed {
			t.Fatal("SkipOrFailNoQdrant failed instead of skipping with ENGRAM_REQUIRE_QDRANT unset")
		}
	})
	t.Run("ignore-require flag set, required also set", func(t *testing.T) {
		saved := ignoreRequire
		t.Cleanup(func() { ignoreRequire = saved })
		ignoreRequire = true
		t.Setenv("ENGRAM_REQUIRE_QDRANT", "1")
		passed := t.Run("inner", func(t *testing.T) {
			SkipOrFailNoQdrant(t)
			t.Fatal("SkipOrFailNoQdrant did not skip; reached past the skip call")
		})
		if !passed {
			t.Fatal("SkipOrFailNoQdrant failed instead of skipping when the package's ignore-require flag is set")
		}
	})
}

// TestDialRoundTrip proves Dial's client can actually reach Qdrant and that
// a store built on top of it round-trips a real write. No subtest opts into
// parallel execution.
func TestDialRoundTrip(t *testing.T) {
	c := Dial(t, RecvLimit)
	ctx := context.Background()
	name := testCollection("roundtrip_" + uuid.NewString())
	st := newTestStore(t, c, name)
	if err := st.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}
	t.Cleanup(func() {
		if err := c.DeleteCollection(ctx, name); err != nil {
			t.Errorf("DeleteCollection(%q): %v", name, err)
		}
	})

	owner := "storetest-roundtrip-owner-" + uuid.NewString()
	m := store.Memory{
		ID:        uuid.NewString(),
		Content:   "storetest round trip content",
		Scope:     "storetest-roundtrip:project:x",
		Owner:     owner,
		CreatedAt: time.Now().UTC(),
	}
	if err := st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := st.Get(ctx, m.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Content != m.Content {
		t.Errorf("Get().Content = %q, want %q", got.Content, m.Content)
	}

	scopes, _, err := st.ListScopes(ctx, store.Authenticated(owner))
	if err != nil {
		t.Fatalf("ListScopes: %v", err)
	}
	var count uint64
	for _, sc := range scopes {
		if sc.Scope == m.Scope {
			count = sc.Count
		}
	}
	if count != 1 {
		t.Errorf("ListScopes count for scope %q = %d, want 1", m.Scope, count)
	}
}
