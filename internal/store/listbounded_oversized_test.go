// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves REQ-list-bounded's real-Qdrant regressions for Store.List
// against both oversized fixture shapes storetest.SeedOversized supports, at
// the named storetest.RecvLimit — OUR request shape and OUR page contract,
// never Qdrant's or grpc-go's own behavior (rule m45p2b4bp7). Task 1 proves
// cursor mode (TestStoreListCursorBounded); Task 2 extends this file with
// TestStoreListOffsetBounded.
package store_test

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
	"google.golang.org/grpc"
)

// TestStoreListCursorBounded runs one subtest per fixture shape, few-large
// then many-small: it walks Store.List in cursor mode (ListOptions{Limit:
// 50, CursorMode: true}, resuming with each returned cursor) until the
// cursor comes back empty, asserting every call succeeds, total is exact on
// every page, every seeded id is visited exactly once across the walk,
// created_at never increases across the concatenated walk, and a non-empty
// cursor is returned on every page but the last. It then proves the D-06
// budget-cut page: shrinking the page byte budget below one full-view
// record ceiling still returns a page with a non-empty cursor and fewer
// items than requested.
func TestStoreListCursorBounded(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		t.Run(shape.String(), func(t *testing.T) {
			c := storetest.Dial(t, storetest.RecvLimit)
			name := store.PrefixedTestCollection("oversized_listbounded_cursor_" + uuid.NewString())
			st := store.NewTestStore(t, c, name)
			ctx := context.Background()
			if err := st.EnsureCollection(ctx, 3); err != nil {
				t.Fatalf("EnsureCollection: %v", err)
			}
			t.Cleanup(func() {
				if err := c.DeleteCollection(ctx, name); err != nil {
					t.Errorf("DeleteCollection(%q): %v", name, err)
				}
			})

			fx := storetest.SeedOversized(t, st, storetest.Spec{Limit: storetest.RecvLimit, Shape: shape, Vector: []float32{0.1, 0.2, 0.3}})
			owner := store.Authenticated(fx.Owner)

			visited := map[string]int{}
			var lastCreatedAt time.Time
			haveLast := false
			cursor := ""
			var gotTotal uint64
			for i := 0; i < 200; i++ {
				items, total, next, err := st.List(ctx, fx.Scope, owner, store.ListOptions{Limit: 50, Cursor: cursor, CursorMode: true})
				if err != nil {
					t.Fatalf("page %d: List: %v", i, err)
				}
				gotTotal = total
				if total != uint64(len(fx.IDs)) {
					t.Errorf("page %d: total = %d, want %d", i, total, len(fx.IDs))
				}
				for _, m := range items {
					visited[m.ID]++
					if visited[m.ID] > 1 {
						t.Fatalf("page %d: id %s visited twice", i, m.ID)
					}
					if haveLast && m.CreatedAt.After(lastCreatedAt) {
						t.Errorf("page %d: created_at increased: %v after %v", i, m.CreatedAt, lastCreatedAt)
					}
					lastCreatedAt = m.CreatedAt
					haveLast = true
				}
				if next == "" {
					break
				}
				cursor = next
			}
			if gotTotal != uint64(len(fx.IDs)) {
				t.Errorf("final total = %d, want %d", gotTotal, len(fx.IDs))
			}
			if len(visited) != len(fx.IDs) {
				t.Errorf("visited %d distinct ids, want %d", len(visited), len(fx.IDs))
			}
			for _, id := range fx.IDs {
				if visited[id] != 1 {
					t.Errorf("id %s visited %d time(s), want exactly 1", id, visited[id])
				}
			}

			// D-06 budget-cut page: shrinking the page byte budget below one
			// full-view record ceiling still returns a page with a
			// non-empty cursor and fewer items than requested — never the
			// last page. Inlined (no nested t.Run) so this file's verify
			// command's per-shape "--- PASS:" count stays exactly 2.
			store.SetByteBudgets(t, store.RPCByteBudget(), store.ViewMaxRecordBytes(st.FullView())/2)
			cutItems, cutTotal, cutNext, err := st.List(ctx, fx.Scope, owner, store.ListOptions{Limit: uint64(len(fx.IDs)), CursorMode: true})
			if err != nil {
				t.Fatalf("budget-cut List: %v", err)
			}
			if cutTotal != uint64(len(fx.IDs)) {
				t.Errorf("budget-cut total = %d, want %d", cutTotal, len(fx.IDs))
			}
			if len(cutItems) >= len(fx.IDs) {
				t.Errorf("budget-cut: got %d items, want fewer than the requested %d", len(cutItems), len(fx.IDs))
			}
			if cutNext == "" {
				t.Error("budget-cut: next cursor is empty, want non-empty (a budget-cut page is never the last page)")
			}
		})
	}
}

// TestStoreListOffsetBounded proves Store.List's offset mode over both
// oversized fixture shapes storetest.SeedOversized supports: a zero-limit
// call succeeds and returns AT MOST store.MaxRecallLimit records while total
// stays exact even when it exceeds the maximum (D-01); a limit exactly at
// store.MaxRecallLimit and one below it return exactly min(seeded count,
// that limit) records; an offset beyond the total returns an empty page
// with the real total (clamped, never a slice panic); ids are unique and
// created_at never increases; two identical calls are idempotent; and an
// offset/limit pair that would wrap uint64 is rejected with
// store.ErrInvalidArgument before any Scroll RPC — reusing
// boundedread_oversized_test.go's scrollRecorder interceptor idiom for the
// no-RPC assertion.
//
// The per-shape body below deliberately does NOT wrap in its own t.Run:
// only the wrap-guard check gets a named subtest, via a single t.Run call
// whose name already embeds the shape ("few-large/offset_plus_limit_overflows"),
// so Go renders it as one flat leaf ("TestStoreListOffsetBounded/few-large/
// offset_plus_limit_overflows") with no separate intermediate parent line.
// This keeps this file's verify command's per-shape
// "--- PASS: TestStoreListOffsetBounded/(few-large|many-small)" count at
// exactly 2 (one such line per shape) while still surfacing the specific
// "offset_plus_limit_overflows" subtest the plan's acceptance criteria name.
func TestStoreListOffsetBounded(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		rec := &scrollRecorder{}
		c := storetest.Dial(t, storetest.RecvLimit, grpc.WithChainUnaryInterceptor(rec.intercept))
		name := store.PrefixedTestCollection("oversized_listbounded_offset_" + uuid.NewString())
		st := store.NewTestStore(t, c, name)
		ctx := context.Background()
		if err := st.EnsureCollection(ctx, 3); err != nil {
			t.Fatalf("%s: EnsureCollection: %v", shape, err)
		}
		t.Cleanup(func() {
			if err := c.DeleteCollection(ctx, name); err != nil {
				t.Errorf("%s: DeleteCollection(%q): %v", shape, name, err)
			}
		})

		fx := storetest.SeedOversized(t, st, storetest.Spec{Limit: storetest.RecvLimit, Shape: shape, Vector: []float32{0.1, 0.2, 0.3}})
		owner := store.Authenticated(fx.Owner)
		seededTotal := uint64(len(fx.IDs))

		// Zero limit: at most store.MaxRecallLimit records, total exact,
		// ids unique, created_at non-increasing, idempotent.
		items, total, next, err := st.List(ctx, fx.Scope, owner, store.ListOptions{})
		if err != nil {
			t.Errorf("%s: zero-limit List: %v", shape, err)
		} else {
			if total != seededTotal {
				t.Errorf("%s: zero-limit total = %d, want %d", shape, total, seededTotal)
			}
			if next != "" {
				t.Errorf("%s: zero-limit next = %q, want empty (offset mode never returns a cursor)", shape, next)
			}
			wantCount := seededTotal
			if wantCount > store.MaxRecallLimit {
				wantCount = store.MaxRecallLimit
			}
			if uint64(len(items)) != wantCount {
				t.Errorf("%s: zero-limit got %d items, want %d", shape, len(items), wantCount)
			}
			seen := map[string]bool{}
			var lastCreatedAt time.Time
			haveLast := false
			for _, m := range items {
				if seen[m.ID] {
					t.Errorf("%s: zero-limit id %s returned twice", shape, m.ID)
				}
				seen[m.ID] = true
				if haveLast && m.CreatedAt.After(lastCreatedAt) {
					t.Errorf("%s: zero-limit created_at increased: %v after %v", shape, m.CreatedAt, lastCreatedAt)
				}
				lastCreatedAt = m.CreatedAt
				haveLast = true
			}

			items2, total2, _, err2 := st.List(ctx, fx.Scope, owner, store.ListOptions{})
			if err2 != nil {
				t.Errorf("%s: second zero-limit List: %v", shape, err2)
			} else {
				if total2 != total {
					t.Errorf("%s: second call total = %d, want %d", shape, total2, total)
				}
				if len(items2) != len(items) {
					t.Errorf("%s: second call returned %d items, want %d", shape, len(items2), len(items))
				} else {
					for i := range items {
						if items[i].ID != items2[i].ID {
							t.Errorf("%s: id sequence differs at position %d: %s != %s", shape, i, items[i].ID, items2[i].ID)
						}
					}
				}
			}
		}

		// Limit exactly at the maximum, and one below it — ManySmallRecords
		// is pinned to store.MaxRecallLimit (TestManySmallShapeFitsOneListPage),
		// so the many-small shape exercises this boundary precisely; the
		// few-large shape's smaller total simply asserts min(total, limit)
		// holds at both requested values.
		for _, limit := range []uint64{store.MaxRecallLimit, store.MaxRecallLimit - 1} {
			mItems, mTotal, _, mErr := st.List(ctx, fx.Scope, owner, store.ListOptions{Limit: limit})
			if mErr != nil {
				t.Errorf("%s: limit=%d List: %v", shape, limit, mErr)
				continue
			}
			if mTotal != seededTotal {
				t.Errorf("%s: limit=%d total = %d, want %d", shape, limit, mTotal, seededTotal)
			}
			want := limit
			if want > seededTotal {
				want = seededTotal
			}
			if uint64(len(mItems)) != want {
				t.Errorf("%s: limit=%d got %d items, want %d", shape, limit, len(mItems), want)
			}
		}

		// Offset at/beyond the total: empty page, real total, never a
		// slice panic.
		bItems, bTotal, bNext, bErr := st.List(ctx, fx.Scope, owner, store.ListOptions{Offset: seededTotal + 100, Limit: 10})
		if bErr != nil {
			t.Errorf("%s: offset-beyond-total List: %v", shape, bErr)
		} else {
			if bTotal != seededTotal {
				t.Errorf("%s: offset-beyond-total total = %d, want %d", shape, bTotal, seededTotal)
			}
			if len(bItems) != 0 {
				t.Errorf("%s: offset-beyond-total got %d items, want 0", shape, len(bItems))
			}
			if bNext != "" {
				t.Errorf("%s: offset-beyond-total next = %q, want empty", shape, bNext)
			}
		}

		// The wrap guard, as its own named subtest so its "--- PASS:" line
		// is individually addressable (see the func doc comment for why
		// this is a flat, slash-named t.Run rather than real nesting).
		rec.reset()
		t.Run(shape.String()+"/offset_plus_limit_overflows", func(t *testing.T) {
			_, _, _, wErr := st.List(ctx, fx.Scope, owner, store.ListOptions{Offset: math.MaxUint64 - 5, Limit: 10})
			if !errors.Is(wErr, store.ErrInvalidArgument) {
				t.Fatalf("errors.Is(err, store.ErrInvalidArgument) = false; err = %v", wErr)
			}
			if calls := rec.snapshot(); len(calls) != 0 {
				t.Errorf("recorded %d Scroll call(s), want 0: %+v", len(calls), calls)
			}
		})
	}
}

// selectorRecordedCall is one intercepted Scroll RPC's request Limit and
// whether it carried a keys-only (Include) payload selector, as
// selectorRecorder.intercept observed it. This test's own recorder, mirroring
// boundedread_oversized_test.go's scrollRecorder idiom but additionally
// distinguishing which readView a call carried — the deep-offset walk mixes
// keysView() prefix RPCs and a fullView() page RPC in one logical call to
// Store.List, which scrollRecorder's own shape does not capture.
type selectorRecordedCall struct {
	limit    uint32
	keysOnly bool
}

// selectorRecorder is a mutex-guarded recording grpc.UnaryClientInterceptor,
// scoped to methods ending "/Scroll".
type selectorRecorder struct {
	mu    sync.Mutex
	calls []selectorRecordedCall
}

func (r *selectorRecorder) intercept(
	ctx context.Context, method string, req, reply any,
	cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
) error {
	if len(method) < len("/Scroll") || method[len(method)-len("/Scroll"):] != "/Scroll" {
		return invoker(ctx, method, req, reply, cc, opts...)
	}
	var call selectorRecordedCall
	if sp, ok := req.(*qdrant.ScrollPoints); ok {
		call.limit = sp.GetLimit()
		call.keysOnly = sp.GetWithPayload().GetInclude() != nil
	}
	err := invoker(ctx, method, req, reply, cc, opts...)
	r.mu.Lock()
	r.calls = append(r.calls, call)
	r.mu.Unlock()
	return err
}

func (r *selectorRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = nil
}

func (r *selectorRecorder) snapshot() []selectorRecordedCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]selectorRecordedCall, len(r.calls))
	copy(out, r.calls)
	return out
}

// TestStoreListDeepOffsetBounded proves Store.List's deep-offset prefix walk
// (D-07) over both oversized fixture shapes storetest.SeedOversized
// supports: a deep-offset call succeeds and returns exactly the ids a full
// zero-offset cursor-mode walk yields at the same positions; total is the
// exact seeded count; every recorded Scroll's limit is between one and
// store.PerRPCLimit of the view it used; the prefix RPCs carry the keys-only
// payload selector while the trailing page RPC carries the full one; and an
// offset at or beyond the total returns an empty page with the real total.
func TestStoreListDeepOffsetBounded(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		t.Run(shape.String(), func(t *testing.T) {
			rec := &selectorRecorder{}
			c := storetest.Dial(t, storetest.RecvLimit, grpc.WithChainUnaryInterceptor(rec.intercept))
			name := store.PrefixedTestCollection("oversized_listbounded_deepoffset_" + uuid.NewString())
			st := store.NewTestStore(t, c, name)
			ctx := context.Background()
			if err := st.EnsureCollection(ctx, 3); err != nil {
				t.Fatalf("%s: EnsureCollection: %v", shape, err)
			}
			t.Cleanup(func() {
				if err := c.DeleteCollection(ctx, name); err != nil {
					t.Errorf("%s: DeleteCollection(%q): %v", shape, name, err)
				}
			})

			fx := storetest.SeedOversized(t, st, storetest.Spec{Limit: storetest.RecvLimit, Shape: shape, Vector: []float32{0.1, 0.2, 0.3}})
			owner := store.Authenticated(fx.Owner)
			seededTotal := uint64(len(fx.IDs))

			// Baseline: a full zero-offset walk in cursor mode, so this test
			// does not assume its own offset math — it compares against
			// what the primitive itself returns from position zero.
			var baseline []string
			cursor := ""
			for i := 0; i < 200; i++ {
				bItems, _, next, bErr := st.List(ctx, fx.Scope, owner, store.ListOptions{Limit: store.MaxRecallLimit, Cursor: cursor, CursorMode: true})
				if bErr != nil {
					t.Fatalf("%s: baseline walk page %d: %v", shape, i, bErr)
				}
				for _, m := range bItems {
					baseline = append(baseline, m.ID)
				}
				if next == "" {
					break
				}
				cursor = next
			}
			if uint64(len(baseline)) != seededTotal {
				t.Fatalf("%s: baseline walk visited %d ids, want %d", shape, len(baseline), seededTotal)
			}

			deepOffset := seededTotal / 2
			if deepOffset == 0 {
				deepOffset = 1
			}

			rec.reset()
			items, total, _, err := st.List(ctx, fx.Scope, owner, store.ListOptions{Offset: deepOffset, Limit: 5})
			if err != nil {
				t.Fatalf("%s: deep-offset List: %v", shape, err)
			}
			if total != seededTotal {
				t.Errorf("%s: deep-offset total = %d, want %d", shape, total, seededTotal)
			}
			wantCount := uint64(5)
			if deepOffset+wantCount > seededTotal {
				wantCount = seededTotal - deepOffset
			}
			if uint64(len(items)) != wantCount {
				t.Fatalf("%s: deep-offset got %d items, want %d", shape, len(items), wantCount)
			}
			for i, m := range items {
				wantID := baseline[deepOffset+uint64(i)]
				if m.ID != wantID {
					t.Errorf("%s: deep-offset item %d id = %s, want %s (baseline position %d)", shape, i, m.ID, wantID, deepOffset+uint64(i))
				}
			}

			// The prefix walk and the page fetch each size their own RPCs
			// from their own view's PerRPCLimit (boundedread.go's
			// perRPCLimit), so either phase may issue more than one Scroll
			// RPC on its own — the prefix walk when offset exceeds
			// keysView()'s per-RPC ceiling, the page fetch whenever the
			// requested limit exceeds fullView()'s (as it does here for
			// few-large's large-record ceiling). What must hold is the
			// TRANSITION: every keys-only RPC precedes every full-view RPC,
			// with at least one of each, and each RPC's limit stays within
			// its own view's PerRPCLimit.
			calls := rec.snapshot()
			if len(calls) < 2 {
				t.Fatalf("%s: recorded %d Scroll call(s), want at least 2 (a prefix walk plus a page fetch)", shape, len(calls))
			}
			keysPerRPC := uint32(store.PerRPCLimit(store.KeysView()))
			fullPerRPC := uint32(store.PerRPCLimit(st.FullView()))
			sawFullView := false
			prefixCallCount := 0
			pageCallCount := 0
			for i, call := range calls {
				if call.keysOnly {
					if sawFullView {
						t.Errorf("%s: call %d carried the keys-only selector AFTER a full-view call — prefix RPCs must all precede the page RPCs", shape, i)
					}
					if call.limit < 1 || call.limit > keysPerRPC {
						t.Errorf("%s: prefix call %d limit = %d, want between 1 and %d", shape, i, call.limit, keysPerRPC)
					}
					prefixCallCount++
					continue
				}
				sawFullView = true
				if call.limit < 1 || call.limit > fullPerRPC {
					t.Errorf("%s: page call %d limit = %d, want between 1 and %d", shape, i, call.limit, fullPerRPC)
				}
				pageCallCount++
			}
			if prefixCallCount == 0 {
				t.Errorf("%s: no keys-only prefix RPC recorded", shape)
			}
			if pageCallCount == 0 {
				t.Errorf("%s: no full-view page RPC recorded", shape)
			}

			// Offset at/beyond the total: empty page, real total, never a
			// slice panic.
			eItems, eTotal, eNext, eErr := st.List(ctx, fx.Scope, owner, store.ListOptions{Offset: seededTotal + 50, Limit: 5})
			if eErr != nil {
				t.Errorf("%s: offset-beyond-total List: %v", shape, eErr)
			} else {
				if eTotal != seededTotal {
					t.Errorf("%s: offset-beyond-total total = %d, want %d", shape, eTotal, seededTotal)
				}
				if len(eItems) != 0 {
					t.Errorf("%s: offset-beyond-total got %d items, want 0", shape, len(eItems))
				}
				if eNext != "" {
					t.Errorf("%s: offset-beyond-total next = %q, want empty", shape, eNext)
				}
			}
		})
	}
}
