// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v5"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/telemetry"
)

// newTestMetrics builds a *telemetry.SummaryQueueMetrics backed by a manual
// reader so tests can assert exact counter values, mirroring
// internal/telemetry/metrics_test.go's "real provider assertions" pattern.
func newTestMetrics(t *testing.T) (*telemetry.SummaryQueueMetrics, *metric.ManualReader) {
	t.Helper()
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	return telemetry.NewSummaryQueueMetrics(mp.Meter("test")), reader
}

// counterSum collects and sums every data point of the named Int64 counter.
func counterSum(t *testing.T, reader *metric.ManualReader, name string) int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}
	var total int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				continue
			}
			for _, dp := range sum.DataPoints {
				total += dp.Value
			}
		}
	}
	return total
}

// shutdownWithinBudget runs q.Shutdown bounded by a fresh context.WithTimeout
// and fails the test if it takes appreciably longer than budget — a
// sleep-free way to bound test cleanup deterministically.
func shutdownWithinBudget(t *testing.T, q *summaryQueue, budget time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	q.Shutdown(ctx)
}

// TestSummaryQueueNeverBlocksWrite proves tryEnqueue never blocks the caller
// even while the single worker is hung on a fill that ignores its context
// entirely (the worst case: an uninterruptible call) — SC#2, T-11-02.
func TestSummaryQueueNeverBlocksWrite(t *testing.T) {
	release := make(chan struct{})
	fill := func(_ context.Context, _ string) error {
		<-release // deliberately ignores ctx: simulates a truly hung call
		return nil
	}
	metrics, _ := newTestMetrics(t)
	q := newSummaryQueue(1, 1, time.Minute, metrics, fill)
	q.Start(context.Background())
	t.Cleanup(func() {
		close(release)
		shutdownWithinBudget(t, q, 2*time.Second)
	})

	// Consumed immediately by the single worker, which now hangs on release.
	q.tryEnqueue("first")

	start := time.Now()
	for i := range 100 {
		q.tryEnqueue(fmt.Sprintf("id-%d", i))
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("100 tryEnqueue calls took %v; want near-instant even while the sole worker is hung", elapsed)
	}
}

// TestSummaryQueueDropsWhenFull asserts a saturated queue's overflow is
// dropped-and-counted, never blocked — T-11-01. No workers are started so the
// channel capacity alone determines exactly how many sends succeed,
// eliminating drain-timing raciness from this assertion.
func TestSummaryQueueDropsWhenFull(t *testing.T) {
	const capacity = 3
	const overflow = 5
	fill := func(context.Context, string) error {
		t.Fatal("fill must never be invoked: no worker was started")
		return nil
	}
	metrics, reader := newTestMetrics(t)
	q := newSummaryQueue(1, capacity, time.Second, metrics, fill)

	start := time.Now()
	for i := range capacity + overflow {
		q.tryEnqueue(fmt.Sprintf("id-%d", i))
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("enqueueing past capacity took %v; want near-instant (never blocks)", elapsed)
	}

	if got := counterSum(t, reader, "engram.summary_queue.dropped"); got != overflow {
		t.Errorf("dropped count = %d, want %d", got, overflow)
	}
	if got := len(q.ch); got != capacity {
		t.Errorf("channel occupancy = %d, want full capacity %d", got, capacity)
	}
}

// TestSummaryQueueRetryGivesUp asserts a fill that always fails is retried up
// to the bounded attempt count, then counted failed — and that the whole
// give-up sequence stays well under a small wall-clock ceiling so a
// brownout can't stall the pool (D-04, T-11-03).
func TestSummaryQueueRetryGivesUp(t *testing.T) {
	metrics, reader := newTestMetrics(t)
	q := newSummaryQueue(1, 1, 2*time.Second, metrics, func(context.Context, string) error {
		return fmt.Errorf("transient gateway error")
	})
	q.Start(context.Background())
	t.Cleanup(func() { shutdownWithinBudget(t, q, 2*time.Second) })

	start := time.Now()
	q.tryEnqueue("always-fails")
	q.Wait()
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Fatalf("retry-to-failure took %v; want a small bounded ceiling (pool must not stall)", elapsed)
	}
	if got := counterSum(t, reader, "engram.summary_queue.failed"); got != 1 {
		t.Errorf("failed count = %d, want 1", got)
	}
	// summaryQueueMaxTries=3 means 2 failed attempts are retried before the
	// 3rd (final) attempt also fails and the operation gives up.
	if got := counterSum(t, reader, "engram.summary_queue.retried"); got < 2 {
		t.Errorf("retried count = %d, want >= 2", got)
	}
}

// TestSummaryQueueHungFillIsInterrupted proves a fill that blocks on
// <-ctx.Done() (never returning on its own) is cut at the injected small
// attemptTimeout rather than stalling the worker indefinitely — Codex
// finding #2, T-11-03.
func TestSummaryQueueHungFillIsInterrupted(t *testing.T) {
	var attempts atomic.Int32
	fill := func(ctx context.Context, _ string) error {
		attempts.Add(1)
		<-ctx.Done() // observes the per-attempt cancellation
		return ctx.Err()
	}
	metrics, reader := newTestMetrics(t)
	q := newSummaryQueue(1, 1, 20*time.Millisecond, metrics, fill)
	q.Start(context.Background())
	t.Cleanup(func() { shutdownWithinBudget(t, q, 2*time.Second) })

	start := time.Now()
	q.tryEnqueue("hangs-forever")
	q.Wait()
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Fatalf("hung-fill interruption took %v; want bounded by the tiny attemptTimeout, not an unbounded hang", elapsed)
	}
	if got := attempts.Load(); got < 1 {
		t.Fatalf("fill was never invoked")
	}
	if got := counterSum(t, reader, "engram.summary_queue.failed"); got != 1 {
		t.Errorf("failed count = %d, want 1", got)
	}
}

// TestSummaryQueuePanicDoesNotWedgeWait proves a fill that panics on one id
// does not kill the pool: the panic is recovered, that id counts failed,
// every other id still drains normally, and Wait() returns (never wedges)
// — Codex finding #4, T-11-04. Run with -race.
func TestSummaryQueuePanicDoesNotWedgeWait(t *testing.T) {
	const panicID = "boom"
	var mu sync.Mutex
	seen := map[string]bool{}
	fill := func(_ context.Context, id string) error {
		if id == panicID {
			panic("simulated fill panic")
		}
		mu.Lock()
		seen[id] = true
		mu.Unlock()
		return nil
	}
	metrics, reader := newTestMetrics(t)
	q := newSummaryQueue(2, 8, time.Second, metrics, fill)
	q.Start(context.Background())
	t.Cleanup(func() { shutdownWithinBudget(t, q, 2*time.Second) })

	ids := []string{"a", panicID, "b", "c"}
	for _, id := range ids {
		q.tryEnqueue(id)
	}
	q.Wait() // must return despite the panic

	if got := counterSum(t, reader, "engram.summary_queue.failed"); got != 1 {
		t.Errorf("failed count = %d, want 1 (only the panicking id)", got)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, id := range []string{"a", "b", "c"} {
		if !seen[id] {
			t.Errorf("id %q was never processed; pool must keep draining after a panic", id)
		}
	}
	if seen[panicID] {
		t.Errorf("panicking id must not have reached the success path")
	}
}

// TestSummaryQueueShutdownDrainsWithinBudget asserts Shutdown returns within
// its passed context's budget even with in-flight work that never completes
// on its own, and does not panic — D-08, T-11-04. Run with -race.
func TestSummaryQueueShutdownDrainsWithinBudget(t *testing.T) {
	release := make(chan struct{})
	fill := func(_ context.Context, _ string) error {
		<-release // deliberately left in-flight for this test
		return nil
	}
	metrics, _ := newTestMetrics(t)
	q := newSummaryQueue(1, 1, time.Minute, metrics, fill)
	q.Start(context.Background())
	q.tryEnqueue("stuck")

	budget := 200 * time.Millisecond
	shCtx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()

	start := time.Now()
	q.Shutdown(shCtx)
	elapsed := time.Since(start)

	close(release) // let the stuck worker finish so nothing leaks past the test

	if elapsed > budget+500*time.Millisecond {
		t.Fatalf("Shutdown took %v, want close to its %v budget (bounded, never hangs)", elapsed, budget)
	}
}

// TestSummaryQueueFillSuccess is the happy path: every enqueued id is filled
// exactly once and Wait() returns once all have completed.
func TestSummaryQueueFillSuccess(t *testing.T) {
	var calls atomic.Int32
	fill := func(_ context.Context, _ string) error {
		calls.Add(1)
		return nil
	}
	metrics, reader := newTestMetrics(t)
	q := newSummaryQueue(2, 8, time.Second, metrics, fill)
	q.Start(context.Background())
	t.Cleanup(func() { shutdownWithinBudget(t, q, 2*time.Second) })

	const n = 5
	for i := range n {
		q.tryEnqueue(fmt.Sprintf("ok-%d", i))
	}
	q.Wait()

	if got := calls.Load(); got != n {
		t.Errorf("fill invocations = %d, want %d", got, n)
	}
	if got := counterSum(t, reader, "engram.summary_queue.failed"); got != 0 {
		t.Errorf("failed count = %d, want 0", got)
	}
	if got := counterSum(t, reader, "engram.summary_queue.enqueued"); got != n {
		t.Errorf("enqueued count = %d, want %d", got, n)
	}
}

// TestSummaryQueueDepthReflectsChannelOccupancy asserts depth() samples live
// channel occupancy for the D-09 OTel gauge callback (registered separately
// by Wave-3 buildDepsFromEnv, not here) — Qdrant-free, no workers started so
// occupancy is deterministic.
func TestSummaryQueueDepthReflectsChannelOccupancy(t *testing.T) {
	metrics, _ := newTestMetrics(t)
	q := newSummaryQueue(1, 4, time.Second, metrics, func(context.Context, string) error { return nil })

	if got := q.depth(); got != 0 {
		t.Fatalf("depth() on empty queue = %d, want 0", got)
	}
	q.tryEnqueue("a")
	q.tryEnqueue("b")
	if got := q.depth(); got != 2 {
		t.Fatalf("depth() after 2 enqueues = %d, want 2", got)
	}

	var nilQ *summaryQueue
	if got := nilQ.depth(); got != 0 {
		t.Fatalf("nil queue depth() = %d, want 0", got)
	}
}

// TestSummaryQueueEnqueueAfterShutdownIsDroppedNotPanic proves a late
// tryEnqueue — from an HTTP handler that outlived httpSrv.Shutdown's grace
// window (net/http does not force-terminate handlers on Shutdown-ctx expiry) —
// is dropped-and-counted rather than panicking with "send on closed channel"
// (CR-01). Run with -race.
func TestSummaryQueueEnqueueAfterShutdownIsDroppedNotPanic(t *testing.T) {
	metrics, reader := newTestMetrics(t)
	q := newSummaryQueue(1, 4, time.Second, metrics, func(context.Context, string) error { return nil })
	q.Start(context.Background())
	shutdownWithinBudget(t, q, 2*time.Second)

	// The channel is now closed. A straggler handler calling tryEnqueue must
	// not panic; it drops-and-counts (summarize-missing reclaims it later).
	q.tryEnqueue("late-after-close") // must not panic
	q.Wait()                         // returns immediately: the late id was never enqueued

	if got := counterSum(t, reader, "engram.summary_queue.dropped"); got != 1 {
		t.Errorf("dropped count = %d, want 1 (late enqueue dropped after shutdown)", got)
	}
	if got := counterSum(t, reader, "engram.summary_queue.enqueued"); got != 0 {
		t.Errorf("enqueued count = %d, want 0 (nothing accepted after shutdown)", got)
	}
}

// TestSummaryQueueRetryBudgetAccommodatesFullTryCount is the WR-02 regression:
// the total retry wall-time budget must be able to hold summaryQueueMaxTries
// attempts each lasting up to the full per-attempt timeout. A hardcoded budget
// shorter than the per-attempt timeout (the old 20s vs a 30s
// ENGRAM_SUMMARY_TIMEOUT) fires the elapsed cap after one slow attempt and
// silently collapses the 3-try budget to one.
func TestSummaryQueueRetryBudgetAccommodatesFullTryCount(t *testing.T) {
	attemptTimeout := 30 * time.Second
	q := newSummaryQueue(1, 1, attemptTimeout, nil, func(context.Context, string) error { return nil })
	if want := attemptTimeout * time.Duration(summaryQueueMaxTries); q.maxElapsed < want {
		t.Errorf("maxElapsed = %v; want >= attemptTimeout*maxTries (%v) so the elapsed cap cannot pre-empt the try count", q.maxElapsed, want)
	}
}

// TestSummaryQueueBackoffBudgetIndependentOfEmbedTimeout is the D-09
// assert-only regression: the summary-queue backoff budget (maxElapsed) is
// derived exclusively from the attemptTimeout argument the caller passes —
// which the production wiring (tools.go: newSummaryQueue(..., summaryTimeout(cfg), ...))
// sources from ENGRAM_SUMMARY_TIMEOUT, never from ENGRAM_EMBED_TIMEOUT or any
// other embed-derived value. Two queues built with distinct attemptTimeout
// values must see maxElapsed scale proportionally with attemptTimeout, not
// collapse to a shared embed-derived 30s literal.
func TestSummaryQueueBackoffBudgetIndependentOfEmbedTimeout(t *testing.T) {
	short := newSummaryQueue(1, 1, 5*time.Second, nil, func(context.Context, string) error { return nil })
	long := newSummaryQueue(1, 1, 60*time.Second, nil, func(context.Context, string) error { return nil })

	if long.maxElapsed <= short.maxElapsed {
		t.Fatalf("maxElapsed did not scale with attemptTimeout: short(5s)=%v, long(60s)=%v; want long > short", short.maxElapsed, long.maxElapsed)
	}

	// Pin the exact derivation so a future refactor cannot silently reintroduce
	// a fixed/embed-derived constant.
	wantShort := (5*time.Second + summaryQueueMaxInterval) * time.Duration(summaryQueueMaxTries)
	wantLong := (60*time.Second + summaryQueueMaxInterval) * time.Duration(summaryQueueMaxTries)
	if short.maxElapsed != wantShort {
		t.Errorf("short.maxElapsed = %v, want %v", short.maxElapsed, wantShort)
	}
	if long.maxElapsed != wantLong {
		t.Errorf("long.maxElapsed = %v, want %v", long.maxElapsed, wantLong)
	}
}

// TestStoreFillFillsEligibleRecord exercises the production fill-builder
// (storeFill, wired into the queue by Wave 3) end-to-end against a live
// Qdrant-backed store: an eligible record (long content, no summary) is
// filled exactly like the summarize-missing sweep's FillSummary call, and
// re-reading it back shows the persisted summary. Integration: needs Qdrant
// (skip-gated via testDeps, same posture as the rest of this package).
func TestStoreFillFillsEligibleRecord(t *testing.T) {
	d, st := testDepsWithStore(t)
	ctx := context.Background()
	id := "10000000-0000-0000-0000-000000000001"
	long := "this is a long body well over the tiny cap used by this integration test"
	m := store.Memory{
		ID: id, Content: long, Scope: "repo:summaryqueue-fill",
		Category: "convention", Source: "agent-inferred", Owner: "owner-Q",
		CreatedAt: time.Now().UTC(),
	}
	if err := d.st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Cleanup(func() { cleanupErr(t, "delete", d.st.Delete(ctx, id, store.Authenticated("owner-Q"))) })

	summarize := func(context.Context, string) (string, error) { return "queue fill summary", nil }
	fill := storeFill(st, summarize, "test-model", 8)
	if err := fill(ctx, id); err != nil {
		t.Fatalf("storeFill: %v", err)
	}

	got, err := d.st.GetReadable(ctx, id, store.Authenticated("owner-Q"))
	if err != nil {
		t.Fatalf("GetReadable: %v", err)
	}
	if got.Summary != "queue fill summary" {
		t.Fatalf("Summary = %q, want %q", got.Summary, "queue fill summary")
	}
}

// TestStoreFillReturnsPermanentOnNotFound asserts a vanished record's Get
// error is wrapped via backoff.Permanent so the retry loop never retries a
// not-found id (RESEARCH Pattern 2). Integration: needs Qdrant.
func TestStoreFillReturnsPermanentOnNotFound(t *testing.T) {
	_, st := testDepsWithStore(t)
	summarize := func(context.Context, string) (string, error) { return "unused", nil }
	fill := storeFill(st, summarize, "test-model", 8)

	err := fill(context.Background(), "20000000-0000-0000-0000-000000000099")
	if err == nil {
		t.Fatal("expected an error for a not-found id")
	}
	var perm *backoff.PermanentError
	if !errors.As(err, &perm) {
		t.Fatalf("expected a *backoff.PermanentError (not retryable), got %T: %v", err, err)
	}
}
