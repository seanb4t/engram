// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v5"

	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/telemetry"
)

const (
	// summaryQueueMaxTries bounds the retry attempt COUNT per id — an
	// explicit low value (not the library's implicit unbounded-by-count
	// default) so a sustained gateway brownout drains-to-failure fast (D-04).
	// This is the PRIMARY, binding retry bound; the wall-time budget
	// (maxElapsed, a per-queue field derived from attemptTimeout) is only a
	// coherent backstop that can never fire before this count is reached
	// (WR-02).
	summaryQueueMaxTries = 3
	// summaryQueueMaxInterval caps the exponential backoff sleep between
	// attempts — an explicit low ceiling, not the library's 60s
	// ExponentialBackOff default (tuned for longer-running network
	// operations than an in-process retry loop).
	summaryQueueMaxInterval = 2 * time.Second
)

// summaryQueue is a bounded, non-blocking, nil-safe in-process worker pool
// that drains record ids through the existing Store.FillSummary seam. A nil
// *summaryQueue is the disabled state (ENGRAM_SUMMARY_ON_WRITE=false or no
// summary model configured): every method is a no-op guarded by a receiver
// nil check, so call sites never need to branch on whether async summaries
// are enabled.
type summaryQueue struct {
	ch chan string

	// mu guards ch's closed state so a late tryEnqueue can never send on a
	// closed channel and panic (CR-01). The D-08 ordering invariant
	// (serve.go drains only AFTER httpSrv.Shutdown returns) is necessary but
	// NOT sufficient: net/http.Server.Shutdown does NOT force-terminate active
	// handlers when its context deadline expires — it just stops waiting — so
	// a slow store_memory/schedule_memory handler (exactly the gateway-brownout
	// case this feature targets) can still be running when Shutdown closes ch.
	// tryEnqueue takes RLock and checks closed before sending; Shutdown takes
	// Lock before close, making send and close mutually exclusive.
	mu     sync.RWMutex
	closed bool

	// wg tracks worker goroutine lifecycle: Add in Start, Done when a
	// worker's `range ch` loop exits (i.e. ch has been closed by Shutdown).
	wg sync.WaitGroup

	// inFlight is the deterministic drain seam: incremented exactly once per
	// successfully enqueued (non-dropped) id, decremented via itemDone in
	// process — whether the fill succeeds, exhausts retries, or panics.
	// Wait() blocks on this instead of a test polling on time.Sleep.
	inFlight sync.WaitGroup

	// fill is the injected per-id fill operation. Production wiring uses
	// storeFill (below); tests inject a fake closure so the whole suite runs
	// without Qdrant or a real summarizer.
	fill func(ctx context.Context, id string) error

	// attemptTimeout bounds EACH individual fill attempt via
	// context.WithTimeout, independent of the backoff retry budget. Without
	// this, a single hung attempt would burn the full retry elapsed-time
	// budget (backoff only evaluates elapsed time AFTER operation returns) —
	// see worker/process below (Codex finding #2).
	attemptTimeout time.Duration

	// maxElapsed is the total retry wall-time budget per id, DERIVED from
	// attemptTimeout in newSummaryQueue rather than hardcoded, so it is always
	// large enough to accommodate summaryQueueMaxTries attempts at the
	// configured per-attempt timeout. A hardcoded budget shorter than the
	// per-attempt timeout (the old 20s vs a 30s ENGRAM_SUMMARY_TIMEOUT) would
	// let the elapsed cap fire after a single slow attempt and silently
	// collapse the intended try count to one (WR-02).
	maxElapsed time.Duration

	workers int
	metrics *telemetry.SummaryQueueMetrics
}

// newSummaryQueue constructs a summaryQueue with a bounded channel of
// capacity queueSize and workers worker goroutines (started separately via
// Start). attemptTimeout bounds each individual fill attempt; tests inject a
// tiny value for deterministic hung-fill interruption, production wiring
// passes summaryTimeout(cfg) so operators tune it via the existing
// ENGRAM_SUMMARY_TIMEOUT — no new env var. metrics may be nil (all record
// calls below are nil-guarded) though production wiring always supplies one.
func newSummaryQueue(workers, queueSize int, attemptTimeout time.Duration, metrics *telemetry.SummaryQueueMetrics, fill func(ctx context.Context, id string) error) *summaryQueue {
	return &summaryQueue{
		ch:             make(chan string, queueSize),
		fill:           fill,
		attemptTimeout: attemptTimeout,
		// Derive the wall-time budget so it can never pre-empt the try count:
		// maxTries attempts, each up to attemptTimeout, plus one capped backoff
		// sleep apiece (WR-02). maxTries stays the primary, binding bound.
		maxElapsed: (attemptTimeout + summaryQueueMaxInterval) * time.Duration(summaryQueueMaxTries),
		workers:    workers,
		metrics:    metrics,
	}
}

// storeFill builds the production fill closure for one record id: re-fetch
// the record, run it through the existing idempotent, vector-preserving
// Store.FillSummary seam, and emit the shared content-free egress audit line.
// Passing only the id through the queue (rather than the full store.Memory)
// and re-fetching here honors SC#1's literal "the record id is enqueued"
// wording and stays correct against a concurrent update — shouldSummarize
// (inside FillSummary) safely skips a record that was already summarized
// between enqueue and processing (RESEARCH Pitfall 4).
func storeFill(st *store.Store, summarize store.SummarizeFunc, model string, maxChars int) func(ctx context.Context, id string) error {
	return func(ctx context.Context, id string) error {
		m, err := st.Get(ctx, id)
		if err != nil {
			// The record vanished (deleted) or is otherwise unreachable —
			// not retryable.
			return backoff.Permanent(err)
		}
		filled, ferr := st.FillSummary(ctx, m, summarize, model, maxChars)
		switch {
		case ferr != nil:
			store.LogSummaryEgress(ctx, m, model, "failed", ferr)
		case filled:
			store.LogSummaryEgress(ctx, m, model, "filled", nil)
		default:
			// Ineligible by the time this worker got to it (already
			// summarized by a concurrent path, or content shrank under the
			// cap) — no egress occurred, but still worth an audit line for
			// traceability of what happened to an enqueued id.
			store.LogSummaryEgress(ctx, m, model, "skipped", nil)
		}
		return ferr
	}
}

// tryEnqueue is the write-path call site's entry point: nil-safe (a disabled
// queue is a no-op), non-blocking (a full queue drops and counts instead of
// stalling the caller — SC#2), and never returns an error since the caller
// must never be slowed down by this best-effort path.
func (q *summaryQueue) tryEnqueue(id string) {
	if q == nil {
		return
	}
	// RLock serializes only against Shutdown's close (concurrent enqueues still
	// proceed in parallel); it guarantees the closed-check and the send below
	// cannot straddle Shutdown's close(ch), so a late enqueue drops instead of
	// panicking on send-to-closed (CR-01).
	q.mu.RLock()
	defer q.mu.RUnlock()
	if q.closed {
		if q.metrics != nil {
			q.metrics.Dropped(context.Background())
		}
		slog.Warn("summary queue closed; dropped, will be reclaimed by summarize-missing", "id", id)
		return
	}
	// Reserve the in-flight slot BEFORE the send so a fast worker cannot run
	// itemDone (inFlight.Done) before this Add and drive the WaitGroup negative;
	// the drop branch releases the reservation to stay balanced.
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
		slog.Warn("summary queue full; dropped, will be reclaimed by summarize-missing", "id", id)
	}
}

// Start launches q.workers long-lived worker goroutines that range over the
// enqueue channel until it is closed (by Shutdown). Nil-safe no-op on a
// disabled queue.
func (q *summaryQueue) Start(ctx context.Context) {
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
func (q *summaryQueue) worker(ctx context.Context) {
	defer q.wg.Done()
	for id := range q.ch {
		q.process(ctx, id)
	}
}

// process runs one id through the bounded-retry fill and always balances the
// in-flight accounting via itemDone — even if fill panics. A panicking fill
// is recovered here so it counts as a failed outcome and the worker keeps
// draining subsequent ids; the pool can never wedge and Wait() always
// eventually returns (Codex finding #4).
func (q *summaryQueue) process(ctx context.Context, id string) {
	defer q.itemDone()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("summary queue: fill panicked; recovered", "id", id, "panic", r)
			if q.metrics != nil {
				q.metrics.Failed(context.Background())
			}
		}
	}()

	start := time.Now()
	_, err := backoff.Retry(ctx, func() (struct{}, error) {
		// PER-ATTEMPT TIMEOUT (Codex finding #2): backoff only evaluates
		// elapsed time AFTER operation() returns, so without this a single
		// hung attempt (e.g. a stuck /v1/chat/completions call under a
		// gateway brownout) would stall this worker for the summarizer's
		// full HTTP timeout instead of being cut short.
		attemptCtx, cancel := context.WithTimeout(ctx, q.attemptTimeout)
		defer cancel()
		return struct{}{}, q.fill(attemptCtx, id)
	},
		backoff.WithBackOff(newRetryBackOff()),
		backoff.WithMaxTries(summaryQueueMaxTries),
		backoff.WithMaxElapsedTime(q.maxElapsed),
		backoff.WithNotify(func(err error, d time.Duration) {
			if q.metrics != nil {
				q.metrics.Retried(context.Background())
			}
			slog.Debug("summary queue: fill retrying", "id", id, "err", err, "next_in", d)
		}),
	)
	if q.metrics != nil {
		q.metrics.RecordFill(context.Background(), time.Since(start).Seconds())
	}
	if err != nil {
		// Retry budget exhausted (or a permanent error): give up. The
		// summarize-missing sweep reclaims it as a backstop.
		if q.metrics != nil {
			q.metrics.Failed(context.Background())
		}
		slog.Warn("summary queue: fill gave up", "id", id, "err", err)
	}
}

// itemDone balances one successfully-enqueued id's in-flight accounting.
// Always called via defer in process, regardless of how process exits
// (success, exhausted retries, or a recovered panic), so Wait() can never
// wedge.
func (q *summaryQueue) itemDone() {
	q.inFlight.Done()
}

// newRetryBackOff builds the ExponentialBackOff used by every fill retry,
// with an explicit low MaxInterval rather than the library's 60s default
// (Open Question 1, resolved).
func newRetryBackOff() *backoff.ExponentialBackOff {
	b := backoff.NewExponentialBackOff()
	b.MaxInterval = summaryQueueMaxInterval
	return b
}

// Shutdown stops accepting new work and waits for in-flight fills to finish,
// bounded by ctx. Two layers protect the channel close from a concurrent send:
// (1) the caller (serve.go) drains only AFTER httpSrv.Shutdown(ctx) returns, so
// most handlers have already finished, and (2) this method takes mu.Lock
// (mutually exclusive with tryEnqueue's RLock) and flips closed before
// close(ch), so even a handler that outlived Shutdown's grace window — Go's
// net/http does NOT force-terminate handlers when Shutdown's context expires —
// is dropped by tryEnqueue rather than panicking on send-to-closed (CR-01).
// Idempotent, and never hangs past ctx's deadline. Nil-safe no-op on a disabled
// queue.
func (q *summaryQueue) Shutdown(ctx context.Context) {
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
// reached a terminal fill outcome (success, exhausted retries, or a
// recovered panic) — the deterministic drain seam tests use instead of
// time.Sleep polling. Nil-safe no-op on a disabled queue.
func (q *summaryQueue) Wait() {
	if q == nil {
		return
	}
	q.inFlight.Wait()
}

// depth samples the current enqueue channel occupancy for the D-09 OTel
// queue-depth gauge (registered separately, by Wave-3 buildDepsFromEnv, via
// telemetry.RegisterSummaryQueueDepth — this queue does not construct the
// gauge itself). Nil-safe: a disabled queue reports depth 0.
func (q *summaryQueue) depth() int64 {
	if q == nil {
		return 0
	}
	return int64(len(q.ch))
}
