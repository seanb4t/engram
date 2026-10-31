// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"sync"
	"time"
)

// warnWindow is how long a distinctWarner remembers the values it admitted.
// The remembered set starts again empty after each window, so a flood cannot
// silence a later warning for longer than that.
const warnWindow = time.Hour

// distinctWarner bounds the warnings logged for a client-supplied value: one
// per distinct value in each warnWindow, for at most limit values, each cut
// to maxLen bytes. The two bounds fix the memory that a flood of values can
// use. Build one with newDistinctWarner.
type distinctWarner struct {
	limit, maxLen int
	// now is the clock for warnWindow; nil means time.Now.
	now func() time.Time

	mu     sync.Mutex
	warned map[string]struct{}
	since  time.Time
}

// newDistinctWarner returns a warner for at most limit values of at most
// maxLen bytes each. It panics unless both are positive, because a zero
// limit admits no value and so logs no warning.
func newDistinctWarner(limit, maxLen int) *distinctWarner {
	if limit <= 0 || maxLen <= 0 {
		panic(fmt.Sprintf("newDistinctWarner: limit %d and maxLen %d must both be positive", limit, maxLen))
	}
	return &distinctWarner{limit: limit, maxLen: maxLen}
}

// admit cuts value to maxLen bytes and returns the cut value, whether to log
// a warning for it, and whether that warning is the one that reaches the
// limit, after which the caller logs one limit warning.
func (d *distinctWarner) admit(value string) (cut string, warn, atLimit bool) {
	cut = truncate(value, d.maxLen)
	now := time.Now
	if d.now != nil {
		now = d.now
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if t := now(); d.warned == nil || t.Sub(d.since) >= warnWindow {
		d.warned = map[string]struct{}{}
		d.since = t
	}
	if _, seen := d.warned[cut]; seen || len(d.warned) >= d.limit {
		return cut, false, false
	}
	d.warned[cut] = struct{}{}
	return cut, true, len(d.warned) == d.limit
}
