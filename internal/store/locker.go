// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"sync"
)

// TargetLocker serializes concurrent Store.Supersede calls against the same
// target id, closing the check-then-act race between the "already
// superseded" guard and the back-stamping SetPayload (see Supersede's doc
// comment). Lock blocks until the lock for key is acquired or ctx is done,
// returning an unlock function the caller MUST invoke exactly once
// (typically via defer) to release it.
//
// This interface exists so the default single-process implementation
// (inProcessTargetLocker) can later be swapped for a distributed lock
// (Redis, etcd, a Qdrant-backed compare-and-swap, etc.) — via WithTargetLocker
// — without touching Supersede's logic. The in-process default only
// guarantees atomicity within one engram server instance: concurrent
// Supersede calls against the SAME target racing across two different
// server replicas are NOT serialized by it.
type TargetLocker interface {
	// Lock acquires the lock for key, blocking until it is free or ctx is
	// done. Returns an unlock func (never nil on success) that releases the
	// lock; callers must always call it, typically via defer.
	Lock(ctx context.Context, key string) (unlock func(), err error)
}

// inProcessTargetLocker is the default TargetLocker: a sync.Map of per-key
// *sync.Mutex, providing single-process mutual exclusion only. Keys are
// never evicted — bounded by the number of distinct records ever superseded
// by this process, an accepted tradeoff (no distributed-lock primitive
// exists in this codebase; see the TargetLocker doc comment).
type inProcessTargetLocker struct {
	mus sync.Map // key: string target id -> *sync.Mutex
}

// newInProcessTargetLocker returns a ready-to-use in-process TargetLocker.
func newInProcessTargetLocker() *inProcessTargetLocker {
	return &inProcessTargetLocker{}
}

// Lock implements TargetLocker. It only observes ctx before blocking (an
// already-canceled context is rejected up front); once acquired, the
// underlying sync.Mutex.Lock() itself does not support cancellation, matching
// every other synchronous store call in this package.
func (l *inProcessTargetLocker) Lock(ctx context.Context, key string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	muAny, _ := l.mus.LoadOrStore(key, &sync.Mutex{})
	mu, _ := muAny.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock, nil
}
