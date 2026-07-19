// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// TestInProcessTargetLockerCanceledContextRejected (IN-03) pins Lock's
// up-front ctx.Err() check: an already-canceled context is rejected
// immediately, without blocking, and without acquiring the lock — mirroring
// every other synchronous store call in this package (see Lock's doc
// comment, locker.go:46-49).
func TestInProcessTargetLockerCanceledContextRejected(t *testing.T) {
	l := newInProcessTargetLocker()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	unlock, err := l.Lock(ctx, "some-key")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Lock with canceled ctx: err = %v, want context.Canceled", err)
	}
	if unlock != nil {
		t.Error("Lock with canceled ctx: unlock func is non-nil, want nil (lock must not be held on rejection)")
	}

	// The rejection must not have taken the lock: a fresh, valid-context Lock
	// on the SAME key must succeed immediately.
	unlock2, err := l.Lock(context.Background(), "some-key")
	if err != nil {
		t.Fatalf("Lock on same key after canceled-ctx rejection: %v", err)
	}
	unlock2()
}

// TestInProcessTargetLockerSameKeySerializes (IN-03) pins the core
// invariant Store.Supersede and Store.Update both depend on: two Lock calls
// on the SAME key must serialize — the second blocks until the first's
// unlock runs. Uses a channel-based ordering assertion (never a sleep) so
// this is deterministic under -race.
func TestInProcessTargetLockerSameKeySerializes(t *testing.T) {
	l := newInProcessTargetLocker()
	const key = "same-key"

	unlock1, err := l.Lock(context.Background(), key)
	if err != nil {
		t.Fatalf("first Lock: %v", err)
	}

	acquired := make(chan struct{})
	go func() {
		unlock2, err := l.Lock(context.Background(), key)
		if err != nil {
			t.Errorf("second Lock: %v", err)
			return
		}
		close(acquired)
		unlock2()
	}()

	// The second Lock must NOT have acquired yet — it should still be
	// blocked behind the first, still-held lock.
	select {
	case <-acquired:
		t.Fatal("second Lock on the same key acquired before the first unlocked")
	case <-time.After(50 * time.Millisecond):
		// expected: still blocked
	}

	unlock1()

	select {
	case <-acquired:
		// expected: the second Lock proceeds once the first releases
	case <-time.After(2 * time.Second):
		t.Fatal("second Lock on the same key never acquired after the first unlocked")
	}
}

// TestInProcessTargetLockerDifferentKeysDoNotBlock (IN-03) pins that Lock
// calls on DIFFERENT keys never contend — the locker is keyed per-target,
// not a single global mutex (see TargetLocker's doc comment: "different
// targets never contend").
func TestInProcessTargetLockerDifferentKeysDoNotBlock(t *testing.T) {
	l := newInProcessTargetLocker()

	unlockA, err := l.Lock(context.Background(), "key-a")
	if err != nil {
		t.Fatalf("Lock key-a: %v", err)
	}
	defer unlockA()

	done := make(chan struct{})
	go func() {
		unlockB, err := l.Lock(context.Background(), "key-b")
		if err != nil {
			t.Errorf("Lock key-b: %v", err)
			return
		}
		defer unlockB()
		close(done)
	}()

	select {
	case <-done:
		// expected: a different key acquires immediately, never blocked by key-a.
	case <-time.After(2 * time.Second):
		t.Fatal("Lock on a different key blocked while an unrelated key's lock was held")
	}
}

// TestInProcessTargetLockerConcurrentDistinctKeys (IN-03) exercises many
// goroutines locking many distinct keys concurrently, under -race, so the
// underlying sync.Map-backed *sync.Mutex storage itself is proven race-free
// under real contention (not just the single-pair cases above).
func TestInProcessTargetLockerConcurrentDistinctKeys(t *testing.T) {
	l := newInProcessTargetLocker()
	const n = 50

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			key := string(rune('a' + i%26))
			unlock, err := l.Lock(context.Background(), key)
			if err != nil {
				t.Errorf("Lock %q: %v", key, err)
				return
			}
			unlock()
		}(i)
	}
	wg.Wait()
}
