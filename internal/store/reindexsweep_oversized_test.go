// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves REQ-sweeps-bounded's real-Qdrant regression for
// engram reindex (TestReindexBoundedOverGRPCLimit): OUR request shape
// stays bounded through the shared byte-budget iterator over a scope
// whose 256-record page would have overflowed the named receive limit
// under the pre-migration nil-offset ScrollAndOffset call, on both
// fixture shapes — never that gRPC enforces a ceiling (rule m45p2b4bp7).
package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
)

// oversizedReindexEmbed is a deterministic dim-4 embedder, mirroring
// reindex_test.go's own embed4 helper: a fixed-dimension vector matching
// the target collection's configured dimension, differing from the
// source's dimension (3) so a passing test proves a dimension-changing
// reindex actually happened.
func oversizedReindexEmbed(_ context.Context, _ string) ([]float32, error) {
	return []float32{0.4, 0.5, 0.6, 0.7}, nil
}

// reindexResumeSafeBatch sizes ReindexOptions.Batch so the OUT-OF-SCOPE,
// byte-unbounded reindexTargetContents Get() call (one request for the
// whole accumulated page's ids, full payload — untouched by this
// migration, see this plan's <trap_note>) stays comfortably under
// storetest.RecvLimit for a given shape's per-record content size. The
// default reindexBatch (256) is safe for ManySmall's small records but
// would make that one Get() request ~5.24 MiB for FewLarge's 40
// 131072-byte records — the exact overflow this migration's SOURCE walk
// now avoids, just relocated to the untouched per-page target lookup.
// Budgeting half of RecvLimit per Get() leaves margin for protobuf/gRPC
// envelope overhead; the clamp keeps the batch well above 1 (never a
// per-record lookup, T-05-04-03) while still forcing multiple accumulator
// flushes for FewLarge.
func reindexResumeSafeBatch(recordBytes int) uint32 {
	n := (storetest.RecvLimit / 2) / recordBytes
	if n < 2 {
		n = 2
	}
	if n > 256 {
		n = 256
	}
	return uint32(n)
}

// TestReindexBoundedOverGRPCLimit proves engram reindex drains an
// oversized source collection through the shared byte-budget iterator
// instead of the pre-migration 256-record ScrollAndOffset page — on both
// fixture shapes — and that the per-page target resume lookup and the
// effective-source override both still hold.
func TestReindexBoundedOverGRPCLimit(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		t.Run(shape.String(), func(t *testing.T) {
			c := storetest.Dial(t, storetest.RecvLimit)
			ctx := context.Background()

			srcName := store.PrefixedTestCollection("oversized_reindex_src_" + uuid.NewString())
			tgtName := store.PrefixedTestCollection("oversized_reindex_tgt_" + uuid.NewString())
			otherName := store.PrefixedTestCollection("oversized_reindex_other_" + uuid.NewString())
			tgt2Name := store.PrefixedTestCollection("oversized_reindex_tgt2_" + uuid.NewString())

			st := store.NewTestStore(t, c, srcName)
			if err := st.EnsureCollection(ctx, 3); err != nil {
				t.Fatalf("%s: EnsureCollection(source): %v", shape, err)
			}
			t.Cleanup(func() {
				for _, name := range []string{srcName, tgtName, otherName, tgt2Name} {
					if err := c.DeleteCollection(ctx, name); err != nil {
						t.Logf("%s: DeleteCollection(%q): %v (may not have been created)", shape, name, err)
					}
				}
			})

			// SeedOversized's records carry real content — exactly what
			// Reindex needs to re-embed.
			fx := storetest.SeedOversized(t, st, storetest.Spec{Limit: storetest.RecvLimit, Shape: shape, Vector: []float32{0.1, 0.2, 0.3}})
			batch := reindexResumeSafeBatch(fx.RecordBytes)

			// Sub-case 1: apply reindex into a fresh target. Before the
			// migration this call fails on FewLarge: one 256-record page
			// returns all 40 large records (~5.24 MiB) at once, overflowing
			// storetest.RecvLimit.
			res, err := st.Reindex(ctx, store.ReindexOptions{Target: tgtName, Dim: 4, Batch: batch}, oversizedReindexEmbed)
			if err != nil {
				t.Fatalf("%s: Reindex: %v (request shape may have exceeded the %d-byte named receive limit)", shape, err, storetest.RecvLimit)
			}
			if res.Scanned != uint64(len(fx.IDs)) {
				t.Errorf("%s: res.Scanned = %d, want %d", shape, res.Scanned, len(fx.IDs))
			}
			if res.Upserted != uint64(len(fx.IDs)) {
				t.Errorf("%s: res.Upserted = %d, want %d", shape, res.Upserted, len(fx.IDs))
			}
			targetCount, cerr := c.Count(ctx, &qdrant.CountPoints{CollectionName: tgtName, Exact: qdrant.PtrOf(true)})
			if cerr != nil {
				t.Fatalf("%s: Count(target): %v", shape, cerr)
			}
			if targetCount != uint64(len(fx.IDs)) {
				t.Errorf("%s: target collection count = %d, want %d", shape, targetCount, len(fx.IDs))
			}
			flushes := (uint32(len(fx.IDs)) + batch - 1) / batch
			t.Logf("%s: apply res=%+v target_count=%d batch=%d accumulator_flushes=%d", shape, res, targetCount, batch, flushes)

			// Sub-case 2: resume — a second run with Resume:true must see
			// the whole scope as Unchanged (Upserted=0), proving the
			// page-accumulated target lookup still sees the same snapshot
			// the single-RPC page fetch used to see (T-05-04-03).
			res2, err := st.Reindex(ctx, store.ReindexOptions{Target: tgtName, Dim: 4, Batch: batch, Resume: true}, oversizedReindexEmbed)
			if err != nil {
				t.Fatalf("%s: Reindex(Resume): %v", shape, err)
			}
			if res2.Unchanged != uint64(len(fx.IDs)) {
				t.Errorf("%s: resume res.Unchanged = %d, want %d", shape, res2.Unchanged, len(fx.IDs))
			}
			if res2.Upserted != 0 {
				t.Errorf("%s: resume res.Upserted = %d, want 0", shape, res2.Upserted)
			}

			// Sub-case 3: source override — a store constructed against a
			// DIFFERENT collection (otherName) still reads the effective
			// source (srcName) named via ReindexOptions.Source, proving the
			// walk never reads s.collection unconditionally. A walk that
			// read s.collection would scan zero records here.
			otherSt := store.NewTestStore(t, c, otherName)
			if err := otherSt.EnsureCollection(ctx, 3); err != nil {
				t.Fatalf("%s: EnsureCollection(other): %v", shape, err)
			}
			res3, err := otherSt.Reindex(ctx, store.ReindexOptions{Target: tgt2Name, Source: srcName, Dim: 4, Batch: batch}, oversizedReindexEmbed)
			if err != nil {
				t.Fatalf("%s: Reindex(Source override): %v", shape, err)
			}
			if res3.Scanned != uint64(len(fx.IDs)) {
				t.Errorf("%s: source-override res.Scanned = %d, want %d (walk must read the effective source, not s.collection)", shape, res3.Scanned, len(fx.IDs))
			}
		})
	}
}

