// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
)

// seedStatusRecordAtVersion upserts an ordinary record (which stamps
// schema_version == migrate.CurrentVersion via payload()) and then overrides
// the raw stored schema_version to v directly — the only way to construct a
// record at an arbitrary version, including one above migrate.CurrentVersion
// (D-06 forward compatibility) or explicitly at v0 (as opposed to the
// key-absent legacy shape seedLegacyRecord constructs).
func seedStatusRecordAtVersion(ctx context.Context, t *testing.T, s *Store, id string, v int) {
	t.Helper()
	mem := Memory{
		ID: id, Content: "status test " + id, Scope: "migrate-status-test:project:" + id,
		Owner: "sub-migrate-status-test", Category: "note", CreatedAt: time.Now().UTC(),
	}
	if err := s.Upsert(ctx, mem, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seedStatusRecordAtVersion: seed upsert(%s): %v", id, err)
	}
	injectRawPayload(ctx, t, s, id, qdrant.NewValueMap(map[string]any{schemaVersionKey: v}))
}

// TestMigrateStatusHistogram is Task 1 step 5: against real pinned Qdrant, a
// mixed-version collection (absent, v0, v1, v2, v42) is reported as a
// distribution — never a scalar — with the future-version population
// reported PER VERSION (M4), and the empty/all-migrated edge shapes hold.
func TestMigrateStatusHistogram(t *testing.T) {
	ctx := context.Background()
	c := dialTestClient(t)
	collection := testCollection("migrate_status_histogram")
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

	s := newTestStore(t, c, collection)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	t.Run("empty collection", func(t *testing.T) {
		res, err := s.MigrateStatus(ctx)
		if err != nil {
			t.Fatalf("MigrateStatus: %v", err)
		}
		if len(res.Buckets) != 0 || res.Absent != 0 || len(res.Future) != 0 || res.FutureTotal != 0 || res.Total != 0 {
			t.Errorf("empty collection: got %+v, want all-zero (len(Future) == 0, not nil-vs-empty-sensitive)", res)
		}
	})

	idFor := func(prefix string, i int) string {
		return fmt.Sprintf("%s-0000-0000-0000-%012d", prefix, i+1)
	}

	// Absent (legacy, no schema_version key at all): 2 records.
	for i := 0; i < 2; i++ {
		seedLegacyRecord(ctx, t, s, idFor("cfa10000", i))
	}
	// Explicit v0: 3 records.
	for i := 0; i < 3; i++ {
		seedStatusRecordAtVersion(ctx, t, s, idFor("cfa20000", i), 0)
	}
	// Explicit v1 (== CurrentVersion): 4 records.
	for i := 0; i < 4; i++ {
		seedStatusRecordAtVersion(ctx, t, s, idFor("cfa30000", i), 1)
	}
	// Future v2: 5 records. Future v42: 7 records — DIFFERENT counts so a
	// swap or a merge between the two future buckets is detectable.
	const n2, n42 = 5, 7
	for i := 0; i < n2; i++ {
		seedStatusRecordAtVersion(ctx, t, s, idFor("cfa40000", i), 2)
	}
	for i := 0; i < n42; i++ {
		seedStatusRecordAtVersion(ctx, t, s, idFor("cfa50000", i), 42)
	}

	t.Run("mixed-version collection", func(t *testing.T) {
		res, err := s.MigrateStatus(ctx)
		if err != nil {
			t.Fatalf("MigrateStatus: %v", err)
		}

		wantBuckets := []VersionBucket{{Version: 0, Count: 3}, {Version: 1, Count: 4}}
		if len(res.Buckets) != len(wantBuckets) {
			t.Fatalf("Buckets = %+v, want %+v", res.Buckets, wantBuckets)
		}
		for i, b := range wantBuckets {
			if res.Buckets[i] != b {
				t.Errorf("Buckets[%d] = %+v, want %+v", i, res.Buckets[i], b)
			}
		}

		if res.Absent != 2 {
			t.Errorf("Absent = %d, want 2", res.Absent)
		}

		// M4: Future MUST be a per-version bucket list, never a scalar. A
		// single-scalar FutureVersion/FutureCount implementation cannot
		// produce this two-entry slice with distinct per-version counts.
		wantFuture := []VersionBucket{{Version: 2, Count: n2}, {Version: 42, Count: n42}}
		if len(res.Future) != len(wantFuture) {
			t.Fatalf("Future = %+v, want %+v", res.Future, wantFuture)
		}
		for i, b := range wantFuture {
			if res.Future[i] != b {
				t.Errorf("Future[%d] = %+v, want %+v", i, res.Future[i], b)
			}
		}

		if res.FutureTotal != n2+n42 {
			t.Errorf("FutureTotal = %d, want %d", res.FutureTotal, n2+n42)
		}

		var bucketSum uint64
		for _, b := range res.Buckets {
			bucketSum += b.Count
		}
		sum := bucketSum + res.Absent + res.FutureTotal
		if sum != res.Total {
			t.Errorf("sum(Buckets)+Absent+FutureTotal = %d, want == Total (%d)", sum, res.Total)
		}
		if res.Total != 2+3+4+n2+n42 {
			t.Errorf("Total = %d, want %d", res.Total, 2+3+4+n2+n42)
		}
	})
}

// dialFacetInterceptingTestClient dials a client wired with interceptor —
// this file's own sibling of dialCapturingTestClient /
// dialFaultInjectingTestClient, kept local since no other file needs a
// generically-interceptable client.
func dialFacetInterceptingTestClient(t *testing.T, interceptor grpc.UnaryClientInterceptor) *qdrant.Client {
	t.Helper()
	if testQdrantAddr == "" {
		required, err := requireQdrant()
		if err != nil {
			t.Fatalf("%v", err)
		}
		if required {
			t.Fatal("no Qdrant available and ENGRAM_REQUIRE_QDRANT is set: failing instead of skipping")
		}
		t.Skip("no Qdrant available: set ENGRAM_QDRANT_TEST_ADDR or start Docker (testcontainers)")
	}
	host, portStr, err := net.SplitHostPort(testQdrantAddr)
	if err != nil {
		t.Fatalf("invalid Qdrant address %q: %v", testQdrantAddr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		t.Fatalf("invalid Qdrant port %q (from %q): %v", portStr, testQdrantAddr, err)
	}
	c, err := qdrant.NewClient(&qdrant.Config{
		Host: host, Port: port,
		GrpcOptions: []grpc.DialOption{grpc.WithUnaryInterceptor(interceptor)},
	})
	if err != nil {
		t.Fatalf("intercepting client: %v", err)
	}
	return c
}

// TestMigrateStatusFacetLimitIsNamedAndAdequate is a pure Go assertion (no
// Qdrant needed) that migrateStatusFacetLimit is the named package-level var
// this task's doc comment declares — a token grep satisfied by any nonzero
// Limit is not proof; TestMigrateStatusDetectsTruncation below is the
// behavioural proof that the FacetCounts literal actually PASSES this var
// (rather than leaving Limit nil or hardcoding a different bound).
func TestMigrateStatusFacetLimitIsNamedAndAdequate(t *testing.T) {
	if migrateStatusFacetLimit != 1024 {
		t.Fatalf("migrateStatusFacetLimit = %d, want 1024", migrateStatusFacetLimit)
	}
}

// TestMigrateStatusDetectsTruncation covers three distinguishable
// mismatch/limit interactions (acceptance criteria for Task 1):
//
//   - the bound is reached AND the sum does not reconcile: truncation.
//   - the sum does not reconcile and the bound is NOT reached: retried
//     exactly once, then reported as a concurrent write (C6-M12).
//   - exactly the bound's worth of distinct versions, WITH a reconciling
//     sum: a clean, full-width histogram, never mistaken for truncation
//     (C7-M1).
func TestMigrateStatusDetectsTruncation(t *testing.T) {
	ctx := context.Background()

	t.Run("bound reached and sum does not reconcile: truncation", func(t *testing.T) {
		c := dialTestClient(t)
		collection := testCollection("migrate_status_truncation")
		_ = c.DeleteCollection(ctx, collection)
		t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

		s := newTestStore(t, c, collection)
		if err := s.EnsureCollection(ctx, 3); err != nil {
			t.Fatalf("EnsureCollection: %v", err)
		}

		saved := migrateStatusFacetLimit
		migrateStatusFacetLimit = 2
		t.Cleanup(func() { migrateStatusFacetLimit = saved })

		// Seed strictly MORE distinct versions than the bound: v1, v2, v3.
		seedStatusRecordAtVersion(ctx, t, s, "cfb10000-0000-0000-0000-000000000001", 1)
		seedStatusRecordAtVersion(ctx, t, s, "cfb10000-0000-0000-0000-000000000002", 2)
		seedStatusRecordAtVersion(ctx, t, s, "cfb10000-0000-0000-0000-000000000003", 3)

		_, err := s.MigrateStatus(ctx)
		if err == nil {
			t.Fatal("MigrateStatus: got nil error, want a truncation error — three distinct versions exceed the bound of 2")
		}
		if !strings.Contains(err.Error(), "migrateStatusFacetLimit") || !strings.Contains(err.Error(), "INCOMPLETE") {
			t.Errorf("error %q does not name migrateStatusFacetLimit and INCOMPLETE", err.Error())
		}
	})

	t.Run("sum mismatch without reaching bound is retried once then reported as a concurrent write", func(t *testing.T) {
		collection := testCollection("migrate_status_race")

		var mu sync.Mutex
		facetCalls := 0
		var writeFn func()

		interceptor := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			_, isFacet := req.(*qdrant.FacetCounts)
			err := invoker(ctx, method, req, reply, cc, opts...)
			if isFacet && err == nil {
				mu.Lock()
				facetCalls++
				fn := writeFn
				mu.Unlock()
				if fn != nil {
					fn()
				}
			}
			return err
		}

		fc := dialFacetInterceptingTestClient(t, interceptor)
		_ = fc.DeleteCollection(ctx, collection)
		t.Cleanup(func() { _ = fc.DeleteCollection(context.Background(), collection) })
		s := newTestStore(t, fc, collection)
		if err := s.EnsureCollection(ctx, 3); err != nil {
			t.Fatalf("EnsureCollection: %v", err)
		}

		// A plain, non-intercepted client/store pointed at the SAME
		// collection performs the racing write — using the intercepted
		// client here would recurse into this same interceptor.
		plain := dialTestClient(t)
		sPlain := newTestStore(t, plain, collection)

		seedStatusRecordAtVersion(ctx, t, s, "cfb20000-0000-0000-0000-000000000001", 1)

		// Every time Facet is observed, insert ONE MORE v1 record via the
		// uncontaminated client before the interceptor returns — so the
		// derived sum (built from the just-returned, now-stale Facet
		// snapshot) always trails the whole-collection Total that the
		// FOLLOWING Count call observes. This reproduces indefinitely,
		// across both the first attempt and its one retry, without relying
		// on real goroutine concurrency.
		var racingIDs []string
		next := 2
		writeFn = func() {
			id := fmt.Sprintf("cfb20000-0000-0000-0000-%012d", next)
			next++
			racingIDs = append(racingIDs, id)
			seedStatusRecordAtVersion(ctx, t, sPlain, id, 1)
		}

		_, err := s.MigrateStatus(ctx)
		if err == nil {
			t.Fatal("MigrateStatus: got nil error, want a non-reconciling-sum error — the racing write must keep the derived sum stale on every attempt")
		}
		if strings.Contains(err.Error(), "INCOMPLETE") {
			t.Errorf("error %q claims truncation — this scenario never reaches migrateStatusFacetLimit and must not be classified as truncation", err.Error())
		}
		if !strings.Contains(err.Error(), "did not reconcile") || !strings.Contains(err.Error(), "concurrent") {
			t.Errorf("error %q does not name a non-reconciling sum and a concurrent writer", err.Error())
		}

		mu.Lock()
		gotCalls := facetCalls
		mu.Unlock()
		if gotCalls != 2 {
			t.Errorf("Facet was called %d times, want exactly 2 (the original attempt plus exactly one retry)", gotCalls)
		}
	})

	t.Run("exactly at the bound with a reconciling sum is a clean full-width histogram", func(t *testing.T) {
		c := dialTestClient(t)
		collection := testCollection("migrate_status_full_width")
		_ = c.DeleteCollection(ctx, collection)
		t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

		s := newTestStore(t, c, collection)
		if err := s.EnsureCollection(ctx, 3); err != nil {
			t.Fatalf("EnsureCollection: %v", err)
		}

		saved := migrateStatusFacetLimit
		migrateStatusFacetLimit = 3
		t.Cleanup(func() { migrateStatusFacetLimit = saved })

		// Seed EXACTLY migrateStatusFacetLimit distinct versions (v1, v2,
		// v3), with nothing racing the read — a fully reconciling sum.
		seedStatusRecordAtVersion(ctx, t, s, "cfb30000-0000-0000-0000-000000000001", 1)
		seedStatusRecordAtVersion(ctx, t, s, "cfb30000-0000-0000-0000-000000000002", 2)
		seedStatusRecordAtVersion(ctx, t, s, "cfb30000-0000-0000-0000-000000000003", 3)

		res, err := s.MigrateStatus(ctx)
		if err != nil {
			t.Fatalf("MigrateStatus: got error %v, want a clean full-width histogram — this fixture's hit count exactly EQUALS the bound but the sum reconciles, which is not truncation (C7-M1)", err)
		}
		if got := len(res.Buckets) + len(res.Future); got != int(migrateStatusFacetLimit) {
			t.Errorf("len(Buckets)+len(Future) = %d, want %d (== migrateStatusFacetLimit)", got, migrateStatusFacetLimit)
		}
	})
}
