// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves WR-01's fix (04-REVIEW.md): fetchPayloadsByID
// (searchfetch.go) carries the same D-07 batch-of-1 legacy-oversized-record
// fallback scrollOrderedPage (orderedpage_oversized_test.go's
// TestScrollOrderedPageBatchOfOneFallback) and scrollAllPoints
// (boundedread.go) already implement for the identical class of pre-cap
// record — mirroring that test's two subtests and its exact content-size
// formulas.
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
)

// TestFetchPayloadsByIDBatchOfOneFallback proves fetchPayloadsByID's D-07
// batch-of-1 fallback: a legacy-oversized record (written via a raw
// Store.Upsert, bypassing the internal/server content cap entirely — this
// package has none of its own) inside a batch whose combined bytes overflow
// storetest.RecvLimit no longer fails its whole batch — every other id in
// that batch, and every other batch, still succeeds ("legacy-window"). A
// record still over the limit at Limit: 1 still fails the call with the
// named sentinel and no partial map ("single-oversized").
func TestFetchPayloadsByIDBatchOfOneFallback(t *testing.T) {
	t.Run("legacy-window", func(t *testing.T) {
		c := storetest.Dial(t, storetest.RecvLimit)
		name := store.PrefixedTestCollection("fetchbyid_fallback_" + uuid.NewString())
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

		scope := "storetest-fallback-fetchbyid:" + uuid.NewString()
		owner := "storetest-owner-" + uuid.NewString()
		base := time.Now().UTC().Truncate(time.Second)
		// Same 5/8-of-RecvLimit legacy size as
		// TestScrollOrderedPageBatchOfOneFallback/legacy-window: two of
		// these in one batch (fullView's perRPCLimit is 2 at default caps)
		// overflow RecvLimit, but each one alone does not.
		legacyContentBytes := storetest.RecvLimit * 5 / 8

		ids := make([]string, 3)
		for i := range ids {
			m := store.Memory{
				ID:        uuid.NewString(),
				Content:   strings.Repeat("x", legacyContentBytes),
				Scope:     scope,
				Owner:     owner,
				Actor:     owner,
				Category:  "decision",
				CreatedAt: base.Add(time.Duration(i) * time.Second),
			}
			if err := st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
				t.Fatalf("Upsert legacy record %d: %v", i, err)
			}
			ids[i] = m.ID
		}

		// f is deliberately nil: this test targets the batch-of-1 fallback
		// mechanism itself, not authz filtering (already covered by
		// TestSearchTwoPhaseBounded's cross-owner subtest) — includeIDs
		// (searchfetch.go) treats a nil f as "no additional filter",
		// narrowing to just the id set.
		fetched, err := st.FetchPayloadsByID(ctx, nil, st.FullView(), ids)
		if err != nil {
			t.Fatalf("FetchPayloadsByID: %v", err)
		}
		if len(fetched) != len(ids) {
			t.Fatalf("fetched %d records, want %d — a legacy-oversized record in one batch must not fail its neighbors", len(fetched), len(ids))
		}
		for _, id := range ids {
			if _, ok := fetched[id]; !ok {
				t.Errorf("id %s missing from fetched result", id)
			}
		}
	})

	t.Run("single-oversized", func(t *testing.T) {
		c := storetest.Dial(t, storetest.RecvLimit)
		name := store.PrefixedTestCollection("fetchbyid_singlefail_" + uuid.NewString())
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

		scope := "storetest-singlefail-fetchbyid:" + uuid.NewString()
		owner := "storetest-owner-" + uuid.NewString()
		base := time.Now().UTC().Truncate(time.Second)
		// One record still over RecvLimit even alone at Limit: 1, plus two
		// tiny records sharing its first batch — same shape as
		// TestScrollOrderedPageBatchOfOneFallback/single-oversized.
		hugeContentBytes := storetest.RecvLimit + storetest.RecvLimit/4

		contents := []string{strings.Repeat("x", hugeContentBytes), strings.Repeat("s", 64), strings.Repeat("s", 64)}
		ids := make([]string, len(contents))
		for i, content := range contents {
			m := store.Memory{
				ID:        uuid.NewString(),
				Content:   content,
				Scope:     scope,
				Owner:     owner,
				Actor:     owner,
				Category:  "decision",
				CreatedAt: base.Add(time.Duration(i) * time.Second),
			}
			if err := st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
				t.Fatalf("Upsert record %d: %v", i, err)
			}
			ids[i] = m.ID
		}

		_, err := st.FetchPayloadsByID(ctx, nil, st.FullView(), ids)
		if err == nil {
			t.Fatal("FetchPayloadsByID: got nil error, want a non-nil error wrapping store.ErrResponseTooLarge")
		}
		if !errors.Is(err, store.ErrResponseTooLarge) {
			t.Fatalf("errors.Is(err, store.ErrResponseTooLarge) = false; err = %v", err)
		}
	})
}
