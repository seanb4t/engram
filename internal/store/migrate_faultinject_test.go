// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves ROADMAP success criterion 4: Store.Migrate's sweep
// survives a forced mid-sequence partial SetPayload failure against a real
// pinned Qdrant, and a subsequent resume converges the backlog to zero —
// reconciling by re-derivation, never by trusting the write call's own
// success/failure signal (D-09).
//
// The pinned server (qdrant/qdrant:v1.18.2) chunks a multi-ID payload write
// internally and mutates every point it finds before raising an error for
// one it does not (qdrant/qdrant#9371), already documented twice in this
// package: store.go:127-140 (qdrantPayloadOpBatchSize, naming the server's
// own chunk size) and store.go:2213-2223 (Store.Supersede's own doc comment
// on the identical hazard). This file's comments cite those rather than
// restating the upstream analysis.
//
// What this proves about #9371, precisely (PA-2, review cycle 1: the
// earlier framing overclaimed). Store.Migrate writes ONE point per
// SetPayload — never a multi-ID batch — which removes the WITHIN-CALL
// multi-ID partiality class BY CONSTRUCTION: there is no second id in the
// call for a later internal chunk to miss. That is a design property, not
// a test result, and TestMigratePartialFailureResume does not reproduce
// it. What remains, and what its three scenarios actually exercise, are
// the two classes per-point writing does NOT remove: CROSS-CALL partial
// progress (some points of a pass land, some do not, and the next pass
// must re-derive rather than replay — scenarios 1 and 2) and UNRELIABLE
// ERROR SIGNALS (a call that committed and still returned an error,
// #9371's actual operational hazard — scenario 3).
package store

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/migrate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// faultMode selects how an armed write is failed by
// setPayloadFaultInterceptor.
type faultMode int

const (
	// faultNone leaves an armed write unaffected. It is the injector's
	// zero value, so a freshly constructed, never-armed
	// *setPayloadFaultInjector is a pure recorder by construction (PA-16)
	// — arm's callers always pair a non-zero failFrom with a real mode.
	faultNone faultMode = iota
	// faultBeforeInvoke returns an error WITHOUT calling invoker: the
	// write never reaches the server. This is the simpler of the two
	// injection modes (PA-7) — it proves the sweep retries, not that it
	// disbelieves the signal.
	faultBeforeInvoke
	// faultAfterInvoke calls invoker first; if the real call succeeds, it
	// substitutes an error on the way back. THIS is the mode that
	// reproduces qdrant/qdrant#9371's actual hazard (PA-7). It is a
	// SEMANTIC simulation, not a literal server-generated wire failure:
	// the RPC genuinely succeeded and committed at the server —
	// invoker(...) returned nil — and this interceptor substitutes the
	// error afterward, so what Store.Migrate observes is an ordinary gRPC
	// status error carrying no information about what landed. That is
	// exactly D-09's epistemic position: an error value that does not
	// describe the collection. No comment in this file may describe this
	// mode as indistinguishable from a real failure at the wire.
	faultAfterInvoke
)

// setPayloadFaultInjector is the fault-injecting/observing counterpart to
// schemaversion_recallgate_test.go's recallCapture: wired into a client's
// dial options via setPayloadFaultInterceptor, it records every outgoing
// *qdrant.SetPayloadPoints request's selected point id and, when armed,
// fails a chosen contiguous ordinal range of them in one of faultMode's two
// modes.
//
// This interceptor is the deliberately different choice D-10 makes instead
// of extending the test-only setPayloadKeys hook field already on Store
// (used by Store.Supersede's tests) — PA-8: that hook lives in production
// code and partly tests the hook rather than the sweep, whereas this
// interceptor lives entirely in the client's dial options, so the
// production code path Store.Migrate runs is byte-identical whether or not
// a test wires this in.
//
// An injector left DISARMED (failFrom == 0, the zero value) is a pure
// RECORDER: seen/injected/recordedIDs are still populated on every
// observed write, but no call is ever failed. Scenario 2's resume observer
// (PA-16) depends on exactly this: a fresh, never-armed
// *setPayloadFaultInjector handed to a SEPARATE client purely to watch
// what that client's own writes do, holding no cursor and no failed-id
// set — nothing Store.Migrate itself reads.
type setPayloadFaultInjector struct {
	mu sync.Mutex

	failFrom  int // 1-based ordinal of the first write to fail; 0 = disarmed
	failCount int // consecutive writes to fail from failFrom; 0 = unbounded
	mode      faultMode

	seenCount     int
	injectedCount int
	recordedIDs   []string // one entry per observed write, in call order
}

// arm configures inj to fail writes [from, from+count) — 1-based, ordinal
// across ALL SetPayload writes this injector has observed — in mode. count
// == 0 means unbounded: every write from ordinal from onward fails.
func (inj *setPayloadFaultInjector) arm(from, count int, mode faultMode) {
	inj.mu.Lock()
	defer inj.mu.Unlock()
	inj.failFrom = from
	inj.failCount = count
	inj.mode = mode
}

// disarm turns inj back into a pure recorder: no future write is failed.
// Counters and recorded ids already accumulated are untouched.
func (inj *setPayloadFaultInjector) disarm() {
	inj.mu.Lock()
	defer inj.mu.Unlock()
	inj.failFrom = 0
	inj.failCount = 0
	inj.mode = faultNone
}

// seen returns the number of *qdrant.SetPayloadPoints requests inj has
// observed so far.
func (inj *setPayloadFaultInjector) seen() int {
	inj.mu.Lock()
	defer inj.mu.Unlock()
	return inj.seenCount
}

// injected returns the number of writes inj has actually failed so far.
func (inj *setPayloadFaultInjector) injected() int {
	inj.mu.Lock()
	defer inj.mu.Unlock()
	return inj.injectedCount
}

// ids returns a copy of the point ids inj has recorded, in observed call
// order — never mutated by a caller through the returned slice.
func (inj *setPayloadFaultInjector) ids() []string {
	inj.mu.Lock()
	defer inj.mu.Unlock()
	return append([]string(nil), inj.recordedIDs...)
}

// selectedPointID returns the single point id sp's PointsSelector carries.
// Store.Migrate issues exactly one SetPayload per point (PA-2), so this is
// always the write's one selected id; "" is returned only if sp somehow
// carries none, which production code never does.
func selectedPointID(sp *qdrant.SetPayloadPoints) string {
	ids := sp.GetPointsSelector().GetPoints().GetIds()
	if len(ids) == 0 {
		return ""
	}
	return ids[0].GetUuid()
}

// setPayloadFaultInterceptor returns a grpc.UnaryClientInterceptor wired to
// inj. It type-switches req: anything that is not a
// *qdrant.SetPayloadPoints passes straight through to invoker unchanged —
// the same default-branch discipline recallCaptureInterceptor
// (schemaversion_recallgate_test.go) uses for its own recognized set. For a
// SetPayloadPoints request, it records the request's selected point id,
// increments seen, and decides whether this call's ordinal falls inside
// inj's currently armed range.
func setPayloadFaultInterceptor(inj *setPayloadFaultInjector) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		sp, ok := req.(*qdrant.SetPayloadPoints)
		if !ok {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		id := selectedPointID(sp)

		inj.mu.Lock()
		inj.seenCount++
		ordinal := inj.seenCount
		inj.recordedIDs = append(inj.recordedIDs, id)
		armed := inj.failFrom > 0 && ordinal >= inj.failFrom &&
			(inj.failCount == 0 || ordinal < inj.failFrom+inj.failCount)
		mode := inj.mode
		if armed {
			inj.injectedCount++
		}
		inj.mu.Unlock()

		if !armed {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		switch mode {
		case faultBeforeInvoke:
			// The write never reaches the server.
			return status.Error(codes.Unavailable,
				"setPayloadFaultInjector: injected failure BEFORE invoke — this write never reached the server")
		case faultAfterInvoke:
			if err := invoker(ctx, method, req, reply, cc, opts...); err != nil {
				return err
			}
			// SEMANTIC SIMULATION of qdrant/qdrant#9371 (PA-7): the RPC
			// succeeded and committed at the server, and this interceptor
			// substitutes an error on the way back. See faultAfterInvoke's
			// doc comment for the precise characterization this message
			// must not contradict.
			return status.Error(codes.Unavailable,
				"setPayloadFaultInjector: injected failure AFTER invoke — this write reached the server and committed")
		default:
			return invoker(ctx, method, req, reply, cc, opts...)
		}
	}
}

// dialFaultInjectingTestClient is dialTestClient's fault-injecting sibling:
// it wires a grpc.WithUnaryInterceptor into the client's dial options so
// setPayloadFaultInterceptor observes (and, when armed, fails) every
// outgoing *qdrant.SetPayloadPoints request. Skips exactly like
// dialTestClient/dialCapturingTestClient when no Qdrant is available, and
// fails closed under ENGRAM_REQUIRE_QDRANT rather than skipping.
func dialFaultInjectingTestClient(t *testing.T, inj *setPayloadFaultInjector) *qdrant.Client {
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
		GrpcOptions: []grpc.DialOption{grpc.WithUnaryInterceptor(setPayloadFaultInterceptor(inj))},
	})
	if err != nil {
		t.Fatalf("fault-injecting client: %v", err)
	}
	return c
}

// assertSortedIDSetEqual asserts got and want denote the same set of ids,
// independent of order, printing both difference directions on failure —
// the same diff-printing discipline TestBacklogFilterMatchesAbsentAndBelowTarget
// (migrate_test.go) uses inline. Never a bare length check or a contains
// check: a length check passes for the wrong two records, and a contains
// check passes for a superset.
func assertSortedIDSetEqual(t *testing.T, label string, got, want []string) {
	t.Helper()
	gotSorted := append([]string(nil), got...)
	wantSorted := append([]string(nil), want...)
	sort.Strings(gotSorted)
	sort.Strings(wantSorted)
	if len(gotSorted) == len(wantSorted) {
		equal := true
		for i := range gotSorted {
			if gotSorted[i] != wantSorted[i] {
				equal = false
				break
			}
		}
		if equal {
			return
		}
	}
	gotSet := make(map[string]bool, len(gotSorted))
	for _, id := range gotSorted {
		gotSet[id] = true
	}
	wantSet := make(map[string]bool, len(wantSorted))
	for _, id := range wantSorted {
		wantSet[id] = true
	}
	var extra, missing []string
	for _, id := range gotSorted {
		if !wantSet[id] {
			extra = append(extra, id)
		}
	}
	for _, id := range wantSorted {
		if !gotSet[id] {
			missing = append(missing, id)
		}
	}
	t.Errorf("%s: got %v, want %v (extra=%v missing=%v)", label, gotSorted, wantSorted, extra, missing)
}

// TestMigratePartialFailureResume is this plan's three-scenario proof
// against a real pinned Qdrant. Each subtest builds its own collection,
// seeds legacy records with seedLegacyRecord, and runs Store.Migrate with
// Steps: []migrate.Step{markerStep(0, 1, "fault_marker")}, Target: 1 — the
// same helpers plan 03-01 established, reused rather than reimplemented.
func TestMigratePartialFailureResume(t *testing.T) {
	ctx := context.Background()

	t.Run("single mid-sequence failure self-heals in a later pass", func(t *testing.T) {
		c := dialTestClient(t)
		collection := testCollection("migrate_faultinject_selfheal")
		_ = c.DeleteCollection(ctx, collection)
		t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

		inj := &setPayloadFaultInjector{}
		fc := dialFaultInjectingTestClient(t, inj)
		s := newTestStore(t, fc, collection)
		if err := s.EnsureCollection(ctx, 3); err != nil {
			t.Fatalf("EnsureCollection: %v", err)
		}

		ids := make([]string, 6)
		for i := range ids {
			ids[i] = fmt.Sprintf("cf500000-0000-0000-0000-%012d", i+1)
			seedLegacyRecord(ctx, t, s, ids[i])
		}

		// The second point written (ordinal 2, across the whole run) never
		// reaches the server. A single mid-sequence write drop, one pass's
		// worth of records short of the full backlog.
		inj.arm(2, 1, faultBeforeInvoke)

		res, err := s.Migrate(ctx, MigrateOptions{
			Target: 1,
			Steps:  []migrate.Step{markerStep(0, 1, "fault_marker")},
			Batch:  3,
		})
		if err != nil {
			t.Fatalf("Migrate: got error %v, want nil — a single mid-sequence failure must self-heal in a later pass", err)
		}
		if res.Failed != 1 {
			t.Errorf("Failed = %d, want 1", res.Failed)
		}
		// Passes > 1 is present but is NOT this scenario's load-bearing
		// claim (review cycle 1): a sweep that always performs one
		// confirming extra pass satisfies Passes > 1 too, whether or not
		// it actually re-derived and recovered anything. The load-bearing
		// evidence is Failed==1 together with the per-record rawPayload
		// reads below, which show the SPECIFIC record whose write was
		// dropped now carries the marker — reachable only by a LATER pass
		// having re-derived and re-processed it.
		if res.Passes <= 1 {
			t.Errorf("Passes = %d, want > 1 (weak signal only — see comment)", res.Passes)
		}

		if backlog := migrateBacklogIDs(ctx, t, s, 1); len(backlog) != 0 {
			t.Errorf("backlog after self-heal = %v, want empty", backlog)
		}

		// Per-record state, one record at a time — a count is not
		// evidence about WHICH record.
		for _, id := range ids {
			raw := rawPayload(ctx, t, s, id)
			if got := raw["fault_marker"].GetStringValue(); got != "marker:fault_marker" {
				t.Errorf("record %s: fault_marker = %q, want %q", id, got, "marker:fault_marker")
			}
			if got := raw[schemaVersionKey].GetIntegerValue(); got != 1 {
				t.Errorf("record %s: schema_version = %d, want 1", id, got)
			}
		}

		// Guard against a vacuous run (durable record x6v6qxqd6f): an
		// interceptor wired to a client the sweep never used would make
		// every "converged" assertion above true for the wrong reason.
		if inj.seen() == 0 {
			t.Fatalf("injector observed zero writes — this subtest is vacuous")
		}
		if got := inj.injected(); got != 1 {
			t.Errorf("injector injected = %d, want 1 (armed count)", got)
		}
	})

	t.Run("persistent failure terminates, and the resume converges", func(t *testing.T) {
		c := dialTestClient(t)
		collection := testCollection("migrate_faultinject_resume")
		_ = c.DeleteCollection(ctx, collection)
		t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

		inj := &setPayloadFaultInjector{}
		fc := dialFaultInjectingTestClient(t, inj)
		s := newTestStore(t, fc, collection)
		if err := s.EnsureCollection(ctx, 3); err != nil {
			t.Fatalf("EnsureCollection: %v", err)
		}

		seededIDs := make([]string, 6)
		for i := range seededIDs {
			seededIDs[i] = fmt.Sprintf("cf600000-0000-0000-0000-%012d", i+1)
			seedLegacyRecord(ctx, t, s, seededIDs[i])
		}

		// Every write from ordinal 2 onward fails before reaching the
		// server — unbounded, so the run cannot outrun the failure.
		inj.arm(2, 0, faultBeforeInvoke)

		opts := MigrateOptions{
			Target: 1,
			Steps:  []migrate.Step{markerStep(0, 1, "fault_marker")},
			Batch:  3,
		}

		res, err := s.Migrate(ctx, opts)
		if err == nil {
			t.Fatalf("Migrate: got nil error, want non-nil — a persistent write failure must terminate on the non-shrinking-backlog guard (PA-3)")
		}
		if !strings.Contains(err.Error(), "did not shrink between passes") {
			t.Errorf("error %q does not name the non-shrinking-backlog guard", err.Error())
		}
		if res.Migrated != 1 {
			t.Errorf("Migrated = %d, want 1", res.Migrated)
		}

		if inj.seen() == 0 {
			t.Fatalf("injector observed zero writes — this subtest is vacuous")
		}

		// succeededID is derived from CAPTURED WIRE TRAFFIC — the point id
		// carried by the injector's FIRST recorded SetPayload, ordinal 1,
		// the only write never armed to fail — never from fixture
		// insertion order (review cycle 1: Qdrant's scroll order is not a
		// contract, so "the first seeded record succeeded" would be an
		// assumption, not a fact).
		recordedIDs := inj.ids()
		if len(recordedIDs) == 0 {
			t.Fatalf("injector recorded zero write ids — cannot derive succeededID")
		}
		succeededID := recordedIDs[0]

		wantOutstanding := make([]string, 0, len(seededIDs)-1)
		for _, id := range seededIDs {
			if id != succeededID {
				wantOutstanding = append(wantOutstanding, id)
			}
		}
		assertSortedIDSetEqual(t, "backlog after persistent failure",
			migrateBacklogIDs(ctx, t, s, 1), wantOutstanding)

		// The resume: disarm the first injector, then call Migrate again
		// through a SECOND *Store built on a SEPARATE client. The second
		// *Store is the whole point — a resume is nothing more than
		// calling the method again, because there is no cursor to be
		// stale (D-07).
		//
		// resumeInj is a test OBSERVER on the resume's own client, not
		// shared sweep state — it holds no cursor and no failed-id set
		// and nothing Store.Migrate reads — and it exists because an
		// assertion phrased against the FIRST client's injector could not
		// see the resume's writes at all and would therefore have been
		// vacuously true (PA-16, review cycle 1's confirmed HIGH finding).
		inj.disarm()
		resumeInj := &setPayloadFaultInjector{}
		resumeClient := dialFaultInjectingTestClient(t, resumeInj)
		s2 := newTestStore(t, resumeClient, collection)

		res2, err2 := s2.Migrate(ctx, opts)
		if err2 != nil {
			t.Fatalf("resume Migrate: %v", err2)
		}
		if res2.Backlog != 0 {
			t.Errorf("resume Backlog = %d, want 0", res2.Backlog)
		}
		if backlog := migrateBacklogIDs(ctx, t, s, 1); len(backlog) != 0 {
			t.Errorf("backlog after resume = %v, want empty", backlog)
		}
		for _, id := range seededIDs {
			raw := rawPayload(ctx, t, s, id)
			if got := raw["fault_marker"].GetStringValue(); got != "marker:fault_marker" {
				t.Errorf("record %s: fault_marker = %q, want %q", id, got, "marker:fault_marker")
			}
		}

		// The resume's writes must be OBSERVED, not assumed. Without
		// seen() > 0, the set-equality assertion below is satisfiable by
		// an interceptor that saw nothing at all.
		if got := resumeInj.seen(); got == 0 {
			t.Fatalf("resumeInj.seen() = 0, want > 0 — the resume's writes were never observed, which would make the next assertion vacuously satisfiable")
		}
		// This is the assertion that distinguishes re-derivation from
		// replay: if the resume re-wrote the already-migrated record,
		// succeededID appears as an unexpected extra and this fails.
		assertSortedIDSetEqual(t, "resume's own recorded write ids", resumeInj.ids(), wantOutstanding)
	})

	t.Run("the error lies and the sweep converges anyway", func(t *testing.T) {
		c := dialTestClient(t)
		collection := testCollection("migrate_faultinject_lying")
		_ = c.DeleteCollection(ctx, collection)
		t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

		inj := &setPayloadFaultInjector{}
		fc := dialFaultInjectingTestClient(t, inj)
		s := newTestStore(t, fc, collection)
		if err := s.EnsureCollection(ctx, 3); err != nil {
			t.Fatalf("EnsureCollection: %v", err)
		}

		ids := make([]string, 4)
		for i := range ids {
			ids[i] = fmt.Sprintf("cf700000-0000-0000-0000-%012d", i+1)
			seedLegacyRecord(ctx, t, s, ids[i])
		}

		// Every write commits at the server; every call returns an error.
		inj.arm(1, 0, faultAfterInvoke)

		res, err := s.Migrate(ctx, MigrateOptions{
			Target: 1,
			Steps:  []migrate.Step{markerStep(0, 1, "fault_marker")},
			Batch:  4,
		})
		// This subtest proves D-09: the sweep's own counters are WRONG and
		// its conclusion is RIGHT, because the conclusion comes from a
		// fresh re-derivation and the counters come from a signal
		// (fail-after-invoke) that does not describe what landed.
		if err != nil {
			t.Fatalf("Migrate: got error %v, want nil — every write committed even though every call reported failure", err)
		}
		if res.Migrated != 0 {
			t.Errorf("Migrated = %d, want 0 (the write SIGNALS claim nothing landed)", res.Migrated)
		}
		if res.Failed != 4 {
			t.Errorf("Failed = %d, want 4 (the write SIGNALS claim nothing landed)", res.Failed)
		}
		if res.Backlog != 0 {
			t.Errorf("Backlog = %d, want 0 (the COLLECTION is right)", res.Backlog)
		}
		if backlog := migrateBacklogIDs(ctx, t, s, 1); len(backlog) != 0 {
			t.Errorf("backlog = %v, want empty", backlog)
		}
		for _, id := range ids {
			raw := rawPayload(ctx, t, s, id)
			if got := raw["fault_marker"].GetStringValue(); got != "marker:fault_marker" {
				t.Errorf("record %s: fault_marker = %q, want %q", id, got, "marker:fault_marker")
			}
			if got := raw[schemaVersionKey].GetIntegerValue(); got != 1 {
				t.Errorf("record %s: schema_version = %d, want 1", id, got)
			}
		}

		if inj.seen() == 0 {
			t.Fatalf("injector observed zero writes — this subtest is vacuous")
		}
		if got := inj.injected(); got != 4 {
			t.Errorf("injector injected = %d, want 4 — the scenario must be proven to have actually injected, not silently no-opped", got)
		}
	})
}
