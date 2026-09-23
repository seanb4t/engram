// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package decide

import (
	"context"
	"fmt"
	"sync"
)

// DecideMany runs decideOne over reqs through a bounded worker pool (D-10):
// results are in input order, regardless of completion order; one item's
// failure or panic never affects another; at most max(1, concurrency) calls
// are ever in flight, so there is never an unbounded fan-out. Before calling
// decideOne for an item, a worker checks ctx: once ctx is done, every
// remaining item is filled with ctx.Err() and decideOne is never called for
// it. A nil or empty reqs returns a non-nil empty slice with zero calls.
//
//nolint:revive // the exported name DecideMany is D-10's spec'd call shape (decide.DecideMany), required verbatim by 02-04-PLAN.md and jev.go's delegation
func DecideMany(ctx context.Context, decideOne func(context.Context, Request) (Response, error), reqs []Request, concurrency int) []Result {
	results := make([]Result, len(reqs))
	if len(reqs) == 0 {
		return results
	}

	workers := max(concurrency, 1)
	workers = min(workers, len(reqs))

	// Filled synchronously (not by a separate producer goroutine) and
	// closed before any worker starts, so `range indexes` below drains it
	// without a second dispatch goroutine — the worker launch below is the
	// only place this file spawns concurrency.
	indexes := make(chan int, len(reqs))
	for i := range reqs {
		indexes <- i
	}
	close(indexes)

	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for i := range indexes {
				if err := ctx.Err(); err != nil {
					results[i] = Result{Err: err}
					continue
				}
				results[i] = decideOneSafe(ctx, decideOne, reqs[i], i)
			}
		}()
	}
	wg.Wait()
	return results
}

// decideOneSafe calls decideOne and recovers a panic into Result.Err, so one
// backend panic can never take down the worker pool or wedge Wait().
func decideOneSafe(ctx context.Context, decideOne func(context.Context, Request) (Response, error), req Request, i int) (result Result) {
	defer func() {
		if r := recover(); r != nil {
			result = Result{Err: fmt.Errorf("decide: request %d: backend panicked: %v", i, r)}
		}
	}()
	resp, err := decideOne(ctx, req)
	return Result{Response: resp, Err: err}
}
