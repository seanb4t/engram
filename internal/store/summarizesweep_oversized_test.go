// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves REQ-sweeps-bounded's real-Qdrant regression for
// engram summarize-missing (TestSummarizeMissingBoundedOverGRPCLimit): OUR
// request shape stays bounded through the shared byte-budget iterator over
// a scope whose 256-record page would have overflowed the named receive
// limit under the pre-migration nil-offset ScrollAndOffset call, on both
// fixture shapes — never that gRPC enforces a ceiling (rule m45p2b4bp7).
package store_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
)

// countingSummarizer returns a store.SummarizeFunc that records how many
// times it was invoked, and a pointer to that count. Used to prove the
// egress boundary did not widen: the summariser must be called exactly
// res.Filled times, never for a record the caller's limit or age filter
// excluded.
func countingSummarizer() (store.SummarizeFunc, *int64) {
	var calls int64
	fn := func(_ context.Context, _ string) (string, error) {
		atomic.AddInt64(&calls, 1)
		return "SUMMARY", nil
	}
	return fn, &calls
}

// TestSummarizeMissingBoundedOverGRPCLimit proves engram summarize-missing
// drains an oversized, no-summary scope through the shared byte-budget
// iterator instead of the pre-migration 256-record ScrollAndOffset page —
// on both fixture shapes — and that the caller-limit mid-page early exit
// (the case none of the 8 in-place tests reaches, since none spans more
// than one page) and the non-dry-run egress boundary both hold.
func TestSummarizeMissingBoundedOverGRPCLimit(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		t.Run(shape.String(), func(t *testing.T) {
			c := storetest.Dial(t, storetest.RecvLimit)
			name := store.PrefixedTestCollection("oversized_summarizesweep_" + uuid.NewString())
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

			// SeedOversized's records carry no summary — exactly the state
			// this sweep looks for — and their content is well over any
			// small MaxChars cap, so every seeded record is eligible.
			fx := storetest.SeedOversized(t, st, storetest.Spec{Limit: storetest.RecvLimit, Shape: shape, Vector: []float32{0.1, 0.2, 0.3}})

			// Sub-case 1: dry-run, no caller limit — the whole scope is
			// scanned and "would fill" every seeded record, and the
			// summariser is never called (DryRun never crosses the
			// egress boundary). Before the migration this call fails on
			// FewLarge: one 256-record page returns all 40 large records
			// (~5.24 MiB) at once, overflowing storetest.RecvLimit.
			dryFn, dryCalls := countingSummarizer()
			dryRes, err := st.SummarizeMissing(ctx, store.SummarizeOptions{
				Scope: fx.Scope, MaxChars: 8, Model: "test", DryRun: true,
			}, dryFn)
			if err != nil {
				t.Fatalf("%s: SummarizeMissing(DryRun): %v (request shape may have exceeded the %d-byte named receive limit)", shape, err, storetest.RecvLimit)
			}
			if dryRes.Scanned != len(fx.IDs) {
				t.Errorf("%s: dry-run res.Scanned = %d, want %d", shape, dryRes.Scanned, len(fx.IDs))
			}
			if dryRes.Filled != len(fx.IDs) {
				t.Errorf("%s: dry-run res.Filled = %d, want %d (\"would fill\" every seeded record)", shape, dryRes.Filled, len(fx.IDs))
			}
			if got := atomic.LoadInt64(dryCalls); got != 0 {
				t.Errorf("%s: dry-run summariser invoked %d times, want 0", shape, got)
			}

			// Sub-case 2: a caller limit strictly between 1 and the
			// seeded count exercises the mid-page early exit — the case
			// no existing summarize_test.go fixture reaches, since none
			// spans more than one page.
			limit := len(fx.IDs) / 2
			if limit < 1 {
				limit = 1
			}
			if limit >= len(fx.IDs) {
				limit = len(fx.IDs) - 1
			}
			limFn, limCalls := countingSummarizer()
			limRes, err := st.SummarizeMissing(ctx, store.SummarizeOptions{
				Scope: fx.Scope, MaxChars: 8, Model: "test", DryRun: true, Limit: limit,
			}, limFn)
			if err != nil {
				t.Fatalf("%s: SummarizeMissing(Limit=%d): %v", shape, limit, err)
			}
			if limRes.Scanned != limit {
				t.Errorf("%s: res.Scanned = %d, want exactly the caller limit %d (mid-page early exit)", shape, limRes.Scanned, limit)
			}
			if got := atomic.LoadInt64(limCalls); got != 0 {
				t.Errorf("%s: dry-run+limit summariser invoked %d times, want 0", shape, got)
			}

			// Sub-case 3: a non-dry-run call crosses the egress boundary.
			// The summariser must be invoked exactly res.Filled times —
			// no more (a widened boundary) and no fewer (a dropped
			// record) — and every seeded record must be filled, since
			// none of them is excluded by the age filter or the limit.
			fillFn, fillCalls := countingSummarizer()
			fillRes, err := st.SummarizeMissing(ctx, store.SummarizeOptions{
				Scope: fx.Scope, MaxChars: 8, Model: "test",
			}, fillFn)
			if err != nil {
				t.Fatalf("%s: SummarizeMissing (apply): %v (request shape may have exceeded the %d-byte named receive limit)", shape, err, storetest.RecvLimit)
			}
			if fillRes.Scanned != len(fx.IDs) {
				t.Errorf("%s: apply res.Scanned = %d, want %d", shape, fillRes.Scanned, len(fx.IDs))
			}
			if fillRes.Filled != len(fx.IDs) {
				t.Errorf("%s: apply res.Filled = %d, want %d", shape, fillRes.Filled, len(fx.IDs))
			}
			if fillRes.Failed != 0 {
				t.Errorf("%s: apply res.Failed = %d, want 0", shape, fillRes.Failed)
			}
			if got := atomic.LoadInt64(fillCalls); got != int64(fillRes.Filled) {
				t.Errorf("%s: summariser invoked %d times, want exactly res.Filled = %d (egress boundary widened or narrowed)", shape, got, fillRes.Filled)
			}

			t.Logf("%s: dry-run Scanned=%d Filled=%d; limited(%d) Scanned=%d; apply Scanned=%d Filled=%d summariser_calls=%d",
				shape, dryRes.Scanned, dryRes.Filled, limit, limRes.Scanned, fillRes.Scanned, fillRes.Filled, atomic.LoadInt64(fillCalls))
		})
	}
}
