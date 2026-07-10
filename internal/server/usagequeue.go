// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"log/slog"
	"sync"

	"github.com/seanb4t/engram/internal/telemetry"
)

// usageQueue is a bounded, non-blocking, nil-safe in-process worker pool that
// drains record ids through an injected single-attempt fill (production
// wiring passes st.IncrementAccess in Plan 06). It reuses the Phase 11
// summaryQueue's shutdown-safety kernel (RWMutex closed guard + inFlight
// reserve-before-send, the CR-01 fix) but drops all retry/backoff machinery
// (D-10): the get-path counter bump is fire-and-forget, so a lost bump is
// acceptable and never worth stalling a worker on. A nil *usageQueue is the
// disabled state: every method is a no-op guarded by a receiver nil check, so
// call sites never need to branch on whether usage signals are enabled.
type usageQueue struct {
	ch chan string

	// mu guards ch's closed state so a late tryEnqueue can never send on a
	// closed channel and panic (CR-01, mirrored from summaryQueue). tryEnqueue
	// takes RLock and checks closed before sending; Shutdown takes Lock before
	// close, making send and close mutually exclusive.
	mu     sync.RWMutex
	closed bool

	// wg tracks worker goroutine lifecycle: Add in Start, Done when a
	// worker's `range ch` loop exits (i.e. ch has been closed by Shutdown).
	wg sync.WaitGroup

	// inFlight is the deterministic drain seam: incremented exactly once per
	// successfully enqueued (non-dropped) id, decremented via itemDone in
	// process — whether the fill succeeds, errors, or panics. Wait() blocks on
	// this instead of a test polling on time.Sleep.
	inFlight sync.WaitGroup

	// fill is the injected per-id increment operation. Production wiring uses
	// st.IncrementAccess (Plan 06); tests inject a fake closure so the whole
	// suite runs without Qdrant.
	fill func(ctx context.Context, id string) error

	workers int
	metrics *telemetry.UsageQueueMetrics
}

// newUsageQueue constructs a usageQueue with a bounded channel of capacity
// queueSize and workers worker goroutines (started separately via Start).
// Unlike newSummaryQueue there is no attemptTimeout/maxElapsed parameter: D-10
// mandates a single fill attempt with no retry budget to derive. metrics may
// be nil (all record calls below are nil-guarded) though production wiring
// always supplies one.
func newUsageQueue(workers, queueSize int, metrics *telemetry.UsageQueueMetrics, fill func(ctx context.Context, id string) error) *usageQueue {
	return &usageQueue{
		ch:      make(chan string, queueSize),
		fill:    fill,
		workers: workers,
		metrics: metrics,
	}
}

// tryEnqueue is the get-path call site's entry point: nil-safe (a disabled
// queue is a no-op), non-blocking (a full queue drops and counts instead of
// stalling the caller — D-04), and never returns an error since the caller
// must never be slowed down by this best-effort path.
func (q *usageQueue) tryEnqueue(id string) {
	if q == nil {
		return
	}
	// RLock serializes only against Shutdown's close (concurrent enqueues
	// still proceed in parallel); it guarantees the closed-check and the send
	// below cannot straddle Shutdown's close(ch), so a late enqueue drops
	// instead of panicking on send-to-closed (CR-01).
	q.mu.RLock()
	defer q.mu.RUnlock()
	if q.closed {
		if q.metrics != nil {
			q.metrics.Dropped(context.Background())
		}
		slog.Warn("usage queue closed; dropped access-count increment", "id", id)
		return
	}
	// Reserve the in-flight slot BEFORE the send so a fast worker cannot run
	// itemDone (inFlight.Done) before this Add and drive the WaitGroup
	// negative; the drop branch releases the reservation to stay balanced.
	q.inFlight.Add(1)
	select {
	case q.ch <- id:
		if q.metrics != nil {
			q.metrics.Enqueued(context.Background())
		}
	default:
		q.inFlight.Done()
		if q.metrics != nil {
			q.metrics.Dropped(context.Background())
		}
		slog.Warn("usage queue full; dropped access-count increment", "id", id)
	}
}

// Start launches q.workers long-lived worker goroutines that range over the
// enqueue channel until it is closed (by Shutdown). Nil-safe no-op on a
// disabled queue.
func (q *usageQueue) Start(ctx context.Context) {
	if q == nil {
		return
	}
	for range q.workers {
		q.wg.Add(1)
		go q.worker(ctx)
	}
}

// worker drains ids off the channel until it is closed, processing each one
// in turn.
func (q *usageQueue) worker(ctx context.Context) {
	defer q.wg.Done()
	for id := range q.ch {
		q.process(ctx, id)
	}
}

// process runs one id through a SINGLE fill attempt — no retry, no backoff
// (D-10) — and always balances the in-flight accounting via itemDone, even if
// fill panics. A panicking fill is recovered here so it counts as a failed
// outcome and the worker keeps draining subsequent ids; the pool can never
// wedge and Wait() always eventually returns.
func (q *usageQueue) process(ctx context.Context, id string) {
	defer q.itemDone()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("usage queue: fill panicked; recovered", "id", id, "panic", r)
			if q.metrics != nil {
				q.metrics.Failed(context.Background())
			}
		}
	}()

	if err := q.fill(ctx, id); err != nil {
		// Single attempt only (D-10): a lost bump is acceptable and never
		// worth stalling this worker over.
		if q.metrics != nil {
			q.metrics.Failed(context.Background())
		}
		slog.Warn("usage queue: fill failed", "id", id, "err", err)
	}
}

// itemDone balances one successfully-enqueued id's in-flight accounting.
// Always called via defer in process, regardless of how process exits
// (success, error, or a recovered panic), so Wait() can never wedge.
func (q *usageQueue) itemDone() {
	q.inFlight.Done()
}

// Shutdown stops accepting new work and waits for in-flight fills to finish,
// bounded by ctx. This method takes mu.Lock (mutually exclusive with
// tryEnqueue's RLock) and flips closed before close(ch), so a handler that
// outlives the server's shutdown grace window is dropped by tryEnqueue rather
// than panicking on send-to-closed (CR-01). Idempotent, and never hangs past
// ctx's deadline. Nil-safe no-op on a disabled queue.
func (q *usageQueue) Shutdown(ctx context.Context) {
	if q == nil {
		return
	}
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return // idempotent: a second Shutdown must not double-close ch
	}
	q.closed = true
	close(q.ch)
	q.mu.Unlock()
	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

// Wait blocks until every enqueued (non-dropped) id currently in flight has
// reached a terminal fill outcome (success, error, or a recovered panic) —
// the deterministic drain seam tests use instead of time.Sleep polling.
// Nil-safe no-op on a disabled queue.
func (q *usageQueue) Wait() {
	if q == nil {
		return
	}
	q.inFlight.Wait()
}

// depth samples the current enqueue channel occupancy for a future D-09-style
// OTel queue-depth gauge, mirroring summaryQueue.depth. Nil-safe: a disabled
// queue reports depth 0.
func (q *usageQueue) depth() int64 {
	if q == nil {
		return 0
	}
	return int64(len(q.ch))
}
