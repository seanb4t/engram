// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"testing"
	"time"
)

// newSpineTestStore returns a Store over a fresh, per-test collection —
// unlike testStore's shared "mem_eval_test" collection, ScanSpine sweeps
// the WHOLE collection with no owner or scope-required filter, so sharing
// a collection across tests would let one test's leftover records
// contaminate another's Total/Owners counts. Deleted before (in case a
// prior run left it behind) and after (via t.Cleanup).
func newSpineTestStore(t *testing.T, collection string, opts ...Option) *Store {
	t.Helper()
	c := dialTestClient(t)
	_ = c.DeleteCollection(context.Background(), collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })
	s := New(c, collection, opts...)
	if err := s.EnsureCollection(context.Background(), 3); err != nil {
		t.Fatalf("ensure collection %q: %v", collection, err)
	}
	return s
}

// seedSpineMemory upserts m with a small deterministic vector, failing the
// test on error.
func seedSpineMemory(t *testing.T, s *Store, m Memory) {
	t.Helper()
	if err := s.Upsert(context.Background(), m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed %s: %v", m.ID, err)
	}
}

// TestScanSpineEmptyCollection is the empty-spine behavior: Total 0 with
// non-nil, zero-length slices — never nil, so a JSON caller always sees
// "[]" rather than having to special-case "null".
func TestScanSpineEmptyCollection(t *testing.T) {
	s := newSpineTestStore(t, "spine_scan_empty")

	res, err := s.ScanSpine(context.Background(), SpineScanOptions{Scope: "spine_scan_empty_scope"})
	if err != nil {
		t.Fatalf("ScanSpine: %v", err)
	}
	if res.Total != 0 {
		t.Errorf("Total = %d, want 0", res.Total)
	}
	if res.ByScopeCategory == nil {
		t.Error("ByScopeCategory is nil, want non-nil empty slice")
	}
	if len(res.ByScopeCategory) != 0 {
		t.Errorf("ByScopeCategory = %v, want empty", res.ByScopeCategory)
	}
}

// TestScanSpineTwoOwners is the Subject-less proof: records belonging to
// two different owners are BOTH counted — Total is their sum and Owners is
// 2, never narrowed to one caller's bucket.
func TestScanSpineTwoOwners(t *testing.T) {
	s := newSpineTestStore(t, "spine_scan_two_owners")
	ctx := context.Background()
	const scope = "spine_scan_two_owners_scope"

	seedSpineMemory(t, s, Memory{
		ID: "a0000000-0000-0000-0000-000000000001", Content: "a", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(),
	})
	seedSpineMemory(t, s, Memory{
		ID: "a0000000-0000-0000-0000-000000000002", Content: "b", Scope: scope,
		Category: "note", Owner: "owner-b", CreatedAt: time.Now().UTC(),
	})

	res, err := s.ScanSpine(ctx, SpineScanOptions{Scope: scope})
	if err != nil {
		t.Fatalf("ScanSpine: %v", err)
	}
	if res.Total != 2 {
		t.Errorf("Total = %d, want 2", res.Total)
	}
	if res.Owners != 2 {
		t.Errorf("Owners = %d, want 2", res.Owners)
	}
}

// TestScanSpineHealthSignals seeds one record of every health-signal shape
// CONTEXT.md's discretion covers (summary present/absent, superseded,
// expired, scheduled, with/without citations) and asserts each bucket
// counts exactly what it claims to — including the recall-hidden ones
// (superseded, expired), which a Search/List-based sweep would silently
// drop.
func TestScanSpineHealthSignals(t *testing.T) {
	fixedNow := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := newSpineTestStore(t, "spine_scan_health", WithClock(func() time.Time { return fixedNow }))
	ctx := context.Background()
	const scope = "spine_scan_health_scope"

	past := fixedNow.Add(-time.Hour)
	future := fixedNow.Add(time.Hour)
	supersededBy := "a0000000-0000-0000-0000-000000000099"

	seedSpineMemory(t, s, Memory{
		ID: "b0000000-0000-0000-0000-000000000001", Content: "no summary", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: fixedNow,
	})
	seedSpineMemory(t, s, Memory{
		ID: "b0000000-0000-0000-0000-000000000002", Content: "has summary", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: fixedNow, Summary: "a summary",
	})
	seedSpineMemory(t, s, Memory{
		ID: "b0000000-0000-0000-0000-000000000003", Content: "superseded", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: fixedNow, SupersededBy: &supersededBy,
	})
	seedSpineMemory(t, s, Memory{
		ID: "b0000000-0000-0000-0000-000000000004", Content: "expired", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: fixedNow, NotAfter: &past,
	})
	seedSpineMemory(t, s, Memory{
		ID: "b0000000-0000-0000-0000-000000000005", Content: "scheduled", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: fixedNow, NotBefore: &future,
	})
	seedSpineMemory(t, s, Memory{
		ID: "b0000000-0000-0000-0000-000000000006", Content: "cited", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: fixedNow,
		Citations: []Citation{{Kind: "file", Ref: "a.go"}, {Kind: "file", Ref: "b.go"}},
	})

	res, err := s.ScanSpine(ctx, SpineScanOptions{Scope: scope})
	if err != nil {
		t.Fatalf("ScanSpine: %v", err)
	}
	if res.Total != 6 {
		t.Fatalf("Total = %d, want 6", res.Total)
	}
	if res.WithoutSummary != 5 {
		t.Errorf("WithoutSummary = %d, want 5", res.WithoutSummary)
	}
	if res.WithSummary != 1 {
		t.Errorf("WithSummary = %d, want 1", res.WithSummary)
	}
	if res.Superseded != 1 {
		t.Errorf("Superseded = %d, want 1", res.Superseded)
	}
	if res.Expired != 1 {
		t.Errorf("Expired = %d, want 1", res.Expired)
	}
	if res.Scheduled != 1 {
		t.Errorf("Scheduled = %d, want 1", res.Scheduled)
	}
	if res.WithCitations != 1 {
		t.Errorf("WithCitations = %d, want 1", res.WithCitations)
	}
	if res.Citations != 2 {
		t.Errorf("Citations = %d, want 2", res.Citations)
	}

	var breakdownTotal uint64
	for _, c := range res.ByScopeCategory {
		breakdownTotal += c.Count
	}
	if breakdownTotal != res.Total {
		t.Errorf("ByScopeCategory sums to %d, want %d (Total)", breakdownTotal, res.Total)
	}
}

// TestScanSpinePaginatesEveryPage is the pagination gate: with the scroll
// batch size forced to 1 and five records across two owners seeded, a
// correct sweep spans EVERY page (Total 5, Owners 2), not just the first
// (Total 1, Owners 1). This is a MUTATION CHECK, not a RED-first
// observation: Task 1 builds the correct paginated iterator before this
// test exists, so the failure state never arises naturally in task order.
// See the plan SUMMARY for the injected-defect failure line this test
// produced when scrollAllPoints' body was temporarily swapped for a single
// non-paginating s.client.Scroll call at the same limit.
func TestScanSpinePaginatesEveryPage(t *testing.T) {
	orig := spineScrollBatch
	spineScrollBatch = 1
	t.Cleanup(func() { spineScrollBatch = orig })

	s := newSpineTestStore(t, "spine_scan_pagination")
	ctx := context.Background()
	const scope = "spine_scan_pagination_scope"

	owners := []string{"owner-a", "owner-b", "owner-a", "owner-b", "owner-a"}
	for i, owner := range owners {
		seedSpineMemory(t, s, Memory{
			ID:      "c0000000-0000-0000-0000-00000000000" + string(rune('1'+i)),
			Content: "point", Scope: scope, Category: "note",
			Owner: owner, CreatedAt: time.Now().UTC(),
		})
	}

	res, err := s.ScanSpine(ctx, SpineScanOptions{Scope: scope})
	if err != nil {
		t.Fatalf("ScanSpine: %v", err)
	}
	if res.Total != 5 {
		t.Errorf("Total = %d, want 5 (batch size 1 must still cross every page)", res.Total)
	}
	if res.Owners != 2 {
		t.Errorf("Owners = %d, want 2 (pagination and Subject-lessness proven together)", res.Owners)
	}
}
