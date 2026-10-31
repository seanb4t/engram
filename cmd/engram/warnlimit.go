// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"sync"
	"time"
)

// warnWindow is how long a distinctWarner remembers the values it admitted.
// The remembered set starts again empty after each window, so a flood cannot
// silence a later warning for longer than that.
const warnWindow = time.Hour

// distinctWarner bounds the warnings logged for a client-supplied value: one
// per distinct value in each warnWindow, for at most limit values. The caller
// cuts each value to a fixed length first, so a flood of values cannot grow
// memory without bound.
type distinctWarner struct {
	limit int
	// now is the clock for warnWindow; nil means time.Now.
	now func() time.Time

	mu     sync.Mutex
	warned map[string]struct{}
	since  time.Time
}

// admit reports whether to log a warning for value, and whether that
// warning is the one that reaches the limit, after which the caller logs
// one limit warning.
func (d *distinctWarner) admit(value string) (warn, atLimit bool) {
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
	if _, seen := d.warned[value]; seen || len(d.warned) >= d.limit {
		return false, false
	}
	d.warned[value] = struct{}{}
	return true, len(d.warned) == d.limit
}
