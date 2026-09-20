// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file hosts TestStoreListOverflowIsResponseTooLarge (CONTEXT.md D-01,
// D-02, D-03, D-12): a real Qdrant overflow at the NAMED storetest.RecvLimit,
// scrolled through Store.List, classified by the base interceptor
// NewQdrantClient installs, into store.ErrResponseTooLarge.
//
// D-12 retarget (plan 04-02, Task 3): this test originally drove Store.List's
// offset mode with Limit:0 over a many-record oversized fixture — the shape
// that overflowed while Store.List issued one unbounded full-payload Scroll.
// Plan 04-02 bounded that path (Store.List now composes scrollOrderedPage's
// byte-budgeted RPCs), so a many-record fixture no longer overflows: it is
// now assembled correctly across several small, budget-respecting RPCs. The
// trigger is retargeted onto a single LEGACY record (this package enforces no
// content cap of its own) whose content alone exceeds storetest.RecvLimit —
// the same D-07 batch-of-1 fallback path orderedpage_oversized_test.go's
// TestScrollOrderedPageBatchOfOneFallback/single-oversized subtest already
// proves at the primitive level. Store.List reaches that same fallback via
// scrollOrderedPage and still fails named, keeping store.ErrResponseTooLarge
// reachable and RED-provable through this entry point after the fix.
package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestStoreListOverflowIsResponseTooLarge drives a single legacy record
// whose content alone exceeds the named storetest.RecvLimit through
// Store.List (offset mode). It asserts the classified sentinel, the gRPC
// status code, and the ResponseTooLargeError detail — never grpc-go's own
// default limit or message format (rule m45p2b4bp7): the named limit is
// storetest.RecvLimit, and the classifier is OUR code under test.
func TestStoreListOverflowIsResponseTooLarge(t *testing.T) {
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

	scope := "storetest-list-singlefail:" + uuid.NewString()
	owner := "storetest-owner-" + uuid.NewString()
	hugeContentBytes := storetest.RecvLimit + storetest.RecvLimit/4
	m := store.Memory{
		ID:        uuid.NewString(),
		Content:   strings.Repeat("x", hugeContentBytes),
		Scope:     scope,
		Owner:     owner,
		Actor:     owner,
		Category:  "decision",
		CreatedAt: time.Now().UTC(),
	}
	if err := st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("Upsert legacy oversized record: %v", err)
	}

	items, _, _, err := st.List(ctx, scope, store.Authenticated(owner), store.ListOptions{Limit: store.MaxRecallLimit})
	if err == nil {
		t.Fatalf("List: got nil error, want an overflow classified as store.ErrResponseTooLarge (a single %d-byte record, over the %d-byte named limit)", hugeContentBytes, storetest.RecvLimit)
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
}
