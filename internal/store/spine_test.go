// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

// newSpineTestStore returns a Store over a fresh, per-test collection —
// unlike testStore's shared "mem_eval_test" collection, ScanSpine sweeps
// the WHOLE collection with no owner or scope-required filter, so sharing
// a collection across tests would let one test's leftover records
// contaminate another's Total/Owners counts. Deleted before (in case a
// prior run left it behind) and after (via t.Cleanup). collection is
// namespaced through testCollection ONCE here, at the helper, so every one
// of its ~30 call sites keeps passing the bare descriptive name it passes
// today — the delete-before-create and t.Cleanup deletion below then both
// operate on the prefixed name automatically (plan 01-05).
func newSpineTestStore(t *testing.T, collection string, opts ...Option) *Store {
	t.Helper()
	collection = testCollection(collection)
	c := dialTestClient(t)
	_ = c.DeleteCollection(context.Background(), collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })
	s := newTestStore(t, c, collection, opts...)
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

// seedSpineMemoryVector upserts m with an explicit vector — used by the
// NearDuplicates fixtures below, which need control over each record's
// cosine relationship to its peers (near-identical, orthogonal, or
// anti-parallel/negative-scoring) rather than seedSpineMemory's shared
// constant vector.
func seedSpineMemoryVector(t *testing.T, s *Store, m Memory, vector []float32) {
	t.Helper()
	if err := s.Upsert(context.Background(), m, vector); err != nil {
		t.Fatalf("seed %s: %v", m.ID, err)
	}
}

// snapshotCollection captures s's collection's exact point count plus a
// deterministic digest of every point's id and payload — the before/after
// pair TestNearDuplicatesDoesNotMutate compares to prove NearDuplicates
// issues no write RPC on any path. Built over scrollAllPoints (this
// phase's one paginated iterator) rather than a second hand-rolled scroll
// loop, and over s.client.Count for the exact point count.
func snapshotCollection(t *testing.T, s *Store) (count uint64, digest string) {
	t.Helper()
	ctx := context.Background()
	n, err := s.client.Count(ctx, &qdrant.CountPoints{
		CollectionName: s.collection, Exact: qdrant.PtrOf(true),
	})
	if err != nil {
		t.Fatalf("snapshotCollection: Count: %v", err)
	}
	h := sha256.New()
	scanErr := s.scrollAllPoints(ctx, nil, qdrant.NewWithPayload(true), func(p *qdrant.RetrievedPoint) error {
		fmt.Fprintf(h, "id=%s|", p.Id.GetUuid())
		keys := make([]string, 0, len(p.Payload))
		for k := range p.Payload {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(h, "%s=%s;", k, p.Payload[k].String())
		}
		h.Write([]byte("\n"))
		return nil
	})
	if scanErr != nil {
		t.Fatalf("snapshotCollection: scroll: %v", scanErr)
	}
	return n, hex.EncodeToString(h.Sum(nil))
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
	seedSpineMemory(t, s, Memory{
		ID: "b0000000-0000-0000-0000-000000000007", Content: "archived", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: fixedNow, ArchivedAt: &fixedNow,
	})

	res, err := s.ScanSpine(ctx, SpineScanOptions{Scope: scope})
	if err != nil {
		t.Fatalf("ScanSpine: %v", err)
	}
	if res.Total != 7 {
		t.Fatalf("Total = %d, want 7", res.Total)
	}
	if res.WithoutSummary != 6 {
		t.Errorf("WithoutSummary = %d, want 6", res.WithoutSummary)
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
	if res.Archived != 1 {
		t.Errorf("Archived = %d, want 1 (a SEPARATE bucket from Expired, never folded into it)", res.Archived)
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

// TestCountExpiredAndPruneExpiredAgree is the boundary-fixture backstop for
// the structural "one filter, called twice" property expiredFilter/
// CountExpired establish (03-03-PLAN.md D-04): a record exactly AT the
// cutoff and one strictly PAST it. CountExpired's preview count and
// PruneExpired's deleted count must agree for the IDENTICAL before instant —
// the grep gate over expiredFilter/CountExpired's call sites is the primary
// proof this can never drift; this fixture is the backstop, not the reverse,
// since two independently drifted filters would still agree on most
// fixtures and only diverge at an edge exactly like this one.
func TestCountExpiredAndPruneExpiredAgree(t *testing.T) {
	fixedNow := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := newSpineTestStore(t, "spine_count_expired_agree")
	ctx := context.Background()
	const scope = "spine_count_expired_agree_scope"

	atCutoff := fixedNow
	pastCutoff := fixedNow.Add(-time.Second)
	futureCutoff := fixedNow.Add(time.Second)

	seedSpineMemory(t, s, Memory{
		ID: "f0000000-0000-0000-0000-000000000001", Content: "at cutoff", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: fixedNow, NotAfter: &atCutoff,
	})
	seedSpineMemory(t, s, Memory{
		ID: "f0000000-0000-0000-0000-000000000002", Content: "past cutoff", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: fixedNow, NotAfter: &pastCutoff,
	})
	seedSpineMemory(t, s, Memory{
		ID: "f0000000-0000-0000-0000-000000000003", Content: "not yet expired", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: fixedNow, NotAfter: &futureCutoff,
	})

	count, err := s.CountExpired(ctx, fixedNow)
	if err != nil {
		t.Fatalf("CountExpired: %v", err)
	}
	// expiredFilter is Lt (strictly before): only "past cutoff" (fixedNow-1s)
	// is < fixedNow; "at cutoff" (==fixedNow) is NOT strictly before it.
	if count != 1 {
		t.Fatalf("CountExpired(fixedNow) = %d, want 1 (only the strictly-past record)", count)
	}

	deleted, err := s.PruneExpired(ctx, fixedNow)
	if err != nil {
		t.Fatalf("PruneExpired: %v", err)
	}
	if deleted != count {
		t.Errorf("PruneExpired(fixedNow) = %d, want %d (must equal CountExpired's count for the identical before)", deleted, count)
	}
}

// TestPruneExpiredExcludesArchived (T-03-18's mitigation, D-12) pins that an
// archived record whose not_after has ALSO lapsed survives PruneExpired and
// is excluded from CountExpired's preview: archiving never touches
// not_after, and expiry alone must not sweep away a record an operator
// deliberately chose to retain via archive. The not_after predicate itself
// is unchanged — a second, ordinary-expired record in the same fixture is
// still pruned, proving this isn't a change to expiredFilter's range bound.
func TestPruneExpiredExcludesArchived(t *testing.T) {
	fixedNow := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := newSpineTestStore(t, "spine_prune_excludes_archived", WithClock(func() time.Time { return fixedNow }))
	ctx := context.Background()
	const scope = "spine_prune_excludes_archived_scope"

	pastCutoff := fixedNow.Add(-time.Hour)

	archivedID := "f1000000-0000-0000-0000-000000000001"
	expiredID := "f1000000-0000-0000-0000-000000000002"
	seedSpineMemory(t, s, Memory{
		ID: archivedID, Content: "archived and lapsed", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: fixedNow, NotAfter: &pastCutoff,
	})
	if _, err := s.Archive(ctx, archivedID); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	seedSpineMemory(t, s, Memory{
		ID: expiredID, Content: "just expired", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: fixedNow, NotAfter: &pastCutoff,
	})

	count, err := s.CountExpired(ctx, fixedNow)
	if err != nil {
		t.Fatalf("CountExpired: %v", err)
	}
	if count != 1 {
		t.Fatalf("CountExpired = %d, want 1 (the archived record must be excluded)", count)
	}

	deleted, err := s.PruneExpired(ctx, fixedNow)
	if err != nil {
		t.Fatalf("PruneExpired: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("PruneExpired deleted = %d, want 1", deleted)
	}

	if _, err := s.Get(ctx, archivedID); err != nil {
		t.Errorf("Get archived record after PruneExpired: %v, want it to survive", err)
	}
	if _, err := s.Get(ctx, expiredID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get expired record after PruneExpired: err = %v, want ErrNotFound", err)
	}
}

// TestEnumerateCitationsEmptyScope is the empty-spine behavior: a scope with
// no citation-bearing records returns a non-nil, zero-length slice.
func TestEnumerateCitationsEmptyScope(t *testing.T) {
	s := newSpineTestStore(t, "spine_enum_empty")
	res, err := s.EnumerateCitations(context.Background(), SpineScanOptions{Scope: "spine_enum_empty_scope"})
	if err != nil {
		t.Fatalf("EnumerateCitations: %v", err)
	}
	if res == nil {
		t.Error("EnumerateCitations returned nil, want a non-nil empty slice")
	}
	if len(res) != 0 {
		t.Errorf("EnumerateCitations = %v, want empty", res)
	}
}

// TestEnumerateCitationsExcludesUncited proves the server-side filter (and
// the payload-level guard behind it) actually narrows to citation-bearing
// records: a record with no citations at all must not appear.
func TestEnumerateCitationsExcludesUncited(t *testing.T) {
	s := newSpineTestStore(t, "spine_enum_uncited")
	ctx := context.Background()
	const scope = "spine_enum_uncited_scope"

	seedSpineMemory(t, s, Memory{
		ID: "d0000000-0000-0000-0000-000000000001", Content: "no citations", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(),
	})
	seedSpineMemory(t, s, Memory{
		ID: "d0000000-0000-0000-0000-000000000002", Content: "cited", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(),
		Citations: []Citation{{Kind: "file", Ref: "a.go"}},
	})

	res, err := s.EnumerateCitations(ctx, SpineScanOptions{Scope: scope})
	if err != nil {
		t.Fatalf("EnumerateCitations: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("EnumerateCitations returned %d records, want 1 (only the cited one)", len(res))
	}
	if res[0].ID != "d0000000-0000-0000-0000-000000000002" {
		t.Errorf("EnumerateCitations returned record %q, want the cited record", res[0].ID)
	}
}

// TestEnumerateCitationsIncludesSuperseded is the coverage-claim-spoofing
// regression (T-03-07's sibling mitigation): EnumerateCitations is
// Subject-less and built on scrollAllPoints, never Search/List, so a
// superseded record -- which recall soft-hides -- must still be enumerated.
func TestEnumerateCitationsIncludesSuperseded(t *testing.T) {
	s := newSpineTestStore(t, "spine_enum_superseded")
	ctx := context.Background()
	const scope = "spine_enum_superseded_scope"
	supersededBy := "d0000000-0000-0000-0000-000000000099"

	seedSpineMemory(t, s, Memory{
		ID: "d0000000-0000-0000-0000-000000000010", Content: "superseded but cited", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(),
		SupersededBy: &supersededBy,
		Citations:    []Citation{{Kind: "file", Ref: "a.go"}},
	})

	res, err := s.EnumerateCitations(ctx, SpineScanOptions{Scope: scope})
	if err != nil {
		t.Fatalf("EnumerateCitations: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("EnumerateCitations returned %d records, want 1 (recall-hidden but still enumerated)", len(res))
	}
}

// TestEnumerateCitationsPaginatesEveryPage is the pagination gate,
// mirroring TestScanSpinePaginatesEveryPage: with the scroll batch size
// forced to 1 and five citation-bearing records seeded, a correct sweep
// returns all five (count equality), not just the first page's worth.
func TestEnumerateCitationsPaginatesEveryPage(t *testing.T) {
	orig := spineScrollBatch
	spineScrollBatch = 1
	t.Cleanup(func() { spineScrollBatch = orig })

	s := newSpineTestStore(t, "spine_enum_pagination")
	ctx := context.Background()
	const scope = "spine_enum_pagination_scope"

	for i := 0; i < 5; i++ {
		seedSpineMemory(t, s, Memory{
			ID:      "d000000" + string(rune('1'+i)) + "-0000-0000-0000-000000000001",
			Content: "point", Scope: scope, Category: "note",
			Owner: "owner-a", CreatedAt: time.Now().UTC(),
			Citations: []Citation{{Kind: "file", Ref: "a.go"}},
		})
	}

	res, err := s.EnumerateCitations(ctx, SpineScanOptions{Scope: scope})
	if err != nil {
		t.Fatalf("EnumerateCitations: %v", err)
	}
	if len(res) != 5 {
		t.Errorf("EnumerateCitations returned %d records, want 5 (batch size 1 must still cross every page)", len(res))
	}
}

// TestNearDuplicatesEmptyScope is the empty-spine behavior: a scope with no
// records returns a non-nil, zero-length slice and a nil error.
func TestNearDuplicatesEmptyScope(t *testing.T) {
	s := newSpineTestStore(t, "spine_nd_empty")
	pairs, err := s.NearDuplicates(context.Background(), NearDuplicateOptions{Scope: "spine_nd_empty_scope"})
	if err != nil {
		t.Fatalf("NearDuplicates: %v", err)
	}
	if pairs == nil {
		t.Error("NearDuplicates returned nil, want a non-nil empty slice")
	}
	if len(pairs) != 0 {
		t.Errorf("NearDuplicates = %v, want empty", pairs)
	}
}

// TestNearDuplicatesSingleRecord proves a lone record in scope has no
// partner: MustNot(NewHasID(id)) excludes it from its own query, and there
// is nothing else in scope to pair it with.
func TestNearDuplicatesSingleRecord(t *testing.T) {
	s := newSpineTestStore(t, "spine_nd_single")
	ctx := context.Background()
	const scope = "spine_nd_single_scope"

	seedSpineMemoryVector(t, s, Memory{
		ID: "aa000000-0000-0000-0000-000000000001", Content: "a", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(),
	}, []float32{1, 0, 0})

	pairs, err := s.NearDuplicates(ctx, NearDuplicateOptions{Scope: scope})
	if err != nil {
		t.Fatalf("NearDuplicates: %v", err)
	}
	if len(pairs) != 0 {
		t.Errorf("NearDuplicates with 1 record = %v, want empty (a single record has no partner)", pairs)
	}
}

// TestNearDuplicatesTwoNearIdentical is the collapse gate: two
// near-identical records in scope produce EXACTLY ONE pair (not two, from
// the two-sided sweep), carrying the cosine score Qdrant reports.
func TestNearDuplicatesTwoNearIdentical(t *testing.T) {
	s := newSpineTestStore(t, "spine_nd_two_near")
	ctx := context.Background()
	const scope = "spine_nd_two_near_scope"

	seedSpineMemoryVector(t, s, Memory{
		ID: "ab000000-0000-0000-0000-000000000001", Content: "a", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(), ShortID: "sidab001",
	}, []float32{1, 0, 0})
	seedSpineMemoryVector(t, s, Memory{
		ID: "ab000000-0000-0000-0000-000000000002", Content: "b", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(), ShortID: "sidab002",
	}, []float32{0.99, 0.1, 0})

	pairs, err := s.NearDuplicates(ctx, NearDuplicateOptions{Scope: scope})
	if err != nil {
		t.Fatalf("NearDuplicates: %v", err)
	}
	if len(pairs) != 1 {
		t.Fatalf("NearDuplicates returned %d pairs, want 1 (the two-sided sweep must collapse (A,B)/(B,A))", len(pairs))
	}
	if pairs[0].Score < 0.9 {
		t.Errorf("Score = %v, want > 0.9 for two near-identical vectors", pairs[0].Score)
	}
	wantKey := orderedPairKey("ab000000-0000-0000-0000-000000000001", "ab000000-0000-0000-0000-000000000002")
	if pairs[0].A != wantKey[0] || pairs[0].B != wantKey[1] {
		t.Errorf("pair = (%s, %s), want (%s, %s)", pairs[0].A, pairs[0].B, wantKey[0], wantKey[1])
	}
	if pairs[0].AShortID == "" || pairs[0].BShortID == "" {
		t.Errorf("pair = %+v, want both short ids populated", pairs[0])
	}
	if pairs[0].AScope != scope || pairs[0].BScope != scope {
		t.Errorf("pair scopes = (%q, %q), want (%q, %q)", pairs[0].AScope, pairs[0].BScope, scope, scope)
	}
}

// TestNearDuplicatesExcludesOutOfScope proves a scoped sweep never crosses
// scope boundaries: a near-identical partner in a DIFFERENT scope must not
// appear in the pair list.
func TestNearDuplicatesExcludesOutOfScope(t *testing.T) {
	s := newSpineTestStore(t, "spine_nd_scope_excl")
	ctx := context.Background()
	const scopeA = "spine_nd_scope_excl_a"
	const scopeB = "spine_nd_scope_excl_b"

	seedSpineMemoryVector(t, s, Memory{
		ID: "ac000000-0000-0000-0000-000000000001", Content: "a", Scope: scopeA,
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(),
	}, []float32{1, 0, 0})
	seedSpineMemoryVector(t, s, Memory{
		ID: "ac000000-0000-0000-0000-000000000002", Content: "b", Scope: scopeB,
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(),
	}, []float32{0.99, 0.1, 0})

	pairs, err := s.NearDuplicates(ctx, NearDuplicateOptions{Scope: scopeA})
	if err != nil {
		t.Fatalf("NearDuplicates: %v", err)
	}
	if len(pairs) != 0 {
		t.Errorf("NearDuplicates(scope=%q) returned %d pairs, want 0 (the near-identical partner is in a different scope)", scopeA, len(pairs))
	}
}

// TestNearDuplicatesAllScopesSpansScopes proves AllScopes:true genuinely
// spans every scope, and that a cross-scope pair names BOTH distinct
// scopes on its row.
//
// This is a MUTATION CHECK, not a RED-first observation: this plan builds
// the AllScopes bool design directly, so the rejected empty-string-scope
// encoding is never built in task order and its failure state cannot arise
// naturally. See the plan SUMMARY for the injected-defect failure line
// this test produced when NearDuplicates' all-scopes path was temporarily
// changed to pass Scope:"" as a filter Must condition instead of omitting
// the scope condition entirely.
func TestNearDuplicatesAllScopesSpansScopes(t *testing.T) {
	s := newSpineTestStore(t, "spine_nd_all_scopes")
	ctx := context.Background()
	const scopeA = "spine_nd_all_scopes_a"
	const scopeB = "spine_nd_all_scopes_b"

	seedSpineMemoryVector(t, s, Memory{
		ID: "ad000000-0000-0000-0000-000000000001", Content: "a", Scope: scopeA,
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(),
	}, []float32{1, 0, 0})
	seedSpineMemoryVector(t, s, Memory{
		ID: "ad000000-0000-0000-0000-000000000002", Content: "b", Scope: scopeB,
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(),
	}, []float32{0.99, 0.1, 0})

	pairs, err := s.NearDuplicates(ctx, NearDuplicateOptions{AllScopes: true})
	if err != nil {
		t.Fatalf("NearDuplicates: %v", err)
	}
	if len(pairs) != 1 {
		t.Fatalf("NearDuplicates(AllScopes) returned %d pairs, want 1", len(pairs))
	}
	if pairs[0].AScope == pairs[0].BScope {
		t.Errorf("pair scopes = (%q, %q), want two DIFFERENT scopes", pairs[0].AScope, pairs[0].BScope)
	}
	gotScopes := map[string]bool{pairs[0].AScope: true, pairs[0].BScope: true}
	if !gotScopes[scopeA] || !gotScopes[scopeB] {
		t.Errorf("pair scopes = %v, want {%q, %q}", gotScopes, scopeA, scopeB)
	}
}

// TestNearDuplicatesAllScopesFalseEmptyScopeReturnsEmpty is the settled
// contract for the omitted-both-flags case: AllScopes:false with Scope:""
// applies a literal scope=="" match condition, which no real record
// satisfies, so the result is empty rather than an accidental unfiltered
// whole-spine sweep.
func TestNearDuplicatesAllScopesFalseEmptyScopeReturnsEmpty(t *testing.T) {
	s := newSpineTestStore(t, "spine_nd_empty_scope_opt")
	ctx := context.Background()

	seedSpineMemoryVector(t, s, Memory{
		ID: "ae000000-0000-0000-0000-000000000001", Content: "a", Scope: "spine_nd_empty_scope_opt_real",
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(),
	}, []float32{1, 0, 0})
	seedSpineMemoryVector(t, s, Memory{
		ID: "ae000000-0000-0000-0000-000000000002", Content: "b", Scope: "spine_nd_empty_scope_opt_real",
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(),
	}, []float32{0.99, 0.1, 0})

	pairs, err := s.NearDuplicates(ctx, NearDuplicateOptions{AllScopes: false, Scope: ""})
	if err != nil {
		t.Fatalf("NearDuplicates: %v", err)
	}
	if len(pairs) != 0 {
		t.Errorf("NearDuplicates(AllScopes:false, Scope:\"\") returned %d pairs, want 0 (no record has a literally-empty scope)", len(pairs))
	}
}

// TestNearDuplicatesAllScopesTrueWithScopeRejected proves the two options
// cannot silently disagree: AllScopes:true combined with a non-empty Scope
// is rejected with ErrInvalidArgument.
func TestNearDuplicatesAllScopesTrueWithScopeRejected(t *testing.T) {
	s := newSpineTestStore(t, "spine_nd_reject_combo")

	_, err := s.NearDuplicates(context.Background(), NearDuplicateOptions{AllScopes: true, Scope: "x"})
	if err == nil {
		t.Fatal("expected an error for AllScopes:true with a non-empty Scope, got nil")
	}
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("error = %v, want it to wrap ErrInvalidArgument", err)
	}
}

// TestNearDuplicatesMinScoreOptionIsPointer is the type-level no-filter
// gate: MinScore must be a pointer, asserted via reflection rather than by
// grepping the struct's declaration line, so a future refactor that
// changes the field's type without updating this test fails loudly.
func TestNearDuplicatesMinScoreOptionIsPointer(t *testing.T) {
	field, ok := reflect.TypeOf(NearDuplicateOptions{}).FieldByName("MinScore")
	if !ok {
		t.Fatal("NearDuplicateOptions has no MinScore field")
	}
	if field.Type.Kind() != reflect.Pointer {
		t.Errorf("MinScore field type = %v, want a pointer (nil must be able to mean \"no filter\", distinct from a float32 zero value)", field.Type)
	}
}

// TestNearDuplicatesNoMinScoreReportsNegativePair is the behavioural
// no-filter gate: cosine similarity is negative-capable, and a pair whose
// score is negative must still be reported when MinScore is nil. A
// MinScore just below the pair's own score still reports it; just above
// drops it.
func TestNearDuplicatesNoMinScoreReportsNegativePair(t *testing.T) {
	s := newSpineTestStore(t, "spine_nd_negative")
	ctx := context.Background()
	const scope = "spine_nd_negative_scope"

	seedSpineMemoryVector(t, s, Memory{
		ID: "af000000-0000-0000-0000-000000000001", Content: "a", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(),
	}, []float32{1, 0, 0})
	seedSpineMemoryVector(t, s, Memory{
		ID: "af000000-0000-0000-0000-000000000002", Content: "b", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(),
	}, []float32{-1, 0, 0})

	pairs, err := s.NearDuplicates(ctx, NearDuplicateOptions{Scope: scope})
	if err != nil {
		t.Fatalf("NearDuplicates: %v", err)
	}
	if len(pairs) != 1 {
		t.Fatalf("NearDuplicates returned %d pairs, want 1", len(pairs))
	}
	if pairs[0].Score >= 0 {
		t.Fatalf("Score = %v, want negative (anti-parallel vectors)", pairs[0].Score)
	}
	gotScore := pairs[0].Score

	below := gotScore - 0.001
	pairsBelow, err := s.NearDuplicates(ctx, NearDuplicateOptions{Scope: scope, MinScore: &below})
	if err != nil {
		t.Fatalf("NearDuplicates(MinScore below): %v", err)
	}
	if len(pairsBelow) != 1 {
		t.Errorf("NearDuplicates(MinScore=%v) returned %d pairs, want 1 (still at/above threshold)", below, len(pairsBelow))
	}

	above := gotScore + 0.001
	pairsAbove, err := s.NearDuplicates(ctx, NearDuplicateOptions{Scope: scope, MinScore: &above})
	if err != nil {
		t.Fatalf("NearDuplicates(MinScore above): %v", err)
	}
	if len(pairsAbove) != 0 {
		t.Errorf("NearDuplicates(MinScore=%v) returned %d pairs, want 0 (below threshold)", above, len(pairsAbove))
	}
}

// TestNearDuplicatesThreeRecordsExactlyOnePair seeds three records where
// two are near-identical and one is distant, and proves a MinScore between
// the two clusters' scores narrows the report to exactly the near-identical
// pair, in the deterministic id-ordered tiebreak.
func TestNearDuplicatesThreeRecordsExactlyOnePair(t *testing.T) {
	s := newSpineTestStore(t, "spine_nd_three")
	ctx := context.Background()
	const scope = "spine_nd_three_scope"

	seedSpineMemoryVector(t, s, Memory{
		ID: "f1000000-0000-0000-0000-000000000001", Content: "a", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(), ShortID: "sidf1001",
	}, []float32{1, 0, 0})
	seedSpineMemoryVector(t, s, Memory{
		ID: "f2000000-0000-0000-0000-000000000002", Content: "b", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(), ShortID: "sidf2002",
	}, []float32{0.99, 0.1, 0})
	seedSpineMemoryVector(t, s, Memory{
		ID: "f3000000-0000-0000-0000-000000000003", Content: "c", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(), ShortID: "sidf3003",
	}, []float32{0, 1, 0})

	threshold := float32(0.5)
	pairs, err := s.NearDuplicates(ctx, NearDuplicateOptions{Scope: scope, MinScore: &threshold})
	if err != nil {
		t.Fatalf("NearDuplicates: %v", err)
	}
	if len(pairs) != 1 {
		t.Fatalf("NearDuplicates returned %d pairs, want 1 (only the near-identical pair clears MinScore=0.5)", len(pairs))
	}
	wantKey := orderedPairKey("f1000000-0000-0000-0000-000000000001", "f2000000-0000-0000-0000-000000000002")
	if pairs[0].A != wantKey[0] || pairs[0].B != wantKey[1] {
		t.Errorf("pair = (%s, %s), want (%s, %s) — the deterministic id-ordered tiebreak", pairs[0].A, pairs[0].B, wantKey[0], wantKey[1])
	}
}

// TestNearDuplicatesPaginatesEveryPage is the pagination gate, mirroring
// TestScanSpinePaginatesEveryPage: with the scroll batch size forced to 1
// and five records seeded in one scope, the Progress callback's FINAL
// scanned/queried counts must both equal 5 — a count equality, not a grep
// for NewQueryID's presence, which would pass on a first-page-only
// enumeration too.
func TestNearDuplicatesPaginatesEveryPage(t *testing.T) {
	orig := spineScrollBatch
	spineScrollBatch = 1
	t.Cleanup(func() { spineScrollBatch = orig })

	s := newSpineTestStore(t, "spine_nd_pagination")
	ctx := context.Background()
	const scope = "spine_nd_pagination_scope"

	for i := 0; i < 5; i++ {
		seedSpineMemoryVector(t, s, Memory{
			ID:      "d000000" + string(rune('1'+i)) + "-0000-0000-0000-000000000002",
			Content: "point", Scope: scope, Category: "note",
			Owner: "owner-a", CreatedAt: time.Now().UTC(),
		}, []float32{float32(i), 1, 0})
	}

	var lastScanned, lastQueried uint64
	_, err := s.NearDuplicates(ctx, NearDuplicateOptions{
		Scope: scope,
		Progress: func(scanned, queried uint64) {
			lastScanned, lastQueried = scanned, queried
		},
	})
	if err != nil {
		t.Fatalf("NearDuplicates: %v", err)
	}
	if lastScanned != 5 {
		t.Errorf("final Progress scanned = %d, want 5 (batch size 1 must still cross every page)", lastScanned)
	}
	if lastQueried != 5 {
		t.Errorf("final Progress queried = %d, want 5 (every enumerated id must be queried)", lastQueried)
	}
}

// TestNearDuplicatesDoesNotMutate is the T-03-16 gate: NearDuplicates
// issues no write RPC on any path. Captures the collection's exact point
// count and a payload digest before and after the call and asserts both
// are byte-identical.
func TestNearDuplicatesDoesNotMutate(t *testing.T) {
	s := newSpineTestStore(t, "spine_nd_no_mutate")
	ctx := context.Background()
	const scope = "spine_nd_no_mutate_scope"

	seedSpineMemoryVector(t, s, Memory{
		ID: "a3000000-0000-0000-0000-000000000001", Content: "a", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(),
	}, []float32{1, 0, 0})
	seedSpineMemoryVector(t, s, Memory{
		ID: "a4000000-0000-0000-0000-000000000002", Content: "b", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(),
	}, []float32{0.99, 0.1, 0})

	beforeCount, beforeDigest := snapshotCollection(t, s)
	if _, err := s.NearDuplicates(ctx, NearDuplicateOptions{Scope: scope}); err != nil {
		t.Fatalf("NearDuplicates: %v", err)
	}
	afterCount, afterDigest := snapshotCollection(t, s)

	if beforeCount != afterCount {
		t.Errorf("point count changed: before=%d after=%d", beforeCount, afterCount)
	}
	if beforeDigest != afterDigest {
		t.Errorf("payload digest changed: before=%s after=%s", beforeDigest, afterDigest)
	}
}

// TestNearDuplicatesIsDeterministic runs NearDuplicates twice over the
// same seeded data and asserts the returned slices are deeply equal,
// proving the sort/tiebreak makes the result order-stable across runs.
func TestNearDuplicatesIsDeterministic(t *testing.T) {
	s := newSpineTestStore(t, "spine_nd_deterministic")
	ctx := context.Background()
	const scope = "spine_nd_deterministic_scope"

	seedSpineMemoryVector(t, s, Memory{
		ID: "a5000000-0000-0000-0000-000000000001", Content: "a", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(), ShortID: "sida5001",
	}, []float32{1, 0, 0})
	seedSpineMemoryVector(t, s, Memory{
		ID: "a6000000-0000-0000-0000-000000000002", Content: "b", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(), ShortID: "sida6002",
	}, []float32{0.99, 0.1, 0})
	seedSpineMemoryVector(t, s, Memory{
		ID: "a7000000-0000-0000-0000-000000000003", Content: "c", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(), ShortID: "sida7003",
	}, []float32{0, 1, 0})

	first, err := s.NearDuplicates(ctx, NearDuplicateOptions{Scope: scope})
	if err != nil {
		t.Fatalf("NearDuplicates (first): %v", err)
	}
	second, err := s.NearDuplicates(ctx, NearDuplicateOptions{Scope: scope})
	if err != nil {
		t.Fatalf("NearDuplicates (second): %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("two runs over the same data returned different results:\nfirst:  %+v\nsecond: %+v", first, second)
	}
}

// TestNearDuplicatesPayloadFrugality is the T-03-28 gate: a distinctive
// content marker seeded on both records of a reported pair must appear
// NOWHERE in the values NearDuplicates returns — every field on
// DuplicatePair is an identifier (id, short id, scope) or a score, never
// content. WithPayload is never set on the per-id QueryPoints call at all
// (every neighbour's identity is resolved from the id-enumeration map), so
// this also proves no incremental payload — let alone content — is ever
// fetched at query time.
func TestNearDuplicatesPayloadFrugality(t *testing.T) {
	s := newSpineTestStore(t, "spine_nd_frugal")
	ctx := context.Background()
	const scope = "spine_nd_frugal_scope"
	const marker = "UNIQUE_CONTENT_MARKER_do_not_leak_9f3c1a"

	seedSpineMemoryVector(t, s, Memory{
		ID: "a8000000-0000-0000-0000-000000000001", Content: marker, Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(), ShortID: "sida8001",
	}, []float32{1, 0, 0})
	seedSpineMemoryVector(t, s, Memory{
		ID: "a9000000-0000-0000-0000-000000000002", Content: marker + "-partner", Scope: scope,
		Category: "note", Owner: "owner-a", CreatedAt: time.Now().UTC(), ShortID: "sida9002",
	}, []float32{0.99, 0.1, 0})

	pairs, err := s.NearDuplicates(ctx, NearDuplicateOptions{Scope: scope})
	if err != nil {
		t.Fatalf("NearDuplicates: %v", err)
	}
	if len(pairs) != 1 {
		t.Fatalf("NearDuplicates returned %d pairs, want 1", len(pairs))
	}
	p := pairs[0]
	for _, v := range []string{p.A, p.B, p.AShortID, p.BShortID, p.AScope, p.BScope} {
		if strings.Contains(v, marker) {
			t.Errorf("returned field %q contains the seeded content marker — content leaked through the near-duplicate sweep", v)
		}
	}
	if p.AShortID == "" || p.BShortID == "" {
		t.Errorf("pair = %+v, want both short ids populated", p)
	}
}

// --- Purge (03-07-PLAN.md) ---------------------------------------------

// TestPurgeFilterPathActive pins the ONE predicate derivePurgeEligible and
// the CLI leaf both read to decide whether D-10's free-form filter path is
// engaged: category/tags always engage it; older-than engages it ONLY when
// no class is selected (alongside a class it is that class's own window
// override instead).
func TestPurgeFilterPathActive(t *testing.T) {
	cases := []struct {
		name string
		opts PurgeOptions
		want bool
	}{
		{"nothing", PurgeOptions{}, false},
		{"class only", PurgeOptions{Classes: []PurgeClass{PurgeClassArchived}}, false},
		{"class plus older-than is a window override", PurgeOptions{Classes: []PurgeClass{PurgeClassArchived}, OlderThan: time.Hour}, false},
		{"category alone", PurgeOptions{Category: "decision"}, true},
		{"tags alone", PurgeOptions{Tags: []string{"x"}}, true},
		{"older-than alone (no class)", PurgeOptions{OlderThan: time.Hour}, true},
		// The asymmetry is deliberate and was previously untested. A class
		// merely parameterized by --older-than stays a derivation, so it does
		// NOT engage the harder gate. Narrowing that same class by category
		// or tag is a semantic judgment no structural class expresses, so it
		// DOES -- even though the resulting run is strictly narrower. D-10
		// gates on how much judgment the operator supplied, not on how many
		// records the run would touch.
		{"class plus category still engages the gate", PurgeOptions{Classes: []PurgeClass{PurgeClassArchived}, Category: "decision"}, true},
		{"class plus tags still engages the gate", PurgeOptions{Classes: []PurgeClass{PurgeClassSuperseded}, Tags: []string{"x"}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PurgeFilterPathActive(tc.opts); got != tc.want {
				t.Errorf("PurgeFilterPathActive(%+v) = %v, want %v", tc.opts, got, tc.want)
			}
		})
	}
}

// TestCheckExtractGate is a pure (no Qdrant) table test over
// checkExtractGate's two paths, run directly against hand-built
// purgeCandidate/milestoneSummaryRecord values -- no store, no RPC.
func TestCheckExtractGate(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	successor := "11111111-0000-0000-0000-000000000001"

	t.Run("empty candidate set passes trivially", func(t *testing.T) {
		if err := checkExtractGate(nil, nil); err != nil {
			t.Fatalf("checkExtractGate(nil, nil) = %v, want nil", err)
		}
	})

	t.Run("per-record link satisfies the gate", func(t *testing.T) {
		cands := []purgeCandidate{{
			ID: "c1", Scope: "s", CreatedAt: base,
			SupersededBy: &successor, SuccessorExists: true, SuccessorCreatedAt: base.Add(time.Hour),
		}}
		if err := checkExtractGate(cands, nil); err != nil {
			t.Fatalf("checkExtractGate with a valid successor = %v, want nil", err)
		}
	})

	t.Run("nonexistent extraction-link target fails the gate", func(t *testing.T) {
		cands := []purgeCandidate{{
			ID: "c1", Scope: "s", CreatedAt: base,
			SupersededBy: &successor, SuccessorExists: false,
		}}
		if err := checkExtractGate(cands, nil); err == nil {
			t.Fatal("checkExtractGate with a nonexistent successor = nil, want an error")
		}
	})

	t.Run("no link and no qualifying summary fails, deletes nothing", func(t *testing.T) {
		cands := []purgeCandidate{{ID: "c1", Scope: "s", CreatedAt: base}}
		if err := checkExtractGate(cands, nil); err == nil {
			t.Fatal("checkExtractGate with no link and no summary = nil, want an error")
		}
	})

	t.Run("qualifying milestone summary satisfies the batch floor", func(t *testing.T) {
		cands := []purgeCandidate{{ID: "c1", Scope: "s", CreatedAt: base}}
		ms := []milestoneSummaryRecord{{ID: "m1", Scope: "s", CreatedAt: base.Add(time.Hour)}}
		if err := checkExtractGate(cands, ms); err != nil {
			t.Fatalf("checkExtractGate with a qualifying summary = %v, want nil", err)
		}
	})

	t.Run("milestone summary predating the newest candidate fails the gate", func(t *testing.T) {
		cands := []purgeCandidate{
			{ID: "c1", Scope: "s", CreatedAt: base},
			{ID: "c2", Scope: "s", CreatedAt: base.Add(2 * time.Hour)}, // newest
		}
		ms := []milestoneSummaryRecord{{ID: "m1", Scope: "s", CreatedAt: base.Add(time.Hour)}} // predates c2
		if err := checkExtractGate(cands, ms); err == nil {
			t.Fatal("checkExtractGate with a summary older than the newest candidate = nil, want an error")
		}
	})

	t.Run("milestone summary in a different scope does not cover a candidate", func(t *testing.T) {
		cands := []purgeCandidate{{ID: "c1", Scope: "scope-a", CreatedAt: base}}
		ms := []milestoneSummaryRecord{{ID: "m1", Scope: "scope-b", CreatedAt: base.Add(time.Hour)}}
		if err := checkExtractGate(cands, ms); err == nil {
			t.Fatal("checkExtractGate with a cross-scope summary = nil, want an error")
		}
	})
}

// TestExtractGateIgnoresCallerSuppliedLinkTag is the gate against reverting
// checkExtractGate's per-record path to a caller-mintable tag.
//
// This is a MUTATION CHECK (inject-and-revert), not RED-first: Task 2 writes
// the SupersededBy-reading gate directly, so the tag-reading failure state
// never arises naturally in task order. See the plan SUMMARY for the
// observed injected-defect failure line.
func TestExtractGateIgnoresCallerSuppliedLinkTag(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// A candidate carrying ONLY a caller-supplied tag naming a successor
	// (SupersededBy left nil) must NOT pass the per-record path.
	tagOnly := purgeCandidate{ID: "c1", Scope: "s", CreatedAt: base}
	if err := checkExtractGate([]purgeCandidate{tagOnly}, nil); err == nil {
		t.Fatal("a candidate with no SupersededBy (only a hypothetical caller tag) passed the gate -- " +
			"the per-record path must read the server-set link, never a tag")
	}

	// The real, server-set link DOES pass.
	successor := "22222222-0000-0000-0000-000000000002"
	linked := purgeCandidate{
		ID: "c2", Scope: "s", CreatedAt: base,
		SupersededBy: &successor, SuccessorExists: true, SuccessorCreatedAt: base.Add(time.Hour),
	}
	if err := checkExtractGate([]purgeCandidate{linked}, nil); err != nil {
		t.Fatalf("a candidate with a real SupersededBy link failed the gate: %v", err)
	}
}

// purgeTestNow anchors every purge boundary test's clock so "at cutoff" and
// "past cutoff" fixtures are computed the same way TestCountExpiredAndPruneExpiredAgree's
// sibling fixtures are, and so PreviewPurge/ApplyPurge (each independently
// resolving opts.Now) agree on the SAME instant when the caller supplies it
// explicitly.
var purgeTestNow = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// TestPreviewPurgeEmptyCandidateSet: a scope with nothing eligible previews
// a verified, zero-id manifest, and ApplyPurge against it deletes nothing
// and returns success.
func TestPreviewPurgeEmptyCandidateSet(t *testing.T) {
	s := newSpineTestStore(t, "spine_purge_empty")
	ctx := context.Background()
	const scope = "spine_purge_empty_scope"

	opts := PurgeOptions{Classes: []PurgeClass{PurgeClassExpired}, Scope: scope, Now: purgeTestNow}
	manifest, err := s.PreviewPurge(ctx, opts)
	if err != nil {
		t.Fatalf("PreviewPurge: %v", err)
	}
	if !manifest.IsVerified() {
		t.Fatal("PreviewPurge returned an unverified manifest")
	}
	if len(manifest.IDs()) != 0 {
		t.Fatalf("IDs() = %v, want empty", manifest.IDs())
	}

	res, err := s.ApplyPurge(ctx, manifest, opts)
	if err != nil {
		t.Fatalf("ApplyPurge: %v", err)
	}
	if len(res.Deleted) != 0 || len(res.Spared) != 0 || len(res.Appeared) != 0 {
		t.Errorf("ApplyPurge on an empty manifest = %+v, want all-empty", res)
	}
}

// TestPurgeSupersededPastGraceSelfSatisfiesGate proves PurgeClassSuperseded's
// named self-satisfying property (checkExtractGate's per-record path reads
// the SAME SupersededBy field the class itself requires to classify a
// record eligible): a superseded record whose successor exists and
// postdates it, past the grace window, previews and applies cleanly with no
// extra artifact.
func TestPurgeSupersededPastGraceSelfSatisfiesGate(t *testing.T) {
	s := newSpineTestStore(t, "spine_purge_superseded")
	ctx := context.Background()
	const scope = "spine_purge_superseded_scope"

	successorID := "60000000-0000-0000-0000-000000000002"
	targetID := "60000000-0000-0000-0000-000000000001"
	seedSpineMemory(t, s, Memory{
		ID: successorID, Content: "successor", Scope: scope, Category: "decision",
		Owner: "owner-a", CreatedAt: purgeTestNow.Add(-2 * time.Hour),
	})
	seedSpineMemory(t, s, Memory{
		ID: targetID, Content: "superseded", Scope: scope, Category: "decision",
		Owner: "owner-a", CreatedAt: purgeTestNow.Add(-3 * time.Hour), SupersededBy: &successorID,
	})

	opts := PurgeOptions{Classes: []PurgeClass{PurgeClassSuperseded}, Scope: scope, Now: purgeTestNow}
	manifest, err := s.PreviewPurge(ctx, opts)
	if err != nil {
		t.Fatalf("PreviewPurge: %v", err)
	}
	if ids := manifest.IDs(); len(ids) != 1 || ids[0] != targetID {
		t.Fatalf("PreviewPurge IDs = %v, want [%s]", ids, targetID)
	}

	res, err := s.ApplyPurge(ctx, manifest, opts)
	if err != nil {
		t.Fatalf("ApplyPurge: %v", err)
	}
	if len(res.Deleted) != 1 || res.Deleted[0] != targetID {
		t.Fatalf("Deleted = %v, want [%s]", res.Deleted, targetID)
	}
	if _, gerr := s.Get(ctx, targetID); !errors.Is(gerr, ErrNotFound) {
		t.Errorf("target still present after ApplyPurge: err=%v", gerr)
	}
	if _, gerr := s.Get(ctx, successorID); gerr != nil {
		t.Errorf("successor was deleted (it was never a candidate): %v", gerr)
	}
}

// TestPurgeBatchFloorRequiresNewerMilestoneSummary covers D-09's batch floor:
// a candidate with no per-record link is blocked without a qualifying
// milestone-summary record, and passes once one exists in-scope with a
// later CreatedAt -- which is itself excluded from the deleted set even
// though it independently matches the selected class/filter.
func TestPurgeBatchFloorRequiresNewerMilestoneSummary(t *testing.T) {
	s := newSpineTestStore(t, "spine_purge_batch_floor")
	ctx := context.Background()
	const scope = "spine_purge_batch_floor_scope"

	past := purgeTestNow.Add(-48 * time.Hour)
	candidateID := "61000000-0000-0000-0000-000000000001"
	seedSpineMemory(t, s, Memory{
		ID: candidateID, Content: "no link", Scope: scope, Category: "decision",
		Owner: "owner-a", CreatedAt: purgeTestNow.Add(-72 * time.Hour), NotAfter: &past,
	})

	opts := PurgeOptions{Classes: []PurgeClass{PurgeClassExpired}, Scope: scope, Now: purgeTestNow}

	// No milestone-summary record yet: the gate blocks, nothing is deleted.
	if _, err := s.PreviewPurge(ctx, opts); err == nil {
		t.Fatal("PreviewPurge with no per-record link and no milestone summary = nil error, want the extract gate to fail")
	}
	if _, gerr := s.Get(ctx, candidateID); gerr != nil {
		t.Fatalf("candidate missing after a FAILED preview: %v", gerr)
	}

	// Seed a qualifying milestone-summary record: same scope, later
	// CreatedAt than the candidate, carrying the marker tag. Its own
	// NotAfter also lapses (so it would independently match the filter
	// too) -- it must still never be deleted.
	summaryID := "61000000-0000-0000-0000-000000000002"
	summaryPast := purgeTestNow.Add(-1 * time.Hour)
	seedSpineMemory(t, s, Memory{
		ID: summaryID, Content: "milestone summary", Scope: scope, Category: "decision",
		Owner: "owner-a", CreatedAt: purgeTestNow.Add(-24 * time.Hour), NotAfter: &summaryPast,
		Tags: []string{purgeMilestoneSummaryTag},
	})

	manifest, err := s.PreviewPurge(ctx, opts)
	if err != nil {
		t.Fatalf("PreviewPurge after seeding a qualifying summary: %v", err)
	}
	if ids := manifest.IDs(); len(ids) != 1 || ids[0] != candidateID {
		t.Fatalf("IDs = %v, want [%s] (the summary record must never be a candidate)", ids, candidateID)
	}

	res, err := s.ApplyPurge(ctx, manifest, opts)
	if err != nil {
		t.Fatalf("ApplyPurge: %v", err)
	}
	if len(res.Deleted) != 1 || res.Deleted[0] != candidateID {
		t.Fatalf("Deleted = %v, want [%s]", res.Deleted, candidateID)
	}
	if _, gerr := s.Get(ctx, summaryID); gerr != nil {
		t.Errorf("milestone-summary record was deleted: %v", gerr)
	}
}

// TestPurgeExcludesDiscoveryAndRuleCategories proves the unconditional
// category exclusion (rule 7smp8vy9hr step 4) survives even when a record
// would otherwise match every selected class AND the free-form filter path.
func TestPurgeExcludesDiscoveryAndRuleCategories(t *testing.T) {
	s := newSpineTestStore(t, "spine_purge_excluded_categories")
	ctx := context.Background()
	const scope = "spine_purge_excluded_categories_scope"

	past := purgeTestNow.Add(-48 * time.Hour)
	discID := "62000000-0000-0000-0000-000000000001"
	ruleID := "62000000-0000-0000-0000-000000000002"
	seedSpineMemory(t, s, Memory{
		ID: discID, Content: "discovery", Scope: scope, Category: "discovery",
		Owner: "owner-a", CreatedAt: purgeTestNow.Add(-72 * time.Hour), NotAfter: &past,
	})
	seedSpineMemory(t, s, Memory{
		ID: ruleID, Content: "rule", Scope: scope, Category: "rule",
		Owner: "owner-a", CreatedAt: purgeTestNow.Add(-72 * time.Hour), NotAfter: &past,
	})

	opts := PurgeOptions{Classes: []PurgeClass{PurgeClassExpired}, Scope: scope, Now: purgeTestNow}
	manifest, err := s.PreviewPurge(ctx, opts)
	if err != nil {
		t.Fatalf("PreviewPurge: %v", err)
	}
	if ids := manifest.IDs(); len(ids) != 0 {
		t.Fatalf("IDs = %v, want empty -- discovery/rule records must never be eligible", ids)
	}

	filterOpts := PurgeOptions{Category: "discovery", Scope: scope, Now: purgeTestNow, OlderThan: time.Hour}
	manifest2, err := s.PreviewPurge(ctx, filterOpts)
	if err != nil {
		t.Fatalf("PreviewPurge (filter path): %v", err)
	}
	if ids := manifest2.IDs(); len(ids) != 0 {
		t.Fatalf("IDs (filter path, category=discovery) = %v, want empty", ids)
	}
}

// TestPurgeDerivationPaginatesEveryPage forces spineScrollBatch to 1 and
// seeds five eligible candidates -- derivePurgeEligible must still return
// all five, proving the derivation crosses every Qdrant page rather than
// evaluating the batch floor against a first page. A companion assertion
// seeds the NEWEST candidate on the last page and confirms the batch floor
// correctly fails against a summary older than it (the milestone-summary
// record here is deliberately OMITTED, so checkExtractGate's own failure
// proves the newest candidate really was seen).
func TestPurgeDerivationPaginatesEveryPage(t *testing.T) {
	saved := spineScrollBatch
	spineScrollBatch = 1
	t.Cleanup(func() { spineScrollBatch = saved })

	s := newSpineTestStore(t, "spine_purge_pagination_count")
	ctx := context.Background()
	const scope = "spine_purge_pagination_count_scope"

	past := purgeTestNow.Add(-48 * time.Hour)
	var newestID string
	for i := 1; i <= 5; i++ {
		id := fmt.Sprintf("64000000-0000-0000-0000-00000000000%d", i)
		created := purgeTestNow.Add(-time.Duration(72-i) * time.Hour) // ascending CreatedAt; last seeded is newest
		seedSpineMemory(t, s, Memory{
			ID: id, Content: "expired", Scope: scope, Category: "decision",
			Owner: "owner-a", CreatedAt: created, NotAfter: &past,
		})
		newestID = id
	}
	summaryID := "64000000-0000-0000-0000-000000000099"
	seedSpineMemory(t, s, Memory{
		ID: summaryID, Content: "summary", Scope: scope, Category: "decision",
		Owner: "owner-a", CreatedAt: purgeTestNow, Tags: []string{purgeMilestoneSummaryTag},
	})

	opts := PurgeOptions{Classes: []PurgeClass{PurgeClassExpired}, Scope: scope, Now: purgeTestNow}
	candidates, ms, err := s.derivePurgeEligible(ctx, opts)
	if err != nil {
		t.Fatalf("derivePurgeEligible: %v", err)
	}
	if len(candidates) != 5 {
		t.Fatalf("candidates = %d, want 5 (batch size 1 must still cross every page)", len(candidates))
	}
	if len(ms) != 1 || ms[0].ID != summaryID {
		t.Fatalf("milestoneSummaries = %v, want exactly [%s]", ms, summaryID)
	}
	if err := checkExtractGate(candidates, ms); err != nil {
		t.Fatalf("checkExtractGate: %v (newest candidate id %s)", err, newestID)
	}

	// Companion assertion: a milestone-summary record that predates the
	// NEWEST candidate (seeded on the LAST page, batch size 1) must still
	// fail the floor -- proving the newest candidate on the final page was
	// genuinely seen by the derivation rather than the floor accidentally
	// passing against a truncated (first-page-only) candidate set.
	olderSummary := milestoneSummaryRecord{ID: "stale-summary", Scope: scope, CreatedAt: purgeTestNow.Add(-time.Duration(72-3) * time.Hour)}
	if err := checkExtractGate(candidates, []milestoneSummaryRecord{olderSummary}); err == nil {
		t.Fatalf("checkExtractGate with a summary older than the newest (last-page) candidate = nil, want an error")
	}
}

// TestPurgeIntersectionSparesIneligibleReportsAppeared is D-11's core
// property: a record eligible at preview but archived (which the run's
// selected classes never included) before apply is spared; a record that
// becomes freshly eligible between preview and apply is reported appeared,
// never deleted, never merged into Deleted.
func TestPurgeIntersectionSparesIneligibleReportsAppeared(t *testing.T) {
	s := newSpineTestStore(t, "spine_purge_intersection")
	ctx := context.Background()
	const scope = "spine_purge_intersection_scope"

	past := purgeTestNow.Add(-48 * time.Hour)
	sparedID := "65000000-0000-0000-0000-000000000001"
	appearedID := "65000000-0000-0000-0000-000000000002"
	summaryID := "65000000-0000-0000-0000-000000000099"

	seedSpineMemory(t, s, Memory{
		ID: sparedID, Content: "will be restored before apply", Scope: scope, Category: "decision",
		Owner: "owner-a", CreatedAt: purgeTestNow.Add(-72 * time.Hour), NotAfter: &past,
	})
	seedSpineMemory(t, s, Memory{
		ID: summaryID, Content: "summary", Scope: scope, Category: "decision",
		Owner: "owner-a", CreatedAt: purgeTestNow, Tags: []string{purgeMilestoneSummaryTag},
	})

	opts := PurgeOptions{Classes: []PurgeClass{PurgeClassExpired}, Scope: scope, Now: purgeTestNow}
	manifest, err := s.PreviewPurge(ctx, opts)
	if err != nil {
		t.Fatalf("PreviewPurge: %v", err)
	}
	if ids := manifest.IDs(); len(ids) != 1 || ids[0] != sparedID {
		t.Fatalf("preview IDs = %v, want [%s]", ids, sparedID)
	}

	// Between preview and apply: sparedID becomes ineligible (its NotAfter
	// is cleared via a whole-payload Upsert simulating a correction), and a
	// brand-new record appears that independently qualifies.
	fresh, gerr := s.Get(ctx, sparedID)
	if gerr != nil {
		t.Fatalf("re-fetch %s: %v", sparedID, gerr)
	}
	fresh.NotAfter = nil
	if uerr := s.Upsert(ctx, fresh, []float32{0.1, 0.2, 0.3}); uerr != nil {
		t.Fatalf("clear NotAfter on %s: %v", sparedID, uerr)
	}
	seedSpineMemory(t, s, Memory{
		ID: appearedID, Content: "newly expired", Scope: scope, Category: "decision",
		Owner: "owner-a", CreatedAt: purgeTestNow.Add(-72 * time.Hour), NotAfter: &past,
	})

	res, err := s.ApplyPurge(ctx, manifest, opts)
	if err != nil {
		t.Fatalf("ApplyPurge: %v", err)
	}
	if len(res.Deleted) != 0 {
		t.Errorf("Deleted = %v, want empty (the only previewed id became ineligible)", res.Deleted)
	}
	if len(res.Spared) != 1 || res.Spared[0] != sparedID {
		t.Errorf("Spared = %v, want [%s]", res.Spared, sparedID)
	}
	if len(res.Appeared) != 1 || res.Appeared[0] != appearedID {
		t.Errorf("Appeared = %v, want [%s]", res.Appeared, appearedID)
	}
	if _, gerr := s.Get(ctx, sparedID); gerr != nil {
		t.Errorf("spared record was deleted: %v", gerr)
	}
	if _, gerr := s.Get(ctx, appearedID); gerr != nil {
		t.Errorf("appeared record was deleted: %v", gerr)
	}
}

// TestApplyPurgeReRunIsNoOp: re-running ApplyPurge with the same manifest
// after a successful apply deletes nothing further and returns success.
func TestApplyPurgeReRunIsNoOp(t *testing.T) {
	s := newSpineTestStore(t, "spine_purge_rerun")
	ctx := context.Background()
	const scope = "spine_purge_rerun_scope"

	past := purgeTestNow.Add(-48 * time.Hour)
	targetID := "66000000-0000-0000-0000-000000000001"
	summaryID := "66000000-0000-0000-0000-000000000099"
	seedSpineMemory(t, s, Memory{
		ID: targetID, Content: "expired", Scope: scope, Category: "decision",
		Owner: "owner-a", CreatedAt: purgeTestNow.Add(-72 * time.Hour), NotAfter: &past,
	})
	seedSpineMemory(t, s, Memory{
		ID: summaryID, Content: "summary", Scope: scope, Category: "decision",
		Owner: "owner-a", CreatedAt: purgeTestNow, Tags: []string{purgeMilestoneSummaryTag},
	})

	opts := PurgeOptions{Classes: []PurgeClass{PurgeClassExpired}, Scope: scope, Now: purgeTestNow}
	manifest, err := s.PreviewPurge(ctx, opts)
	if err != nil {
		t.Fatalf("PreviewPurge: %v", err)
	}
	if _, err := s.ApplyPurge(ctx, manifest, opts); err != nil {
		t.Fatalf("first ApplyPurge: %v", err)
	}
	res2, err := s.ApplyPurge(ctx, manifest, opts)
	if err != nil {
		t.Fatalf("second ApplyPurge (re-run): %v", err)
	}
	if len(res2.Deleted) != 0 {
		t.Errorf("re-run Deleted = %v, want empty", res2.Deleted)
	}
}

// TestPurgeBoundaries proves each of the three structural classes spares a
// record EXACTLY at its window boundary and includes one strictly past it,
// matching pruneCutoff's own strict-comparison semantics.
func TestPurgeBoundaries(t *testing.T) {
	const window = time.Hour

	t.Run("expired", func(t *testing.T) {
		s := newSpineTestStore(t, "spine_purge_boundary_expired")
		ctx := context.Background()
		const scope = "spine_purge_boundary_expired_scope"
		atCutoff := purgeTestNow.Add(-window)
		pastCutoff := atCutoff.Add(-time.Second)
		idAt := "67000000-0000-0000-0000-000000000001"
		idPast := "67000000-0000-0000-0000-000000000002"
		seedSpineMemory(t, s, Memory{ID: idAt, Content: "at", Scope: scope, Category: "decision", Owner: "o", CreatedAt: purgeTestNow.Add(-3 * time.Hour), NotAfter: &atCutoff})
		seedSpineMemory(t, s, Memory{ID: idPast, Content: "past", Scope: scope, Category: "decision", Owner: "o", CreatedAt: purgeTestNow.Add(-3 * time.Hour), NotAfter: &pastCutoff})

		cands, _, err := s.derivePurgeEligible(ctx, PurgeOptions{Classes: []PurgeClass{PurgeClassExpired}, Scope: scope, OlderThan: window, Now: purgeTestNow})
		if err != nil {
			t.Fatalf("derivePurgeEligible: %v", err)
		}
		assertPurgeCandidateIDs(t, cands, idPast)
	})

	t.Run("archived", func(t *testing.T) {
		s := newSpineTestStore(t, "spine_purge_boundary_archived")
		ctx := context.Background()
		const scope = "spine_purge_boundary_archived_scope"
		atCutoff := purgeTestNow.Add(-window)
		pastCutoff := atCutoff.Add(-time.Second)
		idAt := "68000000-0000-0000-0000-000000000001"
		idPast := "68000000-0000-0000-0000-000000000002"
		seedSpineMemory(t, s, Memory{ID: idAt, Content: "at", Scope: scope, Category: "decision", Owner: "o", CreatedAt: purgeTestNow.Add(-3 * time.Hour), ArchivedAt: &atCutoff})
		seedSpineMemory(t, s, Memory{ID: idPast, Content: "past", Scope: scope, Category: "decision", Owner: "o", CreatedAt: purgeTestNow.Add(-3 * time.Hour), ArchivedAt: &pastCutoff})

		cands, _, err := s.derivePurgeEligible(ctx, PurgeOptions{Classes: []PurgeClass{PurgeClassArchived}, Scope: scope, OlderThan: window, Now: purgeTestNow})
		if err != nil {
			t.Fatalf("derivePurgeEligible: %v", err)
		}
		assertPurgeCandidateIDs(t, cands, idPast)
	})

	t.Run("superseded", func(t *testing.T) {
		s := newSpineTestStore(t, "spine_purge_boundary_superseded")
		ctx := context.Background()
		const scope = "spine_purge_boundary_superseded_scope"
		atCutoff := purgeTestNow.Add(-window)
		pastCutoff := atCutoff.Add(-time.Second)

		succAtID := "69000000-0000-0000-0000-000000000011"
		succPastID := "69000000-0000-0000-0000-000000000012"
		idAt := "69000000-0000-0000-0000-000000000001"
		idPast := "69000000-0000-0000-0000-000000000002"
		seedSpineMemory(t, s, Memory{ID: succAtID, Content: "succ-at", Scope: scope, Category: "decision", Owner: "o", CreatedAt: atCutoff})
		seedSpineMemory(t, s, Memory{ID: succPastID, Content: "succ-past", Scope: scope, Category: "decision", Owner: "o", CreatedAt: pastCutoff})
		seedSpineMemory(t, s, Memory{ID: idAt, Content: "at", Scope: scope, Category: "decision", Owner: "o", CreatedAt: purgeTestNow.Add(-5 * time.Hour), SupersededBy: &succAtID})
		seedSpineMemory(t, s, Memory{ID: idPast, Content: "past", Scope: scope, Category: "decision", Owner: "o", CreatedAt: purgeTestNow.Add(-5 * time.Hour), SupersededBy: &succPastID})

		cands, _, err := s.derivePurgeEligible(ctx, PurgeOptions{Classes: []PurgeClass{PurgeClassSuperseded}, Scope: scope, OlderThan: window, Now: purgeTestNow})
		if err != nil {
			t.Fatalf("derivePurgeEligible: %v", err)
		}
		assertPurgeCandidateIDs(t, cands, idPast)
	})
}

// assertPurgeCandidateIDs asserts cands' ids equal exactly want (order
// independent), failing the test with the full candidate list otherwise.
func assertPurgeCandidateIDs(t *testing.T, cands []purgeCandidate, want ...string) {
	t.Helper()
	gotIDs := make([]string, len(cands))
	for i, c := range cands {
		gotIDs[i] = c.ID
	}
	sort.Strings(gotIDs)
	wantSorted := append([]string(nil), want...)
	sort.Strings(wantSorted)
	if !reflect.DeepEqual(gotIDs, wantSorted) {
		t.Fatalf("candidate ids = %v, want %v", gotIDs, wantSorted)
	}
}
