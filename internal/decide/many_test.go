// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package decide

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDecideManyOrder(t *testing.T) {
	const n = 8
	reqs := make([]Request, n)
	decideOne := func(_ context.Context, req Request) (Response, error) {
		idx := len(req.Questions) // marker: number of questions == index sentinel below
		// sleep longer for lower indexes so completion order is reversed
		time.Sleep(time.Duration(n-idx) * time.Millisecond)
		return Response{ID: fmt.Sprintf("resp-%d", idx)}, nil
	}
	for i := range reqs {
		// use question count as an index marker the fake can read back
		qs := make(map[string]Question, i)
		for j := range i {
			qs[fmt.Sprintf("q%d", j)] = Noul("i", "t", "f")
		}
		reqs[i] = Request{Questions: qs}
	}

	results := DecideMany(context.Background(), decideOne, reqs, 4)
	if len(results) != n {
		t.Fatalf("len(results) = %d, want %d", len(results), n)
	}
	for i, r := range results {
		want := fmt.Sprintf("resp-%d", i)
		if r.Response.ID != want {
			t.Errorf("results[%d].Response.ID = %q, want %q (order not preserved)", i, r.Response.ID, want)
		}
	}
}

func TestDecideManyBound(t *testing.T) {
	for _, concurrency := range []int{1, 2, 4} {
		t.Run(fmt.Sprintf("concurrency=%d", concurrency), func(t *testing.T) {
			const n = 12
			var inFlight, peak int64
			decideOne := func(_ context.Context, _ Request) (Response, error) {
				cur := atomic.AddInt64(&inFlight, 1)
				for {
					p := atomic.LoadInt64(&peak)
					if cur <= p || atomic.CompareAndSwapInt64(&peak, p, cur) {
						break
					}
				}
				time.Sleep(5 * time.Millisecond)
				atomic.AddInt64(&inFlight, -1)
				return Response{}, nil
			}
			reqs := make([]Request, n)
			DecideMany(context.Background(), decideOne, reqs, concurrency)

			if atomic.LoadInt64(&peak) > int64(concurrency) {
				t.Errorf("peak in flight = %d, want <= %d", peak, concurrency)
			}
			if concurrency == 4 && atomic.LoadInt64(&peak) != 4 {
				t.Errorf("peak in flight = %d, want exactly 4 (pool not actually used)", peak)
			}
		})
	}
}

func TestDecideManyConcurrencyFloor(t *testing.T) {
	for _, concurrency := range []int{0, -3} {
		t.Run(fmt.Sprintf("concurrency=%d", concurrency), func(t *testing.T) {
			const n = 6
			var inFlight, peak int64
			decideOne := func(_ context.Context, _ Request) (Response, error) {
				cur := atomic.AddInt64(&inFlight, 1)
				for {
					p := atomic.LoadInt64(&peak)
					if cur <= p || atomic.CompareAndSwapInt64(&peak, p, cur) {
						break
					}
				}
				time.Sleep(2 * time.Millisecond)
				atomic.AddInt64(&inFlight, -1)
				return Response{}, nil
			}
			reqs := make([]Request, n)
			DecideMany(context.Background(), decideOne, reqs, concurrency)
			if atomic.LoadInt64(&peak) != 1 {
				t.Errorf("peak in flight = %d, want exactly 1 (non-positive concurrency should floor to 1)", peak)
			}
		})
	}
}

func TestDecideManyIsolation(t *testing.T) {
	const n = 8
	decideOne := func(_ context.Context, req Request) (Response, error) {
		idx := len(req.Questions)
		switch idx {
		case 3:
			return Response{}, ErrDecisionUnavailable
		case 5:
			panic("boom")
		default:
			return Response{ID: fmt.Sprintf("ok-%d", idx)}, nil
		}
	}
	reqs := make([]Request, n)
	for i := range reqs {
		qs := make(map[string]Question, i)
		for j := range i {
			qs[fmt.Sprintf("q%d", j)] = Noul("i", "t", "f")
		}
		reqs[i] = Request{Questions: qs}
	}

	results := DecideMany(context.Background(), decideOne, reqs, 4)
	if len(results) != n {
		t.Fatalf("len(results) = %d, want %d", len(results), n)
	}
	if !errors.Is(results[3].Err, ErrDecisionUnavailable) {
		t.Errorf("results[3].Err = %v, want ErrDecisionUnavailable", results[3].Err)
	}
	if results[5].Err == nil || !strings.Contains(results[5].Err.Error(), "backend panicked") {
		t.Errorf("results[5].Err = %v, want an error containing %q", results[5].Err, "backend panicked")
	}
	for _, i := range []int{0, 1, 2, 4, 6, 7} {
		want := fmt.Sprintf("ok-%d", i)
		if results[i].Err != nil || results[i].Response.ID != want {
			t.Errorf("results[%d] = %+v, want a clean success (%q)", i, results[i], want)
		}
	}
}

func TestDecideManyEmpty(t *testing.T) {
	calls := 0
	decideOne := func(_ context.Context, _ Request) (Response, error) {
		calls++
		return Response{}, nil
	}

	for _, reqs := range [][]Request{nil, {}} {
		results := DecideMany(context.Background(), decideOne, reqs, 4)
		if results == nil {
			t.Error("DecideMany returned nil, want a non-nil empty slice")
		}
		if len(results) != 0 {
			t.Errorf("len(results) = %d, want 0", len(results))
		}
	}
	if calls != 0 {
		t.Errorf("decideOne called %d times, want 0", calls)
	}
}

func TestDecideManyCancel(t *testing.T) {
	const n = 5
	ctx, cancel := context.WithCancel(context.Background())
	var calls int64
	decideOne := func(_ context.Context, _ Request) (Response, error) {
		atomic.AddInt64(&calls, 1)
		cancel()
		return Response{}, nil
	}
	reqs := make([]Request, n)

	results := DecideMany(ctx, decideOne, reqs, 1)

	if len(results) != n {
		t.Fatalf("len(results) = %d, want %d", len(results), n)
	}
	if got := atomic.LoadInt64(&calls); got != 1 {
		t.Errorf("decideOne called %d times, want exactly 1", got)
	}
	for i := 1; i < n; i++ {
		if !errors.Is(results[i].Err, context.Canceled) {
			t.Errorf("results[%d].Err = %v, want context.Canceled", i, results[i].Err)
		}
	}
}
