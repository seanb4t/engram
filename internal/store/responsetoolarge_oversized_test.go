// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file hosts TestStoreListOverflowIsResponseTooLarge (CONTEXT.md D-01,
// D-02, D-03): a real Qdrant overflow at the NAMED storetest.RecvLimit,
// scrolled through Store.List's offset-mode path, classified by the base
// interceptor NewQdrantClient installs, into store.ErrResponseTooLarge. It
// overflows only because this milestone's Phase 3/4 have not bounded
// Store.List yet — when they do, this test's fixture read must be
// re-pointed at a still-unbounded read (or a deliberately tiny named
// limit), never deleted: it is this phase's own proof that a REAL overflow,
// not just a synthetic status, gets classified.
package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestStoreListOverflowIsResponseTooLarge runs one subtest per fixture
// shape, few-large then many-small, over a real Qdrant scroll that exceeds
// the named storetest.RecvLimit. It asserts the classified sentinel, the
// gRPC status code, and the ResponseTooLargeError detail — never grpc-go's
// own default limit or message format (rule m45p2b4bp7): the named limit is
// storetest.RecvLimit, and the classifier is OUR code under test.
func TestStoreListOverflowIsResponseTooLarge(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		t.Run(shape.String(), func(t *testing.T) {
			c := storetest.Dial(t, storetest.RecvLimit)
			name := store.PrefixedTestCollection("oversized_toolarge_" + uuid.NewString())
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

			items, _, _, err := st.List(ctx, fx.Scope, store.Authenticated(fx.Owner), store.ListOptions{Limit: store.MaxRecallLimit})
			if err == nil {
				t.Fatalf("List: got nil error, want an overflow classified as store.ErrResponseTooLarge (%d records at %d bytes each, over the %d-byte named limit)", len(fx.IDs), fx.RecordBytes, storetest.RecvLimit)
			}
			if items != nil {
				t.Errorf("List: items = %v, want nil on overflow (never a success-shaped page)", items)
			}
			if !errors.Is(err, store.ErrResponseTooLarge) {
				t.Fatalf("List: errors.Is(err, store.ErrResponseTooLarge) = false; err = %v", err)
			}
			gotStatus, ok := status.FromError(err)
			if !ok || gotStatus.Code() != codes.ResourceExhausted {
				t.Fatalf("status.FromError(err): ok=%v code=%v, want ok=true code=ResourceExhausted", ok, gotStatus.Code())
			}
			var rtle *store.ResponseTooLargeError
			if !errors.As(err, &rtle) {
				t.Fatalf("errors.As(err, &rtle) = false; err = %v", err)
			}
			if !strings.HasSuffix(rtle.Method, "/Scroll") {
				t.Errorf("rtle.Method = %q, want a suffix of /Scroll", rtle.Method)
			}
			if rtle.Limit != storetest.RecvLimit {
				t.Errorf("rtle.Limit = %d, want %d (storetest.RecvLimit)", rtle.Limit, storetest.RecvLimit)
			}
		})
	}
}
