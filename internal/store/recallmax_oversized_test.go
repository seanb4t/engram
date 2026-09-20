// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves REQ-list-limit-contract-decided's store-level backstop
// (04-CONTEXT.md D-10): every recall entry point refuses a count above
// store.MaxRecallLimit before issuing any Qdrant call, a count at the
// maximum succeeds, every zero-count default is unchanged, and the cursor
// page's former silent clamp is gone — replaced by the same refusal, never
// a quietly shrunk page.
package store_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
	"google.golang.org/grpc"
)

// recallMaxRecorder is a mutex-guarded recording grpc.UnaryClientInterceptor
// counting every intercepted gRPC call since the last reset — this file's
// tests only ever drive List/ListScheduled/Search/SearchDiscovery after
// reset, so a zero count is a strong "no Qdrant call at all" assertion; it
// does not need to distinguish call shapes the way other oversized test
// files' recorders do.
type recallMaxRecorder struct {
	mu    sync.Mutex
	count int
}

func (r *recallMaxRecorder) intercept(
	ctx context.Context, method string, req, reply any,
	cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
) error {
	err := invoker(ctx, method, req, reply, cc, opts...)
	r.mu.Lock()
	r.count++
	r.mu.Unlock()
	return err
}

func (r *recallMaxRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.count = 0
}

func (r *recallMaxRecorder) snapshot() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}

// assertOverMaxErr asserts err satisfies errors.Is(err, store.ErrInvalidArgument),
// that its text names field and store.MaxRecallLimit by number, and that it
// does NOT contain the rejected (over-maximum) value.
func assertOverMaxErr(t *testing.T, err error, field string, rejected uint64) {
	t.Helper()
	if !errors.Is(err, store.ErrInvalidArgument) {
		t.Fatalf("errors.Is(err, store.ErrInvalidArgument) = false; err = %v", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, field) {
		t.Errorf("error %q does not name field %q", msg, field)
	}
	maxStr := strconv.Itoa(store.MaxRecallLimit)
	if !strings.Contains(msg, maxStr) {
		t.Errorf("error %q does not name the maximum %s", msg, maxStr)
	}
	rejectedStr := strconv.FormatUint(rejected, 10)
	if strings.Contains(msg, rejectedStr) {
		t.Errorf("error %q contains the rejected value %s, want it omitted", msg, rejectedStr)
	}
}

// TestStoreRejectsOverMaximumCount proves D-10's backstop across List (in
// both offset and cursor mode), ListScheduled, Search and SearchDiscovery: a
// count at store.MaxRecallLimit succeeds; one above it is refused with
// store.ErrInvalidArgument before any Qdrant call, naming the field and the
// maximum but never the rejected value; every zero-count default (twenty
// for ListScheduled and for a cursor page, the resolved maximum for an
// offset list, a rejection for the shared rerank helper) is unchanged; and
// a cursor-mode List above the maximum is refused rather than quietly
// shrunk to it.
func TestStoreRejectsOverMaximumCount(t *testing.T) {
	rec := &recallMaxRecorder{}
	c := storetest.Dial(t, storetest.RecvLimit, grpc.WithChainUnaryInterceptor(rec.intercept))
	name := store.PrefixedTestCollection("oversized_recallmax_" + uuid.NewString())
	st := store.NewTestStore(t, c, name)
	ctx := context.Background()
	vec := []float32{0.1, 0.2, 0.3}
	if err := st.EnsureCollection(ctx, uint64(len(vec))); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}
	t.Cleanup(func() {
		if err := c.DeleteCollection(ctx, name); err != nil {
			t.Errorf("DeleteCollection(%q): %v", name, err)
		}
	})

	scope := "storetest-recallmax:project:" + uuid.NewString()
	ownerStr := "storetest-owner-" + uuid.NewString()
	owner := store.Authenticated(ownerStr)

	entryPoints := []struct {
		name   string
		field  string
		invoke func(n uint64) error
	}{
		{
			name: "List", field: "limit",
			invoke: func(n uint64) error {
				_, _, _, err := st.List(ctx, scope, owner, store.ListOptions{Limit: n})
				return err
			},
		},
		{
			name: "ListScheduled", field: "limit",
			invoke: func(n uint64) error {
				_, err := st.ListScheduled(ctx, scope, owner, store.ScheduledPending, store.ListOptions{Limit: n})
				return err
			},
		},
		{
			name: "Search", field: "k",
			invoke: func(n uint64) error {
				_, err := st.Search(ctx, scope, owner, vec, n, store.SearchOptions{})
				return err
			},
		},
		{
			name: "SearchDiscovery", field: "k",
			invoke: func(n uint64) error {
				_, err := st.SearchDiscovery(ctx, scope, "", owner, vec, n)
				return err
			},
		},
	}

	for _, ep := range entryPoints {
		t.Run(ep.name+"/at_maximum", func(t *testing.T) {
			rec.reset()
			if err := ep.invoke(store.MaxRecallLimit); err != nil {
				t.Fatalf("%s: at-maximum call: %v", ep.name, err)
			}
		})
		t.Run(ep.name+"/one_above_maximum", func(t *testing.T) {
			rec.reset()
			err := ep.invoke(store.MaxRecallLimit + 1)
			assertOverMaxErr(t, err, ep.field, store.MaxRecallLimit+1)
			if calls := rec.snapshot(); calls != 0 {
				t.Errorf("%s: one-above-maximum call recorded %d Qdrant call(s), want 0", ep.name, calls)
			}
		})
	}

	// Zero-count defaults: the new backstop must never reject a zero count
	// (0 <= MaxRecallLimit), so each entry point's own pre-existing default
	// behavior is unchanged.
	t.Run("List/zero_default_offset", func(t *testing.T) {
		if _, _, _, err := st.List(ctx, scope, owner, store.ListOptions{Limit: 0}); err != nil {
			t.Fatalf("zero-limit offset List: %v", err)
		}
	})
	t.Run("List/zero_default_cursor", func(t *testing.T) {
		if _, _, _, err := st.List(ctx, scope, owner, store.ListOptions{Limit: 0, CursorMode: true}); err != nil {
			t.Fatalf("zero-limit cursor List: %v", err)
		}
	})
	t.Run("ListScheduled/zero_default", func(t *testing.T) {
		if _, err := st.ListScheduled(ctx, scope, owner, store.ScheduledPending, store.ListOptions{Limit: 0}); err != nil {
			t.Fatalf("zero-limit ListScheduled: %v", err)
		}
	})
	t.Run("SearchReranked/zero_rejected", func(t *testing.T) {
		if _, err := st.SearchReranked(ctx, scope, owner, "", vec, 0, store.SearchOptions{}); !errors.Is(err, store.ErrInvalidArgument) {
			t.Fatalf("SearchReranked(k=0): errors.Is(err, store.ErrInvalidArgument) = false; err = %v", err)
		}
	})

	// The retired silent clamp: a cursor-mode List above the maximum used to
	// be quietly shrunk to it; it must now be refused instead, with its own
	// named subtest, so the caller can tell "too many asked for" apart from
	// "that is all there was".
	t.Run("List/cursor_mode_refused", func(t *testing.T) {
		rec.reset()
		_, _, _, err := st.List(ctx, scope, owner, store.ListOptions{Limit: store.MaxRecallLimit + 1, CursorMode: true})
		assertOverMaxErr(t, err, "limit", store.MaxRecallLimit+1)
		if calls := rec.snapshot(); calls != 0 {
			t.Errorf("cursor-mode over-maximum List recorded %d Qdrant call(s), want 0", calls)
		}
	})
}
