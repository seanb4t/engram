// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves the byte-budget sweep primitive (scrollAllPoints's
// readView extension, boundedread.go) against both oversized fixture shapes
// storetest.SeedOversized supports, at the named storetest.RecvLimit — OUR
// request shape and OUR response-size accounting, via a recording
// interceptor chained inside the classifier NewQdrantClient installs (rule
// m45p2b4bp7: never asserting grpc-go's or Qdrant's own behavior as the test
// oracle).
package store_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/qdrant/go-client/qdrant"
)

// scrollRecordedCall is one intercepted Scroll RPC's request Limit and
// outcome, as scrollRecorder.intercept observed it.
type scrollRecordedCall struct {
	limit     uint32
	respBytes int
	code      codes.Code
}

// scrollRecorder is a mutex-guarded recording grpc.UnaryClientInterceptor,
// scoped to methods ending "/Scroll" (both ScrollAndOffset and Scroll issue
// this same RPC method — the interfaces block's own finding). reset/snapshot
// let a subtest isolate the calls belonging to its own scenario.
type scrollRecorder struct {
	mu    sync.Mutex
	calls []scrollRecordedCall
}

func (r *scrollRecorder) intercept(
	ctx context.Context, method string, req, reply any,
	cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
) error {
	if len(method) < len("/Scroll") || method[len(method)-len("/Scroll"):] != "/Scroll" {
		return invoker(ctx, method, req, reply, cc, opts...)
	}
	var limit uint32
	if sp, ok := req.(*qdrant.ScrollPoints); ok {
		limit = sp.GetLimit()
	}
	err := invoker(ctx, method, req, reply, cc, opts...)
	call := scrollRecordedCall{limit: limit}
	if err != nil {
		call.code = status.Code(err)
	} else if msg, ok := reply.(proto.Message); ok {
		call.respBytes = proto.Size(msg)
	}
	r.mu.Lock()
	r.calls = append(r.calls, call)
	r.mu.Unlock()
	return err
}

func (r *scrollRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = nil
}

func (r *scrollRecorder) snapshot() []scrollRecordedCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]scrollRecordedCall, len(r.calls))
	copy(out, r.calls)
	return out
}

// TestScrollAllPointsByteBudget runs one subtest per fixture shape
// (few-large, many-small), each dialing with a chained recording
// interceptor, seeding once, then sweeping both the full view and the
// summary view and asserting: every seeded id is visited exactly once, the
// recorder saw at least one Scroll call, every recorded request Limit
// equals store.SweepLimit(view), no recorded response exceeds
// store.RPCByteBudget(), no call ended ResourceExhausted, and the summary
// view's points carry neither content nor citations.
func TestScrollAllPointsByteBudget(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		shape := shape
		t.Run(shape.String(), func(t *testing.T) {
			rec := &scrollRecorder{}
			c := storetest.Dial(t, storetest.RecvLimit, grpc.WithChainUnaryInterceptor(rec.intercept))
			name := store.PrefixedTestCollection("oversized_boundedread_" + uuid.NewString())
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

			// Pin the default per-view counts once (D-02/D-06's derived
			// values at DefaultRecordCaps()).
			if got := store.SweepLimit(st.FullView()); got != 2 {
				t.Errorf("SweepLimit(FullView()) = %d, want 2", got)
			}
			if got := store.SweepLimit(st.SummaryView()); got != 61 {
				t.Errorf("SweepLimit(SummaryView()) = %d, want 61", got)
			}

			subtests := []struct {
				name string
				view store.ReadView
			}{
				{"full", st.FullView()},
				{"summary", st.SummaryView()},
			}
			for _, sub := range subtests {
				sub := sub
				t.Run(sub.name, func(t *testing.T) {
					rec.reset()
					visited := map[string]int{}
					scanErr := st.ScrollAllPoints(ctx,
						&qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("scope", fx.Scope)}},
						sub.view,
						func(p *qdrant.RetrievedPoint) error {
							visited[p.Id.GetUuid()]++
							if sub.name == "summary" {
								if _, ok := p.Payload["content"]; ok {
									t.Errorf("summary view point %s carries content", p.Id.GetUuid())
								}
								if _, ok := p.Payload["citations"]; ok {
									t.Errorf("summary view point %s carries citations", p.Id.GetUuid())
								}
							}
							return nil
						})
					if scanErr != nil {
						t.Fatalf("ScrollAllPoints: %v", scanErr)
					}
					if len(visited) != len(fx.IDs) {
						t.Errorf("visited %d distinct ids, want %d", len(visited), len(fx.IDs))
					}
					for _, id := range fx.IDs {
						if visited[id] != 1 {
							t.Errorf("id %s visited %d time(s), want exactly 1", id, visited[id])
						}
					}

					calls := rec.snapshot()
					if len(calls) == 0 {
						t.Fatal("recorder observed zero Scroll calls")
					}
					wantLimit := uint32(store.SweepLimit(sub.view))
					for i, call := range calls {
						if call.limit != wantLimit {
							t.Errorf("call %d: Limit = %d, want %d", i, call.limit, wantLimit)
						}
						if call.code == codes.ResourceExhausted {
							t.Errorf("call %d: ended ResourceExhausted", i)
						}
						if call.respBytes > store.RPCByteBudget() {
							t.Errorf("call %d: response %d bytes exceeds RPCByteBudget %d", i, call.respBytes, store.RPCByteBudget())
						}
					}
				})
			}
		})
	}
}
