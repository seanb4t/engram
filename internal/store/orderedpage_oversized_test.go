// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves the ordered-page primitive (orderedpage.go) against both
// oversized fixture shapes storetest.SeedOversized supports, at the named
// storetest.RecvLimit — OUR request shape and OUR page contract, never
// Qdrant's or grpc-go's own behavior (rule m45p2b4bp7). It reuses
// scrollRecorder (boundedread_oversized_test.go, same package) rather than
// redefining it.
package store_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/qdrant/go-client/qdrant"
)

// TestScrollOrderedPageByteBudget walks both fixture shapes and both views,
// asserting the ordered-page contract per page (no error; Bytes within
// PageByteBudget; CutByBudget and Exhausted never both true; a cut page
// holds fewer items than the limit), across pages (no id twice; the union
// equals fx.IDs; CreatedAt never increases), and per recorded RPC (Limit
// within [1, PerRPCLimit(view)]; response within RPCByteBudget; no
// ResourceExhausted). In full view the first page is cut by budget; in
// summary view the first page holds the whole fixture and the following
// call reports Exhausted with zero items.
func TestScrollOrderedPageByteBudget(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		shape := shape
		t.Run(shape.String(), func(t *testing.T) {
			rec := &scrollRecorder{}
			c := storetest.Dial(t, storetest.RecvLimit, grpc.WithChainUnaryInterceptor(rec.intercept))
			name := store.PrefixedTestCollection("oversized_orderedpage_" + uuid.NewString())
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
					var lastCreatedAt time.Time
					haveLast := false
					exhaustedSeen := false

					from := store.ListCursor{}
					for i := 0; i < 200; i++ {
						page, err := st.ScrollOrderedPage(ctx, fx.Scope, store.Authenticated(fx.Owner), sub.view, qdrant.Direction_Desc, from, uint64(len(fx.IDs)))
						if err != nil {
							t.Fatalf("page %d: ScrollOrderedPage: %v", i, err)
						}
						if page.Bytes > store.PageByteBudget() {
							t.Errorf("page %d: Bytes = %d, exceeds PageByteBudget %d", i, page.Bytes, store.PageByteBudget())
						}
						if page.CutByBudget && page.Exhausted {
							t.Errorf("page %d: CutByBudget and Exhausted both true", i)
						}
						if page.CutByBudget && uint64(len(page.Items)) >= uint64(len(fx.IDs)) {
							t.Errorf("page %d: cut page has %d items, want fewer than limit %d", i, len(page.Items), len(fx.IDs))
						}

						if i == 0 && sub.name == "full" && !page.CutByBudget {
							t.Error("full: first page CutByBudget = false, want true")
						}
						if i == 0 && sub.name == "summary" {
							if page.CutByBudget {
								t.Error("summary: first page CutByBudget = true, want false")
							}
							if page.Exhausted {
								t.Error("summary: first page Exhausted = true, want false")
							}
							if len(page.Items) != len(fx.IDs) {
								t.Errorf("summary: first page holds %d items, want %d", len(page.Items), len(fx.IDs))
							}
						}
						if i == 1 && sub.name == "summary" {
							if len(page.Items) != 0 {
								t.Errorf("summary: second page holds %d items, want 0", len(page.Items))
							}
							if !page.Exhausted {
								t.Error("summary: second page Exhausted = false, want true")
							}
						}

						for _, m := range page.Items {
							visited[m.ID]++
							if visited[m.ID] > 1 {
								t.Fatalf("page %d: id %s visited twice", i, m.ID)
							}
							if haveLast && m.CreatedAt.After(lastCreatedAt) {
								t.Errorf("page %d: CreatedAt increased: %v after %v", i, m.CreatedAt, lastCreatedAt)
							}
							lastCreatedAt = m.CreatedAt
							haveLast = true
						}

						if page.Exhausted {
							exhaustedSeen = true
							break
						}
						from = page.Next
					}
					if !exhaustedSeen {
						t.Fatal("did not reach Exhausted within 200 pages")
					}

					if len(visited) != len(fx.IDs) {
						t.Errorf("visited %d distinct ids, want %d", len(visited), len(fx.IDs))
					}
					for _, id := range fx.IDs {
						if visited[id] != 1 {
							t.Errorf("id %s visited %d time(s), want exactly 1", id, visited[id])
						}
					}

					wantMaxLimit := uint32(store.PerRPCLimit(sub.view))
					for i, call := range rec.snapshot() {
						if call.limit < 1 || call.limit > wantMaxLimit {
							t.Errorf("call %d: Limit = %d, want between 1 and %d", i, call.limit, wantMaxLimit)
						}
						if call.respBytes > store.RPCByteBudget() {
							t.Errorf("call %d: response %d bytes exceeds RPCByteBudget %d", i, call.respBytes, store.RPCByteBudget())
						}
						if call.code == codes.ResourceExhausted {
							t.Errorf("call %d: ended ResourceExhausted", i)
						}
					}
				})
			}
		})
	}
}

// TestScrollOrderedPageTiesAcrossRPCBoundaries seeds seven tie-owner records
// sharing one CreatedAt T, three tie-owner records at T-1s, and two private
// other-owner records at T, then shrinks the summary view's per-RPC count to
// 2 (via SetByteBudgets) so the T-tie spans several RPCs within one page.
// Walking pages of limit 4 must visit every tie-owner id exactly once, never
// surface an other-owner id, stay non-increasing by CreatedAt, and at least
// one returned Next must carry C == T with a non-empty Seen (the tie
// genuinely spans an RPC/page boundary).
func TestScrollOrderedPageTiesAcrossRPCBoundaries(t *testing.T) {
	c := storetest.Dial(t, storetest.RecvLimit)
	name := store.PrefixedTestCollection("orderedpage_ties_" + uuid.NewString())
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

	store.SetByteBudgets(t, 2*store.ViewMaxRecordBytes(st.SummaryView()), store.PageByteBudget())

	scope := "storetest-ties:" + uuid.NewString()
	tieOwner := "storetest-tieowner-" + uuid.NewString()
	otherOwner := "storetest-otherowner-" + uuid.NewString()
	tBoundary := time.Now().UTC().Truncate(time.Second)
	tEarlier := tBoundary.Add(-time.Second)

	upsert := func(owner string, createdAt time.Time) string {
		m := store.Memory{
			ID:        uuid.NewString(),
			Content:   "c",
			Scope:     scope,
			Owner:     owner,
			Actor:     owner,
			Category:  "decision",
			CreatedAt: createdAt,
		}
		if err := st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("Upsert: %v", err)
		}
		return m.ID
	}

	tieIDs := map[string]bool{}
	otherIDs := map[string]bool{}
	for i := 0; i < 7; i++ {
		tieIDs[upsert(tieOwner, tBoundary)] = true
	}
	for i := 0; i < 3; i++ {
		tieIDs[upsert(tieOwner, tEarlier)] = true
	}
	for i := 0; i < 2; i++ {
		otherIDs[upsert(otherOwner, tBoundary)] = true
	}

	visited := map[string]int{}
	var lastCreatedAt time.Time
	haveLast := false
	sawBoundaryNext := false
	wantBoundary := tBoundary.UTC().Format(time.RFC3339)
	from := store.ListCursor{}
	exhausted := false
	for i := 0; i < 20; i++ {
		page, err := st.ScrollOrderedPage(ctx, scope, store.Authenticated(tieOwner), st.SummaryView(), qdrant.Direction_Desc, from, 4)
		if err != nil {
			t.Fatalf("page %d: ScrollOrderedPage: %v", i, err)
		}
		for _, m := range page.Items {
			if otherIDs[m.ID] {
				t.Fatalf("page %d: other-owner id %s appeared", i, m.ID)
			}
			visited[m.ID]++
			if visited[m.ID] > 1 {
				t.Fatalf("page %d: id %s visited twice", i, m.ID)
			}
			if haveLast && m.CreatedAt.After(lastCreatedAt) {
				t.Errorf("page %d: CreatedAt increased: %v after %v", i, m.CreatedAt, lastCreatedAt)
			}
			lastCreatedAt = m.CreatedAt
			haveLast = true
		}
		if page.Next.C == wantBoundary && len(page.Next.Seen) > 0 {
			sawBoundaryNext = true
		}
		if page.Exhausted {
			exhausted = true
			break
		}
		from = page.Next
	}
	if !exhausted {
		t.Fatal("did not reach Exhausted within 20 pages")
	}
	if len(visited) != len(tieIDs) {
		t.Errorf("visited %d distinct ids, want %d", len(visited), len(tieIDs))
	}
	for id := range tieIDs {
		if visited[id] != 1 {
			t.Errorf("id %s visited %d time(s), want 1", id, visited[id])
		}
	}
	if !sawBoundaryNext {
		t.Error("no returned Next carried C == the tie boundary with a non-empty Seen")
	}
}

// TestScrollOrderedPageAscending proves qdrant.Direction_Asc works the same
// way as Desc: five records with distinct CreatedAt, walked at limit 2,
// yield pages of 2, 2, 1 in ascending order, the last Exhausted.
func TestScrollOrderedPageAscending(t *testing.T) {
	c := storetest.Dial(t, storetest.RecvLimit)
	name := store.PrefixedTestCollection("orderedpage_asc_" + uuid.NewString())
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

	scope := "storetest-asc:" + uuid.NewString()
	owner := "storetest-owner-" + uuid.NewString()
	base := time.Now().UTC().Truncate(time.Second)
	for i := 0; i < 5; i++ {
		m := store.Memory{
			ID:        uuid.NewString(),
			Content:   "c",
			Scope:     scope,
			Owner:     owner,
			Actor:     owner,
			Category:  "decision",
			CreatedAt: base.Add(time.Duration(i) * time.Second),
		}
		if err := st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("Upsert record %d: %v", i, err)
		}
	}

	wantCounts := []int{2, 2, 1}
	from := store.ListCursor{}
	var lastCreatedAt time.Time
	haveLast := false
	for i, want := range wantCounts {
		page, err := st.ScrollOrderedPage(ctx, scope, store.Authenticated(owner), st.FullView(), qdrant.Direction_Asc, from, 2)
		if err != nil {
			t.Fatalf("page %d: ScrollOrderedPage: %v", i, err)
		}
		if len(page.Items) != want {
			t.Errorf("page %d: got %d items, want %d", i, len(page.Items), want)
		}
		for _, m := range page.Items {
			if haveLast && m.CreatedAt.Before(lastCreatedAt) {
				t.Errorf("page %d: CreatedAt decreased", i)
			}
			lastCreatedAt = m.CreatedAt
			haveLast = true
		}
		wantExhausted := i == len(wantCounts)-1
		if page.Exhausted != wantExhausted {
			t.Errorf("page %d: Exhausted = %v, want %v", i, page.Exhausted, wantExhausted)
		}
		from = page.Next
	}
}

// seedFiveOrderedRecords writes five small records with distinct CreatedAt
// into a fresh prefixed collection, returning the store, scope, owner, and a
// cleanup function the caller must defer.
func seedFiveOrderedRecords(t *testing.T) (st *store.Store, scope, owner string, cleanup func()) {
	t.Helper()
	c := storetest.Dial(t, storetest.RecvLimit)
	name := store.PrefixedTestCollection("orderedpage_boundary_" + uuid.NewString())
	st = store.NewTestStore(t, c, name)
	ctx := context.Background()
	if err := st.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}
	scope = "storetest-boundary:" + uuid.NewString()
	owner = "storetest-owner-" + uuid.NewString()
	base := time.Now().UTC().Truncate(time.Second)
	for i := 0; i < 5; i++ {
		m := store.Memory{
			ID:        uuid.NewString(),
			Content:   "c",
			Scope:     scope,
			Owner:     owner,
			Actor:     owner,
			Category:  "decision",
			CreatedAt: base.Add(time.Duration(i) * time.Second),
		}
		if err := st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("Upsert record %d: %v", i, err)
		}
	}
	cleanup = func() {
		if err := c.DeleteCollection(ctx, name); err != nil {
			t.Errorf("DeleteCollection(%q): %v", name, err)
		}
	}
	return st, scope, owner, cleanup
}

// TestScrollOrderedPageBudgetBoundaries proves the three named boundary
// shapes: a page byte budget smaller than one full-view ceiling still
// advances exactly one record per call (never stalls at zero); a limit
// reached with budget left is neither cut nor exhausted; and an exactly-full
// final page is followed by an empty, exhausted page whose Next is
// unchanged (REQ-byte-budget-pages boundary edge probe, resolved explicit).
func TestScrollOrderedPageBudgetBoundaries(t *testing.T) {
	t.Run("page-smaller-than-one-ceiling", func(t *testing.T) {
		st, scope, owner, cleanup := seedFiveOrderedRecords(t)
		defer cleanup()
		store.SetByteBudgets(t, store.RPCByteBudget(), 512<<10)

		ctx := context.Background()
		from := store.ListCursor{}
		visited := map[string]int{}
		for i := 0; i < 5; i++ {
			page, err := st.ScrollOrderedPage(ctx, scope, store.Authenticated(owner), st.FullView(), qdrant.Direction_Desc, from, 5)
			if err != nil {
				t.Fatalf("page %d: ScrollOrderedPage: %v", i, err)
			}
			if len(page.Items) != 1 {
				t.Errorf("page %d: got %d items, want 1", i, len(page.Items))
			}
			if !page.CutByBudget {
				t.Errorf("page %d: CutByBudget = false, want true", i)
			}
			for _, m := range page.Items {
				visited[m.ID]++
			}
			from = page.Next
		}
		last, err := st.ScrollOrderedPage(ctx, scope, store.Authenticated(owner), st.FullView(), qdrant.Direction_Desc, from, 5)
		if err != nil {
			t.Fatalf("final page: ScrollOrderedPage: %v", err)
		}
		if len(last.Items) != 0 {
			t.Errorf("final page: got %d items, want 0", len(last.Items))
		}
		if !last.Exhausted {
			t.Error("final page: Exhausted = false, want true")
		}
		if len(visited) != 5 {
			t.Errorf("visited %d distinct ids, want 5", len(visited))
		}
	})

	t.Run("limit-reached-with-budget-left", func(t *testing.T) {
		st, scope, owner, cleanup := seedFiveOrderedRecords(t)
		defer cleanup()

		ctx := context.Background()
		page1, err := st.ScrollOrderedPage(ctx, scope, store.Authenticated(owner), st.SummaryView(), qdrant.Direction_Desc, store.ListCursor{}, 3)
		if err != nil {
			t.Fatalf("page 1: ScrollOrderedPage: %v", err)
		}
		if len(page1.Items) != 3 {
			t.Errorf("page 1: got %d items, want 3", len(page1.Items))
		}
		if page1.CutByBudget {
			t.Error("page 1: CutByBudget = true, want false")
		}
		if page1.Exhausted {
			t.Error("page 1: Exhausted = true, want false")
		}

		page2, err := st.ScrollOrderedPage(ctx, scope, store.Authenticated(owner), st.SummaryView(), qdrant.Direction_Desc, page1.Next, 3)
		if err != nil {
			t.Fatalf("page 2: ScrollOrderedPage: %v", err)
		}
		if len(page2.Items) != 2 {
			t.Errorf("page 2: got %d items, want 2", len(page2.Items))
		}
		if !page2.Exhausted {
			t.Error("page 2: Exhausted = false, want true")
		}
	})

	t.Run("exact-final-page", func(t *testing.T) {
		st, scope, owner, cleanup := seedFiveOrderedRecords(t)
		defer cleanup()

		ctx := context.Background()
		page1, err := st.ScrollOrderedPage(ctx, scope, store.Authenticated(owner), st.FullView(), qdrant.Direction_Desc, store.ListCursor{}, 5)
		if err != nil {
			t.Fatalf("page 1: ScrollOrderedPage: %v", err)
		}
		if len(page1.Items) != 5 {
			t.Errorf("page 1: got %d items, want 5", len(page1.Items))
		}
		if page1.Exhausted {
			t.Error("page 1: Exhausted = true, want false")
		}

		page2, err := st.ScrollOrderedPage(ctx, scope, store.Authenticated(owner), st.FullView(), qdrant.Direction_Desc, page1.Next, 5)
		if err != nil {
			t.Fatalf("page 2: ScrollOrderedPage: %v", err)
		}
		if len(page2.Items) != 0 {
			t.Errorf("page 2: got %d items, want 0", len(page2.Items))
		}
		if !page2.Exhausted {
			t.Error("page 2: Exhausted = false, want true")
		}
		if page2.Next.C != page1.Next.C || !slices.Equal(page2.Next.Seen, page1.Next.Seen) {
			t.Errorf("page 2: Next = %+v, want unchanged %+v", page2.Next, page1.Next)
		}
	})
}

// TestScrollOrderedPageTiesAcrossRPCBoundaries's sibling for D-07: three
// legacy over-cap records (written via a raw Store.Upsert, bypassing the
// internal/server content cap entirely — this package has none of its own)
// trigger the batch-of-1 fallback (legacy-window), and one record still over
// storetest.RecvLimit fails the call with the named sentinel and zero items
// (single-oversized).
func TestScrollOrderedPageBatchOfOneFallback(t *testing.T) {
	t.Run("legacy-window", func(t *testing.T) {
		rec := &scrollRecorder{}
		c := storetest.Dial(t, storetest.RecvLimit, grpc.WithChainUnaryInterceptor(rec.intercept))
		name := store.PrefixedTestCollection("orderedpage_fallback_" + uuid.NewString())
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

		scope := "storetest-fallback-ordered:" + uuid.NewString()
		owner := "storetest-owner-" + uuid.NewString()
		base := time.Now().UTC().Truncate(time.Second)
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

		rec.reset()
		visited := map[string]int{}
		from := store.ListCursor{}
		exhausted := false
		for i := 0; i < 10; i++ {
			page, err := st.ScrollOrderedPage(ctx, scope, store.Authenticated(owner), st.FullView(), qdrant.Direction_Desc, from, 3)
			if err != nil {
				t.Fatalf("page %d: ScrollOrderedPage: %v", i, err)
			}
			for _, m := range page.Items {
				visited[m.ID]++
			}
			if page.Exhausted {
				exhausted = true
				break
			}
			from = page.Next
		}
		if !exhausted {
			t.Fatal("did not reach Exhausted")
		}
		for _, id := range ids {
			if visited[id] != 1 {
				t.Errorf("id %s visited %d time(s), want 1", id, visited[id])
			}
		}

		var sawTwoOverflow, sawSingleOK bool
		for _, call := range rec.snapshot() {
			switch {
			case call.limit == 2 && call.code == codes.ResourceExhausted:
				sawTwoOverflow = true
			case call.limit == 1 && call.code == codes.OK:
				sawSingleOK = true
			}
		}
		if !sawTwoOverflow {
			t.Error("recorder never observed a limit==2 call ending ResourceExhausted")
		}
		if !sawSingleOK {
			t.Error("recorder never observed a limit==1 OK call")
		}
	})

	t.Run("single-oversized", func(t *testing.T) {
		c := storetest.Dial(t, storetest.RecvLimit)
		name := store.PrefixedTestCollection("orderedpage_singlefail_" + uuid.NewString())
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

		scope := "storetest-singlefail-ordered:" + uuid.NewString()
		owner := "storetest-owner-" + uuid.NewString()
		base := time.Now().UTC().Truncate(time.Second)
		hugeContentBytes := storetest.RecvLimit + storetest.RecvLimit/4

		contents := []string{strings.Repeat("x", hugeContentBytes), strings.Repeat("s", 64), strings.Repeat("s", 64)}
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
		}

		from := store.ListCursor{}
		var lastErr error
		for i := 0; i < 10; i++ {
			page, err := st.ScrollOrderedPage(ctx, scope, store.Authenticated(owner), st.FullView(), qdrant.Direction_Desc, from, 3)
			if err != nil {
				lastErr = err
				if len(page.Items) != 0 {
					t.Errorf("error page carries %d items, want 0", len(page.Items))
				}
				break
			}
			if page.Exhausted {
				break
			}
			from = page.Next
		}
		if lastErr == nil {
			t.Fatal("ScrollOrderedPage: got nil error, want a non-nil error wrapping store.ErrResponseTooLarge")
		}
		if !errors.Is(lastErr, store.ErrResponseTooLarge) {
			t.Fatalf("errors.Is(lastErr, store.ErrResponseTooLarge) = false; err = %v", lastErr)
		}
	})
}

// tooManySeenIDs returns store.MaxRecallLimit+1 distinct ids, for the
// seen-set-too-large invalid-input case.
func tooManySeenIDs() []string {
	out := make([]string, store.MaxRecallLimit+1)
	for i := range out {
		out[i] = uuid.NewString()
	}
	return out
}

// TestScrollOrderedPageRejectsInvalidInput proves every input-validation
// error is rejected with store.ErrInvalidArgument before any RPC (no Qdrant
// client needed) and returns a zero-item page.
func TestScrollOrderedPageRejectsInvalidInput(t *testing.T) {
	st := store.NewTestStore(t, nil, store.PrefixedTestCollection("ordered_invalid"))
	ctx := context.Background()
	scope := "storetest-invalid:" + uuid.NewString()
	owner := "storetest-owner-" + uuid.NewString()

	cases := []struct {
		name  string
		view  store.ReadView
		from  store.ListCursor
		limit uint64
	}{
		{name: "zero limit", view: st.FullView(), from: store.ListCursor{}, limit: 0},
		// store.ReadView{} is the zero value: no byte-derived ceiling
		// (budgeted() reports false), constructed directly now that Phase
		// 5's count-only view constructor no longer exists. The premise
		// this row proves survives the deletion unchanged: scrollOrderedPage's
		// own argument validation rejects ANY unbudgeted view outright,
		// independent of how one is minted.
		{name: "no byte ceiling", view: store.ReadView{}, from: store.ListCursor{}, limit: 5},
		{name: "seen without boundary", view: st.FullView(), from: store.ListCursor{Seen: []string{"x"}}, limit: 5},
		{name: "seen set too large", view: st.FullView(), from: store.ListCursor{C: time.Now().UTC().Format(time.RFC3339), Seen: tooManySeenIDs()}, limit: 5},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			page, err := st.ScrollOrderedPage(ctx, scope, store.Authenticated(owner), tc.view, qdrant.Direction_Desc, tc.from, tc.limit)
			if !errors.Is(err, store.ErrInvalidArgument) {
				t.Fatalf("errors.Is(err, store.ErrInvalidArgument) = false; err = %v", err)
			}
			if len(page.Items) != 0 {
				t.Errorf("got %d items, want 0", len(page.Items))
			}
		})
	}
}
