// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"fmt"
	"net"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/migrate"
	"google.golang.org/grpc"
)

// TestMigrateConvergesWithoutLock proves ROADMAP success criterion 5: the
// sweep runs with NO collection lock, because the write path stamps a
// record's current version before the sweep ever sees it. "No lock needed"
// is an ordering dependency on Phase 2's write path (payload()'s monotonic
// stamp), not a property of the sweep itself — a test that merely showed
// the sweep finishing would prove nothing about it. This test demonstrates
// the property by writing new records WHILE a sweep is in flight, from a
// deterministic trigger (the sweep's own Scroll request), and observing at
// the wire which point ids the sweep actually chose to write.
//
// PA-10's substitution, stated explicitly: migrate.CurrentVersion is
// pinned at 0 this phase (registry.go's // PHASE4: marker), so an ordinary
// mid-sweep write cannot reach a target of 1 through the constant alone.
// Instead, the mid-sweep "already-current" record is written with an
// explicit Memory.SchemaVersion of migrate.Version(1); payload()'s
// monotonic stamp (store.go:646, int(max(migrate.CurrentVersion,
// m.SchemaVersion))) then stamps 1, matching the sweep's target. What that
// substitutes is the VALUE the write path claims — supplied by the caller
// here instead of by the constant; what it does NOT substitute is the
// PATH: payload() is still the only door a write goes through, the stamp
// is still monotonic, and the write is still atomic-per-record, all three
// already proven by Phase 2 (TestEveryPointWriteRoutesThroughPayload,
// TestEveryFullWriteMethodStampsSchemaVersion). The mid-sweep write goes
// through Store.Upsert — never a raw-payload injection helper — precisely
// so this substitution is exercised rather than bypassed.
//
// PA-10a — what this test proves, and what it does not (both review-cycle-1
// reviewers flagged the earlier framing as an overclaim):
//
//  1. What IS proven here: backlogFilter's strict range bound excludes a
//     record already at the target version; Store.Upsert -> payload() is
//     the production mechanism that puts a record at that version; and the
//     sweep converges under a live concurrent writer with no lock.
//  2. The named condition: this test proves SC5 conditional on payload()'s
//     stamp equalling the sweep's target, an invariant pinned by
//     TestEveryFullWriteMethodStampsSchemaVersion
//     (schemaversion_stamp_test.go:27, together with
//     TestEveryPointWriteRoutesThroughPayload); if that gate is ever
//     weakened, SC5's proof here must be revisited. In production this
//     equality holds because both sides read the SAME CONSTANT
//     (migrate.CurrentVersion); here it holds BY CONSTRUCTION, because the
//     test supplies both the stamp's input (Memory.SchemaVersion) and the
//     sweep's target (MigrateOptions.Target) itself.
//  3. PHASE4: the literal, causal half of SC5 — that new writes arrive
//     already-current BECAUSE the write path stamps the current version —
//     is deferred to Phase 4. When Phase 4 pairs CurrentVersion = 1 with
//     the registered v0->v1 step, this same concurrency test must be
//     re-run with an ORDINARY Memory carrying NO SchemaVersion at all, and
//     MigrateOptions.Target left at zero so it resolves to CurrentVersion.
//     That re-run is the only direct proof of SC5's causal claim, and it
//     is BLOCKING for Phase 4, not optional polish.
//
// The race is deterministic, not a sleep: the trigger fires from inside a
// grpc.UnaryClientInterceptor on the sweep's own *qdrant.ScrollPoints
// request (the sweep's SECOND observed scroll), guarded by a sync.Once and
// counted by an integer fires counter incremented inside that Once body —
// see midSweepHook and PA-11 below. This mirrors this file's existing
// convention (TestSupersedeConcurrent, TestSupersedeVsUpdateConcurrent) of
// documenting in prose, before any code, why a race is deterministic
// enough to assert on.
func TestMigrateConvergesWithoutLock(t *testing.T) {
	ctx := context.Background()

	h := &midSweepHook{fireOnScroll: 2}

	sweepClient := dialMidSweepTestClient(t, h)
	collection := testCollection("migrate_converge")
	_ = sweepClient.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = sweepClient.DeleteCollection(context.Background(), collection) })

	sweepStore := newTestStore(t, sweepClient, collection)
	if err := sweepStore.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	// The mid-sweep writer is a SECOND *Store built on a plain,
	// non-intercepting client over the SAME collection name (PA-12): it
	// does not re-enter midSweepInterceptor and does not contend with the
	// sweeping store's own connection, so there is no deadlock hazard
	// between the interceptor's synchronous callback and the write it
	// issues.
	writerClient := dialTestClient(t)
	writerStore := newTestStore(t, writerClient, collection)

	const scope = "migrate-converge-test:project:mid-sweep"
	seededIDs := make([]string, 6)
	for i := range seededIDs {
		id := fmt.Sprintf("c5100000-0000-0000-0000-%012d", i+1)
		seedLegacyRecord(ctx, t, sweepStore, id)
		seededIDs[i] = id
	}

	const alreadyCurrentID = "c5100000-0000-0000-0000-900000000001"
	const laggardID = "c5100000-0000-0000-0000-900000000002"

	// h.fn runs inside midSweepInterceptor's callback, on the sweep's own
	// (i.e. the test's) goroutine, guarded by sync.Once — but PER PA-11a IT
	// MUST NOT CALL t.Fatal/t.Fatalf/t.Error/t.Errorf: the guarantee that
	// this callback runs on the test's own goroutine is a property of the
	// current grpc-go/go-client call path, not something this test may
	// depend on, and a fatal call from an uncertain goroutine silently
	// fails to fail the test. Failures are instead recorded into h's
	// mutex-guarded error slice via h.recordErr and drained by the test
	// immediately after Store.Migrate returns. t.Logf is not used here
	// either, to keep this closure free of any *testing.T dependency.
	h.fn = func() {
		alreadyCurrent := Memory{
			ID: alreadyCurrentID, Content: "already current at mid-sweep", Scope: scope,
			Owner: "sub-migrate-test", Category: "note", CreatedAt: time.Now().UTC(),
			SchemaVersion: migrate.Version(1),
		}
		if err := writerStore.Upsert(ctx, alreadyCurrent, []float32{0.7, 0.8, 0.9}); err != nil {
			h.recordErr("mid-sweep write: alreadyCurrent upsert(%s): %v", alreadyCurrentID, err)
			return
		}

		// laggard is the BOUNDED-ADVERSARIAL CONTROL (PA-13): a single
		// below-target record inserted at the same mid-sweep instant.
		// Without it, "the already-current record was never re-processed"
		// would also be satisfied by a backlog filter that matches
		// nothing at all — this control is what distinguishes strict
		// exclusion from vacuity. It is deliberately bounded to exactly
		// one insertion, hand-checked against Store.Migrate's
		// non-shrinking-backlog termination guard: six seeded records,
		// Batch: 2, one insertion on the second scroll — each pass still
		// removes two and adds at most one once, so the freshly-counted
		// backlog strictly decreases every pass. This is NOT evidence
		// that the sweep converges against an arbitrary concurrent
		// writer; it proves only that the filter's exclusion is real.
		laggard := Memory{
			ID: laggardID, Content: "laggard at mid-sweep", Scope: scope,
			Owner: "sub-migrate-test", Category: "note", CreatedAt: time.Now().UTC(),
		}
		if err := writerStore.Upsert(ctx, laggard, []float32{0.4, 0.5, 0.6}); err != nil {
			h.recordErr("mid-sweep write: laggard upsert(%s): %v", laggardID, err)
			return
		}

		// Check the stamped values AT THE CAUSE (here, at write time),
		// not later at a confusing downstream symptom: if PA-10's
		// monotonic-max substitution ever stops behaving as documented,
		// this is where that would first be attributable.
		acPayload, err := rawPayloadNoFatal(ctx, writerStore, alreadyCurrentID)
		if err != nil {
			h.recordErr("mid-sweep write: read back alreadyCurrent(%s): %v", alreadyCurrentID, err)
		} else if got := acPayload[schemaVersionKey].GetIntegerValue(); got != 1 {
			h.recordErr("alreadyCurrent(%s) stored schema_version = %d, want 1 (PA-10's monotonic-max stamp did not behave as documented)", alreadyCurrentID, got)
		}
		lgPayload, err := rawPayloadNoFatal(ctx, writerStore, laggardID)
		if err != nil {
			h.recordErr("mid-sweep write: read back laggard(%s): %v", laggardID, err)
		} else if got := lgPayload[schemaVersionKey].GetIntegerValue(); got != 0 {
			h.recordErr("laggard(%s) stored schema_version = %d, want 0 (below the sweep's target of 1)", laggardID, got)
		}
	}

	opts := MigrateOptions{
		Target: 1,
		Steps:  []migrate.Step{markerStep(0, 1, "converge_marker")},
		Batch:  2,
	}

	// No collection lock, mutex or other coordination is added around this
	// call — the interceptor trigger above is the ONLY synchronization,
	// which is the entire point of what this test demonstrates.
	res, migrateErr := sweepStore.Migrate(ctx, opts)

	if hookErrs := h.drainErrs(); len(hookErrs) > 0 {
		t.Fatalf("midSweepHook recorded %d error(s) before any subtest ran:\n%s", len(hookErrs), strings.Join(hookErrs, "\n"))
	}
	if migrateErr != nil {
		t.Fatalf("Store.Migrate: %v", migrateErr)
	}

	fires, triggerMatches, scrolls, writeIDs := h.snapshot()

	t.Run("already-current mid-sweep write is never re-processed", func(t *testing.T) {
		// Wire-level claim: the sweep's own SetPayload calls never
		// selected alreadyCurrent's id.
		if slices.Contains(writeIDs, alreadyCurrentID) {
			t.Errorf("alreadyCurrent id %s appears in the sweep's recorded SetPayload write-id set, want absent", alreadyCurrentID)
		}
		// Collection-level claim: the record was never touched. Both
		// assertions matter — the first alone could pass if the sweep
		// wrote through some other verb, the second alone could pass if
		// the step happened to be a no-op.
		raw := rawPayload(ctx, t, sweepStore, alreadyCurrentID)
		if _, ok := raw["converge_marker"]; ok {
			t.Errorf("alreadyCurrent(%s) carries converge_marker — it was re-processed", alreadyCurrentID)
		}
		if got := raw[schemaVersionKey].GetIntegerValue(); got != 1 {
			t.Errorf("alreadyCurrent(%s) schema_version = %d, want 1 (unchanged)", alreadyCurrentID, got)
		}
	})

	t.Run("below-target mid-sweep write does enter the backlog and is migrated", func(t *testing.T) {
		// This control is what distinguishes "already-current records are
		// excluded" from "the filter matches nothing" (PA-13); it is a
		// BOUNDED-ADVERSARIAL control over exactly one insertion, not
		// evidence for a finite-new-work theorem under arbitrary writers.
		if !slices.Contains(writeIDs, laggardID) {
			t.Errorf("laggard id %s absent from the sweep's recorded SetPayload write-id set, want present", laggardID)
		}
		raw := rawPayload(ctx, t, sweepStore, laggardID)
		if _, ok := raw["converge_marker"]; !ok {
			t.Errorf("laggard(%s) missing converge_marker — it was not migrated", laggardID)
		}
		if got := raw[schemaVersionKey].GetIntegerValue(); got != 1 {
			t.Errorf("laggard(%s) schema_version = %d, want 1", laggardID, got)
		}
	})

	t.Run("the sweep converged, and this run actually observed what it claims", func(t *testing.T) {
		if res.Backlog != 0 {
			t.Errorf("res.Backlog = %d, want 0 (converged)", res.Backlog)
		}
		if backlog := migrateBacklogIDs(ctx, t, sweepStore, 1); len(backlog) != 0 {
			t.Errorf("migrateBacklogIDs(target=1) = %v, want empty", backlog)
		}
		for _, id := range seededIDs {
			raw := rawPayload(ctx, t, sweepStore, id)
			if _, ok := raw["converge_marker"]; !ok {
				t.Errorf("seeded record %s missing converge_marker", id)
			}
			if got := raw[schemaVersionKey].GetIntegerValue(); got != 1 {
				t.Errorf("seeded record %s schema_version = %d, want 1", id, got)
			}
		}

		// The negative claims above are meaningless if the mid-sweep
		// writer never actually ran (durable record x6v6qxqd6f: this
		// repo has shipped a gate that observed nothing before). fires is
		// an INTEGER execution counter incremented inside the sync.Once
		// body — not a boolean — because sync.Once alone ENFORCES
		// at-most-once and a boolean alone only proves at-least-once; the
		// pair can otherwise establish "exactly one" only by trusting
		// sync.Once rather than observing it (PA-11).
		if fires != 1 {
			t.Errorf("h.fires = %d, want 1 (integer execution counter incremented inside sync.Once, not a boolean)", fires)
		}
		if triggerMatches < 1 {
			t.Errorf("h.triggerMatches = %d, want >= 1 (the scroll-ordinal trigger never armed, so every negative assertion above is vacuous)", triggerMatches)
		}
		if triggerMatches > 1 {
			// Not a failure: this is sync.Once doing its job — the
			// ordinal condition kept evaluating true on later scrolls,
			// but only the first call ran h.fn.
			t.Logf("triggerMatches = %d > 1 while fires = %d: sync.Once suppressed re-firing on later scrolls as designed", triggerMatches, fires)
		}
		if scrolls <= 1 {
			t.Errorf("h.scrolls = %d, want > 1 (a mid-sweep moment requires more than a single scroll pass to have existed)", scrolls)
		}
		if len(writeIDs) == 0 {
			t.Errorf("recorded write-id set is empty")
		}

		expected := append(append([]string{}, seededIDs...), laggardID)
		sort.Strings(expected)
		gotSorted := append([]string{}, writeIDs...)
		sort.Strings(gotSorted)
		if !slices.Equal(expected, gotSorted) {
			t.Errorf("recorded write-id set mismatch:\n  missing (expected, not observed): %v\n  extra (observed, not expected): %v",
				diffSorted(expected, gotSorted), diffSorted(gotSorted, expected))
		}
	})
}

// midSweepHook is the shared state driving TestMigrateConvergesWithoutLock's
// deterministic mid-sweep write and its wire-level SetPayload observation.
// All fields are guarded by mu except once and fn, which are set up once
// before dialing and never mutated concurrently thereafter.
type midSweepHook struct {
	mu sync.Mutex

	// fireOnScroll is the 1-indexed ordinal of the sweep's own
	// *qdrant.ScrollPoints request the trigger arms on: once scrolls
	// reaches this value, every subsequent scroll re-evaluates the
	// condition true (incrementing triggerMatches), but fn runs at most
	// once, enforced by once.
	fireOnScroll int
	once         sync.Once
	fn           func()

	scrolls        int
	fires          int // incremented INSIDE once.Do's body (PA-11) — the observed count, not an inference.
	triggerMatches int
	errs           []string
	writeIDs       []string
}

// onScroll is called once per observed *qdrant.ScrollPoints request, from
// midSweepInterceptor, on the sweep's own goroutine.
func (h *midSweepHook) onScroll() {
	h.mu.Lock()
	h.scrolls++
	armed := h.scrolls >= h.fireOnScroll
	h.mu.Unlock()

	if !armed {
		return
	}
	h.mu.Lock()
	h.triggerMatches++
	h.mu.Unlock()

	h.once.Do(func() {
		h.mu.Lock()
		h.fires++
		h.mu.Unlock()
		h.fn()
	})
}

// onSetPayload records the point ids selected by one *qdrant.SetPayloadPoints
// request — the wire-level observation the whole test rests on.
func (h *midSweepHook) onSetPayload(req *qdrant.SetPayloadPoints) {
	ids := req.GetPointsSelector().GetPoints().GetIds()
	if len(ids) == 0 {
		return
	}
	h.mu.Lock()
	for _, id := range ids {
		h.writeIDs = append(h.writeIDs, id.GetUuid())
	}
	h.mu.Unlock()
}

// recordErr appends a described failure detected inside the hook's
// callback. Never call any t.Fatal-family or t.Error-family method from
// inside that callback (PA-11a) — record here instead, and the test drains
// this slice immediately after Store.Migrate returns.
func (h *midSweepHook) recordErr(format string, args ...any) {
	h.mu.Lock()
	h.errs = append(h.errs, fmt.Sprintf(format, args...))
	h.mu.Unlock()
}

// drainErrs returns and clears every error recorded so far.
func (h *midSweepHook) drainErrs() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	errs := h.errs
	h.errs = nil
	return errs
}

// snapshot returns a stable, race-free read of the hook's observation
// counters and recorded write-id set. Called only after Store.Migrate has
// returned, when no further interceptor callback can run concurrently, but
// still takes mu for clarity and to keep -race clean regardless.
func (h *midSweepHook) snapshot() (fires, triggerMatches, scrolls int, writeIDs []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.fires, h.triggerMatches, h.scrolls, append([]string{}, h.writeIDs...)
}

// midSweepInterceptor returns a grpc.UnaryClientInterceptor that type-switches
// the outgoing request: a *qdrant.ScrollPoints drives h.onScroll BEFORE
// invoker runs, so a mid-sweep write triggered by it is genuinely committed
// before the sweep reads that page; a *qdrant.SetPayloadPoints is recorded
// via h.onSetPayload and then passed through unchanged. Everything else
// passes through untouched.
func midSweepInterceptor(h *midSweepHook) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		switch r := req.(type) {
		case *qdrant.ScrollPoints:
			h.onScroll()
		case *qdrant.SetPayloadPoints:
			h.onSetPayload(r)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// dialMidSweepTestClient is dialCapturingTestClient's sibling
// (schemaversion_recallgate_test.go:912): identical dial/skip/parse
// boilerplate, with midSweepInterceptor wired in as the sole interceptor.
func dialMidSweepTestClient(t *testing.T, h *midSweepHook) *qdrant.Client {
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
		GrpcOptions: []grpc.DialOption{grpc.WithUnaryInterceptor(midSweepInterceptor(h))},
	})
	if err != nil {
		t.Fatalf("mid-sweep client: %v", err)
	}
	return c
}

// rawPayloadNoFatal is rawPayload's (schemaversion_compat_test.go:528)
// non-fatal sibling: it returns an error instead of calling t.Fatalf, so it
// is safe to call from inside midSweepHook's callback (PA-11a forbids any
// t.Fatal-family call from there).
func rawPayloadNoFatal(ctx context.Context, s *Store, id string) (map[string]*qdrant.Value, error) {
	pts, err := s.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: s.collection, Ids: []*qdrant.PointId{qdrant.NewID(id)},
		WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, err
	}
	if len(pts) == 0 {
		return nil, fmt.Errorf("point %s not found", id)
	}
	return pts[0].Payload, nil
}

// diffSorted returns the elements of a not present in b, via a set lookup —
// used to print both difference directions on a write-id-set mismatch.
func diffSorted(a, b []string) []string {
	inB := make(map[string]struct{}, len(b))
	for _, x := range b {
		inB[x] = struct{}{}
	}
	var out []string
	for _, x := range a {
		if _, ok := inB[x]; !ok {
			out = append(out, x)
		}
	}
	return out
}
