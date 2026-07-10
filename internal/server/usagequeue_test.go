// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/metric"

	"github.com/seanb4t/engram/internal/telemetry"
)

// newTestUsageMetrics builds a *telemetry.UsageQueueMetrics backed by a
// manual reader so tests can assert exact counter values, mirroring
// newTestMetrics in summaryqueue_test.go.
func newTestUsageMetrics(t *testing.T) (*telemetry.UsageQueueMetrics, *metric.ManualReader) {
	t.Helper()
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	return telemetry.NewUsageQueueMetrics(mp.Meter("test")), reader
}

// usageShutdownWithinBudget runs q.Shutdown bounded by a fresh
// context.WithTimeout and fails the test if it takes appreciably longer than
// budget — a sleep-free way to bound test cleanup deterministically.
func usageShutdownWithinBudget(t *testing.T, q *usageQueue, budget time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	q.Shutdown(ctx)
}

// TestUsageQueueNeverBlocksWrite proves tryEnqueue never blocks the caller
// even while the single worker is hung on a fill that ignores its context
// entirely (the worst case: an uninterruptible call) — mirrors
// TestSummaryQueueNeverBlocksWrite.
func TestUsageQueueNeverBlocksWrite(t *testing.T) {
	release := make(chan struct{})
	fill := func(_ context.Context, _ string) error {
		<-release // deliberately ignores ctx: simulates a truly hung call
		return nil
	}
	metrics, _ := newTestUsageMetrics(t)
	q := newUsageQueue(1, 1, metrics, fill)
	q.Start(context.Background())
	t.Cleanup(func() {
		close(release)
		usageShutdownWithinBudget(t, q, 2*time.Second)
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

// TestUsageQueueDropsWhenFull asserts a saturated queue's overflow is
// dropped-and-counted, never blocked. No workers are started so the channel
// capacity alone determines exactly how many sends succeed, eliminating
// drain-timing raciness from this assertion.
func TestUsageQueueDropsWhenFull(t *testing.T) {
	const capacity = 3
	const overflow = 5
	fill := func(context.Context, string) error {
		t.Fatal("fill must never be invoked: no worker was started")
		return nil
	}
	metrics, reader := newTestUsageMetrics(t)
	q := newUsageQueue(1, capacity, metrics, fill)

	start := time.Now()
	for i := range capacity + overflow {
		q.tryEnqueue(fmt.Sprintf("id-%d", i))
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("enqueueing past capacity took %v; want near-instant (never blocks)", elapsed)
	}

	if got := counterSum(t, reader, "engram.usage_queue.dropped"); got != overflow {
		t.Errorf("dropped count = %d, want %d", got, overflow)
	}
	if got := len(q.ch); got != capacity {
		t.Errorf("channel occupancy = %d, want full capacity %d", got, capacity)
	}
}

// TestUsageQueueSingleAttemptNoRetry asserts a fill that always errors is
// invoked exactly once per enqueued id — never retried (D-10) — and each
// failure is counted.
func TestUsageQueueSingleAttemptNoRetry(t *testing.T) {
	var calls atomic.Int32
	fill := func(context.Context, string) error {
		calls.Add(1)
		return fmt.Errorf("increment failed")
	}
	metrics, reader := newTestUsageMetrics(t)
	q := newUsageQueue(1, 8, metrics, fill)
	q.Start(context.Background())
	t.Cleanup(func() { usageShutdownWithinBudget(t, q, 2*time.Second) })

	const n = 5
	for i := range n {
		q.tryEnqueue(fmt.Sprintf("always-fails-%d", i))
	}
	q.Wait()

	if got := calls.Load(); got != n {
		t.Errorf("fill invocations = %d, want %d (no retry)", got, n)
	}
	if got := counterSum(t, reader, "engram.usage_queue.failed"); got != n {
		t.Errorf("failed count = %d, want %d", got, n)
	}
}

// TestUsageQueuePanicDoesNotWedgeWait proves a fill that panics on one id
// does not kill the pool: the panic is recovered, that id counts failed,
// every other id still drains normally, and Wait() returns (never wedges).
// Run with -race.
func TestUsageQueuePanicDoesNotWedgeWait(t *testing.T) {
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
	metrics, reader := newTestUsageMetrics(t)
	q := newUsageQueue(2, 8, metrics, fill)
	q.Start(context.Background())
	t.Cleanup(func() { usageShutdownWithinBudget(t, q, 2*time.Second) })

	ids := []string{"a", panicID, "b", "c"}
	for _, id := range ids {
		q.tryEnqueue(id)
	}
	q.Wait() // must return despite the panic

	if got := counterSum(t, reader, "engram.usage_queue.failed"); got != 1 {
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

// TestUsageQueueShutdownDrainsWithinBudget asserts Shutdown returns within
// its passed context's budget even with in-flight work that never completes
// on its own, and does not panic. Run with -race.
func TestUsageQueueShutdownDrainsWithinBudget(t *testing.T) {
	release := make(chan struct{})
	fill := func(_ context.Context, _ string) error {
		<-release // deliberately left in-flight for this test
		return nil
	}
	metrics, _ := newTestUsageMetrics(t)
	q := newUsageQueue(1, 1, metrics, fill)
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

// TestUsageQueueEnqueueAfterShutdownIsDroppedNotPanic proves a late
// tryEnqueue — from a handler that outlives the server's shutdown grace
// window — is dropped-and-counted rather than panicking with "send on closed
// channel" (CR-01). Also asserts Shutdown is idempotent: a second Shutdown
// call must not double-close ch. Run with -race.
func TestUsageQueueEnqueueAfterShutdownIsDroppedNotPanic(t *testing.T) {
	metrics, reader := newTestUsageMetrics(t)
	q := newUsageQueue(1, 4, metrics, func(context.Context, string) error { return nil })
	q.Start(context.Background())
	usageShutdownWithinBudget(t, q, 2*time.Second)

	// The channel is now closed. A straggler handler calling tryEnqueue must
	// not panic; it drops-and-counts.
	q.tryEnqueue("late-after-close") // must not panic
	q.Wait()                         // returns immediately: the late id was never enqueued

	if got := counterSum(t, reader, "engram.usage_queue.dropped"); got != 1 {
		t.Errorf("dropped count = %d, want 1 (late enqueue dropped after shutdown)", got)
	}
	if got := counterSum(t, reader, "engram.usage_queue.enqueued"); got != 0 {
		t.Errorf("enqueued count = %d, want 0 (nothing accepted after shutdown)", got)
	}

	// A second Shutdown must be idempotent (no double-close panic).
	usageShutdownWithinBudget(t, q, 2*time.Second)
}

// TestUsageQueueDepthReflectsChannelOccupancy asserts depth() samples live
// channel occupancy, mirroring TestSummaryQueueDepthReflectsChannelOccupancy.
func TestUsageQueueDepthReflectsChannelOccupancy(t *testing.T) {
	metrics, _ := newTestUsageMetrics(t)
	q := newUsageQueue(1, 4, metrics, func(context.Context, string) error { return nil })

	if got := q.depth(); got != 0 {
		t.Fatalf("depth() on empty queue = %d, want 0", got)
	}
	q.tryEnqueue("a")
	q.tryEnqueue("b")
	if got := q.depth(); got != 2 {
		t.Fatalf("depth() after 2 enqueues = %d, want 2", got)
	}

	var nilQ *usageQueue
	if got := nilQ.depth(); got != 0 {
		t.Fatalf("nil queue depth() = %d, want 0", got)
	}
}

// TestUsageQueueFillSuccess is the happy path: every enqueued id is filled
// exactly once and Wait() returns once all have completed.
func TestUsageQueueFillSuccess(t *testing.T) {
	var calls atomic.Int32
	fill := func(_ context.Context, _ string) error {
		calls.Add(1)
		return nil
	}
	metrics, reader := newTestUsageMetrics(t)
	q := newUsageQueue(2, 8, metrics, fill)
	q.Start(context.Background())
	t.Cleanup(func() { usageShutdownWithinBudget(t, q, 2*time.Second) })

	const n = 5
	for i := range n {
		q.tryEnqueue(fmt.Sprintf("ok-%d", i))
	}
	q.Wait()

	if got := calls.Load(); got != n {
		t.Errorf("fill invocations = %d, want %d", got, n)
	}
	if got := counterSum(t, reader, "engram.usage_queue.failed"); got != 0 {
		t.Errorf("failed count = %d, want 0", got)
	}
	if got := counterSum(t, reader, "engram.usage_queue.enqueued"); got != n {
		t.Errorf("enqueued count = %d, want %d", got, n)
	}
}

// TestUsageQueueNilIsNoOp asserts every method on a nil *usageQueue is a
// safe no-op, mirroring the disabled-state contract summaryQueue documents.
func TestUsageQueueNilIsNoOp(t *testing.T) {
	var q *usageQueue
	q.tryEnqueue("id") // must not panic
	q.Start(context.Background())
	q.Wait()
	q.Shutdown(context.Background())
	if got := q.depth(); got != 0 {
		t.Fatalf("nil queue depth() = %d, want 0", got)
	}
}
