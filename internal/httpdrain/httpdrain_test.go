// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package httpdrain

import (
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

// fakeBody is a hand-written io.ReadCloser recording Read and Close calls,
// used to drive Drain without a real HTTP server — this layer tests the
// mechanism in isolation, not the network.
//
// When block is true, Read ignores data and blocks until Close is called,
// mirroring net/http's documented behavior that Close on a response body
// unblocks an in-flight Read (D-01) — the exact mechanism Drain's time bound
// relies on.
type fakeBody struct {
	mu       sync.Mutex
	closed   bool
	closedCh chan struct{}
	reads    int
	data     []byte
	block    bool
}

func newFakeBody(data []byte, block bool) *fakeBody {
	return &fakeBody{closedCh: make(chan struct{}), data: data, block: block}
}

func (b *fakeBody) Read(p []byte) (int, error) {
	b.mu.Lock()
	b.reads++
	b.mu.Unlock()

	if b.block {
		<-b.closedCh
		return 0, errors.New("read on closed body")
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, b.data)
	b.data = b.data[n:]
	return n, nil
}

func (b *fakeBody) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.closed {
		b.closed = true
		close(b.closedCh)
	}
	return nil
}

func (b *fakeBody) readCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.reads
}

func (b *fakeBody) remaining() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.data)
}

func (b *fakeBody) isClosed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closed
}

// TestDrainStopsAtByteBound proves the byte axis: Drain reads no more than
// maxBytes even when the body has far more to give, and it never closes the
// body on this path (the timer never fires within the generous maxTime).
func TestDrainStopsAtByteBound(t *testing.T) {
	body := newFakeBody(make([]byte, 100), false)

	Drain(body, 10, time.Second)

	if got := body.remaining(); got != 90 {
		t.Fatalf("remaining() = %d, want 90 (only 10 of 100 bytes should have been consumed)", got)
	}
	if body.readCount() != 1 {
		t.Fatalf("readCount() = %d, want 1 (LimitReader should stop after the first Read reaches the bound)", body.readCount())
	}
	if body.isClosed() {
		t.Fatal("body was closed, want it left open — the time bound never fired on this path")
	}
}

// TestDrainClosesBodyAtTimeBound proves the time axis: a body whose Read
// blocks forever is still unblocked and abandoned once maxTime elapses,
// because the timer's Close call releases the in-flight Read (D-01). Drain
// must return promptly, not hang for the test's lifetime.
func TestDrainClosesBodyAtTimeBound(t *testing.T) {
	body := newFakeBody(nil, true)

	start := time.Now()
	Drain(body, 1<<20, 20*time.Millisecond)
	elapsed := time.Since(start)

	if elapsed > 500*time.Millisecond {
		t.Fatalf("Drain took %v; want bounded by the tiny maxTime, not a hang", elapsed)
	}
	if !body.isClosed() {
		t.Fatal("body was not closed; want the time bound's timer to have closed it")
	}
	if body.readCount() != 1 {
		t.Fatalf("readCount() = %d, want 1 (the single blocked Read that the timer unblocked)", body.readCount())
	}
}

// TestDrainZeroSkipsEntirely proves a non-positive value on EITHER bound
// closes the body immediately and reads nothing (D-05) — there is no value
// that means "unbounded", and a zero is a deliberate, safe setting rather
// than an oversight to guard against.
func TestDrainZeroSkipsEntirely(t *testing.T) {
	cases := []struct {
		name     string
		maxBytes int64
		maxTime  time.Duration
	}{
		{"zero bytes", 0, time.Second},
		{"zero time", 10, 0},
		{"negative bytes", -1, time.Second},
		{"negative time", 10, -1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := newFakeBody(make([]byte, 10), false)

			Drain(body, tc.maxBytes, tc.maxTime)

			if body.readCount() != 0 {
				t.Fatalf("readCount() = %d, want 0 (a non-positive bound must perform zero reads)", body.readCount())
			}
			if !body.isClosed() {
				t.Fatal("body was not closed; want it closed immediately on a non-positive bound")
			}
		})
	}
}
